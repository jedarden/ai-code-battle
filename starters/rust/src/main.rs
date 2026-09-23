mod strategy;
mod types;

use axum::{
    body::Bytes,
    extract::State,
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::{IntoResponse, Response},
    routing::{get, post},
    Router,
};
use hmac::{Hmac, Mac};
use serde_json::json;
use sha2::Sha256;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

type HmacSha256 = Hmac<Sha256>;
const TIMESTAMP_TOLERANCE_SECONDS: i64 = 30;

#[derive(Clone)]
struct AppState {
    secret: Arc<String>,
}

#[tokio::main]
async fn main() {
    let secret =
        std::env::var("SHARED_SECRET").expect("SHARED_SECRET environment variable must be set");

    let state = AppState {
        secret: Arc::new(secret),
    };

    let app = Router::new()
        .route("/health", get(health))
        .route("/turn", post(turn))
        .with_state(state);

    let port = std::env::var("PORT").unwrap_or_else(|_| "8080".to_string());
    let addr = SocketAddr::from(([0, 0, 0, 0], port.parse().unwrap()));

    println!("Bot listening on port {}", port);

    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .expect("Failed to bind");
    axum::serve(listener, app).await.expect("Server error");
}

async fn health() -> &'static str {
    "OK"
}

async fn turn(
    State(state): State<AppState>,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response, StatusCode> {
    let match_id = headers
        .get("x-acb-match-id")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;
    let turn_str = headers
        .get("x-acb-turn")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;
    let timestamp = headers
        .get("x-acb-timestamp")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;
    let _bot_id = headers
        .get("x-acb-bot-id")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;
    let signature = headers
        .get("x-acb-signature")
        .and_then(|v| v.to_str().ok())
        .filter(|value| !value.is_empty())
        .ok_or(StatusCode::UNAUTHORIZED)?;

    let turn: i32 = turn_str.parse().map_err(|_| StatusCode::UNAUTHORIZED)?;
    if turn < 0 || turn.to_string() != turn_str {
        return Err(StatusCode::UNAUTHORIZED);
    }

    if !verify_signature(
        &body,
        match_id,
        turn_str,
        timestamp,
        signature,
        &state.secret,
    ) {
        return Err(StatusCode::UNAUTHORIZED);
    }

    let body_value: serde_json::Value =
        serde_json::from_slice(&body).map_err(|_| StatusCode::BAD_REQUEST)?;
    let body_match_id = body_value
        .get("match_id")
        .and_then(serde_json::Value::as_str)
        .ok_or(StatusCode::UNAUTHORIZED)?;
    let body_turn = body_value
        .get("turn")
        .and_then(serde_json::Value::as_u64)
        .and_then(|turn| i32::try_from(turn).ok())
        .ok_or(StatusCode::UNAUTHORIZED)?;
    if body_match_id != match_id || body_turn != turn {
        return Err(StatusCode::UNAUTHORIZED);
    }
    let req_state: types::VisibleState =
        serde_json::from_value(body_value).map_err(|_| StatusCode::BAD_REQUEST)?;

    // Compute moves
    let moves = strategy::compute_moves(&req_state);

    // Build response
    let response = json!({ "moves": moves });
    let response_body = serde_json::to_vec(&response).unwrap();

    let sig = sign_response(&response_body, match_id, turn, &state.secret);

    let mut response_headers = HeaderMap::new();
    response_headers.insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("application/json"),
    );
    response_headers.insert(
        header::HeaderName::from_static("x-acb-signature"),
        HeaderValue::from_str(&sig).unwrap(),
    );

    Ok((response_headers, response_body).into_response())
}

fn verify_signature(
    body: &[u8],
    match_id: &str,
    turn: &str,
    timestamp: &str,
    signature: &str,
    secret: &str,
) -> bool {
    use sha2::Digest;

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
    let signing_string = format!(
        "{}.{}.{}.{}",
        match_id,
        turn,
        timestamp,
        hex::encode(body_hash)
    );
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(signing_string.as_bytes());

    let Ok(signature) = hex::decode(signature) else {
        return false;
    };
    mac.verify_slice(&signature).is_ok()
}

fn sign_response(body: &[u8], match_id: &str, turn: i32, secret: &str) -> String {
    use sha2::Digest;
    let body_hash = sha2::Sha256::digest(body);
    let signing_string = format!("{}.{}.{}", match_id, turn, hex::encode(body_hash));
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(signing_string.as_bytes());
    hex::encode(mac.finalize().into_bytes())
}
