#!/bin/bash
# analyze-replay.sh - Check a replay file for combat_death and zone_death events

REPLAY_FILE="$1"

if [[ ! -f "$REPLAY_FILE" ]]; then
    echo "Usage: $0 <replay-file>"
    exit 1
fi

# Detect if gzipped
if [[ "$REPLAY_FILE" == *.gz ]]; then
    CAT="zcat"
else
    CAT="cat"
fi

echo "=== Replay: $REPLAY_FILE ==="

# Get basic info
MATCH_ID=$($CAT "$REPLAY_FILE" | jq -r '.match_id // "unknown"')
NUM_PLAYERS=$($CAT "$REPLAY_FILE" | jq '.players | length')
TURNS=$($CAT "$REPLAY_FILE" | jq '.turns | length')

echo "Match ID: $MATCH_ID"
echo "Players: $NUM_PLAYERS"
echo "Turns: $TURNS"

# Count combat_death events
COMBAT_DEATHS=$($CAT "$REPLAY_FILE" | jq '[.turns[].events[]? | select(.type == "combat_death")] | length')

# Count zone_death events
ZONE_DEATHS=$($CAT "$REPLAY_FILE" | jq '[.turns[].events[]? | select(.type == "zone_death")] | length')

# Count bot_died events with reason="zone" (current implementation)
BOT_DIED_ZONE=$($CAT "$REPLAY_FILE" | jq '[.turns[].events[]? | select(.type == "bot_died" and .details.reason == "zone")] | length')

echo "combat_death events: $COMBAT_DEATHS"
echo "zone_death events: $ZONE_DEATHS"
echo "bot_died (reason=zone): $BOT_DIED_ZONE"

# Calculate deaths per turn
if [[ "$TURNS" -gt 0 ]]; then
    DEATHS_PER_TURN=$(awk "BEGIN {printf \"%.3f\", $COMBAT_DEATHS / $TURNS}")
    echo "Deaths per turn: $DEATHS_PER_TURN"
fi

echo ""
