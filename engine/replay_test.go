package engine

import (
	"math/rand"
	"testing"
)

// TestDeltaEncoding verifies that scores and energy_held are delta-encoded
// (only written when they change) in format version 2.1+
func TestDeltaEncoding(t *testing.T) {
	config := DefaultConfig()
	config.Rows = 10
	config.Cols = 10
	config.MaxTurns = 20

	rw := NewReplayWriter("test-delta", config)
	rw.SetPlayers([]ReplayPlayer{
		{ID: 0, Name: "Player 0"},
		{ID: 1, Name: "Player 1"},
	})

	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(config, rng)

	// Add players
	gs.AddPlayer() // Player 0
	gs.AddPlayer() // Player 1

	// Turn 0: Initial state - scores and energy SHOULD be written
	rw.RecordTurn(gs, nil)

	// Turn 1: Same scores/energy - should NOT be written again (delta encoding)
	gs.Turn = 1
	rw.RecordTurn(gs, nil)

	// Turn 2: Score change for player 0 - should be written
	gs.Turn = 2
	gs.Players[0].Score = 5
	rw.RecordTurn(gs, nil)

	// Turn 3: Same scores as turn 2 - should NOT be written
	gs.Turn = 3
	rw.RecordTurn(gs, nil)

	// Turn 4: Energy change for player 1 - should be written
	gs.Turn = 4
	gs.Players[1].Energy = 10
	rw.RecordTurn(gs, nil)

	// Turn 5: Both change - should be written
	gs.Turn = 5
	gs.Players[0].Score = 7
	gs.Players[1].Energy = 15
	rw.RecordTurn(gs, nil)

	rw.Finalize(nil)
	replay := rw.GetReplay()
	turns := replay.Turns

	// Verify delta encoding behavior
	if len(turns) != 6 {
		t.Fatalf("Expected 6 turns, got %d", len(turns))
	}

	// Turn 0: Should have scores and energy (first turn always written)
	if turns[0].Scores == nil {
		t.Error("Turn 0 should have Scores (first turn)")
	}
	if turns[0].EnergyHeld == nil {
		t.Error("Turn 0 should have EnergyHeld (first turn)")
	}

	// Turn 1: Should NOT have scores or energy (no change from turn 0)
	if turns[1].Scores != nil {
		t.Error("Turn 1 should NOT have Scores (no change)")
	}
	if turns[1].EnergyHeld != nil {
		t.Error("Turn 1 should NOT have EnergyHeld (no change)")
	}

	// Turn 2: Should have scores (changed), but not energy (no change)
	if turns[2].Scores == nil {
		t.Error("Turn 2 should have Scores (score changed)")
	}
	if len(turns[2].Scores) != 2 || turns[2].Scores[0] != 5 {
		t.Errorf("Turn 2 Scores should be [5,0], got %v", turns[2].Scores)
	}
	if turns[2].EnergyHeld != nil {
		t.Error("Turn 2 should NOT have EnergyHeld (no change)")
	}

	// Turn 3: Should NOT have scores or energy (no change from turn 2)
	if turns[3].Scores != nil {
		t.Error("Turn 3 should NOT have Scores (no change)")
	}
	if turns[3].EnergyHeld != nil {
		t.Error("Turn 3 should NOT have EnergyHeld (no change)")
	}

	// Turn 4: Should have energy (changed), but not scores (no change)
	if turns[4].Scores != nil {
		t.Error("Turn 4 should NOT have Scores (no change)")
	}
	if turns[4].EnergyHeld == nil {
		t.Error("Turn 4 should have EnergyHeld (energy changed)")
	}
	if len(turns[4].EnergyHeld) != 2 || turns[4].EnergyHeld[1] != 10 {
		t.Errorf("Turn 4 EnergyHeld should be [0,10], got %v", turns[4].EnergyHeld)
	}

	// Turn 5: Should have both (both changed)
	if turns[5].Scores == nil {
		t.Error("Turn 5 should have Scores (changed)")
	}
	if turns[5].EnergyHeld == nil {
		t.Error("Turn 5 should have EnergyHeld (changed)")
	}
	if len(turns[5].Scores) != 2 || turns[5].Scores[0] != 7 {
		t.Errorf("Turn 5 Scores should be [7,5], got %v", turns[5].Scores)
	}
	if len(turns[5].EnergyHeld) != 2 || turns[5].EnergyHeld[1] != 15 {
		t.Errorf("Turn 5 EnergyHeld should be [0,15], got %v", turns[5].EnergyHeld)
	}

	// Verify format version is 2.1
	if replay.FormatVersion != "2.1" {
		t.Errorf("Expected format version 2.1, got %s", replay.FormatVersion)
	}
}

// TestReplayRoundTrip verifies that a replay can be serialized and deserialized
// with delta encoding intact
func TestReplayRoundTrip(t *testing.T) {
	config := DefaultConfig()
	config.Rows = 10
	config.Cols = 10
	config.MaxTurns = 20

	rw := NewReplayWriter("test-roundtrip", config)
	rw.SetPlayers([]ReplayPlayer{
		{ID: 0, Name: "Player 0"},
		{ID: 1, Name: "Player 1"},
	})

	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(config, rng)
	gs.AddPlayer()
	gs.AddPlayer()

	// Record several turns
	for i := 0; i < 10; i++ {
		gs.Turn = i
		// Change scores every 3 turns
		if i%3 == 0 && i > 0 {
			gs.Players[0].Score = i * 2
		}
		rw.RecordTurn(gs, nil)
	}

	// Finalize and serialize
	rw.Finalize(nil)
	json, err := ReplayToJSON(rw.GetReplay())
	if err != nil {
		t.Fatalf("Failed to serialize replay: %v", err)
	}

	reloaded, err := LoadReplay(json)
	if err != nil {
		t.Fatalf("Failed to deserialize replay: %v", err)
	}

	// Verify format version preserved
	if reloaded.FormatVersion != "2.1" {
		t.Errorf("Expected format version 2.1 after roundtrip, got %s", reloaded.FormatVersion)
	}

	// Verify turns count preserved
	if len(reloaded.Turns) != 10 {
		t.Errorf("Expected 10 turns after roundtrip, got %d", len(reloaded.Turns))
	}

	// Verify delta encoding is preserved in the serialized form
	// (turns without changes should have nil scores/energy)
	for i, turn := range reloaded.Turns {
		if i == 0 {
			// First turn should always have data
			if turn.Scores == nil || turn.EnergyHeld == nil {
				t.Error("First turn should have Scores and EnergyHeld")
			}
		} else if i%3 != 0 {
			// Turns without changes should NOT have scores/energy
			if turn.Scores != nil {
				t.Errorf("Turn %d should NOT have Scores (no change)", i)
			}
			if turn.EnergyHeld != nil {
				t.Errorf("Turn %d should NOT have EnergyHeld (no change)", i)
			}
		} else {
			// Turns with changes SHOULD have data
			if turn.Scores == nil {
				t.Errorf("Turn %d should have Scores (changed)", i)
			}
		}
	}
}
