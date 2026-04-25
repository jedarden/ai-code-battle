# AI Code Battle

A competitive bot programming platform where participants write HTTP servers that control units on a grid world.

## Overview

AI Code Battle is a game simulation platform where:
- Participants write bots in any language that expose HTTP endpoints
- Bots compete on a toroidal (wrapping) grid world
- Matches are executed offline and presented as completed replays
- A web platform shows leaderboards, match history, and replay viewers

## Quick Start

### Prerequisites

- Go 1.21+ (for game engine and CLI tools)
- Node.js 18+ (for web and worker components)
- Docker (for containerized deployment)

### Running Locally

```bash
# Build CLI tools
go build ./cmd/acb-local
go build ./cmd/acb-mapgen

# Run a match between built-in bots
./acb-local -seed 42 -max-turns 100 -output replay.json -verbose

# Start web development server
cd web && npm install && npm run dev
# Open http://localhost:3000/
```

### Viewing Replays

1. Open the web app at `http://localhost:3000/`
2. Navigate to "Replay Viewer" in the menu
3. Load a replay JSON file or enter a URL

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
│   ├── acb-local/       # CLI match runner
│   ├── acb-mapgen/      # Map generator
│   ├── acb-worker/      # Match execution worker
│   └── acb-indexer/     # Index builder for static files
├── web/                 # Cloudflare Pages SPA
│   ├── src/
│   │   ├── pages/       # Page components
│   │   ├── replay-viewer.ts  # Canvas replay renderer
│   │   └── app.ts       # SPA entry point
│   └── public/          # Static assets
├── worker-api/          # Cloudflare Worker API
│   └── src/
│       ├── index.ts     # Router + cron dispatcher
│       ├── jobs.ts      # Job coordination
│       ├── bots.ts      # Bot management
│       └── glicko2.ts   # Rating system
├── bots/                # Strategy bot implementations
│   ├── random/          # Python - RandomBot
│   ├── gatherer/        # Go - GathererBot
│   ├── rusher/          # Rust - RusherBot
│   ├── guardian/        # PHP - GuardianBot
│   ├── swarm/           # TypeScript - SwarmBot
│   └── hunter/          # Java - HunterBot
└── docs/plan/           # Implementation plan
```

## Strategy Bots

| Bot | Language | Strategy |
|-----|----------|----------|
| RandomBot | Python | Random valid moves (baseline) |
| GathererBot | Go | Energy collection, avoid combat |
| RusherBot | Rust | Rush enemy cores aggressively |
| GuardianBot | PHP | Defend cores, cautious expansion |
| SwarmBot | TypeScript | Formation cohesion, group advance |
| HunterBot | Java | Target isolated enemies |

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed deployment instructions.

### Quick Deploy

```bash
# Start all strategy bots
docker-compose -f docker-compose.bots.yml up -d

# Start match workers
docker-compose -f docker-compose.workers.yml up -d
```

## Testing

```bash
# Go engine tests
go test ./engine/... -v

# Worker API tests
cd worker-api && npm test

# Index builder tests
cd cmd/acb-indexer && npm test
```

## Architecture

The platform uses a split architecture:

- **Cloudflare (free tier)**: Static site, API endpoints, D1 database, R2 storage
- **Rackspace Spot**: Match workers, bot containers, index builder

See [docs/plan/plan.md](docs/plan/plan.md) for the full implementation plan.

## License

MIT
