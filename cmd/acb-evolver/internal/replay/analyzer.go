// Package replay analyzes match replays to extract strategic insights
// for the LLM evolution prompt.
//
// The analyzer processes completed match replays and produces:
//   - Key moments: significant events that changed the match trajectory
//   - Strategies: winning tactics employed by the victor
//   - Weaknesses: exploitable patterns in the loser's play
package replay

import (
	"github.com/aicodebattle/acb/engine"
)

// Analysis holds the extracted insights from a single match replay.
type Analysis struct {
	// MatchID is the unique identifier of the analyzed match.
	MatchID string
	// WinnerName is the name of the winning player (empty for draws).
	WinnerName string
	// LoserName is the name of the losing player (empty for draws).
	LoserName string
	// Condition is the win condition: "elimination", "dominance", "turns", or "draw".
	Condition string
	// TurnCount is the total number of turns played.
	TurnCount int
	// Scores holds the final scores for each player slot.
	Scores []int
	// KeyMoments are notable events that influenced the outcome.
	KeyMoments []string
	// Strategies lists the successful tactics used by the winner.
	Strategies []string
	// Weaknesses lists the exploitable patterns in the loser's play.
	Weaknesses []string
}

// Analyzer processes replays and extracts strategic insights.
type Analyzer struct{}

// NewAnalyzer creates a new replay analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// Analyze processes a replay and returns a structured analysis.
func (a *Analyzer) Analyze(replay *engine.Replay) *Analysis {
	if replay == nil {
		return nil
	}

	analysis := &Analysis{
		MatchID:    replay.MatchID,
		TurnCount:  len(replay.Turns),
		Scores:     make([]int, 0),
		Condition:  "",
	}

	// Extract result information
	if replay.Result != nil {
		analysis.Condition = replay.Result.Reason
		if len(replay.Result.Scores) > 0 {
			analysis.Scores = replay.Result.Scores
		}
	}

	// Identify winner and loser
	if len(replay.Players) >= 2 && replay.Result != nil && replay.Result.Winner >= 0 {
		winnerIdx := replay.Result.Winner
		if winnerIdx < len(replay.Players) {
			analysis.WinnerName = replay.Players[winnerIdx].Name
		}
		// Loser is the other player (for 2-player matches)
		if len(replay.Players) == 2 {
			loserIdx := 1 - winnerIdx
			if replay.Result.Winner >= 0 {
				analysis.LoserName = replay.Players[loserIdx].Name
			}
		}
	}

	// Analyze the replay for strategic insights
	a.analyzeKeyMoments(replay, analysis)
	a.analyzeStrategies(replay, analysis)
	a.analyzeWeaknesses(replay, analysis)

	return analysis
}

