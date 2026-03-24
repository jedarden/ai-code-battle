# AI Code Battle — Implementation Plan

## 1. Overview

AI Code Battle is a competitive bot programming platform where participants write
HTTP servers that control units on a grid world. The game engine orchestrates
matches asynchronously, stores replays, and serves a web platform where visitors
watch rendered game replays and browse leaderboards. Matches are never live —
they are evaluated offline by match workers and presented as completed replays.

The platform ships with several built-in strategy bots, each deployed as its own
container, serving as both opponents for new participants and reference
implementations for the HTTP protocol.

---

## 2. System Architecture

The platform is split across two tiers:

1. **Cloudflare (free tier)** — all web-facing infrastructure: static site,
   API endpoints, database, file storage, and scheduling logic
2. **Rackspace Spot** — all compute: match execution, bot hosting, evolution
   pipeline

This split maps cleanly to each provider's strength. Cloudflare excels at
serving content globally with zero egress cost. Rackspace Spot provides cheap
interruptible compute for the CPU-intensive match simulation.

```
┌─────────────────────── Cloudflare (free tier) ───────────────────────┐
│                                                                       │
│  ┌─────────────┐   ┌──────────────────┐   ┌───────────────────────┐  │
│  │  Pages       │   │  Worker (acb-api) │   │  R2 Bucket            │  │
│  │  static site │   │  registration,    │   │  replays/*.json.gz    │  │
│  │  HTML/JS/CSS │   │  job coordination,│   │  data/leaderboard.json│  │
│  │              │   │  cron triggers    │   │  data/bots/*.json     │  │
│  └──────┬──────┘   └────────┬─────────┘   │  data/matches/*.json  │  │
│         │                   │              │  maps/*.json          │  │
│         │ fetches JSON      │ reads/writes └───────────┬───────────┘  │
│         └───────────────────┼─────────────────────────►│              │
│                             │                                         │
│                    ┌────────▼────────┐                                │
│                    │  D1 Database     │                                │
│                    │  bots, matches,  │                                │
│                    │  jobs, ratings   │                                │
│                    └─────────────────┘                                │
└──────────────────────────────┬───────────────────────────────────────┘
                               │ HTTPS (job coordination + result submission)
                               │
┌──────────────────────── Rackspace Spot ──────────────────────────────┐
│                                                                       │
│  ┌──────────────────┐    ┌──────────────────────────────────────────┐ │
│  │  Match Workers    │    │  Bot Containers                          │ │
│  │  (claim jobs,     │───►│  ┌──────────┐ ┌──────────┐ ┌──────────┐│ │
│  │   run simulation, │HTTP│  │ Strategy  │ │ Evolved  │ │ External ││ │
│  │   upload replay   │    │  │ Bots (×6) │ │ Bots     │ │ Bots     ││ │
│  │   to R2, POST     │    │  └──────────┘ └──────────┘ └──────────┘│ │
│  │   result to API)  │    └──────────────────────────────────────────┘ │
│  └──────────────────┘                                                 │
│                                                                       │
│  ┌──────────────────┐                                                 │
│  │  Evolver          │                                                │
│  │  (LLM pipeline,  │                                                 │
│  │   sandbox, eval)  │                                                │
│  └──────────────────┘                                                 │
└──────────────────────────────────────────────────────────────────────┘
```

### Component Summary

| Component | Where | Role |
|-----------|-------|------|
| **Pages** | Cloudflare | Static site — HTML/JS/CSS SPA, fetches JSON from R2 |
| **Worker** | Cloudflare | API endpoints (registration, job coordination) + cron triggers (matchmaking, index rebuilds, health checks) |
| **D1** | Cloudflare | SQLite database — bot registry, match queue, ratings, results |
| **R2** | Cloudflare | Object storage — replay files, pre-built JSON indexes (leaderboard, bot profiles, match lists), maps |
| **Match Workers** | Rackspace Spot | Stateless match execution — claim job from Worker API, run simulation, upload replay to R2, POST result |
| **Bot Containers** | Rackspace Spot | Strategy bots (×6) + evolved bots (0–50) — HTTP servers called by workers during matches |
| **Evolver** | Rackspace Spot | Evolution pipeline — LLM generation, sandbox validation, evaluation matches |

**What's intentionally absent:** no PostgreSQL, no Redis, no always-on VPS for
web infrastructure, no Nginx, no reverse proxy. Cloudflare handles TLS, CDN,
DNS, storage, and compute-at-edge for the entire web-facing tier at zero cost.

---

## 3. Game Mechanics

### 3.1 Map & Grid

The game plays on a **toroidal grid** — a rectangular grid that wraps both
horizontally and vertically (no edges, no corners). This eliminates
positional advantages from map boundaries.

**Tile types:**

| Tile | Symbol | Description |
|------|--------|-------------|
| Open | `.` | Passable empty tile |
| Wall | `#` | Impassable barrier |
| Energy | `*` | Collectible resource (respawns) |
| Core | `C` | Player spawn point (owned by a player) |

**Grid parameters (configurable per match):**

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `rows` | 60 | 30–120 | Grid height |
| `cols` | 60 | 30–120 | Grid width |
| `wall_density` | 0.15 | 0.05–0.30 | Fraction of tiles that are walls |
| `energy_nodes` | 20 | 8–50 | Number of energy spawn locations |
| `cores_per_player` | 1 | 1–2 | Starting cores per player |

### 3.2 Units (Bots)

Each player controls **bots** — mobile units on the grid.

- Bots move one tile per turn in a cardinal direction: `N`, `E`, `S`, `W`
- Bots that do not receive a move order hold position
- Bots are binary — alive or dead, no hit points
- A bot ordered into a wall tile stays in place (order ignored)
- Two friendly bots ordered to the same tile: **both die** (self-collision)
- A bot ordered onto a tile occupied by a stationary enemy: **both die**

Each player starts with one bot spawned at each of their cores.

### 3.3 Energy & Economy

Energy is the sole resource. It is used to spawn new bots.

**Energy nodes:**
- Fixed positions on the map that periodically produce collectible energy
- Energy appears on a node every `energy_interval` turns (default: 10)
- When energy is present on a node, it is visible to any player who can see the tile

**Collection:**
- A bot adjacent to (or on) an energy tile collects it if no enemy bot is also
  adjacent to that energy
- If bots from multiple players are adjacent to the same energy, the energy is
  **destroyed** — nobody gets it (contested resources are denied)
- Collection happens after combat resolution each turn

**Spawning:**
- Cost: **3 energy** per bot
- Spawning happens automatically when a player has ≥3 energy and an unoccupied,
  unrazed core
- One bot spawns per core per turn maximum
- If a player has multiple cores and enough energy, one bot spawns at each
  eligible core simultaneously
- Spawn priority: core that has been idle longest

### 3.4 Combat

Combat uses a **focus fire** algorithm inspired by the aichallenge ants system.
This rewards formations and positioning over raw unit count.

**Attack radius:** squared Euclidean distance ≤ `attack_radius2` (default: **5**,
meaning ~2.24 tiles — includes cardinal and diagonal neighbors plus one more ring).

**Resolution (simultaneous):**

```
for each bot B on the grid:
    enemies_of_B = count of enemy bots within attack_radius2 of B
    for each enemy E within attack_radius2 of B:
        enemies_of_E = count of E's enemies within attack_radius2 of E
        if enemies_of_B >= enemies_of_E:
            mark B as dead
            break  (B is already dead, no need to check further)
```

All deaths are resolved **simultaneously** — no cascading within a single turn.

**Key properties:**
- 2v1: the lone bot dies, the pair survives (superior numbers win cleanly)
- 1v1: both die (mutual destruction)
- Tight formations are defensive — a cluster facing scattered enemies takes
  fewer losses because each bot in the cluster has a lower enemy count
- Multi-player battles create emergent alliances and third-party exploitation

### 3.5 Fog of War

Each player has limited visibility. Only tiles within `vision_radius2`
(default: **49**, ~7 tiles) of any owned bot are visible.

**What players see within their vision:**
- All tile types (open, wall, energy, core)
- Enemy bots and their owner IDs
- Dead bots (for one turn after death)

**What players do NOT see:**
- Anything outside their collective vision radius
- How much energy opponents have
- Total number of opponents (discovered through play)

**Walls** are sent every turn they are visible (no incremental discovery state —
keeps the protocol stateless-friendly for HTTP bots).

### 3.6 Scoring & Win Conditions

**Scoring:**
- Each player starts with **1 point per core** owned
- **Capturing a core** (enemy bot moves onto an undefended enemy core): **+2 points** to capturer, **−1 point** to owner; core is razed
- **Razed cores** stop spawning but the player continues with remaining bots
- **Energy collected**: tracked as a tiebreaker statistic (not added to score)
- **Bots eliminated**: tracked as a statistic

**Win conditions (checked in order):**

| Condition | Trigger | Resolution |
|-----------|---------|------------|
| **Sole Survivor** | Only one player has living bots | That player wins; bonus +2 per surviving enemy core |
| **Annihilation** | All players eliminated simultaneously | Draw |
| **Dominance** | One player controls ≥80% of all bots for 100 consecutive turns | That player wins |
| **Turn Limit** | Turn count reaches `max_turns` (default: 500) | Highest score wins; ties broken by energy collected, then bots alive |

### 3.7 Turn Structure

Each turn executes in a strict, deterministic sequence:

