package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBuildNarrativePrompt_Rise(t *testing.T) {
	req := NarrativeRequest{
		ArcType:    ArcRise,
		BotName:    "TestBot",
		SeasonName: "Season 4",
		RatingStart: 1200,
		RatingEnd: 1450,
		KeyMatches: []KeyMatch{
			{MatchID: "m1", OpponentName: "TopBot", OpponentRating: 1800, MapName: "The Labyrinth", Score: "3-2", TurnCount: 200, Won: true},
		},
		Archetype: "aggressive",
		Origin:   "evolved, go island, generation 5",
	}

	prompt := buildNarrativePrompt(req)

	if !strings.Contains(prompt, "Arc type: Rise") {
		t.Error("prompt should contain arc type")
	}
	if !strings.Contains(prompt, "TestBot") {
		t.Error("prompt should contain bot name")
	}
	if !strings.Contains(prompt, "1200") || !strings.Contains(prompt, "1450") {
		t.Error("prompt should contain rating range")
	}
	if !strings.Contains(prompt, "Season 4") {
		t.Error("prompt should contain season name")
	}
}

func TestBuildNarrativePrompt_Upset(t *testing.T) {
	req := NarrativeRequest{
		ArcType:    ArcUpset,
		BotName:    "UnderdogBot",
		BotBName:   "FavoriteBot",
		RatingStart: 1100,
		RatingEnd:  1800,
		KeyMatches: []KeyMatch{
			{MatchID: "m2", OpponentName: "FavoriteBot", OpponentRating: 1800, MapName: "Open Field", Score: "4-3", TurnCount: 150, Won: true},
		},
	}

	prompt := buildNarrativePrompt(req)

	if !strings.Contains(prompt, "Upset of the Week") {
		t.Error("prompt should contain upset arc type")
	}
	if !strings.Contains(prompt, "UnderdogBot") {
		t.Error("prompt should contain underdog name")
	}
	if !strings.Contains(prompt, "FavoriteBot") {
		t.Error("prompt should contain favorite name")
	}
}

func TestBuildNarrativePrompt_Rivalry(t *testing.T) {
	req := NarrativeRequest{
		ArcType:    ArcRivalry,
		BotName:    "SwarmBot",
		BotBName:   "HunterBot",
		BotAWins:   5,
		BotBWins:   4,
		TotalMatches: 9,
		SeasonName: "Season 4",
	}

	prompt := buildNarrativePrompt(req)

	if !strings.Contains(prompt, "Rivalry Intensifies") {
		t.Error("prompt should contain rivalry arc type")
	}
	if !strings.Contains(prompt, "SwarmBot") || !strings.Contains(prompt, "HunterBot") {
		t.Error("prompt should contain both bot names")
	}
	if !strings.Contains(prompt, "5-4") {
		t.Error("prompt should contain head-to-head record")
	}
}

func TestBuildNarrativePrompt_Evolution(t *testing.T) {
	req := NarrativeRequest{
		ArcType:    ArcEvolutionMilestone,
		BotName:    "evo-go-g31",
		SeasonName: "Season 4",
		RatingEnd:  1580,
		Origin:    "evolved, go island",
		Generation: 31,
		ParentIDs: []string{"evo-go-g28", "evo-go-g25"},
		Archetype: "hybrid swarm-gatherer",
	}

	prompt := buildNarrativePrompt(req)

	if !strings.Contains(prompt, "Evolution Milestone") {
		t.Error("prompt should contain evolution milestone arc type")
	}
	if !strings.Contains(prompt, "evo-go-g31") {
		t.Error("prompt should contain bot name")
	}
	if !strings.Contains(prompt, "generation 31") {
		t.Error("prompt should contain generation")
	}
}

func TestBuildNarrativePrompt_Comeback(t *testing.T) {
	req := NarrativeRequest{
		ArcType:    ArcComeback,
		BotName:    "ComebackBot",
		SeasonName: "Season 4",
		RatingStart: 1300,
		RatingEnd:  1450,
	}

	prompt := buildNarrativePrompt(req)

	if !strings.Contains(prompt, "Comeback") {
		t.Error("prompt should contain comeback arc type")
	}
	if !strings.Contains(prompt, "1300") {
		t.Error("prompt should contain rating recovery")
	}
}

