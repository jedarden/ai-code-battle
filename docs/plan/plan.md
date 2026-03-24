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

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Web Platform                                │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────────┐ │
│  │  Leaderboard  │  │ Match History │  │    Replay Viewer (Canvas) │ │
│  └──────────────┘  └──────────────┘  └───────────────────────────┘ │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTPS
                    ┌──────────▼──────────┐
                    │     API Server       │
                    │  (user reg, bot reg, │
                    │  leaderboard, replay │
                    │   metadata, health)  │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
     ┌────────▼───────┐ ┌─────▼──────┐ ┌───────▼────────┐
     │   PostgreSQL    │ │  Match      │ │  Object Store  │
     │   (users, bots, │ │  Queue      │ │  (S3-compat)   │
     │   matches,      │ │  (Redis)    │ │  replay JSON   │
     │   ratings)      │ │             │ │  + map data     │
     └────────────────┘ └─────┬──────┘ └────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │   Match Workers    │  ← Rackspace Spot instances
                    │   (stateless,      │
                    │    interruptible)   │
                    └─────────┬─────────┘
                              │ HTTP (per-turn requests)
              ┌───────────────┼───────────────┐
              │               │               │
     ┌────────▼──────┐ ┌─────▼─────┐ ┌───────▼──────┐
     │ Participant    │ │ Built-in   │ │ Participant   │
     │ Bot A          │ │ Strategy   │ │ Bot B         │
     │ (external)     │ │ Bots       │ │ (external)    │
     └───────────────┘ │ (containers)│ └──────────────┘
                       └────────────┘
```

### Component Summary

| Component | Role | Scaling Model |
|-----------|------|---------------|
| API Server | REST API for web platform, bot registration, match metadata | Horizontally scaled, always-on |
| Match Worker | Pulls match jobs from queue, executes full game simulation, uploads replay | Stateless pods on Rackspace Spot |
| Tournament Scheduler | Creates match jobs based on matchmaking algorithm | Single process, cron-like |
| Web Frontend | Static SPA — replay viewer, leaderboard, registration | CDN / static hosting |
| Strategy Bots | Built-in HTTP bots (one container each) | Always-on, lightweight |
| PostgreSQL | Users, bots, matches, ratings | Single primary + read replica |
| Redis | Match job queue, rate limiting, caching | Single instance |
| Object Store | Replay JSON files, map definitions | S3-compatible (Minio or provider) |

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
ladder opponents. Each is deployed as its own container running a lightweight
HTTP server.

### 5.1 RandomBot

**Strategy:** Makes uniformly random valid moves each turn.

**Behavior:**
- For each owned bot, pick a random direction (N/E/S/W) or hold (20% chance)
- No pathfinding, no memory, no awareness of enemies
- Serves as the absolute baseline — any reasonable bot should beat this

**Value:** Ensures new participants have an easy opponent to test against.
Rating floor anchor.

### 5.2 GathererBot

**Strategy:** Maximize energy collection, avoid combat entirely.

**Behavior:**
- BFS from each owned bot to the nearest visible energy
- Assign each bot to the closest uncontested energy (greedy matching)
- If an enemy bot is within vision, move away from it
- Never voluntarily enters attack range of an enemy
- Spawns bots as fast as energy allows

**Value:** Tests whether aggressive bots can actually close games or whether
passive resource hoarding is dominant (it shouldn't be).

### 5.3 RusherBot

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

### 5.4 GuardianBot

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

### 5.5 SwarmBot

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

### 5.6 HunterBot

**Strategy:** Target isolated enemy bots for efficient kills.

**Behavior:**
- Identify enemy bots that are ≥4 tiles from their nearest friendly bot
  (isolated targets)
- Send pairs of bots to intercept isolated enemies (2v1 wins cleanly)
- If no isolated targets, default to gatherer behavior
- Maintain a map of known enemy positions, predict movement
- Avoid engaging formations of 3+ enemy bots

**Value:** Sophisticated target selection and prediction. Represents an
intermediate-skill bot. Should beat random/gatherer/rusher but struggle
against swarm formations.

### 5.7 Container Template

All strategy bots share a common container structure:

```
strategy-{name}/
├── Dockerfile
├── main.go                  # HTTP server, HMAC verification, JSON parsing
├── strategy.go              # Strategy-specific logic
├── game/
│   ├── state.go             # Game state types (deserialized from engine JSON)
│   ├── grid.go              # Grid utilities (BFS, distance, wrapping)
│   └── moves.go             # Move types (serialized to engine JSON)
└── go.mod
```

**Base HTTP server (shared across all bots):**
- Listens on port 8080
- `POST /turn` — receives game state, runs strategy, returns moves
- `GET /health` — returns 200 (used for registration health check)
- HMAC signature verification on incoming requests
- HMAC signature on outgoing responses
- Request logging (turn number, compute time, move count)

**Container spec:**
- Base image: `golang:1.24-alpine` (build) → `alpine:3.21` (runtime)
- Memory limit: 128MB
- CPU limit: 0.25 cores
- These are intentionally constrained — strategy bots should be lightweight

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

- Replays are stored in S3-compatible object storage (Minio self-hosted or
  provider-managed)
- Path: `replays/{year}/{month}/{match_id}.json.gz`
- Retention: indefinite for top-100 matches per month; older matches pruned
  after 90 days
- Map definitions stored separately: `maps/{map_id}.json`
- The API server returns signed URLs for replay access (no public bucket)

### 7.3 Browser Replay Viewer

The replay viewer is a client-side TypeScript application rendered on
HTML5 Canvas.

**Rendering pipeline:**
1. Fetch replay JSON from object storage (via signed URL from API)
2. Parse and index: build per-turn game state by replaying events from turn 0
3. Render the current turn to canvas
4. User controls advance/rewind the turn index

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

### 8.1 User Registration

- Email + password or OAuth (GitHub recommended — target audience is developers)
- Email verification required before bot registration
- Profile: username (unique), display name, avatar (from OAuth provider)

### 8.2 Bot Registration

**Registration flow:**

1. User navigates to "Register Bot" in their dashboard
2. Provides:
   - **Bot name** (unique, alphanumeric + hyphens, 3–32 chars)
   - **Endpoint URL** (HTTPS required for competitive play; HTTP allowed for
     development with a flag)
   - **Description** (optional, shown on leaderboard)
3. Platform generates:
   - `bot_id`: unique identifier (`b_` prefix + 8 hex chars)
   - `shared_secret`: 256-bit random, hex-encoded (64 chars)
4. Platform displays the shared secret **once** — user must copy it
5. Platform performs a **health check**: `GET {endpoint_url}/health`
   - Must return 200 within 5 seconds
   - If health check fails, registration is saved but bot is marked **inactive**
6. Platform performs a **protocol test**: sends a mock turn-0 game state to
   `POST {endpoint_url}/turn` with valid HMAC
   - Bot must return a valid (possibly empty) moves response within 3 seconds
   - If protocol test fails, bot is marked **inactive** with an error message

**Bot status lifecycle:**
```
PENDING → ACTIVE → INACTIVE (health check failed)
                  → SUSPENDED (manual by admin)
                  → RETIRED (by owner)
