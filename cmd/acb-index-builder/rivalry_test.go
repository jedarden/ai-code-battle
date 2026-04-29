package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComputeRivalries_BasicPair(t *testing.T) {
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	// Create 12 matches between bot1 and bot2 (above min threshold of 10)
	matches := make([]MatchData, 12)
	for i := 0; i < 6; i++ {
		matches[i] = MatchData{
			ID:        fmt.Sprintf("m_a_%d", i),
			WinnerID:  "bot1",
			PlayedAt:  now.Add(-time.Duration(12-i) * 24 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 5 - i%3, Won: true},
				{BotID: "bot2", Score: 2 + i%2, Won: false},
			},
		}
	}
	for i := 0; i < 6; i++ {
		matches[6+i] = MatchData{
			ID:        fmt.Sprintf("m_b_%d", i),
			WinnerID:  "bot2",
			PlayedAt:  now.Add(-time.Duration(6-i) * 24 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 2 + i%2, Won: false},
				{BotID: "bot2", Score: 5 - i%3, Won: true},
			},
		}
	}

	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "bot1", Name: "AlphaBot"},
			{ID: "bot2", Name: "BetaBot"},
		},
		Matches: matches,
	}

	botNameMap := map[string]string{"bot1": "AlphaBot", "bot2": "BetaBot"}
	rivalries := computeRivalries(data, botNameMap)

	if len(rivalries) != 1 {
		t.Fatalf("Expected 1 rivalry, got %d", len(rivalries))
	}

	r := rivalries[0]
	if r.BotA.ID != "bot1" || r.BotB.ID != "bot2" {
		t.Errorf("Bot IDs: got %q/%q, want bot1/bot2", r.BotA.ID, r.BotB.ID)
	}
	if r.TotalMatches != 12 {
		t.Errorf("TotalMatches: got %d, want 12", r.TotalMatches)
	}
	if r.Record.AWins != 6 || r.Record.BWins != 6 {
		t.Errorf("Record: got %d-%d, want 6-6", r.Record.AWins, r.Record.BWins)
	}
	if r.Score <= 0 {
		t.Errorf("Score should be positive, got %f", r.Score)
	}
	if r.Narrative == "" {
		t.Error("Narrative should not be empty")
	}
	if r.ClosestMatch == "" {
		t.Error("ClosestMatch should not be empty")
	}
	if len(r.RecentMatches) > 10 {
		t.Errorf("RecentMatches: got %d, want at most 10", len(r.RecentMatches))
	}
}

func TestComputeRivalries_BelowThreshold(t *testing.T) {
	now := time.Now()

	// Only 5 matches — below rivalryMinMatches (10)
	matches := make([]MatchData, 5)
	for i := 0; i < 5; i++ {
		matches[i] = MatchData{
			ID:        fmt.Sprintf("m_%d", i),
			WinnerID:  "bot1",
			PlayedAt:  now.Add(-time.Duration(i) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 3, Won: true},
				{BotID: "bot2", Score: 1, Won: false},
			},
		}
	}

	data := &IndexData{
		GeneratedAt: now,
		Matches:     matches,
	}

	rivalries := computeRivalries(data, nil)
	if len(rivalries) != 0 {
		t.Errorf("Expected 0 rivalries below threshold, got %d", len(rivalries))
	}
}

