//! AssassinBot - Decapitation archetype. All units rush the enemy core.
//!
//! Ignores enemy units and economy; pushes straight for the enemy core.
//! No perimeter defense — commits fully.

mod game;
#[allow(dead_code)]
mod protocol;
mod strategy;

use axum::{
    body::Bytes,
    extract::State,
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::IntoResponse,
    routing::{get, post},
    Router,
};
use game::{GameState, MoveResponse};
use hmac::{Hmac, Mac};
use sha2::{Digest, Sha256};
use std::env;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};
use strategy::AssassinStrategy;
use tokio::sync::Mutex;
use tracing::{info, Level};
use tracing_subscriber::FmtSubscriber;

type HmacSha256 = Hmac<Sha256>;
const TIMESTAMP_TOLERANCE_SECONDS: i64 = 30;

struct BotState {
    secret: String,
    strategy: AssassinStrategy,
}

#[tokio::main]
async fn main() {
    let subscriber = FmtSubscriber::builder()
        .with_max_level(Level::INFO)
        .finish();
    tracing::subscriber::set_global_default(subscriber).expect("Failed to set subscriber");

    let port = env::var("BOT_PORT").unwrap_or_else(|_| "8082".to_string());
    let secret = env::var("BOT_SECRET").expect("BOT_SECRET environment variable is required");

    let state = Arc::new(Mutex::new(BotState {
        secret,
        strategy: AssassinStrategy::new(),
    }));

    let app = Router::new()
        .route("/turn", post(handle_turn))
        .route("/health", get(handle_health))
        .with_state(state);

    let addr = format!("0.0.0.0:{}", port);
    info!("AssassinBot starting on {}", addr);

    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}

async fn handle_turn(
    State(state): State<Arc<Mutex<BotState>>>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<impl IntoResponse, StatusCode> {
    // Content-Type is part of the transport contract (step 1 of the
    // documented verification order).
    if headers
        .get(header::CONTENT_TYPE)
        .and_then(|v| v.to_str().ok())
        != Some("application/json")
    {
        return Err(StatusCode::UNAUTHORIZED);
    }

    let match_id = headers
        .get("X-ACB-Match-Id")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let turn_str = headers
        .get("X-ACB-Turn")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let timestamp = headers
        .get("X-ACB-Timestamp")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let _bot_id = headers
        .get("X-ACB-Bot-Id")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let signature = headers
        .get("X-ACB-Signature")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let turn = turn_str
        .parse::<u32>()
        .ok()
        .filter(|turn| turn.to_string() == turn_str)
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let mut state = state.lock().await;
    if !verify_signature(
        &state.secret,
        match_id,
        turn_str,
        timestamp,
        &body,
        signature,
    ) {
        return Err(StatusCode::UNAUTHORIZED);
    }

    // Strict schema before identity: a missing, unknown, or mis-typed
    // field is authenticated malformed input (400); present but
    // contradictory values are an authentication failure (401). The
    // schema closes every object the contract names — including
    // zone.center and each bots/energy/cores/walls/dead element.
    let (body_match_id, body_turn) =
        protocol::validate_request_schema(&body).map_err(|_| StatusCode::BAD_REQUEST)?;
    if body_match_id != match_id || body_turn != i64::from(turn) {
        return Err(StatusCode::UNAUTHORIZED);
    }
    let game_state: GameState =
        serde_json::from_slice(&body).map_err(|_| StatusCode::BAD_REQUEST)?;

    let moves = state.strategy.compute_moves(&game_state);

    info!("Turn {}: {} moves computed", turn, moves.len());

    let response = MoveResponse { moves };
    let response_body = serde_json::to_vec(&response).unwrap();
    let response_sig = sign_response(&state.secret, match_id, turn, &response_body);

    let mut resp_headers = HeaderMap::new();
    resp_headers.insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("application/json"),
    );
    resp_headers.insert(
        "X-ACB-Signature",
        HeaderValue::from_str(&response_sig).unwrap(),
    );

    Ok((resp_headers, response_body))
}

async fn handle_health() -> &'static str {
    "OK"
}

fn verify_signature(
    secret: &str,
    match_id: &str,
    turn: &str,
    timestamp: &str,
    body: &[u8],
    signature: &str,
) -> bool {
    let Ok(parsed_timestamp) = timestamp.parse::<i64>() else {
        return false;
    };
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs() as i64)
        .unwrap_or(0);
    if parsed_timestamp < now - TIMESTAMP_TOLERANCE_SECONDS
        || parsed_timestamp > now + TIMESTAMP_TOLERANCE_SECONDS
    {
        return false;
    }

    let body_hash = sha2::Sha256::digest(body);
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

fn sign_response(secret: &str, match_id: &str, turn: u32, body: &[u8]) -> String {
    let body_hash = sha2::Sha256::digest(body);
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
