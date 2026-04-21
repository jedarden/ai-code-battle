package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// ── shouldEnrich tests ─────────────────────────────────────────────────────

func TestShouldEnrich_BackAndForth(t *testing.T) {
	m := MatchData{
		ID:        "m_backforth",
		WinnerID:  "bot_a",
		TurnCount: 250,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 5, Won: true, PreMatchRating: 1500},
			{BotID: "bot_b", Score: 4, Won: false, PreMatchRating: 1500},
		},
	}
	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_a", Rating: 1500},
			{ID: "bot_b", Rating: 1500},
		},
	}

	criteria, ok := shouldEnrich(m, data)
	if !ok {
		t.Fatal("expected match to be selected for enrichment")
	}
	found := false
	for _, c := range criteria {
		if c == "back_and_forth" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected back_and_forth criterion, got %v", criteria)
	}
}

func TestShouldEnrich_Upset(t *testing.T) {
	m := MatchData{
		ID:        "m_upset",
		WinnerID:  "bot_weak",
		TurnCount: 300,
		Participants: []ParticipantData{
			{BotID: "bot_weak", Score: 5, Won: true, PreMatchRating: 1200},
			{BotID: "bot_strong", Score: 2, Won: false, PreMatchRating: 1600},
		},
	}
	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_weak", Rating: 1200},
			{ID: "bot_strong", Rating: 1600},
		},
	}

	criteria, ok := shouldEnrich(m, data)
	if !ok {
		t.Fatal("expected upset match to be selected for enrichment")
	}

	upsetFound := false
	for _, c := range criteria {
		if c == "upset_400" {
			upsetFound = true
		}
	}
	if !upsetFound {
		t.Errorf("expected upset_400 criterion, got %v", criteria)
	}
}

func TestShouldEnrich_EvolutionMilestone(t *testing.T) {
	// getBotRank returns index+1 in the Bots slice, so evo_bot at index 2 = rank 3
	m := MatchData{
		ID:        "m_evomilestone",
		WinnerID:  "evo_bot",
		TurnCount: 200,
		Participants: []ParticipantData{
			{BotID: "evo_bot", Score: 5, Won: true, PreMatchRating: 1700},
			{BotID: "bot_other", Score: 3, Won: false, PreMatchRating: 1650},
		},
	}
	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_top1", Rating: 1800},
			{ID: "bot_top2", Rating: 1750},
			{ID: "evo_bot", Rating: 1700, Evolved: true},
			{ID: "bot_other", Rating: 1650},
		},
	}

	criteria, ok := shouldEnrich(m, data)
	if !ok {
		t.Fatal("expected evolution milestone match to be selected")
	}

	found := false
	for _, c := range criteria {
		if c == "evolution_milestone" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected evolution_milestone criterion, got %v", criteria)
	}
}

func TestShouldEnrich_HighInterest(t *testing.T) {
	m := MatchData{
		ID:        "m_interesting",
		WinnerID:  "bot_a",
		TurnCount: 450,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 5, Won: true, PreMatchRating: 1800},
			{BotID: "bot_b", Score: 4, Won: false, PreMatchRating: 1700},
		},
	}
	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_a", Rating: 1800},
			{ID: "bot_b", Rating: 1700},
		},
	}

	_, ok := shouldEnrich(m, data)
	if !ok {
		t.Fatal("expected high interest match to be selected")
	}
}

func TestShouldEnrich_BoringMatchNotSelected(t *testing.T) {
	m := MatchData{
		ID:        "m_trulyboring",
		WinnerID:  "bot_a",
		TurnCount: 50,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 10, Won: true, PreMatchRating: 1500},
			{BotID: "bot_b", Score: 8, Won: false, PreMatchRating: 1510},
		},
	}
	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_a", Rating: 1500},
			{ID: "bot_b", Rating: 1510},
		},
	}

	_, ok := shouldEnrich(m, data)
	if ok {
		t.Error("expected boring match to not be selected for enrichment")
	}
}

func TestShouldEnrich_NoWinner(t *testing.T) {
	m := MatchData{
		ID:        "m_nowinner",
		WinnerID:  "",
		TurnCount: 300,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 5},
			{BotID: "bot_b", Score: 5},
		},
	}
	data := &IndexData{}

	_, ok := shouldEnrich(m, data)
	if ok {
		t.Error("expected match with no winner to not be selected")
	}
}

