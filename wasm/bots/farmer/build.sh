#!/bin/sh
# Build farmer.wasm from Go source
set -e
cd "$(dirname "$0")"
mkdir -p ../../dist
GOOS=js GOARCH=wasm go build -o ../../dist/farmer.wasm
echo "Built wasm/bots/farmer -> dist/farmer.wasm"
