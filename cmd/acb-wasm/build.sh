#!/usr/bin/env bash
# Build the WASM engine and the six built-in bot WASM modules.
#
# Usage:
#   ./cmd/acb-wasm/build.sh
#
# Outputs are written to web/public/wasm/:
#   engine.wasm           – game engine with loadState/step/runMatch API
#   wasm_exec.js          – Go WASM runtime shim (copied from GOROOT)
#   bots/random.wasm      – Random strategy
#   bots/gatherer.wasm    – Gatherer strategy
#   bots/rusher.wasm      – Rusher strategy
#   bots/guardian.wasm    – Guardian strategy
#   bots/swarm.wasm       – Swarm strategy
#   bots/hunter.wasm      – Hunter strategy
#
# The bot WASM files implement the ACB WASM bot interface:
#   acbBot.init(configJSON)           – initialise for a new match
#   acbBot.compute_moves(stateJSON)   – return movesJSON for the current turn
#
# Prerequisites: Go 1.21+ with WASM support.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$REPO_ROOT/web/public/wasm"

mkdir -p "$OUT/bots"

echo "Building engine.wasm…"
GOOS=js GOARCH=wasm go build \
  -o "$OUT/engine.wasm" \
  ./cmd/acb-wasm/

echo "Copying wasm_exec.js…"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$OUT/"

echo "Building bot WASM modules…"
for bot in random gatherer rusher guardian swarm hunter; do
  echo "  → bots/${bot}.wasm"
  GOOS=js GOARCH=wasm go build \
    -o "$OUT/bots/${bot}.wasm" \
    "./cmd/acb-wasm/botmain/${bot}/"
done

echo "Done. Outputs in $OUT"
