#!/bin/sh
# Build economist.wasm from Go source
set -e
cd "$(dirname "$0")"
mkdir -p ../../dist
GOOS=js GOARCH=wasm go build -o ../../dist/economist.wasm
echo "Built wasm/bots/economist -> dist/economist.wasm"
