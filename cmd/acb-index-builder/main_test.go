package main

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
				Participants: []MatchParticipant{
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
				Participants: []MatchParticipant{
					{BotID: "bot1", Score: 3, Won: true},
					{BotID: "bot2", Score: 2, Won: false}, // Close finish (diff = 1)
				},
			},
			{
				ID:           "match2",
				WinnerID:     "bot2",
				TurnCount:    150,
				EndCondition: "dominance",
				CompletedAt:  now.Add(-time.Hour),
				Participants: []MatchParticipant{
					{BotID: "bot1", Score: 0, Won: false},
					{BotID: "bot2", Score: 10, Won: true}, // Not close (diff = 10)
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

	// Check closest-finishes playlist
	content, err := os.ReadFile(filepath.Join(playlistsDir, "closest-finishes.json"))
	if err != nil {
		t.Fatalf("Failed to read closest-finishes.json: %v", err)
	}

	var playlist Playlist
	if err := json.Unmarshal(content, &playlist); err != nil {
		t.Fatalf("Failed to parse closest-finishes.json: %v", err)
	}

	// Should only include match1 (close finish)
	if len(playlist.Matches) != 1 {
		t.Errorf("closest-finishes: expected 1 match, got %d", len(playlist.Matches))
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

func generateTestImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		}
	}
	return img
}
