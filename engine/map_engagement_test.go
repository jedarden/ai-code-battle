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
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}},
		WinProb: []WinProbEntry{
			{0.6, 0.4}, // Player 0 leading
			{0.4, 0.6}, // Player 1 leading - 1st crossing
			{0.6, 0.4}, // Player 0 leading - 2nd crossing
			{0.4, 0.6}, // Player 1 leading - 3rd crossing
		},
		Turns: []ReplayTurn{
			{Turn: 0, Bots: []ReplayBot{{Position: Position{Row: 0, Col: 0}, Alive: true}}},
			{Turn: 1, Bots: []ReplayBot{{Position: Position{Row: 1, Col: 1}, Alive: true}}},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 10, Col: 10}},
		},
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
		Result: &MatchResult{Turns: 50, Scores: []int{5, 4}},
		CriticalMoments: []CriticalMoment{
			{Turn: 10, Delta: 0.20, Player: 0, Description: "Player 0 scores"},
			{Turn: 25, Delta: -0.25, Player: 1, Description: "Player 1 fights back"},
		},
		Turns: []ReplayTurn{
			{Turn: 0, Bots: []ReplayBot{{Position: Position{Row: 0, Col: 0}, Alive: true}}},
		},
		Map: ReplayMap{
			Walls: []Position{{Row: 10, Col: 10}},
		},
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

// TestMapEngagement_EmptyReplay handles empty/nil replays gracefully.
func TestMapEngagement_EmptyReplay(t *testing.T) {
	score1 := CalculateMapEngagement(nil)
	score2 := CalculateMapEngagement(&Replay{})

	// Both should return zero scores without panicking
	if score1.Engagement != 0 || score2.Engagement != 0 {
		t.Error("Empty replay should return zero engagement")
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
