# AI Code Battle - Rust Starter Bot

A minimal Rust bot for AI Code Battle using Axum framework with type-safe game types.

## Quick Start

1. Copy this bot to your own repository
2. Edit `src/strategy.rs` to implement your bot's logic
3. Build: `cargo build --release`
4. Run: `SHARED_SECRET=test ./target/release/starter-bot`
5. Test: `curl http://localhost:8080/health` should return "OK"

## Strategy Interface

Implement the `compute_moves` function in `src/strategy.rs`:

```rust
pub fn compute_moves(state: &VisibleState) -> Vec<Move> {
    state.bots.iter()
        .filter(|bot| bot.owner == state.you.id)
        .map(|bot| Move {
            position: bot.position,
            direction: Direction::None,
        })
        .collect()
}
```

## Deployment

Build a release binary, package in Docker container, push to registry, and register at https://ai-code-battle.pages.dev/#/register
