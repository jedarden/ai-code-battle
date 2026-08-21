package llm

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		apiKey    string
		model     string
		wantURL   string
		wantModel string
	}{
		{
			name:      "full configuration",
			baseURL:   "https://api.example.com/v1",
			apiKey:    "test-key",
			model:     "gpt-4",
			wantURL:   "https://api.example.com/v1",
			wantModel: "gpt-4",
		},
		{
			name:      "default model",
			baseURL:   "https://api.openai.com/v1",
			apiKey:    "test-key",
			model:     "",
			wantURL:   "https://api.openai.com/v1",
			wantModel: "gpt-4o-mini",
		},
		{
			name:      "empty api key is allowed",
			baseURL:   "https://api.openai.com/v1",
			apiKey:    "",
			model:     "gpt-4",
			wantURL:   "https://api.openai.com/v1",
			wantModel: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.apiKey, tt.model)

			if client == nil {
				t.Fatal("NewClient returned nil")
			}
			if client.baseURL != tt.wantURL {
				t.Errorf("Expected baseURL %s, got %s", tt.wantURL, client.baseURL)
			}
			if client.apiKey != tt.apiKey {
				t.Errorf("Expected apiKey %s, got %s", tt.apiKey, client.apiKey)
			}
			if client.model != tt.wantModel {
				t.Errorf("Expected model %s, got %s", tt.wantModel, client.model)
			}
			if client.httpClient == nil {
				t.Error("Expected httpClient to be initialized")
			}
		})
	}
}

func TestNewClient_TrimTrailingSlash(t *testing.T) {
	client := NewClient("https://api.example.com/v1/", "key", "model")

	if client.baseURL != "https://api.example.com/v1" {
		t.Errorf("Expected baseURL to trim trailing slash, got %s", client.baseURL)
	}
}

func TestBuildPrompt(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	req := GenerateCommentaryRequest{
		MatchID:    "match-123",
		ReplayJSON: `{"turns": []}`,
		Metadata: MatchMetadata{
			Players: []PlayerInfo{
				{ID: 0, Name: "AlphaBot", Rating: 1500},
				{ID: 1, Name: "BetaBot", Rating: 1400},
			},
			MapSize:       "60x60",
			TurnCount:     150,
			Winner:        0,
			Condition:     "elimination",
			FinalScores:   []int{100, 95},
			IsUpset:       false,
			IsCloseFinish: true,
			IsFeatured:    true,
		},
		KeyMoments: []KeyMoment{
			{Turn: 50, Delta: 0.3, Description: "Core capture"},
		},
		MaxTokens:   3000,
		Temperature: 0.7,
	}

	prompt := client.buildPrompt(req)

	// Check prompt contains key sections
	requiredSubstrings := []string{
		"esports commentator",
		"AI Code Battle",
		"match-123",
		"60x60 grid",
		"150 turns",
		"Win Condition: elimination",
		"AlphaBot (Rating: 1500)",
		"BetaBot (Rating: 1400)",
		"Turn 50: Core capture",
		"CLOSE FINISH",
		"FEATURED",
		"key_moments",
		"summary",
		"narrative",
	}

	for _, substr := range requiredSubstrings {
		if !strings.Contains(prompt, substr) {
			t.Errorf("Prompt missing required substring '%s'", substr)
		}
	}
}

func TestBuildPrompt_WinnerMark(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	req := GenerateCommentaryRequest{
		Metadata: MatchMetadata{
			Players: []PlayerInfo{
				{ID: 0, Name: "AlphaBot", Rating: 1500},
				{ID: 1, Name: "BetaBot", Rating: 1400},
			},
			Winner: 1, // BetaBot wins
		},
	}

	prompt := client.buildPrompt(req)

	if !strings.Contains(prompt, "BetaBot (Rating: 1400) (winner)") {
		t.Error("Winner not marked in prompt")
	}
	if strings.Contains(prompt, "AlphaBot (winner)") {
		t.Error("Non-winner marked as winner")
	}
}

func TestBuildPrompt_Upset(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	req := GenerateCommentaryRequest{
		Metadata: MatchMetadata{
			Players: []PlayerInfo{
				{ID: 0, Name: "AlphaBot", Rating: 1300},
				{ID: 1, Name: "BetaBot", Rating: 1500},
			},
			Winner:  0,
			IsUpset: true,
		},
	}

	prompt := client.buildPrompt(req)

	if !strings.Contains(prompt, "UPSET") {
		t.Error("UPSET flag not in prompt")
	}
}

