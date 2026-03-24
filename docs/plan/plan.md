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

### Phase 8: Enhanced Features

**Deliverables:**
- WASM game engine build (`GOOS=js GOARCH=wasm`) with `loadState()`,
  `step()`, and `runMatch()` API for browser use
- WASM bot interface spec: `init()`, `compute_moves()`, `free_result()`
  exports for bot-to-engine communication
- Pre-compiled WASM builds of the 6 built-in strategy bots (Go/Rust/TS
  natively; PHP/Java reimplemented in Go for WASM)
- In-browser sandbox: Monaco editor (TS quick-start) + WASM upload mode +
  opponent selector + replay viewer integration
- Win probability computation in the match worker (Monte Carlo rollout) +
  critical moments detector + replay viewer sparkline graph
- Replay enrichment pipeline: selective AI commentary for featured matches
- Clip maker: GIF + MP4 export with 5 social media format presets
  (landscape, square, portrait, compact GIF, square GIF)
- Rival detection query + rivalry pages with template-generated narratives
- Community replay feedback system: tagged annotations feeding evolution
- D1 schema additions: `replay_feedback` table
- Worker API addition: `POST /api/feedback` for submitting replay annotations

**Exit criteria:** users can write and test bots in the browser (TS
quick-start or uploaded WASM) without deploying anything, watch enriched
replays with commentary and win probability, export clips for social
sharing, view rivalries, and submit tactical feedback that influences the
evolution pipeline.

### Phase 9: Platform Depth

**Deliverables:**
- Bot debug telemetry: optional `debug` field in move response schema,
  stored in replay, rendered in viewer side panel + grid overlays
- Replay view modes: dots (default), Voronoi territory, influence gradient
  — all computed client-side, toggled via viewer toolbar
- Embeddable replay widget: `/embed/{match_id}` route on Pages, minimal
  Chrome, auto-play, ~50KB, Open Graph tags
- Replay playlists: auto-curated collections rebuilt by index cron, stored
  in R2, browsable on the static site
- Prediction system: D1 `predictions` table, Worker endpoints for submit
  + resolve, prediction leaderboard JSON in R2
- Map evolution pipeline: engagement scoring, breeding/mutation, symmetry
  validation, positional fairness monitoring, user map voting
- Multi-game series: D1 `series` table, series scheduler, unified replay
  presentation, spoiler toggle
- Match event timeline: client-side event extraction, icon ribbon in
  replay viewer, click-to-jump
- Seasonal system: D1 `seasons` table, ladder reset logic, season archive
  pages, versioned game rules with backward compatibility
- Bot profile cards: Canvas-rendered PNG, shareable URL with OG tags

**Exit criteria:** the platform supports seasonal competition with map
evolution, multi-game series, predictions for non-coders, embeddable
replays, curated playlists, three replay view modes, bot debug telemetry,
event timelines, and shareable bot profile cards. All within Cloudflare
free tier.

### Phase 10: Ecosystem & Polish

**Deliverables:**
- Weekly meta report: auto-generated blog post published to R2, rendered
  on `/blog` with LLM-enhanced narrative sections
- Public match data: documented static JSON file paths in R2, OpenAPI-style
  documentation at `/docs/api`, versioned replay format spec
- Accessibility suite: Tol color-blind palette + shape-per-player, keyboard
  shortcuts for replay viewer, high contrast mode, reduced motion, screen
  reader transcript, focus indicators
- Live evolution observatory: evolver writes `live.json` to R2 every cycle,
  observatory page polls and renders live feed + lineage tree + meta shift
  chart
- Narrative engine: weekly story arc detection, LLM-generated 200-word
  chronicles, published alongside meta reports on `/blog`

**Exit criteria:** the platform publishes weekly editorial content (meta
report + story arcs) as blog posts, exposes all match data as documented
static JSON, meets WCAG accessibility standards for color and keyboard
navigation, and streams the evolution process as a live observatory.

---

## 12. Enhanced Features

### 12.1 In-Browser WASM Game Sandbox

The game engine and bots compile to WebAssembly, enabling users to develop
and test bots entirely in the browser against real opponents — zero
deployment, zero server setup.

**Architecture — WASM per module, not JS functions:**

A meaningful bot needs pathfinding, state tracking across turns, spatial
data structures, and threat assessment. That's a real program, not a
20-line JavaScript function. Limiting bots to JS callbacks would undermine
the platform's multi-language premise.

Instead, the sandbox loads **separate WASM modules** for the game engine
and each bot:

```
Browser
├── Game Engine (Go → WASM, ~15 MB)
│   ├── loadState(json) → set engine to a specific turn state
│   ├── step(moves[]) → advance one turn, return events
│   └── runMatch(config, map) → run full match coordinating bot WASMs
│
├── Bot WASMs (pre-compiled, loaded on demand)
│   ├── gatherer.wasm    (Go → WASM, ~12 MB)
│   ├── rusher.wasm      (Rust → WASM, ~3 MB)
│   ├── swarm.wasm       (TypeScript → WASM via wasm-pack, ~5 MB)
│   ├── random.wasm      (Go → WASM, ~10 MB)  -- lightweight reimpl
│   ├── guardian.wasm     (Go → WASM, ~12 MB)  -- reimpl from PHP
│   └── hunter.wasm      (Go → WASM, ~12 MB)  -- reimpl from Java
│
├── User's Bot WASM (compiled locally, uploaded as .wasm file)
│   └── or: user writes Go/Rust/TS, compiles in-browser via toolchain
│
├── Monaco Editor (code editing for quick-start JS/TS mode)
└── Replay Viewer (Canvas, renders result)
```

**WASM communication interface:**

Each bot WASM exports a standard interface:

```
// Exported by every bot WASM module
fn init(config_json: *const u8, config_len: u32)
fn compute_moves(state_json: *const u8, state_len: u32) -> *const u8
fn free_result(ptr: *const u8)
```

The engine WASM orchestrates the match: each turn, it serializes the
fog-filtered game state as JSON, calls each bot WASM's `compute_moves`,
deserializes the moves, and advances the simulation. Bots maintain their
own internal state across turns inside their WASM linear memory.

**Language support in the sandbox:**

| Language | WASM Compilation | Sandbox Support |
|----------|-----------------|-----------------|
| Go | `GOOS=js GOARCH=wasm` (native) | Full |
| Rust | `wasm32-unknown-unknown` (native) | Full |
| TypeScript | AssemblyScript or wasm-pack | Full |
| Python | Pyodide (~20 MB runtime) | Heavy but feasible |
| PHP | Not practical for WASM | HTTP ladder only |
| Java | Not practical for WASM | HTTP ladder only |

For the built-in opponents, GuardianBot (PHP) and HunterBot (Java) are
**reimplemented in Go** as sandbox-only WASM builds. They are behaviorally
equivalent — same BFS, same combat logic, same heuristics — not identical
code.

**Memory budget:**

| Configuration | Memory |
|--------------|--------|
| Engine + 1 user bot + 1 opponent | ~30–40 MB |
| Engine + 1 user bot + 3 opponents (4-player) | ~55–75 MB |
| Engine + 1 user bot + 5 opponents (6-player) | ~75–105 MB |
| With Pyodide (Python user bot) | Add ~20 MB |

Desktop browsers typically have 2–4 GB available. Even the heaviest
configuration is <5% of available memory. Mobile is tighter but the
sandbox is a desktop-first dev tool.

