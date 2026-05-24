package engine

import (
	"math/rand"
	"testing"
)

// TestMapEngagement_WinProbDependency verifies that CalculateMapEngagement
// correctly uses win_prob data to count lead changes.
func TestMapEngagement_WinProbDependency(t *testing.T) {
	// Create a replay with alternating win probs to simulate lead changes
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		WinProb: []WinProbEntry{
			{0.6, 0.4}, // Player 0 leading
			{0.4, 0.6}, // Player 1 leading - 1st crossing
			{0.6, 0.4}, // Player 0 leading - 2nd crossing
			{0.4, 0.6}, // Player 1 leading - 3rd crossing
		},
		Turns: []ReplayTurn{
			{Turn: 0, Bots: []ReplayBot{{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0}}},
			{Turn: 1, Bots: []ReplayBot{{Position: Position{Row: 1, Col: 1}, Alive: true, Owner: 0}}},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 10, Col: 10}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	// Should have 3 win prob crossings (lead changes)
	if score.WinProbCrossings != 3 {
		t.Errorf("Expected 3 win_prob crossings, got %.0f", score.WinProbCrossings)
	}

	// Engagement score should be positive (crossings contribute 3.0 each)
	if score.Engagement <= 0 {
		t.Errorf("Expected positive engagement score, got %.2f", score.Engagement)
	}
}

// TestMapEngagement_CriticalMomentsDependency verifies that CalculateMapEngagement
// correctly counts critical moments from the replay.
func TestMapEngagement_CriticalMomentsDependency(t *testing.T) {
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		CriticalMoments: []CriticalMoment{
			{Turn: 10, Delta: 0.20, Player: 0, Description: "Player 0 scores"},
			{Turn: 25, Delta: -0.25, Player: 1, Description: "Player 1 fights back"},
		},
		Turns: []ReplayTurn{
			{Turn: 0, Bots: []ReplayBot{{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0}}},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 10, Col: 10}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	// Should have 2 critical moments
	if score.CriticalMoments != 2 {
		t.Errorf("Expected 2 critical moments, got %d", score.CriticalMoments)
	}

	// Engagement should include critical moments contribution (2.0 each)
	expectedContribution := float64(2) * 2.0
	if score.Engagement < expectedContribution {
		t.Errorf("Expected engagement >= %.2f from critical moments, got %.2f", expectedContribution, score.Engagement)
	}
}

// TestMapEngagement_ResourceContestTurns verifies that contested energy turns are counted.
func TestMapEngagement_ResourceContestTurns(t *testing.T) {
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		Turns: []ReplayTurn{
			{
				Turn:   0,
				Energy: []Position{{Row: 5, Col: 5}},
				Bots: []ReplayBot{
					{Position: Position{Row: 5, Col: 4}, Alive: true, Owner: 0}, // Adjacent to energy
					{Position: Position{Row: 5, Col: 6}, Alive: true, Owner: 1}, // Adjacent to energy
				},
			},
			{
				Turn:   1,
				Energy: []Position{{Row: 10, Col: 10}},
				Bots: []ReplayBot{
					{Position: Position{Row: 10, Col: 9}, Alive: true, Owner: 0}, // Only player 0 adjacent
				},
			},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 15, Col: 15}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	// Turn 0 is contested (both players adjacent), turn 1 is not
	if score.ResourceContestTurns != 1 {
		t.Errorf("Expected 1 resource contest turn, got %d", score.ResourceContestTurns)
	}
}

// TestMapEngagement_SurvivalTurns verifies that survival turns are counted correctly.
func TestMapEngagement_SurvivalTurns(t *testing.T) {
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		Turns: []ReplayTurn{
			{
				Turn: 0,
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: true, Owner: 1},
				},
			},
			{
				Turn: 1,
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: false, Owner: 1}, // Player 1 bot died
				},
			},
			{
				Turn: 2,
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: true, Owner: 1},
				},
			},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 15, Col: 15}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	// Turns 0 and 2 have all players alive, turn 1 does not
	if score.SurvivalTurns != 2 {
		t.Errorf("Expected 2 survival turns, got %d", score.SurvivalTurns)
	}
}

