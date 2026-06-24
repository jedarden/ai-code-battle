# AI Code Battle

A competitive bot programming platform where participants write HTTP servers that control units on a grid world. Bots compete in real-time matches, and the winner is the last player with surviving cores.

## Overview

- Write a bot in **any language** — it just needs to run an HTTP server
- The game engine calls your bot every turn with the current game state
- Your bot responds with move orders for each of its units
- Matches run offline; results are published as animated replays with a leaderboard
- Ratings use **Glicko-2** (same system as chess.com)

## Table of Contents

1. [Game Rules](#game-rules)
2. [Writing a Bot](#writing-a-bot)
3. [Starter Templates](#starter-templates)
4. [Strategy Bots](#strategy-bots)
5. [Running Locally](#running-locally)
6. [Project Structure](#project-structure)
7. [Architecture](#architecture)
8. [Testing](#testing)

---

## Game Rules

### The Grid World

Matches are played on a **toroidal (wrapping) grid** — moving off one edge brings you out the other side. Tiles:

| Tile | Symbol | Effect |
|------|--------|--------|
| Open | `.`    | Units can occupy and traverse |
| Wall | `#`    | Impassable |
| Energy | `*`  | Collected by units standing on it |
| Core | `C`    | Win condition — protect yours, destroy theirs |

### Win Condition

The last player with at least one surviving **Core** tile wins. Cores are destroyed when an enemy unit occupies an undefended core tile during the Capture phase.

### Turn Phases

Each turn resolves in this order:

1. **Move** — Each unit executes its ordered direction (`N`, `E`, `S`, `W`) or stays
2. **Combat (focus-fire)** — Units in the same tile deal damage; outnumbered units take extra damage
3. **Zone** — A shrinking storm closes in from the edges, dealing damage to units caught in it
4. **Capture** — Enemy units on undefended Core tiles raze them
5. **Collect** — Units on Energy tiles collect energy for their player
6. **Spawn** — Players spend energy to spawn new units at Core tiles
7. **Energy Tick** — Passive energy regeneration
8. **Endgame Check** — Victory condition evaluated

### Key Rules

- **Self-collision**: If two or more of your own units move to the same tile, they **all die**
- **Energy economy**: Spawning units costs energy; energy is collected from `*` tiles and regenerated passively
- **Visibility**: Bots only see tiles and units within their units' sight radius (fog of war)

---

## Writing a Bot

A bot is an **HTTP server** with two endpoints. The game engine calls your server every turn.

### Required Endpoints

#### `GET /health`

Returns `200 OK` when your bot is ready. The engine polls this before starting a match.

```
GET /health HTTP/1.1

HTTP/1.1 200 OK
```

#### `POST /turn`

The engine sends the current game state; your bot responds with move orders.

**Request headers:**

| Header | Value |
|--------|-------|
| `Content-Type` | `application/json` |
| `X-ACB-Match-Id` | Match identifier string |
| `X-ACB-Turn` | Current turn number (integer as string) |
| `X-ACB-Signature` | HMAC-SHA256 signature of this request |

**Request body** (JSON game state):

```json
{
  "turn": 42,
  "player_id": "player-1",
  "bots": [
    {
      "id": 1,
      "x": 5,
      "y": 3,
      "health": 100,
      "owner": "player-1"
    }
  ],
  "tiles": [
    { "x": 4, "y": 3, "type": "open" },
    { "x": 5, "y": 3, "type": "energy" }
  ],
  "energy_locations": [{ "x": 5, "y": 3 }],
  "core_locations": [
    { "x": 0, "y": 0, "owner": "player-1" },
    { "x": 9, "y": 9, "owner": "player-2" }
  ],
  "scores": {
    "player-1": { "energy": 150, "cores": 2 },
    "player-2": { "energy": 80, "cores": 1 }
  }
}
```

> Only tiles and units visible to your bots are included (fog of war).

**Response body** (JSON move orders):

```json
{
  "moves": [
    { "bot_id": 1, "direction": "N" },
    { "bot_id": 2, "direction": "E" },
    { "bot_id": 3, "direction": "stay" }
  ]
}
```

Valid directions: `N`, `E`, `S`, `W`, `stay`

**Response header:**

| Header | Value |
|--------|-------|
| `X-ACB-Signature` | HMAC-SHA256 signature of your response body |

### HMAC Authentication

Every request and response is signed. The signing key is your `SHARED_SECRET` environment variable.

**Signing string format:**

```
{match_id}.{turn}.{sha256_hex(body)}
```

**Signature:**

```
HMAC-SHA256(key=SHARED_SECRET, message=signing_string)  →  hex-encoded
```

**To verify an incoming request:**

1. Read `X-ACB-Match-Id`, `X-ACB-Turn`, and the raw request body
2. Compute `sha256(body)` as hex
3. Build the signing string: `"{match_id}.{turn}.{body_sha256}"`
4. Compute `HMAC-SHA256(SHARED_SECRET, signing_string)` as hex
5. Compare with `X-ACB-Signature` (constant-time comparison)

**To sign your response:**

1. Compute `sha256(response_body)` as hex
2. Build the same signing string with the same `match_id` and `turn`
3. Compute `HMAC-SHA256(SHARED_SECRET, signing_string)` as hex
4. Set `X-ACB-Signature` response header to that value

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SHARED_SECRET` | Shared HMAC key provided by the match coordinator |
| `PORT` | Port your HTTP server listens on (default: `8080`) |

---

## Starter Templates

Ready-to-use templates are in the `starters/` directory. Each implements `/health` and `/turn` with HMAC auth and a random-move strategy — replace the move logic with your own.

| Language | Directory |
|----------|-----------|
| Python | `starters/python/` |
| Go | `starters/go/` |
| Rust | `starters/rust/` |
| TypeScript | `starters/typescript/` |
| JavaScript | `starters/javascript/` |
| Java | `starters/java/` |
| PHP | `starters/php/` |
| C# | `starters/csharp/` |

### Quick-start with Python

```bash
cd starters/python
pip install -r requirements.txt
SHARED_SECRET=test PORT=8080 python main.py
```

Then in another terminal, test your bot health:

```bash
curl http://localhost:8080/health
```

---

## Strategy Bots

The `bots/` directory contains 21 reference bots demonstrating different strategies. These serve as benchmarks and opponents in the automated ladder.

| Bot | Strategy |
|-----|----------|
| `random` | Random valid moves — baseline reference |
| `gatherer` | Energy collection priority, avoids combat |
| `rusher` | Rushes enemy cores aggressively |
| `guardian` | Defends cores, cautious expansion |
| `swarm` | Formation cohesion, advances as a group |
| `hunter` | Targets isolated enemy units |
| `assassin` | Prioritizes high-value targets |
| `coordinator` | Coordinates unit actions across the field |
| `defender` | Pure defensive posture |
| `economist` | Maximizes energy efficiency |
| `farmer` | Long-term energy farming strategy |
| `kamikaze` | Sacrificial rush tactics |
| `leader-targeter` | Focuses fire on the strongest opponent |
| `nomad` | Mobile, avoids prolonged engagements |
| `opportunist` | Exploits momentary advantages |
| `pacifist` | Avoids combat entirely |
| `phalanx` | Tight defensive formation |
| `raider` | Hit-and-run energy denial |
| `scout` | Prioritizes map exploration |
| `siege` | Methodical core destruction |
| `zone-driver` | Exploits the shrinking zone mechanic |

Bot implementations span Go, Rust, Python, TypeScript, PHP, and Java.

### Bot Evolver

The `acb-evolver/` component mutates and evolves bot strategies automatically. It runs genetic-algorithm-style evolution over strategy parameters, producing improved variants over time.

---

## Running Locally

### Prerequisites

- Go 1.21+ (game engine and CLI tools)
- Node.js 18+ (web app and worker API)
- Docker (for containerized bots and deployment)

### Build and Run a Match

```bash
# Build CLI tools
go build ./cmd/acb-local
go build ./cmd/acb-mapgen

# Run a match between two built-in bots (outputs replay JSON)
./acb-local -seed 42 -max-turns 100 -output replay.json -verbose
```

### Test Your Bot

```bash
# Start the Python starter bot
cd starters/python
pip install -r requirements.txt
SHARED_SECRET=test PORT=8080 python main.py

# In another terminal — run a match against a built-in bot
./acb-local -bot1 http://localhost:8080 -bot2 builtin:rusher \
  -secret1 test -seed 42 -output replay.json -verbose
```

### View Replays

```bash
# Start the web dev server
cd web && npm install && npm run dev
# Open http://localhost:3000/ → Replay Viewer → load replay.json
```

The replay viewer shows turn-by-turn animation on a canvas, win probability tracking, an event timeline, and auto-generated commentary.

---

## Project Structure

```
ai-code-battle/
├── engine/              # Go game simulation library
│   ├── types.go         # Core data types
│   ├── grid.go          # Toroidal grid implementation
│   ├── game.go          # Game state management
│   ├── turn.go          # Turn execution phases
│   ├── replay.go        # Replay recording
│   └── *_test.go        # Test files
├── cmd/
│   ├── acb-local/       # CLI match runner (local testing)
│   ├── acb-mapgen/      # Map generator
│   ├── acb-worker/      # Match execution worker
│   └── acb-indexer/     # Index builder for static files
├── web/                 # Cloudflare Pages SPA
│   └── src/
│       ├── replay-viewer.ts  # Canvas replay renderer
│       └── app.ts            # SPA entry point
├── worker-api/          # Cloudflare Worker API
│   └── src/
│       ├── index.ts     # Router + cron dispatcher
│       ├── jobs.ts      # Job coordination
│       ├── bots.ts      # Bot management
│       └── glicko2.ts   # Glicko-2 rating system
├── bots/                # Strategy bot implementations (21 bots)
├── starters/            # Starter templates (8 languages)
│   ├── python/
│   ├── go/
│   ├── rust/
│   ├── typescript/
│   ├── javascript/
│   ├── java/
│   ├── php/
│   └── csharp/
└── acb-evolver/         # Bot evolution engine
```

---

## Architecture

The platform is split between Cloudflare's edge network and a cloud VM:

- **Cloudflare Pages** — Static SPA (replay viewer, leaderboard, match history)
- **Cloudflare Workers + D1 + R2** — Match coordination API, bot registry, rating updates, replay storage
- **Cloud VM** — Match workers that pull jobs from the API, run bots as containers, and publish replays

Match flow:

1. Worker API schedules match jobs
2. Match worker pulls a job and starts both bot containers
3. Engine runs the match turn-by-turn, calling each bot's `/turn` endpoint
4. Replay JSON is uploaded to R2
5. Worker API updates Glicko-2 ratings and triggers index rebuild
6. Web frontend fetches the new replay and displays it

**Rating system**: Glicko-2 — accounts for rating reliability (RD) and rating volatility, converges faster than Elo, same algorithm used by chess.com.

---

## Testing

```bash
# Game engine unit tests
go test ./engine/... -v

# Worker API tests
cd worker-api && npm test

# Index builder tests
cd cmd/acb-indexer && npm test
```

---

## License

MIT
