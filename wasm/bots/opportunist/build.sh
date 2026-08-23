#!/bin/sh
# Build opportunist.wasm from Go source
set -e
cd "$(dirname "$0")"
mkdir -p ../../dist
GOOS=js GOARCH=wasm go build -o ../../dist/opportunist.wasm
echo "Built wasm/bots/opportunist -> dist/opportunist.wasm"