func TestShouldEnrich_TooFewParticipants(t *testing.T) {
	m := MatchData{
		ID:        "m_onebot",
		WinnerID:  "bot_a",
		TurnCount: 300,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 5, Won: true},
		},
	}
	data := &IndexData{}

	_, ok := shouldEnrich(m, data)
	if ok {
		t.Error("expected match with < 2 participants to not be selected")
	}
}

// ── parseCommentaryResponse tests ──────────────────────────────────────────

func TestParseCommentaryResponse(t *testing.T) {
	input := "1|setup|The bots face off on a 60x60 grid\n" +
		"42|action|First contact near the central energy cluster\n" +
		"87|climax|SwarmBot breaks through the eastern corridor\n" +
		"200|reaction|GathererBot attempts to regroup\n" +
		"499|denouement|The match ends with a decisive victory"

	entries := parseCommentaryResponse(input)
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	if entries[0].Turn != 1 {
		t.Errorf("entry 0 turn: got %d, want 1", entries[0].Turn)
	}
	if entries[0].Type != "setup" {
		t.Errorf("entry 0 type: got %q, want setup", entries[0].Type)
	}
	if entries[0].Text != "The bots face off on a 60x60 grid" {
		t.Errorf("entry 0 text: got %q", entries[0].Text)
	}

	if entries[2].Type != "climax" {
		t.Errorf("entry 2 type: got %q, want climax", entries[2].Type)
	}
	if entries[4].Turn != 499 {
		t.Errorf("entry 4 turn: got %d, want 499", entries[4].Turn)
	}
	if entries[4].Type != "denouement" {
		t.Errorf("entry 4 type: got %q, want denouement", entries[4].Type)
	}
}

func TestParseCommentaryResponse_SkipsInvalid(t *testing.T) {
	input := "# This is a comment\n" +
		"1|setup|Valid entry\n" +
		"\n" +
		"INVALID_LINE\n" +
		"42|action|Another valid entry\n" +
		"bad_turn|action|Bad turn number\n" +
		"100|invalid_type|Defaults to action"

	entries := parseCommentaryResponse(input)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (skipping comment, blank, invalid line, bad turn), got %d", len(entries))
	}

	if entries[0].Turn != 1 {
		t.Errorf("entry 0 turn: got %d, want 1", entries[0].Turn)
	}
	if entries[1].Turn != 42 {
		t.Errorf("entry 1 turn: got %d, want 42", entries[1].Turn)
	}
	if entries[2].Turn != 100 {
		t.Errorf("entry 2 turn: got %d, want 100", entries[2].Turn)
	}
	if entries[2].Type != "action" {
		t.Errorf("entry 2 type: got %q, want action (default for invalid type)", entries[2].Type)
	}
}

func TestParseCommentaryResponse_Empty(t *testing.T) {
	entries := parseCommentaryResponse("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(entries))
	}
}

func TestParseCommentaryResponse_SlashSlashComments(t *testing.T) {
	input := "// Another kind of comment\n10|action|Real entry"

	entries := parseCommentaryResponse(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Turn != 10 {
		t.Errorf("entry 0 turn: got %d, want 10", entries[0].Turn)
	}
}

// ── countWinProbCrossings tests ────────────────────────────────────────────

func TestCountWinProbCrossings_NoCrossings(t *testing.T) {
	wp := [][]float64{
		{0.8, 0.2},
		{0.85, 0.15},
		{0.9, 0.1},
		{0.88, 0.12},
	}
	if n := countWinProbCrossings(wp); n != 0 {
		t.Errorf("expected 0 crossings, got %d", n)
	}
}

func TestCountWinProbCrossings_ThreeCrossings(t *testing.T) {
	wp := [][]float64{
		{0.7, 0.3},
		{0.4, 0.6}, // crossing 1
		{0.3, 0.7},
		{0.6, 0.4}, // crossing 2
		{0.8, 0.2},
		{0.4, 0.6}, // crossing 3
		{0.3, 0.7},
	}
	if n := countWinProbCrossings(wp); n != 3 {
		t.Errorf("expected 3 crossings, got %d", n)
	}
}

func TestCountWinProbCrossings_Empty(t *testing.T) {
	if n := countWinProbCrossings(nil); n != 0 {
		t.Errorf("expected 0 for nil, got %d", n)
	}
	if n := countWinProbCrossings([][]float64{}); n != 0 {
		t.Errorf("expected 0 for empty, got %d", n)
	}
	if n := countWinProbCrossings([][]float64{{0.5, 0.5}}); n != 0 {
		t.Errorf("expected 0 for single entry, got %d", n)
	}
}

func TestCountWinProbCrossings_Exactly50(t *testing.T) {
	wp := [][]float64{
		{0.4, 0.6},
		{0.5, 0.5}, // crosses up to >= 0.5
	}
	if n := countWinProbCrossings(wp); n != 1 {
		t.Errorf("expected 1 crossing at 0.5 boundary, got %d", n)
	}
}

// ── buildCommentaryPrompt tests ────────────────────────────────────────────

func TestBuildCommentaryPrompt(t *testing.T) {
	m := MatchData{
		ID:        "m_test",
		WinnerID:  "bot_a",
		TurnCount: 300,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 5, Won: true, PreMatchRating: 1800},
			{BotID: "bot_b", Score: 3, Won: false, PreMatchRating: 1600},
		},
	}

	replay := makeTestReplayStruct("SwarmBot", "HunterBot", 0, "turn_limit", 300, []int{5, 3})

	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_a", Name: "SwarmBot", Rating: 1800},
			{ID: "bot_b", Name: "HunterBot", Rating: 1600},
		},
	}

	prompt := buildCommentaryPrompt(m, replay, []string{"upset_200", "back_and_forth"}, data)

	checks := []string{
		"AI Code Battle commentator",
		"TURN|TYPE|TEXT",
		"SwarmBot vs HunterBot",
		"turn_limit",
		"300 turns",
		"upset_200",
		"back_and_forth",
		"SwarmBot",
		"HunterBot",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing expected substring %q", check)
		}
	}
}

