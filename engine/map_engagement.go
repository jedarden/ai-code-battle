package engine

import "math"

// MapEngagementScore represents the engagement metrics for a map from a single match.
type MapEngagementScore struct {
	WinProbCrossings     float64 // Number of times win prob crossed 50%
	CriticalMoments      int     // Count of critical moments (bot deaths/captures)
	CombatDeaths         int     // Count of focus-fire combat deaths (EventCombatDeath)
	ResourceContestTurns int     // Turns where energy was contested (multiple players adjacent)
	SurvivalTurns        int     // Turns where all players had at least one bot alive
	Engagement           float64 // Combined engagement score
}

// CalculateMapEngagement computes the engagement score for a map based on replay data.
// The engagement formula (from plan §14.6, extended for combat density) is:
// score = win_prob_crossings * 3.0 + combat_deaths * 3.0 + critical_moments * 2.0 +
//         resource_contest_turns * 1.5 + survival_turns * 0.5
func CalculateMapEngagement(replay *Replay) MapEngagementScore {
	if replay == nil || len(replay.Turns) == 0 {
		return MapEngagementScore{}
	}

	// Count win probability crossings (times the leader changed)
	winProbCrossings := countWinProbCrossings(replay.WinProb)

	// Count combat deaths (focus-fire kills)
	combatDeaths := countCombatDeaths(replay)

	// Count critical moments (bot deaths/captures with significant win prob shifts)
	criticalMoments := len(replay.CriticalMoments)

	// Count resource contest turns (turns where energy was contested)
	resourceContestTurns := countResourceContestTurns(replay)

	// Count survival turns (turns where all players had at least one bot alive)
	survivalTurns := countSurvivalTurns(replay)

	// Calculate combined engagement score per plan §14.6
	// Combat deaths are weighted heavily (3.0) to bias map evolution toward combat-dense maps
	engagement := float64(winProbCrossings)*3.0 +
		float64(combatDeaths)*3.0 +
		float64(criticalMoments)*2.0 +
		float64(resourceContestTurns)*1.5 +
		float64(survivalTurns)*0.5

	return MapEngagementScore{
		WinProbCrossings:     winProbCrossings,
		CriticalMoments:      criticalMoments,
		CombatDeaths:         combatDeaths,
		ResourceContestTurns: resourceContestTurns,
		SurvivalTurns:        survivalTurns,
		Engagement:           engagement,
	}
}

// countWinProbCrossings counts how many times the win probability crossed 50% for any player.
// This indicates lead changes and momentum shifts.
func countWinProbCrossings(winProbs []WinProbEntry) float64 {
	if len(winProbs) < 2 {
		return 0
	}

	crossings := 0

	// Track which player was leading (had highest win prob) at each turn
	for i := 1; i < len(winProbs); i++ {
		prevLeader := findLeader(winProbs[i-1])
		currLeader := findLeader(winProbs[i])

		if prevLeader != currLeader {
			crossings++
		}
	}

	return float64(crossings)
}

// findLeader returns the index of the player with the highest win probability.
// Returns -1 if there's a tie or no clear leader.
func findLeader(entry WinProbEntry) int {
	if len(entry) == 0 {
		return -1
	}

	maxProb := entry[0]
	leaderIdx := 0

	// Check if there's a clear leader (no ties)
	for i := 1; i < len(entry); i++ {
		if entry[i] > maxProb {
			maxProb = entry[i]
			leaderIdx = i
		}
	}

	// Verify the leader is significantly ahead (not a tie)
	isTie := false
	for i := 0; i < len(entry); i++ {
		if i != leaderIdx && math.Abs(entry[i]-maxProb) < 0.01 {
			isTie = true
			break
		}
	}

	if isTie {
		return -1
	}

	return leaderIdx
}

// calculateMapCoverage computes the percentage of map tiles that were visited by any bot.
func calculateMapCoverage(replay *Replay) float64 {
	if replay == nil || len(replay.Turns) == 0 {
		return 0
	}

	totalTiles := replay.Config.Rows * replay.Config.Cols
	if totalTiles == 0 {
		return 0
	}

	// Count unique tiles visited across all turns
	visited := make(map[string]struct{})
	for _, turn := range replay.Turns {
		for _, bot := range turn.Bots {
			if bot.Alive {
				key := string(rune(bot.Position.Row)) + "," + string(rune(bot.Position.Col))
				visited[key] = struct{}{}
			}
		}
	}

	// Subtract wall tiles from total (they're not visitable)
	wallTiles := len(replay.Map.Walls)
	visitbleTiles := totalTiles - wallTiles
	if visitbleTiles <= 0 {
		return 0
	}

	return float64(len(visited)) / float64(visitbleTiles)
}

