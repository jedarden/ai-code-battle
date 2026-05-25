#!/bin/sh
# Build rusher.wasm from Rust source
set -e
cd "$(dirname "$0")"
wasm-pack build --target web --out-dir ../../dist/rusher
echo "Built wasm/bots/rusher -> dist/rusher"
