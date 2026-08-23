# WASM Bot Builds

This directory contains the WebAssembly builds for the browser sandbox.
Each bot compiles to a separate WASM module with the standard interface.

## Bot WASM Interface

Each WASM module exports three functions:

```javascript
// Initialize the bot with game config
bot.init(configJSON: string): string // {ok: bool, error?: string}

// Compute moves for the current turn
bot.compute_moves(stateJSON: string): string // moves JSON array

// Free result (no-op for Go/AS, required for interface compatibility)
bot.free_result(ptr: number): void
```

## Directory Structure

Original sandbox roster:

```
wasm/bots/
├── gatherer/     # Go → WASM (energy-focused, avoids combat)
├── random/       # Go → WASM (random moves)
├── guardian/     # Go → WASM (defends own cores)
├── hunter/       # Go → WASM (hunts nearest enemy)
├── rusher/       # Rust → WASM (attacks enemy cores)
└── swarm/        # TypeScript/AssemblyScript → WASM (tight formations)
```

Extended roster (plan §13.1, ports of the ladder bots under `bots/`):

```
wasm/bots/
├── farmer/       # Go → WASM (max energy income, avoids combat)
├── opportunist/  # Go → WASM (contests energy nodes near enemies)
├── siege/        # Go → WASM (surrounds and grinds enemy cores)
├── economist/    # Go → WASM (spawn-heavy compounding; port of the Python ladder bot)
├── assassin/     # Rust → WASM (all units rush the enemy core)
├── phalanx/      # Rust → WASM (formation advance, mutual support)
└── zone-driver/  # Rust → WASM (controls territory zones)
```

The shipped sandbox artifacts under `web/public/wasm/bots/` are built from
the Go ports in `cmd/acb-wasm/` (see `cmd/acb-wasm/build.sh`), which cover
the full thirteen-bot roster behind one loader; this directory keeps the
per-language reference builds.

## Building

Build all bots:
```bash
cd wasm/bots
make all
```

Build individual bot:
```bash
cd wasm/bots/gatherer
./build.sh
```

Output directory: `wasm/dist/`

## Bot Sizes (estimated)

| Bot | Language | Size |
|-----|----------|------|
| gatherer | Go → WASM | ~12 MB |
| random | Go → WASM | ~10 MB |
| guardian | Go → WASM | ~12 MB |
| hunter | Go → WASM | ~12 MB |
| rusher | Rust → WASM | ~3 MB |
| swarm | AssemblyScript → WASM | ~5 MB |
| farmer | Go → WASM | ~12 MB |
| opportunist | Go → WASM | ~12 MB |
| siege | Go → WASM | ~12 MB |
| economist | Go → WASM | ~12 MB |
| assassin | Rust → WASM | ~100 KB |
| phalanx | Rust → WASM | ~100 KB |
| zone-driver | Rust → WASM | ~100 KB |

## Plan Reference

This implements plan §11.1 lines 2566-2576 and plan §13.1 WASM sandbox.
