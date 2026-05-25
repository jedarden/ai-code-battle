// RusherBot WASM implementation - aggressive core-rushing strategy.
// Uses low-level WASM interface compatible with the sandbox loader.

#![no_std]
#![allow(non_snake_case)]

use core::panic::PanicInfo;

extern crate alloc;

use alloc::vec::Vec;
use alloc::string::String;
use alloc::string::ToString;
use alloc::boxed::Box;

// Import memory from the runtime (provided by sandbox)
extern "C" {
    // These are provided by the WebAssembly environment
    static mut __heap_base: usize;
}

// Simple bump allocator
const HEAP_SIZE: usize = 1024 * 1024; // 1MB heap
static mut HEAP_PTR: usize = 0;
static mut HEAP_END: usize = 0;

fn heap_init() {
    unsafe {
        if HEAP_PTR == 0 {
            HEAP_PTR = align_up(&mut __heap_base as *mut _ as usize, 8);
            HEAP_END = HEAP_PTR + HEAP_SIZE;
        }
    }
}

fn align_up(addr: usize, align: usize) -> usize {
    (addr + align - 1) & !(align - 1)
}

// Simple allocator for alloc crate
struct BumpAllocator;

unsafe impl core::alloc::GlobalAlloc for BumpAllocator {
    unsafe fn alloc(&self, layout: core::alloc::Layout) -> *mut u8 {
        heap_init();
        let size = layout.size();
        let align = layout.align();

        let mut ptr = align_up(HEAP_PTR, align);

        if ptr + size > HEAP_END {
            return core::ptr::null_mut();
        }

        HEAP_PTR = ptr + size;
        ptr as *mut u8
    }

    unsafe fn dealloc(&self, _ptr: *mut u8, _layout: core::alloc::Layout) {
        // Bump allocator doesn't free (memory is reclaimed after each compute_moves call)
    }
}

#[global_allocator]
static ALLOCATOR: BumpAllocator = BumpAllocator;

#[panic_handler]
fn panic(_info: &PanicInfo) -> ! {
    loop {}
}

// Reset the allocator between calls
fn reset_allocator() {
    unsafe {
        heap_init();
        HEAP_PTR = align_up(&mut __heap_base as *mut _ as usize, 8);
    }
}

// Game structures
#[derive(Clone, Copy, Default, PartialEq, Eq)]
struct Position {
    row: i32,
    col: i32,
}

#[derive(Clone, Default)]
struct Config {
    rows: i32,
    cols: i32,
    attack_radius2: i32,
    max_turns: i32,
}

#[derive(Clone, Default)]
struct PlayerInfo {
    id: i32,
    energy: i32,
    score: i32,
}

#[derive(Clone, Default)]
struct VisibleBot {
    position: Position,
    owner: i32,
}

#[derive(Clone, Default)]
struct VisibleCore {
    position: Position,
    owner: i32,
    active: bool,
}

#[derive(Default)]
struct VisibleState {
    you: PlayerInfo,
    bots: Vec<VisibleBot>,
    cores: Vec<VisibleCore>,
    energy: Vec<Position>,
    walls: Vec<Position>,
    dead: Vec<VisibleBot>,
}

#[derive(Clone, Copy)]
struct Move {
    position: Position,
    direction: Direction,
}

#[derive(Clone, Copy, PartialEq)]
enum Direction {
    N,
    E,
    S,
    W,
    None,
}

// Global state
static mut CONFIG: Option<Config> = None;
static mut KNOWN_ENEMY_CORES: Vec<Position> = Vec::new();

// Simple JSON parsing
struct JsonParser<'a> {
    input: &'a str,
    pos: usize,
}

impl<'a> JsonParser<'a> {
    fn new(input: &'a str) -> Self {
        Self { input, pos: 0 }
    }

    fn skip_whitespace(&mut self) {
        while self.pos < self.input.len() {
            let c = self.input.as_bytes()[self.pos];
            if c == b' ' || c == b'\n' || c == b'\r' || c == b'\t' {
                self.pos += 1;
            } else {
                break;
            }
        }
    }

