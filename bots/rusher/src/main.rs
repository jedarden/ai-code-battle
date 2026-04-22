//! RusherBot - A bot that rushes enemy cores aggressively.
//!
//! Strategy: Identify and rush the nearest enemy core as fast as possible.
//! Uses BFS pathfinding to navigate toward cores while ignoring energy
//! and enemy bots (unless they block the path).

mod game;
mod strategy;

use axum::{
    extract::State,
    http::{HeaderMap, HeaderValue, StatusCode},
    response::IntoResponse,
    routing::{get, post},
    Json, Router,
};
use game::{GameState, Move, MoveResponse};
use hmac::{Hmac, Mac};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::env;
use std::sync::Arc;
use strategy::RusherStrategy;
use tokio::sync::Mutex;
use tracing::{info, Level};
use tracing_subscriber::FmtSubscriber;

type HmacSha256 = Hmac<Sha256>;

/// Bot server state
struct BotState {
    secret: String,
    strategy: RusherStrategy,
}

#[tokio::main]
async fn main() {
    // Initialize logging
    let subscriber = FmtSubscriber::builder()
        .with_max_level(Level::INFO)
        .finish();
    tracing::subscriber::set_global_default(subscriber).expect("Failed to set subscriber");

    let port = env::var("BOT_PORT").unwrap_or_else(|_| "8082".to_string());
    let secret = env::var("BOT_SECRET").expect("BOT_SECRET environment variable is required");

    let state = Arc::new(Mutex::new(BotState {
        secret,
        strategy: RusherStrategy::new(),
    }));

    let app = Router::new()
        .route("/turn", post(handle_turn))
        .route("/health", get(handle_health))
        .with_state(state);

    let addr = format!("0.0.0.0:{}", port);
    info!("RusherBot starting on {}", addr);

    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}

/// Handle turn requests from the game engine
async fn handle_turn(
    State(state): State<Arc<Mutex<BotState>>>,
    headers: HeaderMap,
    body: String,
) -> Result<impl IntoResponse, StatusCode> {
    // Extract auth headers
    let match_id = headers
        .get("X-ACB-Match-Id")
        .and_then(|v| v.to_str().ok())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let turn_str = headers
        .get("X-ACB-Turn")
        .and_then(|v| v.to_str().ok())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let timestamp = headers
        .get("X-ACB-Timestamp")
        .and_then(|v| v.to_str().ok())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let signature = headers
        .get("X-ACB-Signature")
        .and_then(|v| v.to_str().ok())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    // Verify signature
    let mut state = state.lock().await;
    if !verify_signature(&state.secret, match_id, turn_str, timestamp, &body, signature) {
        return Err(StatusCode::UNAUTHORIZED);
    }

    // Parse game state
    let game_state: GameState = serde_json::from_str(&body).map_err(|_| StatusCode::BAD_REQUEST)?;

    // Compute moves
    let moves = state.strategy.compute_moves(&game_state);
    let turn: u32 = turn_str.parse().unwrap_or(0);

    info!("Turn {}: {} moves computed", turn, moves.len());

    // Build response
    let response = MoveResponse { moves };

    // Sign response
    let response_body = serde_json::to_string(&response).unwrap();
    let response_sig = sign_response(&state.secret, match_id, turn, &response_body);

    let mut resp_headers = HeaderMap::new();
    resp_headers.insert("X-ACB-Signature", HeaderValue::from_str(&response_sig).unwrap());

    Ok((resp_headers, Json(response)))
}

/// Handle health check requests
async fn handle_health() -> &'static str {
    "OK"
}

/// Verify HMAC signature of incoming request
fn verify_signature(
    secret: &str,
    match_id: &str,
    turn: &str,
    timestamp: &str,
    body: &str,
    signature: &str,
) -> bool {
    let body_hash = sha2::Sha256::digest(body.as_bytes());
    let body_hash_hex = hex::encode(body_hash);

    let signing_string = format!("{}.{}.{}.{}", match_id, turn, timestamp, body_hash_hex);

    let mut mac = match HmacSha256::new_from_slice(secret.as_bytes()) {
        Ok(m) => m,
        Err(_) => return false,
    };
    mac.update(signing_string.as_bytes());
    let expected = hex::encode(mac.finalize().into_bytes());

    hmac_equal(signature, &expected)
}

/// Sign response body
fn sign_response(secret: &str, match_id: &str, turn: u32, body: &str) -> String {
    let body_hash = sha2::Sha256::digest(body.as_bytes());
    let body_hash_hex = hex::encode(body_hash);

    let signing_string = format!("{}.{}.{}", match_id, turn, body_hash_hex);

    let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(signing_string.as_bytes());
    hex::encode(mac.finalize().into_bytes())
}

/// Constant-time string comparison
fn hmac_equal(a: &str, b: &str) -> bool {
    if a.len() != b.len() {
        return false;
    }
    a.as_bytes()
        .iter()
        .zip(b.as_bytes().iter())
        .fold(0, |acc, (x, y)| acc | (x ^ y))
        == 0
}
