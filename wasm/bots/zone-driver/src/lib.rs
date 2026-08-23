// ZoneDriverBot WASM implementation — weaponizes the shrinking zone: saves
// own bots near the edge, blocks enemy escape routes from the kill band,
// sweeps to herd enemies deeper as the zone closes.
// Port of bots/zone-driver/src/strategy.rs per plan §13.1.
//
// Uses the low-level WASM interface compatible with the sandbox loader:
//   allocate(size) -> ptr            — buffer for the host to write input into
//   init(ptr, len)                  — config JSON for a new match
//   compute_moves(ptr, len) -> ptr  — state JSON in, moves JSON out (NUL-terminated)
//   free_result(ptr)                — release a result buffer

use std::collections::HashSet;
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
// Strategy (port of bots/zone-driver/src/strategy.rs)
// ────────────────────────────────────────────────────────────────────────────

fn zone_driver_moves(state: &GameState) -> Vec<Move> {
    let rows = state.config.rows;
    let cols = state.config.cols;
    let my_id = state.you.id;

    let mut my_bots = Vec::new();
    let mut enemy_bots = Vec::new();
    for bot in &state.bots {
        if bot.owner == my_id {
            my_bots.push(bot.position);
        } else {
            enemy_bots.push(bot.position);
        }
    }
    if my_bots.is_empty() {
        return vec![];
    }

    let walls: HashSet<Position> = state.walls.iter().copied().collect();
    let enemy_set: HashSet<Position> = enemy_bots.iter().copied().collect();

    let zone = match state.zone {
        Some(z) if z.active => z,
        // No active zone — play conservatively.
        _ => return defensive_fallback(&my_bots, &enemy_set, &walls, rows, cols),
    };

    let mut moves = Vec::with_capacity(my_bots.len());
    let mut assigned: HashSet<Position> = HashSet::new();

    // PRIORITY 1: save own bots outside or on the zone edge.
    for pos in &my_bots {
        if distance_to_zone_edge(*pos, zone, rows, cols) <= 0 {
            if let Some(dir) = retreat_dir(*pos, zone, &walls, rows, cols) {
                moves.push(Move {
                    position: *pos,
                    direction: dir,
                });
                assigned.insert(*pos);
            }
        }
    }

    // PRIORITY 2: block escape routes of enemies in the kill band (the ring
    // just inside the zone edge where enemies die next shrink).
    let kill_band_inner = (zone.radius - 2).max(0);
    let kill_band_outer = zone.radius;
    for pos in &my_bots {
        if assigned.contains(pos) {
            continue;
        }
        if let Some(target) =
            enemy_in_kill_band(*pos, &enemy_bots, zone, kill_band_inner, kill_band_outer, rows, cols)
        {
            if let Some(dir) = block_escape_dir(*pos, target, zone, &walls, rows, cols) {
                moves.push(Move {
                    position: *pos,
                    direction: dir,
                });
                assigned.insert(*pos);
            }
        }
    }

    // PRIORITY 3: sweep to apply pressure.
    for pos in &my_bots {
        if assigned.contains(pos) {
            continue;
        }
        if let Some(dir) = advance_pressure_dir(*pos, &enemy_set, zone, &walls, rows, cols) {
            moves.push(Move {
                position: *pos,
                direction: dir,
            });
            assigned.insert(*pos);
        }
    }

    // Remaining bots hold position (the ladder bot emits a nominal N here).
    for pos in &my_bots {
        if !assigned.contains(pos) {
            moves.push(Move {
                position: *pos,
                direction: Direction::N,
            });
        }
    }
    moves
}

/// Positive inside the zone, 0 on the edge, negative outside. Truncates
/// toward zero, matching the ladder bot's int() conversion.
fn distance_to_zone_edge(pos: Position, zone: ZoneBounds, rows: i32, cols: i32) -> i32 {
    let dist = (pos.distance2(zone.center, rows, cols) as f64).sqrt();
    (zone.radius as f64 - dist) as i32
}

/// Step that most reduces squared distance to the zone center.
fn retreat_dir(
    pos: Position,
    zone: ZoneBounds,
    walls: &HashSet<Position>,
    rows: i32,
    cols: i32,
) -> Option<Direction> {
    let current = pos.distance2(zone.center, rows, cols);
    let mut best = None;
    let mut best_reduction = i32::MIN;
    for dir in Direction::all() {
        let step = pos.move_toward(dir, rows, cols);
        if walls.contains(&step) {
            continue;
        }
        let next = step.distance2(zone.center, rows, cols);
        if next < current && current - next > best_reduction {
            best_reduction = current - next;
            best = Some(dir);
        }
    }
    best
}