func TestBuildCommentaryPrompt_WithCriticalMoments(t *testing.T) {
	m := MatchData{
		ID:        "m_cm",
		WinnerID:  "bot_a",
		TurnCount: 200,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 4, Won: true, PreMatchRating: 1500},
			{BotID: "bot_b", Score: 2, Won: false, PreMatchRating: 1500},
		},
	}

	replay := makeTestReplayStruct("BotA", "BotB", 0, "elimination", 200, []int{4, 2})

	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_a", Name: "BotA", Rating: 1500},
			{ID: "bot_b", Name: "BotB", Rating: 1500},
		},
	}

	prompt := buildCommentaryPrompt(m, replay, []string{"high_interest"}, data)

	if !strings.Contains(prompt, "Critical moments:") {
		t.Error("prompt missing critical moments header")
	}
	if !strings.Contains(prompt, "Turn 87") {
		t.Error("prompt missing critical moment turn 87")
	}
	if !strings.Contains(prompt, "6 bots killed") {
		t.Error("prompt missing critical moment description")
	}
	if !strings.Contains(prompt, "Turn 150") {
		t.Error("prompt missing critical moment turn 150")
	}
}

func TestBuildCommentaryPrompt_WithWinProb(t *testing.T) {
	m := MatchData{
		ID:        "m_wp",
		WinnerID:  "bot_a",
		TurnCount: 100,
		Participants: []ParticipantData{
			{BotID: "bot_a", Score: 3, Won: true, PreMatchRating: 1500},
			{BotID: "bot_b", Score: 2, Won: false, PreMatchRating: 1500},
		},
	}

	replay := makeTestReplayStruct("BotA", "BotB", 0, "turn_limit", 100, []int{3, 2})
	replay.WinProb = [][]float64{
		{0.5, 0.5},
		{0.3, 0.7},
		{0.6, 0.4},
		{0.2, 0.8},
		{0.8, 0.2},
	}

	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_a", Name: "BotA", Rating: 1500},
			{ID: "bot_b", Name: "BotB", Rating: 1500},
		},
	}

	prompt := buildCommentaryPrompt(m, replay, []string{"back_and_forth"}, data)

	if !strings.Contains(prompt, "crossed 0.5:") {
		t.Error("prompt missing win prob crossings info")
	}
	if !strings.Contains(prompt, "Biggest swing:") {
		t.Error("prompt missing biggest swing info")
	}
}

// ── EnrichedCommentary JSON round-trip ─────────────────────────────────────

