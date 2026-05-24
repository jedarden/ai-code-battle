#!/bin/bash
# Verify combat_death event rates across different bot combinations
# Expected: 2-player ~65-80%, 6-player 100%

set -e

cd "$(dirname "$0")/.."

echo "=== Combat Density Verification ==="
echo "Running 20 matches per configuration..."
echo

# Function to run matches and calculate combat_death rate
run_matches() {
    local bots="$1"
    local count="$2"
    local description="$3"
    local with_deaths=0
    local total_deaths=0

    echo "Testing: $description"
    for i in $(seq 1 $count); do
        local output="/tmp/verify-$i.json"
        ./acb-local -bots $bots -max-turns 100 -output $output >/dev/null 2>&1
        local deaths=$(python3 -c "import json; r=json.load(open('$output')); print(len([e for t in r['turns'] for e in t.get('events', []) if e.get('type') == 'combat_death']))" 2>/dev/null || echo 0)
        if [ "$deaths" -gt 0 ]; then
            with_deaths=$((with_deaths + 1))
            total_deaths=$((total_deaths + deaths))
        fi
    done

    local rate=$((with_deaths * 100 / count))
    echo "  Matches with combat_deaths: $with_deaths/$count ($rate%)"
    echo "  Total combat_death events: $total_deaths"
    echo "  Average per match: $(python3 -c "print($total_deaths / $count)")"
    echo

    # Return 1 if rate is below threshold
    if [ "$description" = "2-player" ] && [ $rate -lt 65 ]; then
        return 1
    fi
    if [ "$description" = "6-player" ] && [ $rate -lt 100 ]; then
        return 1
    fi
    return 0
}

# Test 2-player matches
run_matches "random,random" 20 "2-player (random bots)"
run_matches "gatherer,rusher" 20 "2-player (aggressive bots)"

# Test 6-player matches
run_matches "random,gatherer,rusher,guardian,swarm,hunter" 20 "6-player (mixed bots)"

echo "=== Verification Complete ==="
