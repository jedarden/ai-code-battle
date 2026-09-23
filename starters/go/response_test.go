package main

import (
	"encoding/json"
	"testing"
)

// TestResponseIsMovesOnly pins the /turn wire contract documented in the
// README's Game Protocol section: the starter's response carries moves and
// nothing else. Spawning is engine-automatic — new bots appear at your
// active, unoccupied cores while energy covers spawn_cost — so there is no
// spawn order to send. The engine drops unknown response fields silently,
// which means a spawns field added here would do nothing while looking like
// it works; this test makes that a visible failure instead.
func TestResponseIsMovesOnly(t *testing.T) {
	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		You:     You{ID: 0, Energy: 9, Score: 1},
		Bots: []VisibleBot{
			{Position: Position{Row: 5, Col: 5}, Owner: 0},
			{Position: Position{Row: 10, Col: 10}, Owner: 1},
		},
	}

	// Same response shape handleTurn builds.
	response := map[string]any{"moves": ComputeMoves(state)}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var shaped map[string]json.RawMessage
	if err := json.Unmarshal(body, &shaped); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}
	if _, ok := shaped["moves"]; !ok {
		t.Error("response is missing the moves key")
	}
	if len(shaped) != 1 {
		t.Errorf("response should carry only moves, got %d keys", len(shaped))
	}
	for _, key := range []string{"spawns", "spawn"} {
		if _, ok := shaped[key]; ok {
			t.Errorf("response has %q key; spawn orders are not part of the schema (spawning is engine-automatic)", key)
		}
	}

	// Each move carries only position and direction.
	var moves []map[string]json.RawMessage
	if err := json.Unmarshal(shaped["moves"], &moves); err != nil {
		t.Fatalf("moves is not a JSON array: %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("got %d moves, want 1 (only the owned bot)", len(moves))
	}
	if len(moves[0]) != 2 {
		t.Errorf("move should have exactly position and direction, got %d keys", len(moves[0]))
	}
	var direction string
	if err := json.Unmarshal(moves[0]["direction"], &direction); err != nil {
		t.Fatalf("direction is not a string: %v", err)
	}
	if direction != "stay" {
		t.Errorf("direction = %q, want stay", direction)
	}
}