func TestComputeRivalries_ScoreRanking(t *testing.T) {
	now := time.Now()

	// Pair A: 20 matches, 10-10 split (perfect balance, high volume)
	var matchesA []MatchData
	for i := 0; i < 10; i++ {
		matchesA = append(matchesA, MatchData{
			ID: fmt.Sprintf("a_win_%d", i), WinnerID: "botA",
			PlayedAt: now.Add(-time.Duration(i) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "botA", Score: 3, Won: true},
				{BotID: "botB", Score: 1, Won: false},
			},
		})
		matchesA = append(matchesA, MatchData{
			ID: fmt.Sprintf("b_win_%d", i), WinnerID: "botB",
			PlayedAt: now.Add(-time.Duration(10+i) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "botA", Score: 1, Won: false},
				{BotID: "botB", Score: 3, Won: true},
			},
		})
	}

	// Pair C/D: 12 matches, 10-2 split (imbalanced, lower score)
	var matchesCD []MatchData
	for i := 0; i < 10; i++ {
		matchesCD = append(matchesCD, MatchData{
			ID: fmt.Sprintf("cd_a_%d", i), WinnerID: "botC",
			PlayedAt: now.Add(-time.Duration(i) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "botC", Score: 5, Won: true},
				{BotID: "botD", Score: 1, Won: false},
			},
		})
	}
	for i := 0; i < 2; i++ {
		matchesCD = append(matchesCD, MatchData{
			ID: fmt.Sprintf("cd_b_%d", i), WinnerID: "botD",
			PlayedAt: now.Add(-time.Duration(10+i) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "botC", Score: 1, Won: false},
				{BotID: "botD", Score: 3, Won: true},
			},
		})
	}

	allMatches := append(matchesA, matchesCD...)

	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "botA", Name: "Alpha"},
			{ID: "botB", Name: "Beta"},
			{ID: "botC", Name: "Charlie"},
			{ID: "botD", Name: "Delta"},
		},
		Matches: allMatches,
	}

	botNameMap := map[string]string{
		"botA": "Alpha", "botB": "Beta", "botC": "Charlie", "botD": "Delta",
	}
	rivalries := computeRivalries(data, botNameMap)

	if len(rivalries) != 2 {
		t.Fatalf("Expected 2 rivalries, got %d", len(rivalries))
	}

	// The balanced pair (A/B) should rank higher than the imbalanced pair (C/D)
	if rivalries[0].BotA.ID == "botC" || rivalries[0].BotB.ID == "botC" {
		// Check if the balanced pair is second (would be wrong)
		if rivalries[1].BotA.ID == "botA" || rivalries[1].BotB.ID == "botA" {
			t.Error("Balanced pair (A/B) should rank higher than imbalanced pair (C/D)")
		}
	}
}

func TestComputeRivalries_TopKLimit(t *testing.T) {
	now := time.Now()

	// Create 25 qualifying pairs (each with 10 matches)
	var matches []MatchData
	for pair := 0; pair < 25; pair++ {
		botA := fmt.Sprintf("bot_%02da", pair)
		botB := fmt.Sprintf("bot_%02db", pair)
		for i := 0; i < 10; i++ {
			winner := botA
			if i >= 5 {
				winner = botB
			}
			matches = append(matches, MatchData{
				ID:        fmt.Sprintf("pair%d_m%d", pair, i),
				WinnerID:  winner,
				PlayedAt:  now.Add(-time.Duration(i) * time.Hour),
				Participants: []ParticipantData{
					{BotID: botA, Score: 3, Won: winner == botA},
					{BotID: botB, Score: 2, Won: winner == botB},
				},
			})
		}
	}

	data := &IndexData{
		GeneratedAt: now,
		Matches:     matches,
	}

	rivalries := computeRivalries(data, nil)

	if len(rivalries) > rivalryTopK {
		t.Errorf("Expected at most %d rivalries, got %d", rivalryTopK, len(rivalries))
	}
	if len(rivalries) != rivalryTopK {
		t.Errorf("Expected exactly %d rivalries (25 pairs qualify), got %d", rivalryTopK, len(rivalries))
	}
}

func TestComputeRivalries_Draws(t *testing.T) {
	now := time.Now()

	// 10 matches with 4 draws
	matches := []MatchData{
		{ID: "m1", WinnerID: "bot1", PlayedAt: now.Add(-10 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 3, Won: true}, {BotID: "bot2", Score: 1, Won: false}}},
		{ID: "m2", WinnerID: "bot2", PlayedAt: now.Add(-9 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 1, Won: false}, {BotID: "bot2", Score: 3, Won: true}}},
		{ID: "m3", PlayedAt: now.Add(-8 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 2}, {BotID: "bot2", Score: 2}}}, // draw
		{ID: "m4", WinnerID: "bot1", PlayedAt: now.Add(-7 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 4, Won: true}, {BotID: "bot2", Score: 2, Won: false}}},
		{ID: "m5", PlayedAt: now.Add(-6 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 2}, {BotID: "bot2", Score: 2}}}, // draw
		{ID: "m6", WinnerID: "bot2", PlayedAt: now.Add(-5 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 1, Won: false}, {BotID: "bot2", Score: 3, Won: true}}},
		{ID: "m7", WinnerID: "bot1", PlayedAt: now.Add(-4 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 3, Won: true}, {BotID: "bot2", Score: 2, Won: false}}},
		{ID: "m8", PlayedAt: now.Add(-3 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 3}, {BotID: "bot2", Score: 3}}}, // draw
		{ID: "m9", WinnerID: "bot2", PlayedAt: now.Add(-2 * time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 2, Won: false}, {BotID: "bot2", Score: 4, Won: true}}},
		{ID: "m10", PlayedAt: now.Add(-time.Hour), Participants: []ParticipantData{{BotID: "bot1", Score: 1}, {BotID: "bot2", Score: 1}}}, // draw
	}

	data := &IndexData{
		GeneratedAt: now,
		Matches:     matches,
	}

	rivalries := computeRivalries(data, map[string]string{"bot1": "A", "bot2": "B"})

	if len(rivalries) != 1 {
		t.Fatalf("Expected 1 rivalry, got %d", len(rivalries))
	}

	r := rivalries[0]
	if r.Record.Draws != 4 {
		t.Errorf("Draws: got %d, want 4", r.Record.Draws)
	}
	if r.Record.AWins+r.Record.BWins+r.Record.Draws != 10 {
		t.Errorf("Total: got %d, want 10", r.Record.AWins+r.Record.BWins+r.Record.Draws)
	}
}

