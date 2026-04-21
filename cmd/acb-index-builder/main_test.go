package main

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func generateTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		}
	}
	return img
}

func TestLoadConfig(t *testing.T) {
	// Set test environment variables
	t.Setenv("ACB_POSTGRES_HOST", "testhost")
	t.Setenv("ACB_POSTGRES_PORT", "5433")
	t.Setenv("ACB_POSTGRES_DATABASE", "testdb")
	t.Setenv("ACB_POSTGRES_USER", "testuser")
	t.Setenv("ACB_POSTGRES_PASSWORD", "testpass")
	t.Setenv("ACB_BUILD_INTERVAL", "30s")
	t.Setenv("ACB_DEPLOY_INTERVAL", "3")
	t.Setenv("ACB_MAX_LIFETIME", "2h")
	t.Setenv("ACB_BUILD_TIMEOUT", "5m")
	t.Setenv("ACB_OUTPUT_DIR", "/tmp/test-output")

	cfg := LoadConfig()

	if cfg.PostgresHost != "testhost" {
		t.Errorf("PostgresHost: got %q, want %q", cfg.PostgresHost, "testhost")
	}
	if cfg.PostgresPort != 5433 {
		t.Errorf("PostgresPort: got %d, want %d", cfg.PostgresPort, 5433)
	}
	if cfg.BuildInterval != 30*time.Second {
		t.Errorf("BuildInterval: got %v, want %v", cfg.BuildInterval, 30*time.Second)
	}
	if cfg.DeployInterval != 3 {
		t.Errorf("DeployInterval: got %d, want %d", cfg.DeployInterval, 3)
	}
	if cfg.MaxLifetime != 2*time.Hour {
		t.Errorf("MaxLifetime: got %v, want %v", cfg.MaxLifetime, 2*time.Hour)
	}
	if cfg.BuildTimeout != 5*time.Minute {
		t.Errorf("BuildTimeout: got %v, want %v", cfg.BuildTimeout, 5*time.Minute)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Clear all env vars
	os.Clearenv()

	cfg := LoadConfig()

	// Check defaults
	if cfg.PostgresHost != "cnpg-apexalgo-rw.cnpg.svc.cluster.local" {
		t.Errorf("PostgresHost default: got %q", cfg.PostgresHost)
	}
	if cfg.PostgresPort != 5432 {
		t.Errorf("PostgresPort default: got %d", cfg.PostgresPort)
	}
	if cfg.BuildInterval != 15*time.Minute {
		t.Errorf("BuildInterval default: got %v", cfg.BuildInterval)
	}
	if cfg.DeployInterval != 6 {
		t.Errorf("DeployInterval default: got %d", cfg.DeployInterval)
	}
	if cfg.MaxLifetime != 4*time.Hour {
		t.Errorf("MaxLifetime default: got %v", cfg.MaxLifetime)
	}
	if cfg.BuildTimeout != 10*time.Minute {
		t.Errorf("BuildTimeout default: got %v", cfg.BuildTimeout)
	}
}

func TestGenerateLeaderboard(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{
				ID:               "bot1",
				Name:             "TestBot1",
				OwnerID:          "owner1",
				Rating:           1650.0,
				RatingDeviation:  50.0,
				MatchesPlayed:    100,
				MatchesWon:       75,
				HealthStatus:     "ACTIVE",
				Evolved:          false,
				CreatedAt:        time.Now(),
			},
			{
				ID:               "bot2",
				Name:             "TestBot2",
				OwnerID:          "owner2",
				Rating:           1550.0,
				RatingDeviation:  75.0,
				MatchesPlayed:    50,
				MatchesWon:       25,
				HealthStatus:     "ACTIVE",
				Evolved:          true,
				Island:           "python",
				Generation:       5,
				CreatedAt:        time.Now(),
			},
		},
		Matches: []MatchData{},
	}

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	if err := generateLeaderboard(data, tmpDir); err != nil {
		t.Fatalf("generateLeaderboard failed: %v", err)
	}

	// Read and verify the generated file
	content, err := os.ReadFile(filepath.Join(tmpDir, "data", "leaderboard.json"))
	if err != nil {
		t.Fatalf("Failed to read leaderboard.json: %v", err)
	}

	var leaderboard struct {
		Entries []LeaderboardEntry `json:"entries"`
	}
	if err := json.Unmarshal(content, &leaderboard); err != nil {
		t.Fatalf("Failed to parse leaderboard.json: %v", err)
	}

	if len(leaderboard.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(leaderboard.Entries))
	}

	// First entry should be highest rated
	if leaderboard.Entries[0].BotID != "bot1" {
		t.Errorf("First entry bot_id: got %q, want %q", leaderboard.Entries[0].BotID, "bot1")
	}
	if leaderboard.Entries[0].Rating != 1650 {
		t.Errorf("First entry rating: got %d, want %d", leaderboard.Entries[0].Rating, 1650)
	}
}

