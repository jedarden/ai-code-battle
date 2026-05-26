use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct Position {
    pub row: i32,
    pub col: i32,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Direction {
    None,
    N,
    E,
    S,
    W,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct VisibleBot {
    pub position: Position,
    pub owner: i32,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct VisibleCore {
    pub position: Position,
    pub owner: i32,
    pub active: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct You {
    pub id: i32,
    pub energy: i32,
    pub score: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VisibleState {
    pub match_id: String,
    pub turn: i32,
    pub config: serde_json::Value,
    pub you: You,
    pub bots: Vec<VisibleBot>,
    pub energy: Vec<Position>,
    pub cores: Vec<VisibleCore>,
    pub walls: Vec<Position>,
    pub dead: Vec<VisibleBot>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Move {
    pub position: Position,
    pub direction: Direction,
}
