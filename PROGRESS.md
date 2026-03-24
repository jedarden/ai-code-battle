# AI Code Battle - Implementation Progress

## Current Phase: Phase 4 - Match Orchestration

**Status: ✅ COMPLETE**

### Phase 4 Progress

- [x] Cloudflare Worker project structure (`worker-api/`)
  - TypeScript + Wrangler configuration
  - D1 database schema (bots, matches, jobs, rating_history tables)
- [x] Glicko-2 rating system (`worker-api/src/glicko2.ts`)
  - Rating scale conversion
  - Rating updates after matches
  - Rating decay for inactive bots
  - Unit tests (17 tests)
- [x] Job coordination endpoints (`worker-api/src/jobs.ts`)
  - GET /api/jobs/next - Get next pending job
  - POST /api/jobs/:id/claim - Claim job for execution
  - POST /api/jobs/:id/heartbeat - Update job heartbeat
  - POST /api/jobs/:id/result - Submit match result
  - POST /api/jobs/:id/fail - Mark job as failed
- [x] Bot management endpoints (`worker-api/src/bots.ts`)
  - POST /api/register - Register new bot
  - GET /api/bots - List all bots
  - GET /api/bots/:id - Get bot details
  - PUT /api/bots/:id - Update bot
  - POST /api/rotate-key - Rotate API key
  - GET /api/leaderboard - Get leaderboard
- [x] Data export endpoint (`worker-api/src/export.ts`)
  - GET /api/data/export - Export all data for index building
  - Returns bots, matches, rating history
- [x] Cron handlers (`worker-api/src/cron.ts`)
  - Matchmaker (every minute) - Creates match jobs
  - Health checker (every 15 min) - Pings bot endpoints
  - Stale job reaper (every 5 min) - Reclaims timed-out jobs
- [x] Match worker container (`cmd/acb-worker/`)
  - Polls Worker API for pending jobs
  - Claims jobs and executes matches using game engine
  - Uploads replays to R2 via S3-compatible API
  - Sends heartbeats during match execution
  - Submits results back to Worker API
  - Retry logic with exponential backoff
  - API client tests (10 tests)
- [x] Index builder container (`cmd/acb-indexer/`)
  - Fetches data from Worker API via `/api/data/export`
  - Generates static JSON index files:
    - `leaderboard.json` - Sorted bot rankings
    - `bots/index.json` - Bot directory
    - `bots/{bot_id}.json` - Per-bot profiles with rating history
    - `matches/index.json` - Recent match list
  - Optional deploy to Cloudflare Pages
  - Unit tests (6 tests)

### Phase 3 Completed

### Phase 1 Completed

- [x] Go module initialization (`github.com/aicodebattle/acb`)
- [x] Project structure (`engine/`, `cmd/acb-local/`, `cmd/acb-mapgen/`)
- [x] Core types (`engine/types.go`)
- [x] Grid implementation (`engine/grid.go`) - Toroidal wrapping, distances, visibility
- [x] Game state (`engine/game.go`) - State management, fog of war
- [x] Turn execution (`engine/turn.go`) - Movement, combat, capture, energy, spawn
- [x] Replay writer (`engine/replay.go`) - Full replay JSON format
- [x] Match runner (`engine/match.go`) - Concurrent bot communication
- [x] Map generator (`cmd/acb-mapgen/`) - Rotational symmetry, connectivity validation
- [x] Unit tests - 32+ tests passing, determinism verified

### Phase 2 Completed

- [x] HMAC Authentication (`engine/auth.go`)
  - Request signing: `{match_id}.{turn}.{timestamp}.{sha256(body)}`
  - Response signing: `{match_id}.{turn}.{sha256(body)}`
  - Timestamp tolerance (30s) for replay attack prevention
  - Secret generation (256-bit, hex-encoded)
- [x] HTTP Bot Client (`engine/bot_http.go`)
  - HTTPBot implementing BotInterface
  - Per-turn timeout (3s default)
  - Crash detection (10 consecutive failures)
  - Move validation (position ownership, direction validity)
  - Response signature verification
- [x] Integration Tests (`engine/integration_test.go`)
  - Full HTTP match between mock bots
  - HMAC authentication round-trip
  - Response signing verification
- [x] Strategy Bot Implementations (6 languages)
  - **RandomBot** (Python) - Random moves, rating floor
  - **GathererBot** (Go) - Energy-focused, combat avoidance
  - **RusherBot** (Rust) - Aggressive core rushing
  - **GuardianBot** (PHP) - Defensive core protection
  - **SwarmBot** (TypeScript) - Formation-based combat
  - **HunterBot** (Java) - Target isolation and hunting

### Phase 3 Completed

- [x] Web project setup (`web/`)
  - TypeScript + Vite build tooling
  - Type definitions matching Go replay format
- [x] ReplayViewer class (`web/src/replay-viewer.ts`)
  - Canvas-based grid rendering
  - Bot, core, energy, wall visualization
  - Player color coding (6 distinct colors)
