//! AI Code Battle - Rust Starter Kit
//!
//! A minimal bot scaffold with HMAC authentication and a placeholder
//! random strategy. Replace `compute_moves()` with your own logic.

mod grid;

use axum::{
    body::Bytes,
    extract::State,
    http::{HeaderMap, StatusCode},
    routing::{get, post},
    Json, Router,
};
use hmac::{Hmac, Mac};
use serde::{Deserialize, Serialize};
use sha2::Sha256;
use std::env;

type HmacSha256 = Hmac<Sha256>;

// Engine constants
const DIRECTIONS: [&str; 4] = ["N", "E", "S", "W"];

#[derive(Deserialize)]
struct GameState {
    match_id: String,
    turn: u32,
    config: GameConfig,
    you: You,
    bots: Vec<VisibleBot>,
    energy: Vec<Position>,
    cores: Vec<VisibleCore>,
    walls: Vec<Position>,
    dead: Vec<VisibleBot>,
}

#[derive(Deserialize)]
struct GameConfig {
    rows: u32,
    cols: u32,
    max_turns: u32,
    vision_radius2: u32,
    attack_radius2: u32,
    spawn_cost: u32,
    energy_interval: u32,
}

#[derive(Deserialize)]
struct You {
    id: u32,
    energy: u32,
    score: u32,
}

#[derive(Deserialize, Serialize, Clone)]
pub struct Position {
    pub row: u32,
    pub col: u32,
}

#[derive(Deserialize)]
struct VisibleBot {
    position: Position,
    owner: u32,
}

#[derive(Deserialize)]
struct VisibleCore {
    position: Position,
    owner: u32,
    active: bool,
}

#[derive(Serialize)]
struct MoveResponse {
    moves: Vec<Move>,
}

#[derive(Serialize)]
struct Move {
    position: Position,
    direction: String,
}

struct AppState {
    secret: String,
}

#[tokio::main]
async fn main() {
    let port = env::var("BOT_PORT").unwrap_or_else(|_| "8080".into());
    let secret = env::var("BOT_SECRET").expect("BOT_SECRET is required");

    let state = AppState { secret };
    let app = Router::new()
        .route("/turn", post(handle_turn))
        .route("/health", get(handle_health))
        .with_state(state);

    let addr = format!("0.0.0.0:{}", port);
    println!("Bot listening on {}", addr);

    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}

async fn handle_health() -> &'static str {
    "OK"
}

async fn handle_turn(
    State(state): State<AppState>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<(StatusCode, [(&str, String); 2], String), StatusCode> {
    let signature = headers
        .get("X-ACB-Signature")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    let match_id = headers
        .get("X-ACB-Match-Id")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    let turn_str = headers
        .get("X-ACB-Turn")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("0");
    let timestamp = headers
        .get("X-ACB-Timestamp")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");

    if signature.is_empty()
        || !verify_signature(
            &state.secret,
            match_id,
            turn_str,
            timestamp,
            &body,
            signature,
        )
    {
        return Err(StatusCode::UNAUTHORIZED);
    }

    let game_state: GameState =
        serde_json::from_slice(&body).map_err(|_| StatusCode::BAD_REQUEST)?;

    let moves = compute_moves(&game_state);
    let response = MoveResponse { moves };
    let response_body = serde_json::to_string(&response).unwrap();

    let turn: u32 = turn_str.parse().unwrap_or(0);
    let response_sig = sign_response(&state.secret, match_id, turn, response_body.as_bytes());

    Ok((
        StatusCode::OK,
        [
            ("Content-Type".to_string(), "application/json".to_string()),
            ("X-ACB-Signature".to_string(), response_sig),
        ],
        response_body,
    ))
}

fn compute_moves(state: &GameState) -> Vec<Move> {
    // Replace this with your strategy!
    let rows = state.config.rows;
    let cols = state.config.cols;
    let mut moves = Vec::new();
    let mut rng = rand::thread_rng();

    let cardinal: [(i32, i32, &str); 4] = [
        (-1, 0, "N"),
        (0, 1, "E"),
        (1, 0, "S"),
        (0, -1, "W"),
    ];

    for bot in &state.bots {
        if bot.owner != state.you.id {
            continue;
        }

        // Find direction toward nearest energy using toroidal distance
        if !state.energy.is_empty() {
            let mut best_dist = u32::MAX;
            let mut best_dir: Option<&str> = None;
            for (dr, dc, dir) in &cardinal {
                let nr = (bot.position.row as i32 + dr).rem_euclid(rows as i32) as u32;
                let nc = (bot.position.col as i32 + dc).rem_euclid(cols as i32) as u32;
                let step = Position { row: nr, col: nc };
                for e in &state.energy {
                    let d = grid::toroidal_manhattan(&step, e, rows, cols);
                    if d < best_dist {
                        best_dist = d;
                        best_dir = Some(dir);
                    }
                }
            }
            if let Some(dir) = best_dir {
                moves.push(Move {
                    position: bot.position.clone(),
                    direction: dir.to_string(),
                });
                continue;
            }
        }

        if rand::Rng::gen_ratio(&mut rng, 1, 2) {
            let dir = DIRECTIONS[rand::Rng::gen_range(&mut rng, 0..4)];
            moves.push(Move {
                position: bot.position.clone(),
                direction: dir.to_string(),
            });
        }
    }
    moves
}

fn verify_signature(
    secret: &str,
    match_id: &str,
    turn: &str,
    timestamp: &str,
    body: &[u8],
    signature: &str,
) -> bool {
    use hex::FromHex;
    let body_hash = sha2::Sha256::digest(body);
    let signing_string = format!(
        "{}.{}.{}.{}",
        match_id,
        turn,
        timestamp,
        hex::encode(body_hash)
    );

    let mut mac =
        HmacSha256::new_from_slice(secret.as_bytes()).expect("HMAC key error");
    mac.update(signing_string.as_bytes());
    let expected = mac.finalize().into_bytes();

    match Vec::from_hex(signature) {
        Ok(sig_bytes) => {
            let sig_truncated: &[u8] = &sig_bytes;
            hmac::digest::constant_time_eq(sig_truncated, &expected)
        }
        Err(_) => false,
    }
}

fn sign_response(secret: &str, match_id: &str, turn: u32, body: &[u8]) -> String {
    let body_hash = sha2::Sha256::digest(body);
    let signing_string = format!("{}.{}.{}", match_id, turn, hex::encode(body_hash));

    let mut mac =
        HmacSha256::new_from_slice(secret.as_bytes()).expect("HMAC key error");
    mac.update(signing_string.as_bytes());
    hex::encode(mac.finalize().into_bytes())
}