func TestGenerateBotDirectory(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{ID: "bot1", Name: "Bot1", Rating: 1500, MatchesPlayed: 10, MatchesWon: 5},
			{ID: "bot2", Name: "Bot2", Rating: 1600, MatchesPlayed: 20, MatchesWon: 10},
		},
	}

	tmpDir := t.TempDir()
	botsDir := filepath.Join(tmpDir, "data", "bots")
	if err := os.MkdirAll(botsDir, 0755); err != nil {
		t.Fatalf("Failed to create bots dir: %v", err)
	}

	if err := generateBotDirectory(data, tmpDir); err != nil {
		t.Fatalf("generateBotDirectory failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(botsDir, "index.json"))
	if err != nil {
		t.Fatalf("Failed to read bots/index.json: %v", err)
	}

	var dir BotDirectory
	if err := json.Unmarshal(content, &dir); err != nil {
		t.Fatalf("Failed to parse bots/index.json: %v", err)
	}

	if len(dir.Bots) != 2 {
		t.Errorf("Expected 2 bots, got %d", len(dir.Bots))
	}
}

func TestGenerateMatchIndex(t *testing.T) {
	now := time.Now()
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "bot1", Name: "Bot1"},
			{ID: "bot2", Name: "Bot2"},
		},
		Matches: []MatchData{
			{
				ID:           "match1",
				WinnerID:     "bot1",
				TurnCount:    200,
				EndCondition: "elimination",
				CompletedAt:  now,
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 5, Won: true},
					{BotID: "bot2", Score: 2, Won: false},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	matchesDir := filepath.Join(tmpDir, "data", "matches")
	if err := os.MkdirAll(matchesDir, 0755); err != nil {
		t.Fatalf("Failed to create matches dir: %v", err)
	}

	botNameMap := map[string]string{"bot1": "Bot1", "bot2": "Bot2"}
	if err := generateMatchIndex(data, tmpDir, botNameMap); err != nil {
		t.Fatalf("generateMatchIndex failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(matchesDir, "index.json"))
	if err != nil {
		t.Fatalf("Failed to read matches/index.json: %v", err)
	}

	var index MatchIndex
	if err := json.Unmarshal(content, &index); err != nil {
		t.Fatalf("Failed to parse matches/index.json: %v", err)
	}

	if len(index.Matches) != 1 {
		t.Errorf("Expected 1 match, got %d", len(index.Matches))
	}
	if index.Matches[0].ID != "match1" {
		t.Errorf("Match ID: got %q, want %q", index.Matches[0].ID, "match1")
	}
	if index.Matches[0].Turns != 200 {
		t.Errorf("Match turns: got %d, want %d", index.Matches[0].Turns, 200)
	}
}

func TestGeneratePlaylists(t *testing.T) {
	now := time.Now()
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "bot1", Name: "Bot1"},
			{ID: "bot2", Name: "Bot2"},
		},
		Matches: []MatchData{
			{
				ID:           "match1",
				WinnerID:     "bot1",
				TurnCount:    200,
				EndCondition: "elimination",
				CompletedAt:  now,
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 3, Won: true},
					{BotID: "bot2", Score: 2, Won: false},
				},
			},
			{
				ID:           "match2",
				WinnerID:     "bot2",
				TurnCount:    350,
				EndCondition: "dominance",
				CompletedAt:  now.Add(-time.Hour),
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 0, Won: false, PreMatchRating: 1800},
					{BotID: "bot2", Score: 10, Won: true, PreMatchRating: 1500},
				},
			},
			{
				ID:           "match3",
				WinnerID:     "bot1",
				TurnCount:    400,
				EndCondition: "turn_limit",
				CompletedAt:  now.Add(-2 * time.Hour),
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 5, Won: true, PreMatchRating: 1700},
					{BotID: "bot2", Score: 3, Won: false, PreMatchRating: 1600},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	playlistsDir := filepath.Join(tmpDir, "data", "playlists")
	if err := os.MkdirAll(playlistsDir, 0755); err != nil {
		t.Fatalf("Failed to create playlists dir: %v", err)
	}

	botNameMap := map[string]string{"bot1": "Bot1", "bot2": "Bot2"}
	if err := generatePlaylists(data, tmpDir, botNameMap); err != nil {
		t.Fatalf("generatePlaylists failed: %v", err)
	}

	// Verify index.json was generated
	indexContent, err := os.ReadFile(filepath.Join(playlistsDir, "index.json"))
	if err != nil {
		t.Fatalf("Failed to read playlists/index.json: %v", err)
	}
	var index PlaylistIndex
	if err := json.Unmarshal(indexContent, &index); err != nil {
		t.Fatalf("Failed to parse playlists/index.json: %v", err)
	}
	if len(index.Playlists) == 0 {
		t.Error("Expected playlists in index.json, got 0")
	}

	// Verify each playlist has required fields
	for _, p := range index.Playlists {
		if p.Slug == "" {
			t.Error("Playlist summary missing slug")
		}
		if p.Title == "" {
			t.Error("Playlist summary missing title")
		}
		if p.Category == "" {
			t.Error("Playlist summary missing category")
		}
	}

	// Verify closest-finishes playlist content
	content, err := os.ReadFile(filepath.Join(playlistsDir, "closest-finishes.json"))
	if err != nil {
		t.Fatalf("Failed to read closest-finishes.json: %v", err)
	}
	var playlist Playlist
	if err := json.Unmarshal(content, &playlist); err != nil {
		t.Fatalf("Failed to parse closest-finishes.json: %v", err)
	}
	if playlist.Category != "close_games" {
		t.Errorf("closest-finishes category: got %q, want %q", playlist.Category, "close_games")
	}
	if len(playlist.Matches) != 2 {
		t.Errorf("closest-finishes: expected 2 matches, got %d", len(playlist.Matches))
	}
	if len(playlist.Matches) > 0 && playlist.Matches[0].MatchID != "match1" {
		t.Errorf("closest-finishes first (closest): got %q, want %q", playlist.Matches[0].MatchID, "match1")
	}

	// Verify marathon-matches playlist
	marathonContent, err := os.ReadFile(filepath.Join(playlistsDir, "marathon-matches.json"))
	if err != nil {
		t.Fatalf("Failed to read marathon-matches.json: %v", err)
	}
	var marathon Playlist
	if err := json.Unmarshal(marathonContent, &marathon); err != nil {
		t.Fatalf("Failed to parse marathon-matches.json: %v", err)
	}
	if marathon.Category != "long_games" {
		t.Errorf("marathon-matches category: got %q, want %q", marathon.Category, "long_games")
	}
	// Should include match2 (350) and match3 (400), sorted by turn count desc
	if len(marathon.Matches) != 2 {
		t.Errorf("marathon-matches: expected 2 matches, got %d", len(marathon.Matches))
	}
	if len(marathon.Matches) > 0 && marathon.Matches[0].MatchID != "match3" {
		t.Errorf("marathon-matches first: got %q, want %q", marathon.Matches[0].MatchID, "match3")
	}

	// Verify biggest-upsets playlist
	upsetContent, err := os.ReadFile(filepath.Join(playlistsDir, "biggest-upsets.json"))
	if err != nil {
		t.Fatalf("Failed to read biggest-upsets.json: %v", err)
	}
	var upsets Playlist
	if err := json.Unmarshal(upsetContent, &upsets); err != nil {
		t.Fatalf("Failed to parse biggest-upsets.json: %v", err)
	}
	// match2 has winner rating 1500 vs loser 1800 → upset of 300
	if len(upsets.Matches) != 1 {
		t.Errorf("biggest-upsets: expected 1 match, got %d", len(upsets.Matches))
	}
}