func TestTruncateSummary(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"Short text", 50, "Short text"},
		{"This is exactly fifty chars long, no more, no less.", 50, "This is exactly fifty chars long, no more, no..."},
		{"A very long piece of text that needs to be truncated", 20, "A very long piece..."},
	}

	for _, tc := range tests {
		result := truncateSummary(tc.input, tc.maxLen)
		if result != tc.expected {
		t.Errorf("truncateSummary(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
	}
	}
}

func TestGetBotRatingHistory(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC),
		RatingHistory: []RatingHistoryEntry{
			{BotID: "bot1", Rating: 1000, RecordedAt: time.Date(2024, 3, 20, 12, 0, 0, 0, time.UTC)},
			{BotID: "bot1", Rating: 1100, RecordedAt: time.Date(2024, 3, 22, 12, 0, 0, 0, time.UTC)},
			{BotID: "bot1", Rating: 1200, RecordedAt: time.Date(2024, 3, 25, 12, 0, 0, 0, time.UTC)},
			{BotID: "bot1", Rating: 1300, RecordedAt: time.Date(2024, 3, 28, 12, 0, 0, 0, time.UTC)},
			{BotID: "bot2", Rating: 1500, RecordedAt: time.Date(2024, 3, 28, 12, 0, 0, 0, time.UTC)},
		},
	}

	history := getBotRatingHistory("bot1", data)
	if len(history) != 4 {
		t.Errorf("expected 4 history entries for bot1, got %d", len(history))
	}

	history = getBotRatingHistory("bot2", data)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry for bot2, got %d", len(history))
	}

	history = getBotRatingHistory("nonexistent", data)
	if len(history) != 0 {
		t.Errorf("expected 0 history entries for nonexistent bot, got %d", len(history))
	}
}

func TestDetectRiseArcs(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{ID: "bot1", Name: "RisingBot", Rating: 1500},
			{ID: "bot2", Name: "StableBot", Rating: 1200},
		},
		RatingHistory: []RatingHistoryEntry{
			// bot1 rose from 1200 to 1500 (300 point gain = rise arc)
			{BotID: "bot1", Rating: 1200, RecordedAt: time.Date(2024, 3, 22, 12, 0, 0, 0, time.UTC)},
			{BotID: "bot1", Rating: 1500, RecordedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)},
			// bot2 only moved 50 points (no arc)
			{BotID: "bot2", Rating: 1150, RecordedAt: time.Date(2024, 3, 22, 12, 0, 0, 0, time.UTC)},
			{BotID: "bot2", Rating: 1200, RecordedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)},
		},
	}

	arcs := detectRiseArcs(data)
	if len(arcs) != 1 {
		t.Errorf("expected 1 rise arc, got %d", len(arcs))
	}
	if len(arcs) > 0 && arcs[0].BotName != "RisingBot" {
		t.Errorf("expected rise arc for RisingBot, got %s", arcs[0].BotName)
	}
}

func TestDetectFallArcs(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{ID: "bot1", Name: "FallingBot", Rating: 1000},
		},
		RatingHistory: []RatingHistoryEntry{
			// bot1 fell from 1300 to 1000 (300 point loss = fall arc)
			{BotID: "bot1", Rating: 1300, RecordedAt: time.Date(2024, 3, 22, 12, 0, 0, 0, time.UTC)},
			{BotID: "bot1", Rating: 1000, RecordedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)},
		},
	}

	arcs := detectFallArcs(data)
	if len(arcs) != 1 {
		t.Errorf("expected 1 fall arc, got %d", len(arcs))
	}
}

