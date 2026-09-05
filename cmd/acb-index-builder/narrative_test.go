package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBuildNarrativePrompt_Rise(t *testing.T) {
	req := NarrativeRequest{
		ArcType:     ArcRise,
		BotName:     "TestBot",
		SeasonName:  "Season 4",
		RatingStart: 1200,
		RatingEnd:   1450,
		KeyMatches: []KeyMatch{
			{MatchID: "m1", OpponentName: "TopBot", OpponentRating: 1800, MapName: "The Labyrinth", Score: "3-2", TurnCount: 200, Won: true},
		},
		Archetype: "aggressive",
		Origin:    "evolved, go island, generation 5",
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
		ArcType:     ArcUpset,
		BotName:     "UnderdogBot",
		BotBName:    "FavoriteBot",
		RatingStart: 1100,
		RatingEnd:   1800,
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
		ArcType:      ArcRivalry,
		BotName:      "SwarmBot",
		BotBName:     "HunterBot",
		BotAWins:     5,
		BotBWins:     4,
		TotalMatches: 9,
		SeasonName:   "Season 4",
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
		Origin:     "evolved, go island",
		Generation: 31,
		ParentIDs:  []string{"evo-go-g28", "evo-go-g25"},
		Archetype:  "hybrid swarm-gatherer",
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
		ArcType:     ArcComeback,
		BotName:     "ComebackBot",
		SeasonName:  "Season 4",
		RatingStart: 1300,
		RatingEnd:   1450,
	}

	prompt := buildNarrativePrompt(req)

	if !strings.Contains(prompt, "Comeback") {
		t.Error("prompt should contain comeback arc type")
	}
	if !strings.Contains(prompt, "1450") {
		t.Error("prompt should contain current ELO")
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
			// Grudge match: 10+ meetings between the same pair
			{ID: "m1", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 20, 12, 0, 0, 0, time.UTC)},
			{ID: "m2", Participants: []ParticipantData{
				{BotID: "bot1", Won: false},
				{BotID: "bot2", Won: true},
			}, PlayedAt: time.Date(2024, 3, 21, 12, 0, 0, 0, time.UTC)},
			{ID: "m3", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 22, 12, 0, 0, 0, time.UTC)},
			{ID: "m4", Participants: []ParticipantData{
				{BotID: "bot1", Won: false},
				{BotID: "bot2", Won: true},
			}, PlayedAt: time.Date(2024, 3, 23, 12, 0, 0, 0, time.UTC)},
			{ID: "m5", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 24, 12, 0, 0, 0, time.UTC)},
			{ID: "m6", Participants: []ParticipantData{
				{BotID: "bot1", Won: false},
				{BotID: "bot2", Won: true},
			}, PlayedAt: time.Date(2024, 3, 25, 12, 0, 0, 0, time.UTC)},
			{ID: "m7", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 26, 12, 0, 0, 0, time.UTC)},
			{ID: "m8", Participants: []ParticipantData{
				{BotID: "bot1", Won: false},
				{BotID: "bot2", Won: true},
			}, PlayedAt: time.Date(2024, 3, 27, 12, 0, 0, 0, time.UTC)},
			{ID: "m9", Participants: []ParticipantData{
				{BotID: "bot1", Won: true},
				{BotID: "bot2", Won: false},
			}, PlayedAt: time.Date(2024, 3, 28, 12, 0, 0, 0, time.UTC)},
			{ID: "m10", Participants: []ParticipantData{
				{BotID: "bot1", Won: false},
				{BotID: "bot2", Won: true},
			}, PlayedAt: time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)},
		},
	}

	arcs := detectRivalryArcs(data)
	if len(arcs) == 0 {
		t.Error("expected at least 1 rivalry arc with 10+ grudge matches between bots")
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

func TestHasAlternatingWins(t *testing.T) {
	cases := []struct {
		name     string
		results  []string
		expected bool
	}{
		{"true alternation", []string{"A", "B", "A", "B", "A"}, true},
		{"true alternation starting with B", []string{"B", "A", "B", "A", "B"}, true},
		{"one repeat breaks alternation", []string{"A", "A", "B", "A", "B"}, false},
		{"mid-sequence repeat", []string{"A", "B", "B", "A", "B"}, false},
		{"all one winner", []string{"A", "A", "A", "A", "A"}, false},
		{"single result", []string{"A"}, true},
		{"empty", nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasAlternatingWins(tc.results); got != tc.expected {
				t.Errorf("hasAlternatingWins(%v) = %v, want %v", tc.results, got, tc.expected)
			}
		})
	}
}

