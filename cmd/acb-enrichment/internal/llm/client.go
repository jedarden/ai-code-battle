// Package llm provides an LLM client for generating AI commentary on match replays.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMaxTokens   = 3000
	defaultTemperature = 0.7
	defaultTimeout     = 120 * time.Second
)

// Client is an OpenAI-compatible LLM client for generating replay commentary.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient creates a new LLM client.
func NewClient(baseURL, apiKey, model string) *Client {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// GenerateCommentaryRequest holds the parameters for generating commentary.
type GenerateCommentaryRequest struct {
	MatchID     string
	ReplayJSON  string // Full replay JSON as string
	Metadata    MatchMetadata
	KeyMoments  []KeyMoment
	WinProbData string // Sampled win probability data
	MaxTokens   int
	Temperature float64
}

// MatchMetadata contains match information for commentary generation.
type MatchMetadata struct {
	Players       []PlayerInfo
	MapSize       string // e.g. "60x60"
	TurnCount     int
	Winner        int
	Condition     string
	FinalScores   []int
	IsUpset       bool
	IsCloseFinish bool
	IsFeatured    bool
}

// PlayerInfo describes a single player/bot in the match.
type PlayerInfo struct {
	ID     int
	Name   string
	Rating int
}

// KeyMoment represents a significant event in the match.
type KeyMoment struct {
	Turn        int
	Delta       float64
	Description string
}

// GenerateCommentaryResponse contains the AI-generated commentary.
type GenerateCommentaryResponse struct {
	KeyMoments []KeyMomentCommentary `json:"key_moments"`
	Summary    string                `json:"summary"`
	Narrative  string                `json:"narrative"`
}

// KeyMomentCommentary is commentary for a specific moment in the match.
type KeyMomentCommentary struct {
	Turn         int      `json:"turn"`
	Description  string   `json:"description"`
	Significance string   `json:"significance"` // "high", "medium", "low"
	Tags         []string `json:"tags"`         // e.g. ["combat", "core_capture", "turning_point"]
}

// GenerateCommentary sends the replay data to the LLM and returns structured commentary.
func (c *Client) GenerateCommentary(ctx context.Context, req GenerateCommentaryRequest) (*GenerateCommentaryResponse, error) {
	prompt := c.buildPrompt(req)

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}
	temp := req.Temperature
	if temp == 0 {
		temp = defaultTemperature
	}

	raw, err := c.chatCompletion(ctx, prompt, maxTokens, temp)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	result, err := c.parseResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (raw: %.500s)", err, raw)
	}

	return result, nil
}

// buildPrompt constructs the LLM prompt from the request data.
func (c *Client) buildPrompt(req GenerateCommentaryRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert esports commentator for AI Code Battle, a competitive bot programming game.\n\n")
	sb.WriteString("Generate engaging, insightful commentary for the following match replay.\n\n")

	// Match metadata
	sb.WriteString("## Match Information\n\n")
	sb.WriteString(fmt.Sprintf("- Match ID: %s\n", req.MatchID))
	sb.WriteString(fmt.Sprintf("- Map: %s grid\n", req.Metadata.MapSize))
	sb.WriteString(fmt.Sprintf("- Duration: %d turns\n", req.Metadata.TurnCount))
	sb.WriteString(fmt.Sprintf("- Win Condition: %s\n", req.Metadata.Condition))

	// Players
	sb.WriteString("\n### Players\n\n")
	for _, p := range req.Metadata.Players {
		winnerMark := ""
		if p.ID == req.Metadata.Winner {
			winnerMark = " (winner)"
		}
		sb.WriteString(fmt.Sprintf("- %s (Rating: %d)%s\n", p.Name, p.Rating, winnerMark))
	}

	// Final scores
	if len(req.Metadata.FinalScores) > 0 {
		sb.WriteString("\n### Final Scores\n\n")
		for i, score := range req.Metadata.FinalScores {
			if i < len(req.Metadata.Players) {
				sb.WriteString(fmt.Sprintf("- %s: %d\n", req.Metadata.Players[i].Name, score))
			}
		}
	}

	// Key moments from engine analysis
	if len(req.KeyMoments) > 0 {
		sb.WriteString("\n### Key Moments (Engine Analysis)\n\n")
		for _, km := range req.KeyMoments {
			sb.WriteString(fmt.Sprintf("- Turn %d: %s (win prob shift: %.2f)\n", km.Turn, km.Description, km.Delta))
		}
	}

	// Tags for this match
	sb.WriteString("\n### Match Characteristics\n\n")
	if req.Metadata.IsUpset {
		sb.WriteString("- UPSET: Lower-rated bot won\n")
	}
	if req.Metadata.IsCloseFinish {
		sb.WriteString("- CLOSE FINISH: Final score difference was 2 or less\n")
	}
	if req.Metadata.IsFeatured {
		sb.WriteString("- FEATURED: This is a highlighted match\n")
	}

	sb.WriteString("\n## Output Format\n\n")
	sb.WriteString("Return a JSON object with the following structure:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{
  "key_moments": [
    {
      "turn": 87,
      "description": "SwarmBot loses 6 units in eastern engagement",
      "significance": "high",
      "tags": ["combat", "turning_point"]
    }
  ],
  "summary": "A 2-3 sentence summary of the match narrative",
  "narrative": "A paragraph-length play-by-play narrative suitable for a replay viewer"
}
`)
	sb.WriteString("```\n\n")

	sb.WriteString("## Guidelines\n\n")
	sb.WriteString("- Focus on strategic insights, not just play-by-play\n")
	sb.WriteString("- Highlight what made this match interesting (upsets, close finish, turning points)\n")
	sb.WriteString("- Use esports terminology (positioning, economy, pressure, comeback)\n")
	sb.WriteString("- Keep descriptions concise but vivid\n")
	sb.WriteString("- Significance levels: 'high' for match-defining moments, 'medium' for important shifts, 'low' for notable events\n")
	sb.WriteString("- Tags help categorize moments: combat, core_capture, spawn_wave, energy_control, turning_point, comeback\n")

	return sb.String()
}

// chatCompletion sends a chat completion request to the LLM.
func (c *Client) chatCompletion(ctx context.Context, prompt string, maxTokens int, temperature float64) (string, error) {
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type chatRequest struct {
		Model       string        `json:"model"`
		Messages    []chatMessage `json:"messages"`
		MaxTokens   int           `json:"max_tokens,omitempty"`
		Temperature float64       `json:"temperature,omitempty"`
	}
	type chatResponse struct {
		Choices []struct {
			Message chatMessage `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	body, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   maxTokens,
		Temperature: temperature,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm api returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var cr chatResponse
	if err := json.Unmarshal(respBytes, &cr); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("llm api error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("llm api returned no choices")
	}

	return cr.Choices[0].Message.Content, nil
}

// parseResponse extracts JSON from the LLM response.
func (c *Client) parseResponse(raw string) (*GenerateCommentaryResponse, error) {
	// Try to extract JSON from markdown code blocks
	raw = strings.TrimSpace(raw)

	// Remove markdown code block markers if present
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}

	var result GenerateCommentaryResponse
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("unmarshal commentary JSON: %w", err)
	}

	return &result, nil
}
