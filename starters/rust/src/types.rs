use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct Position {
    pub row: i32,
    pub col: i32,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Direction {
    #[serde(rename = "stay")]
    None,
    #[serde(rename = "N")]
    N,
    #[serde(rename = "E")]
    E,
    #[serde(rename = "S")]
    S,
    #[serde(rename = "W")]
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