    fn parse_config(&mut self) -> Config {
        let mut config = Config::default();
        self.skip_whitespace();
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'{' {
            return config;
        }
        self.pos += 1;

        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b'}' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'"' {
                let key = self.parse_string();
                self.skip_whitespace();
                if self.pos < self.input.len() && self.input.as_bytes()[self.pos] == b':' {
                    self.pos += 1;
                    self.skip_whitespace();
                    let value = self.parse_number();
                    match key.as_str() {
                        "rows" => config.rows = value,
                        "cols" => config.cols = value,
                        "attack_radius2" => config.attack_radius2 = value,
                        "max_turns" => config.max_turns = value,
                        _ => {}
                    }
                }
            }
        }
        config
    }

    fn parse_state(&mut self) -> VisibleState {
        let mut state = VisibleState::default();
        self.skip_whitespace();
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'{' {
            return state;
        }
        self.pos += 1;

        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b'}' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'"' {
                let key = self.parse_string();
                self.skip_whitespace();
                if self.pos < self.input.len() && self.input.as_bytes()[self.pos] == b':' {
                    self.pos += 1;
                    self.skip_whitespace();
                    match key.as_str() {
                        "you" => state.you = self.parse_you(),
                        "bots" => state.bots = self.parse_bots(),
                        "cores" => state.cores = self.parse_cores(),
                        "walls" => state.walls = self.parse_positions(),
                        _ => {
                            self.skip_value();
                        }
                    }
                }
            }
        }
        state
    }

    fn parse_you(&mut self) -> PlayerInfo {
        let mut info = PlayerInfo::default();
        self.skip_whitespace();
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'{' {
            return info;
        }
        self.pos += 1;
        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b'}' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'"' {
                let key = self.parse_string();
                self.skip_whitespace();
                if self.pos < self.input.len() && self.input.as_bytes()[self.pos] == b':' {
                    self.pos += 1;
                    self.skip_whitespace();
                    let value = self.parse_number();
                    match key.as_str() {
                        "id" => info.id = value,
                        "energy" => info.energy = value,
                        "score" => info.score = value,
                        _ => {}
                    }
                }
            }
        }
        info
    }

    fn parse_bots(&mut self) -> Vec<VisibleBot> {
        let mut bots = Vec::new();
        self.skip_whitespace();
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'[' {
            return bots;
        }
        self.pos += 1;
        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b']' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'{' {
                bots.push(self.parse_bot());
            }
        }
        bots
    }

    fn parse_bot(&mut self) -> VisibleBot {
        let mut bot = VisibleBot::default();
        self.pos += 1;
        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b'}' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'"' {
                let key = self.parse_string();
                self.skip_whitespace();
                if self.pos < self.input.len() && self.input.as_bytes()[self.pos] == b':' {
                    self.pos += 1;
                    self.skip_whitespace();
                    if key == "position" {
                        bot.position = self.parse_position();
                    } else if key == "owner" {
                        bot.owner = self.parse_number();
                    }
                }
            }
        }
        bot
    }

    fn parse_cores(&mut self) -> Vec<VisibleCore> {
        let mut cores = Vec::new();
        self.skip_whitespace();
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'[' {
            return cores;
        }
        self.pos += 1;
        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b']' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'{' {
                cores.push(self.parse_core());
            }
        }
        cores
    }

    fn parse_core(&mut self) -> VisibleCore {
        let mut core = VisibleCore::default();
        self.pos += 1;
        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b'}' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'"' {
                let key = self.parse_string();
                self.skip_whitespace();
                if self.pos < self.input.len() && self.input.as_bytes()[self.pos] == b':' {
                    self.pos += 1;
                    self.skip_whitespace();
                    if key == "position" {
                        core.position = self.parse_position();
                    } else if key == "owner" {
                        core.owner = self.parse_number();
                    } else if key == "active" {
                        let next = self.input.as_bytes()[self.pos];
                        core.active = if next == b't' {
                            self.pos += 4;
                            true
                        } else {
                            self.pos += 5;
                            false
                        };
                    }
                }
            }
        }
        core
    }

    fn parse_positions(&mut self) -> Vec<Position> {
        let mut positions = Vec::new();
        self.skip_whitespace();
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'[' {
            return positions;
        }
        self.pos += 1;
        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b']' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'{' {
                positions.push(self.parse_position());
            }
        }
        positions
    }

    fn parse_position(&mut self) -> Position {
        let mut pos = Position::default();
        self.skip_whitespace();
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'{' {
            return pos;
        }
        self.pos += 1;
        loop {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let c = self.input.as_bytes()[self.pos];
            if c == b'}' {
                self.pos += 1;
                break;
            }
            if c == b',' {
                self.pos += 1;
                continue;
            }
            if c == b'"' {
                let key = self.parse_string();
                self.skip_whitespace();
                if self.pos < self.input.len() && self.input.as_bytes()[self.pos] == b':' {
                    self.pos += 1;
                    self.skip_whitespace();
                    let value = self.parse_number();
                    if key == "row" {
                        pos.row = value;
                    } else if key == "col" {
                        pos.col = value;
                    }
                }
            }
        }
        pos
    }

    fn parse_string(&mut self) -> String {
        if self.pos >= self.input.len() || self.input.as_bytes()[self.pos] != b'"' {
            return String::new();
        }
        self.pos += 1;
        let start = self.pos;
        while self.pos < self.input.len() {
            let c = self.input.as_bytes()[self.pos];
            if c == b'"' {
                let result = &self.input[start..self.pos];
                self.pos += 1;
                return result.to_string();
            }
            if c == b'\\' {
                self.pos += 2;
            } else {
                self.pos += 1;
            }
        }
        String::new()
    }

    fn parse_number(&mut self) -> i32 {
        self.skip_whitespace();
        let start = self.pos;
        if self.pos < self.input.len() && self.input.as_bytes()[self.pos] == b'-' {
            self.pos += 1;
        }
        while self.pos < self.input.len() {
            let c = self.input.as_bytes()[self.pos];
            if c >= b'0' && c <= b'9' {
                self.pos += 1;
            } else {
                break;
            }
        }
        let s = &self.input[start..self.pos];
        s.parse().unwrap_or(0)
    }

    fn skip_value(&mut self) {
        self.skip_whitespace();
        if self.pos >= self.input.len() {
            return;
        }
        let c = self.input.as_bytes()[self.pos];
        match c {
            b'"' => {
                self.parse_string();
            }
            b'{' => {
                self.pos += 1;
                let mut depth = 1;
                while self.pos < self.input.len() && depth > 0 {
                    match self.input.as_bytes()[self.pos] {
                        b'{' => depth += 1,
                        b'}' => depth -= 1,
                        b'"' => {
                            self.parse_string();
                            continue;
                        }
                        _ => {}
                    }
                    self.pos += 1;
                }
            }
            b'[' => {
                self.pos += 1;
                let mut depth = 1;
                while self.pos < self.input.len() && depth > 0 {
                    match self.input.as_bytes()[self.pos] {
                        b'[' => depth += 1,
                        b']' => depth -= 1,
                        b'"' => {
                            self.parse_string();
                            continue;
                        }
                        _ => {}
                    }
                    self.pos += 1;
                }
            }
            _ => {
                while self.pos < self.input.len() {
                    let c = self.input.as_bytes()[self.pos];
                    if c == b',' || c == b'}' || c == b']' {
                        break;
                    }
                    self.pos += 1;
                }
            }
        }
    }
}

