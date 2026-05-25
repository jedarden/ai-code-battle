use wasm_bindgen::prelude::*;
use serde::{Deserialize, Serialize};

#[wasm_bindgen]
pub struct RusherBot {
    config: Option<GameConfig>,
}

#[derive(Deserialize)]
struct GameConfig {
    rows: i32,
    cols: i32,
    attack_radius2: i32,
    #[serde(default)]
    max_turns: i32,
}

#[derive(Deserialize)]
struct VisibleState {
    #[serde(default)]
    you: PlayerInfo,
    #[serde(default)]
    bots: Vec<VisibleBot>,
    #[serde(default)]
    cores: Vec<VisibleCore>,
    #[serde(default)]
    energy: Vec<Position>,
}

#[derive(Deserialize)]
struct PlayerInfo {
    id: i32,
}

#[derive(Deserialize)]
struct VisibleBot {
    position: Position,
    owner: i32,
}

#[derive(Deserialize)]
struct VisibleCore {
    position: Position,
    owner: i32,
    active: bool,
}

#[derive(Deserialize, Serialize, Clone)]
struct Position {
    row: i32,
    col: i32,
}

#[derive(Serialize)]
struct Move {
    position: Position,
    direction: String,
}

const DIRS: &[&str] = &["N", "E", "S", "W"];

#[wasm_bindgen]
impl RusherBot {
    #[wasm_bindgen(constructor)]
    pub fn new() -> Self {
        Self { config: None }
    }

    #[wasm_bindgen]
    pub fn init(&mut self, config_json: &str) -> Result<String, JsError> {
        self.config = Some(serde_json::from_str(config_json)?);
        Ok(serde_json::to_string(&json!({"ok": true}))?)
    }

    #[wasm_bindgen]
    pub fn compute_moves(&self, state_json: &str) -> Result<String, JsError> {
        let state: VisibleState = serde_json::from_str(state_json)?;
        let config = self.config.as_ref().ok_or_else(|| JsError::new("not initialized"))?;

        let my_id = state.you.id;
        let mut moves = Vec::new();

        // Find enemy cores
        let mut enemy_cores: Vec<Position> = Vec::new();
        for core in &state.cores {
            if core.owner != my_id && core.active {
                enemy_cores.push(core.position.clone());
            }
        }

        // Find enemy bots
        let mut enemy_bots: Vec<Position> = Vec::new();
        for bot in &state.bots {
            if bot.owner != my_id {
                enemy_bots.push(bot.position.clone());
            }
        }

        for bot in &state.bots {
            if bot.owner != my_id {
                continue;
            }

            let dir = if !enemy_cores.is_empty() {
                self.toward_nearest(&bot.position, &enemy_cores, config)
            } else {
                self.toward_nearest(&bot.position, &enemy_bots, config)
            };

            moves.push(Move {
                position: bot.position.clone(),
                direction: dir,
            });
        }

        Ok(serde_json::to_string(&moves)?)
    }

    #[wasm_bindgen]
    pub fn free_result(&self, _ptr: usize) {
        // No-op for Rust (Wasm-bindgen handles memory)
    }
}

impl RusherBot {
    fn toward_nearest(&self, from: &Position, targets: &[Position], config: &GameConfig) -> String {
        if targets.is_empty() {
            return DIRS[fastrand::usize(0..4)].to_string();
        }

        let mut best_dir = DIRS[0];
        let mut best_dist = i32::MAX;

        for &dir in DIRS {
            let np = self.apply_dir(from, dir, config);
            for target in targets {
                let dist = self.dist2(&np, target, config);
                if dist < best_dist {
                    best_dist = dist;
                    best_dir = dir;
                }
            }
        }

        best_dir.to_string()
    }

    fn apply_dir(&self, pos: &Position, dir: &str, config: &GameConfig) -> Position {
        let (dr, dc) = match dir {
            "N" => (-1, 0),
            "E" => (0, 1),
            "S" => (1, 0),
            "W" => (0, -1),
            _ => (0, 0),
        };

        Position {
            row: ((pos.row + dr) % config.rows + config.rows) % config.rows,
            col: ((pos.col + dc) % config.cols + config.cols) % config.cols,
        }
    }

    fn dist2(&self, a: &Position, b: &Position, config: &GameConfig) -> i32 {
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
}
