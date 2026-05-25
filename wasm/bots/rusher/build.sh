#!/bin/sh
# Build rusher.wasm from Rust source
# Uses low-level WASM interface compatible with the sandbox loader
set -e
cd "$(dirname "$0")"
cargo build --target wasm32-unknown-unknown --release
cp target/wasm32-unknown-unknown/release/rusher_wasm.wasm ../../dist/rusher.wasm
echo "Built wasm/bots/rusher -> dist/rusher.wasm"
