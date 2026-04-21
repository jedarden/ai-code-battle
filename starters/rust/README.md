# acb-starter-rust

Rust starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform.

Uses `axum` for the HTTP server with `serde` for JSON and `hmac`/`sha2` for authentication.

## Quick Start

```bash
# Run locally
export BOT_SECRET=dev-secret
cargo run

# Run with Docker
docker build -t my-bot .
docker run -e BOT_SECRET=your-secret -p 8080:8080 my-bot
```

Your bot listens on port 8080 and responds to `POST /turn` with move commands.

## Register Your Bot

Once your bot is deployed and accessible via HTTPS:

```bash
curl -X POST https://api.aicodebattle.com/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-rust-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Project Structure

```
src/main.rs  # HTTP server, HMAC auth, game types, and strategy entry point
Cargo.toml   # Rust dependencies
Dockerfile   # Multi-stage container build
```

## Customization

Edit `compute_moves()` in `src/main.rs` to implement your strategy. The `GameState` struct provides:

- `bots` — all visible bots (yours and enemies)
- `energy` — visible energy pickup locations
- `cores` — visible core positions
- `walls` — visible wall positions
- `you.energy` — your current energy count
- `you.score` — your current score
- `config` — match parameters (grid size, etc.)

Return a `Vec<Move>`, each with the bot's current `position` and a `direction` (`"N"`, `"E"`, `"S"`, or `"W"`). Bots not included in the response stay in place.

## Protocol

- **Endpoint:** `POST /turn` — receives game state JSON, returns moves JSON
- **Health:** `GET /health` — must return 200
- **Timeout:** 3 seconds per turn
- **Auth:** HMAC-SHA256 via `X-ACB-Signature` header
