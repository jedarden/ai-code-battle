package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"

	"github.com/aicodebattle/acb/engine"
)

// SimpleRandomBot is a bot that makes random moves.
type SimpleRandomBot struct {
	rng *rand.Rand
}

func (b *SimpleRandomBot) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	moves := make([]engine.Move, 0, len(state.Bots))
	for _, bot := range state.Bots {
		if bot.Owner != state.You.ID {
			continue
		}
		// 20% chance to move in a random direction
		if b.rng.Float64() < 0.2 {
			directions := []engine.Direction{
				engine.DirN, engine.DirE, engine.DirS, engine.DirW,
			}
			dir := directions[b.rng.Intn(len(directions))]
			moves = append(moves, engine.Move{
				Position:  bot.Position,
				Direction: dir,
			})
		}
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
	fmt.Printf("  AttackRadius2: %d (~%.1f tiles)\n", config.AttackRadius2, sqrt(float64(config.AttackRadius2)))
	fmt.Printf("  ZoneStartTurn: %d, ZoneShrinkInterval: %d, ZoneShrinkStep: %d, ZoneMinRadius: %d\n",
		config.ZoneStartTurn, config.ZoneShrinkInterval, config.ZoneShrinkStep, config.ZoneMinRadius)
	fmt.Printf("  MaxTurns: %d\n\n", config.MaxTurns)

	// Create match runner with verbose logging
	rng := rand.New(rand.NewSource(42))
	logger := log.New(os.Stdout, "[MATCH] ", log.LstdFlags)
	mr := engine.NewMatchRunner(config, engine.WithVerbose(true), engine.WithLogger(logger), engine.WithRNG(rng))

	// Add bots
	bot1 := &SimpleRandomBot{rng: rand.New(rand.NewSource(1))}
	bot2 := &SimpleRandomBot{rng: rand.New(rand.NewSource(2))}

	mr.AddBot(bot1, "Bot1")
	mr.AddBot(bot2, "Bot2")

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
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == "combat_death" {
				combatDeathCount++
			}
		}
	}
	fmt.Printf("\nTotal combat_death events: %d\n", combatDeathCount)

	// Show some sample combat events
	fmt.Printf("\n=== SAMPLE COMBAT EVENTS ===\n")
	count := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == "combat_death" {
				details := event.Details.(map[string]interface{})
				fmt.Printf("Turn %d: Bot %d (owner %d) killed at (%d,%d), killers: %v\n",
					turn.Turn, details["bot_id"], details["owner"],
					details["position"].(map[string]interface{})["row"],
					details["position"].(map[string]interface{})["col"],
					details["killers"])
				count++
				if count >= 5 {
					break
				}
			}
		}
		if count >= 5 {
			break
		}
	}

	// Save replay for inspection
	replayJSON, _ := json.MarshalIndent(replay, "", "  ")
	os.WriteFile("/tmp/focus_fire_replay.json", replayJSON, 0644)
	fmt.Printf("\nReplay saved to /tmp/focus_fire_replay.json\n")

	// Check for any bots getting close enough for combat
	fmt.Printf("\n=== PROXIMITY ANALYSIS ===\n")
	for _, turn := range replay.Turns {
		if turn.Turn < 5 || turn.Turn > 20 {
			continue
		}

		// Count pairs of enemy bots within attack radius
		closePairs := 0
		totalEnemyPairs := 0

		for i, botA := range turn.Bots {
			if !botA.Alive {
				continue
			}
			for j, botB := range turn.Bots {
				if j <= i || !botB.Alive {
					continue
				}
				if botA.Owner != botB.Owner {
					totalEnemyPairs++
					// Calculate distance
					d2 := distSq(botA.Position, botB.Position, config.Rows, config.Cols)
					if d2 <= config.AttackRadius2 {
						closePairs++
					}
				}
			}
		}

		if totalEnemyPairs > 0 {
			fmt.Printf("Turn %d: %d/%d enemy pairs within attack range (%.1f%%)\n",
				turn.Turn, closePairs, totalEnemyPairs,
				100.0*float64(closePairs)/float64(totalEnemyPairs))
		}

		if turn.Turn == 20 {
			break
		}
	}
}

func sqrt(x float64) float64 {
	// Simple approximation
	if x == 0 {
		return 0
	}
	guess := x / 2
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

func distSq(a, b engine.Position, rows, cols int) int {
	// Toroidal distance squared
	dr := a.Row - b.Row
	dc := a.Col - b.Col

	// Handle wraparound
	if dr < 0 {
		dr = -dr
	}
	if dc < 0 {
		dc = -dc
	}

	// Toroidal wrap
	if dr > rows/2 {
		dr = rows - dr
	}
	if dc > cols/2 {
		dc = cols - dc
	}

	return dr*dr + dc*dc
}
