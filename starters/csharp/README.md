# acb-starter-csharp

C# (.NET) starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform.

Uses ASP.NET Core minimal API with zero external dependencies beyond the framework.

## Quick Start

```bash
# Run locally
export BOT_SECRET=dev-secret
dotnet run

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
    "name": "my-csharp-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Project Structure

```
Program.cs                    # HTTP server, HMAC auth, types, and strategy
Grid.cs                       # Grid utilities (toroidal distance, BFS, neighbors)
acb-starter-csharp.csproj    # .NET project file
Dockerfile                    # Container build
```

## Grid Helpers

`Grid.cs` provides static utility methods for the toroidal grid:

- `Grid.ToroidalManhattan(r1, c1, r2, c2, rows, cols)` — Manhattan distance with wrap-around
- `Grid.ToroidalChebyshev(r1, c1, r2, c2, rows, cols)` — Chebyshev distance with wrap-around
- `Grid.Neighbors(p, rows, cols)` — 8-directional neighbors with wrap
- `Grid.Bfs(start, goal, passable, rows, cols)` — BFS pathfinding, returns path or `null`

## Customization

Edit `ComputeMoves()` in `Program.cs` to implement your strategy. The `GameState` record provides:

- `Bots` — all visible bots (yours and enemies)
- `Energy` — visible energy pickup locations
- `Cores` — visible core positions
- `Walls` — visible wall positions
- `You.Energy` — your current energy count
- `You.Score` — your current score
- `Config` — match parameters (grid size, etc.)

Return a `List<Move>`, each with the bot's current `Position` and a `Direction` (`"N"`, `"E"`, `"S"`, or `"W"`). Bots not included in the response stay in place.

## Protocol

- **Endpoint:** `POST /turn` — receives game state JSON, returns moves JSON
- **Health:** `GET /health` — must return 200
- **Timeout:** 3 seconds per turn
- **Auth:** HMAC-SHA256 via `X-ACB-Signature` header
