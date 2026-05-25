#!/bin/sh
# Build swarm.wasm from AssemblyScript source
set -e
cd "$(dirname "$0")"
npm install
npx asc assembly/index.ts -b ../../dist/swarm.wasm
echo "Built wasm/bots/swarm -> dist/swarm.wasm"
