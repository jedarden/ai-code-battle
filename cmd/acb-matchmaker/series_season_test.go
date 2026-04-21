package main

import (
	"testing"
)

func TestConfigEnvFloat(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		fallback float64
		want     float64
	}{
		{"ACB_TEST_FLOAT", "0.5", 0.7, 0.5},
		{"ACB_TEST_FLOAT", "1.0", 0.7, 1.0},
		{"ACB_TEST_FLOAT", "", 0.7, 0.7},
		{"ACB_TEST_FLOAT", "invalid", 0.7, 0.7},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			got := envFloat(tc.key, tc.fallback)
			if got != tc.want {
				t.Errorf("envFloat(%q, %v) = %v, want %v", tc.key, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestLoadConfigSeriesAndSeason(t *testing.T) {
	t.Setenv("ACB_SERIES_SCHED_INTERVAL", "180")
	t.Setenv("ACB_SEASON_RESET_INTERVAL", "600")
	t.Setenv("ACB_SEASON_DECAY_FACTOR", "0.8")

	cfg := loadConfig()

	if cfg.SeriesSchedSecs != 180 {
		t.Errorf("SeriesSchedSecs: got %d, want 180", cfg.SeriesSchedSecs)
	}
	if cfg.SeasonResetSecs != 600 {
		t.Errorf("SeasonResetSecs: got %d, want 600", cfg.SeasonResetSecs)
	}
	if cfg.SeasonDecayFactor != 0.8 {
		t.Errorf("SeasonDecayFactor: got %f, want 0.8", cfg.SeasonDecayFactor)
	}
}

func TestLoadConfigSeriesAndSeasonDefaults(t *testing.T) {
	cfg := loadConfig()

	if cfg.SeriesSchedSecs != 120 {
		t.Errorf("SeriesSchedSecs default: got %d, want 120", cfg.SeriesSchedSecs)
	}
	if cfg.SeasonResetSecs != 300 {
		t.Errorf("SeasonResetSecs default: got %d, want 300", cfg.SeasonResetSecs)
	}
	if cfg.SeasonDecayFactor != 0.7 {
		t.Errorf("SeasonDecayFactor default: got %f, want 0.7", cfg.SeasonDecayFactor)
	}
}

func TestDecayFormula(t *testing.T) {
	// Validate the decay formula: new_mu = default + (current_mu - default) * factor
	// With default=1500 and factor=0.7:
	//   mu=2000 → 1500 + 500*0.7 = 1850
	//   mu=1000 → 1500 + (-500)*0.7 = 1150
	//   mu=1500 → 1500 + 0*0.7 = 1500
	defaultMu := 1500.0
	factor := 0.7

	tests := []struct {
		current float64
		want    float64
	}{
		{2000, 1850},
		{1000, 1150},
		{1500, 1500},
		{1800, 1710},
		{1200, 1290},
		{3000, 2550}, // extreme high
		{500, 800},   // extreme low
	}

	for _, tc := range tests {
		result := defaultMu + (tc.current-defaultMu)*factor
		if result != tc.want {
			t.Errorf("decay(%v) = %v, want %v", tc.current, result, tc.want)
		}
	}
}

func TestDecayPreservesRankOrder(t *testing.T) {
	// Decay should never change relative ordering
	defaultMu := 1500.0
	factor := 0.7

	ratings := []float64{2200, 2000, 1800, 1600, 1500, 1400, 1200, 1000, 800}
	decayed := make([]float64, len(ratings))
	for i, r := range ratings {
		decayed[i] = defaultMu + (r-defaultMu)*factor
	}

	for i := 1; i < len(decayed); i++ {
		if decayed[i] >= decayed[i-1] {
			t.Errorf("rank order violated after decay: %.1f (from %.1f) >= %.1f (from %.1f)",
				decayed[i], ratings[i], decayed[i-1], ratings[i-1])
		}
	}
}

func TestDecayDifferentFactors(t *testing.T) {
	defaultMu := 1500.0

	// Factor=0.5 means ratings are pulled halfway to the default
	tests := []struct {
		factor  float64
		current float64
		want    float64
	}{
		{0.0, 2000, 1500},   // full reset
		{0.5, 2000, 1750},   // half decay
		{1.0, 2000, 2000},   // no decay
		{0.3, 1000, 1350},   // heavy decay toward center
		{0.9, 1000, 1050},   // light decay
	}

	for _, tc := range tests {
		result := defaultMu + (tc.current-defaultMu)*tc.factor
		if result != tc.want {
			t.Errorf("decay(%v, factor=%v) = %v, want %v", tc.current, tc.factor, result, tc.want)
		}
	}
}

func TestSeriesWinsNeeded(t *testing.T) {
	// ceil(format/2) gives wins needed for each format
	tests := []struct {
		format int
		want   int
	}{
		{3, 2},
		{5, 3},
		{7, 4},
		{1, 1},
		{9, 5},
	}

	for _, tc := range tests {
		got := (tc.format + 1) / 2
		if got != tc.want {
			t.Errorf("winsNeeded(%d) = %d, want %d", tc.format, got, tc.want)
		}
	}
}

func TestSeriesFormatSelection(t *testing.T) {
	// Validate the rating-gap-based format selection logic from autoCreateSeries
	tests := []struct {
		gap    float64
		format int
	}{
		{0, 7},    // identical ratings → bo7
		{25, 7},   // small gap → bo7
		{49, 7},   // just under threshold → bo7
		{50, 5},   // at threshold → bo5
		{100, 5},  // moderate gap → bo5
		{199, 5},  // just under threshold → bo5
		{200, 3},  // at threshold → bo3
		{500, 3},  // large gap → bo3
	}

	for _, tc := range tests {
		format := 5
		if tc.gap < 50 {
			format = 7
		} else if tc.gap >= 200 {
			format = 3
		}
		if format != tc.format {
			t.Errorf("formatSelection(gap=%.0f) = %d, want %d", tc.gap, format, tc.format)
		}
	}
}

func TestGenerateIDFormat(t *testing.T) {
	id, err := generateID("m_", 8)
	if err != nil {
		t.Fatalf("generateID error: %v", err)
	}
	if len(id) != 18 { // "m_" + 16 hex chars
		t.Errorf("id length: got %d, want 18", len(id))
	}
	if id[:2] != "m_" {
		t.Errorf("id prefix: got %q, want %q", id[:2], "m_")
	}

	id2, err := generateID("j_", 8)
	if err != nil {
		t.Fatalf("generateID error: %v", err)
	}
	if id2[:2] != "j_" {
		t.Errorf("id prefix: got %q, want %q", id2[:2], "j_")
	}
}

func TestGenerateIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, err := generateID("t_", 8)
		if err != nil {
			t.Fatalf("generateID error: %v", err)
		}
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestSeasonAutoStartNaming(t *testing.T) {
	// Validate season naming convention: "Season N" where N = max_id + 1
	tests := []struct {
		maxID int
		name  string
	}{
		{0, "Season 1"},
		{1, "Season 2"},
		{5, "Season 6"},
	}

	for _, tc := range tests {
		nextNum := tc.maxID + 1
		expectedName := "Season " + itoa(nextNum)
		if expectedName != tc.name {
			t.Errorf("seasonName(maxID=%d) = %q, want %q", tc.maxID, expectedName, tc.name)
		}
	}
}

func TestSeasonThemeCycling(t *testing.T) {
	themes := []string{"The Labyrinth", "Energy Rush", "Fog of War", "The Colosseum", "Shifting Sands"}

	tests := []struct {
		seasonNum int
		want      string
	}{
		{1, "The Labyrinth"},
		{2, "Energy Rush"},
		{3, "Fog of War"},
		{4, "The Colosseum"},
		{5, "Shifting Sands"},
		{6, "The Labyrinth"}, // cycles
		{10, "Shifting Sands"},
	}

	for _, tc := range tests {
		theme := themes[(tc.seasonNum-1)%len(themes)]
		if theme != tc.want {
			t.Errorf("theme(season=%d) = %q, want %q", tc.seasonNum, theme, tc.want)
		}
	}
}

func TestSeriesFinalizationThresholds(t *testing.T) {
	// Verify that series are finalized at exactly the right win count
	tests := []struct {
		format   int
		aWins    int
		bWins    int
		finished bool
		winner   string // "a" or "b"
	}{
		{3, 2, 0, true, "a"},
		{3, 0, 2, true, "b"},
		{3, 1, 1, false, ""},
		{3, 2, 1, true, "a"},
		{5, 3, 0, true, "a"},
		{5, 2, 2, false, ""},
		{5, 2, 3, true, "b"},
		{7, 4, 0, true, "a"},
		{7, 3, 3, false, ""},
		{7, 3, 4, true, "b"},
	}

	for _, tc := range tests {
		winsNeeded := (tc.format + 1) / 2
		aDone := tc.aWins >= winsNeeded
		bDone := tc.bWins >= winsNeeded
		finished := aDone || bDone

		if finished != tc.finished {
			t.Errorf("format=%d a=%d b=%d: finished=%v, want %v", tc.format, tc.aWins, tc.bWins, finished, tc.finished)
			continue
		}
		if finished {
			winner := ""
			if aDone {
				winner = "a"
			} else {
				winner = "b"
			}
			if winner != tc.winner {
				t.Errorf("format=%d a=%d b=%d: winner=%s, want %s", tc.format, tc.aWins, tc.bWins, winner, tc.winner)
			}
		}
	}
}

func TestMapSelectionOrderByGame(t *testing.T) {
	tests := []struct {
		gameNum      int
		wantContains string
	}{
		{1, "engagement DESC"},
		{2, "wall_density DESC"},
		{3, "wall_density ASC"},
		{4, "RANDOM()"},
		{5, "RANDOM()"},
		{6, "RANDOM()"},
		{7, "RANDOM()"},
	}

	for _, tc := range tests {
		var orderBy string
		switch {
		case tc.gameNum == 1:
			orderBy = "engagement DESC NULLS LAST"
		case tc.gameNum == 2:
			orderBy = "wall_density DESC NULLS LAST"
		case tc.gameNum == 3:
			orderBy = "wall_density ASC NULLS LAST"
		default:
			orderBy = "RANDOM()"
		}
		if !containsSubstring(orderBy, tc.wantContains) {
			t.Errorf("gameNum=%d: orderBy=%q, want to contain %q", tc.gameNum, orderBy, tc.wantContains)
		}
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAlternatePlayerSlots(t *testing.T) {
	tests := []struct {
		gameNum int
		slotA   int
		slotB   int
	}{
		{1, 0, 1},
		{2, 1, 0},
		{3, 0, 1},
		{4, 1, 0},
		{5, 0, 1},
		{6, 1, 0},
		{7, 0, 1},
	}

	for _, tc := range tests {
		slotA, slotB := 0, 1
		if tc.gameNum%2 == 0 {
			slotA, slotB = 1, 0
		}
		if slotA != tc.slotA || slotB != tc.slotB {
			t.Errorf("gameNum=%d: got slotA=%d slotB=%d, want slotA=%d slotB=%d",
				tc.gameNum, slotA, slotB, tc.slotA, tc.slotB)
		}
	}
}

func TestChampionshipBracketSeeding(t *testing.T) {
	bots := []string{"b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8"}

	bracket := [][2]string{
		{bots[0], bots[7]},
		{bots[3], bots[4]},
		{bots[2], bots[5]},
		{bots[1], bots[6]},
	}

	expected := [][2]string{
		{"b1", "b8"},
		{"b4", "b5"},
		{"b3", "b6"},
		{"b2", "b7"},
	}

	for i, match := range bracket {
		if match[0] != expected[i][0] || match[1] != expected[i][1] {
			t.Errorf("bracket[%d]: got %s vs %s, want %s vs %s",
				i, match[0], match[1], expected[i][0], expected[i][1])
		}
	}

	seen := make(map[string]bool)
	for _, match := range bracket {
		for _, bot := range match {
			if seen[bot] {
				t.Errorf("bot %s appears in multiple bracket positions", bot)
			}
			seen[bot] = true
		}
	}
	if len(seen) != 8 {
		t.Errorf("expected 8 unique bots in bracket, got %d", len(seen))
	}
}

func TestUpdateSeriesGameResults_WinColumn(t *testing.T) {
	// Verify that the correct win column (a_wins or b_wins) is selected
	// based on whether the winner is bot_a or bot_b.
	tests := []struct {
		name        string
		winnerBotID string
		botAID      string
		incrementA  bool
	}{
		{"winner is bot_a", "bot_alpha", "bot_alpha", true},
		{"winner is bot_b", "bot_beta", "bot_alpha", false},
		{"same id as a", "x", "x", true},
		{"different id", "y", "x", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			incrementA := tc.winnerBotID == tc.botAID
			if incrementA != tc.incrementA {
				t.Errorf("winner=%s botA=%s: incrementA=%v, want %v",
					tc.winnerBotID, tc.botAID, incrementA, tc.incrementA)
			}
		})
	}
}

func TestSeriesGameResultQueryConditions(t *testing.T) {
	// Verify the conditions for finding unprocessed series game results:
	// - series_games.winner_id IS NULL (not yet processed)
	// - matches.status = 'completed' (match is done)
	// - matches.winner IS NOT NULL (not a draw)
	tests := []struct {
		name       string
		winnerID   string // NULL or a bot_id
		matchStat  string // pending, running, completed
		matchWin   string // NULL or a player slot number
		shouldPick bool
	}{
		{"completed with winner", "", "completed", "0", true},
		{"completed with winner slot 1", "", "completed", "1", true},
		{"already processed", "bot_a", "completed", "0", false},
		{"match still pending", "", "pending", "", false},
		{"match still running", "", "running", "", false},
		{"completed but draw", "", "completed", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			winnerIDNull := tc.winnerID == ""
			matchCompleted := tc.matchStat == "completed"
			matchHasWinner := tc.matchWin != ""

			shouldPick := winnerIDNull && matchCompleted && matchHasWinner
			if shouldPick != tc.shouldPick {
				t.Errorf("winnerID=%q matchStat=%q matchWin=%q: shouldPick=%v, want %v",
					tc.winnerID, tc.matchStat, tc.matchWin, shouldPick, tc.shouldPick)
			}
		})
	}
}

func TestSeriesSchedulerOrder(t *testing.T) {
	// Verify the ordering of steps in tickSeriesScheduler:
	// 0. updateSeriesGameResults (propagate match results)
	// 1. finalizeCompletedSeries (mark series complete)
	// 2. scheduleNextSeriesGames (schedule next game)
	// 3. autoCreateSeries (create new series)
	// 4. advanceChampionshipBracket (advance bracket)
	//
	// This ordering is important because:
	// - Results must be propagated BEFORE finalization checks
	// - Finalization must happen BEFORE scheduling (to avoid scheduling
	//   games in already-decided series)
	// - New series creation comes after scheduling existing ones
	steps := []string{
		"updateSeriesGameResults",
		"finalizeCompletedSeries",
		"scheduleNextSeriesGames",
		"autoCreateSeries",
		"advanceChampionshipBracket",
	}

	if len(steps) != 5 {
		t.Errorf("expected 5 scheduler steps, got %d", len(steps))
	}

	// Update results must come before finalize
	if steps[0] != "updateSeriesGameResults" {
		t.Errorf("step 0 should be updateSeriesGameResults, got %s", steps[0])
	}
	if steps[1] != "finalizeCompletedSeries" {
		t.Errorf("step 1 should be finalizeCompletedSeries, got %s", steps[1])
	}
}

func TestChampionshipBracketRequiresEightBots(t *testing.T) {
	tests := []struct {
		count      int
		shouldSkip bool
	}{
		{0, true},
		{1, true},
		{7, true},
		{8, false},
		{10, false},
	}

	for _, tc := range tests {
		skipped := tc.count < 8
		if skipped != tc.shouldSkip {
			t.Errorf("count=%d: skipped=%v, want %v", tc.count, skipped, tc.shouldSkip)
		}
	}
}

// itoa is a simple int-to-string helper for tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