func TestComputeRivalries_MultiPlayerSkipped(t *testing.T) {
	now := time.Now()

	// 10 two-player matches + 5 three-player matches (should be ignored)
	var matches []MatchData
	for i := 0; i < 10; i++ {
		matches = append(matches, MatchData{
			ID:        fmt.Sprintf("2p_%d", i),
			WinnerID:  "bot1",
			PlayedAt:  now.Add(-time.Duration(i) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 3, Won: true},
				{BotID: "bot2", Score: 1, Won: false},
			},
		})
	}
	for i := 0; i < 5; i++ {
		matches = append(matches, MatchData{
			ID:        fmt.Sprintf("3p_%d", i),
			WinnerID:  "bot1",
			PlayedAt:  now.Add(-time.Duration(i) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 3, Won: true},
				{BotID: "bot2", Score: 1, Won: false},
				{BotID: "bot3", Score: 0, Won: false},
			},
		})
	}

	data := &IndexData{
		GeneratedAt: now,
		Matches:     matches,
	}

	rivalries := computeRivalries(data, nil)

	if len(rivalries) != 1 {
		t.Fatalf("Expected 1 rivalry (only 2-player matches), got %d", len(rivalries))
	}
	// Should only count 10 two-player matches, not 15 total
	if rivalries[0].TotalMatches != 10 {
		t.Errorf("TotalMatches: got %d, want 10", rivalries[0].TotalMatches)
	}
}

func TestComputeRivalries_RecencyBoost(t *testing.T) {
	now := time.Now()

	// Pair 1: all matches in the last week (high recency), balanced 5-5 split
	var recentMatches []MatchData
	for i := 0; i < 5; i++ {
		recentMatches = append(recentMatches, MatchData{
			ID:       fmt.Sprintf("recent_a_%d", i),
			WinnerID: "bot1",
			PlayedAt: now.Add(-time.Duration(i*12) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 3, Won: true},
				{BotID: "bot2", Score: 2, Won: false},
			},
		})
		recentMatches = append(recentMatches, MatchData{
			ID:       fmt.Sprintf("recent_b_%d", i),
			WinnerID: "bot2",
			PlayedAt: now.Add(-time.Duration(i*12+6) * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 2, Won: false},
				{BotID: "bot2", Score: 3, Won: true},
			},
		})
	}

	// Pair 2: all matches 6 months ago (low recency), balanced 5-5 split
	var oldMatches []MatchData
	for i := 0; i < 5; i++ {
		oldMatches = append(oldMatches, MatchData{
			ID:       fmt.Sprintf("old_a_%d", i),
			WinnerID: "bot3",
			PlayedAt: now.Add(-180*24*time.Hour - time.Duration(i)*time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot3", Score: 3, Won: true},
				{BotID: "bot4", Score: 2, Won: false},
			},
		})
		oldMatches = append(oldMatches, MatchData{
			ID:       fmt.Sprintf("old_b_%d", i),
			WinnerID: "bot4",
			PlayedAt: now.Add(-180*24*time.Hour - time.Duration(i+5)*time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot3", Score: 2, Won: false},
				{BotID: "bot4", Score: 3, Won: true},
			},
		})
	}

	allMatches := append(recentMatches, oldMatches...)

	data := &IndexData{
		GeneratedAt: now,
		Matches:     allMatches,
	}

	rivalries := computeRivalries(data, nil)

	if len(rivalries) != 2 {
		t.Fatalf("Expected 2 rivalries, got %d", len(rivalries))
	}

	// The recent pair should rank higher than the old pair
	if rivalries[0].BotA.ID != "bot1" && rivalries[0].BotB.ID != "bot1" {
		t.Error("Recent pair should rank higher than old pair due to recency weighting")
	}
}