// analyzeKeyMoments identifies significant events that shaped the match.
func (a *Analyzer) analyzeKeyMoments(replay *engine.Replay, analysis *Analysis) {
	var moments []string

	winnerID := -1
	if replay.Result != nil {
		winnerID = replay.Result.Winner
	}

	// Track key metrics over time to detect pivotal moments
	prevBotCounts := make(map[int]int)
	prevScores := make(map[int]int)
	coreFlips := make(map[int]int) // track core ownership changes by player

	for turnIdx, turn := range replay.Turns {
		turnNum := turn.Turn

		// Count bots per player
		botCounts := make(map[int]int)
		for _, bot := range turn.Bots {
			if bot.Alive {
				botCounts[bot.Owner]++
			}
		}

		// Detect bot count changes (spawn/death events)
		for playerID, count := range botCounts {
			prevCount := prevBotCounts[playerID]
			if turnNum > 0 && prevCount > 0 {
				diff := count - prevCount
				if diff <= -3 {
					moments = append(moments, formatMoment(turnNum, playerID, replay.Players,
						"lost %d bots in rapid succession", -diff))
				} else if diff >= 3 {
					moments = append(moments, formatMoment(turnNum, playerID, replay.Players,
						"spawned %d bots in rapid succession", diff))
				}
			}
			prevBotCounts[playerID] = count
		}

		// Process events for notable occurrences
		for _, event := range turn.Events {
			switch event.Type {
			case "core_captured":
				details, ok := event.Details.(map[string]interface{})
				if !ok {
					continue
				}
				// Track core ownership changes
				if attacker, ok := details["attacker_id"].(float64); ok {
					playerID := int(attacker)
					coreFlips[playerID]++
					if coreFlips[playerID] == 1 {
						moments = append(moments, formatMoment(turnNum, playerID, replay.Players,
							"captured first enemy core"))
					}
				}
			case "combat_death":
				if turnNum < 50 {
					moments = append(moments, formatMoment(turnNum, -1, replay.Players,
						"early combat casualty"))
				}
			}
		}

		// Detect score swings
		for playerID, score := range turn.Scores {
			prevScore := prevScores[playerID]
			if turnNum > 0 && prevScore > 0 {
				diff := score - prevScore
				if diff >= 20 {
					moments = append(moments, formatMoment(turnNum, playerID, replay.Players,
						"gained %d score in single turn", diff))
				}
			}
			prevScores[playerID] = score
		}

		// Limit key moments to avoid prompt bloat
		if len(moments) >= 5 && turnIdx < len(replay.Turns)-10 {
			break
		}
	}

	// Add final score summary if there's a clear winner
	if winnerID >= 0 && len(analysis.Scores) >= 2 {
		scoreDiff := analysis.Scores[winnerID]
		if len(analysis.Scores) > 1 {
			loserID := 1 - winnerID
			if winnerID == 1 {
				loserID = 0
			}
			if loserID < len(analysis.Scores) {
				scoreDiff = analysis.Scores[winnerID] - analysis.Scores[loserID]
			}
		}
		if scoreDiff > 100 {
			moments = append(moments, "Final score advantage: dominant victory")
		} else if scoreDiff > 50 {
			moments = append(moments, "Final score advantage: clear victory")
		} else if scoreDiff > 20 {
			moments = append(moments, "Final score advantage: narrow victory")
		}
	}

	analysis.KeyMoments = dedupeMoments(moments)
}

// analyzeStrategies identifies winning tactics from the replay.
func (a *Analyzer) analyzeStrategies(replay *engine.Replay, analysis *Analysis) {
	if replay.Result == nil || replay.Result.Winner < 0 {
		return
	}

	winnerID := replay.Result.Winner
	var strategies []string

	// Analyze early game (first 50 turns)
	earlyBots := make(map[int]int)
	earlyEnergy := make(map[int]int)
	for _, turn := range replay.Turns {
		if turn.Turn > 50 {
			break
		}
		for _, bot := range turn.Bots {
			if bot.Alive {
				earlyBots[bot.Owner]++
			}
		}
		earlyEnergy[turn.Turn%10] = len(turn.Energy)
	}

	// Detect aggressive early expansion
	if earlyBots[winnerID] > earlyBots[1-winnerID]*2 && winnerID < len(earlyBots) {
		strategies = append(strategies, "aggressive early expansion")
	}

	// Analyze core control
	coreCaptures := make(map[int]int)
	for _, turn := range replay.Turns {
		for _, core := range turn.Cores {
			if core.Owner == winnerID && core.Active {
				coreCaptures[winnerID]++
			}
		}
	}
	if coreCaptures[winnerID] > 1 {
		strategies = append(strategies, "multi-core control")
	}

	// Detect win condition patterns
	switch replay.Result.Reason {
	case "elimination":
		strategies = append(strategies, "complete elimination of opponent")
	case "dominance":
		strategies = append(strategies, "sustained bot superiority")
	case "turns":
		strategies = append(strategies, "score accumulation strategy")
	}

	// Analyze spawn patterns (energy management)
	spawnRate := 0
	if len(replay.Turns) > 100 {
		botGrowth := 0
		for _, turn := range replay.Turns[80:100] {
			for _, bot := range turn.Bots {
				if bot.Alive && bot.Owner == winnerID {
					botGrowth++
				}
			}
		}
		spawnRate = botGrowth / 20
	}
	if spawnRate >= 3 {
		strategies = append(strategies, "high spawn tempo")
	} else if spawnRate >= 1 {
		strategies = append(strategies, "controlled spawn rate")
	}

	// Detect energy focus vs combat focus
	energyCollected := 0
	combatDeaths := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == "energy_collected" {
				energyCollected++
			} else if event.Type == "combat_death" {
				combatDeaths++
			}
		}
	}
	if energyCollected > combatDeaths*2 {
		strategies = append(strategies, "energy-focused economy")
	} else if combatDeaths > energyCollected {
		strategies = append(strategies, "aggressive combat pressure")
	}

	analysis.Strategies = dedupe(strategies)
}

