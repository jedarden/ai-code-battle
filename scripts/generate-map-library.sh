#!/usr/bin/env bash
# generate-map-library.sh - Regenerate the map library
# Usage: scripts/generate-map-library.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MAPGEN_BIN="${PROJECT_ROOT}/.cache/bin/acb-mapgen"
MAPS_DIR="${PROJECT_ROOT}/maps"

# Build acb-mapgen if needed
if [ ! -f "$MAPGEN_BIN" ]; then
    echo "Building acb-mapgen..."
    mkdir -p "$(dirname "$MAPGEN_BIN")"
    go build -o "$MAPGEN_BIN" "${PROJECT_ROOT}/cmd/acb-mapgen"
fi

# Generate maps for each player count
for players in 2 3 4 6; do
    player_dir="${MAPS_DIR}/${players}player"
    mkdir -p "$player_dir"

    echo "Generating 50 maps for $players players..."
    for i in $(seq 1 50); do
        seed=$((1000 + players * 100 + i))
        output="${player_dir}/map_${i}.json"
        "$MAPGEN_BIN" -players "$players" -seed "$seed" -output "$output"
    done

    echo "Generated $(ls "$player_dir" | wc -l) maps for $players players"
done

echo "Map library generation complete!"
echo "Total maps: $(find "$MAPS_DIR" -name "*.json" | wc -l)"
