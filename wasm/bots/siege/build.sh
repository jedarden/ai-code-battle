#!/bin/sh
# Build siege.wasm from Go source
set -e
cd "$(dirname "$0")"
mkdir -p ../../dist
GOOS=js GOARCH=wasm go build -o ../../dist/siege.wasm
echo "Built wasm/bots/siege -> dist/siege.wasm"