func TestInterestScore(t *testing.T) {
	now := time.Now()
	// Close finish + upset + long game → high score
	m := MatchData{
		WinnerID:   "bot2",
		TurnCount:  450,
		CompletedAt: now,
		Participants: []ParticipantData{
			{BotID: "bot1", Score: 3, Won: false, PreMatchRating: 1800},
			{BotID: "bot2", Score: 2, Won: true, PreMatchRating: 1400},
		},
	}
	score := interestScore(m)
	if score < 5.0 {
		t.Errorf("interestScore for exciting match: got %f, want >= 5.0", score)
	}

	// Boring match → low score
	m2 := MatchData{
		WinnerID:   "bot1",
		TurnCount:  100,
		CompletedAt: now,
		Participants: []ParticipantData{
			{BotID: "bot1", Score: 10, Won: true, PreMatchRating: 1500},
			{BotID: "bot2", Score: 0, Won: false, PreMatchRating: 1500},
		},
	}
	score2 := interestScore(m2)
	if score2 >= 2.0 {
		t.Errorf("interestScore for boring match: got %f, want < 2.0", score2)
	}
}

func TestFormatMatchTitle(t *testing.T) {
	data := &IndexData{
		Bots: []BotData{
			{ID: "bot1", Name: "SwarmBot"},
			{ID: "bot2", Name: "HunterBot"},
		},
	}
	m := MatchData{
		ID: "match1",
		Participants: []ParticipantData{
			{BotID: "bot1", Score: 3},
			{BotID: "bot2", Score: 2},
		},
	}
	title := formatMatchTitle(m, data)
	if title != "SwarmBot 3 – 2 HunterBot" {
		t.Errorf("formatMatchTitle: got %q, want %q", title, "SwarmBot 3 – 2 HunterBot")
	}
}

func TestIsComeback(t *testing.T) {
	// Close upset (rating diff >= 80, score diff <= 3) = comeback
	m := MatchData{
		WinnerID:  "bot2",
		TurnCount: 300,
		Participants: []ParticipantData{
			{BotID: "bot1", Score: 3, Won: false, PreMatchRating: 1800},
			{BotID: "bot2", Score: 2, Won: true, PreMatchRating: 1600},
		},
	}
	if !isComeback(m) {
		t.Error("Expected close upset to be a comeback")
	}

	// Decisive win, no upset = not a comeback
	m2 := MatchData{
		WinnerID:  "bot1",
		TurnCount: 150,
		Participants: []ParticipantData{
			{BotID: "bot1", Score: 8, Won: true, PreMatchRating: 1700},
			{BotID: "bot2", Score: 1, Won: false, PreMatchRating: 1500},
		},
	}
	if isComeback(m2) {
		t.Error("Expected decisive non-upset to not be a comeback")
	}

	// No winner = not a comeback
	m3 := MatchData{
		WinnerID: "",
		Participants: []ParticipantData{
			{BotID: "bot1", Score: 3},
			{BotID: "bot2", Score: 2},
		},
	}
	if isComeback(m3) {
		t.Error("Expected no-winner match to not be a comeback")
	}
}

func TestTurnaroundMagnitude(t *testing.T) {
	// Bigger upset + closer score = bigger turnaround
	big := MatchData{
		WinnerID:  "underdog",
		TurnCount: 400,
		Participants: []ParticipantData{
			{BotID: "favored", Score: 2, Won: false, PreMatchRating: 1900},
			{BotID: "underdog", Score: 3, Won: true, PreMatchRating: 1500},
		},
	}
	small := MatchData{
		WinnerID:  "slight_underdog",
		TurnCount: 200,
		Participants: []ParticipantData{
			{BotID: "favored", Score: 1, Won: false, PreMatchRating: 1600},
			{BotID: "slight_underdog", Score: 3, Won: true, PreMatchRating: 1500},
		},
	}
	bigMag := turnaroundMagnitude(big)
	smallMag := turnaroundMagnitude(small)
	if bigMag <= smallMag {
		t.Errorf("Expected bigger turnaround (%f) > smaller (%f)", bigMag, smallMag)
	}
}

func TestIsEvolutionBreakthrough(t *testing.T) {
	data := &IndexData{
		Bots: []BotData{
			{ID: "evo1", Name: "EvolvedBot", Evolved: true},
			{ID: "human1", Name: "HumanBot", Evolved: false},
		},
	}

	// Evolved bot beats high-rated opponent
	m := MatchData{
		WinnerID: "evo1",
		Participants: []ParticipantData{
			{BotID: "evo1", Score: 4, Won: true, PreMatchRating: 1400},
			{BotID: "human1", Score: 2, Won: false, PreMatchRating: 1650},
		},
	}
	if !isEvolutionBreakthrough(m, data) {
		t.Error("Expected evolved bot beating rated opponent to be a breakthrough")
	}

	// Human bot wins = not a breakthrough
	m2 := MatchData{
		WinnerID: "human1",
		Participants: []ParticipantData{
			{BotID: "evo1", Score: 1, Won: false, PreMatchRating: 1400},
			{BotID: "human1", Score: 5, Won: true, PreMatchRating: 1650},
		},
	}
	if isEvolutionBreakthrough(m2, data) {
		t.Error("Expected human bot winning to not be a breakthrough")
	}

	// Evolved bot beats low-rated opponent = not a breakthrough
	m3 := MatchData{
		WinnerID: "evo1",
		Participants: []ParticipantData{
			{BotID: "evo1", Score: 5, Won: true, PreMatchRating: 1400},
			{BotID: "human1", Score: 1, Won: false, PreMatchRating: 1200},
		},
	}
	if isEvolutionBreakthrough(m3, data) {
		t.Error("Expected evolved bot beating low-rated opponent to not be a breakthrough")
	}
}