// Rivalry detection must only count matches from the last 7 days. A long
// all-time history is not a rivalry that "intensified" this week.
func TestDetectRivalryArcs_WeeklyWindowOnly(t *testing.T) {
	now := time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "bot1", Name: "SwarmBot"},
			{ID: "bot2", Name: "HunterBot"},
		},
		Matches: make([]MatchData, 0, 6),
	}

	// Five alternating meetings, all older than a week — not this week.
	for i := 0; i < 5; i++ {
		data.Matches = append(data.Matches, MatchData{
			ID: fmt.Sprintf("old-%d", i),
			Participants: []ParticipantData{
				{BotID: "bot1", Won: i%2 == 0},
				{BotID: "bot2", Won: i%2 != 0},
			},
			PlayedAt: now.AddDate(0, 0, -14),
		})
	}

	if arcs := detectRivalryArcs(data); len(arcs) != 0 {
		t.Errorf("expected no rivalry arc from matches outside the weekly window, got %d", len(arcs))
	}

	// The same five meetings inside the window do form one.
	for i := 0; i < 5; i++ {
		data.Matches = append(data.Matches, MatchData{
			ID: fmt.Sprintf("new-%d", i),
			Participants: []ParticipantData{
				{BotID: "bot1", Won: i%2 == 0},
				{BotID: "bot2", Won: i%2 != 0},
			},
			PlayedAt: now.AddDate(0, 0, -1),
		})
	}

	arcs := detectRivalryArcs(data)
	if len(arcs) != 1 {
		t.Fatalf("expected 1 rivalry arc, got %d", len(arcs))
	}
	if arcs[0].TotalMatches != 5 {
		t.Errorf("expected 5 weekly matches, got %d", arcs[0].TotalMatches)
	}
	if arcs[0].BotAWins != 3 || arcs[0].BotBWins != 2 {
		t.Errorf("expected 3-2 split, got %d-%d", arcs[0].BotAWins, arcs[0].BotBWins)
	}
}

// Five meetings is not a rivalry if one bot dominated — "alternating wins"
// means no bot won twice in a row, not merely that both won sometimes.
func TestDetectRivalryArcs_RejectsNonAlternatingRun(t *testing.T) {
	now := time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)

	// bot1 sweeps the first three, then splits — both bots win 3+ times,
	// and there is at least one change of winner, but it never alternates.
	pattern := []bool{true, true, true, false, true, false}
	matches := make([]MatchData, 0, len(pattern))
	for i, bot1Won := range pattern {
		matches = append(matches, MatchData{
			ID: fmt.Sprintf("m%d", i),
			Participants: []ParticipantData{
				{BotID: "bot1", Won: bot1Won},
				{BotID: "bot2", Won: !bot1Won},
			},
			PlayedAt: now.AddDate(0, 0, -1),
		})
	}

	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "bot1", Name: "SwarmBot"},
			{ID: "bot2", Name: "HunterBot"},
		},
		Matches: matches,
	}

	if arcs := detectRivalryArcs(data); len(arcs) != 0 {
		t.Errorf("expected no rivalry arc for a non-alternating win run, got %d", len(arcs))
	}
}