func TestBuildPrompt_NoKeyMoments(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	req := GenerateCommentaryRequest{
		Metadata: MatchMetadata{
			Players: []PlayerInfo{
				{ID: 0, Name: "AlphaBot", Rating: 1500},
			},
		},
		KeyMoments: []KeyMoment{},
	}

	prompt := client.buildPrompt(req)

	// Should not include key moments section if empty
	if strings.Contains(prompt, "### Key Moments") {
		t.Error("Key moments section included when empty")
	}
}

func TestBuildPrompt_Defaults(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	req := GenerateCommentaryRequest{
		Metadata: MatchMetadata{
			Players: []PlayerInfo{{ID: 0, Name: "Bot1", Rating: 1500}},
		},
		// MaxTokens and Temperature not set
	}

	prompt := client.buildPrompt(req)

	// Prompt should still be valid
	if prompt == "" {
		t.Error("buildPrompt returned empty string")
	}
}

func TestParseResponse_ValidJSON(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	raw := `{
		"key_moments": [
			{
				"turn": 50,
				"description": "Core capture",
				"significance": "high",
				"tags": ["combat", "core_capture"]
			}
		],
		"summary": "An intense match",
		"narrative": "Bot1 established early control"
	}`

	result, err := client.parseResponse(raw)
	if err != nil {
		t.Fatalf("parseResponse() error = %v", err)
	}

	if len(result.KeyMoments) != 1 {
		t.Errorf("parseResponse() returned %d key moments, want 1", len(result.KeyMoments))
	}
	if result.KeyMoments[0].Turn != 50 {
		t.Errorf("KeyMoment turn = %d, want 50", result.KeyMoments[0].Turn)
	}
	if result.Summary != "An intense match" {
		t.Errorf("Summary = %s, want 'An intense match'", result.Summary)
	}
	if result.Narrative != "Bot1 established early control" {
		t.Errorf("Narrative = %s, want 'Bot1 established early control'", result.Narrative)
	}
}

func TestParseResponse_MarkdownJSON(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	raw := "```json\n" +
		"{\n" +
		"	\"key_moments\": [],\n" +
		"	\"summary\": \"Test summary\",\n" +
		"	\"narrative\": \"Test narrative\"\n" +
		"}\n" +
		"```"

	result, err := client.parseResponse(raw)
	if err != nil {
		t.Fatalf("parseResponse() error = %v", err)
	}

	if result.Summary != "Test summary" {
		t.Errorf("Summary = %s, want 'Test summary'", result.Summary)
	}
}

func TestParseResponse_PlainMarkdown(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	raw := "```\n" +
		"{\n" +
		"	\"key_moments\": [],\n" +
		"	\"summary\": \"Test summary\",\n" +
		"	\"narrative\": \"Test narrative\"\n" +
		"}\n" +
		"```"

	result, err := client.parseResponse(raw)
	if err != nil {
		t.Fatalf("parseResponse() error = %v", err)
	}

	if result.Summary != "Test summary" {
		t.Errorf("Summary = %s, want 'Test summary'", result.Summary)
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	raw := "not valid json"

	_, err := client.parseResponse(raw)
	if err == nil {
		t.Error("parseResponse() expected error for invalid JSON, got nil")
	}
}

func TestParseResponse_MissingFields(t *testing.T) {
	client := NewClient("https://api.example.com/v1", "test-key", "gpt-4")

	raw := `{"summary": "Only summary"}`

	_, err := client.parseResponse(raw)
	if err != nil {
		t.Fatalf("parseResponse() error = %v (should accept partial JSON)", err)
	}
}

func TestGenerateCommentaryRequest_Defaults(t *testing.T) {
	req := GenerateCommentaryRequest{
		MatchID:    "test",
		ReplayJSON: "{}",
		Metadata: MatchMetadata{
			Players: []PlayerInfo{{ID: 0, Name: "Bot1", Rating: 1500}},
		},
		// MaxTokens and Temperature not set - should use defaults
	}

	// Test that defaults are applied
	if req.MaxTokens != 0 {
		t.Logf("MaxTokens will use default: %d", defaultMaxTokens)
	}
	if req.Temperature != 0 {
		t.Logf("Temperature will use default: %f", defaultTemperature)
	}
}

func TestKeyMomentCommentary(t *testing.T) {
	km := KeyMomentCommentary{
		Turn:         100,
		Description:  "Major engagement",
		Significance: "high",
		Tags:         []string{"combat", "turning_point"},
	}

	if km.Turn != 100 {
		t.Errorf("Turn = %d, want 100", km.Turn)
	}
	if km.Description != "Major engagement" {
		t.Errorf("Description = %s, want 'Major engagement'", km.Description)
	}
	if km.Significance != "high" {
		t.Errorf("Significance = %s, want 'high'", km.Significance)
	}
	if len(km.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(km.Tags))
	}
}