func TestDetectRivalryArcs(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{ID: "bot1", Name: "SwarmBot"},
			{ID: "bot2", Name: "HunterBot"},
		},
		Matches: []MatchData{
			{ID: "m1", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 25, 12, 0, 0, 0, time.UTC)},
			{ID: "m2", Participants: []ParticipantData{
				{BotID: "bot1", Won: false},
				{BotID: "bot2", Won: true},
			}, PlayedAt: time.Date(2024, 3, 26, 12, 0, 0, 0, time.UTC)},
			{ID: "m3", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 27, 12, 0, 0, 0, time.UTC)},
			{ID: "m4", Participants: []ParticipantData{
				{BotID: "bot1", Won: false},
				{BotID: "bot2", Won: true},
			}, PlayedAt: time.Date(2024, 3, 28, 12, 0, 0, 0, time.UTC)},
			{ID: "m5", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)},
		},
	}

	arcs := detectRivalryArcs(data)
	if len(arcs) == 0 {
		t.Error("expected at least 1 rivalry arc with 5 matches between bots")
	}
}

// Mock LLM client for testing
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) GenerateNarrative(ctx context.Context, req NarrativeRequest) (headline, narrative string, err error) {
	if m.err != nil {
		return "", "", m.err
	}
	return "Test Headline", m.response, nil
}

func TestGenerateLLMChronicle_Success(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{ID: "bot1", Name: "TestBot", Rating: 1500},
		},
	}

	arc := StoryArc{
		Type:        ArcRise,
		BotID:       "bot1",
		BotName:     "TestBot",
		RatingStart: 1200,
		RatingEnd:   1500,
	}

	// Test with nil LLM client (should fall back to template)
	post := generateTemplateChronicle(arc, data)
	if post.Title == "" {
		t.Error("expected non-empty title from template chronicle")
	}
	if !strings.Contains(post.BodyMarkdown, "TestBot") {
		t.Error("expected chronicle to mention TestBot")
	}
}

func TestGenerateBlogPost(t *testing.T) {
	dateStr := "2024-03-29"
	post := BlogPost{
		Slug:         "test-post",
		Title:        "Test Post",
		PublishedAt:  dateStr,
		Date:         dateStr,
		Type:         "chronicle",
		BodyMarkdown: "# Test\n\nContent here.",
		ContentMd:    "# Test\n\nContent here.",
		Summary:      "Test summary",
		Tags:         []string{"test"},
	}

	if post.Slug != "test-post" {
		t.Errorf("unexpected slug: %s", post.Slug)
	}
	if post.PublishedAt != dateStr {
		t.Errorf("unexpected published_at: %s", post.PublishedAt)
	}
	if post.BodyMarkdown == "" {
		t.Error("expected non-empty body_markdown")
	}

	if post.Slug != "test-post" {
		t.Errorf("unexpected slug: %s", post.Slug)
	}
	if len(post.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(post.Tags))
	}
}

func TestShouldGenerateMetaReport_NoDir(t *testing.T) {
	// Non-existent directory should trigger generation
	tmpDir := t.TempDir()
	postsDir := tmpDir + "/nonexistent"

	result := shouldGenerateMetaReport(postsDir)
	if !result {
		t.Error("should generate when posts directory does not exist")
	}
}

func TestShouldGenerateMetaReport_EmptyDir(t *testing.T) {
	// Empty directory should trigger generation
	postsDir := t.TempDir()

	result := shouldGenerateMetaReport(postsDir)
	if !result {
		t.Error("should generate when no meta reports exist")
	}
}