- [x] Playback controls
  - Play/pause toggle
  - Turn-by-step navigation (prev/next)
  - Turn scrubber slider
  - Speed control (20ms - 1000ms per turn)
  - Keyboard shortcuts (Space, arrows, Home/End)
- [x] Fog of War perspective toggle
  - Per-player visibility calculation
  - Vision radius from game config
- [x] Score overlay
  - Real-time scores per player
  - Energy held display
  - Player name with color indicator
- [x] Match info panel
  - Match ID, winner, turns, reason
- [x] Event log
  - Turn-by-turn event display
- [x] File/URL loading
  - Local file upload
  - Remote URL fetch

### Exit Criteria Progress

| Criterion | Status |
|-----------|--------|
| TypeScript Canvas-based replay viewer | ✅ Complete |
| Play/pause, scrub, speed control | ✅ Complete |
| Fog of war perspective toggle | ✅ Complete |
| Score overlay | ✅ Complete |
| Loads replay JSON from file or URL | ✅ Complete |

### Phase 4 Exit Criteria

| Criterion | Status |
|-----------|--------|
| Matchmaker cron creates jobs in D1 | ✅ Complete |
| Workers claim and execute matches | ✅ Complete |
| Replays land in R2 | ✅ Complete |
| Results flow into D1 | ✅ Complete |
| Ratings update via Glicko-2 | ✅ Complete |
| Leaderboard.json rebuilds automatically | ✅ Complete |
| Stale job reaper recovers from worker disappearance | ✅ Complete |

## Next Phase: Phase 5 - Web Platform

**Status: Ready to start**

## File Structure

```
ai-code-battle/
├── go.mod
├── engine/
│   ├── types.go        # Core data types
│   ├── grid.go         # Toroidal grid implementation
│   ├── game.go         # Game state management
│   ├── turn.go         # Turn execution phases
│   ├── replay.go       # Replay recording
│   ├── match.go        # Match runner
│   ├── bot_local.go    # Local bot interface
│   ├── bot_http.go     # HTTP bot client
│   ├── auth.go         # HMAC authentication
│   └── *_test.go       # Test files
├── cmd/
│   ├── acb-local/      # CLI match runner
│   ├── acb-mapgen/     # Map generator
│   ├── acb-worker/     # Match execution worker
│   │   ├── main.go      # Worker entry point
│   │   ├── api.go       # Worker API client
│   │   ├── api_test.go  # API client tests
│   │   └── r2.go        # R2 upload client
│   └── acb-indexer/    # Index builder
│       ├── package.json
│       ├── Dockerfile
│       └── src/
│           ├── index.ts       # Entry point
│           ├── api.ts         # Worker API client
│           ├── generator.ts   # Index file generator
│           ├── writer.ts      # File system writer
│           └── generator.test.ts
├── worker-api/
│   ├── package.json    # npm dependencies
│   ├── wrangler.toml   # Cloudflare Worker config
│   └── src/
│       ├── index.ts        # Router + cron dispatcher
│       ├── types.ts        # TypeScript types
│       ├── glicko2.ts      # Glicko-2 rating system
│       ├── glicko2.test.ts # Rating system tests
│       ├── jobs.ts         # Job coordination endpoints
│       ├── bots.ts         # Bot management endpoints
│       ├── export.ts       # Data export endpoint
│       └── cron.ts         # Cron handlers
├── web/
│   ├── package.json    # npm dependencies
│   ├── tsconfig.json   # TypeScript config
│   ├── vite.config.ts  # Vite bundler config
│   ├── index.html      # Replay viewer page
│   └── src/
│       ├── types.ts        # Replay type definitions
│       ├── replay-viewer.ts # Canvas viewer class
│       └── main.ts         # UI controller
├── bots/
│   ├── random/         # Python - RandomBot
│   ├── gatherer/       # Go - GathererBot
│   ├── rusher/         # Rust - RusherBot
│   ├── guardian/       # PHP - GuardianBot
│   ├── swarm/          # TypeScript - SwarmBot
│   └── hunter/         # Java - HunterBot
└── docs/
    └── plan/
        └── plan.md     # Full implementation plan
```

## Strategy Bot Summary

| Bot | Language | Strategy | Expected Rank |
|-----|----------|----------|---------------|
| RandomBot | Python | Random valid moves | 6th (floor) |
| GathererBot | Go | Energy collection, avoid combat | 4th-5th |
| RusherBot | Rust | Rush enemy cores aggressively | 4th-5th |
| GuardianBot | PHP | Defend cores, cautious expansion | 3rd-4th |
| SwarmBot | TypeScript | Formation cohesion, group advance | 1st-2nd |
| HunterBot | Java | Target isolated enemies | 1st-2nd |

## Running Tests

```bash
# Go engine tests
go test ./engine/... -v

# Web build verification
cd web && npm run build
```

## Building CLI Tools

```bash
go build ./cmd/acb-local
go build ./cmd/acb-mapgen
```

## Running a Match

```bash
./acb-local -seed 42 -max-turns 100 -output replay.json -verbose
```

## Viewing a Replay

```bash
cd web
npm run dev
# Open http://localhost:3000 and load replay.json
```