func TestIsRivalryMatch(t *testing.T) {
	_ = time.Now() // ensure time import used
	matches := []MatchData{
		{ID: "m1", WinnerID: "bot1", Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
		{ID: "m2", WinnerID: "bot2", Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
		{ID: "m3", WinnerID: "bot1", Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
		{ID: "m4", WinnerID: "bot3", Participants: []ParticipantData{{BotID: "bot3"}, {BotID: "bot4"}}},
	}
	data := &IndexData{Matches: matches}

	// bot1 vs bot2 has 3 matches = rivalry
	m := MatchData{
		WinnerID:     "bot1",
		Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}},
	}
	if !isRivalryMatch(m, data) {
		t.Error("Expected 3-match pair to be a rivalry")
	}

	// bot3 vs bot4 has only 1 match = not a rivalry
	m2 := MatchData{
		WinnerID:     "bot3",
		Participants: []ParticipantData{{BotID: "bot3"}, {BotID: "bot4"}},
	}
	if isRivalryMatch(m2, data) {
		t.Error("Expected 1-match pair to not be a rivalry")
	}
}

func TestCurateWeeklyHighlights(t *testing.T) {
	now := time.Now()

	matches := []MatchData{
		// 1. Upset: underdog wins by large rating margin
		{
			ID: "upset1", WinnerID: "bot2", TurnCount: 250, CompletedAt: now.Add(-time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 1, Won: false, PreMatchRating: 1800},
				{BotID: "bot2", Score: 3, Won: true, PreMatchRating: 1400},
			},
		},
		// 2. Elite clash: very high combined rating
		{
			ID: "elite1", WinnerID: "bot1", TurnCount: 150, CompletedAt: now.Add(-2 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 4, Won: true, PreMatchRating: 1800},
				{BotID: "bot2", Score: 2, Won: false, PreMatchRating: 1700},
			},
		},
		// 3. Marathon: very long match
		{
			ID: "marathon1", WinnerID: "bot1", TurnCount: 450, CompletedAt: now.Add(-3 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 5, Won: true, PreMatchRating: 1200},
				{BotID: "bot2", Score: 1, Won: false, PreMatchRating: 1200},
			},
		},
		// 4. Close finish: score diff of 1
		{
			ID: "close1", WinnerID: "bot1", TurnCount: 200, CompletedAt: now.Add(-4 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 3, Won: true, PreMatchRating: 1200},
				{BotID: "bot2", Score: 2, Won: false, PreMatchRating: 1200},
			},
		},
		// Extra filler matches to fill out the criteria
		{
			ID: "filler1", WinnerID: "bot1", TurnCount: 100, CompletedAt: now.Add(-5 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 5, Won: true, PreMatchRating: 1500},
				{BotID: "bot2", Score: 0, Won: false, PreMatchRating: 1500},
			},
		},
		{
			ID: "filler2", WinnerID: "bot2", TurnCount: 150, CompletedAt: now.Add(-6 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 1, Won: false, PreMatchRating: 1500},
				{BotID: "bot2", Score: 4, Won: true, PreMatchRating: 1500},
			},
		},
		// Old match — outside 7 days
		{
			ID: "old1", WinnerID: "bot1", TurnCount: 400, CompletedAt: now.Add(-8 * 24 * time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 3, Won: true, PreMatchRating: 1500},
				{BotID: "bot2", Score: 2, Won: false, PreMatchRating: 1500},
			},
		},
		// No winner
		{
			ID: "nowin", WinnerID: "", TurnCount: 300, CompletedAt: now.Add(-time.Hour),
			Participants: []ParticipantData{
				{BotID: "bot1", Score: 2},
				{BotID: "bot2", Score: 2},
			},
		},
	}

	cutoff := now.AddDate(0, 0, -7)
	curated := curateWeeklyHighlights(matches, cutoff)

	if len(curated) == 0 {
		t.Fatal("Expected curated matches, got 0")
	}
	if len(curated) > 20 {
		t.Errorf("Expected at most 20 curated matches, got %d", len(curated))
	}

	seenIDs := make(map[string]bool)
	for _, c := range curated {
		if c.Match.ID == "" {
			t.Error("Curated match has empty ID")
		}
		if c.Tag == "" {
			t.Errorf("Curated match %s has empty tag", c.Match.ID)
		}
		if seenIDs[c.Match.ID] {
			t.Errorf("Duplicate match ID in curated results: %s", c.Match.ID)
		}
		seenIDs[c.Match.ID] = true
	}

	if seenIDs["old1"] {
		t.Error("Old match should not appear in weekly highlights")
	}
	if seenIDs["nowin"] {
		t.Error("No-winner match should not appear in weekly highlights")
	}

	// Verify at least one tag per criteria type
	tagTypes := make(map[string]bool)
	for _, c := range curated {
		if strings.Contains(c.Tag, "Closest finish") {
			tagTypes["closest"] = true
		}
		if strings.Contains(c.Tag, "Marathon battle") {
			tagTypes["marathon"] = true
		}
		if strings.Contains(c.Tag, "Elite clash") {
			tagTypes["elite"] = true
		}
		if strings.Contains(c.Tag, "Upset victory") {
			tagTypes["upset"] = true
		}
	}
	if !tagTypes["closest"] {
		t.Error("Expected at least one 'Closest finish' tag")
	}
	if !tagTypes["marathon"] {
		t.Error("Expected at least one 'Marathon battle' tag")
	}
	if !tagTypes["elite"] {
		t.Error("Expected at least one 'Elite clash' tag")
	}
	if !tagTypes["upset"] {
		t.Error("Expected at least one 'Upset victory' tag")
	}
}

