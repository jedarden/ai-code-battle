#!/bin/bash
# Test siege bot in arena: 10 matches vs rusher+gatherer+guardian

BOTS="siege,rusher,gatherer"
WINS=0
TOTAL=10

echo "Testing siege bot: $TOTAL matches vs $BOTS"
echo "======================================="

for i in $(seq 1 $TOTAL); do
    echo -n "Match $i: "
    OUTPUT=$(./acb-local -bots $BOTS -seed $((12345 + i)) 2>&1)

    # Extract winner (look for "Winner: Player N" or similar)
    WINNER=$(echo "$OUTPUT" | grep -oP "Winner: Player \K\d+" || echo "")

    if [ -z "$WINNER" ]; then
        # Try alternative patterns
        WINNER=$(echo "$OUTPUT" | grep -oP "Player \K\d+(?= wins)" || echo "")
    fi

    if [ "$WINNER" = "0" ]; then
        echo "Player 0 (siege) WINS!"
        ((WINS++))
    elif [ -n "$WINNER" ]; then
        echo "Player $WINNER wins"
    else
        # Check output for match result
        if echo "$OUTPUT" | grep -q "siege"; then
            echo "Result unclear (check output)"
        else
            echo "No clear winner detected"
        fi
    fi
done

echo "======================================="
echo "Final Score: $WINS / $TOTAL wins"

if [ $WINS -ge 1 ]; then
    echo "✓ PASS: Siege bot won at least 1 match"
    exit 0
else
    echo "✗ FAIL: Siege bot failed to win any match"
    exit 1
fi
