package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/aicodebattle/acb/engine"
)

// StationaryBot does not move, just stays in place.
type StationaryBot struct{}

func (b *StationaryBot) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	// No moves = stay in place
	return []engine.Move{}, nil
}

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
	// Test different scenarios
	scenarios := []struct {
		name       string
		bot1Type   string
		bot2Type   string
		config     engine.Config
	}{
		{
			name:     "Stationary vs Stationary (2-player, zone on)",
			bot1Type: "stationary",
			bot2Type: "stationary",
			config:   engine.ConfigForPlayers(2, 2),
		},
		{
			name:     "Aggressive vs Aggressive (2-player, zone on)",
			bot1Type: "aggressive",
			bot2Type: "aggressive",
			config:   engine.ConfigForPlayers(2, 2),
		},
		{
			name:     "Aggressive vs Stationary (2-player, zone on)",
			bot1Type: "aggressive",
			bot2Type: "stationary",
			config:   engine.ConfigForPlayers(2, 2),
		},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n=== SCENARIO: %s ===\n", scenario.name)

		config := scenario.config
		config.ZoneEnabled = true
		config.KillScore = 1

		fmt.Printf("Map: %dx%d, AttackRadius2: %d (~%.1f tiles)\n",
			config.Rows, config.Cols, config.AttackRadius2,
			math.Sqrt(float64(config.AttackRadius2)))

		rng := rand.New(rand.NewSource(42))
		mr := engine.NewMatchRunner(config,
			engine.WithVerbose(false),
			engine.WithRNG(rng))

		// Create bots
		var bot1, bot2 engine.BotInterface
		var rng1, rng2 *rand.Rand

		switch scenario.bot1Type {
		case "stationary":
			bot1 = &StationaryBot{}
		case "aggressive":
			rng1 = rand.New(rand.NewSource(1))
			bot1 = &AggressiveBot{rng: rng1}
		}

		switch scenario.bot2Type {
		case "stationary":
			bot2 = &StationaryBot{}
		case "aggressive":
			rng2 = rand.New(rand.NewSource(2))
			bot2 = &AggressiveBot{rng: rng2}
		}

		mr.AddBot(bot1, "Bot1")
		mr.AddBot(bot2, "Bot2")

		// Run match
		result, replay, err := mr.Run()
		if err != nil {
			log.Printf("Match failed: %v", err)
			continue
		}

		// Count events
		combatDeaths := 0
		selfCollisionDeaths := 0
		zoneDeaths := 0
		coreCaptures := 0

		for _, turn := range replay.Turns {
			for _, event := range turn.Events {
				switch event.Type {
				case "combat_death":
					combatDeaths++
				case "bot_died":
					if details, ok := event.Details.(map[string]interface{}); ok {
						if reason, ok := details["reason"].(string); ok {
							switch reason {
							case "self_collision":
								selfCollisionDeaths++
							}
						}
					}
				case "zone_death":
					zoneDeaths++
				case "core_captured":
					coreCaptures++
				}
			}
		}

		fmt.Printf("Result: Player %d won (%s) in %d turns\n", result.Winner, result.Reason, result.Turns)
		fmt.Printf("Scores: %v\n", result.Scores)
		fmt.Printf("CombatDeaths (tracked): %v\n", result.CombatDeaths)
		fmt.Printf("Events: combat_death=%d, self_collision=%d, zone_death=%d, core_capture=%d\n",
			combatDeaths, selfCollisionDeaths, zoneDeaths, coreCaptures)

		// Analyze turns where bots were in attack range
		fmt.Printf("\nProximity analysis:\n")
		for _, turn := range replay.Turns {
			if turn.Turn < 5 || turn.Turn > 25 || turn.Turn%5 != 0 {
				continue
			}

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
						d2 := distSq(botA, botB, config.Rows, config.Cols)
						if d2 <= config.AttackRadius2 {
							closePairs++
						}
					}
				}
			}

			if totalEnemyPairs > 0 {
				pct := 100.0 * float64(closePairs) / float64(totalEnemyPairs)
				fmt.Printf("  Turn %d: %d/%d enemy pairs in range (%.1f%%), living bots: %d\n",
					turn.Turn, closePairs, totalEnemyPairs, pct, countLiving(turn.Bots))
			}
		}
	}
}

func countLiving(bots []engine.ReplayBot) int {
	count := 0
	for _, b := range bots {
		if b.Alive {
			count++
		}
	}
	return count
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
