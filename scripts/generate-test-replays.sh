#!/bin/bash
# Generate test replays that match web/public/data/matches/index.json
# Run this from the repo root after building acb-local

set -e

echo "=== Generating Test Replays ==="
echo ""

# Build acb-local if not already built
if [ ! -f "./acb-local" ]; then
    echo "Building acb-local..."
    go build -o acb-local ./cmd/acb-local
fi

# Replays directory
REPLAY_DIR="test-replays"
mkdir -p "$REPLAY_DIR"

# Helper function to generate replay with specific match_id
generate_replay() {
    local match_id=$1
    local bots=$2
    local seed=$3
    local output="$REPLAY_DIR/${match_id}.json"

    echo "Generating $match_id..."
    ./acb-local -bots "$bots" -seed "$seed" -output "$output" -rows 89 -cols 89

    # Update match_id in the replay JSON
    temp_file=$(mktemp)
    jq --arg id "$match_id" '.match_id = $id' "$output" > "$temp_file"
    mv "$temp_file" "$output"

    # Gzip it
    gzip -f "$output"
    echo "  Created: ${output}.gz"
}

# Generate replays matching index.json entries
generate_replay "m_test_upset_v1" "random,guardian" 1
generate_replay "m_test_6p_v1" "swarm,hunter,gatherer,rusher,guardian,random" 2
generate_replay "m_test_close_v1" "hunter,gatherer" 3
generate_replay "m_test_domination_v1" "swarm,rusher" 4
generate_replay "m_test_4p_v1" "hunter,gatherer,guardian,random" 5
generate_replay "m_test_comeback_v1" "rusher,random" 6
generate_replay "m_test_marathon_v1" "guardian,random" 7
generate_replay "m_test_quick_v1" "rusher,random" 8

echo ""
echo "=== Done! Replays generated in $REPLAY_DIR/ ==="
echo ""
echo "To upload to R2, run: ./scripts/upload-test-replays.sh"
