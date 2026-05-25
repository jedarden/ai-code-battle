#!/bin/sh
# Build swarm.wasm from AssemblyScript source
set -e
cd "$(dirname "$0")"
mkdir -p ../../dist
npm install
npx asc index.ts -o build/swarm.wasm --runtime incremental
cp build/swarm.wasm ../../dist/swarm.wasm
echo "Built wasm/bots/swarm -> dist/swarm.wasm"