func TestBestOfWeekPlaylistHasCurationTags(t *testing.T) {
	now := time.Now()
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "bot1", Name: "Bot1"},
			{ID: "bot2", Name: "Bot2"},
		},
		Matches: []MatchData{
			{
				ID: "weekly_close", WinnerID: "bot1", TurnCount: 200, CompletedAt: now.Add(-time.Hour),
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 3, Won: true, PreMatchRating: 1500},
					{BotID: "bot2", Score: 2, Won: false, PreMatchRating: 1500},
				},
			},
			{
				ID: "weekly_marathon", WinnerID: "bot2", TurnCount: 450, CompletedAt: now.Add(-2 * time.Hour),
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 2, Won: false, PreMatchRating: 1700},
					{BotID: "bot2", Score: 4, Won: true, PreMatchRating: 1600},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	playlistsDir := filepath.Join(tmpDir, "data", "playlists")
	if err := os.MkdirAll(playlistsDir, 0755); err != nil {
		t.Fatalf("Failed to create playlists dir: %v", err)
	}

	botNameMap := map[string]string{"bot1": "Bot1", "bot2": "Bot2"}
	if err := generatePlaylists(data, tmpDir, botNameMap); err != nil {
		t.Fatalf("generatePlaylists failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(playlistsDir, "best-of-week.json"))
	if err != nil {
		t.Fatalf("Failed to read best-of-week.json: %v", err)
	}
	var playlist Playlist
	if err := json.Unmarshal(content, &playlist); err != nil {
		t.Fatalf("Failed to parse best-of-week.json: %v", err)
	}

	if playlist.Category != "weekly" {
		t.Errorf("best-of-week category: got %q, want %q", playlist.Category, "weekly")
	}

	tagCount := 0
	for _, m := range playlist.Matches {
		if m.CurationTag != "" {
			tagCount++
		}
	}
	if tagCount == 0 {
		t.Error("Expected best-of-week matches to have curation tags, got 0")
	}
}

func TestGeneratePlaylistsWithNewTypes(t *testing.T) {
	now := time.Now()
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "human1", Name: "HumanBot", Evolved: false},
			{ID: "evo1", Name: "EvoBot", Evolved: true},
		},
		Matches: []MatchData{
			// Comeback: close upset
			{
				ID: "comeback1", WinnerID: "human1", TurnCount: 400, CompletedAt: now,
				Participants: []ParticipantData{
					{BotID: "human1", Score: 3, Won: true, PreMatchRating: 1500},
					{BotID: "evo1", Score: 2, Won: false, PreMatchRating: 1700},
				},
			},
			// Evolution breakthrough: evolved bot beats rated opponent
			{
				ID: "evo1", WinnerID: "evo1", TurnCount: 300, CompletedAt: now,
				Participants: []ParticipantData{
					{BotID: "evo1", Score: 4, Won: true, PreMatchRating: 1400},
					{BotID: "human1", Score: 2, Won: false, PreMatchRating: 1650},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	playlistsDir := filepath.Join(tmpDir, "data", "playlists")
	if err := os.MkdirAll(playlistsDir, 0755); err != nil {
		t.Fatalf("Failed to create playlists dir: %v", err)
	}

	botNameMap := map[string]string{"human1": "HumanBot", "evo1": "EvoBot"}
	if err := generatePlaylists(data, tmpDir, botNameMap); err != nil {
		t.Fatalf("generatePlaylists failed: %v", err)
	}

	// Verify best-comebacks.json
	cb, err := os.ReadFile(filepath.Join(playlistsDir, "best-comebacks.json"))
	if err != nil {
		t.Fatalf("Failed to read best-comebacks.json: %v", err)
	}
	var cbPlaylist Playlist
	if err := json.Unmarshal(cb, &cbPlaylist); err != nil {
		t.Fatalf("Failed to parse best-comebacks.json: %v", err)
	}
	if cbPlaylist.Category != "comebacks" {
		t.Errorf("comebacks category: got %q", cbPlaylist.Category)
	}

	// Verify evolution-breakthroughs.json
	eb, err := os.ReadFile(filepath.Join(playlistsDir, "evolution-breakthroughs.json"))
	if err != nil {
		t.Fatalf("Failed to read evolution-breakthroughs.json: %v", err)
	}
	var ebPlaylist Playlist
	if err := json.Unmarshal(eb, &ebPlaylist); err != nil {
		t.Fatalf("Failed to parse evolution-breakthroughs.json: %v", err)
	}
	if len(ebPlaylist.Matches) < 1 {
		t.Errorf("Expected at least 1 evolution breakthrough, got %d", len(ebPlaylist.Matches))
	}

	// Verify index.json includes new playlist types
	idx, err := os.ReadFile(filepath.Join(playlistsDir, "index.json"))
	if err != nil {
		t.Fatalf("Failed to read index.json: %v", err)
	}
	var index PlaylistIndex
	if err := json.Unmarshal(idx, &index); err != nil {
		t.Fatalf("Failed to parse index.json: %v", err)
	}
	slugs := make(map[string]bool)
	for _, p := range index.Playlists {
		slugs[p.Slug] = true
	}
	for _, required := range []string{"best-comebacks", "evolution-breakthroughs", "rivalry-classics"} {
		if !slugs[required] {
			t.Errorf("Missing playlist %q in index", required)
		}
	}
}

func TestSortSlice(t *testing.T) {
	matches := []MatchData{
		{ID: "m1", TurnCount: 100},
		{ID: "m2", TurnCount: 300},
		{ID: "m3", TurnCount: 200},
	}
	sortSlice(matches, func(i, j int) bool {
		return matches[i].TurnCount > matches[j].TurnCount
	})
	if matches[0].ID != "m2" || matches[1].ID != "m3" || matches[2].ID != "m1" {
		t.Errorf("sortSlice: unexpected order: %v", matches)
	}
}

func TestFilterMatches(t *testing.T) {
	matches := []MatchData{
		{ID: "m1", TurnCount: 100},
		{ID: "m2", TurnCount: 200},
		{ID: "m3", TurnCount: 300},
	}

	filtered := filterMatches(matches, func(m MatchData) bool {
		return m.TurnCount >= 200
	})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(filtered))
	}
}

