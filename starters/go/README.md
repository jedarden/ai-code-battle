# acb-starter-go

Go starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform where you write HTTP servers that control units on a grid world.

## Quick Start

```bash
# Set your bot secret (required for HMAC authentication)
export BOT_SECRET=your-secret-here

# Run locally
go run main.go

# Or build and run
go build -o bot && ./bot
```

Your bot listens on port 8080 and responds to `POST /turn` with move commands.

## Run with Docker

```bash
# Build the container
docker build -t my-go-bot .

# Run the container
docker run -e BOT_SECRET=your-secret-here -p 8080:8080 my-go-bot
```

## Project Structure

```
.
├── main.go       # HTTP server, request/response handling
├── game/         # Shared package (reuse in other projects!)
│   ├── types.go  # Game state types and constants
│   ├── auth.go   # HMAC signature verification
│   └── grid.go   # Grid utilities (distance, BFS, neighbors)
├── go.mod        # Go module definition
├── Dockerfile    # Multi-stage container build
└── README.md     # This file
```

## Implement Your Strategy

Edit `computeMoves()` in `main.go` to implement your bot's strategy. The function receives a `game.GameState` struct with:

- **`state.Bots`** — all visible bots (yours and enemies)
- **`state.Energy`** — visible energy pickup locations
- **`state.Cores`** — visible core positions
- **`state.Walls`** — visible wall positions
- **`state.You.ID`** — your player ID (0, 1, 2, etc.)
- **`state.You.Energy`** — your current energy count
- **`state.You.Score`** — your current score
- **`state.Config`** — match parameters (grid size, attack range, etc.)

Return a slice of `game.Move` structs, each with:
- **`Position`** — your bot's current position (row, col)
- **`Direction`** — one of: `"N"`, `"E"`, `"S"`, `"W"`

Any bot not included in the response stays in place.

### Example Strategy

```go
func computeMoves(state *game.GameState) []game.Move {
    var moves []game.Move

    for _, bot := range state.Bots {
        if bot.Owner != state.You.ID {
            continue
        }

        // Find nearest energy and move toward it
        if len(state.Energy) > 0 {
            nearest := state.Energy[0]
            bestDist := game.ToroidalManhattan(bot.Position, nearest,
                state.Config.Rows, state.Config.Cols)

            for _, e := range state.Energy[1:] {
                dist := game.ToroidalManhattan(bot.Position, e,
                    state.Config.Rows, state.Config.Cols)
                if dist < bestDist {
                    bestDist = dist
                    nearest = e
                }
            }

            // Use BFS to find direction toward energy
            passable := func(p game.Position) bool {
                // Simple passable check (you can add wall checking)
                return true
            }
            dir := game.BFSDirection(bot.Position, nearest, passable,
                state.Config.Rows, state.Config.Cols)

            if dir != "" {
                moves = append(moves, game.Move{
                    Position:  bot.Position,
                    Direction: dir,
                })
            }
        }
    }

    return moves
}
```

## Grid Utilities

The `game` package provides utility functions for the toroidal grid:

### Distance Calculations

- **`game.ToroidalManhattan(a, b, rows, cols)`** — Manhattan distance with wrap-around
- **`game.ToroidalDistance2(a, b, rows, cols)`** — Squared Euclidean distance with wrap

### Movement

- **`game.Neighbors(pos, rows, cols)`** — 4 cardinal neighbors (N, E, S, W)
- **`game.NeighborInDirection(pos, dir, rows, cols)`** — Move one step in a direction
- **`game.AllNeighbors(pos, rows, cols)`** — 8-directional neighbors (including diagonals)

### Pathfinding

- **`game.BFSDirection(start, goal, passable, rows, cols)`** — BFS pathfinding, returns the first direction to move (or empty string if no path)

The `passable` function should return `true` for positions the bot can enter (e.g., not walls).

## Authentication

The bot uses HMAC-SHA256 signatures for authentication:

- **Request verification**: Validates the `X-ACB-Signature` header from the engine
- **Response signing**: Adds `X-ACB-Signature` to all responses
- **Timestamp validation**: Rejects requests with timestamps outside ±30 seconds

The `BOT_SECRET` environment variable must be set. This secret is shared only between you and the game engine — never expose it publicly.

## Register Your Bot

Once your bot is deployed and accessible via HTTPS:

```bash
curl -X POST https://api.aicodebattle.com/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-go-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome Go bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Protocol

- **`POST /turn`** — Main game loop endpoint (receives game state, returns moves)
- **`GET /health`** — Health check (must return 200 OK)
- **Timeout**: 3 seconds per turn
- **Authentication**: HMAC-SHA256 via `X-ACB-Signature` header

## Tips

- **The grid wraps around** — use the toroidal distance functions for accurate calculations
- **Fog of war** — you only see tiles within your vision radius
- **Energy spawns periodically** — check `state.Config.EnergyInterval`
- **Spawn cost** — spawning a bot costs 3 energy (`state.Config.SpawnCost`)
- **Combat** — bots within `AttackRadius2` may be destroyed each turn

## Further Reading

- [Full Game Protocol](https://aicodebattle.com/docs/protocol)
- [Replay Format](https://aicodebattle.com/docs/replay-format)
- [Strategy Guide](https://aicodebattle.com/docs/strategy)

## License

MIT — feel free to use this starter kit as a template for your own bot.
