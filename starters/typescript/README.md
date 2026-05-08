# acb-starter-typescript

TypeScript/Node.js starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform.

Fastify server with full TypeScript type definitions for the game protocol.

## Quick Start

```bash
# Install dependencies
npm install

# Build TypeScript
npm run build

# Run locally (requires BOT_SECRET)
BOT_SECRET=your-secret npm start

# Or run in development mode (builds and starts)
npm run dev
```

## Run with Docker

```bash
# Build image
docker build -t my-ts-bot .

# Run container
docker run -e BOT_SECRET=your-secret -p 8080:8080 my-ts-bot
```

## Register Your Bot

Once your bot is deployed and accessible via HTTPS:

```bash
curl -X POST https://api.aicodebattle.com/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-ts-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome TypeScript bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Project Structure

```
src/
├── index.ts    # Fastify HTTP server, HMAC auth, request handling
├── types.ts    # Complete TypeScript type definitions for the protocol
├── auth.ts     # HMAC signing/verification utilities
├── strategy.ts # Bot strategy - implement your logic here
└── grid.ts     # Toroidal grid utilities (distance, BFS, neighbors)
package.json
tsconfig.json
Dockerfile
```

## Type Definitions

The `src/types.ts` file contains complete TypeScript definitions for:

- `GameState` - Fog-filtered game state sent each turn
- `GameConfig` - Match configuration parameters
- `PlayerInfo` - Your player information (energy, score)
- `Move` - Movement order for a single bot
- `MoveResponse` - Response format with optional debug telemetry
- `Direction` - Cardinal direction type ("N" | "E" | "S" | "W")

## Grid Helpers

`src/grid.ts` provides utility functions for the toroidal grid:

- `toroidalManhattan()` - Manhattan distance with wrap-around
- `toroidalChebyshev()` - Chebyshev distance with wrap-around
- `toroidalDistanceSquared()` - Squared Euclidean distance (faster for comparisons)
- `neighbors()` - 8-directional neighbors with wrap
- `cardinalNeighbors()` - 4 cardinal neighbors with direction labels
- `bfs()` - BFS pathfinding, returns path or null
- `findNearest()` - Find closest target from a list

## Customization

Edit `computeMoves()` in `src/strategy.ts` to implement your strategy.

The `state` object provides:

- `state.bots` - All visible bots (yours and enemies)
- `state.energy` - Visible energy pickup locations
- `state.cores` - Visible core positions
- `state.walls` - Visible wall positions
- `state.you.energy` - Your current energy count
- `state.you.score` - Your current score
- `state.config` - Match parameters (grid size, etc.)

Return an array of `Move` objects, each with:
- `row`, `col` - Your bot's current position
- `direction` - One of "N", "E", "S", or "W"

Bots not included in the response stay in place.

## Debug Telemetry

Optional debug info can be included in your response for replay visualization:

```typescript
const response: MoveResponse = {
  moves: computedMoves,
  debug: {
    reasoning: "Moving toward energy at (15, 20)",
    targets: [
      { row: 15, col: 20, label: "energy", priority: 0.9 }
    ],
    values: {
      energy_reserves: state.you.energy,
      mode: "gathering"
    }
  }
};
```

Debug data is stored in replays but never parsed by the engine.

## Protocol

- **Endpoint:** `POST /turn` — receives game state JSON, returns moves JSON
- **Health:** `GET /health` — must return 200 (used during registration)
- **Timeout:** 3 seconds per turn
- **Auth:** HMAC-SHA256 via `X-ACB-Signature` header

## Development

```bash
# Type checking without emitting
npm run typecheck

# Build for production
npm run build

# Start production server
BOT_SECRET=dev-secret npm start
```