// analyzeWeaknesses identifies exploitable patterns in the loser's play.
func (a *Analyzer) analyzeWeaknesses(replay *engine.Replay, analysis *Analysis) {
	if replay.Result == nil || replay.Result.Winner < 0 || len(replay.Players) < 2 {
		return
	}

	loserID := 1 - replay.Result.Winner
	var weaknesses []string

	// Analyze bot count trends
	botCounts := make(map[int][]int)
	for _, turn := range replay.Turns {
		count := 0
		for _, bot := range turn.Bots {
			if bot.Alive && bot.Owner == loserID {
				count++
			}
		}
		botCounts[loserID] = append(botCounts[loserID], count)
	}

	// Detect bot shortage issues
	if len(botCounts[loserID]) > 50 {
		lateBots := botCounts[loserID][len(botCounts[loserID])-1]
		if lateBots < 3 {
			weaknesses = append(weaknesses, "insufficient bot production")
		}
	}

	// Detect passive play
	spawnEvents := 0
	for i, turn := range replay.Turns {
		if i == 0 {
			continue
		}
		prevCount := 0
		currCount := 0
		for _, bot := range replay.Turns[i-1].Bots {
			if bot.Alive && bot.Owner == loserID {
				prevCount++
			}
		}
		for _, bot := range turn.Bots {
			if bot.Alive && bot.Owner == loserID {
				currCount++
			}
		}
		if currCount > prevCount {
			spawnEvents++
		}
	}
	if spawnEvents < 5 && len(replay.Turns) > 100 {
		weaknesses = append(weaknesses, "passive spawn behavior")
	}

	// Detect core defense issues
	coreLosses := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == "core_captured" {
				details, ok := event.Details.(map[string]interface{})
				if ok {
					if victim, ok := details["victim_id"].(float64); ok && int(victim) == loserID {
						coreLosses++
					}
				}
			}
		}
	}
	if coreLosses > 0 {
		weaknesses = append(weaknesses, "weak core defense")
	}

	// Detect energy inefficiency
	energyCollected := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == "energy_collected" {
				energyCollected++
			}
		}
	}
	if energyCollected < 10 && len(replay.Turns) > 100 {
		weaknesses = append(weaknesses, "poor energy collection")
	}

	// Detect early elimination vulnerability
	if replay.Result.Reason == "elimination" && len(replay.Turns) < 100 {
		weaknesses = append(weaknesses, "vulnerable to early aggression")
	}

	// Detect score gap accumulation
	if len(analysis.Scores) > loserID && len(analysis.Scores) > replay.Result.Winner {
		scoreGap := analysis.Scores[replay.Result.Winner] - analysis.Scores[loserID]
		if scoreGap > 100 {
			weaknesses = append(weaknesses, "failed to contest score")
		}
	}

	analysis.Weaknesses = dedupe(weaknesses)
}

// formatMoment creates a formatted key moment string.
func formatMoment(turn, playerID int, players []engine.ReplayPlayer, format string, args ...interface{}) string {
	playerName := ""
	if playerID >= 0 && playerID < len(players) {
		playerName = players[playerID].Name
	}
	return formatPlayerMoment(turn, playerName, format, args...)
}

// formatPlayerMoment formats a moment with player name context.
func formatPlayerMoment(turn int, playerName, format string, args ...interface{}) string {
	args = append([]interface{}{turn}, args...)
	if playerName != "" {
		return playerName + " (turn %d): " + format
	}
	return "Turn %d: " + format
}

// dedupeMoments removes duplicate or similar moments.
func dedupeMoments(moments []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range moments {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	// Limit to 5 most relevant moments
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}

// dedupe removes duplicate strings from a slice.
func dedupe(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