// Strategy functions
fn get_rush_targets(state: &VisibleState, my_id: i32) -> Vec<Position> {
    let mut targets: Vec<Position> = state
        .cores
        .iter()
        .filter(|c| c.owner != my_id && c.active)
        .map(|c| c.position)
        .collect();

    unsafe {
        for pos in &KNOWN_ENEMY_CORES {
            if !targets.contains(pos) {
                targets.push(*pos);
            }
        }
    }

    targets
}

fn apply_dir(pos: Position, dir: Direction, rows: i32, cols: i32) -> Position {
    let (dr, dc) = match dir {
        Direction::N => (-1, 0),
        Direction::E => (0, 1),
        Direction::S => (1, 0),
        Direction::W => (0, -1),
        Direction::None => (0, 0),
    };

    Position {
        row: ((pos.row + dr) % rows + rows) % rows,
        col: ((pos.col + dc) % cols + cols) % cols,
    }
}

fn dist2(a: Position, b: Position, config: &Config) -> i32 {
    let mut dr = (a.row - b.row).abs();
    let mut dc = (a.col - b.col).abs();

    if dr > config.rows / 2 {
        dr = config.rows - dr;
    }
    if dc > config.cols / 2 {
        dc = config.cols - dc;
    }

    dr * dr + dc * dc
}

fn find_best_move(
    start: Position,
    targets: &[Position],
    enemy_positions: &[Position],
    walls: &[Position],
    config: &Config,
) -> Option<Direction> {
    if targets.is_empty() {
        return None;
    }

    let rows = config.rows;
    let cols = config.cols;

    let mut best_dir = Direction::None;
    let mut best_dist = i32::MAX;

    for &dir in &[Direction::N, Direction::E, Direction::S, Direction::W] {
        let next = apply_dir(start, dir, rows, cols);

        if walls.contains(&next) || enemy_positions.contains(&next) {
            continue;
        }

        let mut min_target_dist = i32::MAX;
        for target in targets {
            let d = dist2(next, *target, config);
            if d < min_target_dist {
                min_target_dist = d;
            }
        }

        if min_target_dist < best_dist {
            best_dist = min_target_dist;
            best_dir = dir;
        }
    }

    if best_dir != Direction::None {
        Some(best_dir)
    } else {
        None
    }
}