/// Nearest enemy within the kill band [inner, outer] (distances from center).
fn enemy_in_kill_band(
    my_pos: Position,
    enemies: &[Position],
    zone: ZoneBounds,
    inner: i32,
    outer: i32,
    rows: i32,
    cols: i32,
) -> Option<Position> {
    let mut best = None;
    let mut best_dist = i32::MAX;
    for pos in enemies {
        let dist = (pos.distance2(zone.center, rows, cols) as f64).sqrt();
        if dist < inner as f64 || dist > outer as f64 {
            continue;
        }
        let d = my_pos.distance2(*pos, rows, cols);
        if d < best_dist {
            best_dist = d;
            best = Some(*pos);
        }
    }
    best
}

/// Moves toward the tile one step inward of the enemy (between it and the
/// zone center).
fn block_escape_dir(
    my_pos: Position,
    enemy_pos: Position,
    zone: ZoneBounds,
    walls: &HashSet<Position>,
    rows: i32,
    cols: i32,
) -> Option<Direction> {
    let dr = (zone.center.row - enemy_pos.row) as f64;
    let dc = (zone.center.col - enemy_pos.col) as f64;
    let length = (dr * dr + dc * dc).sqrt();
    if length < 0.1 {
        return None;
    }
    let ideal_row = enemy_pos.row as f64 + dr / length;
    let ideal_col = enemy_pos.col as f64 + dc / length;

    let mut best = None;
    let mut best_dist = f64::MAX;
    for dir in Direction::all() {
        let step = my_pos.move_toward(dir, rows, cols);
        if walls.contains(&step) {
            continue;
        }
        let fdr = ideal_row - step.row as f64;
        let fdc = ideal_col - step.col as f64;
        let d = (fdr * fdr + fdc * fdc).sqrt();
        if d < best_dist {
            best_dist = d;
            best = Some(dir);
        }
    }
    best
}

fn advance_pressure_dir(
    pos: Position,
    enemy_set: &HashSet<Position>,
    zone: ZoneBounds,
    walls: &HashSet<Position>,
    rows: i32,
    cols: i32,
) -> Option<Direction> {
    // Enemies visible — advance toward the nearest.
    if !enemy_set.is_empty() {
        let mut nearest = Position::default();
        let mut nearest_dist = i32::MAX;
        for e in enemy_set {
            let d = pos.distance2(*e, rows, cols);
            if d < nearest_dist {
                nearest_dist = d;
                nearest = *e;
            }
        }
        let mut best = None;
        let mut best_dist = i32::MAX;
        for dir in Direction::all() {
            let step = pos.move_toward(dir, rows, cols);
            if walls.contains(&step) {
                continue;
            }
            let d = step.distance2(nearest, rows, cols);
            if d < best_dist {
                best_dist = d;
                best = Some(dir);
            }
        }
        if best.is_some() {
            return best;
        }
    }

    // No enemies — move out to the pressure ring (radius − 3).
    let target_radius = (zone.radius - 3).max(0);
    let target_dist2 = target_radius * target_radius;
    let current = pos.distance2(zone.center, rows, cols);
    if current < target_dist2 {
        let mut best = None;
        let mut best_increase = i32::MIN;
        for dir in Direction::all() {
            let step = pos.move_toward(dir, rows, cols);
            if walls.contains(&step) {
                continue;
            }
            let increase = step.distance2(zone.center, rows, cols) - current;
            if increase > best_increase {
                best_increase = increase;
                best = Some(dir);
            }
        }
        return best;
    }
    None
}

fn defensive_fallback(
    my_bots: &[Position],
    enemy_set: &HashSet<Position>,
    walls: &HashSet<Position>,
    rows: i32,
    cols: i32,
) -> Vec<Move> {
    let mut moves = Vec::new();
    for pos in my_bots {
        if enemy_set.is_empty() {
            continue;
        }
        let mut nearest = Position::default();
        let mut nearest_dist = i32::MAX;
        for e in enemy_set {
            let d = pos.distance2(*e, rows, cols);
            if d < nearest_dist {
                nearest_dist = d;
                nearest = *e;
            }
        }
        let mut best = None;
        let mut best_dist = i32::MAX;
        for dir in Direction::all() {
            let step = pos.move_toward(dir, rows, cols);
            if walls.contains(&step) || enemy_set.contains(&step) {
                continue;
            }
            let d = step.distance2(nearest, rows, cols);
            if d < best_dist {
                best_dist = d;
                best = Some(dir);
            }
        }
        if let Some(dir) = best {
            moves.push(Move {
                position: *pos,
                direction: dir,
            });
        }
    }
    moves
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
    // The zone driver is stateless; nothing to initialize.
    let _ = read_input(ptr, len);
}

#[no_mangle]
pub extern "C" fn compute_moves(ptr: *const u8, len: usize) -> *const u8 {
    let input = read_input(ptr, len);
    let state: GameState = match serde_json::from_str(&input) {
        Ok(s) => s,
        Err(_) => return return_result(b"[]"),
    };
    let moves = zone_driver_moves(&state);
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
