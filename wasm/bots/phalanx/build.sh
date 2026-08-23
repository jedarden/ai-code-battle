#!/bin/sh
# Build phalanx.wasm from Rust source
# Uses low-level WASM interface compatible with the sandbox loader
set -e
cd "$(dirname "$0")"
mkdir -p ../../dist
# Pin the target dir: a global ~/.cargo/config.toml target-dir would otherwise
# send the artifact elsewhere and the cp below would fail.
CARGO_TARGET_DIR=./target cargo build --target wasm32-unknown-unknown --release
cp target/wasm32-unknown-unknown/release/phalanx_wasm.wasm ../../dist/phalanx.wasm
echo "Built wasm/bots/phalanx -> dist/phalanx.wasm"
