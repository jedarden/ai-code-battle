//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"

	"github.com/aicodebattle/acb/engine"
)

// AggressiveBot moves toward nearest enemy bot.
type AggressiveBot struct {
	rng *rand.Rand
}

func (b *AggressiveBot) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	moves := make([]engine.Move, 0, len(state.Bots))

	for _, bot := range state.Bots {
		if bot.Owner != state.You.ID {
			continue
		}

		// Find nearest enemy
		nearestDist := 999999.0
		nearestEnemy := bot.Position

		for _, other := range state.Bots {
			if other.Owner == state.You.ID {
				continue
			}

			d := math.Sqrt(float64((bot.Position.Row-other.Position.Row)*(bot.Position.Row-other.Position.Row) +
				(bot.Position.Col-other.Position.Col)*(bot.Position.Col-other.Position.Col)))
			if d < nearestDist {
				nearestDist = d
				nearestEnemy = other.Position
			}
		}

		// Move toward nearest enemy
		var dir engine.Direction
		dr := nearestEnemy.Row - bot.Position.Row
		dc := nearestEnemy.Col - bot.Position.Col

		// Normalize to single direction
		if math.Abs(float64(dr)) > math.Abs(float64(dc)) {
			if dr > 0 {
				dir = engine.DirS
			} else {
				dir = engine.DirN
			}
		} else {
			if dc > 0 {
				dir = engine.DirE
			} else {
				dir = engine.DirW
			}
		}

		moves = append(moves, engine.Move{
			Position:  bot.Position,
			Direction: dir,
		})
	}

	return moves, nil
}

func main() {
	// Create config for 2 players with zone enabled
	config := engine.ConfigForPlayers(2, 2)
	config.ZoneEnabled = true
	config.KillScore = 1

	fmt.Printf("Config:\n")
	fmt.Printf("  Rows: %d, Cols: %d\n", config.Rows, config.Cols)
	fmt.Printf("  AttackRadius2: %d (~%.1f tiles)\n", config.AttackRadius2, math.Sqrt(float64(config.AttackRadius2)))
	fmt.Printf("  ZoneStartTurn: %d, ZoneShrinkInterval: %d, ZoneShrinkStep: %d, ZoneMinRadius: %d\n",
		config.ZoneStartTurn, config.ZoneShrinkInterval, config.ZoneShrinkStep, config.ZoneMinRadius)
	fmt.Printf("  MaxTurns: %d\n\n", config.MaxTurns)

	// Create match runner with verbose logging
	rng := rand.New(rand.NewSource(42))
	logger := log.New(os.Stdout, "[MATCH] ", log.LstdFlags)
	mr := engine.NewMatchRunner(config, engine.WithVerbose(true), engine.WithLogger(logger), engine.WithRNG(rng))

	// Add bots
	bot1 := &AggressiveBot{rng: rand.New(rand.NewSource(1))}
	bot2 := &AggressiveBot{rng: rand.New(rand.NewSource(2))}

	mr.AddBot(bot1, "Aggressive1")
	mr.AddBot(bot2, "Aggressive2")

	// Run match
	result, replay, err := mr.Run()
	if err != nil {
		log.Fatalf("Match failed: %v", err)
	}

	// Analyze combat deaths
	fmt.Printf("\n=== MATCH RESULTS ===\n")
	fmt.Printf("Winner: Player %d (%s)\n", result.Winner, result.Reason)
	fmt.Printf("Turns: %d\n", result.Turns)
	fmt.Printf("Scores: %v\n", result.Scores)
	fmt.Printf("CombatDeaths: %v\n", result.CombatDeaths)
	fmt.Printf("BotsAlive: %v\n", result.BotsAlive)

	// Count combat events
	combatDeathCount := 0
	selfCollisionCount := 0
	zoneDeathCount := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == "combat_death" {
				combatDeathCount++
			}
			if event.Type == "bot_died" {
				if details, ok := event.Details.(map[string]interface{}); ok {
					if reason, ok := details["reason"].(string); ok {
						if reason == "self_collision" {
							selfCollisionCount++
						}
					}
				}
			}
			if event.Type == "zone_death" {
				zoneDeathCount++
			}
		}
	}
	fmt.Printf("\nTotal events: combat_death=%d, self_collision=%d, zone_death=%d\n",
		combatDeathCount, selfCollisionCount, zoneDeathCount)

	// Save replay for inspection
	replayJSON, _ := json.MarshalIndent(replay, "", "  ")
	os.WriteFile("test-replay-aggressive.json", replayJSON, 0644)
	fmt.Printf("\nReplay saved to test-replay-aggressive.json\n")
}