```
1.  Send game state to all players (HTTP POST, filtered by fog of war)
2.  Await responses (up to 3-second timeout per player, in parallel)
3.  Validate all responses against schema
4.  Phase: MOVE        — execute valid movement orders
5.  Phase: COMBAT      — resolve focus-fire algorithm, remove dead bots
6.  Phase: CAPTURE     — enemy bots on undefended cores raze them
7.  Phase: COLLECT     — uncontested energy adjacent to bots is collected
8.  Phase: SPAWN       — players with ≥3 energy spawn bots at eligible cores
9.  Phase: ENERGY_TICK — energy nodes on their interval produce new energy
10. Phase: ENDGAME     — check win conditions
11. Record turn state for replay
```

All player requests in step 1 are sent **concurrently**. Responses are collected
with the 3-second deadline. The engine does not proceed to step 3 until all
responses are in or timed out.

### 3.8 Map Generation

Maps are generated offline and stored in the map library. They are not generated
on-the-fly during matches.

**Symmetry requirements:**
- 2-player maps: 180° rotational symmetry (point symmetry through center)
- 3-player maps: 120° rotational symmetry
- 4-player maps: 90° rotational symmetry
- 6-player maps: 60° rotational symmetry

**Generation algorithm:**
1. Generate one **sector** (1/N of the map for N players)
2. Place walls using cellular automata (random seed → smooth with neighbor rules)
3. Place cores and energy nodes within the sector
4. Validate connectivity: BFS from core must reach all energy nodes and the
   sector boundary
5. Mirror/rotate the sector to fill the full map
6. Validate full-map connectivity: all cores must be reachable from each other
7. Store the map with metadata (player count, dimensions, wall density)

**Map library:**
- Pre-generated pool of 50+ maps per player count (2, 3, 4, 6)
- Maps are curated — auto-generated then play-tested with strategy bots
- Matchmaking selects the least-recently-used map for each match

---

## 4. Communication Protocol

### 4.1 HTTP Interface

The game engine communicates with bots via HTTP POST requests. Each bot exposes
a single endpoint.

**Bot endpoint:** `POST {bot_base_url}/turn`

The engine sends the game state as a JSON body. The bot responds with its moves
as a JSON body. No other endpoints are required from the bot (though `/health` is
recommended for registration validation).

**Request flow per turn:**
```
Engine                          Bot
  │                              │
  │  POST /turn                  │
  │  Headers: auth + metadata    │
  │  Body: game state JSON       │
  │─────────────────────────────►│
  │                              │  (bot computes moves)
  │  200 OK                      │
  │  Body: moves JSON            │
  │◄─────────────────────────────│
  │                              │
```

### 4.2 Game State Schema (Engine → Bot)

```json
{
  "match_id": "m_7f3a9b2c",
  "turn": 42,
  "config": {
    "rows": 60,
    "cols": 60,
    "max_turns": 500,
    "vision_radius2": 49,
    "attack_radius2": 5,
    "spawn_cost": 3,
    "energy_interval": 10
  },
  "you": {
    "id": 0,
    "energy": 7,
    "score": 3
  },
  "bots": [
    { "row": 10, "col": 15, "owner": 0 },
    { "row": 12, "col": 15, "owner": 0 },
    { "row": 30, "col": 40, "owner": 1 }
  ],
  "energy": [
    { "row": 20, "col": 25 }
  ],
  "cores": [
    { "row": 5, "col": 5, "owner": 0, "active": true },
    { "row": 55, "col": 55, "owner": 1, "active": true }
  ],
  "walls": [
    { "row": 10, "col": 10 },
    { "row": 10, "col": 11 }
  ],
  "dead": [
    { "row": 15, "col": 20, "owner": 1 }
  ]
}
```

**Schema rules:**
- `bots`, `energy`, `cores`, `walls`, `dead` — only includes tiles within the
  player's collective vision
- `owner` IDs are consistent within a match but randomized per match (player 0
  is always "you")
- `config` is identical for all players and does not change between turns
- `walls` are sent every turn they are visible (stateless — bot does not need to
  track previously seen walls, though smart bots will)
- `dead` contains bots that died on the previous turn (visible for one turn)

### 4.3 Move Schema (Bot → Engine)

```json
{
  "moves": [
    { "row": 10, "col": 15, "direction": "N" },
    { "row": 12, "col": 15, "direction": "E" }
  ]
}
```

**Validation rules:**
- `moves` must be an array (may be empty — all bots hold position)
- Each move must reference a `(row, col)` where the player owns a bot
- `direction` must be one of: `"N"`, `"E"`, `"S"`, `"W"`
- Duplicate `(row, col)` entries: first valid entry wins, rest ignored
- Moves referencing tiles with no owned bot: ignored
- Moves into walls: ignored (bot stays)
- Any response that fails top-level schema validation: entire response
  discarded, all bots hold
- **The engine never parses, evaluates, or interprets any field beyond
  `moves[].row`, `moves[].col`, `moves[].direction`**

### 4.4 Authentication (HMAC Shared Secret)

Each registered bot has a **shared secret** generated at registration time. The
secret is known only to the bot owner and the game engine. It authenticates both
directions — the bot can verify requests came from the real game engine, and the
engine can verify responses came from the real bot.

**Engine → Bot (request signing):**

Headers sent with every request:
```
X-ACB-Match-Id: m_7f3a9b2c
X-ACB-Turn: 42
X-ACB-Timestamp: 1711200000
X-ACB-Bot-Id: b_4e8c1d2f
X-ACB-Signature: <hex-encoded HMAC-SHA256>
```

Signature computation:
```
signing_string = "{match_id}.{turn}.{timestamp}.{sha256(request_body)}"
signature = HMAC-SHA256(shared_secret, signing_string)
```

The bot verifies:
1. Compute the expected signature from the headers and request body
2. Compare with `X-ACB-Signature` (constant-time comparison)
3. Verify `X-ACB-Timestamp` is within ±30 seconds of current time (prevents
   replay attacks)
4. If verification fails: bot should return 401 and ignore the request

**Bot → Engine (response signing):**

Response headers:
```
X-ACB-Signature: <hex-encoded HMAC-SHA256>
```

Signature computation:
```
signing_string = "{match_id}.{turn}.{sha256(response_body)}"
signature = HMAC-SHA256(shared_secret, signing_string)
```

The engine verifies the response signature. If invalid, the response is
discarded (bots hold position). This prevents man-in-the-middle from
injecting moves.

**Why HMAC over OAuth/JWT/mTLS:**
- Minimal complexity — no token refresh, no certificate management
- Bot developers add a single header computation, not an auth library
- Symmetric: both sides can verify the other with the same secret
- Sufficient for the threat model (prevent impersonation and tampering)

**Secret management:**
- Secrets are generated as 256-bit random values, hex-encoded (64 characters)
- Displayed once at registration time; bot owner must save it
- Can be rotated via the web platform (old secret invalidated immediately)
- Stored hashed (bcrypt) in the database — the engine uses the hash to verify,
  so the raw secret is never stored. **Correction**: HMAC requires the raw
  secret, so it is stored encrypted (AES-256-GCM) with a master key, not
  hashed. The master key is held in an environment variable, never in the database.

### 4.5 Timeout & Error Handling

| Scenario | Behavior |
|----------|----------|
| Bot responds within 3s | Moves validated and applied normally |
| Bot responds after 3s | Response discarded; bots hold position for that turn |
| Bot returns non-200 status | Treated as timeout; bots hold position |
| Bot returns invalid JSON | Treated as timeout; bots hold position |
| Bot returns valid JSON failing schema | Entire response discarded; bots hold position |
| Bot connection refused | Bots hold position; engine retries next turn |
| Bot connection timeout (TCP) | Engine uses 2s connect timeout within the 3s budget |
| 10 consecutive failures | Bot marked as **crashed** for this match; bots become inert for remaining turns |

The bot is **never killed or disconnected**. Even after being marked crashed, the
match continues — the crashed bot's units simply hold position every turn until
they are destroyed or the match ends.

---

## 5. Strategy Bots

Six built-in strategy bots serve as reference implementations and permanent
ladder opponents. Each is implemented in a **different programming language**
to demonstrate that the HTTP protocol is truly language-agnostic and to
provide starter code for participants across the most popular ecosystems.

Each bot is deployed as its own container running a lightweight HTTP server.

| Bot | Language | Complexity | Expected Rank |
|-----|----------|------------|---------------|
| RandomBot | Python | Trivial | 6th (floor) |
| GathererBot | Go | Low | 4th–5th |
| RusherBot | Rust | Low | 4th–5th |
| GuardianBot | PHP | Medium | 3rd–4th |
| SwarmBot | TypeScript | Medium | 1st–2nd |
| HunterBot | Java | High | 1st–2nd |

### 5.1 RandomBot — Python

**Language rationale:** Python is the most accessible language for newcomers.
The random bot doubles as the simplest possible starter template — a
participant can fork it and have a working bot in minutes.

**Strategy:** Makes uniformly random valid moves each turn.

**Behavior:**
- For each owned bot, pick a random direction (N/E/S/W) or hold (20% chance)
- No pathfinding, no memory, no awareness of enemies
- Serves as the absolute baseline — any reasonable bot should beat this

**Value:** Ensures new participants have an easy opponent to test against.
Rating floor anchor.

