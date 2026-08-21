package selector

import (
	"context"
	"testing"
	"time"

	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/db"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MinTurnCount != 100 {
		t.Errorf("Expected MinTurnCount 100, got %d", cfg.MinTurnCount)
	}
	if cfg.MinCrossings != 3 {
		t.Errorf("Expected MinCrossings 3, got %d", cfg.MinCrossings)
	}
	if cfg.UpsetThreshold != 150.0 {
		t.Errorf("Expected UpsetThreshold 150.0, got %f", cfg.UpsetThreshold)
	}
	if cfg.MaxPerHour != 20 {
		t.Errorf("Expected MaxPerHour 20, got %d", cfg.MaxPerHour)
	}
}

func TestNewSelector_Defaults(t *testing.T) {
	mockStore := &mockStore{}
	sel := NewSelector(mockStore, Config{})

	if sel.minTurnCount != 100 {
		t.Errorf("Expected minTurnCount 100, got %d", sel.minTurnCount)
	}
	if sel.minCrossings != 3 {
		t.Errorf("Expected minCrossings 3, got %d", sel.minCrossings)
	}
	if sel.upsetThreshold != 150.0 {
		t.Errorf("Expected upsetThreshold 150.0, got %f", sel.upsetThreshold)
	}
	if sel.maxPerHour != 20 {
		t.Errorf("Expected maxPerHour 20, got %d", sel.maxPerHour)
	}
}

func TestNewSelector_CustomConfig(t *testing.T) {
	mockStore := &mockStore{}
	cfg := Config{
		MinTurnCount:   200,
		MinCrossings:   5,
		UpsetThreshold: 200.0,
		MaxPerHour:     50,
	}

	sel := NewSelector(mockStore, cfg)

	if sel.minTurnCount != 200 {
		t.Errorf("Expected minTurnCount 200, got %d", sel.minTurnCount)
	}
	if sel.minCrossings != 5 {
		t.Errorf("Expected minCrossings 5, got %d", sel.minCrossings)
	}
	if sel.upsetThreshold != 200.0 {
		t.Errorf("Expected upsetThreshold 200.0, got %f", sel.upsetThreshold)
	}
	if sel.maxPerHour != 50 {
		t.Errorf("Expected maxPerHour 50, got %d", sel.maxPerHour)
	}
}