A 500-turn 2-player match simulates in ~2–3 seconds (WASM-to-WASM calls
have overhead vs native, but each turn's computation is trivial).

**User flows (two modes):**

*Quick-start mode (JS/TS in Monaco):*

1. User visits `/sandbox`
2. Monaco editor pre-loaded with a TypeScript starter bot
3. User writes strategy code with full type hints and autocomplete
4. Code compiles to WASM in-browser via AssemblyScript
5. Selects opponent and map, clicks "Run Match"
6. Engine orchestrates match between user WASM and opponent WASM (~2–3s)
7. Replay viewer renders result inline
8. Modify, re-run — instant feedback loop

*Full mode (upload compiled WASM):*

1. User develops a bot locally in Go, Rust, or any WASM-targeting language
2. Compiles to `.wasm` using their own toolchain
3. Uploads the `.wasm` file to the sandbox page
4. Sandbox validates the exported interface (`init`, `compute_moves`)
5. Runs match against selected opponents
6. When ready for the real ladder, deploys the same bot logic as an HTTP
   server using a starter kit

**Why this matters:** The sandbox preserves the platform's multi-language
strength while eliminating the deployment barrier. Users can develop
substantial, stateful bots in real languages — not toy JS functions —
and iterate locally before committing to the HTTP ladder.

### 12.2 Win Probability Graph + Critical Moments

Every match replay includes a **win probability curve** — a per-turn estimate
of each player's chance of winning — and a set of **critical moments** where
the game's outcome shifted decisively.

**Win probability computation:**

After each match, the worker computes win probability using Monte Carlo
rollout:

```
for each turn T in the match:
    state = game_state_at_turn_T
    wins = [0, 0, ..., 0]  // per player
    for i in 1..100:
        result = simulate_random_play(state, remaining_turns)
        wins[result.winner] += 1
    win_prob[T] = wins / 100
```

`simulate_random_play` runs the game engine with random valid moves for all
players from the given state to completion. 100 rollouts × 500 turns is
~50,000 engine steps — the Go engine handles this in <1 second.

The result is stored in the replay JSON as a `win_prob` array:
```json
"win_prob": [
    [0.50, 0.50],
    [0.51, 0.49],
    [0.48, 0.52],
    ...
]
```

Size: ~4 KB for a 500-turn, 2-player match. Negligible.

**Critical moments:**

A critical moment is any turn where `|Δwin_prob|` exceeds 0.15 (15%) for
any player. Typically 3–5 per match. Stored in the replay JSON:

```json
"critical_moments": [
    { "turn": 87, "delta": 0.22, "description": "SwarmBot loses 6 units in eastern engagement" },
    { "turn": 203, "delta": -0.31, "description": "GathererBot's core captured" }
]
```

The `description` is auto-generated from the turn's events (deaths, captures,
large position changes). No LLM needed — template-based.

**Replay viewer integration:**

- **Sparkline graph** below the main canvas: one line per player, color-coded
- Horizontal axis: turns. Vertical axis: 0%–100% win probability
- **Critical moment markers**: vertical dashed lines on the graph with labels
- **Click to jump**: clicking any point on the graph scrubs to that turn
- **Quick nav buttons**: "Next critical moment" / "Previous critical moment"
  to skip between turning points

This transforms replay viewing from "press play and wait 5 minutes" to
"click the 3 interesting moments and watch 30 seconds of decisive action."

### 12.3 Replay Enrichment (Selective AI Commentary)

Select replays receive AI-generated natural language commentary — a
play-by-play narration that makes matches accessible to casual viewers.

**Not all replays are enriched.** Commentary is generated selectively for:

- **Featured matches**: matches flagged by the system as particularly
  interesting (high win probability variance, close finishes, upsets where
  a lower-rated bot wins)
- **Rivalry matches**: matches between detected rivals (§12.5)
- **Evolution milestones**: first match of a newly promoted evolved bot,
  or matches where an evolved bot breaks into the top 10
- **User-requested**: participants can request enrichment for their own
  matches (rate-limited: 5 per day per bot)

**Selection criteria (automatic):**
```
enrich if:
  - win_prob crossed 0.5 at least 3 times (back-and-forth match)
  - final score difference ≤ 2
  - winner's rating was ≥100 lower than loser's (upset)
  - match involved a newly promoted evolved bot
  - match is between detected rivals
```

At ~60 matches/hour, roughly 10–15% qualify — about 6–9 enriched replays
per hour.

**Commentary generation:**

Run a fast, cheap LLM (Haiku-class) over the replay data at match
completion. This happens as an optional post-processing step on the match
worker.

**Input prompt:**
```
Narrate this bot battle. Provide commentary for key moments only
(not every turn). Write 1-2 sentences per key moment. Be specific
about positions, unit counts, and tactical decisions.

Match: {players, ratings, map_size}
Win probability curve: {win_prob array, sampled every 10 turns}
Critical moments: {critical_moments array}
Key events by turn: {deaths, captures, large movements, energy collected}
```

**Output:** array of `{turn, text}` entries stored in the replay JSON:
```json
"commentary": [
    { "turn": 1, "text": "Both bots spawn at opposite corners of a 60x60 grid with heavy wall cover in the center. SwarmBot immediately sends all units east in a tight cluster." },
    { "turn": 42, "text": "First contact. GathererBot's scout stumbles into SwarmBot's formation near the central energy cluster. The scout is outnumbered 8-to-1 and eliminated instantly." },
    { "turn": 87, "text": "The turning point. SwarmBot pushes through the eastern corridor but GathererBot has quietly amassed 14 units behind the western wall line — a force SwarmBot doesn't know exists." }
]
```

**Cost:** ~$0.01–0.03 per enriched match at Haiku pricing. At 9
enriched matches/hour: ~$2–6/day, ~$60–180/month. Reasonable.

**Replay viewer integration:**
- Commentary appears as subtitles below the canvas, synchronized to turn
  playback
- Toggle on/off via a "Commentary" button
- Enriched replays are badged on the match list ("Featured" / "Narrated")

### 12.4 Shareable Replay Clips

One-click export of a replay segment as a GIF or video, formatted for
major social media platforms. This is the viral growth engine.

**Export formats:**

| Preset | Resolution | Aspect | Format | Target |
|--------|-----------|--------|--------|--------|
| Landscape | 1920×1080 | 16:9 | MP4 | YouTube, Twitter, Discord |
| Square | 1080×1080 | 1:1 | MP4 | Twitter, Instagram feed |
| Portrait | 1080×1920 | 9:16 | MP4 | TikTok, YouTube Shorts, IG Stories |
| GIF (compact) | 640×360 | 16:9 | GIF | Discord embeds, forums |
| GIF (square) | 480×480 | 1:1 | GIF | Twitter, Slack |

**User flow:**

1. While watching a replay, click "Clip" (scissors icon)
2. Drag handles on the turn scrubber to select a segment (default: 20 turns
   centered on the current turn, or the nearest critical moment)
3. Select format preset from dropdown
4. Optional: toggle overlays (score, win probability, commentary subtitles)
5. Click "Export"
6. Browser records the Canvas replay segment using `OffscreenCanvas` +
   `MediaRecorder` API (MP4/WebM) or gif.js (GIF)