```

Only `ACTIVE` bots participate in matchmaking.

**Ongoing health checks:** the platform pings each active bot's `/health`
endpoint every 15 minutes. Three consecutive failures → marked `INACTIVE`.
Bots automatically return to `ACTIVE` when health checks resume passing.

### 8.3 Leaderboard

- Default sort: Glicko-2 display rating (mu - 2*phi) descending
- Columns: rank, bot name, owner, rating, games played, win rate, last active
- Filterable by: player count tier (2p, 3p, 4p, 6p), time range
- Updates in near-real-time (WebSocket push or 30-second polling)
- Public — no login required to view

### 8.4 Match History & Profiles

**Bot profile page** (`/bot/{bot_name}`):
- Current rating + rating history chart
- Recent matches (last 50) with links to replay viewer
- Win/loss/draw breakdown
- Performance vs. each opponent
- Bot description, owner, registration date

**User profile page** (`/user/{username}`):
- List of owned bots
- Aggregate statistics across all bots

**Match page** (`/match/{match_id}`):
- Participants, map, final scores
- Embedded replay viewer (auto-plays)
- Turn-by-turn event log (collapsible)

---

## 9. Deployment & Infrastructure

### 9.1 Container Architecture

| Image | Base | Purpose | Replicas |
|-------|------|---------|----------|
| `acb-api` | Go binary on Alpine | REST API server | 2 (always-on) |
| `acb-worker` | Go binary on Alpine | Match execution worker | 3–10 (spot) |
| `acb-scheduler` | Go binary on Alpine | Tournament matchmaking | 1 (always-on) |
| `acb-web` | Nginx + static files | Frontend SPA | 1 (or CDN) |
| `acb-strategy-random` | Go on Alpine | RandomBot | 1 |
| `acb-strategy-gatherer` | Go on Alpine | GathererBot | 1 |
| `acb-strategy-rusher` | Go on Alpine | RusherBot | 1 |
| `acb-strategy-guardian` | Go on Alpine | GuardianBot | 1 |
| `acb-strategy-swarm` | Go on Alpine | SwarmBot | 1 |
| `acb-strategy-hunter` | Go on Alpine | HunterBot | 1 |

### 9.2 Rackspace Spot Deployment

Match workers are the primary consumers of compute and are **perfectly suited
for spot instances**:

- **Stateless**: workers pull jobs from a queue, execute, and push results.
  No persistent local state.
- **Interruptible**: if a spot instance is reclaimed mid-match, the match job
  is re-queued after a staleness timeout (10 minutes with no progress update).
  The match is replayed from scratch on another worker.
- **Bursty**: match throughput can flex with spot availability. More instances
  = faster ladder convergence, but no hard deadline.

**Instance sizing:**
- Match workers: 2 vCPU, 4 GB RAM per instance (each runs one match at a time)
- Strategy bots: can share a single small instance (all 6 use <256MB total)
- API server + scheduler: 2 vCPU, 4 GB RAM, always-on (not spot)

**Deployment layout:**

```
Always-on tier (standard instances):
├── acb-api (×2, behind load balancer)
├── acb-scheduler (×1)
├── acb-web (×1 or CDN)
├── acb-strategy-* (×1 each, shared instance)
├── PostgreSQL (managed or self-hosted)
├── Redis (managed or self-hosted)
└── Minio / S3-compatible store