func TestShouldGenerateMetaReport_RecentStateFile(t *testing.T) {
	postsDir := t.TempDir()

	// Write a recent state file (today)
	stateFile := postsDir + "/.last-meta-report"
	recentTime := time.Now().UTC().Add(-1 * 24 * time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(stateFile, []byte(recentTime), 0644); err != nil {
		t.Fatal(err)
	}

	// Not Monday and less than 7 days — should NOT generate
	result := shouldGenerateMetaReport(postsDir)
	if time.Now().UTC().Weekday() == time.Monday {
		t.Skip("test only valid on non-Mondays")
	}
	if result {
		t.Error("should NOT generate when last report was < 7 days ago")
	}
}

func TestShouldGenerateMetaReport_OldStateFile(t *testing.T) {
	postsDir := t.TempDir()

	// Write an old state file (10 days ago)
	stateFile := postsDir + "/.last-meta-report"
	oldTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(stateFile, []byte(oldTime), 0644); err != nil {
		t.Fatal(err)
	}

	result := shouldGenerateMetaReport(postsDir)
	if !result {
		t.Error("should generate when last report was > 7 days ago")
	}
}

func TestShouldGenerateMetaReport_FallbackToFileScan(t *testing.T) {
	postsDir := t.TempDir()

	// Create a meta report file (no state file — tests backward compat fallback)
	metaFile := postsDir + "/meta-week-13-2024-03-25.json"
	if err := os.WriteFile(metaFile, []byte(`{"slug":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Set its mod time to 8 days ago
	oldTime := time.Now().UTC().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(metaFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	result := shouldGenerateMetaReport(postsDir)
	if !result {
		t.Error("should generate when last meta file is > 7 days old")
	}
}

func TestRecordMetaReportGenerated(t *testing.T) {
	postsDir := t.TempDir()

	recordMetaReportGenerated(postsDir)

	stateFile := postsDir + "/.last-meta-report"
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("state file contains invalid timestamp: %v", err)
	}

	// Should be within the last few seconds
	if time.Since(parsed) > 5*time.Second {
		t.Errorf("state file timestamp too old: %v", parsed)
	}
}

func TestBuildSpotlightPrompt(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{ID: "bot1", Name: "TopBot", Rating: 1800, MatchesPlayed: 50, MatchesWon: 35, Archetype: "swarm"},
			{ID: "bot2", Name: "SecondBot", Rating: 1700, MatchesPlayed: 40, MatchesWon: 20, Archetype: "hunter"},
		},
		Matches: []MatchData{
			{ID: "m1", PlayedAt: time.Date(2024, 3, 28, 12, 0, 0, 0, time.UTC)},
		},
	}

	movers := []eloMover{
		{BotName: "RisingBot", OldRating: 1200, NewRating: 1450, Delta: 250, Archetype: "gatherer", MatchesWon: 8, MatchesLost: 2},
	}
	strats := []strategyCount{
		{Archetype: "swarm", Count: 10, AvgRating: 1600, InTop20: 5},
	}
	bestMatch := &notableMatch{
		MatchID:     "m_best",
		Description: "TopBot vs SecondBot",
		Score:       "3-2",
		TurnCount:   287,
	}

	rivalries := []RivalryData{
		{BotAID: "bot1", BotBID: "bot2", BotAWins: 5, BotBWins: 4, TotalMatches: 9},
	}
	prompt := buildSpotlightPrompt(data, movers, strats, bestMatch, nil, data.Bots[:2], rivalries)

	if !strings.Contains(prompt, "Counter-Strategy Spotlight") {
		t.Error("prompt should mention Counter-Strategy Spotlight")
	}
	if !strings.Contains(prompt, "TopBot vs SecondBot") {
		t.Error("prompt should contain rivalry matchup")
	}
	if !strings.Contains(prompt, "TopBot") {
		t.Error("prompt should contain top bot name")
	}
	if !strings.Contains(prompt, "RisingBot") {
		t.Error("prompt should contain ELO mover name")
	}
	if !strings.Contains(prompt, "swarm") {
		t.Error("prompt should contain strategy archetype")
	}
	if !strings.Contains(prompt, "m_best") {
		t.Error("prompt should reference best match")
	}
}

func TestBuildEvolutionDeepDivePrompt(t *testing.T) {
	data := &IndexData{
		GeneratedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC),
		Bots: []BotData{
			{ID: "evo1", Name: "evo-go-g31", Rating: 1580, Evolved: true},
		},
		TopPredictors: []PredictorStats{
			{PredictorID: "p1", Correct: 15, Incorrect: 3, BestStreak: 10},
		},
	}

	evoHighlights := []evolutionHighlight{
		{BotName: "evo-go-g31", Rating: 1580, Island: "go", Generation: 31, WeekMatches: 10, WeekWins: 7, Archetype: "hybrid"},
	}
	rivalries := []RivalryData{
		{BotAID: "evo1", BotBID: "bot2", BotAWins: 5, BotBWins: 4, TotalMatches: 9},
	}

	prompt := buildEvolutionDeepDivePrompt(data, evoHighlights, rivalries, data.TopPredictors, nil)

	if !strings.Contains(prompt, "Evolution Deep Dive") {
		t.Error("prompt should mention Evolution Deep Dive")
	}
	if !strings.Contains(prompt, "evo-go-g31") {
		t.Error("prompt should contain evolved bot name")
	}
	if !strings.Contains(prompt, "go") {
		t.Error("prompt should contain island name")
	}
}

func TestSpliceLLMContent(t *testing.T) {
	template := `# Week 13 Meta Report

## Top 5 Leaderboard

| Rank | Bot | Rating |
|------|-----|--------|
| 1 | Bot1 | 1800 |

## Evolution Highlights

No evolved bots active this week.

## Looking Ahead

The meta continues to evolve.`

	result := spliceLLMContent(template, "Swarm tactics are rising.", "evo-go-g31 shows promise.")

	if !strings.Contains(result, "## Counter-Strategy Spotlight") {
		t.Error("should contain Counter-Strategy Spotlight section")
	}
	if !strings.Contains(result, "Swarm tactics are rising.") {
		t.Error("should contain spotlight content")
	}
	if !strings.Contains(result, "### Evolution Deep Dive") {
		t.Error("should contain Evolution Deep Dive section")
	}
	if !strings.Contains(result, "evo-go-g31 shows promise.") {
		t.Error("should contain evolution narrative")
	}
	// Verify ordering: spotlight before Evolution Highlights, deep dive before Looking Ahead
	spotlightIdx := strings.Index(result, "## Counter-Strategy Spotlight")
	evoIdx := strings.Index(result, "## Evolution Highlights")
	deepDiveIdx := strings.Index(result, "### Evolution Deep Dive")
	lookingAheadIdx := strings.Index(result, "## Looking Ahead")

	if spotlightIdx >= evoIdx {
		t.Error("Counter-Strategy Spotlight should appear before Evolution Highlights")
	}
	if deepDiveIdx >= lookingAheadIdx {
		t.Error("Evolution Deep Dive should appear before Looking Ahead")
	}
}

func TestSpliceLLMContent_SpotlightOnly(t *testing.T) {
	template := `# Report

## Looking Ahead

The end.`

	result := spliceLLMContent(template, "Analysis text.", "")

	if !strings.Contains(result, "## Counter-Strategy Spotlight") {
		t.Error("should contain spotlight section")
	}
	if strings.Contains(result, "### Evolution Deep Dive") {
		t.Error("should NOT contain deep dive when evoNarrative is empty")
	}
}

func TestSpliceLLMContent_NoInsertionPoints(t *testing.T) {
	template := "# Simple Report\n\nSome content."

	result := spliceLLMContent(template, "Extra analysis.", "Evo details.")

	if !strings.Contains(result, "## Counter-Strategy Spotlight") {
		t.Error("should append spotlight when no insertion point found")
	}
	if !strings.Contains(result, "### Evolution Deep Dive") {
		t.Error("should append deep dive when no insertion point found")
	}
}

func TestExtractFirstSentence(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Swarm tactics dominate the meta. Other bots struggle.", "Swarm tactics dominate the meta."},
		{"Short.", "Short."},
		{"No sentence end", "No sentence end"},
		{"Multiple? Yes! Indeed.", "Multiple?"},
	}

	for _, tc := range tests {
		result := extractFirstSentence(tc.input)
		if result != tc.expected {
			t.Errorf("extractFirstSentence(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestCountWeeklyMatches(t *testing.T) {
	now := time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)
	data := &IndexData{
		GeneratedAt: now,
		Matches: []MatchData{
			{ID: "m1", PlayedAt: now.Add(-1 * 24 * time.Hour)},
			{ID: "m2", PlayedAt: now.Add(-3 * 24 * time.Hour)},
			{ID: "m3", PlayedAt: now.Add(-10 * 24 * time.Hour)}, // outside week
			{ID: "m4", PlayedAt: now.Add(-5 * 24 * time.Hour)},
		},
	}

	count := countWeeklyMatches(data)
	if count != 3 {
		t.Errorf("countWeeklyMatches: got %d, want 3", count)
	}
}

func TestNonEmpty(t *testing.T) {
	if nonEmpty("", "fallback") != "fallback" {
		t.Error("empty string should return fallback")
	}
	if nonEmpty("value", "fallback") != "value" {
		t.Error("non-empty string should return itself")
	}
}
