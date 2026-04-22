# acb-starter-python

Python 3 starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform.

## Quick Start

```bash
# Run locally
pip install -r requirements.txt
BOT_SECRET=dev-secret python3 main.py

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
    "name": "my-python-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Project Structure

```
main.py              # HTTP server, HMAC auth, and strategy entry point
grid.py              # Grid utilities (toroidal distance, BFS, neighbors)
requirements.txt     # Python dependencies (stdlib only for this starter)
Dockerfile           # Container build
```

## Grid Helpers

`grid.py` provides utility functions for the toroidal grid:

- `toroidal_manhattan(r1, c1, r2, c2, cols, rows)` — Manhattan distance with wrap-around
- `toroidal_chebyshev(r1, c1, r2, c2, cols, rows)` — Chebyshev distance with wrap-around
- `neighbors(row, col, rows, cols)` — 8-directional neighbors with wrap
- `bfs(start, goal, passable, rows, cols)` — BFS pathfinding, returns path or `None`

## Customization

Edit `compute_moves()` in `main.py` to implement your strategy. The `GameState` object provides:

- `bots` — all visible bots (yours and enemies)
- `energy` — visible energy pickup locations
- `cores` — visible core positions
- `walls` — visible wall positions
- `you_energy` — your current energy count
- `you_score` — your current score
- `config` — match parameters (grid size, etc.)

Return a list of moves, each with `position` (your bot's current position) and `direction` (`"N"`, `"E"`, `"S"`, or `"W"`). Bots not included in the moves list stay in place.

## Protocol

- **Endpoint:** `POST /turn` — receives game state JSON, returns moves JSON
- **Health:** `GET /health` — must return 200
- **Timeout:** 3 seconds per turn
- **Auth:** HMAC-SHA256 via `X-ACB-Signature` header