**Implementation:** Flask or bare `http.server`. ~50 lines of strategy code.
HMAC verification via `hmac` stdlib module.

### 5.2 GathererBot — Go

**Language rationale:** Go is the same language as the game engine and
platform services, making this the canonical "how to build a bot" reference.
Demonstrates idiomatic Go HTTP server patterns.

**Strategy:** Maximize energy collection, avoid combat entirely.

**Behavior:**
- BFS from each owned bot to the nearest visible energy
- Assign each bot to the closest uncontested energy (greedy matching)
- If an enemy bot is within vision, move away from it
- Never voluntarily enters attack range of an enemy
- Spawns bots as fast as energy allows

**Value:** Tests whether aggressive bots can actually close games or whether
passive resource hoarding is dominant (it shouldn't be).

**Implementation:** `net/http` stdlib server. Shared `game/` package with
grid utilities, BFS, and distance calculations that participants can reuse.

### 5.3 RusherBot — Rust

**Language rationale:** Rust participants get maximum compute within the
3-second timeout. This bot demonstrates that Rust's performance advantage
matters less than strategy — a dumb fast bot still loses to a smart slow one.

**Strategy:** Identify and rush the nearest enemy core as fast as possible.

**Behavior:**
- BFS from each owned bot toward the nearest known enemy core
- If no enemy core is known, spread out to explore (random walk with
  bias toward unexplored territory)
- Ignores energy except incidentally (walks over it)
- Ignores enemy bots unless they block the path
- Spawns bots immediately and sends all toward the target

**Value:** Punishes bots that neglect defense. Tests whether the combat
system allows pure aggression to dominate (it shouldn't — rusher bots will
walk into defensive formations and die).

**Implementation:** `axum` or `actix-web`. Serde for JSON. HMAC via `hmac`
and `sha2` crates. Demonstrates Rust's zero-copy deserialization.

### 5.4 GuardianBot — PHP

**Language rationale:** PHP is often overlooked in competitive programming
but is widely known and trivially deployable. This demonstrates that even
PHP — without async, without frameworks — can compete on equal footing
when the interface is HTTP. Lowers the barrier for the large PHP developer
community.

**Strategy:** Defend own core, gather nearby energy, cautious expansion.

**Behavior:**
- Maintain a perimeter of bots within 5 tiles of each owned core
- Assign excess bots (beyond perimeter needs) to gather energy within
  10 tiles of a core
- If enemy bots are spotted approaching, consolidate defenders between
  the enemy and the core
- Only sends scouts (lone bots) to explore beyond the safe zone
- Very conservative spawning — maintains energy reserve of 6

**Value:** Tests whether turtling is viable. Should beat rushers but lose to
gatherers/swarms in the long game (inferior economy due to limited territory).

**Implementation:** PHP built-in server (`php -S`) with a single router
script. `hash_hmac()` for HMAC. JSON via `json_decode`/`json_encode`.
BFS implemented with `SplQueue`.

### 5.5 SwarmBot — TypeScript

**Language rationale:** TypeScript (Node.js) is the most popular language
for web developers entering the platform. This bot demonstrates maintaining
complex state across turns — the swarm's formation tracking, rally points,
and center-of-mass calculation benefit from TypeScript's type system.

**Strategy:** Keep units in tight formations, advance as a group toward enemies.

**Behavior:**
- All bots maintain cohesion — no bot moves if it would be >3 tiles from the
  nearest friendly bot
- The swarm moves as a unit toward the nearest enemy presence
- BFS-based center-of-mass steering: average position of all owned bots
  is the swarm center; steer toward enemy center of mass
- Energy collection is incidental (pass over it during advance)
- New spawns rally to the swarm before advancing

**Value:** Exploits the focus combat system — a tight group defeats scattered
enemies. But slow expansion means inferior economy. Should dominate combat
but can be outscored by gatherers on large maps.

**Implementation:** Express.js or Fastify. State persisted in-process across
turns (the HTTP server stays alive between requests). HMAC via Node.js
`crypto` module. Typed interfaces for game state and moves.

### 5.6 HunterBot — Java

**Language rationale:** Java is dominant in competitive programming (Battlecode
is Java-only). This is the most sophisticated strategy bot, demonstrating
that Java's verbosity is offset by mature data structures (`PriorityQueue`,
`HashMap`) and predictable GC behavior within the timeout window.

**Strategy:** Target isolated enemy bots for efficient kills.

**Behavior:**
- Identify enemy bots that are ≥4 tiles from their nearest friendly bot
  (isolated targets)
- Send pairs of bots to intercept isolated enemies (2v1 wins cleanly)
- If no isolated targets, default to gatherer behavior
- Maintain a map of known enemy positions across turns, predict movement
  based on last-seen direction and speed
- Avoid engaging formations of 3+ enemy bots
- Opportunistic energy collection when not actively hunting

**Value:** Sophisticated target selection and prediction. Represents an
intermediate-to-advanced-skill bot. Should beat random/gatherer/rusher but
struggle against swarm formations.

**Implementation:** Javalin or `com.sun.net.httpserver`. `javax.crypto.Mac`
for HMAC. Maintains a `HashMap<Position, EnemyTracker>` across turns for
movement prediction. Hungarian algorithm for optimal bot-to-target assignment.

### 5.7 Container Templates

Each language has its own container structure. All share the same contract:
listen on port 8080, serve `POST /turn` and `GET /health`.

**Go (GathererBot):**
```
strategy-gatherer/
├── Dockerfile
├── main.go                  # HTTP server, HMAC verification
├── strategy.go              # Gatherer-specific logic
├── game/
│   ├── state.go             # Game state types
│   ├── grid.go              # Grid utilities (BFS, distance, wrapping)
│   └── moves.go             # Move response types
└── go.mod
```

**Python (RandomBot):**
```
strategy-random/
├── Dockerfile
├── main.py                  # HTTP server, HMAC verification, strategy
├── game.py                  # Game state types and grid utilities
└── requirements.txt         # (minimal — stdlib only for random bot)
```

**Rust (RusherBot):**
```
strategy-rusher/
├── Dockerfile
├── Cargo.toml
└── src/
    ├── main.rs              # HTTP server, HMAC verification
    ├── strategy.rs          # Rusher-specific logic
    └── game.rs              # Game state types, grid utilities
```

**PHP (GuardianBot):**
```
strategy-guardian/
├── Dockerfile
├── index.php                # Router + HMAC verification
├── strategy.php             # Guardian-specific logic
├── game.php                 # Game state types, BFS, grid utilities
└── composer.json             # (optional — no external deps needed)
```

**TypeScript (SwarmBot):**
```
strategy-swarm/
├── Dockerfile
├── package.json
├── tsconfig.json
└── src/
    ├── index.ts             # HTTP server, HMAC verification
    ├── strategy.ts          # Swarm-specific logic
    └── game.ts              # Game state types, grid utilities
```

**Java (HunterBot):**
```
strategy-hunter/
├── Dockerfile
├── pom.xml
└── src/main/java/com/acb/hunter/
    ├── App.java             # HTTP server, HMAC verification
    ├── Strategy.java        # Hunter-specific logic
    ├── GameState.java       # Game state deserialization
    └── Grid.java            # Grid utilities, BFS, distance
```

**Shared contract (all languages):**
- Listen on port 8080
- `POST /turn` — receives game state, runs strategy, returns moves
- `GET /health` — returns 200 (used for registration health check)
- HMAC signature verification on incoming requests
- HMAC signature on outgoing responses
- Request logging (turn number, compute time, move count)

**Container specs:**

| Bot | Build Image | Runtime Image | Memory Limit | CPU Limit |
|-----|-------------|---------------|-------------|-----------|
| RandomBot | `python:3.13-slim` | `python:3.13-slim` | 64MB | 0.1 cores |
| GathererBot | `golang:1.24-alpine` | `alpine:3.21` | 128MB | 0.25 cores |
| RusherBot | `rust:1.85-alpine` | `alpine:3.21` | 128MB | 0.25 cores |
| GuardianBot | `php:8.4-cli-alpine` | `php:8.4-cli-alpine` | 128MB | 0.25 cores |
| SwarmBot | `node:22-alpine` | `node:22-alpine` | 128MB | 0.25 cores |
| HunterBot | `eclipse-temurin:21-alpine` | `eclipse-temurin:21-jre-alpine` | 256MB | 0.5 cores |

Java gets a higher resource allocation due to JVM overhead. All others are
intentionally constrained — strategy bots should be lightweight.

### 5.8 Starter Kit & SDK Libraries

To lower the barrier for participants writing their own bots, the platform
provides **starter kits** for each supported language. Each starter kit is a
minimal, forkable repository containing:

- A working HTTP server with HMAC verification already implemented
- Type definitions for the game state and move schemas
- Grid utility functions (toroidal distance, BFS, neighbor enumeration)
- A stub strategy function that holds all bots in place (participant fills in)
- A Dockerfile that builds and runs the bot
- A README with quickstart instructions

**Starter kit languages (matching strategy bots):**

| Kit | Repository | Notes |
|-----|-----------|-------|
| `acb-starter-python` | Template repo | Flask-based, ~100 lines total |
| `acb-starter-go` | Template repo | Shares `game/` package with GathererBot |
| `acb-starter-rust` | Template repo | `axum` + `serde`, strongly typed |
| `acb-starter-php` | Template repo | Zero dependencies, built-in server |
| `acb-starter-typescript` | Template repo | Fastify, full type definitions |
| `acb-starter-java` | Template repo | Javalin, Maven-based |

