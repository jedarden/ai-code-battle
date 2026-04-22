//! Game state types for AI Code Battle protocol.

use serde::{Deserialize, Serialize};

/// Position on the grid
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct Position {
    pub row: i32,
    pub col: i32,
}

/// Game configuration
#[derive(Debug, Clone, Deserialize)]
pub struct GameConfig {
    pub rows: u32,
    pub cols: u32,
    pub max_turns: u32,
    pub vision_radius2: u32,
    pub attack_radius2: u32,
    pub spawn_cost: u32,
    pub energy_interval: u32,
}

/// Player info
#[derive(Debug, Clone, Deserialize)]
pub struct PlayerInfo {
    pub id: u32,
    pub energy: u32,
    pub score: u32,
}

/// Visible bot
#[derive(Debug, Clone, Deserialize)]
pub struct VisibleBot {
    pub position: Position,
    pub owner: u32,
}

/// Visible core
#[derive(Debug, Clone, Deserialize)]
pub struct VisibleCore {
    pub position: Position,
    pub owner: u32,
    pub active: bool,
}

/// Fog-filtered game state visible to this bot
#[derive(Debug, Clone, Deserialize)]
pub struct GameState {
    pub match_id: String,
    pub turn: u32,
    pub config: GameConfig,
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
    pub dead: Vec<VisibleBot>,
}

/// Movement direction
#[derive(Debug, Clone, Copy, Serialize)]
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

/// A single move command
#[derive(Debug, Clone, Serialize)]
pub struct Move {
    pub position: Position,
    pub direction: Direction,
}

/// Response containing moves
#[derive(Debug, Clone, Serialize)]
pub struct MoveResponse {
    pub moves: Vec<Move>,
}

impl Direction {
    /// All directions in order: N, E, S, W
    pub fn all() -> [Direction; 4] {
        [Direction::N, Direction::E, Direction::S, Direction::W]
    }
}

impl Position {
    /// Move in a direction, wrapping around the toroidal grid
    pub fn move_toward(&self, dir: Direction, rows: i32, cols: i32) -> Position {
        match dir {
            Direction::N => Position {
                row: (self.row - 1 + rows) % rows,
                col: self.col,
            },
            Direction::E => Position {
                row: self.row,
                col: (self.col + 1) % cols,
            },
            Direction::S => Position {
                row: (self.row + 1) % rows,
                col: self.col,
            },
            Direction::W => Position {
                row: self.row,
                col: (self.col - 1 + cols) % cols,
            },
        }
    }

    /// Calculate squared distance with toroidal wrapping
    pub fn distance2(&self, other: &Position, rows: i32, cols: i32) -> u32 {
        let dr = (self.row - other.row).abs();
        let dc = (self.col - other.col).abs();
        let dr = dr.min(rows - dr);
        let dc = dc.min(cols - dc);
        (dr * dr + dc * dc) as u32
    }
}