func TestRound1(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{75.0, 75.0},
		{75.55, 75.6},
		{75.54, 75.5},
		{0.0, 0.0},
		{99.99, 100.0},
	}

	for _, tt := range tests {
		result := round1(tt.input)
		if result != tt.expected {
			t.Errorf("round1(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestComputeTopPredictors(t *testing.T) {
	stats := []PredictorStats{
		{PredictorID: "p1", Correct: 10, Incorrect: 2, Streak: 5, BestStreak: 8},
		{PredictorID: "p2", Correct: 8, Incorrect: 3, Streak: 2, BestStreak: 5},
		{PredictorID: "p3", Correct: 15, Incorrect: 5, Streak: 3, BestStreak: 10},
	}

	top := computeTopPredictors(stats)

	// Should return all 3 if under 50
	if len(top) != 3 {
		t.Errorf("Expected 3 predictors, got %d", len(top))
	}
}

func TestWriteJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := map[string]string{"key": "value"}
	if err := writeJSON(path, data); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Verify it's valid JSON with proper formatting
	var result map[string]string
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("JSON content: got %q, want %q", result["key"], "value")
	}

	// Verify indentation (should contain newlines)
	if len(content) < 20 {
		t.Errorf("JSON seems unformatted: %q", string(content))
	}
}

func TestGenerateBotCard(t *testing.T) {
	cfg := DefaultCardConfig

	bot := BotCard{
		BotID:         "bot_test123",
		Name:          "TestBot",
		Rating:        1650,
		WinRate:       75.5,
		MatchesPlayed: 100,
		Wins:          75,
		Losses:        25,
		Rank:          1,
		Evolved:       false,
		HealthStatus:  "ACTIVE",
	}

	img, err := generateBotCard(bot, cfg)
	if err != nil {
		t.Fatalf("generateBotCard failed: %v", err)
	}

	// Verify image dimensions
	bounds := img.Bounds()
	if bounds.Dx() != cfg.Width {
		t.Errorf("Image width: got %d, want %d", bounds.Dx(), cfg.Width)
	}
	if bounds.Dy() != cfg.Height {
		t.Errorf("Image height: got %d, want %d", bounds.Dy(), cfg.Height)
	}

	// Verify the image is not blank (should have some non-zero pixels)
	hasContent := false
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if r > 0 || g > 0 || b > 0 || a > 0 {
				hasContent = true
				break
			}
		}
		if hasContent {
			break
		}
	}

	if !hasContent {
		t.Error("Generated image appears to be blank")
	}
}

func TestGenerateBotCardEvolved(t *testing.T) {
	cfg := DefaultCardConfig

	bot := BotCard{
		BotID:         "evolved_bot456",
		Name:          "EvolvedBot",
		Rating:        1820,
		WinRate:       82.0,
		MatchesPlayed: 50,
		Wins:          41,
		Losses:        9,
		Rank:          5,
		Evolved:       true,
		Island:        "python",
		Generation:    10,
		HealthStatus:  "ACTIVE",
	}

	img, err := generateBotCard(bot, cfg)
	if err != nil {
		t.Fatalf("generateBotCard failed: %v", err)
	}

	// Verify image dimensions
	bounds := img.Bounds()
	if bounds.Dx() != cfg.Width {
		t.Errorf("Image width: got %d, want %d", bounds.Dx(), cfg.Width)
	}
	if bounds.Dy() != cfg.Height {
		t.Errorf("Image height: got %d, want %d", bounds.Dy(), cfg.Height)
	}
}

func TestGenerateAllBotCards(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Now(),
		Bots: []BotData{
			{
				ID:            "bot1",
				Name:          "TestBot1",
				Rating:        1650.0,
				MatchesPlayed: 100,
				MatchesWon:    75,
				HealthStatus:  "ACTIVE",
				Evolved:       false,
			},
			{
				ID:            "bot2",
				Name:          "TestBot2",
				Rating:        1550.0,
				MatchesPlayed: 50,
				MatchesWon:    25,
				HealthStatus:  "ACTIVE",
				Evolved:       true,
				Island:        "python",
				Generation:    5,
			},
		},
	}

	tmpDir := t.TempDir()

	if err := generateAllBotCards(data, tmpDir); err != nil {
		t.Fatalf("generateAllBotCards failed: %v", err)
	}

	// Verify cards directory was created
	cardsDir := filepath.Join(tmpDir, "cards")
	if _, err := os.Stat(cardsDir); os.IsNotExist(err) {
		t.Error("Cards directory was not created")
	}

	// Verify PNG files were created for each bot
	for _, bot := range data.Bots {
		cardPath := filepath.Join(cardsDir, bot.ID+".png")
		if _, err := os.Stat(cardPath); os.IsNotExist(err) {
			t.Errorf("Card file not created for bot %s", bot.ID)
		}

		// Verify the file is a valid PNG by checking its header
		content, err := os.ReadFile(cardPath)
		if err != nil {
			t.Errorf("Failed to read card file for bot %s: %v", bot.ID, err)
			continue
		}

		// PNG files start with these bytes
		pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		if len(content) < len(pngHeader) {
			t.Errorf("Card file too small for bot %s: %d bytes", bot.ID, len(content))
			continue
		}

		for i, b := range pngHeader {
			if content[i] != b {
				t.Errorf("Card file for bot %s is not a valid PNG (header mismatch at byte %d)", bot.ID, i)
				break
			}
		}
	}
}

func TestGetColorForRating(t *testing.T) {
	tests := []struct {
		rating    int
		name      string
		checkR    uint8
	}{
		{2100, "gold", 255},
		{1850, "silver", 192},
		{1650, "bronze", 205},
		{1450, "green", 100},
		{1200, "gray", 200},
	}

	for _, tt := range tests {
		col := getColorForRating(tt.rating)
		if col.R != tt.checkR {
			t.Errorf("getColorForRating(%d): R = %d, want %d", tt.rating, col.R, tt.checkR)
		}
	}
}

