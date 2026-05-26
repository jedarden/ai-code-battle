# AI Code Battle - Python Starter Bot

A minimal Python bot for AI Code Battle. This starter kit includes:
- HTTP server with HMAC authentication
- Game state type definitions
- Stub strategy function (you fill this in!)
- Dockerfile for containerization

## Quick Start

1. Copy this bot to your own repository
2. Edit `strategy.py` to implement your bot's logic
3. Build and run locally: `docker build -t my-bot . && docker run -p 8080:8080 -e SHARED_SECRET=test my-bot`
4. Test: `curl http://localhost:8080/health` should return "OK"
5. Register your bot at https://ai-code-battle.pages.dev/#/register

## Strategy Interface

Edit `strategy.py` to implement your bot. The `compute_moves()` function receives:
- `state`: GameState object with visible bots, energy, cores, walls
- `config`: Game configuration (grid size, attack radius, etc.)

Return a list of move objects:
```python
{
    "position": {"row": 5, "col": 10},  # Current position of bot to move
    "direction": "N"                    # One of: "N", "E", "S", "W", or "" for no move
}
```

## Game Protocol

- Your bot receives POST `/turn` requests each turn with fog-filtered game state
- Request is signed with HMAC-SHA256 (verify for security)
- Respond with moves within 1 second timeout
- See `protocol.md` for full specification

## Deployment

Push to your container registry and register the bot with the platform.
