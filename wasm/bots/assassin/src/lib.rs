// AssassinBot WASM implementation — decapitation archetype: every unit rushes
// the enemy core, ignoring enemies and economy; relies on speed and mass.
// Port of bots/assassin/src/strategy.rs per plan §13.1.
//
// Uses the low-level WASM interface compatible with the sandbox loader:
//   allocate(size) -> ptr            — buffer for the host to write input into
//   init(ptr, len)                  — config JSON for a new match
//   compute_moves(ptr, len) -> ptr  — state JSON in, moves JSON out (NUL-terminated)
//   free_result(ptr)                — release a result buffer

use std::collections::{HashMap, HashSet, VecDeque};
use std::sync::Mutex;

use serde::{Deserialize, Serialize};

// ────────────────────────────────────────────────────────────────────────────
// Game protocol types (engine VisibleState JSON)
// ────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct Position {
    pub row: i32,
    pub col: i32,
}

#[derive(Debug, Clone, Copy, Default, Deserialize)]
pub struct GameConfig {
    #[serde(default)]
    pub rows: i32,
    #[serde(default)]
    pub cols: i32,
    #[serde(default)]
    pub max_turns: i32,
    #[serde(default)]
    pub vision_radius2: i32,
    #[serde(default)]
    pub attack_radius2: i32,
    #[serde(default)]
    pub spawn_cost: i32,
    #[serde(default)]
    pub energy_interval: i32,
}

#[derive(Debug, Clone, Copy, Default, Deserialize)]
pub struct PlayerInfo {
    #[serde(default)]
    pub id: i32,
    #[serde(default)]
    pub energy: i32,
    #[serde(default)]
    pub score: i32,
}

#[derive(Debug, Clone, Copy, Deserialize)]
pub struct VisibleBot {
    pub position: Position,
    #[serde(default)]
    pub owner: i32,
}

#[derive(Debug, Clone, Copy, Deserialize)]
pub struct VisibleCore {
    pub position: Position,
    #[serde(default)]
    pub owner: i32,
    #[serde(default)]
    pub active: bool,
}

#[derive(Debug, Clone, Copy, Default, Deserialize)]
pub struct ZoneBounds {
    #[serde(default)]
    pub center: Position,
    #[serde(default)]
    pub radius: i32,
    #[serde(default)]
    pub active: bool,
}

#[derive(Debug, Clone, Deserialize)]
pub struct GameState {
    #[serde(default)]
    pub match_id: String,
    #[serde(default)]
    pub turn: i32,
    #[serde(default)]
    pub config: GameConfig,
    #[serde(default)]
    pub you: PlayerInfo,
    #[serde(default)]
    pub bots: Vec<VisibleBot>,
    #[serde(default)]
    pub energy: Vec<Position>,
    #[serde(default)]
    pub cores: Vec<VisibleCore>,
    #[serde(default)]
    pub walls: Vec<Position>,
    #[serde(default)]
    pub zone: Option<ZoneBounds>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
pub enum Direction {
    #[serde(rename = "N")]
    N,
    #[serde(rename = "E")]
    E,
    #[serde(rename = "S")]
    S,
    #[serde(rename = "W")]
    W,
}

#[derive(Debug, Clone, Copy, Serialize)]
pub struct Move {
    pub position: Position,
    pub direction: Direction,
}

impl Direction {
    /// All directions in order: N, E, S, W.
    pub fn all() -> [Direction; 4] {
        [Direction::N, Direction::E, Direction::S, Direction::W]
    }

    fn delta(self) -> (i32, i32) {
        match self {
            Direction::N => (-1, 0),
            Direction::E => (0, 1),
            Direction::S => (1, 0),
            Direction::W => (0, -1),
        }
    }
}

impl Position {
    /// Move in a direction, wrapping around the toroidal grid.
    pub fn move_toward(self, dir: Direction, rows: i32, cols: i32) -> Position {
        let (dr, dc) = dir.delta();
        Position {
            row: (self.row + dr + rows) % rows,
            col: (self.col + dc + cols) % cols,
        }
    }

