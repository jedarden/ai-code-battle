#! /run/current-system/sw/bin/bash
# Regenerate combat-density test replays with zone_death event format
# Run after engine changes that affect event emission (e.g., commit f0a0673)

set -e

cd "$(dirname "$0")/.."

echo "=== Regenerating Combat-Density Test Replays ==="
echo "This will regenerate 30 replays (10 each for 2p, 4p, 6p)"
echo ""

# Replays directory
REPLAY_DIR="test-replays/combat-density"
mkdir -p "$REPLAY_DIR"

# Bot combinations for variety
BOTS_2P="gatherer,rusher hunter,swarm guardian,hunter random,rusher swarm,gatherer"
BOTS_4P="gatherer,rusher,guardian,swarm hunter,swarm,gatherer,random random,guardian,hunter,rusher swarm,hunter,gatherer,guardian"
BOTS_6P="swarm,hunter,gatherer,rusher,guardian,random hunter,gatherer,rusher,guardian,random,swarm random,swarm,hunter,gatherer,rusher,guardian"

# Seed base for reproducibility
SEED_BASE=1000

# Function to generate replay
generate_replay() {
    local prefix=$1
    local index=$2
    local bots=$3
    local seed=$4

    local output="$REPLAY_DIR/${prefix}_${index}.json"

    echo "Generating ${prefix}_${index}.json..."
    ./acb-local -bots "$bots" -seed "$seed" -output "$output" -cores 2 >/dev/null 2>&1
}

# Generate 2-player replays
echo "Generating 2-player replays..."
for i in $(seq 1 10); do
    bots_array=($BOTS_2P)
    bots=${bots_array[$(( (i-1) % ${#bots_array[@]} ))]}
    seed=$((SEED_BASE + i))
    generate_replay "2p" "$i" "$bots" "$seed"
done

# Generate 4-player replays
echo "Generating 4-player replays..."
for i in $(seq 1 10); do
    bots_array=($BOTS_4P)
    bots=${bots_array[$(( (i-1) % ${#bots_array[@]} ))]}
    seed=$((SEED_BASE + 100 + i))
    generate_replay "4p" "$i" "$bots" "$seed"
done

# Generate 6-player replays
echo "Generating 6-player replays..."
for i in $(seq 1 10); do
    bots_array=($BOTS_6P)
    bots=${bots_array[$(( (i-1) % ${#bots_array[@]} ))]}
    seed=$((SEED_BASE + 200 + i))
    generate_replay "6p" "$i" "$bots" "$seed"
done

echo ""
echo "=== Done! Regenerated 30 replays in $REPLAY_DIR/ ==="
echo ""
echo "Verifying zone_death events..."
./scripts/verify-combat-density.sh
