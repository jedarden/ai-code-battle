#!/bin/sh
# Build guardian.wasm from Go source
set -e
cd "$(dirname "$0")"
GOOS=js GOARCH=wasm go build -o ../../dist/guardian.wasm
echo "Built wasm/bots/guardian -> dist/guardian.wasm"