func TestPriorityScore(t *testing.T) {
	mockStore := &mockStore{}
	sel := NewSelector(mockStore, Config{})

	tests := []struct {
		name  string
		match db.CandidateMatch
		want  float64
	}{
		{
			name: "upset with many crossings",
			match: db.CandidateMatch{
				MatchID:          "match-1",
				WinProbCrossings: 5,
				IsUpset:          true,
				IsCloseFinish:    false,
				TurnCount:        250,
			},
			want: 1000 + 250 + 20, // 1000 (upset) + 5*50 (crossings) + 20 (short match)
		},
		{
			name: "close finish",
			match: db.CandidateMatch{
				MatchID:          "match-2",
				WinProbCrossings: 3,
				IsUpset:          false,
				IsCloseFinish:    true,
				TurnCount:        200,
			},
			want: 500 + 150 + 20, // 500 (close finish) + 3*50 (crossings) + 20 (short match)
		},
		{
			name: "upset and close finish",
			match: db.CandidateMatch{
				MatchID:          "match-3",
				WinProbCrossings: 4,
				IsUpset:          true,
				IsCloseFinish:    true,
				TurnCount:        150,
			},
			want: 1000 + 500 + 200 + 20, // All bonuses
		},
		{
			name: "long match with crossings",
			match: db.CandidateMatch{
				MatchID:          "match-4",
				WinProbCrossings: 6,
				IsUpset:          false,
				IsCloseFinish:    false,
				TurnCount:        400,
			},
			want: 300, // 6*50 (crossings only, no short match bonus)
		},
		{
			name: "minimal interesting match",
			match: db.CandidateMatch{
				MatchID:          "match-5",
				WinProbCrossings: 1,
				IsUpset:          false,
				IsCloseFinish:    false,
				TurnCount:        100,
			},
			want: 70, // 1*50 + 20 (short match)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sel.priorityScore(tt.match)
			if got != tt.want {
				t.Errorf("priorityScore() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestSortByPriority(t *testing.T) {
	mockStore := &mockStore{}
	sel := NewSelector(mockStore, Config{})

	matches := []db.CandidateMatch{
		{MatchID: "low", WinProbCrossings: 1, IsUpset: false, IsCloseFinish: false, TurnCount: 400},
		{MatchID: "high-upset", WinProbCrossings: 2, IsUpset: true, IsCloseFinish: false, TurnCount: 200},
		{MatchID: "high-close", WinProbCrossings: 2, IsUpset: false, IsCloseFinish: true, TurnCount: 200},
		{MatchID: "both", WinProbCrossings: 1, IsUpset: true, IsCloseFinish: true, TurnCount: 150},
		{MatchID: "medium", WinProbCrossings: 5, IsUpset: false, IsCloseFinish: false, TurnCount: 300},
	}

	sel.sortByPriority(matches)

	// Expected order: both > high-upset > high-close > medium > low
	expected := []string{"both", "high-upset", "high-close", "medium", "low"}
	for i, wantID := range expected {
		if matches[i].MatchID != wantID {
			t.Errorf("Position %d: expected %s, got %s", i, wantID, matches[i].MatchID)
		}
	}
}

func TestSelect_RateLimit(t *testing.T) {
	tests := []struct {
		name           string
		maxPerHour     int
		recentCount    int
		candidateCount int
		wantSelected   int
		wantSkipped    int
	}{
		{
			name:           "under rate limit",
			maxPerHour:     20,
			recentCount:    5,
			candidateCount: 10,
			wantSelected:   10,
			wantSkipped:    0,
		},
		{
			name:           "at rate limit",
			maxPerHour:     20,
			recentCount:    20,
			candidateCount: 10,
			wantSelected:   0,
			wantSkipped:    0,
		},
		{
			name:           "over rate limit",
			maxPerHour:     20,
			recentCount:    25,
			candidateCount: 10,
			wantSelected:   0,
			wantSkipped:    0,
		},
		{
			name:           "partial rate limit",
			maxPerHour:     20,
			recentCount:    15,
			candidateCount: 10,
			wantSelected:   5,  // 20 - 15 = 5 remaining
			wantSkipped:    5,  // 10 candidates - 5 selected = 5 skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStore{
				enrichmentCount: tt.recentCount,
				candidates:      makeCandidates(tt.candidateCount),
			}

			cfg := Config{
				MaxPerHour: tt.maxPerHour,
			}
			sel := NewSelector(mockStore, cfg)

			result, err := sel.Select(context.Background())
			if err != nil {
				t.Fatalf("Select() error = %v", err)
			}

			if len(result.Matches) != tt.wantSelected {
				t.Errorf("Select() selected %d matches, want %d", len(result.Matches), tt.wantSelected)
			}
			if result.Skipped != tt.wantSkipped {
				t.Errorf("Select() skipped %d matches, want %d", result.Skipped, tt.wantSkipped)
			}
		})
	}
}

func TestSelect_NoCandidates(t *testing.T) {
	mockStore := &mockStore{
		enrichmentCount: 5,
		candidates:      []db.CandidateMatch{},
	}

	cfg := Config{MaxPerHour: 20}
	sel := NewSelector(mockStore, cfg)

	result, err := sel.Select(context.Background())
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if len(result.Matches) != 0 {
		t.Errorf("Select() selected %d matches, want 0", len(result.Matches))
	}
	if result.Skipped != 0 {
		t.Errorf("Select() skipped %d matches, want 0", result.Skipped)
	}
}

func TestSelect_Error(t *testing.T) {
	mockStore := &mockStore{
		queryError: true,
	}

	cfg := Config{MaxPerHour: 20}
	sel := NewSelector(mockStore, cfg)

	_, err := sel.Select(context.Background())
	if err == nil {
		t.Error("Select() expected error, got nil")
	}
}

func TestShuffle(t *testing.T) {
	matches := makeCandidates(10)

	// Track original order
	originalIDs := make([]string, len(matches))
	for i, m := range matches {
		originalIDs[i] = m.MatchID
	}

	// Shuffle
	Shuffle(matches)

	// Check that order changed (highly unlikely to be the same)
	sameOrder := true
	for i, m := range matches {
		if m.MatchID != originalIDs[i] {
			sameOrder = false
			break
		}
	}

	if sameOrder {
		t.Error("Shuffle() did not change order")
	}

	// Check that all elements are still present
	originalMap := make(map[string]bool)
	for _, id := range originalIDs {
		originalMap[id] = true
	}

	for _, m := range matches {
		if !originalMap[m.MatchID] {
			t.Errorf("Shuffle() lost match %s", m.MatchID)
		}
	}
}

// Mock store for testing

type mockStore struct {
	enrichmentCount int
	candidates     []db.CandidateMatch
	queryError     bool
}

func (m *mockStore) GetEnrichmentCount(ctx context.Context, since time.Time) (int, error) {
	if m.queryError {
		return 0, context.DeadlineExceeded
	}
	return m.enrichmentCount, nil
}

func (m *mockStore) FindCandidates(ctx context.Context, minTurns, minCrossings int, upsetThreshold float64) ([]db.CandidateMatch, error) {
	if m.queryError {
		return nil, context.DeadlineExceeded
	}
	return m.candidates, nil
}

// Helper to create test candidates

func makeCandidates(count int) []db.CandidateMatch {
	candidates := make([]db.CandidateMatch, count)
	for i := 0; i < count; i++ {
		candidates[i] = db.CandidateMatch{
			MatchID:     generateMatchID(i),
			TurnCount:   150 + i*10,
			Winner:      i % 2,
			Condition:    "elimination",
			FinalScores:  []int{100 + i, 95 - i},
			WinProbCrossings: 3,
			IsUpset:      i%3 == 0,
			IsCloseFinish: i%2 == 0,
			Players: []db.PlayerData{
				{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500},
				{ID: 1, BotID: "bot-2", Name: "Bot2", Rating: 1400},
			},
		}
	}
	return candidates
}

func generateMatchID(i int) string {
	return "match-" + string(rune('a'+i%26))
}
