// PhalanxBot WASM implementation — tight formation combat: circular-mean
// centroid, hex formation slots, rally when cohesion breaks, advance on the
// enemy concentration otherwise.
// Port of bots/phalanx/src/strategy.rs per plan §13.1.
//
// Uses the low-level WASM interface compatible with the sandbox loader:
//   allocate(size) -> ptr            — buffer for the host to write input into
//   init(ptr, len)                  — config JSON for a new match
//   compute_moves(ptr, len) -> ptr  — state JSON in, moves JSON out (NUL-terminated)
//   free_result(ptr)                — release a result buffer

use std::collections::{HashMap, HashSet};
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
// Strategy (port of bots/phalanx/src/strategy.rs)
// ────────────────────────────────────────────────────────────────────────────

const PHX_FORMATION_RADIUS2: f64 = 9.0; // max mean squared distance from centroid before rally
const PHX_ADVANCE_WEIGHT: f64 = 10.0;
const PHX_FORMATION_WEIGHT: f64 = 8.0;
const PHX_ATTACK_RANGE_BONUS: f64 = 50.0;

struct PhalanxState {
    match_id: String,
    /// Smoothed centroid carried across turns; None on the first turn.
    centroid: Option<Position>,
}

static STATE: Mutex<Option<PhalanxState>> = Mutex::new(None);

fn phalanx_moves(state: &GameState) -> Vec<Move> {
    let rows = state.config.rows;
    let cols = state.config.cols;
    let my_id = state.you.id;

    let mut guard = STATE.lock().unwrap();
    let persistent = match guard.as_mut() {
        Some(s) if s.match_id == state.match_id => s,
        _ => {
            *guard = Some(PhalanxState {
                match_id: state.match_id.clone(),
                centroid: None,
            });
            guard.as_mut().unwrap()
        }
    };

    let my_bots: Vec<Position> = state
        .bots
        .iter()
        .filter(|b| b.owner == my_id)
        .map(|b| b.position)
        .collect();
    let enemy_list: Vec<Position> = state
        .bots
        .iter()
        .filter(|b| b.owner != my_id)
        .map(|b| b.position)
        .collect();
    if my_bots.is_empty() {
        return vec![];
    }

    let walls: HashSet<Position> = state.walls.iter().copied().collect();
    let enemies: HashSet<Position> = enemy_list.iter().copied().collect();

    // Circular-mean centroid, smoothed with last turn's value (70% new).
    let mut centroid = circular_mean(&my_bots, rows, cols);
    if let Some(prev) = persistent.centroid {
        centroid = smooth_centroid(prev, centroid, rows, cols);
    }
    persistent.centroid = Some(centroid);
    drop(guard);

    let mean_dist = mean_distance2_from(&my_bots, centroid, rows, cols);
    let rallying = mean_dist > PHX_FORMATION_RADIUS2;

    let mut advance_target = centroid;
    if !rallying {
        if !enemy_list.is_empty() {
            advance_target = circular_mean(&enemy_list, rows, cols);
        } else {
            advance_target = Position {
                row: rows / 2,
                col: cols / 2,
            };
        }
    }

    let slots = formation_slots(centroid, my_bots.len(), rows, cols);
    let assignments = assign_slots(&my_bots, &slots, rows, cols);

    let mut claimed: HashSet<Position> = HashSet::new();
    let mut moves = Vec::with_capacity(my_bots.len());
    for pos in &my_bots {
        let slot = assignments.get(pos).copied();
        if let Some(dir) = scored_dir(
            *pos,
            slot,
            advance_target,
            centroid,
            &enemies,
            &walls,
            &claimed,
            rallying,
            state.config.attack_radius2,
            rows,
            cols,
        ) {
            let dest = pos.move_toward(dir, rows, cols);
            claimed.insert(dest);
            moves.push(Move {
                position: *pos,
                direction: dir,
            });
        } else {
            claimed.insert(*pos);
        }
    }
    moves
}

/// Scores each candidate step by slot cohesion, centroid proximity, advance
/// toward the target, and attack-range presence.
#[allow(clippy::too_many_arguments)]
fn scored_dir(
    pos: Position,
    slot: Option<Position>,
    advance_target: Position,
    centroid: Position,
    enemies: &HashSet<Position>,
    walls: &HashSet<Position>,
    claimed: &HashSet<Position>,
    rallying: bool,
    attack_radius2: i32,
    rows: i32,
    cols: i32,
) -> Option<Direction> {
    let mut best_dir = None;
    let mut best_score = f64::NEG_INFINITY;
    for dir in Direction::all() {
        let step = pos.move_toward(dir, rows, cols);
        if walls.contains(&step) || enemies.contains(&step) || claimed.contains(&step) {
            continue;
        }
        let mut score = 0.0;

        if let Some(slot) = slot {
            score += (pos.distance2(slot, rows, cols) as f64
                - step.distance2(slot, rows, cols) as f64)
                * PHX_FORMATION_WEIGHT;
        }
        score += (pos.distance2(centroid, rows, cols) as f64
            - step.distance2(centroid, rows, cols) as f64)
            * (PHX_FORMATION_WEIGHT * 0.3);

        let mut advance = PHX_ADVANCE_WEIGHT;
        if rallying {
            advance *= 2.0;
        }
        score += (pos.distance2(advance_target, rows, cols) as f64
            - step.distance2(advance_target, rows, cols) as f64)
            * advance;

        if !rallying {
            for ep in enemies {
                if step.distance2(*ep, rows, cols) <= attack_radius2 {
                    score += PHX_ATTACK_RANGE_BONUS;
                }
            }
        }

        if best_dir.is_none() || score > best_score {
            best_score = score;
            best_dir = Some(dir);
        }
    }
    best_dir
}

