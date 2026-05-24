#!/bin/bash
# Analyze combat_death rate from replay files
# Usage: ./scripts/analyze-combat-deaths.sh <replay-files-directory>

set -e

REPLAY_DIR="${1:-/tmp/replays}"

if [ ! -d "$REPLAY_DIR" ]; then
  echo "Error: Replay directory not found: $REPLAY_DIR"
  echo "Usage: $0 <replay-files-directory>"
  exit 1
fi

echo "=== Combat Death Rate Analysis ==="
echo "Analyzing replays in: $REPLAY_DIR"
echo

# Count replays with combat_deaths by player count
analyze_replays() {
  local player_count=$1
  local total=0
  local with_deaths=0
  local total_deaths=0

  for replay in "$REPLAY_DIR"/*.json "$REPLAY_DIR"/*.json.gz 2>/dev/null; do
    [ -f "$replay" ] || continue

    # Check player count
    local pc=$(python3 -c "
import json, sys
try:
    if '$replay'.endswith('.gz'):
        import gzip
        f = gzip.open('$replay', 'rt')
    else:
        f = open('$replay')
    r = json.load(f)
    print(len(r.get('players', [])))
except Exception as e:
    print(0)
" 2>/dev/null)

    if [ "$pc" != "$player_count" ]; then
      continue
    fi

    total=$((total + 1))

    # Count combat_deaths
    local deaths=$(python3 -c "
import json, sys
try:
    if '$replay'.endswith('.gz'):
        import gzip
        f = gzip.open('$replay', 'rt')
    else:
        f = open('$replay')
    r = json.load(f)
    deaths = 0
    for t in r.get('turns', []):
        for e in t.get('events', []):
            if e.get('type') == 'combat_death':
                deaths += 1
    print(deaths)
except Exception as e:
    print(0)
" 2>/dev/null)

    if [ "$deaths" -gt 0 ]; then
      with_deaths=$((with_deaths + 1))
      total_deaths=$((total_deaths + deaths))
    fi
  done

  if [ $total -eq 0 ]; then
    echo "No $player_count-player replays found"
  else
    local rate=$((with_deaths * 100 / total))
    local avg=$(python3 -c "print($total_deaths / $total)")
    echo "$player_count-player: $with_deaths/$total ($rate%) with combat_deaths, avg $avg per match"
  fi
}

# Analyze by player count
for pc in 2 3 4 6; do
  analyze_replays $pc
done

echo
echo "To monitor production matches:"
echo "1. Download replays from R2: aws s3 sync s3://acb-replays /tmp/replays"
echo "2. Run: ./scripts/analyze-combat-deaths.sh /tmp/replays"
