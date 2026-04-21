# acb-starter-go

Go starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform.

## Quick Start

```bash
# Run locally
export BOT_SECRET=dev-secret
go run main.go

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
    "name": "my-go-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Project Structure

```
main.go     # HTTP server, HMAC auth, game types, and strategy entry point
go.mod      # Go module definition
Dockerfile  # Multi-stage container build
```

## Customization

Edit `computeMoves()` in `main.go` to implement your strategy. The `GameState` struct provides:

- `Bots` — all visible bots (yours and enemies)
- `Energy` — visible energy pickup locations
- `Cores` — visible core positions
- `Walls` — visible wall positions
- `You.Energy` — your current energy count
- `You.Score` — your current score
- `Config` — match parameters (grid size, attack range, etc.)

Return a slice of `Move` structs, each with the bot's current `Position` and a `Direction` (`"N"`, `"E"`, `"S"`, or `"W"`). Bots not included in the response stay in place.

## Protocol

- **Endpoint:** `POST /turn` — receives game state JSON, returns moves JSON
- **Health:** `GET /health` — must return 200
- **Timeout:** 3 seconds per turn
- **Auth:** HMAC-SHA256 via `X-ACB-Signature` header
