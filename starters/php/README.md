# acb-starter-php

PHP 8 starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform.

## Quick Start

```bash
# Run locally
BOT_SECRET=dev-secret php index.php

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
    "name": "my-php-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Project Structure

```
index.php      # HTTP server, HMAC auth, and routing
strategy.php   # Your bot logic (implement compute_moves())
game.php       # Game state types and grid utilities
composer.json  # Composer config (no external deps required)
Dockerfile     # Container build
```

## Grid Helpers

`game.php` provides utility functions for the toroidal grid:

- `toroidal_manhattan($a, $b, $rows, $cols)` — Manhattan distance with wrap-around
- `toroidal_chebyshev($a, $b, $rows, $cols)` — Chebyshev distance with wrap-around
- `neighbors($p, $rows, $cols)` — 8-directional neighbors with wrap
- `cardinal_steps($p, $rows, $cols)` — Cardinal directions with positions
- `bfs($start, $goal, $passable, $rows, $cols)` — BFS pathfinding, returns path or `null`

## Customization

Edit `compute_moves()` in `strategy.php` to implement your strategy. The `GameState` object provides:

- `bots` — all visible bots (yours and enemies)
- `energy` — visible energy pickup locations
- `cores` — visible core positions
- `walls` — visible wall positions
- `you->energy` — your current energy count
- `you->score` — your current score
- `config` — match parameters (grid size, vision radius, etc.)

Return an array of `Move` objects, each with the bot's current `position` and a `direction` (`"N"`, `"E"`, `"S"`, or `"W"`). Bots not included in the moves array stay in place.

## Protocol

- **Endpoint:** `POST /turn` — receives game state JSON, returns moves JSON
- **Health:** `GET /health` — must return 200
- **Timeout:** 3 seconds per turn
- **Auth:** HMAC-SHA256 via `X-ACB-Signature` header