func TestGetWinRateColor(t *testing.T) {
	tests := []struct {
		winRate float64
		name    string
		checkG  uint8
	}{
		{75.0, "green", 197},
		{60.0, "blue", 130},
		{40.0, "yellow", 179},
		{20.0, "red", 68},
	}

	for _, tt := range tests {
		col := getWinRateColor(tt.winRate)
		if col.G != tt.checkG {
			t.Errorf("getWinRateColor(%f): G = %d, want %d", tt.winRate, col.G, tt.checkG)
		}
	}
}

func TestGetRankBadgeColor(t *testing.T) {
	tests := []struct {
		rank    int
		name    string
		checkR  uint8
	}{
		{1, "gold", 255},
		{2, "silver", 192},
		{3, "bronze", 205},
		{5, "blue", 59},
		{50, "gray", 100},
	}

	for _, tt := range tests {
		col := getRankBadgeColor(tt.rank)
		if col.R != tt.checkR {
			t.Errorf("getRankBadgeColor(%d): R = %d, want %d", tt.rank, col.R, tt.checkR)
		}
	}
}

func TestGetAccentColor(t *testing.T) {
	// Test evolved bot
	evolvedCol := getAccentColor(true, "ACTIVE")
	if evolvedCol.R != 138 || evolvedCol.G != 43 || evolvedCol.B != 226 {
		t.Errorf("Evolved accent color: got R=%d,G=%d,B=%d, want purple (138,43,226)",
			evolvedCol.R, evolvedCol.G, evolvedCol.B)
	}

	// Test inactive bot
	inactiveCol := getAccentColor(false, "INACTIVE")
	if inactiveCol.R != 128 || inactiveCol.G != 128 || inactiveCol.B != 128 {
		t.Errorf("Inactive accent color: got R=%d,G=%d,B=%d, want gray (128,128,128)",
			inactiveCol.R, inactiveCol.G, inactiveCol.B)
	}

	// Test active bot
	activeCol := getAccentColor(false, "ACTIVE")
	if activeCol.R != 59 || activeCol.G != 130 || activeCol.B != 246 {
		t.Errorf("Active accent color: got R=%d,G=%d,B=%d, want blue (59,130,246)",
			activeCol.R, activeCol.G, activeCol.B)
	}
}

func TestSavePNG(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.png")

	// Create a simple test image
	img := generateTestImage(100, 100)

	if err := savePNG(path, img); err != nil {
		t.Fatalf("savePNG failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("PNG file was not created")
	}

	// Verify file is a valid PNG
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read PNG file: %v", err)
	}

	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngHeader {
		if content[i] != b {
			t.Errorf("File is not a valid PNG (header mismatch at byte %d)", i)
			break
		}
	}
}

// ── Fast playlist helper tests ────────────────────────────────────────────

func TestBuildFirstMatchPerBot(t *testing.T) {
	now := time.Now()
	matches := []MatchData{
		{ID: "m1", WinnerID: "bot1", CompletedAt: now.Add(-3 * time.Hour),
			Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
		{ID: "m2", WinnerID: "bot2", CompletedAt: now.Add(-2 * time.Hour),
			Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
		{ID: "m3", WinnerID: "bot3", CompletedAt: now.Add(-time.Hour),
			Participants: []ParticipantData{{BotID: "bot3"}, {BotID: "bot4"}}},
	}

	firstMap := buildFirstMatchPerBot(matches)

	if firstMap["bot1"] != "m1" {
		t.Errorf("bot1 first match: got %q, want m1", firstMap["bot1"])
	}
	if firstMap["bot2"] != "m1" {
		t.Errorf("bot2 first match: got %q, want m1", firstMap["bot2"])
	}
	if firstMap["bot3"] != "m3" {
		t.Errorf("bot3 first match: got %q, want m3", firstMap["bot3"])
	}
	if firstMap["bot4"] != "m3" {
		t.Errorf("bot4 first match: got %q, want m3", firstMap["bot4"])
	}
}

func TestBuildFirstMatchPerBot_SkipsIncomplete(t *testing.T) {
	now := time.Now()
	matches := []MatchData{
		{ID: "m_incomplete", WinnerID: "", CompletedAt: now,
			Participants: []ParticipantData{{BotID: "bot1"}}},
		{ID: "m_complete", WinnerID: "bot1", CompletedAt: now.Add(time.Hour),
			Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
	}

	firstMap := buildFirstMatchPerBot(matches)

	if firstMap["bot1"] != "m_complete" {
		t.Errorf("bot1 should only have m_complete (incomplete skipped), got %q", firstMap["bot1"])
	}
}

func TestBuildFirstMatchPerBot_Empty(t *testing.T) {
	firstMap := buildFirstMatchPerBot(nil)
	if len(firstMap) != 0 {
		t.Errorf("expected empty map for nil input, got %d entries", len(firstMap))
	}
}

func TestIsNewBotDebutFast(t *testing.T) {
	firstMap := map[string]string{
		"bot1": "m1",
		"bot2": "m2",
	}

	// bot1's debut is m1
	m1 := MatchData{ID: "m1", WinnerID: "bot1",
		Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}}
	if !isNewBotDebutFast(m1, firstMap) {
		t.Error("m1 should be a debut (bot1's first match)")
	}

	// m3 is neither bot's first match
	m3 := MatchData{ID: "m3", WinnerID: "bot1",
		Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}}
	if isNewBotDebutFast(m3, firstMap) {
		t.Error("m3 should not be a debut")
	}

	// No winner = not a debut
	m4 := MatchData{ID: "m1", WinnerID: "",
		Participants: []ParticipantData{{BotID: "bot1"}}}
	if isNewBotDebutFast(m4, firstMap) {
		t.Error("match with no winner should not be a debut")
	}
}

func TestBuildPairFrequency(t *testing.T) {
	matches := []MatchData{
		{Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
		{Participants: []ParticipantData{{BotID: "bot2"}, {BotID: "bot1"}}},
		{Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}},
		{Participants: []ParticipantData{{BotID: "bot3"}, {BotID: "bot4"}}},
		// 3-player match should be skipped
		{Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}, {BotID: "bot5"}}},
	}

	freq := buildPairFrequency(matches)

	if freq["bot1:bot2"] != 3 {
		t.Errorf("bot1:bot2 frequency: got %d, want 3", freq["bot1:bot2"])
	}
	if freq["bot3:bot4"] != 1 {
		t.Errorf("bot3:bot4 frequency: got %d, want 1", freq["bot3:bot4"])
	}
	if _, ok := freq["bot1:bot5"]; ok {
		t.Error("3-player match should not create a pair entry")
	}
}