// TestMapEngagement_EmptyReplay handles empty/nil replays gracefully.
func TestMapEngagement_EmptyReplay(t *testing.T) {
	score1 := CalculateMapEngagement(nil)
	score2 := CalculateMapEngagement(&Replay{})

	// Both should return zero scores without panicking
	if score1.Engagement != 0 || score2.Engagement != 0 {
		t.Error("Empty replay should return zero engagement")
	}
}

// TestMapEngagement_Formula verifies the engagement score uses the correct formula:
// score = win_prob_crossings * 3.0 + critical_moments * 2.0 + resource_contest_turns * 1.5 + survival_turns * 0.5
func TestMapEngagement_Formula(t *testing.T) {
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		WinProb: []WinProbEntry{
			{0.6, 0.4}, // Player 0 leading
			{0.4, 0.6}, // Player 1 leading - 1st crossing
		},
		CriticalMoments: []CriticalMoment{
			{Turn: 10, Delta: 0.20, Player: 0, Description: "Player 0 scores"},
		},
		Turns: []ReplayTurn{
			{
				Turn:   0,
				Energy: []Position{{Row: 5, Col: 5}},
				Bots: []ReplayBot{
					{Position: Position{Row: 5, Col: 4}, Alive: true, Owner: 0},
					{Position: Position{Row: 5, Col: 6}, Alive: true, Owner: 1},
				},
			},
			{
				Turn:   1,
				Energy: []Position{{Row: 10, Col: 10}},
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: true, Owner: 1},
				},
			},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 15, Col: 15}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	// Count each metric
	winProbCrossings := 1.0   // One lead change
	combatDeaths := 0         // No combat deaths in this replay
	criticalMoments := 1      // One critical moment
	resourceContestTurns := 1 // Turn 0 has contested energy
	survivalTurns := 2        // Both turns have all players alive

	// Expected formula: 1.0*3.0 + 0*3.0 + 1*2.0 + 1*1.5 + 2*0.5 = 3.0 + 0 + 2.0 + 1.5 + 1.0 = 7.5
	expectedEngagement := winProbCrossings*3.0 + float64(combatDeaths)*3.0 + float64(criticalMoments)*2.0 + float64(resourceContestTurns)*1.5 + float64(survivalTurns)*0.5

	if score.Engagement != expectedEngagement {
		t.Errorf("Expected engagement %.2f, got %.2f", expectedEngagement, score.Engagement)
	}

	if score.WinProbCrossings != winProbCrossings {
		t.Errorf("Expected win_prob_crossings %.0f, got %.0f", winProbCrossings, score.WinProbCrossings)
	}

	if score.CombatDeaths != combatDeaths {
		t.Errorf("Expected combat_deaths %d, got %d", combatDeaths, score.CombatDeaths)
	}

	if score.CriticalMoments != criticalMoments {
		t.Errorf("Expected critical_moments %d, got %d", criticalMoments, score.CriticalMoments)
	}

	if score.ResourceContestTurns != resourceContestTurns {
		t.Errorf("Expected resource_contest_turns %d, got %d", resourceContestTurns, score.ResourceContestTurns)
	}

	if score.SurvivalTurns != survivalTurns {
		t.Errorf("Expected survival_turns %d, got %d", survivalTurns, score.SurvivalTurns)
	}
}

// TestMapEngagement_NoContestedEnergy verifies that energy contested by only one player is not counted.
func TestMapEngagement_NoContestedEnergy(t *testing.T) {
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		Turns: []ReplayTurn{
			{
				Turn:   0,
				Energy: []Position{{Row: 5, Col: 5}},
				Bots: []ReplayBot{
					{Position: Position{Row: 5, Col: 4}, Alive: true, Owner: 0}, // Only player 0 adjacent
				},
			},
			{
				Turn:   1,
				Energy: []Position{}, // No energy
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
				},
			},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 15, Col: 15}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	if score.ResourceContestTurns != 0 {
		t.Errorf("Expected 0 resource contest turns, got %d", score.ResourceContestTurns)
	}
}

