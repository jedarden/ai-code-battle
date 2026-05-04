package engine

import "math"

// MapEngagementScore represents the engagement metrics for a map from a single match.
type MapEngagementScore struct {
	WinProbCrossings float64 // Number of times win prob crossed 50%
	CriticalMoments  int     // Count of critical moments
	MapCoveragePct   float64 // Percentage of map tiles visited
	Closeness        float64 // 1.0 - (score_diff / max_possible_score)
	TurnPct          float64 // Actual turns / max_turns
	Engagement       float64 // Combined engagement score
}

// CalculateMapEngagement computes the engagement score for a map based on replay data.
// The engagement formula is:
// engagement = win_prob_crossings * 3.0 + critical_moments * 2.0 + map_coverage_pct * 1.0 + closeness * 2.0 + turn_pct * 1.0
func CalculateMapEngagement(replay *Replay) MapEngagementScore {
	if replay == nil || len(replay.Turns) == 0 {
		return MapEngagementScore{}
	}

	// Count win probability crossings (times the leader changed)
	winProbCrossings := countWinProbCrossings(replay.WinProb)

	// Count critical moments
	criticalMoments := len(replay.CriticalMoments)

	// Calculate map coverage (percentage of unique tiles visited)
	mapCoveragePct := calculateMapCoverage(replay)

	// Calculate closeness (how close the final score was)
	closeness := calculateCloseness(replay)

	// Calculate turn percentage
	turnPct := float64(replay.Result.Turns) / float64(replay.Config.MaxTurns)

	// Calculate combined engagement score
	engagement := float64(winProbCrossings)*3.0 +
		float64(criticalMoments)*2.0 +
		mapCoveragePct*1.0 +
		closeness*2.0 +
		turnPct*1.0

	return MapEngagementScore{
		WinProbCrossings: winProbCrossings,
		CriticalMoments:  criticalMoments,
		MapCoveragePct:   mapCoveragePct,
		Closeness:        closeness,
		TurnPct:          turnPct,
		Engagement:       engagement,
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
