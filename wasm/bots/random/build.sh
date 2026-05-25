#!/bin/sh
# Build random.wasm from Go source
set -e
cd "$(dirname "$0")"
GOOS=js GOARCH=wasm go build -o ../../dist/random.wasm
echo "Built wasm/bots/random -> dist/random.wasm"