Participants are not limited to these languages. Any language that can serve
HTTP and compute HMAC-SHA256 can compete. The starter kits simply eliminate
boilerplate for the most common choices.

---

## 6. Tournament System

### 6.1 Matchmaking

Matches are created continuously by the **tournament scheduler**, a
process that runs on a fixed interval (default: every 10 seconds).

**Algorithm:**

1. **Select seed bot**: the registered bot with the most time since its last
   match (tiebreak: lowest bot ID)
2. **Determine match size**: based on the seed bot's least-played format
   (2-player, 3-player, 4-player, or 6-player)
3. **Select opponents**: from the eligible pool, preferring:
   a. Closest skill rating to seed (Pareto distribution: 80% within 16 ranks)
   b. Least recently paired with the seed
   c. Fewest games played in the last 24 hours (keeps game counts even)
4. **Select map**: least recently used map for the chosen player count
5. **Assign player slots**: random
6. **Create match job**: push to Redis queue with match config + bot endpoints

**Eligibility:**
- Bot must be registered and active (passed health check within last hour)
- Bot must not be in a match currently (one match at a time per bot)
- Bot must not have been marked crashed in its last 3 consecutive matches
  (cooldown: 30 minutes)

### 6.2 Rating System

**Algorithm: Glicko-2**

Glicko-2 is preferred over TrueSkill for this platform because:
- No licensing concerns (TrueSkill is patented by Microsoft)
- Includes a volatility parameter (σ) that adapts to inconsistent performance
- Well-suited to multi-player games via pairwise decomposition
- Established in competitive gaming (chess, Go, online games)

**Parameters per bot:**
- `mu` (μ): rating estimate (default: 1500)
- `phi` (φ): rating deviation / uncertainty (default: 350)
- `sigma` (σ): rating volatility (default: 0.06)

**Display rating:** `mu - 2*phi` (conservative estimate shown on leaderboard)

**Update frequency:** after every match. Ratings converge quickly — a new bot
reaches a stable rating within ~30 matches.

**Multi-player adaptation:**
- A 4-player match produces 6 pairwise results (every pair of players)
- Each pairwise result is: win/loss based on relative score, or draw if equal
- Glicko-2 update is applied once per match using all pairwise outcomes

### 6.3 Continuous Tournament

The tournament runs indefinitely with no seasons or resets (initially).

**Match throughput target:** enough matches that every active bot plays at
least 10 matches per day. With N active bots and M match workers:
- 2-player matches: each match involves 2 bots, takes ~3 minutes (500 turns × 3s max + overhead)
- One worker produces ~20 matches/hour
- 3 workers: ~60 matches/hour, ~1440/day — supports ~288 active bots at 10 games/day

**Scaling:** add more spot workers to increase throughput.

---

## 7. Replay System

### 7.1 Replay Data Format

Replays are JSON files optimized for compact storage while supporting full
client-side reconstruction of every game turn.

```json
{
  "version": 1,
  "match_id": "m_7f3a9b2c",
  "date": "2026-03-23T14:30:00Z",
  "players": [
    { "bot_id": "b_4e8c1d2f", "name": "SwarmBot", "owner": "alice" },
    { "bot_id": "b_9a1b3c4d", "name": "HunterBot", "owner": "bob" }
  ],
  "result": {
    "winner": 0,
    "condition": "turn_limit",
    "final_scores": [7, 3],
    "final_energy": [12, 4],
    "final_bots": [18, 6]
  },
  "config": {
    "rows": 60,
    "cols": 60,
    "max_turns": 500,
    "vision_radius2": 49,
    "attack_radius2": 5,
    "spawn_cost": 3,
    "energy_interval": 10
  },
  "map": {
    "walls": [[10,10], [10,11], [10,12]],
    "energy_nodes": [[20,25], [40,35]],
    "cores": [
      { "pos": [5,5], "owner": 0 },
      { "pos": [55,55], "owner": 1 }
    ]
  },
  "turns": [
    {
      "moves": {
        "0": [{"from":[10,15],"dir":"N"},{"from":[12,15],"dir":"E"}],
        "1": [{"from":[50,45],"dir":"S"}]
      },
      "spawns": [[5,5,0]],
      "deaths": [[30,40,1]],
      "captures": [],
      "energy_collected": {"0": [[20,25]]},
      "energy_spawned": [[35,15]],
      "scores": [3, 1]
    }
  ]
}
```

**Size estimate:** a 500-turn, 4-player match with ~50 bots total produces
a replay of ~200–500 KB uncompressed, ~30–80 KB gzipped.

**Optimization:** for very long matches, the `turns` array can use delta
encoding — only recording events that changed from the previous turn.

### 7.2 Storage

Replays are stored in **Cloudflare R2** and served to the browser via R2's
custom domain with zero egress cost. No API intermediary for reads.

**R2 bucket layout** (public-read via custom domain):
```
replays/{match_id}.json.gz           # individual replay files
maps/{map_id}.json                   # map definitions
data/matches/index.json              # paginated match list (last 1000)
data/matches/{match_id}.json         # match metadata (participants, scores)
data/leaderboard.json                # current leaderboard snapshot
data/bots/{bot_id}.json              # per-bot profile (rating history, recent matches)
data/bots/index.json                 # bot directory
data/evolution/lineage.json          # evolution lineage graph
data/evolution/meta.json             # current meta/Nash snapshot
```

**How data flows:**
1. Match worker completes a match → uploads `replay.json.gz` directly to R2
   via S3-compatible API (worker has a scoped R2 API token)
2. Worker POSTs small result metadata to the Cloudflare Worker API endpoint
3. Worker API writes match result to D1
4. Index rebuilder cron (every 2 min) reads new results from D1, rebuilds
   `leaderboard.json`, `bots/*.json`, `matches/index.json`, writes to R2
5. Static site (Pages) fetches these JSON files from R2's custom domain

**Retention:**
- Indefinite for top-100 matches per month
- Older replays pruned after 90 days (metadata in D1 kept)
- Index files are append-with-rotation: `index.json` holds the last 1000;
  older pages at `index-{page}.json`

**R2 free tier usage at this scale:**
- Writes (Class A): ~43K/month (replays + index rebuilds) vs 1M limit
- Reads (Class B): ~30K/month (page views loading JSON) vs 10M limit
- Storage: ~3–5 GB after 90 days (well under 10 GB limit)
- Egress: always free, unlimited

### 7.3 Browser Replay Viewer

The replay viewer is a client-side TypeScript application rendered on
HTML5 Canvas.

**Rendering pipeline:**
1. Fetch `replay.json.gz` from R2 custom domain (zero egress cost; browser
   handles gzip decompression via `Accept-Encoding`)
2. Parse and index: build per-turn game state by replaying events from turn 0
3. Render the current turn to canvas
4. User controls advance/rewind the turn index

No Worker invocations — the viewer is a static Pages page loading a file
directly from R2.

**Visual design:**

| Element | Rendering |
|---------|-----------|
| Grid | Subtle grid lines on dark background |
| Walls | Dark gray filled squares |
| Open tiles | Transparent (background shows through) |
| Energy nodes | Small yellow diamond; pulse animation when energy is present |
| Cores | Large player-colored circle with ring; X overlay when razed |
| Bots | Player-colored filled circles; brief trail showing last move direction |
| Dead bots | Fading red X for one turn |
| Fog of war | Dark semi-transparent overlay on tiles outside selected player's vision |
| Combat | Flash effect on tiles where kills occurred |

**Controls:**

| Control | Function |
|---------|----------|
| Play / Pause | Toggle automatic playback |
| Speed slider | 1x, 2x, 4x, 8x, 16x (turns per second: 2, 4, 8, 16, 32) |
| Turn scrubber | Drag to any turn; displays turn number |
| Perspective dropdown | "All" (omniscient) or per-player fog of war view |
| Zoom | Scroll to zoom; drag to pan |
| Score overlay | Per-player score, energy, bot count — updates each turn |
| Minimap | Small overview of full grid in corner (for large maps) |

**Shareable URLs:** `https://aicodebattle.com/replay/{match_id}` — the
replay viewer is the landing page for any match. No login required to watch.

---

## 8. Web Platform

The web-facing platform runs entirely on Cloudflare's free tier: **Pages**
for the static site, a **Worker** for the API and scheduling logic, **D1**
for the database, and **R2** for file storage.

### 8.1 Cloudflare Pages (Static Site)

The website is a static SPA deployed to Cloudflare Pages. Every page that
shows dynamic content fetches pre-built JSON files from R2 and renders
client-side.

```
/                          → Landing page, featured replays, leaderboard summary
/leaderboard               → Full leaderboard (fetches leaderboard.json from R2)
/matches                   → Match history (fetches matches/index.json from R2)
/replay/{match_id}         → Replay viewer (fetches replay .json.gz from R2)
/bot/{bot_id}              → Bot profile (fetches bots/{bot_id}.json from R2)
/evolution                 → Evolution dashboard (fetches evolution/*.json from R2)
/register                  → Bot registration form (submits to Worker API)
/docs                      → Protocol spec, starter kit links, getting started
```

**Build:** Vite + TypeScript, deployed via `wrangler pages deploy` or git
integration. 500 builds/month on the free tier (ample for daily deploys).
No build-time data fetching — all data loaded at runtime.

