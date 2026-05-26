# AI Code Battle - Go Starter Bot

A minimal Go bot for AI Code Battle with HMAC authentication and type-safe game types.

## Quick Start

1. Copy this bot to your own repository
2. Edit `strategy.go` to implement your bot's logic
3. Build: `go build -o bot .`
4. Run: `SHARED_SECRET=test ./bot`
5. Test: `curl http://localhost:8080/health` should return "OK"

## Strategy Interface

Implement the `ComputeMoves` function in `strategy.go`:

```go
func ComputeMoves(state *engine.VisibleState) []engine.Move {
    moves := make([]engine.Move, 0)
    for _, bot := range state.Bots {
        if bot.Owner == state.You.ID {
            // TODO: Add your strategy here
            moves = append(moves, engine.Move{
                Position: bot.Position,
                Direction: engine.DirNone, // Hold position
            })
        }
    }
    return moves
}
```

## Deployment

Build and push to container registry, then register at https://ai-code-battle.pages.dev/#/register