func TestEnrichedCommentaryJSON(t *testing.T) {
	comm := &EnrichedCommentary{
		MatchID:   "m_test",
		Generated: "2026-04-21T12:00:00Z",
		Criteria:  []string{"upset_300", "back_and_forth"},
		Entries: []CommentaryEntry{
			{Turn: 1, Text: "Opening moves", Type: "setup"},
			{Turn: 87, Text: "Major engagement", Type: "climax"},
			{Turn: 499, Text: "Match concludes", Type: "denouement"},
		},
	}

	data, err := json.Marshal(comm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed EnrichedCommentary
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.MatchID != "m_test" {
		t.Errorf("match_id: got %q, want m_test", parsed.MatchID)
	}
	if len(parsed.Entries) != 3 {
		t.Fatalf("entries: got %d, want 3", len(parsed.Entries))
	}
	if parsed.Entries[1].Turn != 87 {
		t.Errorf("entry 1 turn: got %d, want 87", parsed.Entries[1].Turn)
	}
	if parsed.Entries[1].Type != "climax" {
		t.Errorf("entry 1 type: got %q, want climax", parsed.Entries[1].Type)
	}
	if len(parsed.Criteria) != 2 {
		t.Errorf("criteria: got %d, want 2", len(parsed.Criteria))
	}
}

// ── enrichReplays integration test (LLM nil) ───────────────────────────────

func TestEnrichReplays_NilLLM(t *testing.T) {
	ctx := context.Background()
	data := &IndexData{
		Matches: []MatchData{
			{
				ID:        "m_test",
				WinnerID:  "bot_a",
				TurnCount: 300,
				Participants: []ParticipantData{
					{BotID: "bot_a", Score: 3, Won: true, PreMatchRating: 1500},
					{BotID: "bot_b", Score: 2, Won: false, PreMatchRating: 1500},
				},
			},
		},
	}

	err := enrichReplays(ctx, data, &Config{}, nil)
	if err != nil {
		t.Errorf("enrichReplays with nil LLM should not error, got: %v", err)
	}
}

// ── isEvolved helper test ──────────────────────────────────────────────────

func TestIsEvolved(t *testing.T) {
	data := &IndexData{
		Bots: []BotData{
			{ID: "bot_human", Evolved: false},
			{ID: "bot_evo", Evolved: true},
		},
	}

	if isEvolved("bot_human", data) {
		t.Error("human bot should not be marked as evolved")
	}
	if !isEvolved("bot_evo", data) {
		t.Error("evolved bot should be marked as evolved")
	}
	if isEvolved("bot_missing", data) {
		t.Error("missing bot should not be marked as evolved")
	}
}

// ── makeTestReplayStruct creates the anonymous struct used by buildCommentaryPrompt ─

func makeTestReplayStruct(p0Name, p1Name string, winner int, reason string, turns int, scores []int) struct {
	WinProb         [][]float64 `json:"win_prob"`
	CriticalMoments []struct {
		Turn        int     `json:"turn"`
		Delta       float64 `json:"delta"`
		Description string  `json:"description"`
	} `json:"critical_moments"`
	Result struct {
		Winner int    `json:"winner"`
		Reason string `json:"reason"`
		Turns  int    `json:"turns"`
		Scores []int  `json:"scores"`
	} `json:"result"`
	Players []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"players"`
	Turns []struct {
		Turn   int `json:"turn"`
		Events []struct {
			Type    string `json:"type"`
			Turn    int    `json:"turn"`
			Details any    `json:"details"`
		} `json:"events"`
		Scores []int `json:"scores"`
	} `json:"turns"`
} {
	var r struct {
		WinProb         [][]float64 `json:"win_prob"`
		CriticalMoments []struct {
			Turn        int     `json:"turn"`
			Delta       float64 `json:"delta"`
			Description string  `json:"description"`
		} `json:"critical_moments"`
		Result struct {
			Winner int    `json:"winner"`
			Reason string `json:"reason"`
			Turns  int    `json:"turns"`
			Scores []int  `json:"scores"`
		} `json:"result"`
		Players []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"players"`
		Turns []struct {
			Turn   int `json:"turn"`
			Events []struct {
				Type    string `json:"type"`
				Turn    int    `json:"turn"`
				Details any    `json:"details"`
			} `json:"events"`
			Scores []int `json:"scores"`
		} `json:"turns"`
	}

	r.Players = []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{{ID: 0, Name: p0Name}, {ID: 1, Name: p1Name}}
	r.Result.Winner = winner
	r.Result.Reason = reason
	r.Result.Turns = turns
	r.Result.Scores = scores
	r.CriticalMoments = []struct {
		Turn        int     `json:"turn"`
		Delta       float64 `json:"delta"`
		Description string  `json:"description"`
	}{
		{Turn: 87, Delta: 0.22, Description: "6 bots killed in eastern engagement"},
		{Turn: 150, Delta: -0.31, Description: "Core captured by " + p0Name},
	}

	return r
}