    /// Squared distance with toroidal wrapping.
    pub fn distance2(self, other: Position, rows: i32, cols: i32) -> i32 {
        let dr = (self.row - other.row).abs();
        let dc = (self.col - other.col).abs();
        let dr = dr.min(rows - dr);
        let dc = dc.min(cols - dc);
        dr * dr + dc * dc
    }
}

// ────────────────────────────────────────────────────────────────────────────
// Strategy (port of bots/assassin/src/strategy.rs)
// ────────────────────────────────────────────────────────────────────────────

struct AssassinState {
    match_id: String,
    /// Enemy cores discovered so far (persisted across turns); value = last-known active.
    known_targets: HashMap<Position, bool>,
}

static STATE: Mutex<Option<AssassinState>> = Mutex::new(None);

fn assassin_moves(state: &GameState) -> Vec<Move> {
    let my_id = state.you.id;
    let rows = state.config.rows;
    let cols = state.config.cols;

    let mut guard = STATE.lock().unwrap();
    let persistent = match guard.as_mut() {
        Some(s) if s.match_id == state.match_id => s,
        _ => {
            *guard = Some(AssassinState {
                match_id: state.match_id.clone(),
                known_targets: HashMap::new(),
            });
            guard.as_mut().unwrap()
        }
    };

    // Update known targets from visible cores.
    for core in &state.cores {
        if core.owner != my_id {
            persistent
                .known_targets
                .entry(core.position)
                .and_modify(|a| *a = core.active)
                .or_insert(core.active);
        }
    }
    let targets = persistent.known_targets.clone();
    drop(guard);

    let my_bots: Vec<_> = state.bots.iter().filter(|b| b.owner == my_id).collect();
    if my_bots.is_empty() {
        return vec![];
    }

    let walls: HashSet<Position> = state.walls.iter().copied().collect();

    // Active targets, sorted by distance from our center of mass.
    let center = center_of_mass(&my_bots);
    let mut sorted_targets: Vec<Position> = targets
        .iter()
        .filter(|(_, active)| **active)
        .map(|(pos, _)| *pos)
        .collect();
    sorted_targets.sort_by_key(|t| center.distance2(*t, rows, cols));

    // If no known targets, explore outward.
    if sorted_targets.is_empty() {
        return explore_moves(&my_bots, &walls, rows, cols);
    }

    // Primary target: nearest active enemy core to our center of mass.
    let primary = sorted_targets[0];

    // BFS from each bot to the primary target, walking through enemies —
    // only walls block movement. Avoid already-claimed destinations.
    let mut moves = Vec::with_capacity(my_bots.len());
    let mut destinations: HashSet<Position> = HashSet::new();

    for bot in &my_bots {
        if let Some(dir) = bfs_toward(bot.position, primary, &walls, &destinations, rows, cols) {
            let dest = bot.position.move_toward(dir, rows, cols);
            destinations.insert(dest);
            moves.push(Move {
                position: bot.position,
                direction: dir,
            });
        }
    }

    moves
}

/// BFS toward a target position. Unlike rusher, does NOT avoid enemy bots —
/// we walk straight through them. Only walls block movement.
fn bfs_toward(
    start: Position,
    goal: Position,
    walls: &HashSet<Position>,
    claimed: &HashSet<Position>,
    rows: i32,
    cols: i32,
) -> Option<Direction> {
    if start == goal {
        return None;
    }

    let mut visited: HashSet<Position> = HashSet::new();
    let mut queue: VecDeque<(Position, Option<Direction>)> = VecDeque::new();
    visited.insert(start);
    queue.push_back((start, None));

    while let Some((pos, first_dir)) = queue.pop_front() {
        if pos == goal {
            return first_dir;
        }

        for dir in Direction::all() {
            let next = pos.move_toward(dir, rows, cols);
            if visited.contains(&next) || walls.contains(&next) {
                continue;
            }
            visited.insert(next);
            queue.push_back((next, first_dir.or(Some(dir))));
        }
    }

    // No path to goal — pick the direction that gets us closest.
    let mut best_dir = None;
    let mut best_dist = i32::MAX;
    for dir in Direction::all() {
        let next = start.move_toward(dir, rows, cols);
        if walls.contains(&next) || claimed.contains(&next) {
            continue;
        }
        let dr = (next.row - goal.row).abs();
        let dc = (next.col - goal.col).abs();
        let dist = dr.min(rows - dr) + dc.min(cols - dc);
        if dist < best_dist {
            best_dist = dist;
            best_dir = Some(dir);
        }
    }
    best_dir
}

/// When no targets are known, spread bots outward to find enemy cores.
fn explore_moves(my_bots: &[&VisibleBot], walls: &HashSet<Position>, rows: i32, cols: i32) -> Vec<Move> {
    let mut moves = Vec::with_capacity(my_bots.len());

    // Spread in a line toward the opposite side of the map.
    for (i, bot) in my_bots.iter().enumerate() {
        let target_col = if i % 2 == 0 { cols - 1 } else { 0 };
        let target_row = if i % 3 == 0 { rows / 2 } else { rows - 1 };
        let target = Position { row: target_row, col: target_col };

        let mut best_dir = None;
        let mut best_dist = i32::MAX;
        for dir in Direction::all() {
            let next = bot.position.move_toward(dir, rows, cols);
            if walls.contains(&next) {
                continue;
            }
            let dr = (next.row - target.row).abs();
            let dc = (next.col - target.col).abs();
            let dist = dr.min(rows - dr) + dc.min(cols - dc);
            if dist < best_dist {
                best_dist = dist;
                best_dir = Some(dir);
            }
        }
        if let Some(dir) = best_dir {
            moves.push(Move {
                position: bot.position,
                direction: dir,
            });
        }
    }

    moves
}

/// Center of mass of our bots.
fn center_of_mass(bots: &[&VisibleBot]) -> Position {
    if bots.is_empty() {
        return Position { row: 0, col: 0 };
    }
    let sum_r: i32 = bots.iter().map(|b| b.position.row).sum();
    let sum_c: i32 = bots.iter().map(|b| b.position.col).sum();
    Position {
        row: sum_r / bots.len() as i32,
        col: sum_c / bots.len() as i32,
    }
}

// ────────────────────────────────────────────────────────────────────────────
// WASM ABI
// ────────────────────────────────────────────────────────────────────────────

/// Static input buffer: the host calls allocate() then writes bytes into it
/// before invoking init/compute_moves with the returned pointer.
const INPUT_CAPACITY: usize = 1 << 20; // 1 MiB
static INPUT: Mutex<[u8; INPUT_CAPACITY]> = Mutex::new([0; INPUT_CAPACITY]);

fn read_input(ptr: *const u8, len: usize) -> String {
    if ptr.is_null() {
        return "{}".to_string();
    }
    let bytes = unsafe { std::slice::from_raw_parts(ptr, len) };
    String::from_utf8_lossy(bytes).into_owned()
}

/// Result buffers carry a 8-byte little-endian length header before the
/// payload and a NUL terminator after it, so free_result can reconstruct and
/// drop them, and hosts that read C strings stop at the terminator.
fn return_result(bytes: &[u8]) -> *const u8 {
    let mut v = Vec::with_capacity(8 + bytes.len() + 1);
    v.extend_from_slice(&(bytes.len() as u64).to_le_bytes());
    v.extend_from_slice(bytes);
    v.push(0);
    let ptr = unsafe { v.as_ptr().add(8) };
    std::mem::forget(v);
    ptr
}

#[no_mangle]
pub extern "C" fn allocate(size: usize) -> *mut u8 {
    let mut buf = INPUT.lock().unwrap();
    if size < INPUT_CAPACITY {
        return buf.as_mut_ptr();
    }
    // Oversized (anomalous) input: leak a dedicated buffer rather than fail.
    let mut v = Vec::<u8>::with_capacity(size);
    let ptr = v.as_mut_ptr();
    std::mem::forget(v);
    ptr
}

#[no_mangle]
pub extern "C" fn init(ptr: *const u8, len: usize) {
    // The assassin has no use for the config beyond what each state carries;
    // match-scoped state resets on the first turn of a new match.
    let _ = read_input(ptr, len);
}

#[no_mangle]
pub extern "C" fn compute_moves(ptr: *const u8, len: usize) -> *const u8 {
    let input = read_input(ptr, len);
    let state: GameState = match serde_json::from_str(&input) {
        Ok(s) => s,
        Err(_) => return return_result(b"[]"),
    };
    let moves = assassin_moves(&state);
    match serde_json::to_vec(&moves) {
        Ok(bytes) => return_result(&bytes),
        Err(_) => return_result(b"[]"),
    }
}

#[no_mangle]
pub extern "C" fn free_result(ptr: *const u8) {
    if ptr.is_null() {
        return;
    }
    unsafe {
        // The 8-byte length header precedes the payload pointer (see
        // return_result), so the allocation base is 8 bytes BEFORE ptr.
        let base = ptr.sub(8) as *mut u8;
        let len = u64::from_le_bytes(base.cast::<[u8; 8]>().read()) as usize;
        let total = 8 + len + 1;
        drop(Vec::from_raw_parts(base, total, total));
    }
}