fn direction_to_string(dir: Direction) -> &'static str {
    match dir {
        Direction::N => "N",
        Direction::E => "E",
        Direction::S => "S",
        Direction::W => "W",
        Direction::None => "",
    }
}

// JSON output
fn write_moves_json(moves: &[Move], buf: &mut Vec<u8>) {
    buf.push(b'[');
    for (i, mv) in moves.iter().enumerate() {
        if i > 0 {
            buf.push(b',');
        }
        buf.push(b'{');
        buf.extend_from_slice(b"\"position\":{\"row\":");
        write_number(mv.position.row, buf);
        buf.push(b',');
        buf.extend_from_slice(b"\"col\":");
        write_number(mv.position.col, buf);
        buf.push(b'}');
        buf.push(b',');
        buf.extend_from_slice(b"\"direction\":\"");
        buf.extend_from_slice(direction_to_string(mv.direction).as_bytes());
        buf.push(b'"');
        buf.push(b'}');
    }
    buf.push(b']');
}

fn write_number(n: i32, buf: &mut Vec<u8>) {
    let mut n = n;
    if n < 0 {
        buf.push(b'-');
        n = -n;
    }
    let mut digits = [0u8; 11];
    let mut i = 10;
    if n == 0 {
        buf.push(b'0');
        return;
    }
    while n > 0 {
        digits[i] = b'0' + (n % 10) as u8;
        n /= 10;
        i -= 1;
    }
    buf.extend_from_slice(&digits[i + 1..]);
}

// ────────────────────────────────────────────────────────────────────────────────
// WASM exports
// ────────────────────────────────────────────────────────────────────────────────

#[no_mangle]
pub extern "C" fn allocate(size: usize) -> *mut u8 {
    unsafe {
        heap_init();
        let ptr = align_up(HEAP_PTR, 8);
        HEAP_PTR = ptr + size;
        if HEAP_PTR > HEAP_END {
            return core::ptr::null_mut();
        }
        ptr as *mut u8
    }
}

#[no_mangle]
pub extern "C" fn init(ptr: *const u8, len: usize) {
    reset_allocator();
    let bytes = unsafe { core::slice::from_raw_parts(ptr, len) };
    let input = core::str::from_utf8(bytes).unwrap_or("{}");

    let mut parser = JsonParser::new(input);
    let config = parser.parse_config();

    unsafe {
        CONFIG = Some(config);
        KNOWN_ENEMY_CORES = Vec::new();
    }
}

#[no_mangle]
pub extern "C" fn compute_moves(ptr: *const u8, len: usize) -> *const u8 {
    reset_allocator();
    let bytes = unsafe { core::slice::from_raw_parts(ptr, len) };
    let input = core::str::from_utf8(bytes).unwrap_or("{}");

    let mut parser = JsonParser::new(input);
    let state = parser.parse_state();

    let config = unsafe { CONFIG.as_ref().unwrap() };
    let my_id = state.you.id;

    // Update known enemy cores
    for core in &state.cores {
        if core.owner != my_id {
            unsafe {
                if !KNOWN_ENEMY_CORES.contains(&core.position) {
                    KNOWN_ENEMY_CORES.push(core.position);
                }
            }
        }
    }

    // Separate my bots from enemies
    let (my_bots, enemy_bots): (Vec<_>, Vec<_>) =
        state.bots.iter().partition(|b| b.owner == my_id);

    if my_bots.is_empty() {
        return return_result("[]");
    }

    let enemy_positions: Vec<Position> =
        enemy_bots.iter().map(|b| b.position).collect();
    let walls = state.walls.clone();
    let targets = get_rush_targets(&state, my_id);

    let mut moves = Vec::with_capacity(my_bots.len());

    for bot in &my_bots {
        if let Some(dir) = find_best_move(
            bot.position,
            &targets,
            &enemy_positions,
            &walls,
            config,
        ) {
            moves.push(Move {
                position: bot.position,
                direction: dir,
            });
        }
    }

    let mut output = Vec::new();
    write_moves_json(&moves, &mut output);

    return_result_bytes(&output)
}

#[no_mangle]
pub extern "C" fn free_result(_ptr: *const u8) {
    // Bump allocator - no free needed
}

fn return_result(s: &str) -> *const u8 {
    return_result_bytes(s.as_bytes())
}

fn return_result_bytes(bytes: &[u8]) -> *const u8 {
    unsafe {
        let ptr = allocate(bytes.len());
        if ptr.is_null() {
            return core::ptr::null();
        }
        core::ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, bytes.len());
        ptr
    }
}