7. Processing happens entirely client-side — no server cost
8. Download button appears, plus "Share" buttons:
   - **Twitter/X**: opens compose with the clip attached + auto-generated
     text ("SwarmBot pulls off a comeback against HunterBot! 🎮
     aicodebattle.com/replay/{id}")
   - **Reddit**: copies a markdown link with embedded video
   - **Discord**: downloads the file (under Discord's 25MB upload limit)
   - **Copy link**: shareable URL to the replay at the specific turn range

**Clip overlay:** the exported clip includes:
- Player names + colors in a header bar
- Score overlay (bottom-left)
- Win probability mini-graph (bottom strip, if enabled)
- "aicodebattle.com" watermark (small, bottom-right)

**GIF optimization:** GIFs are limited to 256 colors and can be large.
The clip maker uses:
- Reduced frame rate (10 fps for GIF vs 30 fps for MP4)
- Color quantization optimized for the grid art style
- Max 10-second duration for GIFs (longer clips → MP4 only)
- Target size: <5 MB for GIFs, <15 MB for MP4

**Implementation:** ~200 lines of TypeScript. `MediaRecorder` for MP4,
`gif.js` for GIF, `OffscreenCanvas` for headless rendering. All runs in
the browser. The share buttons use Web Share API where available, fallback
to window.open() with pre-composed URLs.

### 12.5 Automatic Rival Detection

The platform automatically identifies **rivalries** — pairs of bots that
frequently play each other with close results — and surfaces them as
narrative-driven content.

**Detection algorithm:**

```sql
-- Run by the index rebuilder cron
SELECT
    a.bot_id AS bot_a,
    b.bot_id AS bot_b,
    COUNT(*) AS matches,
    SUM(CASE WHEN winner = a.player_slot THEN 1 ELSE 0 END) AS a_wins,
    SUM(CASE WHEN winner = b.player_slot THEN 1 ELSE 0 END) AS b_wins
FROM match_participants a
JOIN match_participants b ON a.match_id = b.match_id AND a.bot_id < b.bot_id
JOIN matches m ON m.match_id = a.match_id
WHERE m.status = 'completed'
GROUP BY a.bot_id, b.bot_id
HAVING COUNT(*) >= 10
ORDER BY COUNT(*) * (1.0 - ABS(CAST(a_wins - b_wins AS REAL) / COUNT(*))) DESC
LIMIT 20
```

The ranking formula: `matches_played × (1 - |win_rate_imbalance|)`.
High-scoring pairs have many matches with near-50/50 results — the
definition of a rivalry.

**Rivalry page** (`/rivalry/{bot_a_id}/{bot_b_id}`):

```json
{
    "bot_a": { "id": "b_4e8c1d2f", "name": "SwarmBot", "owner": "alice" },
    "bot_b": { "id": "b_9a1b3c4d", "name": "HunterBot", "owner": "bob" },
    "matches": 23,
    "record": { "a_wins": 11, "b_wins": 11, "draws": 1 },
    "closest_match": "m_abc123",
    "longest_streak": { "holder": "b_4e8c1d2f", "length": 4 },
    "recent_matches": ["m_abc123", "m_def456", ...],
    "narrative": "SwarmBot and HunterBot have met 23 times — the series is dead even at 11-11-1. SwarmBot held a 4-match winning streak from Mar 15-18, but HunterBot answered with 3 straight victories. Their last match was decided by a single point."
}
```

The narrative is template-generated from the stats (no LLM needed):
```
"{bot_a} and {bot_b} have met {n} times — {record_description}.
{streak_description}. {recent_trend_description}."
```

**Platform integration:**
- Rivalry widget on the landing page: "Top Rivalries" with head-to-head
  records and links to key matches
- Bot profile pages show "Rivals" section listing detected rivalries
- Rivalry matches are auto-flagged for replay enrichment (§12.3)
- Leaderboard can show "rivalry mode" — filter to matches between two
  specific bots

### 12.6 Community Replay Feedback

Users can leave **tagged feedback** on specific moments in replays.
Feedback is anchored to a `(replay_id, turn)` pair and is visible to
other viewers. High-signal feedback is fed into the evolution pipeline
as strategic hints.

**Feedback types:**

| Type | Icon | Purpose |
|------|------|---------|
| Tactical insight | 💡 | "This flanking move was brilliant because..." |
| Mistake spotted | ⚠️ | "Bot should have retreated here — outnumbered 3:1" |
| Strategy idea | 🧪 | "What if a bot used this wall corridor as a chokepoint?" |
| Highlight | ⭐ | "Amazing play" (lightweight, like a star/upvote) |

**D1 schema:**

```sql
CREATE TABLE replay_feedback (
    feedback_id   TEXT PRIMARY KEY,
    match_id      TEXT NOT NULL,
    turn          INTEGER NOT NULL,
    type          TEXT NOT NULL,  -- 'insight', 'mistake', 'idea', 'highlight'
    body          TEXT NOT NULL,
    author        TEXT NOT NULL,  -- free text (no accounts, like registration)
    upvotes       INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL
);

CREATE INDEX idx_feedback_match ON replay_feedback(match_id, turn);
```

**Replay viewer integration:**

- Small markers appear on the turn scrubber at turns with feedback
- Hovering shows a preview count: "3 comments at turn 87"
- Clicking opens a side panel showing all feedback for that turn
- Users can add their own feedback via a form in the side panel
- Upvote button on each feedback item (1 per visitor via localStorage)

**Feeding into evolution:**

The evolution pipeline's prompt builder (§10.3) consumes community
feedback as an additional signal:

1. Index rebuilder aggregates high-upvote feedback of type `idea` and
   `mistake` into `data/evolution/community_hints.json`
2. The evolver reads this file and includes the top-voted recent hints
   in the prompt:

```
## Community Tactical Insights (from replay annotations)

Replay m_abc123, Turn 87 (12 upvotes):
"The bot should have used the narrow wall corridor at (30,42)-(30,48)
as a chokepoint instead of engaging in the open. A defensive line of
3 units there could have held off the 8-unit swarm."

Replay m_def456, Turn 203 (8 upvotes):
"When outnumbered 3:1, retreating toward the nearest energy cluster
and spawning reinforcements is better than fighting — the focus combat
system guarantees you lose the 3v1."
```

3. If a resulting evolved bot performs well, the feedback items that
   contributed to its prompt are credited on the evolution dashboard:
   "Feedback from user 'tactician42' on replay m_abc123 contributed to
   evo-py-g42-7f3a (rating: 1720)"

**Moderation:**

- Feedback is plain text, max 500 characters
- No accounts means no banning — but feedback is public and upvote-ranked,
  so low-quality content sinks
- A simple word filter catches obvious spam
- The evolution pipeline only consumes feedback with ≥3 upvotes, filtering
  noise automatically
- Admin can delete feedback via a D1 query (no UI needed initially)

**Why this matters:** It creates a human-AI collaboration loop. Spectators
contribute strategic insight, the AI translates it into code, the platform
evaluates it, and successful feedback is credited. This gives non-coders a
way to participate meaningfully in the competition.

---

## 13. Platform Depth Features

### 13.1 Bot Debug Telemetry + Reasoning Visualization

Bots can optionally include a `debug` field in their move response. The
engine stores it in the replay without interpreting it. The replay viewer
renders it.

**Extended move response schema:**

```json
{
  "moves": [
    { "row": 10, "col": 15, "direction": "N" }
  ],
  "debug": {
    "reasoning": "3 energy within 5 tiles east; enemy cluster north — avoiding",
    "targets": [
      { "row": 20, "col": 25, "label": "energy", "priority": 0.9 },
      { "row": 8, "col": 30, "label": "threat", "priority": 0.7 }
    ],
    "values": {
      "energy_reserves": 7,
      "threat_level": "medium",
      "mode": "gathering"
    },
    "heatmap": {
      "name": "threat",
      "data": [[0, 0, 0.2, 0.8], [0, 0.1, 0.5, 0.9]]
    }
  }
}
```

**Schema rules for `debug`:**
- Entirely optional — bots that omit it behave identically
- Max size: 10 KB per turn (prevents replay bloat; excess is truncated)
- The engine never reads or acts on debug data — it's pass-through to replay
- No fields inside `debug` are validated beyond size — bots can put anything
- Only the bot's owner sees debug data by default; owners can toggle public
  visibility per-bot in their bot profile

**Replay viewer rendering:**

| Debug field | Rendering |
|-------------|-----------|
| `reasoning` | Text in a collapsible side panel, one entry per turn |
| `targets` | Colored markers on the grid (green = high priority, red = low) with labels |
| `values` | Key-value table in the side panel, updates each turn |
| `heatmap` | Semi-transparent color overlay on the grid (blue→red gradient) |

All debug rendering is toggled via a "Debug" button in the viewer toolbar.
When off, no debug data is shown (default for spectators). When on, the
viewer shows the selected player's debug output.

**Replay size impact:**

A bot sending 5 KB of debug data per turn across 500 turns adds 2.5 MB
to the replay. With gzip compression (~90% on structured JSON), that's
~250 KB. Acceptable alongside the ~50 KB base replay.

**Why it matters:** This is a visual debugger for distributed bot code.
Instead of reading logs, developers watch their bot's thought process
alongside its actions. For spectators who opt in, seeing "the bot is
scared of the northern cluster" while watching it move south creates
narrative that no commentary system can match.

### 13.2 Territory Control Heatmap Overlay

The replay viewer supports three visualization modes, toggled via a toolbar
dropdown. All computed client-side from bot positions — no server cost.

**Mode 1: Dots (default)**

The current view — bots as colored circles on the grid. Minimal, clean,
fast.

**Mode 2: Voronoi Territory**

Each tile on the grid is colored by which player's nearest bot is closest.
Creates clean territorial borders that shift each turn.

```
Computation per turn:
  for each visible tile (row, col):
    min_dist = infinity
    owner = none
    for each bot on the grid:
      d = toroidal_distance_squared(tile, bot)
      if d < min_dist:
        min_dist = d
        owner = bot.owner
    tile_color = player_colors[owner] at 30% opacity
```

For a 60×60 grid with 50 bots, that's 3,600 × 50 = 180,000 distance
calculations per turn — trivial for modern JS (~1ms). The result is a
per-tile color array rendered as a single full-grid Canvas `fillRect` pass
underneath the bot sprites.

**Mode 3: Influence Gradient**

Force projection based on bot count and distance. Each player's influence
at a tile is the sum of `1 / (1 + distance)` across all their bots.
Rendered as a smooth gradient:

```
for each visible tile:
    influence = [0, 0, ..., 0]  // per player
    for each bot:
        d = toroidal_distance(tile, bot)
        influence[bot.owner] += 1.0 / (1.0 + d)
    dominant = argmax(influence)
    strength = influence[dominant] / sum(influence)
    tile_color = player_colors[dominant] at (strength × 50%) opacity
```

The gradient creates a softer, more organic visualization than Voronoi —
you can see where influence is strong (dense, saturated) vs weak (faint,
contested). Frontlines appear as narrow bands where no player dominates.

**Performance:** both modes compute in <5ms per turn on a 60×60 grid.
The replay viewer caches the overlay bitmap per turn and only recomputes
on turn change. At 32 turns/second (16× speed), this stays within frame
budget.

**Toolbar UI:**

```
View: [Dots ▼]  [Dots | Territory | Influence]
```

Switching modes is instant — the underlying replay data doesn't change,
only the rendering pipeline.

### 13.3 Embeddable Replay Widget

A lightweight, standalone replay player that works in an iframe anywhere.

**URL format:**
```
https://aicodebattle.com/embed/{match_id}
https://aicodebattle.com/embed/{match_id}?start=87&speed=4&mode=territory
```

**Query parameters:**

| Param | Default | Description |
|-------|---------|-------------|
| `start` | 0 | Starting turn |
| `speed` | 2 | Playback speed (1, 2, 4, 8, 16) |
| `mode` | dots | Visualization mode (dots, territory, influence) |
| `autoplay` | true | Start playing on load |
| `controls` | true | Show play/pause and speed controls |

**Widget design:**

Stripped-down replay viewer: canvas + minimal controls bar. No scrubber,
no side panel, no fog-of-war toggle. Just the match playing.

```
┌──────────────────────────────┐
│                              │
│        [Canvas]              │
│                              │
├──────────────────────────────┤
│ ▶ 2x  SwarmBot 3 — 1 Hunter │
│            Watch full ↗      │
└──────────────────────────────┘
```

"Watch full" links to the main replay page on aicodebattle.com.

**Implementation:**

- Separate route on Cloudflare Pages: `/embed/{match_id}`
- Loads the same replay JSON from R2
- Renders with the same Canvas engine, minus chrome
- Total bundle: ~50 KB (JS + CSS)
- Open Graph tags for rich previews when pasting the URL:
  ```html
  <meta property="og:title" content="SwarmBot vs HunterBot — AI Code Battle" />
  <meta property="og:description" content="SwarmBot wins 3-1 in 342 turns" />
  <meta property="og:image" content="https://data.aicodebattle.com/thumbnails/m_7f3a9b2c.png" />
  ```
- Thumbnail: auto-generated PNG of the final turn state, created by the
  index rebuilder using OffscreenCanvas in a Worker (or pre-rendered by
  the match worker)

**Cloudflare free tier impact:** embed loads are Pages requests (unlimited).
The replay JSON fetch is an R2 Class B read — already accounted for in the
existing budget.

### 13.4 Replay Playlists + Auto-Curation

Automatically curated collections of replays, browsable from the static
site's landing page.

**Playlist definitions:**

| Playlist | Query Criteria | Rebuild Frequency |
|----------|---------------|-------------------|
| "Closest Finishes" | `final_score_diff <= 1` sorted by `win_prob_crossings DESC` | Every 2 min (index cron) |
| "Biggest Upsets" | `winner_rating - loser_rating <= -150` | Every 2 min |
| "Best Comebacks" | `min(win_prob) < 0.2 AND winner = underdog` | Every 2 min |
| "Evolution Breakthroughs" | Evolved bot's first win against a top-10 bot | Every 2 min |
| "Rivalry Classics" | Matches between detected rivals, sorted by closeness | Every 2 min |
| "This Week's Highlights" | Top 10 by community upvote count (from §12.6) | Every 2 min |
| "New Bot Debuts" | First match of each newly registered bot | Every 2 min |
| "Season Highlights" | Top 20 matches of the current season by engagement | Every 2 min |

**R2 storage:** `data/playlists/{slug}.json`

```json
{
  "name": "Closest Finishes",
  "description": "Matches decided by a single point or less",
  "updated_at": "2026-03-23T14:35:00Z",
  "matches": [
    {
      "match_id": "m_7f3a9b2c",
      "players": ["SwarmBot", "HunterBot"],
      "scores": [3, 2],
      "date": "2026-03-23T14:30:00Z",
      "thumbnail_url": "https://data.aicodebattle.com/thumbnails/m_7f3a9b2c.png",
      "enriched": true
    }
  ]
}
```

**Static site UI:** landing page shows playlists as horizontal scrollable
rows (Netflix-style). Each card shows a thumbnail, player names, and score.
Click opens the replay.

**Cloudflare free tier impact:** playlist JSONs are tiny (<50 KB each).
They're rebuilt by the existing index rebuilder cron — just additional D1
queries and R2 writes within existing budget.

### 13.5 Prediction System

Visitors predict outcomes of upcoming notable matches. Correct predictions
earn reputation. A prediction leaderboard tracks the best analysts.

**Which matches get predictions:**

The matchmaker flags a match as "predictable" when:
- Both bots are in the top 20
- It's a rivalry match
- It's a series match (§13.7)
- An evolved bot faces a top-10 human-written bot

At ~60 matches/hour, roughly 5–10% are flagged — about 3–6 per hour.

**Flow:**

1. Scheduler creates a match job with `predictable: true`
2. Worker API writes the match to a `predictions_open` state in D1
3. Static site shows "Upcoming Matches" with a predict button
4. Visitor clicks a player to predict (stored via `POST /api/predict`)
5. Prediction window: open from job creation until the match starts
   executing (typically 1–5 minutes)
6. Match executes normally
7. On result submission, Worker resolves predictions in D1
8. Index rebuilder updates the prediction leaderboard JSON in R2

**D1 schema:**

```sql
CREATE TABLE predictions (
    prediction_id   TEXT PRIMARY KEY,
    match_id        TEXT NOT NULL,
    predictor_id    TEXT NOT NULL,  -- localStorage-generated UUID
    predictor_name  TEXT,           -- optional display name
    predicted_winner INTEGER NOT NULL,
    correct         INTEGER,        -- null until resolved
    created_at      TEXT NOT NULL
);

CREATE TABLE predictor_stats (
    predictor_id    TEXT PRIMARY KEY,
    predictor_name  TEXT,
    correct         INTEGER NOT NULL DEFAULT 0,
    incorrect       INTEGER NOT NULL DEFAULT 0,
    streak          INTEGER NOT NULL DEFAULT 0,
    best_streak     INTEGER NOT NULL DEFAULT 0,
    rating          REAL NOT NULL DEFAULT 1000.0
);
```

Predictor rating uses a simplified Elo: correct prediction on a balanced
match (close ratings) = small gain; correct prediction on a heavy underdog
= large gain.

**Cloudflare free tier check:**

| Metric | Usage | Limit |
|--------|-------|-------|
| D1 writes | ~6 predictions/match × 6 matches/hour × 24h = ~864/day | 100K/day |
| D1 reads | ~50 leaderboard reads/day | 5M/day |
| Worker requests | `POST /api/predict` ~864/day | 100K/day |

Comfortably within limits. Even at 10× the assumed prediction volume
(8,640/day), still under 9% of the write limit.

**Static site UI:**

- "Predictions" page showing upcoming predictable matches with bot profiles
  and head-to-head records
- One-click predict button (no login required — UUID from localStorage)
- After match: result shown with "You were right/wrong" + points earned
- Prediction leaderboard: top 50 analysts ranked by prediction rating

### 13.6 Map Evolution

Maps evolve alongside bots. High-engagement maps breed to produce new maps.
Low-engagement maps retire. User feedback and positional fairness monitoring
ensure quality.

**Engagement scoring:**

After each match, the map receives an engagement score:

```
engagement = (
    win_prob_crossings × 3.0 +
    critical_moments × 2.0 +
    map_coverage_pct × 1.0 +
    closeness × 2.0 +
    avg_turn_count / max_turns × 1.0
)

where:
    closeness = 1.0 - (abs(score_diff) / max(total_score, 1))
    map_coverage_pct = tiles_visited_by_any_bot / total_open_tiles
```

The map's engagement score is the rolling average across its last 20 matches.

**Positional fairness monitoring:**

A map is **positionally fair** if no starting position has a systematic
advantage. Monitored by tracking win rate per player slot:

```sql
SELECT
    map_id,
    player_slot,
    COUNT(*) AS games,
    AVG(CASE WHEN winner = player_slot THEN 1.0 ELSE 0.0 END) AS win_rate
FROM match_participants mp
JOIN matches m ON m.match_id = mp.match_id
GROUP BY map_id, player_slot
HAVING COUNT(*) >= 20
```

If any player slot's win rate deviates from the expected rate (1/N for
N-player maps) by more than **10 percentage points** across 20+ matches,
the map is flagged as **unfair** and removed from the competitive pool.

Example: on a 2-player map, if player slot 0 wins 62% of the time after
20 matches, the map is flagged (62% - 50% = 12% > 10% threshold).

**User map voting:**

After watching a replay, visitors can upvote or downvote the map (not the
match — the map). Stored in D1:

```sql
CREATE TABLE map_votes (
    vote_id     TEXT PRIMARY KEY,
    map_id      TEXT NOT NULL,
    voter_id    TEXT NOT NULL,  -- localStorage UUID
    vote        INTEGER NOT NULL,  -- +1 or -1
    created_at  TEXT NOT NULL,
    UNIQUE(map_id, voter_id)
);
```

Map voting influences the evolution system:
- Maps with net negative votes get a 0.5× engagement multiplier (less likely
  to breed)
- Maps with >10 net positive votes get a 1.5× multiplier
- Maps with >20 net negative votes are force-retired regardless of engagement

The replay viewer shows a simple 👍/👎 widget for the map (not the bots)
alongside map metadata (name, dimensions, wall density, energy count).

**Breeding algorithm:**

Runs weekly on the evolver (Rackspace Spot). Produces ~5 new maps per
player-count tier.

```
1. Select parents:
   - Top 5 maps by engagement × vote_multiplier for this player count
   - Weighted random: higher engagement = more likely to be selected

2. Crossover:
   - Divide parent maps into quadrants (or thirds for 3/6-player)
   - Randomly select quadrants from each parent
   - Compose into a new map

3. Apply symmetry:
   - Generate one sector from the composed quadrants
   - Mirror/rotate to fill the full map for the target player count
   - This guarantees positional fairness by construction

4. Mutate:
   - Randomly flip 5-10% of tiles (wall ↔ open)
   - Shift 1-3 energy node positions by 1-3 tiles
   - Apply cellular automata smoothing (2 iterations) to avoid
     jagged walls

5. Validate:
   - BFS from every core must reach every other core
   - BFS from every core must reach ≥3 energy nodes
   - Open area per player must be between 900 and 5000 tiles
   - Wall density must be between 5% and 30%

6. Smoke-test:
   - Run 3 matches with built-in bots on the candidate map
   - Engagement score must exceed 50th percentile of current pool
   - If failed: discard and retry (max 3 attempts per candidate)

7. Add to pool:
   - Store map JSON in R2
   - Insert into D1 maps table with `status: 'active'`
   - Available for matchmaking in the next scheduler cycle
```

**Lifecycle:**

| Status | Meaning |
|--------|---------|
| `active` | In the matchmaking pool, eligible for competitive play |
| `probation` | Fairness flag triggered — under review, still playable |
| `retired` | Removed from pool (low engagement, unfair, or force-retired) |
| `classic` | Top 5 all-time maps, immune from retirement |

- Active pool: 50 maps per player count (2, 3, 4, 6)
- New maps: ~5 per week per player count
- Retirement: bottom 10% by engagement score pruned monthly
- Classic promotion: maps that sustain top-5 engagement for 3+ months

### 13.7 Multi-Game Series

Best-of-N matches between two bots across different maps. Series produce
more meaningful ratings than single matches and create narrative arcs.

**Series types:**

| Type | Games | Trigger |
|------|-------|---------|
| Best-of-3 | 3 | Auto-scheduled for top-20 bots, 1 per day per bot |
| Best-of-5 | 5 | Weekly featured series between top rivalries |
| Best-of-7 | 7 | Season championship bracket (§13.9) |

**Map selection for series:**

Each game in a series uses a different map, selected to test different
strategic dimensions:

```
Game 1: Map with highest engagement score (the "classic")
Game 2: Map with highest wall density in pool (corridors/chokepoints)
Game 3: Map with lowest wall density in pool (open field)
Game 4: Most recently evolved map (untested terrain)
Game 5+: Random from remaining pool
```

This ensures series test bot adaptability, not just performance on one
map type.

**D1 schema:**

```sql
CREATE TABLE series (
    series_id     TEXT PRIMARY KEY,
    bot_a_id      TEXT NOT NULL,
    bot_b_id      TEXT NOT NULL,
    format        INTEGER NOT NULL,  -- 3, 5, or 7
    status        TEXT NOT NULL DEFAULT 'pending',
    a_wins        INTEGER NOT NULL DEFAULT 0,
    b_wins        INTEGER NOT NULL DEFAULT 0,
    season_id     TEXT,
    created_at    TEXT NOT NULL,
    completed_at  TEXT
);

CREATE TABLE series_games (
    series_id     TEXT NOT NULL,
    game_number   INTEGER NOT NULL,
    match_id      TEXT,  -- null until played
    map_id        TEXT NOT NULL,
    winner        INTEGER,
    PRIMARY KEY (series_id, game_number)
);
```

**Execution:**

The scheduler creates all games in a series as pending jobs with sequential
ordering. Workers execute them in order (game 2 doesn't start until game 1
completes). If either bot reaches the winning threshold (2 for bo3, 3 for
bo5, 4 for bo7), remaining games are skipped.

**Rating impact:**

Series results contribute to Glicko-2 ratings as follows:
- Each individual game in the series contributes to the pairwise rating
  update (same as a single match)
- The series winner gets a bonus rating adjustment of +10 mu (small but
  meaningful — rewards series consistency)

**Replay presentation:**

The series page (`/series/{series_id}`) shows all games as a unified
experience:

```
SwarmBot vs HunterBot — Best of 5 (Season 4 Semifinals)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Game 1  ✓ SwarmBot    3-1  Map: The Labyrinth      [Watch]
Game 2  ✓ HunterBot   2-4  Map: Open Expanse       [Watch]
Game 3  ✓ HunterBot   1-3  Map: Coral Reef         [Watch]
Game 4    ???                                       [Reveal]
Game 5    ???                                       [Reveal]

Series: HunterBot leads 2-1
```

**Spoiler toggle:** by default, future games are hidden ("???"). Viewers
click "Reveal" to show the result — or "Watch All" to experience the
series sequentially with auto-advancing between games.

### 13.8 Match Event Timeline

A horizontal event ribbon below the replay canvas showing significant
events as colored, clickable icons.

**Event types:**

| Icon | Event | Trigger |
|------|-------|---------|
| ⚔️ | Combat | 2+ bots died this turn |
| 🏰 | Core captured | A core was razed |
| 💎 | Energy milestone | Player collected 3+ energy in one turn |
| 💀 | Mass death | 5+ bots died this turn |
| 📈 | Momentum shift | Win probability crossed 50% |
| 🌟 | Critical moment | Win probability shifted >15% |
| 🐣 | Spawn wave | 3+ bots spawned this turn |

**Implementation:**

Events are extracted client-side from the replay data on load. For each
turn, check the events array (deaths, captures, spawns, energy_collected)
against the trigger thresholds. Win probability events come from the
`win_prob` and `critical_moments` arrays already in the replay.

**Rendering:**

```
┌──────────────────────────────────────────────────┐
│                  [Canvas]                         │
├──────────────────────────────────────────────────┤
│ Win Prob: ~~~~~~~~~/\~~~~~/\~~~~/\~~~~~~         │  ← sparkline
├──────────────────────────────────────────────────┤
│ Events: ·💎·····⚔️··💎···🏰⚔️···💎···⚔️💀··🏰🌟│  ← timeline
├──────────────────────────────────────────────────┤
│ ◄ ▶ ⏸  Turn 203/500   Speed: 4x   View: [Dots]│  ← controls
└──────────────────────────────────────────────────┘
```

- Icons are positioned proportionally along the timeline by turn number
- Hovering an icon shows a tooltip: "Turn 87: 3 bots killed in eastern
  corridor"
- Clicking an icon scrubs the replay to that turn
- Dense clusters of icons indicate "hot zones" of activity — visually
  obvious even at a glance
- The timeline is rendered as an HTML element overlaid on the viewer
  (not Canvas) for accessibility and hover interactions

The event timeline and win probability graph work together: the graph
shows the *trend*, the timeline shows the *moments*. A viewer can scan
the timeline for icon clusters, then check the win probability graph to
see if those moments mattered.

### 13.9 Seasonal Rotations

The platform runs in **seasons** — 4-week competitive periods with a fresh
map pool, a new ladder, and a theme. Seasons provide urgency, freshness,
and a reason to come back.

**Season structure:**

| Week | Phase | Description |
|------|-------|-------------|
| 1 | Discovery | New map pool + theme released. All bots start at default rating. Exploration matches. |
| 2–3 | Competition | Main ladder. Matchmaking intensifies. Mid-season stats published. |
| 4 | Championship | Top 8 bots by rating enter a best-of-7 bracket. Season champion crowned. |
| Between | Break (3 days) | New maps bred via map evolution. Season archive published. |

**What resets each season:**
- Glicko-2 ratings (mu/phi/sigma reset to defaults)
- Map pool (evolved maps from previous season + new generated maps)
- Prediction standings
- Playlist contents

**What persists:**
- Bot registrations and endpoints (bots don't re-register)
- All-time records and historical season archives (browsable)
- Evolution population (continues across seasons, adapts to new maps)
- Community feedback and replay annotations

**D1 schema:**

```sql
CREATE TABLE seasons (
    season_id     TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    theme         TEXT NOT NULL,
    rules_version INTEGER NOT NULL,
    started_at    TEXT NOT NULL,
    ended_at      TEXT,
    champion_id   TEXT,
    status        TEXT NOT NULL DEFAULT 'active'
);
```

**Season themes and game rule versioning:**

Each season can introduce **minor rule variations** that keep the meta
fresh. The critical constraint: **existing bots must continue to work
without modification.** This is achieved through additive, optional
changes only.

**Backward compatibility rules:**

```
ALLOWED per-season changes (additive, non-breaking):
  ✓ New tile types that bots can ignore (treated as open by old bots)
  ✓ New optional fields in the game state JSON (old bots ignore them)
  ✓ Adjusted numeric parameters within the existing schema:
    - vision_radius2, attack_radius2, spawn_cost, energy_interval
    - These are sent in the config object each match — bots that read
      config adapt automatically; bots that hardcode values still work
      but may be suboptimal
  ✓ New scoring bonuses (additive to existing scoring)
  ✓ Map pool changes (different maps, not different map format)

FORBIDDEN (would break existing bots):
  ✗ Removing or renaming existing fields in game state / move schema
  ✗ Changing the meaning of existing fields
  ✗ New required fields in the move response
  ✗ Changing the coordinate system or grid topology
  ✗ Removing movement directions (N/E/S/W)
  ✗ Changing the turn structure (phases must remain in the same order)
```

**Example seasonal themes:**

| Season | Theme | Rule Variation |
|--------|-------|---------------|
| 1 | "The Labyrinth" | High wall density maps, `vision_radius2: 36` (reduced from 49) |
| 2 | "Energy Rush" | `energy_interval: 5` (doubled production), `spawn_cost: 2` (cheaper bots) |
| 3 | "Fog of War" | `vision_radius2: 25` (heavily reduced), new optional `sonar` field in game state showing approximate enemy count per quadrant |
| 4 | "The Colosseum" | `attack_radius2: 8` (extended range), open maps, aggressive meta |
| 5 | "Shifting Sands" | New tile type `quicksand` in game state (bots that don't handle it treat it as open — they can enter but movement costs 2 turns) |

For season 5's `quicksand` example: the game state sends
`{ "row": 15, "col": 20, "type": "quicksand" }` in a new `terrain` array.
Old bots that don't read `terrain` still function — they walk through
quicksand unknowingly (and get slowed). New bots that parse `terrain` can
avoid quicksand tiles, gaining a strategic edge. This creates an incentive
to update bots each season without *forcing* anyone to.

**Season config in the match protocol:**

The game state's `config` object already includes all tunable parameters.
Seasonal changes are just different values:

```json
{
  "config": {
    "season_id": "s4",
    "season_name": "The Colosseum",
    "rules_version": 4,
    "rows": 60,
    "cols": 60,
    "max_turns": 500,
    "vision_radius2": 49,
    "attack_radius2": 8,
    "spawn_cost": 3,
    "energy_interval": 10,
    "special_tiles": ["quicksand"]
  }
}
```

Bots that read `config.attack_radius2` adapt automatically. Bots that
hardcode `attack_radius2 = 5` still work but use stale assumptions.
`special_tiles` is a new array listing any non-standard tile types in
play — old bots that don't read it are unaffected.

**Season archive:**

Each completed season gets an archive page (`/season/{season_id}`):
- Champion + top 10 + bracket results
- Most improved bot (biggest rating gain)
- Best newcomer (highest-rated bot registered this season)
- Most watched match (by replay view count)
- Evolution highlights (best evolved bot, most creative strategy)
- Map of the season (highest engagement score)
- All replays preserved and browsable

**Season championship bracket:**

In week 4, the top 8 bots enter a single-elimination bracket of best-of-7
series (§13.7). The bracket is published on the season page with live
updates as series complete.

```
Quarterfinals:
  #1 SwarmBot vs #8 NewBot         → SwarmBot (4-1)
  #4 GathererBot vs #5 RusherBot   → RusherBot (4-3)
  #3 HunterBot vs #6 evo-go-g12    → HunterBot (4-2)
  #2 GuardianBot vs #7 evo-py-g8   → GuardianBot (4-0)

Semifinals:
  SwarmBot vs RusherBot             → SwarmBot (4-2)
  HunterBot vs GuardianBot          → HunterBot (4-3)

Finals:
  SwarmBot vs HunterBot             → ???
```

### 13.10 Bot Profile Cards

Auto-generated visual cards summarizing a bot's identity, stats, and
character in a single shareable image.

**Card generation:**

The card is rendered as a PNG via OffscreenCanvas (in the browser on
demand, or pre-rendered by the index rebuilder for top-50 bots).

**Card content:**

```
┌─────────────────────────────────┐
│                                 │
│   SwarmBot              #3      │
│   by alice         Rating: 1820 │
│                                 │
│   ┌─────────────────────────┐   │
│   │ Archetype:              │   │
│   │   FORMATION SWARM       │   │
│   │                         │   │
│   │ Season 4 · 142 games    │   │
│   └─────────────────────────┘   │
│                                 │
│   Win Rate     69%  ████████░░  │
│   vs Rushers   82%  █████████░  │
│   vs Turtles   45%  ████░░░░░░  │
│                                 │
│   Signature: Eastern corridor   │
│   push on 4-player maps         │
│                                 │
│   Rival: HunterBot (11-11-1)   │
│                                 │
│   ⚔️ 847 kills  💎 2.1k energy  │
│   🏰 23 captures  📈 +320 Elo   │
│                                 │
│   aicodebattle.com              │
└─────────────────────────────────┘
```

**Data sources (all from existing bot profile JSON):**

| Field | Source |
|-------|--------|
| Rating, rank | Leaderboard |
| Archetype | Strategy classifier from behavioral features (§12 evolution meta) |
| Win rate breakdown | D1 query: wins vs each archetype cluster |
| Signature | Most statistically distinctive behavior vs population average |
| Rival | From rival detection (§12.5) |
| Kill/energy/capture stats | Aggregate from match_participants |

**"Signature" computation:**

For each bot, compare its behavioral features (aggression, economy,
exploration, formation) to the population mean. The dimension where the
bot deviates most is its signature. Combined with map-type analysis:

```
if bot.aggression is 2σ above mean AND best_map_type == "4-player":
    signature = "Aggressive multi-front warfare on 4-player maps"
if bot.economy is 1.5σ above mean AND bot.exploration > 80%:
    signature = "Full-map economic dominance"
```

Template-generated from ~20 signature patterns.

**Sharing:**

- "Share Card" button on the bot profile page generates a PNG download
- Direct URL: `https://aicodebattle.com/card/{bot_id}.png`
  - Served as a static PNG from R2 (pre-rendered for top-50 bots)
  - Or rendered on-demand via a Worker that reads the bot profile JSON,
    draws to Canvas (using `@cloudflare/workers-types` Canvas API or
    a pre-built image template), and returns the PNG
- Open Graph tags on the URL so pasting it into Twitter/Discord/Slack
  shows the card as a rich preview:
  ```html
  <meta property="og:image" content="https://aicodebattle.com/card/b_4e8c1d2f.png" />
  <meta property="og:title" content="SwarmBot — #3 Rated — AI Code Battle" />
  ```
- The card image includes the platform URL as a watermark, driving traffic

---

## 14. Ecosystem & Polish

### 14.1 Weekly Meta Report (Blog Posts)

Every Monday, the platform publishes a "State of the Game" blog post — an
auto-generated analysis of the competitive landscape for the current season.

**Published to:** `/blog/meta-week-{N}-season-{S}` on the static site.

**Blog infrastructure:**

Blog posts are Markdown files stored in R2 (`blog/posts/{slug}.json`),
each containing:

```json
{
  "slug": "meta-week-12-season-4",
  "title": "Week 12 Meta Report — Season 4: The Colosseum",
  "date": "2026-03-23",
  "type": "meta-report",
  "content_md": "# Week 12 Meta Report\n\n## Dominant Strategies\n...",
  "summary": "Swarm tactics dominate as attack_radius2 increase favors formations...",
  "tags": ["meta-report", "season-4"]
}
```

The static site's `/blog` page fetches `blog/index.json` (list of all
posts) and renders them client-side with a Markdown renderer.

**Report contents:**

| Section | Data Source | Generation |
|---------|------------|------------|
| Dominant Strategies | Archetype distribution of top-20 bots | D1 query → template |
| Rising / Falling Bots | Biggest rating movers (±) this week | D1 query → template |
| Counter-Strategy Spotlight | Under-represented archetypes in top 20 | D1 query → LLM narrative |
| Map of the Week | Highest engagement map | D1 query → template |
| Evolution Highlights | Promotion count, best evolved bot, most novel attempt | D1 query → LLM narrative |
| Prediction Standings | Top 5 predictors, accuracy rates | D1 query → template |
| Season Progress | Weeks remaining, championship seedings | D1 query → template |

**Generation pipeline:**

1. Worker cron fires weekly (using one of the 5 cron slots — shares with
   the index rebuilder, running on a `if (dayOfWeek === 1)` check)
2. Queries D1 for all data points above
3. Template-fills the structured sections (strategy distribution, ratings,
   maps, predictions)
4. Sends the free-text sections (counter-strategy spotlight, evolution
   highlights) to a cheap LLM with the data context + a journalism-style
   prompt
5. Assembles the full Markdown post
6. Writes to R2 as a blog JSON file
7. Updates `blog/index.json`

**Cost:** one LLM call per week (~$0.05). Negligible.

**Why blog posts:** Blog posts are indexable by search engines (driving
organic traffic), shareable as URLs, and accumulate into a historical
record of the platform's competitive evolution. They also give the
platform a human-feeling editorial voice even though the content is
auto-generated.

### 14.2 Public Match Data (Static JSON)

All platform data is already pre-computed and stored as static JSON files
in R2. The "API" is simply **documented file paths** — no Worker
endpoints, no query parameters, no rate limiting needed.

**Documented data paths:**

```
DATA_BASE = https://data.aicodebattle.com

Leaderboard:
  GET {DATA_BASE}/data/leaderboard.json

Bot directory:
  GET {DATA_BASE}/data/bots/index.json
  GET {DATA_BASE}/data/bots/{bot_id}.json

Match history:
  GET {DATA_BASE}/data/matches/index.json
  GET {DATA_BASE}/data/matches/index-{page}.json   (older pages)
  GET {DATA_BASE}/data/matches/{match_id}.json

Replays:
  GET {DATA_BASE}/replays/{match_id}.json.gz

Maps:
  GET {DATA_BASE}/maps/index.json
  GET {DATA_BASE}/maps/{map_id}.json

Series:
  GET {DATA_BASE}/data/series/index.json
  GET {DATA_BASE}/data/series/{series_id}.json

Seasons:
  GET {DATA_BASE}/data/seasons/index.json
  GET {DATA_BASE}/data/seasons/{season_id}.json

Playlists:
  GET {DATA_BASE}/data/playlists/{slug}.json

Meta:
  GET {DATA_BASE}/data/meta/archetypes.json
  GET {DATA_BASE}/data/meta/rivalries.json

Evolution:
  GET {DATA_BASE}/data/evolution/live.json
  GET {DATA_BASE}/data/evolution/lineage.json
  GET {DATA_BASE}/data/evolution/meta.json

Blog:
  GET {DATA_BASE}/blog/index.json
  GET {DATA_BASE}/blog/posts/{slug}.json

Predictions:
  GET {DATA_BASE}/data/predictions/leaderboard.json
  GET {DATA_BASE}/data/predictions/open.json
```

**Replay format specification:**

Published at `/docs/replay-format` on the static site. Contains:

- JSON Schema file (`replay-schema-v{N}.json`) in R2 — third-party tools
  can validate replays programmatically
- Field-by-field documentation with types, semantics, and examples
- Versioning policy: additive changes only, matching the seasonal backward
  compatibility rules (§13.9). New fields may appear in future versions;
  old fields are never removed or renamed.
- Example replays for each version (downloadable from R2)
- Changelog of schema changes per season

**Documentation page** (`/docs/data`):

A static page listing every data path above with descriptions, update
frequency, and example `curl` commands. No authentication, no API keys,
no rate limiting — it's just static files.

**Why static JSON, not a Worker API:**

All this data already exists in R2 as part of the normal platform
operation. The index rebuilder cron already produces leaderboard.json,
bot profiles, match indexes, playlists, etc. Adding an API layer on top
would consume Worker invocations (limited to 100K/day on free tier) for
data that's already pre-computed and publicly readable. Static files
scale infinitely on R2 with zero egress cost.

Third-party tools just `fetch()` the URLs. If they need to poll for
updates, they check the `updated_at` field in each JSON file. Cache
headers on R2 objects guide freshness (leaderboard: 60s, match data:
immutable, bot profiles: 300s).

### 14.3 Accessibility Suite

**Color-blind safe palettes:**

The platform ships with two palette options. Users toggle between them
via a dropdown in the replay viewer toolbar. Preference persists in
localStorage.

| Players | Default | Color-Blind Safe (Tol) |
|---------|---------|----------------------|
| Player 1 | Blue (#2196F3) | Blue (#0077BB) |
| Player 2 | Red (#F44336) | Orange (#EE7733) |
| Player 3 | Green (#4CAF50) | Cyan (#009988) |
| Player 4 | Yellow (#FFEB3B) | Magenta (#EE3377) |
| Player 5 | Purple (#9C27B0) | Grey (#BBBBBB) |
| Player 6 | Teal (#009688) | Black (#000000) |

The Tol palette is designed by Paul Tol for maximum distinguishability
under protanopia, deuteranopia, and tritanopia.

**Shape-per-player (redundant encoding):**

Each player's bots are rendered with a distinct shape in addition to
color, ensuring identification without color vision:

| Player | Shape |
|--------|-------|
| 1 | Circle ● |
| 2 | Square ■ |
| 3 | Triangle ▲ |
| 4 | Diamond ◆ |
| 5 | Pentagon ⬠ |
| 6 | Hexagon ⬡ |

Shapes are visible in all three view modes (dots, territory, influence).
In territory/influence mode, bot sprites retain their shapes on top of
the colored overlay.

**Keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `Space` | Play / Pause |
| `←` / `→` | Step back / forward one turn |
| `Shift+←` / `Shift+→` | Jump 10 turns |
| `[` / `]` | Previous / Next critical moment |
| `1`–`5` | Speed preset (1×, 2×, 4×, 8×, 16×) |
| `V` | Cycle view mode (dots → territory → influence) |
| `F` | Cycle fog of war perspective |
| `T` | Toggle debug telemetry panel |
| `E` | Toggle event timeline |
| `C` | Toggle commentary subtitles |
| `?` | Show keyboard shortcuts overlay |

A "⌨️" icon in the toolbar opens the shortcuts reference as an overlay.

**High contrast mode:**

Toggled via toolbar or `H` key. Changes:
- Grid lines: thin grey → bold white
- Background: dark grey → pure black
- Bot sprites: add 2px white outline
- Territory/influence overlays: increase opacity from 30% to 50%
- Energy nodes: yellow → bright white with yellow border
- Walls: dark grey → medium grey with white border
- Dead bots: fading red → solid white X

**Reduced motion:**

Respects the `prefers-reduced-motion` CSS media query automatically.
When active:
- Energy node pulse animation → static icon
- Dead bot fade effect → instant removal
- Bot movement trails → disabled
- Combat flash → static highlight for one turn
- Replay speed presets remain (this is user-controlled motion, not
  decorative)

**Screen reader transcript:**

A "Transcript" button in the toolbar opens a text panel showing a
turn-by-turn summary generated from replay events:

```
Turn 87: Player 1 (SwarmBot) moved 8 bots east. Player 2 (HunterBot)
moved 3 bots south. Combat at (30,42): 2 SwarmBot units and 1 HunterBot
unit killed. SwarmBot collected energy at (25,38). Win probability:
SwarmBot 62%, HunterBot 38%.
```

Generated client-side from the replay data. ARIA live region announces
each turn's summary during auto-playback.

**Focus management:**

- All interactive elements have visible focus indicators (2px blue
  outline, offset by 2px for contrast)
- Tab order follows a logical flow: toolbar → canvas (focusable for
  keyboard shortcuts) → scrubber → controls
- Canvas receives focus on click; keyboard shortcuts only activate when
  canvas is focused (prevents conflicts with page-level shortcuts)
- Skip-to-content link at page top for screen reader users

### 14.4 Live Evolution Observatory

The evolution dashboard becomes a real-time observatory where visitors
watch the AI evolution system work — candidates being generated, tested,
rejected, and promoted.

**Data flow:**

The evolver on Rackspace writes a status file to R2 at each stage of
every evolution cycle:

```
PUT data.aicodebattle.com/data/evolution/live.json
```

Updated at every state transition: generation start, validation
complete, each evaluation match result, promotion decision. At ~15
minutes per cycle with ~5 state transitions, that's ~20 R2 writes
per hour (~14,400/month — 1.4% of the 1M Class A free limit).

**`live.json` schema:**

```json
{
  "updated_at": "2026-03-23T14:32:15Z",
  "cycle": {
    "generation": 847,
    "started_at": "2026-03-23T14:20:00Z",
    "phase": "evaluating",
    "candidate": {
      "id": "go-847-3",
      "island": "go",
      "language": "Go",
      "parents": [
        { "id": "go-831-1", "rating": 1580 },
        { "id": "go-839-2", "rating": 1540 }
      ],
      "community_hint": "try retreating when outnumbered 3:1",
      "validation": {
        "syntax": { "passed": true, "time_ms": 120 },
        "schema": { "passed": true, "time_ms": 450 },
        "smoke": { "passed": true, "time_ms": 3200 }
      },
      "evaluation": {
        "matches_total": 10,
        "matches_played": 4,
        "results": [
          { "opponent": "strategy-random", "won": true, "score": "5-1" },
          { "opponent": "strategy-swarm", "won": false, "score": "2-3" },
          { "opponent": "evo-go-g840", "won": true, "score": "4-2" },
          { "opponent": "strategy-hunter", "won": true, "score": "3-1" }
        ]
      }
    }
  },
  "recent_activity": [
    {
      "time": "2026-03-23T14:32:00Z",
      "generation": 847,
      "candidate": "go-847-2",
      "island": "go",
      "result": "rejected",
      "reason": "Nash gate: expected payoff -0.12 vs Nash mixture",
      "stage": "promotion"
    },
    {
      "time": "2026-03-23T14:28:00Z",
      "generation": 846,
      "candidate": "py-846-5",
      "island": "python",
      "result": "rejected",
      "reason": "Smoke test: crashed on turn 12",
      "stage": "validation"
    },
    {
      "time": "2026-03-23T14:25:00Z",
      "generation": 846,
      "candidate": "rs-846-1",
      "island": "rust",
      "result": "promoted",
      "bot_id": "evo-rs-g846",
      "initial_rating": 1500,
      "stage": "deployment"
    }
  ],
  "islands": {
    "python": { "population": 18, "best_rating": 1580, "best_bot": "evo-py-g820" },
    "go": { "population": 20, "best_rating": 1650, "best_bot": "evo-go-g831" },
    "rust": { "population": 17, "best_rating": 1520, "best_bot": "evo-rs-g846" },
    "mixed": { "population": 20, "best_rating": 1710, "best_bot": "evo-mx-g802" }
  },
  "totals": {
    "generations_total": 847,
    "candidates_today": 96,
    "promoted_today": 12,
    "promotion_rate_7d": 0.12,
    "highest_evolved_rating": 1710,
    "evolved_in_top_10": 3
  }
}
```

**Observatory page (`/evolution`):**

The static site polls `live.json` every 10 seconds and renders:

**Top bar: island overview**
```
┌────────────┬────────────┬────────────┬────────────┐
│  🐍 Python  │  🔵 Go      │  🦀 Rust    │  🔀 Mixed   │
│  pop: 18   │  pop: 20   │  pop: 17   │  pop: 20   │
│  best: 1580│  best: 1650│  best: 1520│  best: 1710│
└────────────┴────────────┴────────────┴────────────┘
```

**Center: current cycle status**

Shows the current candidate's progress through the pipeline as a
step indicator: `[Generate] → [✓ Syntax] → [✓ Schema] → [✓ Smoke] →
[Evaluating 4/10] → [Promotion?]`

Below that, a mini-results table showing the candidate's evaluation
matches as they complete: opponent, result, score.

If a community hint influenced this candidate's prompt, it's shown:
`💡 Community hint: "try retreating when outnumbered 3:1" (by tactician42)`

**Bottom: activity feed**

A scrolling log of recent evolution events, color-coded:
- 🟢 Promoted (green)
- 🔴 Rejected at validation (red)
- 🟡 Rejected at Nash gate (yellow)

Each entry shows the candidate ID, island, result, and reason.

**Tabs: lineage tree + meta chart**

- **Lineage tree**: interactive d3.js force-directed graph. Each node is
  a bot (evolved or built-in). Edges connect parents to children. Nodes
  are colored by island. Size proportional to rating. Click a node to
  see the bot's profile. The tree grows as new bots are promoted.

- **Meta shift chart**: stacked area chart (d3.js or Chart.js) showing
  the archetype distribution of the evolved population over generations.
  X-axis: generation number. Y-axis: percentage. Each archetype is a
  colored band. Watch strategies emerge, dominate, and get countered
  over time.

Both visualizations are built from `data/evolution/lineage.json` and
`data/evolution/meta.json` (already produced by the index rebuilder).
The live feed overlay is the only component that polls `live.json`.

### 14.5 Narrative Engine (Chronicles)

Auto-generated storylines from match data, published alongside the weekly
meta report as blog posts on `/blog`.

**Story arc detection:**

The weekly cron (same as the meta report, §14.1) scans D1 for active
story arcs:

| Arc Type | D1 Query Trigger |
|----------|-----------------|
| **Rise** | Bot gained ≥200 rating in the last 7 days |
| **Fall** | Bot lost ≥200 rating in the last 7 days |
| **Rivalry Intensifies** | Rivalry pair played 5+ matches this week with alternating wins |
| **Upset of the Week** | Biggest single-match rating gap where the underdog won |
| **Evolution Milestone** | Evolved bot reached a new all-time-high rating or entered top 5 |
| **Comeback** | Bot recovered ≥150 rating after a decline |
| **Season Narrative** | End of season (championship results, final standings) |

**Generation pipeline:**

1. Detect 3–5 active arcs from D1 queries
2. For each arc, compile context: bot profiles, rating history, key
   match IDs with scores, archetype data, rival relationships
3. Prompt a cheap LLM (Haiku-class):

```
Write a 200-word sports-journalism narrative about this event in the
AI Code Battle platform. Be dramatic but factual. Reference specific
matches. Write in present tense. Do not use emojis.

Arc type: Rise
Bot: evo-go-g31
Season: 4 (The Colosseum)
Rating: 1320 → 1580 over 7 days
Key matches:
  - Beat SwarmBot (#1, 1820) on "The Labyrinth" — score 4-2, turn 287
  - Won bo3 series vs HunterBot (#4, 1650) 2-1
  - Lost to GuardianBot (#2, 1720) by 1 point on "Open Expanse"
Archetype: hybrid swarm-gatherer
Origin: evolved, generation 31, Go island
Parents: evo-go-g28 (gatherer archetype) × evo-go-g25 (swarm archetype)
Community hint that influenced it: "combine tight formations with
energy-first opening"
```

4. Assemble output as a blog post JSON file with:
   - Headline (generated by LLM)
   - 200-word narrative
   - Embedded replay links for key matches
   - Bot profile card image (§13.10)
   - Rating chart (data for client-side rendering)
5. Write to R2: `blog/posts/{slug}.json`
6. Update `blog/index.json`

**Blog page (`/blog`):**

- Lists all posts reverse-chronologically
- Post types: `meta-report` and `chronicle` (story arcs)
- Each post renders as a full page with embedded replay widgets (§13.3)
  at key moments
- Tags for filtering: `meta-report`, `rise`, `fall`, `rivalry`, `upset`,
  `evolution`, `comeback`, `season-recap`

**Weekly output:** 1 meta report + 3–5 chronicles = 4–6 blog posts/week.

**Cost:** ~$0.05 per LLM call × 6 posts/week = ~$0.30/week, ~$1.30/month.

**Why it matters:** Chronicles transform raw match data into stories that
people share, discuss, and follow. "The Rise of evo-go-g31" is a headline
someone posts on Hacker News. "GathererBot's Decline" is a cautionary
tale that sparks strategy discussion. The narrative engine gives the
platform a *voice* — it feels alive, with characters and plot arcs, not
just numbers on a leaderboard.
