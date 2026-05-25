#!/bin/sh
# Build hunter.wasm from Go source
set -e
cd "$(dirname "$0")"
GOOS=js GOARCH=wasm go build -o ../../dist/hunter.wasm
echo "Built wasm/bots/hunter -> dist/hunter.wasm"
