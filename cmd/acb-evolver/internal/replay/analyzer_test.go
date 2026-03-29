package replay

import (
	"testing"
	"time"

	"github.com/aicodebattle/acb/engine"
)

func TestAnalyzer_Analyze_NilReplay(t *testing.T) {
	a := NewAnalyzer()
	got := a.Analyze(nil)
	if got != nil {
		t.Errorf("expected nil for nil replay, got %+v", got)
	}
}

func TestAnalyzer_Analyze_BasicMatch(t *testing.T) {
	a := NewAnalyzer()

	replay := &engine.Replay{
		FormatVersion: "1.0",
		MatchID:       "test-match-001",
		StartTime:     time.Now(),
		EndTime:       time.Now(),
		Result: &engine.MatchResult{
			Winner:  0,
			Reason:  "dominance",
			Turns:   150,
			Scores:  []int{120, 45},
			Energy:  []int{15, 8},
			BotsAlive: []int{8, 2},
		},
		Players: []engine.ReplayPlayer{
			{ID: 0, Name: "WinnerBot"},
			{ID: 1, Name: "LoserBot"},
		},
		Map: engine.ReplayMap{
			Rows: 60,
			Cols: 60,
		},
		Turns: []engine.ReplayTurn{
			{
				Turn:   0,
				Bots:   []engine.ReplayBot{},
				Scores: []int{0, 0},
			},
			{
				Turn:   50,
				Bots:   []engine.ReplayBot{},
				Scores: []int{40, 20},
			},
			{
				Turn:   100,
				Bots:   []engine.ReplayBot{},
				Scores: []int{80, 35},
			},
			{
				Turn:   150,
				Bots:   []engine.ReplayBot{},
				Scores: []int{120, 45},
			},
		},
	}

	got := a.Analyze(replay)

	if got == nil {
		t.Fatal("expected non-nil analysis")
	}

	if got.MatchID != "test-match-001" {
		t.Errorf("expected MatchID 'test-match-001', got %q", got.MatchID)
	}

	if got.WinnerName != "WinnerBot" {
		t.Errorf("expected WinnerName 'WinnerBot', got %q", got.WinnerName)
	}

	if got.LoserName != "LoserBot" {
		t.Errorf("expected LoserName 'LoserBot', got %q", got.LoserName)
	}

	if got.Condition != "dominance" {
		t.Errorf("expected Condition 'dominance', got %q", got.Condition)
	}

	if got.TurnCount != 4 {
		t.Errorf("expected TurnCount 4 (number of turn records), got %d", got.TurnCount)
	}
}

func TestAnalyzer_Analyze_EliminationMatch(t *testing.T) {
	a := NewAnalyzer()

	replay := &engine.Replay{
		MatchID: "elimination-match",
		Result: &engine.MatchResult{
			Winner:  1,
			Reason:  "elimination",
			Turns:   75,
			Scores:  []int{10, 85},
			BotsAlive: []int{0, 6},
		},
		Players: []engine.ReplayPlayer{
			{ID: 0, Name: "EliminatedBot"},
			{ID: 1, Name: "VictorBot"},
		},
		Turns: []engine.ReplayTurn{
			{Turn: 0, Bots: []engine.ReplayBot{}},
			{Turn: 30, Bots: []engine.ReplayBot{
				{ID: 1, Owner: 0, Alive: true},
				{ID: 2, Owner: 1, Alive: true},
				{ID: 3, Owner: 1, Alive: true},
			}},
			{Turn: 60, Bots: []engine.ReplayBot{
				{ID: 2, Owner: 1, Alive: true},
				{ID: 3, Owner: 1, Alive: true},
				{ID: 4, Owner: 1, Alive: true},
			}},
			{Turn: 75, Bots: []engine.ReplayBot{
				{ID: 2, Owner: 1, Alive: true},
				{ID: 3, Owner: 1, Alive: true},
				{ID: 4, Owner: 1, Alive: true},
				{ID: 5, Owner: 1, Alive: true},
				{ID: 6, Owner: 1, Alive: true},
				{ID: 7, Owner: 1, Alive: true},
			}},
		},
	}

	got := a.Analyze(replay)

	if got.WinnerName != "VictorBot" {
		t.Errorf("expected WinnerName 'VictorBot', got %q", got.WinnerName)
	}

	if got.LoserName != "EliminatedBot" {
		t.Errorf("expected LoserName 'EliminatedBot', got %q", got.LoserName)
	}

	if got.Condition != "elimination" {
		t.Errorf("expected Condition 'elimination', got %q", got.Condition)
	}

	// Should detect elimination strategy
	foundElimination := false
	for _, s := range got.Strategies {
		if s == "complete elimination of opponent" {
			foundElimination = true
			break
		}
	}
	if !foundElimination {
		t.Error("expected 'complete elimination of opponent' in strategies")
	}

	// Should detect vulnerability to early aggression
	foundVulnerable := false
	for _, w := range got.Weaknesses {
		if w == "vulnerable to early aggression" {
			foundVulnerable = true
			break
		}
	}
	if !foundVulnerable {
		t.Errorf("expected 'vulnerable to early aggression' in weaknesses, got %v", got.Weaknesses)
	}
}