func TestPlayerInfo(t *testing.T) {
	p := PlayerInfo{
		ID:     1,
		Name:   "TestBot",
		Rating: 1400,
	}

	if p.ID != 1 {
		t.Errorf("ID = %d, want 1", p.ID)
	}
	if p.Name != "TestBot" {
		t.Errorf("Name = %s, want 'TestBot'", p.Name)
	}
	if p.Rating != 1400 {
		t.Errorf("Rating = %d, want 1400", p.Rating)
	}
}

func TestMatchMetadata(t *testing.T) {
	metadata := MatchMetadata{
		Players: []PlayerInfo{
			{ID: 0, Name: "Bot1", Rating: 1500},
			{ID: 1, Name: "Bot2", Rating: 1400},
		},
		MapSize:       "60x60",
		TurnCount:     150,
		Winner:        0,
		Condition:     "elimination",
		FinalScores:   []int{100, 95},
		IsUpset:       false,
		IsCloseFinish: true,
		IsFeatured:    true,
	}

	if len(metadata.Players) != 2 {
		t.Errorf("Players length = %d, want 2", len(metadata.Players))
	}
	if metadata.MapSize != "60x60" {
		t.Errorf("MapSize = %s, want '60x60'", metadata.MapSize)
	}
	if metadata.TurnCount != 150 {
		t.Errorf("TurnCount = %d, want 150", metadata.TurnCount)
	}
	if metadata.Winner != 0 {
		t.Errorf("Winner = %d, want 0", metadata.Winner)
	}
	if metadata.Condition != "elimination" {
		t.Errorf("Condition = %s, want 'elimination'", metadata.Condition)
	}
	if len(metadata.FinalScores) != 2 {
		t.Errorf("FinalScores length = %d, want 2", len(metadata.FinalScores))
	}
	if metadata.IsUpset {
		t.Error("IsUpset = true, want false")
	}
	if !metadata.IsCloseFinish {
		t.Error("IsCloseFinish = false, want true")
	}
	if !metadata.IsFeatured {
		t.Error("IsFeatured = false, want true")
	}
}

func TestConstants(t *testing.T) {
	if defaultMaxTokens != 3000 {
		t.Errorf("defaultMaxTokens = %d, want 3000", defaultMaxTokens)
	}
	if defaultTemperature != 0.7 {
		t.Errorf("defaultTemperature = %f, want 0.7", defaultTemperature)
	}
	if defaultTimeout != 120*time.Second {
		t.Errorf("defaultTimeout = %v, want 120s", defaultTimeout)
	}
}

func TestGenerateCommentaryResponse_JSON(t *testing.T) {
	resp := GenerateCommentaryResponse{
		KeyMoments: []KeyMomentCommentary{
			{
				Turn:         50,
				Description:  "Core capture",
				Significance: "high",
				Tags:         []string{"combat", "core_capture"},
			},
		},
		Summary:   "Test summary",
		Narrative: "Test narrative",
	}

	// Test JSON marshaling
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled GenerateCommentaryResponse
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(unmarshaled.KeyMoments) != 1 {
		t.Errorf("Unmarshaled KeyMoments length = %d, want 1", len(unmarshaled.KeyMoments))
	}
	if unmarshaled.Summary != "Test summary" {
		t.Errorf("Unmarshaled Summary = %s, want 'Test summary'", unmarshaled.Summary)
	}
	if unmarshaled.Narrative != "Test narrative" {
		t.Errorf("Unmarshaled Narrative = %s, want 'Test narrative'", unmarshaled.Narrative)
	}
}
