mod strategy;
mod types;

use axum::{
    extract::State,
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::{IntoResponse, Json, Response},
    routing::{get, post},
    Router,
};
use hmac::{Hmac, Mac};
use serde_json::json;
use sha2::Sha256;
use std::net::SocketAddr;
use std::sync::Arc;

type HmacSha256 = Hmac<Sha256>;

#[derive(Clone)]
struct AppState {
    secret: Arc<String>,
}

#[tokio::main]
async fn main() {
    let secret = std::env::var("SHARED_SECRET")
        .expect("SHARED_SECRET environment variable must be set");

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
    axum::serve(listener, app)
        .await
        .expect("Server error");
}

async fn health() -> &'static str {
    "OK"
}

async fn turn(
    State(state): State<AppState>,
    headers: HeaderMap,
    Json(req_state): Json<types::VisibleState>,
) -> Result<Response, StatusCode> {
    // Verify signature (optional but recommended)
    if let Some(signature) = headers.get("x-acb-signature") {
        let match_id = headers
            .get("x-acb-match-id")
            .and_then(|v| v.to_str().ok())
            .unwrap_or("");
        let turn = headers
            .get("x-acb-turn")
            .and_then(|v| v.to_str().ok())
            .unwrap_or("0");

        let body = serde_json::to_vec(&req_state).unwrap();
        if !verify_signature(&body, match_id, turn, signature.to_str().unwrap(), &state.secret) {
            return Err(StatusCode::UNAUTHORIZED);
        }
    }

    // Compute moves
    let moves = strategy::compute_moves(&req_state);

    // Build response
    let response = json!({ "moves": moves });
    let body = serde_json::to_vec(&response).unwrap();

    let turn = req_state.turn;
    let match_id = &req_state.match_id;
    let sig = sign_response(&body, match_id, turn, &state.secret);

    Ok((
        [(header::HeaderName::from_static("x-acb-signature"), HeaderValue::from_str(&sig).unwrap())],
        Json(response),
    )
        .into_response())
}

fn verify_signature(body: &[u8], match_id: &str, turn: &str, signature: &str, secret: &str) -> bool {
    use sha2::Digest;
    let body_hash = sha2::Sha256::digest(body);
    let signing_string = format!("{}.{}.{}", match_id, turn, hex::encode(body_hash));
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(signing_string.as_bytes());
    let expected_sig = hex::encode(mac.finalize().into_bytes());

    // Constant-time comparison to prevent timing attacks
    if signature.len() != expected_sig.len() {
        return false;
    }
    signature.bytes().zip(expected_sig.bytes()).all(|(a, b)| a == b)
}

fn sign_response(body: &[u8], match_id: &str, turn: i32, secret: &str) -> String {
    use sha2::Digest;
    let body_hash = sha2::Sha256::digest(body);
    let signing_string = format!("{}.{}.{}", match_id, turn, hex::encode(body_hash));
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).unwrap();
    mac.update(signing_string.as_bytes());
    hex::encode(mac.finalize().into_bytes())
}