**Data loading pattern:**
```js
const R2_BASE = 'https://data.aicodebattle.com'
const data = await fetch(`${R2_BASE}/data/leaderboard.json`)
const leaderboard = await data.json()
// render client-side
```

R2 serves these files via custom domain with zero egress cost. Stale data
is acceptable — JSON indexes are rebuilt every 2 minutes by the Worker cron.
No real-time push. Visitors see data that is at most ~2 minutes old.

### 8.2 Cloudflare Worker (API + Scheduling)

A single Worker (`acb-api`) handles all server-side logic. It has D1 and R2
bindings.

**API endpoints (HTTP routes):**

```
POST /api/register         → register a new bot
POST /api/rotate-key       → rotate a bot's shared secret
GET  /api/status/{bot_id}  → check bot health status
GET  /api/jobs/next         → worker claims next pending match job (authenticated)
POST /api/jobs/{id}/result  → worker submits match result metadata (authenticated)
```

**Cron triggers (5 available on free tier):**

| Cron | Interval | What It Does |
|------|----------|--------------|
| Matchmaker | Every 1 min | Queries active bots from D1, computes pairings, inserts job rows |
| Index rebuilder | Every 2 min | Reads new results from D1, rebuilds leaderboard.json + bot profiles + match index, writes to R2 |
| Health checker | Every 15 min | Pings each active bot's `/health` endpoint, updates status in D1 |
| Stale job reaper | Every 5 min | Marks jobs running >15 min as abandoned, resets to pending |
| (reserved) | — | Available for evolution pipeline trigger |

**CPU time budget (10ms free tier):**

All D1 queries, R2 writes, and `fetch()` calls are I/O — they don't count
against the 10ms CPU limit. Only JavaScript computation counts. At modest
scale (~50 bots):
- Matchmaking sort + pairing: <1ms CPU
- JSON serialization for index rebuilds: <2ms CPU
- HMAC computation for registration: <1ms CPU
- All cron triggers fit comfortably within 10ms

**Worker authentication for Rackspace endpoints:**

The `/api/jobs/*` endpoints are called by Rackspace match workers. They
authenticate with a static API key passed in the `Authorization` header.
The key is stored in the Worker's environment variables (Cloudflare encrypted
secrets). This prevents unauthorized job claims or result injection.

### 8.3 Cloudflare D1 (Database)

D1 is a serverless SQLite database accessible from the Worker.

**Schema:**

```sql
CREATE TABLE bots (
    bot_id        TEXT PRIMARY KEY,
    name          TEXT UNIQUE NOT NULL,
    owner         TEXT NOT NULL,
    endpoint_url  TEXT NOT NULL,
    shared_secret TEXT NOT NULL,  -- encrypted, see §4.4
    status        TEXT NOT NULL DEFAULT 'pending',
    rating_mu     REAL NOT NULL DEFAULT 1500.0,
    rating_phi    REAL NOT NULL DEFAULT 350.0,
    rating_sigma  REAL NOT NULL DEFAULT 0.06,
    evolved       INTEGER NOT NULL DEFAULT 0,
    island        TEXT,
    generation    INTEGER,
    description   TEXT,
    created_at    TEXT NOT NULL,
    last_active   TEXT
);

CREATE TABLE matches (
    match_id      TEXT PRIMARY KEY,
    map_id        TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    winner        INTEGER,
    condition     TEXT,
    turn_count    INTEGER,
    scores_json   TEXT,
    created_at    TEXT NOT NULL,
    completed_at  TEXT
);

CREATE TABLE match_participants (
    match_id      TEXT NOT NULL,
    bot_id        TEXT NOT NULL,
    player_slot   INTEGER NOT NULL,
    score         INTEGER,
    status        TEXT,
    PRIMARY KEY (match_id, bot_id)
);

CREATE TABLE jobs (
    job_id        TEXT PRIMARY KEY,
    match_id      TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    config_json   TEXT NOT NULL,
    claimed_at    TEXT,
    completed_at  TEXT
);

CREATE TABLE rating_history (
    bot_id        TEXT NOT NULL,
    match_id      TEXT NOT NULL,
    rating        REAL NOT NULL,
    recorded_at   TEXT NOT NULL
);
```

**Free tier usage at scale:**
- Writes: ~1,500/day (match results + job state changes + ratings) vs 100K limit
- Reads: ~50K/day (matchmaking queries + index rebuilds + API lookups) vs 5M limit
- Storage: <100 MB after months of operation vs 5 GB limit

### 8.4 Bot Registration

**Registration flow:**

1. Participant fills out the form on the static site (`/register`)
2. Form POSTs to the Worker: `POST /api/register`
   - **Bot name** (unique, alphanumeric + hyphens, 3–32 chars)
   - **Endpoint URL** (HTTPS required for competitive; HTTP allowed for dev)
   - **Owner name** (free text, shown on leaderboard)
   - **Description** (optional)
3. Worker generates:
   - `bot_id`: `b_` + 8 hex chars (from `crypto.randomUUID()`)
   - `shared_secret`: 256-bit random, hex-encoded (`crypto.getRandomValues()`)
4. Worker performs a **health check**: `fetch(endpoint_url + '/health')`
   - Must return 200 within 5 seconds
5. Worker performs a **protocol test**: sends mock game state to
   `POST {endpoint_url}/turn` with valid HMAC
   - Must return valid moves JSON within 3 seconds
6. Worker inserts bot record into D1
7. Worker returns `bot_id` and `shared_secret` to the participant
   (displayed once — they must save it)

**No user accounts.** Registration is bot-level. The owner name is
self-reported. The shared secret is the only authentication — whoever has
it can rotate the key or retire the bot. No OAuth, no sessions, no
password storage.

**Bot status lifecycle:**
```
PENDING → ACTIVE → INACTIVE (health check failed)
                  → RETIRED (by owner via /api/rotate-key with retire flag)
```

Only `ACTIVE` bots participate in matchmaking. The health checker cron pings
each active bot every 15 min. Three consecutive failures → `INACTIVE`. Bots
automatically return to `ACTIVE` when health checks pass again.

### 8.5 Leaderboard

The leaderboard is a **JSON file** in R2 (`data/leaderboard.json`) rebuilt
by the index rebuilder cron every 2 minutes.

```json
{
  "updated_at": "2026-03-23T14:35:00Z",
  "entries": [
    {
      "rank": 1,
      "bot_id": "b_4e8c1d2f",
      "name": "SwarmBot",
      "owner": "alice",
      "rating": 1820,
      "games": 142,
      "wins": 98,
      "losses": 40,
      "draws": 4,
      "evolved": false,
      "last_match": "2026-03-23T14:30:00Z"
    }
  ]
}
```

The static site fetches this file directly from R2 (no Worker invocation).
Client-side sorting and filtering (by player count tier, time range,
human-only vs all). Auto-refresh every 60 seconds. Public — no login.

### 8.6 Match History & Profiles

**Bot profile** (`/bot/{bot_id}`) — fetches `data/bots/{bot_id}.json` from R2:
- Current rating + rating history (array of `[timestamp, rating]` pairs
  rendered as a chart client-side)
- Recent matches (last 50) with links to replay viewer
- Win/loss/draw breakdown
- Bot description, owner, registration date
- If evolved: lineage, generation, island

**Match list** (`/matches`) — fetches `data/matches/index.json` from R2:
- Paginated list of recent matches
- Each entry: match_id, participants, scores, date, link to replay

**Match detail** (`/replay/{match_id}`):
- Fetches `data/matches/{match_id}.json` from R2 for metadata
- Fetches `replays/{match_id}.json.gz` from R2 for the replay
- Embedded replay viewer (auto-plays)
- Score breakdown, participants, match duration

---

## 9. Deployment & Infrastructure

### 9.1 Design Principles

The platform is split across two providers based on their strengths:

- **Cloudflare (free tier)** handles everything web-facing: the site, the
  API, the database, file storage, and scheduling. This tier has zero cost,
  zero ops burden (no servers to maintain), and global edge distribution.
- **Rackspace Spot** handles everything compute-heavy: match execution, bot
  hosting, and the evolution pipeline. These workloads are stateless and
  interruptible — perfect for spot pricing.

All durable state lives in Cloudflare (D1 + R2). Rackspace instances are
fully ephemeral — they can be reclaimed at any time with zero data loss.

### 9.2 Cloudflare Tier (Free Plan)

| Service | Usage | Free Limit | Headroom |
|---------|-------|------------|----------|
| **Pages** | ~1K views/day | Unlimited bandwidth + requests | Unlimited |
| **Workers** | ~5K requests/day (API + crons) | 100K requests/day | 95% |
| **Workers CPU** | <5ms per invocation | 10ms per invocation | 50% |
| **R2 storage** | ~3–5 GB | 10 GB | 50–70% |
| **R2 Class A** (writes) | ~43K/month | 1M/month | 96% |
| **R2 Class B** (reads) | ~30K/month | 10M/month | 99.7% |
| **R2 egress** | Unlimited | Unlimited (always free) | — |
| **D1 writes** | ~1.5K/day | 100K/day | 98.5% |
| **D1 reads** | ~50K/day | 5M/day | 99% |
| **D1 storage** | <100 MB | 5 GB | 98% |
| **Cron triggers** | 4 used | 5 per account | 1 spare |