func TestBuildPairFrequency_Empty(t *testing.T) {
	freq := buildPairFrequency(nil)
	if len(freq) != 0 {
		t.Errorf("expected empty map for nil input, got %d entries", len(freq))
	}
}

func TestIsRivalryMatchFast(t *testing.T) {
	freq := map[string]int{
		"bot1:bot2": 5,
		"bot3:bot4": 2,
	}

	// 5 matches = rivalry
	m1 := MatchData{WinnerID: "bot1",
		Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}}
	if !isRivalryMatchFast(m1, freq) {
		t.Error("bot1 vs bot2 with 5 matches should be a rivalry")
	}

	// 2 matches = not a rivalry
	m2 := MatchData{WinnerID: "bot3",
		Participants: []ParticipantData{{BotID: "bot3"}, {BotID: "bot4"}}}
	if isRivalryMatchFast(m2, freq) {
		t.Error("bot3 vs bot4 with 2 matches should not be a rivalry")
	}

	// No winner = not a rivalry
	m3 := MatchData{WinnerID: "",
		Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}}}
	if isRivalryMatchFast(m3, freq) {
		t.Error("match with no winner should not be a rivalry")
	}

	// 3+ players = not checked
	m4 := MatchData{WinnerID: "bot1",
		Participants: []ParticipantData{{BotID: "bot1"}, {BotID: "bot2"}, {BotID: "bot3"}}}
	if isRivalryMatchFast(m4, freq) {
		t.Error("3-player match should not be a rivalry")
	}
}

func TestGeneratePlaylistsWithFastLookups(t *testing.T) {
	now := time.Now()
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "bot1", Name: "Bot1"},
			{ID: "bot2", Name: "Bot2"},
			{ID: "bot3", Name: "Bot3"},
		},
		Matches: []MatchData{
			// New bot debut for bot3
			{ID: "debut1", WinnerID: "bot3", TurnCount: 200, CompletedAt: now,
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 2, Won: false, PreMatchRating: 1500},
					{BotID: "bot3", Score: 3, Won: true, PreMatchRating: 1400},
				}},
			// Rivalry match (bot1 vs bot2, 3rd meeting)
			{ID: "rival1", WinnerID: "bot1", TurnCount: 300, CompletedAt: now.Add(-time.Hour),
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 4, Won: true, PreMatchRating: 1600},
					{BotID: "bot2", Score: 3, Won: false, PreMatchRating: 1550},
				}},
			{ID: "rival2", WinnerID: "bot2", TurnCount: 250, CompletedAt: now.Add(-2 * time.Hour),
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 1, Won: false, PreMatchRating: 1580},
					{BotID: "bot2", Score: 5, Won: true, PreMatchRating: 1560},
				}},
			{ID: "rival3", WinnerID: "bot1", TurnCount: 350, CompletedAt: now.Add(-3 * time.Hour),
				Participants: []ParticipantData{
					{BotID: "bot1", Score: 3, Won: true, PreMatchRating: 1590},
					{BotID: "bot2", Score: 2, Won: false, PreMatchRating: 1570},
				}},
		},
	}

	tmpDir := t.TempDir()
	playlistsDir := filepath.Join(tmpDir, "data", "playlists")
	if err := os.MkdirAll(playlistsDir, 0755); err != nil {
		t.Fatalf("Failed to create playlists dir: %v", err)
	}

	botNameMap := map[string]string{"bot1": "Bot1", "bot2": "Bot2", "bot3": "Bot3"}
	if err := generatePlaylists(data, tmpDir, botNameMap); err != nil {
		t.Fatalf("generatePlaylists failed: %v", err)
	}

	// Verify new-bot-debuts includes bot3's debut
	debutContent, err := os.ReadFile(filepath.Join(playlistsDir, "new-bot-debuts.json"))
	if err != nil {
		t.Fatalf("Failed to read new-bot-debuts.json: %v", err)
	}
	var debutPlaylist Playlist
	if err := json.Unmarshal(debutContent, &debutPlaylist); err != nil {
		t.Fatalf("Failed to parse new-bot-debuts.json: %v", err)
	}
	foundDebut := false
	for _, m := range debutPlaylist.Matches {
		if m.MatchID == "debut1" {
			foundDebut = true
			break
		}
	}
	if !foundDebut {
		t.Errorf("new-bot-debuts should include debut1 (bot3's first match), got %d matches", len(debutPlaylist.Matches))
	}

	// Verify rivalry-classics includes bot1 vs bot2 matches
	rivalryContent, err := os.ReadFile(filepath.Join(playlistsDir, "rivalry-classics.json"))
	if err != nil {
		t.Fatalf("Failed to read rivalry-classics.json: %v", err)
	}
	var rivalryPlaylist Playlist
	if err := json.Unmarshal(rivalryContent, &rivalryPlaylist); err != nil {
		t.Fatalf("Failed to parse rivalry-classics.json: %v", err)
	}
	if len(rivalryPlaylist.Matches) < 3 {
		t.Errorf("rivalry-classics should have 3 matches for bot1:bot2 (count=3), got %d", len(rivalryPlaylist.Matches))
	}
}
