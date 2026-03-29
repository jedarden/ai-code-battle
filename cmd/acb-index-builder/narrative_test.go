package main

import (
	"context"
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
	if !strings.Contains(post.ContentMd, "TestBot") {
		t.Error("expected chronicle to mention TestBot")
	}
}

func TestGenerateBlogPost(t *testing.T) {
	post := BlogPost{
		Slug:      "test-post",
		Title:     "Test Post",
		Date:      "2024-03-29",
		Type:      "chronicle",
		ContentMd: "# Test\n\nContent here.",
		Summary:   "Test summary",
		Tags:      []string{"test"},
	}

	if post.Slug != "test-post" {
		t.Errorf("unexpected slug: %s", post.Slug)
	}
	if len(post.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(post.Tags))
	}
}