**Cloudflare deployment:**
```
Cloudflare Account:
├── Pages project: aicodebattle.com (static site)
├── Worker: acb-api
│   ├── Routes: api.aicodebattle.com/*
│   ├── Crons: matchmaker (1m), indexer (2m), health (15m), reaper (5m)
│   ├── D1 binding: ACB_DB
│   └── R2 binding: ACB_DATA
├── R2 bucket: acb-data
│   └── Custom domain: data.aicodebattle.com (public read)
└── D1 database: acb-db
```

**What Cloudflare handles:**
- TLS termination (automatic, free)
- DNS (Cloudflare nameservers)
- CDN for static assets (Pages, global edge)
- DDoS protection (free tier includes basic)
- File serving with zero egress (R2)
- Database with automatic backups (D1, 7-day Time Travel)

### 9.3 Rackspace Spot Tier

Everything on Rackspace is stateless and interruptible. All durable state
is in Cloudflare (D1 + R2).

**Container architecture:**

| Image | Base | Purpose | Instances |
|-------|------|---------|-----------|
| `acb-worker` | Go binary on Alpine | Match execution | 1–10 (spot) |
| `acb-evolver` | Go binary on Alpine | Evolution pipeline | 1 (spot) |
| `acb-strategy-random` | Python 3.13 slim | RandomBot | 1 |
| `acb-strategy-gatherer` | Go on Alpine | GathererBot | 1 |
| `acb-strategy-rusher` | Rust on Alpine | RusherBot | 1 |
| `acb-strategy-guardian` | PHP 8.4 CLI Alpine | GuardianBot | 1 |
| `acb-strategy-swarm` | Node 22 Alpine | SwarmBot (TypeScript) | 1 |
| `acb-strategy-hunter` | Temurin 21 JRE Alpine | HunterBot (Java) | 1 |
| `acb-evolved-*` | Varies by language | LLM-generated bots | 0–50 |

**Deployment layout:**
```
Spot instance A (4 vCPU, 8 GB RAM, "bot host"):
├── acb-strategy-* (all 6 built-in bots, ~1 GB total)
└── acb-evolved-* (0–50 evolved bots, dynamic)

Spot instance B (2 vCPU, 4 GB RAM, "worker"):
└── acb-worker (runs 1 match at a time)

Spot instance C (2 vCPU, 4 GB RAM, "worker"):
└── acb-worker (runs 1 match at a time)

Spot instance D (4 vCPU, 8 GB RAM, "evolver"):
└── acb-evolver (LLM pipeline, sandbox, evaluation)
```

### 9.4 Match Job Coordination

Workers coordinate with the Cloudflare Worker API. The Worker + D1 are the
single point of coordination.

**Job flow:**
1. Matchmaker cron creates jobs in D1 (`status: 'pending'`)
2. Rackspace worker polls: `GET api.aicodebattle.com/api/jobs/next`
   (authenticated with API key)
3. Worker API atomically claims the job (D1 transaction: set `status: 'running'`,
   record `claimed_at`), returns job config JSON including:
   - Map data (or map_id to fetch from R2)
   - Bot endpoints + shared secrets for HMAC signing
   - Match config (turns, radii, etc.)
4. Rackspace worker executes the full match (500 turns, HTTP calls to bots)
5. Worker uploads replay: `PUT` directly to R2 via S3-compatible API
   (scoped R2 API token, `PutObject` only on `replays/` prefix)
6. Worker submits result metadata:
   `POST api.aicodebattle.com/api/jobs/{id}/result`
   - Small JSON body: scores, winner, turn count, condition
7. Worker API writes result to D1, marks job `completed`
8. Index rebuilder cron (next 2-min cycle) reads new results, rebuilds
   leaderboard.json + bot profiles + match index, writes to R2

**Stale job recovery:**
- Reaper cron checks D1 every 5 minutes for jobs `running` >15 minutes
- Assumed abandoned (spot instance reclaimed)
- Reset to `pending` for re-execution

### 9.5 Spot Reclamation Behavior

**If bot-host spot instance is reclaimed:**
- All built-in + evolved bots go offline
- Health checker cron detects failures, marks bots `INACTIVE` in D1
- Matchmaker skips inactive bots — only external bots can play
- When a new bot-host instance starts, bots come back online, health checks
  pass, matchmaker resumes including them
- Matches in progress where a bot disappeared: that bot times out on each
  turn, its units hold position, it effectively loses

**If all worker instances are reclaimed:**
- Jobs accumulate as `pending` in D1
- The website, leaderboard, and replays remain fully functional (Cloudflare)
- When workers return, they drain the queue

**If everything on Rackspace is gone simultaneously:**
- Visitors see a working website with stale-but-valid data
- No matches run, no bots respond to health checks
- All bots eventually marked inactive
- Full recovery when any Rackspace instances return

The user-facing experience degrades gracefully because all web infrastructure
is on Cloudflare, not Rackspace.

### 9.6 Networking & Security

**External traffic (Cloudflare):**
- `aicodebattle.com` → Cloudflare Pages (static site)
- `data.aicodebattle.com` → R2 public bucket (JSON data + replays)
- `api.aicodebattle.com` → Cloudflare Worker (API endpoints)
- TLS, CDN, DDoS protection all handled by Cloudflare automatically

**Rackspace → Cloudflare:**
- Workers → Worker API: HTTPS to `api.aicodebattle.com` (authenticated with
  API key in `Authorization` header)
- Workers → R2: HTTPS via S3-compatible API (scoped R2 API token)

**Rackspace → Bots (during matches):**
- Workers → built-in/evolved bots: HTTP within Rackspace private network
  (or Tailscale if across instances)
- Workers → external participant bots: outbound HTTPS to registered URLs

**Security boundaries:**
- The game engine (workers) never executes bot code — HTTP only
- All bot responses are schema-validated before processing
- HMAC authentication prevents request/response forgery
- Worker API endpoints authenticated with API key (job coordination)
- R2 API token scoped to `PutObject` on `replays/` prefix only
- Registration endpoint validates bot URLs (no internal IPs, no private ranges)
- D1 is only accessible from the bound Worker (not publicly queryable)
- R2 data bucket is public-read — contains no secrets

### 9.7 Cost Model

| Component | Provider | Cost |
|-----------|----------|------|
| Pages + Worker + D1 + R2 | Cloudflare | **$0/mo** (free tier) |
| Bot host (×1 avg) | Rackspace Spot | ~$10–20/mo |
| Match workers (×2–3 avg) | Rackspace Spot | ~$15–30/mo |
| Evolver (×1) | Rackspace Spot | ~$10–20/mo |
| **Infrastructure total** | | **~$35–70/mo** |
| LLM API (evolution pipeline) | Various | ~$150–600/mo |

Compared to the previous architecture ($65–110/mo), moving the web tier to
Cloudflare saves ~$30–40/mo (the stable instance) and eliminates all web
infrastructure ops (no Nginx config, no TLS certs, no volume management,
no backup scripts for the data directory).

### 9.8 Monitoring

| Signal | Method | Alert |
|--------|--------|-------|
| Site up | Cloudflare analytics (built-in) | Auto |
| Worker errors | Cloudflare Worker analytics | Error rate >5% |
| D1 usage | Cloudflare dashboard | Approaching free tier limits |
| R2 storage | Cloudflare dashboard | >8 GB (approaching 10 GB) |
| Active Rackspace workers | Worker API tracks last job claim time | No claim in >30 min |
| Match throughput | D1 query: completions per hour | <10/hour for >1 hour |
| Bot health failures | D1 query in health checker cron | >50% failing |
| Stale jobs | Reaper cron count | >10 stale in a cycle |

Alerts via Worker → webhook to Discord/Slack. No external monitoring
service needed — Cloudflare provides built-in analytics for Pages, Workers,
R2, and D1.

---

## 10. LLM-Driven Bot Evolution

The platform includes an autonomous evolution pipeline that uses LLMs to
continuously generate, evaluate, and promote new bot strategies. Evolved bots
compete on the same ladder as human-written bots — visitors see an ever-changing
meta where strategies emerge, dominate, and get countered without human
intervention.

### 10.1 Architecture Overview

The evolution system combines two proven approaches:

- **FunSearch/AlphaEvolve island model** — maintains diverse, independent
  populations of bot code that cross-pollinate. Prevents premature convergence
  to a single dominant strategy.
- **LLM-PSRO (Policy Space Response Oracle)** — uses Nash equilibrium as the
  promotion gate. A new bot must beat the optimal mixed strategy over the
  current population, not just one specific opponent. This provides
  mathematically grounded regression prevention.

