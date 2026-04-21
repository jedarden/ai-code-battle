package engine

import (
	"fmt"
	"math"
	"math/rand"
)

// WinProbEntry holds per-turn win probabilities for each player.
type WinProbEntry []float64

// CriticalMoment identifies a turn where win probability shifted significantly.
type CriticalMoment struct {
	Turn        int     `json:"turn"`
	Delta       float64 `json:"delta"`
	Player      int     `json:"player"`
	Description string  `json:"description"`
}

// ComputeWinProbability runs Monte Carlo rollouts from each snapshot to estimate
// per-turn win probability. For each turn T, it clones the state, runs numRollouts
// random-play rollouts to match end, and computes win_prob[T] = wins[i] / numRollouts.
func ComputeWinProbability(snapshots []*GameState, numRollouts int, rng *rand.Rand) ([]WinProbEntry, []CriticalMoment) {
	if len(snapshots) == 0 || numRollouts <= 0 {
		return nil, nil
	}

	numPlayers := len(snapshots[0].Players)
	winProbs := make([]WinProbEntry, len(snapshots))

	for t, snap := range snapshots {
		wins := make([]int, numPlayers)
		draws := 0

		for r := 0; r < numRollouts; r++ {
			clone := snap.Clone()
			clone.rng = rand.New(rand.NewSource(rng.Int63()))
			winner := runRandomRollout(clone)
			if winner >= 0 && winner < numPlayers {
				wins[winner]++
			} else {
				draws++
			}
		}

		entry := make(WinProbEntry, numPlayers)
		for i := 0; i < numPlayers; i++ {
			entry[i] = float64(wins[i]) / float64(numRollouts)
		}
		winProbs[t] = entry
	}

	criticalMoments := detectCriticalMoments(winProbs, snapshots)

	return winProbs, criticalMoments
}

// runRandomRollout plays random moves from the given state until the match ends,
// returning the winner player ID (-1 for draw).
func runRandomRollout(gs *GameState) int {
	directions := []Direction{DirNone, DirN, DirE, DirS, DirW}

	for gs.Turn < gs.Config.MaxTurns {
		gs.ClearTurnState()
		submitRandomMoves(gs, directions)
		result := gs.ExecuteTurn()
		if result != nil {
			return result.Winner
		}
	}

	// Max turns reached — determine winner by score
	winner := gs.findWinnerByScore()
	return winner
}

// submitRandomMoves assigns a random direction to each living bot.
func submitRandomMoves(gs *GameState, directions []Direction) {
	for _, b := range gs.Bots {
		if !b.Alive {
			continue
		}
		dir := directions[gs.rng.Intn(len(directions))]
		if dir != DirNone {
			dest := gs.Grid.Move(b.Position, dir)
			if gs.Grid.IsPassable(dest) {
				gs.SubmitMove(b.Position, dir)
			}
		}
	}
}

// detectCriticalMoments finds turns where win probability shifted by more than
// threshold for any player. It uses events from the game state snapshots to
// generate human-readable descriptions.
func detectCriticalMoments(winProbs []WinProbEntry, snapshots []*GameState) []CriticalMoment {
	const threshold = 0.15

	var moments []CriticalMoment

	for t := 1; t < len(winProbs); t++ {
		prev := winProbs[t-1]
		curr := winProbs[t]

		for player := 0; player < len(curr); player++ {
			delta := curr[player] - prev[player]
			if math.Abs(delta) >= threshold {
				desc := describeCriticalTurn(snapshots, t, player, delta)
				moments = append(moments, CriticalMoment{
					Turn:        t,
					Delta:       math.Round(delta*100) / 100,
					Player:      player,
					Description: desc,
				})
			}
		}
	}

	return moments
}

// describeCriticalTurn generates a template-based description of why a turn was critical.
func describeCriticalTurn(snapshots []*GameState, turn int, player int, delta float64) string {
	if turn >= len(snapshots) {
		return fmt.Sprintf("Player %d win probability %s to %.0f%%", player, direction(delta), math.Round(math.Abs(delta)*100))
	}

	snap := snapshots[turn]

	// Count events for this player
	var combatDeaths, captures, botDied int
	for _, ev := range snap.Events {
		switch ev.Type {
		case EventCombatDeath:
			if details, ok := ev.Details.(map[string]interface{}); ok {
				if owner, ok := details["owner"].(int); ok && owner == player {
					combatDeaths++
				}
			}
		case EventBotDied:
			if details, ok := ev.Details.(map[string]interface{}); ok {
				if owner, ok := details["owner"].(int); ok && owner == player {
					botDied++
				}
			}
		case EventCoreCaptured:
			if details, ok := ev.Details.(map[string]interface{}); ok {
				if newOwner, ok := details["new_owner"].(int); ok {
					if newOwner == player {
						captures++
					}
				}
			}
		}
	}

	switch {
	case combatDeaths > 0 && delta < 0:
		return fmt.Sprintf("Player %d loses %d unit(s) in combat, win probability %s to %.0f%%",
			player, combatDeaths, direction(delta), math.Round(math.Abs(delta)*100))
	case combatDeaths > 0 && delta > 0:
		return fmt.Sprintf("Player %d wins engagement eliminating %d enemy unit(s), win probability %s to %.0f%%",
			player, combatDeaths, direction(delta), math.Round(math.Abs(delta)*100))
	case captures > 0:
		return fmt.Sprintf("Player %d captures a core, win probability %s to %.0f%%",
			player, direction(delta), math.Round(math.Abs(delta)*100))
	case botDied > 0 && delta < 0:
		return fmt.Sprintf("Player %d loses %d unit(s), win probability %s to %.0f%%",
			player, botDied, direction(delta), math.Round(math.Abs(delta)*100))
	default:
		return fmt.Sprintf("Player %d win probability %s to %.0f%%",
			player, direction(delta), math.Round(math.Abs(delta)*100))
	}
}

func direction(delta float64) string {
	if delta > 0 {
		return "rises"
	}
	return "drops"
}
