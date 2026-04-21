#!/bin/bash
# Build the Go game engine as WebAssembly for the browser sandbox.
# Outputs: web/public/wasm/engine.wasm

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
WASM_DIR="$PROJECT_ROOT/web/public/wasm"
GO_WASM="$WASM_DIR/engine.wasm"

mkdir -p "$WASM_DIR"

echo "Building Go WASM engine..."
cd "$PROJECT_ROOT"

GOOS=js GOARCH=wasm go build -o "$GO_WASM" ./cmd/acb-wasm

WASM_SIZE=$(du -h "$GO_WASM" | cut -f1)
echo "Built $GO_WASM ($WASM_SIZE)"