```
┌──────────────────────────────────────────────────────────┐
│                     Programs Database                     │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌──────────┐ │
│  │  Island 1  │ │  Island 2  │ │  Island 3  │ │ Island 4 │ │
│  │  (Python)  │ │  (Go)      │ │  (Rust)    │ │ (mixed)  │ │
│  │  pop: 20   │ │  pop: 20   │ │  pop: 20   │ │ pop: 20  │ │
│  └───────────┘ └───────────┘ └───────────┘ └──────────┘ │
└──────────────────────────┬───────────────────────────────┘
                           │
             sample 2-3 parents + match replays
                           │
                ┌──────────▼───────────┐
                │    Prompt Builder     │
                │  • Parent source code │
                │  • Recent loss replay │
                │  • Win/loss analysis  │
                │  • Current meta desc  │
                │  • "Beat this mix"    │
                └──────────┬───────────┘
                           │
                ┌──────────▼───────────┐
                │    LLM Ensemble       │
                │  • Fast model (×8)    │
                │    exploration/breadth │
                │  • Strong model (×2)  │
                │    exploitation/depth  │
                └──────────┬───────────┘
                           │ generates candidate bot code
                ┌──────────▼───────────┐
                │    Validation Gate    │
                │  1. Syntax check      │
                │  2. Compile/lint      │
                │  3. Schema test       │
                │  4. Sandbox smoke run │
                └──────────┬───────────┘
                           │ passes validation
                ┌──────────▼───────────┐
                │    Evaluation Arena   │
                │  • 10 matches vs      │
                │    population sample  │
                │  • Compute win rate   │
                │  • Build payoff row   │
                └──────────┬───────────┘
                           │
                ┌──────────▼───────────┐
                │    Promotion Gate     │
                │  • Compute Nash eq.   │
                │    over population    │
                │  • Candidate must     │
                │    beat Nash mixture  │
                │  • Or: fill empty     │
                │    MAP-Elites niche   │
                └──────────┬───────────┘
                           │ promoted
                ┌──────────▼───────────┐
                │    Deploy & Register  │
                │  • Build container    │
                │  • Push to registry   │
                │  • Register on ladder │
                │  • Enter island DB    │
                └──────────────────────┘
```

### 10.2 Programs Database (Island Model)

The programs database stores all evolved bot code, organized into **islands**
that evolve independently to maintain strategic diversity.

**Island structure:**
- **4 islands**, one per primary language (Python, Go, Rust, mixed)
- Each island holds up to **20 programs** ranked by fitness
- Programs are clustered by **behavior signature** — a vector of outcomes
  across a fixed set of benchmark matches (e.g., win/loss/score against each
  of the 6 built-in strategy bots)
- Sampling favors high-scoring clusters; within a cluster, favors shorter/simpler
  code (Occam pressure prevents bloat)