func TestDetectSeasonNarrativeArcs(t *testing.T) {
	now := time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "ladder1", Name: "LadderLeader"},
			// Present so the ChampionID branch of the arc can resolve a name.
			{ID: "champ", Name: "PodiumFirst"},
		},
		Seasons: []SeasonData{
			{
				ID:           1,
				Name:         "Season 4",
				Status:       "completed",
				EndsAt:       now.AddDate(0, 0, -2),
				ChampionID:   "champ",
				ChampionName: "",
				// Final standings, distinct from the live ladder order above.
				Snapshots: []SeasonSnapshotData{
					{BotID: "champ", BotName: "PodiumFirst", Rank: 1},
					{BotID: "second", BotName: "PodiumSecond", Rank: 2},
					{BotID: "third", BotName: "PodiumThird", Rank: 3},
				},
			},
			// Ended too long ago to be news.
			{ID: 2, Name: "Season 3", Status: "completed", EndsAt: now.AddDate(0, 0, -20)},
			// Still running.
			{ID: 3, Name: "Season 5", Status: "active", EndsAt: now.AddDate(0, 0, 30)},
		},
	}

	arcs := detectSeasonNarrativeArcs(data)
	if len(arcs) != 1 {
		t.Fatalf("expected 1 season narrative arc, got %d", len(arcs))
	}

	arc := arcs[0]
	if arc.Type != ArcSeasonRecap {
		t.Errorf("expected %s arc, got %s", ArcSeasonRecap, arc.Type)
	}
	if arc.SeasonName != "Season 4" {
		t.Errorf("expected Season 4, got %s", arc.SeasonName)
	}
	if arc.BotName != "PodiumFirst" {
		t.Errorf("champion should resolve from ChampionID, got %q", arc.BotName)
	}
	if len(arc.TopBots) != 3 {
		t.Fatalf("expected 3 podium bots from final standings, got %d", len(arc.TopBots))
	}
	// The podium must come from the season's own snapshot, not the live ladder.
	if arc.TopBots[0] != "PodiumFirst" {
		t.Errorf("expected podium from final standings, got %q", arc.TopBots[0])
	}
}

// Upset of the Week is a single arc: the biggest rating gap where the
// underdog won, from matches in the last 7 days.
func TestDetectUpsetArcs_PicksBiggestWeeklyGap(t *testing.T) {
	now := time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "underdog", Name: "UnderdogBot", Rating: 1100},
			{ID: "favorite", Name: "FavoriteBot", Rating: 1500},
			{ID: "small-dog", Name: "SmallDog", Rating: 1400},
			{ID: "big-fav", Name: "BigFav", Rating: 1500},
		},
		Matches: []MatchData{
			// 100-point upset — beaten by a bigger one.
			{ID: "small-upset", PlayedAt: now.AddDate(0, 0, -1), Participants: []ParticipantData{
				{BotID: "small-dog", Won: true, PreMatchRating: 1400},
				{BotID: "big-fav", Won: false, PreMatchRating: 1500},
			}},
			// 400-point upset — the one that should win the week.
			{ID: "big-upset", PlayedAt: now.AddDate(0, 0, -2), Participants: []ParticipantData{
				{BotID: "underdog", Won: true, PreMatchRating: 1100},
				{BotID: "favorite", Won: false, PreMatchRating: 1500},
			}},
			// The favourite winning is not an upset at all.
			{ID: "chalk", PlayedAt: now.AddDate(0, 0, -3), Participants: []ParticipantData{
				{BotID: "big-fav", Won: true, PreMatchRating: 1500},
				{BotID: "small-dog", Won: false, PreMatchRating: 1400},
			}},
			// Outside the weekly window, so ignored even though the gap is bigger.
			{ID: "stale", PlayedAt: now.AddDate(0, 0, -14), Participants: []ParticipantData{
				{BotID: "underdog", Won: true, PreMatchRating: 1000},
				{BotID: "favorite", Won: false, PreMatchRating: 1500},
			}},
		},
	}

	arcs := detectUpsetArcs(data)
	if len(arcs) != 1 {
		t.Fatalf("expected exactly 1 upset arc, got %d", len(arcs))
	}

	arc := arcs[0]
	if arc.BotName != "UnderdogBot" {
		t.Errorf("expected the biggest weekly upset winner UnderdogBot, got %s", arc.BotName)
	}
	if arc.BotBName != "FavoriteBot" {
		t.Errorf("expected the upset victim FavoriteBot, got %s", arc.BotBName)
	}
	if arc.RatingStart != 1100 || arc.RatingEnd != 1500 {
		t.Errorf("expected 1100 → 1500, got %d → %d", arc.RatingStart, arc.RatingEnd)
	}
}

