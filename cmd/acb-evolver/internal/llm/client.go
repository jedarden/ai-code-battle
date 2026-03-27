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

// Tier selects the LLM model tier for a generation request.
type Tier string

const (
	// TierFast uses GLM-5-Turbo for bulk candidate generation.
	TierFast Tier = "fast"
	// TierStrong uses GLM-5 for high-quality refinement passes.
	TierStrong Tier = "strong"
)

const (
	modelFast   = "GLM-5-Turbo"
	modelStrong = "GLM-5"

	defaultMaxTokens   = 4096
	defaultTemperature = 0.85
	defaultTimeout     = 120 * time.Second
)

// Client is an OpenAI-compatible LLM client that routes requests through the
// ZAI proxy.  Create one with NewClient and reuse it across calls.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a Client that sends requests to baseURL (e.g.
// "http://zai-proxy-apexalgo.tail1b1987.ts.net:8080").
// apiKey may be empty when the proxy does not require authentication.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// GenerateRequest specifies a single code-generation task.
type GenerateRequest struct {
	// Prompt is the full evolution prompt assembled by the prompt builder.
	Prompt string
	// Tier selects the model: TierFast for bulk generation, TierStrong for
	// refinement.
	Tier Tier
	// MaxTokens caps the response length (0 → defaultMaxTokens).
	MaxTokens int
	// Temperature controls response randomness (0 → defaultTemperature).
	Temperature float64
	// TargetLang is the expected language of the returned code block
	// (e.g. "go").  Used during extraction.
	TargetLang string
}

// GenerateResponse holds extracted code and the raw LLM output.
type GenerateResponse struct {
	// Candidate is the extracted bot code and its detected language.
	Candidate *Candidate
	// RawText is the unprocessed LLM response text.
	RawText string
}

// Generate sends the prompt to the configured LLM tier and returns the best
// extracted bot code candidate.
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	model := modelFast
	if req.Tier == TierStrong {
		model = modelStrong
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}
	temp := req.Temperature
	if temp == 0 {
		temp = defaultTemperature
	}

	raw, err := c.chatCompletion(ctx, model, req.Prompt, maxTokens, temp)
	if err != nil {
		return nil, err
	}

	candidate, err := ExtractBestCandidate(raw, req.TargetLang)
	if err != nil {
		return nil, fmt.Errorf("extract candidate: %w (raw preview: %.200s)", err, raw)
	}

	return &GenerateResponse{
		Candidate: candidate,
		RawText:   raw,
	}, nil
}

// ── OpenAI-compatible wire types ──────────────────────────────────────────

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) chatCompletion(ctx context.Context, model, prompt string, maxTokens int, temperature float64) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: model,
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