**Cross-pollination:**
- Every 50 generations, the top program from each island is copied to a random
  other island (translated to that island's language by the LLM if needed)
- This spreads successful strategies across languages without homogenizing
  the populations

**Behavior dimensions for MAP-Elites diversity:**

| Dimension | Low | High |
|-----------|-----|------|
| Aggression | Never enters enemy territory | Rushes enemy core immediately |
| Economy | Ignores energy entirely | Maximizes energy per turn |
| Exploration | Stays near core | Covers >80% of visible map |
| Formation | Units always scattered | Units always in tight groups |

Each dimension is binned into 3 levels, creating a 3⁴ = 81-cell behavior grid.
The database tries to fill every cell with the highest-scoring bot for that
behavioral profile. This ensures the evolved population contains turtles,
rushers, economists, swarmers, and everything in between — not just one
dominant archetype.

### 10.3 Prompt Construction

The LLM prompt is the critical interface between match performance data and
code generation. Each prompt is constructed from:

**Parent code (2–3 programs):**
- Sampled from the island's high-scoring clusters
- Included as full source code with inline comments noting their rating and
  behavioral profile
- The LLM sees concrete working examples, not abstract descriptions

**Match analysis (from recent losses):**
- The replay of the parent's worst recent loss is summarized:
  - Turn-by-turn narrative of critical moments (when the bot lost a formation,
    missed energy, walked into a trap)
  - Final score breakdown
  - Opponent's apparent strategy (inferred from replay)
- This gives the LLM specific failure modes to address

**Meta description:**
- Current Nash equilibrium mixture over the population (e.g., "the optimal
  counter-strategy is 40% swarm, 30% hunter, 30% gatherer")
- The candidate should beat this mixture, not just one opponent
- Weaknesses in the current meta are highlighted (e.g., "no bot currently
  exploits the east-side energy clusters on 4-player maps")

**Constraints:**
- Target language for this island
- Must implement the HTTP bot interface (`POST /turn`, `GET /health`)
- Must include HMAC verification
- Maximum source code size (10 KB — prevents bloat)
- Must respond within 3-second timeout with reasonable compute

**Prompt template (simplified):**

```
You are evolving a competitive bot for AI Code Battle, a grid-based
strategy game. Your bot must be an HTTP server that receives game state
and returns moves.

## Game Rules
{game_rules_summary}

## HTTP Protocol
{protocol_spec}

## Parent Bots (these work — improve on them)

### Parent A — Rating: 1650, Style: aggressive-gatherer
```{language}
{parent_a_source}
```

### Parent B — Rating: 1580, Style: defensive-swarm
```{language}
{parent_b_source}
```

## Parent A's Worst Loss (Replay Summary)
{replay_analysis}

## Current Meta
The Nash equilibrium mixture is:
{nash_mixture_description}

Known weaknesses in current population:
{meta_weaknesses}

## Your Task
Write a new bot in {language} that:
1. Addresses Parent A's failure mode shown in the replay
2. Incorporates Parent B's strongest tactical element
3. Can beat the Nash mixture described above
4. Fits in a single file under 10 KB

Return the complete source code.
```

### 10.4 LLM Ensemble

The evolution system uses two model tiers, inspired by AlphaEvolve:

**Exploration tier (fast model, 80% of generations):**
- Cheaper, faster model (e.g., Claude Haiku, GPT-4o-mini, Gemini Flash)
- Generates 8 candidates per cycle
- High temperature (0.9–1.0) for diversity
- Purpose: broad search across strategy space; most candidates will fail,
  but occasional novel approaches emerge

**Exploitation tier (strong model, 20% of generations):**
- More capable model (e.g., Claude Sonnet/Opus, GPT-4o, Gemini Pro)
- Generates 2 candidates per cycle
- Lower temperature (0.3–0.5) for refinement
- Purpose: take the best current strategies and make them better; refine
  tactical details, optimize pathfinding, improve edge-case handling

**Total throughput:** 10 candidates per evolution cycle. With a cycle time
of ~15 minutes (generation + validation + 10 evaluation matches), the system
produces ~96 candidates/day, of which ~5–15% pass the promotion gate.

### 10.5 Validation Pipeline

Every LLM-generated candidate passes through a multi-stage validation
before it touches the evaluation arena:

**Stage 1: Syntax & Compilation**
- Language-specific: `python -m py_compile`, `go build`, `cargo check`,
  `php -l`, `tsc --noEmit`, `javac`
- Reject: syntax errors, missing imports, type errors
- ~40% of candidates fail here (expected — LLMs produce broken code often)

**Stage 2: Schema Compliance**
- Start the bot container
- Send a mock turn-0 game state to `POST /turn`
- Verify response parses as valid moves JSON
- Verify `GET /health` returns 200
- Verify HMAC signature is present and valid
- Reject: bots that can't speak the protocol
- ~20% of remaining candidates fail here

**Stage 3: Sandbox Smoke Test**
- Run a 50-turn match against RandomBot inside nsjail
- Verify the bot doesn't crash, timeout on every turn, or produce
  identical moves every turn (degenerate)
- Verify the bot scores ≥ 0 (doesn't actively self-destruct)
- Reject: bots that crash, hang, or do nothing
- ~10% of remaining candidates fail here

**Net yield:** ~30–40% of generated candidates survive to the evaluation
arena. At 10 candidates/cycle, that's 3–4 evaluated candidates per cycle.

**Sandboxing (nsjail):**
- All LLM-generated code executes inside nsjail containers
- No network access (game state is piped via the engine, not fetched)
- No filesystem access beyond the bot's own directory
- CPU time limit: 5 seconds per turn (generous; 3-second HTTP timeout is
  enforced by the engine separately)
- Memory limit: 512 MB
- Process limit: 10 (prevents fork bombs)

### 10.6 Evaluation Arena

Candidates that pass validation enter a mini-tournament:

**Evaluation protocol:**
1. Play 10 matches against opponents sampled from the current population:
   - 2 matches vs each of the 3 closest-rated bots in the candidate's island
   - 2 matches vs a random bot from a different island
   - 2 matches vs the current island champion
2. Record results → compute win rate and per-opponent scores
3. Build the candidate's **payoff row** in the population's payoff matrix

**Match configuration:**
- 2-player matches only (faster evaluation; multi-player tested post-promotion)
- Standard maps, standard timeout
- Evaluation matches are **not** counted toward ladder ratings (they use a
  separate evaluation queue)

### 10.7 Promotion Gate (Nash Equilibrium / PSRO)

The promotion gate determines whether a candidate enters the population and
gets deployed to the ladder.

**Primary gate: Nash equilibrium (LLM-PSRO)**

1. Compute the Nash equilibrium mixture σ* over the current island population
   using the existing payoff matrix
2. Compute the candidate's expected payoff against σ* (using the payoff row
   from the evaluation arena)
3. **Promote if** the candidate's expected payoff against σ* is positive
   (i.e., the candidate beats the current optimal mixed strategy)
4. If promoted, add the candidate to the island population, recompute Nash

This ensures the population's game-theoretic strength monotonically increases.
A bot that just exploits one opponent's weakness but loses to the overall mix
is rejected.

**Secondary gate: MAP-Elites niche filling**

Even if a candidate doesn't beat the Nash mixture, it may fill an **empty
cell** in the behavior grid (section 10.2). If the candidate's behavior
signature maps to an unoccupied cell, it is promoted anyway. This maintains
strategic diversity even when the Nash gate is tight.

**Replacement policy:**
- If the candidate's behavior cell already has an occupant, the candidate
  replaces it only if the candidate's fitness is higher
- Island population size is capped at 20; if full and no cell is improved,
  the candidate is discarded
- The worst-performing program in an over-populated cluster is evicted first

### 10.8 Deployment Pipeline

Promoted bots are automatically containerized and registered on the ladder:

**Build:**
1. Write the bot's source code to a temporary directory
2. Copy the language-appropriate Dockerfile from the starter kit template
3. Build the container image: `acb-evolved-{island}-{generation}-{hash}`
4. Push to container registry

**Register:**
1. Generate a new `bot_id` and `shared_secret`
2. Deploy the container to the always-on strategy bot instance pool
3. Register the bot via the platform API with metadata:
   - `owner`: "evolution-system" (system account)
   - `name`: auto-generated (e.g., `evo-py-g42-7f3a`)
   - `description`: auto-generated from the LLM's strategy summary
   - `lineage`: parent bot IDs + generation number
   - `island`: which island produced it
4. Health check → mark ACTIVE → enters matchmaking

**Lifecycle management:**
- Evolved bots are tagged with `evolved: true` in the database
- The evolution system tracks the **lineage** of every bot (parent IDs,
  generation number, island of origin)
- Evolved bots that drop below rating 800 (bottom 10% of ladder) for 7
  consecutive days are **retired** automatically to prevent population bloat
- Maximum active evolved bots: 50 (configurable). When the cap is reached,
  the lowest-rated evolved bot is retired before a new one is promoted.
- Retired evolved bots remain in the programs database for future sampling
  (their code may still contain useful tactics) but are removed from the
  ladder and their containers are stopped

### 10.9 Evolution Cycle Timing

| Phase | Duration | Notes |
|-------|----------|-------|
| Parent sampling + prompt construction | ~10 seconds | CPU-bound, fast |
| LLM generation (10 candidates) | ~30–60 seconds | Parallel across ensemble |
| Validation (syntax, schema, smoke) | ~2 minutes | Parallel per candidate |
| Evaluation arena (10 matches) | ~10 minutes | Sequential matches, 3s/turn × 500 turns worst case; but most against weak bots end faster |
| Nash computation + promotion | ~5 seconds | Small matrix, fast |
| Container build + deploy | ~2 minutes | Docker build + push |
| **Total cycle time** | **~15 minutes** | |

**Daily output:** ~96 candidates generated, ~10–15 promoted, ~5–10 survive
on the ladder after the 7-day retirement window.

### 10.10 Evolution Dashboard

The web platform includes a dedicated evolution section visible to all visitors:

**Lineage viewer:**
- Interactive tree/graph showing the ancestry of every evolved bot
- Click a node to see the bot's source code, rating history, and match record
- Color-coded by island/language
- Animated timeline showing which bots were active at which point

**Meta tracker:**
- Current Nash equilibrium mixture visualization (pie chart of strategy archetypes)
- How the meta has shifted over time (stacked area chart)
- Which behavioral niches are filled vs empty in the MAP-Elites grid

**Generation log:**
- Stream of recent evolution attempts: generated, validated, evaluated, promoted/rejected
- For each attempt: the prompt summary, the LLM's output, validation results,
  evaluation match results, and promotion decision with reasoning

**Statistics:**
- Total generations run, candidates generated, promotion rate
- Average rating of evolved bots vs human-written bots over time
- Island diversity metrics (how different are the islands from each other)

### 10.11 Separation from Human Ladder

Evolved bots compete on the **same ladder** as human-written bots — there is
no separate tier. This is a deliberate design choice:

**Why mix them:**
- The entire point is to see if LLM-evolved strategies can compete with or
  surpass human-written ones
- Humans can study evolved bot replays and learn new tactics, then write
  better bots that push the meta further — a human-AI co-evolution dynamic
- Separate ladders would remove the competitive pressure that drives evolution

**Identification:**
- Evolved bots are clearly tagged on the leaderboard (`[EVO]` prefix or badge)
- Their lineage and source code are publicly viewable (transparency)
- Human participants can opt to filter the leaderboard to show human-only rankings
- Match history shows whether opponents were evolved or human-written

**Fair play:**
- Evolved bots follow the same rules: same timeout, same schema, same HMAC
- No special treatment in matchmaking — rated and matched identically
- The evolution system is rate-limited (max 50 active evolved bots) to prevent
  flooding the ladder

---

## 11. Implementation Phases

### Phase 1: Core Engine (Foundation)

Build the game simulation as a standalone Go library with a CLI runner.

**Deliverables:**
- `engine/` package: grid, bots, energy, combat, fog of war, turn execution
- `cmd/acb-local/` CLI: run a match between two local bot processes
  (stdin/stdout for dev convenience) and output a replay JSON file
- Replay JSON writer
- Comprehensive unit tests for combat resolution, fog of war, wrapping,
  collision, scoring, endgame conditions
- Map generation tool: `cmd/acb-mapgen/`

**Exit criteria:** can run a complete 500-turn match between two bots locally
and produce a valid replay file.

### Phase 2: HTTP Protocol & Strategy Bots

**Deliverables:**
- HTTP bot interface in the engine (replaces stdin/stdout for production)
- HMAC signing and verification library (Go, reusable by GathererBot)
- GathererBot (Go) and RandomBot (Python) — validate the protocol works
  across languages before building the remaining four
- RusherBot (Rust), GuardianBot (PHP), SwarmBot (TypeScript), HunterBot (Java)
- All 6 bots containerized with language-appropriate Dockerfiles
- Starter kit template repos for each language (fork-ready)
- Integration test: engine runs a full match between bots in different
  languages over HTTP

**Exit criteria:** can run a complete match between any two strategy bot
containers (in different languages) over HTTP, with HMAC authentication,
producing a valid replay.

### Phase 3: Replay Viewer

**Deliverables:**
- TypeScript Canvas-based replay viewer
- Play/pause, scrub, speed control
- Fog of war perspective toggle
- Score overlay
- Loads replay JSON from local file or URL

**Exit criteria:** can open a replay file in a browser and watch a complete
match with all visual elements rendering correctly.

### Phase 4: Match Orchestration

**Deliverables:**
- Cloudflare Worker (`acb-api`): job coordination endpoints
  (`/api/jobs/next`, `/api/jobs/{id}/result`), authenticated with API key
- D1 schema: `bots`, `matches`, `match_participants`, `jobs`,
  `rating_history` tables
- Worker cron: matchmaker (1 min), stale job reaper (5 min)
- Worker cron: index rebuilder (2 min) — reads D1, writes leaderboard.json +
  bot profiles + match index to R2
- Match worker container (`acb-worker`): claims jobs from Worker API, runs
  matches, uploads replays to R2 via S3 API, POSTs results to Worker API
- Glicko-2 rating update logic in the Worker (runs on result submission)

**Exit criteria:** matchmaker cron creates jobs in D1, Rackspace workers claim
and execute them, replays land in R2, results flow into D1, ratings update,
and leaderboard.json rebuilds automatically. System recovers from worker
disappearance via the stale job reaper.

### Phase 5: Web Platform

**Deliverables:**
- Cloudflare Pages static site: leaderboard, match history, bot profiles,
  replay viewer, registration form, docs/getting-started page
- Worker API: registration endpoints (`/api/register`, `/api/rotate-key`,
  `/api/status/{id}`)
- Worker cron: health checker (15 min) — pings bot endpoints, updates D1
- R2 bucket with custom domain for public-read data access
- All pages load data by fetching JSON from R2 — no Worker invocations
  for page views

**Exit criteria:** a participant can register a bot via the web form, the
bot appears on the leaderboard after matches complete, and anyone can browse
matches and watch replays — all served from Cloudflare free tier.

### Phase 6: Deployment & Production

**Deliverables:**
- Cloudflare: Pages project, Worker deployed via Wrangler, D1 database
  created, R2 bucket with custom domain, DNS configured
- Rackspace Spot: match worker containers pulling jobs from Cloudflare
  Worker API, bot-host container running all strategy bots
- R2 API token (scoped) distributed to Rackspace workers
- Worker API key distributed to Rackspace workers
- Monitoring: Cloudflare analytics + Worker-based alerting webhooks

**Exit criteria:** platform is publicly accessible on Cloudflare (zero
infrastructure cost), matches run on Rackspace Spot, the site remains fully
functional when all Rackspace instances are reclaimed, and external
participants can register and play.

### Phase 7: LLM-Driven Evolution

**Deliverables:**
- Programs database with island model (4 islands, MAP-Elites behavior grid)
- Prompt builder: parent sampling, replay analysis, meta description
- LLM ensemble integration (fast + strong model tiers)
- Validation pipeline: syntax → schema → sandbox smoke test (nsjail)
- Evaluation arena: 10-match mini-tournament per candidate
- Promotion gate: Nash equilibrium computation (PSRO) + MAP-Elites niche fill
- Automated container build + deploy + register pipeline for promoted bots
- Retirement policy: auto-retire low-rated evolved bots, enforce population cap
- Evolution dashboard: lineage viewer, meta tracker, generation log
- Seed the programs database with the 6 built-in strategy bots as initial
  population

**Exit criteria:** evolution system runs autonomously — generates candidates,
validates, evaluates, promotes, deploys, and retires bots without human
intervention. At least one evolved bot reaches the top 50% of the ladder
within the first week of operation.