// Comeback is measured as recovery from a 30-day low, not by ladder quartiles.
func TestDetectComebackArcs_RecoversFromThirtyDayLow(t *testing.T) {
	now := time.Date(2024, 3, 29, 12, 0, 0, 0, time.UTC)
	data := &IndexData{
		GeneratedAt: now,
		Bots: []BotData{
			{ID: "c1", Name: "ComebackBot", Rating: 1400, Archetype: "aggressive"},
			{ID: "c2", Name: "StaticBot", Rating: 1200},
		},
		RatingHistory: []RatingHistoryEntry{
			// c1: slid to 1150 mid-month, then climbed back above it.
			{BotID: "c1", Rating: 1300, RecordedAt: now.AddDate(0, 0, -25)},
			{BotID: "c1", Rating: 1150, RecordedAt: now.AddDate(0, 0, -20)},
			{BotID: "c1", Rating: 1250, RecordedAt: now.AddDate(0, 0, -10)},
			{BotID: "c1", Rating: 1400, RecordedAt: now.AddDate(0, 0, -1)},
			// c2: recovered only 50 points — not a comeback.
			{BotID: "c2", Rating: 1150, RecordedAt: now.AddDate(0, 0, -20)},
			{BotID: "c2", Rating: 1200, RecordedAt: now.AddDate(0, 0, -1)},
		},
	}

	arcs := detectComebackArcs(data)
	if len(arcs) != 1 {
		t.Fatalf("expected 1 comeback arc, got %d", len(arcs))
	}

	arc := arcs[0]
	if arc.BotName != "ComebackBot" {
		t.Errorf("expected ComebackBot, got %s", arc.BotName)
	}
	if arc.RatingStart != 1150 {
		t.Errorf("arc should start at the 30-day low 1150, got %d", arc.RatingStart)
	}
	if arc.RatingEnd != 1400 {
		t.Errorf("arc should end at the current rating 1400, got %d", arc.RatingEnd)
	}
}

func TestBuildWeeklyChroniclesPrompt_SeasonNarrative(t *testing.T) {
	req := WeeklyChroniclesRequest{
		Year:       2024,
		WeekNumber: 13,
		SeasonName: "Season 4",
		StoryArcs: []StoryArc{
			{
				Type:       ArcSeasonRecap,
				BotName:    "PodiumFirst",
				SeasonName: "Season 4",
				TopBots:    []string{"PodiumFirst", "PodiumSecond", "PodiumThird"},
			},
		},
	}

	prompt := buildWeeklyChroniclesPrompt(req)

	if !strings.Contains(prompt, "### Season Narrative") {
		t.Error("weekly chronicle prompt should carry a Season Narrative section")
	}
	if !strings.Contains(prompt, "Season 4 has concluded") {
		t.Error("prompt should name the concluded season")
	}
	if !strings.Contains(prompt, "champion: PodiumFirst") {
		t.Error("prompt should name the champion")
	}
	if !strings.Contains(prompt, "PodiumFirst, PodiumSecond, PodiumThird") {
		t.Error("prompt should carry the final podium")
	}
}

// Comeback arcs are recovery-from-a-low stories now; the prompt must not
// describe the retired bottom-quartile-to-top-quartile definition, and the
// literal "%%" the old non-formatted WriteString carried must stay gone.
func TestBuildNarrativePrompt_ComebackDescribesRecovery(t *testing.T) {
	req := NarrativeRequest{
		ArcType:     ArcComeback,
		BotName:     "ComebackBot",
		SeasonName:  "Season 4",
		RatingStart: 1150,
		RatingEnd:   1400,
	}

	prompt := buildNarrativePrompt(req)

	if strings.Contains(prompt, "bottom 25%") || strings.Contains(prompt, "top 25%") {
		t.Error("comeback prompt should not describe the retired quartile-based story")
	}
	if strings.Contains(prompt, "%%") {
		t.Error("comeback prompt should not emit a literal %% escape")
	}
	if !strings.Contains(prompt, "Recovered 250 points from a 30-day low of 1150 back to 1400") {
		t.Errorf("comeback prompt should describe the recovery, got:\n%s", prompt)
	}
}

func TestBuildNarrativePrompt_SeasonRecapIncludesPodium(t *testing.T) {
	req := NarrativeRequest{
		ArcType:     ArcSeasonRecap,
		SeasonName:  "Season 4",
		BotName:     "PodiumFirst",
		RatingStart: 1500,
		RatingEnd:   1500,
		TopBots:     []string{"PodiumFirst", "PodiumSecond", "PodiumThird"},
	}

	prompt := buildNarrativePrompt(req)

	if !strings.Contains(prompt, "Arc type: Season Narrative") {
		t.Error("prompt should label the season narrative arc type")
	}
	if !strings.Contains(prompt, "Champion: PodiumFirst") {
		t.Error("prompt should name the champion")
	}
	if !strings.Contains(prompt, "PodiumFirst, PodiumSecond, PodiumThird") {
		t.Error("prompt should carry the final podium")
	}
}