func TestLongestStreak(t *testing.T) {
	tests := []struct {
		name     string
		winners  []string
		botA     string
		botB     string
		wantLen  int
		wantNil  bool
	}{
		{"empty", []string{}, "a", "b", 0, true},
		{"single", []string{"a"}, "a", "b", 0, true}, // < 2
		{"two streak", []string{"a", "a"}, "a", "b", 2, false},
		{"alternating", []string{"a", "b", "a", "b"}, "a", "b", 0, true},
		{"long streak", []string{"a", "a", "a", "b", "a", "a", "a", "a"}, "a", "b", 4, false},
		{"draws break streak", []string{"a", "a", "draw", "a", "a"}, "a", "b", 2, false},
		{"all draws", []string{"draw", "draw"}, "a", "b", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := longestStreak(tt.winners, tt.botA, tt.botB)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
			} else {
				if result == nil {
					t.Fatal("expected non-nil streak")
				}
				if result.Length != tt.wantLen {
					t.Errorf("streak length: got %d, want %d", result.Length, tt.wantLen)
				}
			}
		})
	}
}

func TestBuildRivalryNarrative(t *testing.T) {
	tests := []struct {
		name      string
		aWins     int
		bWins     int
		draws     int
		streak    *RivalryStreak
		wantEmpty bool
	}{
		{"tied", 5, 5, 0, nil, false},
		{"tied with draws", 5, 5, 2, nil, false},
		{"dominant", 8, 2, 0, nil, false},
		{"with streak", 7, 3, 0, &RivalryStreak{Holder: "AlphaBot", Length: 4}, false},
		{"close", 6, 4, 0, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			narrative := buildRivalryNarrative("AlphaBot", "BetaBot", "alpha-id", "beta-id", tt.aWins+tt.bWins+tt.draws, tt.aWins, tt.bWins, tt.draws, tt.streak)
			if tt.wantEmpty && narrative != "" {
				t.Errorf("expected empty narrative, got %q", narrative)
			}
			if !tt.wantEmpty && narrative == "" {
				t.Error("expected non-empty narrative")
			}
		})
	}
}

func TestGenerateRivalriesIndex(t *testing.T) {
	tmpDir := t.TempDir()

	rivalries := []RivalryEntry{
		{
			BotA:         RivalryBot{ID: "bot1", Name: "Alpha"},
			BotB:         RivalryBot{ID: "bot2", Name: "Beta"},
			TotalMatches: 15,
			Record:       RivalryRecord{AWins: 8, BWins: 7, Draws: 0},
			Score:        2.34,
			Narrative:    "Alpha and Beta have met 15 times.",
		},
	}

	if err := generateRivalriesIndex(rivalries, tmpDir); err != nil {
		t.Fatalf("generateRivalriesIndex failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "data", "meta", "rivalries.json"))
	if err != nil {
		t.Fatalf("Failed to read rivalries.json: %v", err)
	}

	var index RivalriesIndex
	if err := json.Unmarshal(content, &index); err != nil {
		t.Fatalf("Failed to parse rivalries.json: %v", err)
	}

	if len(index.Rivalries) != 1 {
		t.Errorf("Expected 1 rivalry, got %d", len(index.Rivalries))
	}
	if index.UpdatedAt == "" {
		t.Error("UpdatedAt should not be empty")
	}
	if index.Rivalries[0].BotA.Name != "Alpha" {
		t.Errorf("BotA name: got %q, want %q", index.Rivalries[0].BotA.Name, "Alpha")
	}
}

func TestGenerateRivalriesIndex_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	if err := generateRivalriesIndex(nil, tmpDir); err != nil {
		t.Fatalf("generateRivalriesIndex with nil failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "data", "meta", "rivalries.json"))
	if err != nil {
		t.Fatalf("Failed to read rivalries.json: %v", err)
	}

	var index RivalriesIndex
	if err := json.Unmarshal(content, &index); err != nil {
		t.Fatalf("Failed to parse rivalries.json: %v", err)
	}

	if len(index.Rivalries) != 0 {
		t.Errorf("Expected 0 rivalries, got %d", len(index.Rivalries))
	}
}

func TestPairKey(t *testing.T) {
	if pairKey("a", "b") != "a:b" {
		t.Errorf("pairKey(a,b): got %q", pairKey("a", "b"))
	}
	if pairKey("b", "a") != "a:b" {
		t.Errorf("pairKey(b,a): got %q", pairKey("b", "a"))
	}
	if pairKey("b", "a") != pairKey("a", "b") {
		t.Error("pairKey should be canonical")
	}
}