/// Toroidally-correct center of mass (circular mean).
fn circular_mean(positions: &[Position], rows: i32, cols: i32) -> Position {
    if positions.is_empty() {
        return Position {
            row: rows / 2,
            col: cols / 2,
        };
    }
    let row_scale = 2.0 * std::f64::consts::PI / rows as f64;
    let col_scale = 2.0 * std::f64::consts::PI / cols as f64;
    let n = positions.len() as f64;

    let (mut sin_r, mut cos_r, mut sin_c, mut cos_c) = (0.0f64, 0.0f64, 0.0f64, 0.0f64);
    for p in positions {
        sin_r += (p.row as f64 * row_scale).sin();
        cos_r += (p.row as f64 * row_scale).cos();
        sin_c += (p.col as f64 * col_scale).sin();
        cos_c += (p.col as f64 * col_scale).cos();
    }

    let avg_row = (sin_r / n).atan2(cos_r / n) / row_scale;
    let avg_col = (sin_c / n).atan2(cos_c / n) / col_scale;
    Position {
        row: wrap_f64(avg_row, rows),
        col: wrap_f64(avg_col, cols),
    }
}

fn wrap_f64(v: f64, n: i32) -> i32 {
    let n = n as f64;
    let wrapped = (v % n + n) % n;
    wrapped.round() as i32
}

/// Blends 70% of the delta into the previous centroid.
fn smooth_centroid(prev: Position, current: Position, rows: i32, cols: i32) -> Position {
    let dr = toroidal_delta(prev.row, current.row, rows);
    let dc = toroidal_delta(prev.col, current.col, cols);
    Position {
        row: wrap_int((prev.row as f64 + 0.7 * dr as f64).round() as i32, rows),
        col: wrap_int((prev.col as f64 + 0.7 * dc as f64).round() as i32, cols),
    }
}

/// Signed shortest delta from a to b on a ring of size n (|delta| <= n/2).
fn toroidal_delta(a: i32, b: i32, n: i32) -> i32 {
    let mut d = b - a;
    if d > n / 2 {
        d -= n;
    } else if d < -n / 2 {
        d += n;
    }
    d
}

fn wrap_int(v: i32, n: i32) -> i32 {
    ((v % n) + n) % n
}

fn mean_distance2_from(positions: &[Position], center: Position, rows: i32, cols: i32) -> f64 {
    if positions.is_empty() {
        return 0.0;
    }
    let total: i32 = positions
        .iter()
        .map(|p| p.distance2(center, rows, cols))
        .sum();
    total as f64 / positions.len() as f64
}

/// Lays out hex-ring packing slots around the centroid.
fn formation_slots(centroid: Position, count: usize, rows: i32, cols: i32) -> Vec<Position> {
    if count == 0 {
        return vec![];
    }
    let mut slots = vec![centroid];
    let mut ring = 1;
    while slots.len() < count && ring <= 20 {
        for d in hex_ring(ring) {
            if slots.len() >= count {
                break;
            }
            slots.push(Position {
                row: wrap_int(centroid.row + d.0, rows),
                col: wrap_int(centroid.col + d.1, cols),
            });
        }
        ring += 1;
    }
    slots
}

/// Generates the 6*ring offsets of a hex ring in offset coordinates
/// (axial hex -> offset_col = q + r/2).
fn hex_ring(ring: i32) -> Vec<(i32, i32)> {
    if ring == 0 {
        return vec![(0, 0)];
    }
    let hex_dirs = [(1, 0), (0, 1), (-1, 1), (-1, 0), (0, -1), (1, -1)];
    let mut result = Vec::with_capacity((6 * ring) as usize);
    let (mut q, mut r) = (ring, 0);
    for d in hex_dirs {
        for _ in 0..ring {
            result.push((r, q + r / 2));
            q += d.0;
            r += d.1;
        }
    }
    result
}

/// Greedily assigns each bot its nearest unused slot.
fn assign_slots(bots: &[Position], slots: &[Position], rows: i32, cols: i32) -> HashMap<Position, Position> {
    let mut assignments = HashMap::with_capacity(bots.len());
    let mut used = vec![false; slots.len()];
    for bot in bots {
        let mut best_slot = usize::MAX;
        let mut best_dist = i32::MAX;
        for (si, slot) in slots.iter().enumerate() {
            if used[si] {
                continue;
            }
            let d = bot.distance2(*slot, rows, cols);
            if d < best_dist {
                best_dist = d;
                best_slot = si;
            }
        }
        if best_slot < slots.len() {
            used[best_slot] = true;
            assignments.insert(*bot, slots[best_slot]);
        }
    }
    assignments
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
    // Per-match state resets on the first turn of a new match; nothing to do.
    let _ = read_input(ptr, len);
}

#[no_mangle]
pub extern "C" fn compute_moves(ptr: *const u8, len: usize) -> *const u8 {
    let input = read_input(ptr, len);
    let state: GameState = match serde_json::from_str(&input) {
        Ok(s) => s,
        Err(_) => return return_result(b"[]"),
    };
    let moves = phalanx_moves(&state);
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