Spot tier (preemptible instances):
├── acb-worker (×3 minimum, scale up as available)
└── (each worker is a standalone container, no coordination needed)
```

**Spot reclaim handling:**
1. Worker registers a shutdown hook that catches SIGTERM
2. On SIGTERM, worker sets the current match status to `interrupted` in Redis
3. Worker exits gracefully (within the 30-second SIGTERM grace period)
4. Scheduler's stale-match reaper detects `interrupted` or stale `in_progress`
   matches and re-queues them
5. Another worker picks up the job

### 9.3 Data Stores

**PostgreSQL:**
- Tables: `users`, `bots`, `matches`, `match_participants`, `maps`, `ratings`
- Single primary instance; read replica for leaderboard queries
- Connection pooling via PgBouncer
- Backup: daily automated dumps to object storage

**Redis:**
- Match job queue (Redis Streams or List-based queue)
- Rate limiting (per-bot, per-endpoint)
- Session cache
- Leaderboard cache (sorted sets)
- No persistence required — queue jobs are recoverable from PostgreSQL match
  records with `queued` status

**Object Storage (S3-compatible):**
- Replay files (gzipped JSON)
- Map definition files
- Bot submission metadata / logs
- Signed URL generation for replay access (1-hour expiry)

### 9.4 Networking & Security

**External traffic:**
- Web platform: HTTPS only, behind reverse proxy (Caddy or nginx)
- Bot endpoints: engine connects outbound to registered URLs

**Internal traffic:**
- API ↔ PostgreSQL: private network
- API ↔ Redis: private network
- Workers ↔ Redis: private network (workers may be in different regions — use
  Redis over TLS if cross-region)
- Workers → bot endpoints: public internet (HTTPS required for competitive bots)
- Workers → strategy bots: private network (same infrastructure)

**Security boundaries:**
- The game engine (workers) never executes bot code — HTTP only
- All bot responses are schema-validated before processing
- HMAC authentication prevents request/response forgery
- Rate limiting on API endpoints (registration, health checks)
- Bot endpoint URLs validated at registration (no internal IPs, no localhost)
- Workers run with no inbound ports — they only make outbound HTTP calls

### 9.5 Monitoring

| Signal | Tool | Alert Threshold |
|--------|------|-----------------|
| Match throughput | Prometheus counter | <10 matches/hour for >30 minutes |
| Worker count | Prometheus gauge | <2 live workers for >15 minutes |
| Bot health check failures | Prometheus counter | >50% of active bots failing |
| API latency (p99) | Prometheus histogram | >500ms |
| Match queue depth | Redis metric | >100 pending matches |
| Replay storage usage | S3 metric | >80% of quota |
| Error rate (5xx) | Access logs | >1% of requests |

---

## 10. Implementation Phases

### Phase 1: Core Engine (foundation)

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
- HMAC signing and verification library
- Strategy bot container template (shared HTTP server + game state types)
- All 6 strategy bots implemented and containerized
- Integration test: engine runs a full match between two containerized bots

**Exit criteria:** can run a complete match between any two strategy bot
containers over HTTP, with HMAC authentication, producing a valid replay.

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
- Match worker service (`acb-worker`): pulls from Redis queue, runs matches,
  uploads replays, records results
- Tournament scheduler (`acb-scheduler`): matchmaking algorithm, creates jobs
- PostgreSQL schema and migrations
- Stale match reaper (handles interrupted spot instances)
- Match result → Glicko-2 rating update pipeline

**Exit criteria:** scheduler creates matches, workers execute them
autonomously, ratings update, replays are stored. System recovers from
worker interruption.

### Phase 5: Web Platform

**Deliverables:**
- API server (`acb-api`): user registration, bot registration, leaderboard,
  match history, replay URLs
- Web frontend (`acb-web`): registration, bot management dashboard,
  leaderboard, match history, embedded replay viewer
- Bot health check system (periodic + on-registration)
- Shared secret generation, display, rotation

**Exit criteria:** a user can register, add a bot, see it appear on the
leaderboard after matches are played, and watch replays of its games.

### Phase 6: Deployment & Production

**Deliverables:**
- Container images pushed to registry
- Rackspace Spot deployment for workers
- Always-on deployment for API, scheduler, strategy bots, datastores
- TLS termination, DNS, CDN for static assets
- Monitoring dashboards and alerts
- Backup automation for PostgreSQL and replay storage

**Exit criteria:** platform is publicly accessible, matches run continuously,
strategy bots compete on the ladder, external participants can register and
play.