// TestMapEngagement_PlayerElimination verifies survival turns count decreases when a player is eliminated.
func TestMapEngagement_PlayerElimination(t *testing.T) {
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		Turns: []ReplayTurn{
			{
				Turn: 0,
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: true, Owner: 1},
				},
			},
			{
				Turn: 1,
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: true, Owner: 1},
				},
			},
			{
				Turn: 2,
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: false, Owner: 1}, // Eliminated
				},
			},
			{
				Turn: 3,
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
					{Position: Position{Row: 10, Col: 10}, Alive: false, Owner: 1}, // Still dead
				},
			},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 15, Col: 15}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	// Only turns 0 and 1 have all players alive
	if score.SurvivalTurns != 2 {
		t.Errorf("Expected 2 survival turns, got %d", score.SurvivalTurns)
	}
}

// TestWinProb_ComputeAndSet verifies that ComputeWinProbability produces
// valid results and SetWinProbability correctly stores them.
func TestWinProb_ComputeAndSet(t *testing.T) {
	config := DefaultConfig()
	config.Rows = 10
	config.Cols = 10
	config.MaxTurns = 20

	gs := NewGameState(config, rand.New(rand.NewSource(42)))
	gs.AddPlayer()
	gs.AddPlayer()

	// Create simple snapshots
	snapshots := []*GameState{gs.Clone()}

	rng := rand.New(rand.NewSource(123))
	winProbs, moments := ComputeWinProbability(snapshots, 10, rng)

	// Should have win prob for each snapshot
	if len(winProbs) != len(snapshots) {
		t.Errorf("Expected %d win prob entries, got %d", len(snapshots), len(winProbs))
	}

	// Each entry should have 2 player probabilities
	for i, entry := range winProbs {
		if len(entry) != 2 {
			t.Errorf("WinProb entry %d has %d values, want 2", i, len(entry))
		}
		// Values should be in [0, 1]
		for j, prob := range entry {
			if prob < 0 || prob > 1 {
				t.Errorf("WinProb entry %d player %d has invalid prob %.2f", i, j, prob)
			}
		}
	}

	// Verify replay writer can store the data
	rw := NewReplayWriter("test_match", config)
	rw.SetWinProbability(winProbs, moments)

	replay := rw.GetReplay()
	if len(replay.WinProb) != len(winProbs) {
		t.Errorf("Replay has %d win prob entries, want %d", len(replay.WinProb), len(winProbs))
	}
	if len(replay.CriticalMoments) != len(moments) {
		t.Errorf("Replay has %d critical moments, want %d", len(replay.CriticalMoments), len(moments))
	}
}

// TestMapEngagement_CombatDeaths verifies that focus-fire combat deaths are counted correctly.
func TestMapEngagement_CombatDeaths(t *testing.T) {
	replay := &Replay{
		Config: Config{Rows: 20, Cols: 20, MaxTurns: 100},
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}, CombatDeaths: []int{0, 0}},
		Turns: []ReplayTurn{
			{
				Turn: 0,
				Events: []Event{
					{Type: EventCombatDeath, Turn: 0, Details: map[string]interface{}{"bot_id": 1, "owner": 0}},
				},
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 1},
				},
			},
			{
				Turn: 1,
				Events: []Event{
					{Type: EventCombatDeath, Turn: 1, Details: map[string]interface{}{"bot_id": 2, "owner": 1}},
					{Type: EventCombatDeath, Turn: 1, Details: map[string]interface{}{"bot_id": 3, "owner": 1}},
				},
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
				},
			},
			{
				Turn: 2,
				Events: []Event{
					{Type: EventBotDied, Turn: 2, Details: map[string]interface{}{"bot_id": 4, "owner": 0, "reason": "zone"}},
				},
				Bots: []ReplayBot{
					{Position: Position{Row: 0, Col: 0}, Alive: true, Owner: 0},
				},
			},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 15, Col: 15}},
		},
		Players: []ReplayPlayer{{ID: 0}, {ID: 1}},
	}

	score := CalculateMapEngagement(replay)

	// Should count 3 combat deaths (1 on turn 0, 2 on turn 1)
	// The zone death on turn 2 should NOT be counted
	if score.CombatDeaths != 3 {
		t.Errorf("Expected 3 combat deaths, got %d", score.CombatDeaths)
	}

	// Combat deaths should contribute 3.0 * 3 = 9.0 to engagement
	expectedCombatContribution := 3.0 * 3.0
	if score.Engagement < expectedCombatContribution {
		t.Errorf("Expected engagement >= %.2f from combat deaths, got %.2f", expectedCombatContribution, score.Engagement)
	}
}
