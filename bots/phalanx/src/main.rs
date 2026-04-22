//! PhalanxBot - Tight formation archetype.
//!
//! All units move as a coordinated group, maximizing local firepower
//! at the cost of map coverage.

mod game;
mod strategy;

use axum::{
    extract::State,
    http::{HeaderMap, StatusCode},
    routing::{get, post},
    Json, Router,
};
use game::{GameState, MoveResponse};
use hmac::{Hmac, Mac};
use sha2::{Digest, Sha256};
use std::env;
use std::sync::Arc;
use strategy::PhalanxStrategy;
use tokio::sync::Mutex;
use tracing::{info, Level};
use tracing_subscriber::FmtSubscriber;

type HmacSha256 = Hmac<Sha256>;

struct BotState {
    secret: String,
    strategy: PhalanxStrategy,
}

#[tokio::main]
async fn main() {
    let subscriber = FmtSubscriber::builder()
        .with_max_level(Level::INFO)
        .finish();
    tracing::subscriber::set_global_default(subscriber).expect("Failed to set subscriber");

    let port = env::var("BOT_PORT").unwrap_or_else(|_| "8090".to_string());
    let secret = env::var("BOT_SECRET").expect("BOT_SECRET environment variable is required");

    let state = Arc::new(Mutex::new(BotState {
        secret,
        strategy: PhalanxStrategy::new(),
    }));

    let app = Router::new()
        .route("/turn", post(handle_turn))
        .route("/health", get(handle_health))
        .with_state(state);

    let addr = format!("0.0.0.0:{}", port);
    info!("PhalanxBot starting on {}", addr);

    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}

async fn handle_turn(
    State(state): State<Arc<Mutex<BotState>>>,
    headers: HeaderMap,
    body: String,
) -> Result<Json<MoveResponse>, StatusCode> {
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

    let mut state = state.lock().await;
    if !verify_signature(&state.secret, match_id, turn_str, timestamp, &body, signature) {
        return Err(StatusCode::UNAUTHORIZED);
    }

    let game_state: GameState =
        serde_json::from_str(&body).map_err(|_| StatusCode::BAD_REQUEST)?;

    let moves = state.strategy.compute_moves(&game_state);
    let turn: u32 = turn_str.parse().unwrap_or(0);

    info!("Turn {}: {} moves computed", turn, moves.len());

    let response = MoveResponse { moves };
    let response_body = serde_json::to_string(&response).unwrap();
    let _response_sig = sign_response(&state.secret, match_id, turn, &response_body);

    Ok(Json(response))
}

async fn handle_health() -> &'static str {
    "OK"
}

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

fn sign_response(secret: &str, match_id: &str, turn: u32, body: &str) -> String {
    let body_hash = sha2::Sha256::digest(body.as_bytes());
    let body_hash_hex = hex::encode(body_hash);

    let signing_string = format!("{}.{}.{}", match_id, turn, body_hash_hex);

    let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(signing_string.as_bytes());
    hex::encode(mac.finalize().into_bytes())
}

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
