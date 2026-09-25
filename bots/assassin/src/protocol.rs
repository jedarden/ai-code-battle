//! Strict request-schema validation for docs/bot-protocol.md.
//!
//! Runs after signature verification: a decoding failure here is
//! authenticated malformed input (400), never an authentication failure.
//! Unknown fields are rejected everywhere the contract closes an object:
//! the top level, `config`, `you`, `zone`, `zone.center`, and every array
//! element — mirroring the engine's `VisibleState` wire shapes exactly.

use serde::Deserialize;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictPosition {
    row: i64,
    col: i64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictBotElement {
    position: StrictPosition,
    owner: i64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictCoreElement {
    position: StrictPosition,
    owner: i64,
    active: bool,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictConfig {
    rows: i64,
    cols: i64,
    max_turns: i64,
    vision_radius2: i64,
    attack_radius2: i64,
    spawn_cost: i64,
    energy_interval: i64,
    cores_per_player: i64,
    #[serde(default)]
    map_id: String,
    #[serde(default)]
    season_id: String,
    #[serde(default)]
    rules_version: String,
    #[serde(default)]
    turn_timeout: i64,
    zone_enabled: bool,
    zone_start_turn: i64,
    zone_shrink_interval: i64,
    zone_shrink_step: i64,
    zone_min_radius: i64,
    kill_score: i64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictYou {
    id: i64,
    energy: i64,
    score: i64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictZone {
    center: StrictPosition,
    radius: i64,
    active: bool,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct StrictRequest {
    match_id: String,
    turn: i64,
    config: StrictConfig,
    you: StrictYou,
    bots: Vec<StrictBotElement>,
    energy: Vec<StrictPosition>,
    cores: Vec<StrictCoreElement>,
    walls: Vec<StrictPosition>,
    dead: Vec<StrictBotElement>,
    #[serde(default)]
    zone: Option<StrictZone>,
}

/// Validates the raw body against the documented request schema. Returns the
/// parsed match id and turn on success so the caller can compare them with
/// the authenticated headers.
pub fn validate_request_schema(body: &[u8]) -> Result<(String, i64), ()> {
    let request: StrictRequest = serde_json::from_slice(body).map_err(|_| ())?;
    Ok((request.match_id, request.turn))
}
