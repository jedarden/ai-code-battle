//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"

	"github.com/aicodebattle/acb/engine"
)

// StationaryBot does not move.
type StationaryBot struct{}

func (b *StationaryBot) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	return []engine.Move{}, nil
}

func main() {
	config := engine.ConfigForPlayers(2, 2)
	config.ZoneEnabled = true
	config.KillScore = 1

	fmt.Printf("=== PROXIMITY ANALYSIS TEST ===\n")
	fmt.Printf("Map: %dx%d, AttackRadius2: %d (~%.1f tiles)\n",
		config.Rows, config.Cols, config.AttackRadius2, sqrt(float64(config.AttackRadius2)))
	fmt.Printf("ZoneStartTurn: %d, ZoneMinRadius: %d, ZoneShrinkStep: %d\n\n",
		config.ZoneStartTurn, config.ZoneMinRadius, config.ZoneShrinkStep)

	rng := rand.New(rand.NewSource(42))
	mr := engine.NewMatchRunner(config,
		engine.WithVerbose(true),
		engine.WithLogger(log.New(os.Stdout, "[MATCH] ", log.LstdFlags)),
		engine.WithRNG(rng))

	bot1 := &StationaryBot{}
	bot2 := &StationaryBot{}

	mr.AddBot(bot1, "Bot1")
	mr.AddBot(bot2, "Bot2")

	result, replay, err := mr.Run()
	if err != nil {
		log.Fatalf("Match failed: %v", err)
	}

	fmt.Printf("\n=== FINAL RESULTS ===\n")
	fmt.Printf("Winner: Player %d (%s) in %d turns\n", result.Winner, result.Reason, result.Turns)
	fmt.Printf("CombatDeaths: %v\n", result.CombatDeaths)

	// Save replay for detailed analysis
	replayJSON, _ := json.MarshalIndent(replay, "", "  ")
	os.WriteFile("/tmp/proximity_replay.json", replayJSON, 0644)
	fmt.Printf("\nReplay saved to /tmp/proximity_replay.json\n")

	// Analyze spawn positions and distances
	fmt.Printf("\n=== SPAWN ANALYSIS ===\n")
	if len(replay.Turns) > 0 {
		turn0 := replay.Turns[0]
		var bots0 []engine.ReplayBot
		if b, ok := turn0.Bots.([]interface{}); ok {
			// Convert from JSON interface
			for _, botInterface := range b {
				if botMap, ok := botInterface.(map[string]interface{}); ok {
					bot := engine.ReplayBot{}
					if id, ok := botMap["id"].(float64); ok {
						bot.ID = int(id)
					}
					if owner, ok := botMap["owner"].(float64); ok {
						bot.Owner = int(owner)
					}
					if alive, ok := botMap["alive"].(bool); ok {
						bot.Alive = alive
					}
					if pos, ok := botMap["position"].(map[string]interface{}); ok {
						bot.Position.Row = int(pos["row"].(float64))
						bot.Position.Col = int(pos["col"].(float64))
					}
					bots0 = append(bots0, bot)
				}
			}
		}

		for i, bot := range bots0 {
			fmt.Printf("Bot %d (Player %d): spawned at (%d,%d)\n",
				bot.ID, bot.Owner, bot.Position.Row, bot.Position.Col)
		}

		// Calculate distances between enemy bots
		for i, botA := range bots0 {
			for j, botB := range bots0 {
				if j <= i {
					continue
				}
				if botA.Owner != botB.Owner {
					d2 := distSq(botA, botB, config.Rows, config.Cols)
					d := sqrt(float64(d2))
					inRange := "NO"
					if d2 <= config.AttackRadius2 {
						inRange = "YES"
					}
					fmt.Printf("Distance Bot %d->Bot %d: %.1f tiles (in combat range: %s)\n",
						botA.ID, botB.ID, d, inRange)
				}
			}
		}
	}

	// Analyze proximity over time
	fmt.Printf("\n=== PROXIMITY OVER TIME ===\n")
	for i, turn := range replay.Turns {
		if i == 0 {
			continue // Skip spawn turn
		}
		if i > 30 {
			break
		}

		// Count living bots per player
		living := make(map[int]int)
		for _, bot := range turn.Bots {
			if bot.Alive {
				living[bot.Owner]++
			}
		}

		// Find closest enemy pair
		minDist := 999999.0
		var closestA, closestB *engine.ReplayBot

		for i, botA := range turn.Bots {
			if !botA.Alive {
				continue
			}
			for j, botB := range turn.Bots {
				if j <= i || !botB.Alive {
					continue
				}
				if botA.Owner != botB.Owner {
					d2 := distSq(botA, botB, config.Rows, config.Cols)
					d := sqrt(float64(d2))
					if d < minDist {
						minDist = d
						closestA = &turn.Bots[i]
						closestB = &turn.Bots[j]
					}
				}
			}
		}

		if minDist < 999999.0 {
			inRange := "NO"
			if minDist <= sqrt(float64(config.AttackRadius2)) {
				inRange = "YES"
			}
			fmt.Printf("Turn %d: Player0=%d bots, Player1=%d bots, Closest enemies: %.1f tiles apart (Bot %d vs Bot %d, in range: %s)\n",
				turn.Turn, living[0], living[1], minDist, closestA.ID, closestB.ID, inRange)
		}

		// Check for zone shrink
		if i > 0 {
			prevZone := replay.Turns[i-1].ZoneBounds
			currZone := turn.ZoneBounds
			if prevZone != nil && currZone != nil {
				if prevZone.Radius != currZone.Radius {
					fmt.Printf("  -> Zone shrank from radius %d to %d\n", prevZone.Radius, currZone.Radius)
				}
			}
		}
	}

	// Count event types
	fmt.Printf("\n=== EVENT SUMMARY ===\n")
	eventCounts := make(map[string]int)
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			eventCounts[event.Type]++
		}
	}
	for eventType, count := range eventCounts {
		fmt.Printf("%s: %d\n", eventType, count)
	}
}

func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	guess := x / 2
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

func distSq(a, b engine.ReplayBot, rows, cols int) int {
	aRow := a.Position.Row
	aCol := a.Position.Col
	bRow := b.Position.Row
	bCol := b.Position.Col

	dr := aRow - bRow
	dc := aCol - bCol

	if dr < 0 {
		dr = -dr
	}
	if dc < 0 {
		dc = -dc
	}

	if dr > rows/2 {
		dr = rows - dr
	}
	if dc > cols/2 {
		dc = cols - dc
	}

	return dr*dr + dc*dc
}