func TestAnalyzer_Analyze_DrawMatch(t *testing.T) {
	a := NewAnalyzer()

	replay := &engine.Replay{
		MatchID: "draw-match",
		Result: &engine.MatchResult{
			Winner:  -1,
			Reason:  "draw",
			Turns:   500,
			Scores:  []int{100, 100},
		},
		Players: []engine.ReplayPlayer{
			{ID: 0, Name: "Bot1"},
			{ID: 1, Name: "Bot2"},
		},
		Turns: []engine.ReplayTurn{
			{Turn: 0, Bots: []engine.ReplayBot{}},
			{Turn: 500, Bots: []engine.ReplayBot{}},
		},
	}

	got := a.Analyze(replay)

	if got.WinnerName != "" {
		t.Errorf("expected empty WinnerName for draw, got %q", got.WinnerName)
	}

	if got.LoserName != "" {
		t.Errorf("expected empty LoserName for draw, got %q", got.LoserName)
	}

	if got.Condition != "draw" {
		t.Errorf("expected Condition 'draw', got %q", got.Condition)
	}
}

func TestAnalyzer_Analyze_WithEvents(t *testing.T) {
	a := NewAnalyzer()

	replay := &engine.Replay{
		MatchID: "eventful-match",
		Result: &engine.MatchResult{
			Winner:  0,
			Reason:  "dominance",
			Turns:   200,
			Scores:  []int{200, 50},
		},
		Players: []engine.ReplayPlayer{
			{ID: 0, Name: "Aggressor"},
			{ID: 1, Name: "Defender"},
		},
		Turns: []engine.ReplayTurn{
			{Turn: 0, Bots: []engine.ReplayBot{}, Events: nil},
			{Turn: 30, Bots: []engine.ReplayBot{
				{ID: 1, Owner: 0, Alive: true},
				{ID: 2, Owner: 1, Alive: true},
			}, Events: []engine.Event{
				{Type: "core_captured", Turn: 30, Details: map[string]interface{}{
					"attacker_id": float64(0),
					"victim_id":   float64(1),
				}},
			}},
			{Turn: 60, Bots: []engine.ReplayBot{
				{ID: 1, Owner: 0, Alive: true},
				{ID: 2, Owner: 0, Alive: true},
				{ID: 3, Owner: 0, Alive: true},
				{ID: 4, Owner: 0, Alive: true},
			}, Events: []engine.Event{
				{Type: "energy_collected", Turn: 60, Details: nil},
				{Type: "energy_collected", Turn: 60, Details: nil},
			}, Scores: []int{80, 30}},
		},
	}

	got := a.Analyze(replay)

	// Should have detected key moments
	if len(got.KeyMoments) == 0 {
		t.Error("expected some key moments from events")
	}
}

func TestDedupeMoments(t *testing.T) {
	moments := []string{
		"First moment",
		"Second moment",
		"First moment", // duplicate
		"Third moment",
		"Second moment", // duplicate
	}

	got := dedupeMoments(moments)

	if len(got) != 3 {
		t.Errorf("expected 3 unique moments, got %d", len(got))
	}
}

func TestDedupeMoments_Limit(t *testing.T) {
	moments := make([]string, 10)
	for i := range moments {
		moments[i] = "moment"
	}
	moments[0] = "unique1"
	moments[1] = "unique2"
	moments[2] = "unique3"
	moments[3] = "unique4"
	moments[4] = "unique5"
	moments[5] = "unique6"

	got := dedupeMoments(moments)

	if len(got) > 5 {
		t.Errorf("expected at most 5 moments, got %d", len(got))
	}
}

func TestDedupe(t *testing.T) {
	items := []string{"a", "b", "a", "c", "b", "d"}
	got := dedupe(items)

	if len(got) != 4 {
		t.Errorf("expected 4 unique items, got %d: %v", len(got), got)
	}

	// Check all expected items are present
	seen := make(map[string]bool)
	for _, item := range got {
		seen[item] = true
	}
	for _, expected := range []string{"a", "b", "c", "d"} {
		if !seen[expected] {
			t.Errorf("expected item %q in result", expected)
		}
	}
}
