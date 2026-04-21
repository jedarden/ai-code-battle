# WASM Bot Interface Specification

Version: 1.0
Last Updated: 2025-04-21

## Overview

The AI Code Battle sandbox supports WASM-based bots written in any language that compiles to WebAssembly. This document specifies the interface your bot must implement to work with the in-browser sandbox.

## Interface

Your WASM module must export a global `acbBot` object with two functions:

### `init(configJSON: string): void`

Called once at the start of the match, before any turns.

**Parameters:**
- `configJSON`: JSON string containing the game configuration

**Config Schema:**
```json
{
  "rows": 30,
  "cols": 30,
  "max_turns": 200,
  "vision_radius2": 49,
  "attack_radius2": 5,
  "spawn_cost": 3,
  "energy_interval": 10
}
```

**Purpose:** Initialize your bot's internal state (data structures, caches, etc.)

### `compute_moves(stateJSON: string): string`

Called each turn. Returns your bot's moves as a JSON string.

**Parameters:**
- `stateJSON`: JSON string containing the visible game state (fog-filtered)

**Visible State Schema:**
```json
{
  "match_id": "m_abc123",
  "turn": 42,
  "config": { /* same as init */ },
  "you": {
    "id": 0,
    "energy": 7,
    "score": 12
  },
  "bots": [
    { "position": {"row": 10, "col": 15}, "owner": 0 },
    { "position": {"row": 12, "col": 15}, "owner": 1 }
  ],
  "energy": [
    {"row": 20, "col": 25}
  ],
  "cores": [
    {"position": {"row": 5, "col": 5}, "owner": 0, "active": true}
  ],
  "walls": [
    {"row": 10, "col": 10}
  ],
  "dead": []
}
```

**Return Value:**
JSON string representing an array of moves:
```json
[
  {"position": {"row": 10, "col": 15}, "direction": "N"},
  {"position": {"row": 12, "col": 15}, "direction": "E"}
]
```

**Move Schema:**
- `position`: The current location of a bot you own
- `direction`: One of `"N"`, `"E"`, `"S"`, `"W"`, or `""` (hold position)

## Language-Specific Guides

### Go

```go
//go:build js && wasm

package main

import (
    "encoding/json"
    "syscall/js"
)

func main() {
    js.Global().Set("acbBot", js.ValueOf(map[string]interface{}{
        "init": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
            // Parse config, initialize state
            return nil
        }),
        "compute_moves": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
            // Parse state, compute moves, return JSON string
            return "[]"
        }),
    }))

    select {} // Keep WASM alive
}
```

**Build:**
```bash
GOOS=js GOARCH=wasm go build -o mybot.wasm .
```

**Upload:** Use the "Upload WASM" button in the sandbox.

### Rust

```rust
use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub struct AcbBot {
    config: Option<Config>,
}

#[wasm_bindgen]
impl AcbBot {
    #[wasm_bindgen(constructor)]
    pub fn new() -> Self {
        Self { config: None }
    }

    pub fn init(&mut self, config_json: &str) {
        // Parse and store config
    }

    pub fn compute_moves(&self, state_json: &str) -> String {
        // Parse state, compute moves, return JSON
        "[]".to_string()
    }
}
```

**Build:**
```bash
wasm-pack build --target web --out-file mybot.wasm
```

### TypeScript (AssemblyScript)

```typescript
// asconfig.json
{
  "extends": "node_modules/assemblyscript/std/assembly.json",
  "include": ["**/*.ts"],
  "imports": {
    "acb-bot": "./acb-bot.ts"
  }
}

// assembly/index.ts
import { Config, VisibleState, Move } from "acb-bot";

let config: Config;

export function init(configJSON: string): void {
  config = JSON.parse(configJSON) as Config;
}

export function compute_moves(stateJSON: string): string {
  const state = JSON.parse(stateJSON) as VisibleState;
  const moves: Move[] = [];
  // ... compute moves
  return JSON.stringify(moves);
}
```

**Build:**
```bash
asc assembly/index.ts -b mybot.wasm \
  --runtime stub \
  --use Date=Date \
  --exportRuntime
```

## Quick Start

1. Clone the bot template from `cmd/acb-wasm/bot-template/`
2. Modify the `computeMoves` function with your strategy
3. Build: `GOOS=js GOARCH=wasm go build -o mybot.wasm .`
4. Open the sandbox page and click "Upload WASM"
5. Select your `.wasm` file
6. Click "Run Match" to test against built-in opponents

## Memory Constraints

- Desktop browsers typically have 2-4 GB available for WASM
- Mobile browsers have ~500 MB - 1 GB
- The Go engine + one bot is ~15-20 MB
- Keep your bot's memory usage reasonable (<50 MB recommended)

## Testing Locally

You can test your bot without uploading:

```bash
# Build your bot
GOOS=js GOARCH=wasm go build -o testbot.wasm .

# Copy to public directory
cp testbot.wasm web/public/wasm/

# Update sandbox page to load from /wasm/testbot.wasm
```

## Troubleshooting

**"Go WASM runtime not loaded"**
- The sandbox should automatically load wasm_exec.js. If you see this error, ensure web/public/wasm/wasm_exec.js exists.

**"acbBot.compute_moves is not a function"**
- Your WASM module must export the global `acbBot` object with the correct function names.

**Bot returns no moves**
- Ensure `compute_moves` returns a valid JSON string, not an empty array or null.

**Bot crashes silently**
- Check the browser console (F12) for error messages. Use `console.log` or equivalent for debugging.

## Example Bots

Full example implementations are available at:
- `cmd/acb-wasm/bot-template/` - Go starter bot
- `cmd/acb-wasm/botmain/` - Built-in strategy bots (gatherer, rusher, etc.)

## Further Reading

- [Go WebAssembly](https://go.dev/wiki/WebAssembly)
- [Rust wasm-bindgen](https://rustwasm.github.io/wasm-bindgen/)
- [AssemblyScript](https://www.assemblyscript.org/)
