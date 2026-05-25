#!/bin/sh
# Build gatherer.wasm from Go source
set -e
cd "$(dirname "$0")"
GOOS=js GOARCH=wasm go build -o ../../dist/gatherer.wasm
echo "Built wasm/bots/gatherer -> dist/gatherer.wasm"