// calculateCloseness computes how close the final score was.
// Returns 1.0 for a draw/tie, decreasing to 0.0 for a blowout.
func calculateCloseness(replay *Replay) float64 {
	if replay == nil || replay.Result == nil || len(replay.Result.Scores) == 0 {
		return 0
	}

	// Find the max and min scores
	maxScore := replay.Result.Scores[0]
	minScore := replay.Result.Scores[0]
	for _, score := range replay.Result.Scores {
		if score > maxScore {
			maxScore = score
		}
		if score < minScore {
			minScore = score
		}
	}

	scoreDiff := maxScore - minScore
	if scoreDiff == 0 {
		return 1.0 // Perfect tie
	}

	// Normalize: closeness = 1 - (score_diff / max_possible_score)
	// Assume max possible score is roughly 3x the number of turns (3 points per capture)
	maxPossibleScore := float64(replay.Config.MaxTurns) * 3.0
	if maxPossibleScore <= 0 {
		return 1.0
	}

	normalizedDiff := float64(scoreDiff) / maxPossibleScore
	if normalizedDiff > 1.0 {
		normalizedDiff = 1.0
	}

	return 1.0 - normalizedDiff
}

// countResourceContestTurns counts turns where energy was contested by multiple players.
// A turn is contested if at least one energy tile has bots from two or more different players
// adjacent to it (meaning they could both collect it, but contested energy is destroyed).
func countResourceContestTurns(replay *Replay) int {
	if replay == nil || len(replay.Turns) == 0 {
		return 0
	}

	contestedTurns := 0

	for _, turn := range replay.Turns {
		if isEnergyContested(turn) {
			contestedTurns++
		}
	}

	return contestedTurns
}

// isEnergyContested checks if any energy in this turn is contested by multiple players.
// Energy is contested when two or more players have bots adjacent to the same energy node.
func isEnergyContested(turn ReplayTurn) bool {
	if len(turn.Energy) == 0 {
		return false
	}

	// For each energy tile, check which players have adjacent bots
	for _, energyPos := range turn.Energy {
		playersAdjacent := make(map[int]struct{})

		for _, bot := range turn.Bots {
			if !bot.Alive {
				continue
			}

			// Check if bot is adjacent to this energy (including being on it)
			dist := toroidalDistance(bot.Position, energyPos, int(turn.Bots[0].Position.Row), int(turn.Bots[0].Position.Col))
			if dist <= 1.5 { // Adjacent or on the tile (using sqrt(2) ~ 1.41 for diagonal)
				playersAdjacent[bot.Owner] = struct{}{}
			}
		}

		// If 2+ players are adjacent to this energy, it's contested
		if len(playersAdjacent) >= 2 {
			return true
		}
	}

	return false
}

// countSurvivalTurns counts turns where all players had at least one bot alive.
// This indicates the match was still competitive with all participants active.
func countSurvivalTurns(replay *Replay) int {
	if replay == nil || len(replay.Turns) == 0 {
		return 0
	}

	numPlayers := len(replay.Players)
	if numPlayers == 0 {
		return 0
	}

	survivalTurns := 0

	for _, turn := range replay.Turns {
		if allPlayersAlive(turn, numPlayers) {
			survivalTurns++
		}
	}

	return survivalTurns
}

// allPlayersAlive checks if every player has at least one living bot.
func allPlayersAlive(turn ReplayTurn, numPlayers int) bool {
	playersWithBots := make(map[int]struct{})

	for _, bot := range turn.Bots {
		if bot.Alive {
			playersWithBots[bot.Owner] = struct{}{}
		}
	}

	// All players must have at least one living bot
	return len(playersWithBots) == numPlayers
}

// countCombatDeaths counts the total number of focus-fire combat deaths (EventCombatDeath)
// across all turns in the replay. This is the key combat-density metric.
func countCombatDeaths(replay *Replay) int {
	if replay == nil || len(replay.Turns) == 0 {
		return 0
	}

	combatDeaths := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == EventCombatDeath {
				combatDeaths++
			}
		}
	}
	return combatDeaths
}

// toroidalDistance computes the toroidal distance between two positions.
func toroidalDistance(a, b Position, rows, cols int) float64 {
	dr := float64(a.Row - b.Row)
	dc := float64(a.Col - b.Col)

	// Handle toroidal wrapping
	if dr < 0 {
		dr = -dr
	}
	if dr > float64(rows)/2 {
		dr = float64(rows) - dr
	}

	if dc < 0 {
		dc = -dc
	}
	if dc > float64(cols)/2 {
		dc = float64(cols) - dc
	}

	return dr*dr + dc*dc // Return squared distance for comparison
}
