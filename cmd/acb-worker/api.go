// API client for Worker API communication
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient communicates with the Worker API.
type APIClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
	maxRetries int
}

// NewAPIClient creates a new API client.
func NewAPIClient(cfg *Config) *APIClient {
	return &APIClient{
		endpoint: cfg.APIEndpoint,
		apiKey:   cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: cfg.MaxRetries,
	}
}

// Job represents a pending job from the API.
type Job struct {
	ID          string     `json:"id"`
	MatchID     string     `json:"match_id"`
	Status      string     `json:"status"`
	WorkerID    *string    `json:"worker_id"`
	ClaimedAt   *time.Time `json:"claimed_at"`
	HeartbeatAt *time.Time `json:"heartbeat_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// JobClaimResponse contains the data needed to execute a match.
type JobClaimResponse struct {
	Job          Job           `json:"job"`
	Match        Match         `json:"match"`
	Participants []Participant `json:"participants"`
	Map          MapData       `json:"map"`
	Bots         []BotInfo     `json:"bots"`
	BotSecrets   []BotSecret   `json:"bot_secrets"`
}

// Match represents match metadata.
type Match struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	WinnerID    *string    `json:"winner_id"`
	Turns       *int       `json:"turns"`
	EndReason   *string    `json:"end_reason"`
	MapID       string     `json:"map_id"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// Participant represents a match participant.
type Participant struct {
	ID                   string `json:"id"`
	MatchID              string `json:"match_id"`
	BotID                string `json:"bot_id"`
	PlayerIndex          int    `json:"player_index"`
	Score                int    `json:"score"`
	RatingBefore         int    `json:"rating_before"`
	RatingAfter          *int   `json:"rating_after"`
	RatingDeviationBefore int   `json:"rating_deviation_before"`
	RatingDeviationAfter *int   `json:"rating_deviation_after"`
}

// MapData represents map configuration.
type MapData struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Walls  string `json:"walls"`
	Spawns string `json:"spawns"`
	Cores  string `json:"cores"`
}

// BotInfo contains bot endpoint information.
type BotInfo struct {
	ID          string `json:"id"`
	EndpointURL string `json:"endpoint_url"`
}

// BotSecret contains bot authentication secret.
type BotSecret struct {
	BotID  string `json:"bot_id"`
	Secret string `json:"secret"`
}

// APIResponse is a generic API response.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// GetNextJob fetches the next pending job.
func (c *APIClient) GetNextJob(ctx context.Context) (*Job, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/jobs/next", nil)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	if apiResp.Data == nil {
		return nil, nil // No pending jobs
	}

	var job Job
	if err := json.Unmarshal(apiResp.Data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse job: %w", err)
	}

	return &job, nil
}

// ClaimJob claims a job for execution.
func (c *APIClient) ClaimJob(ctx context.Context, jobID string, workerID string) (*JobClaimResponse, error) {
	body := map[string]string{"worker_id": workerID}

	resp, err := c.doRequest(ctx, "POST", "/api/jobs/"+jobID+"/claim", body)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	var claimResp JobClaimResponse
	if err := json.Unmarshal(apiResp.Data, &claimResp); err != nil {
		return nil, fmt.Errorf("failed to parse claim response: %w", err)
	}

	return &claimResp, nil
}

// Heartbeat sends a heartbeat for a claimed job.
func (c *APIClient) Heartbeat(ctx context.Context, jobID string, workerID string) error {
	body := map[string]string{"worker_id": workerID}

	_, err := c.doRequest(ctx, "POST", "/api/jobs/"+jobID+"/heartbeat", body)
	return err
}

// SubmitResult submits the result of a completed match.
func (c *APIClient) SubmitResult(ctx context.Context, jobID string, result *MatchResult, replayURL string) error {
	body := map[string]interface{}{
		"winner_id":   result.WinnerID,
		"turns":       result.Turns,
		"end_reason":  result.EndReason,
		"replay_url":  replayURL,
		"scores":      result.Scores,
	}

	_, err := c.doRequest(ctx, "POST", "/api/jobs/"+jobID+"/result", body)
	return err
}

// FailJob marks a job as failed.
func (c *APIClient) FailJob(ctx context.Context, jobID string, workerID string, errorMessage string) error {
	body := map[string]string{
		"worker_id":     workerID,
		"error_message": errorMessage,
	}

	_, err := c.doRequest(ctx, "POST", "/api/jobs/"+jobID+"/fail", body)
	return err
}

// doRequest makes an HTTP request with retries.
func (c *APIClient) doRequest(ctx context.Context, method string, path string, body interface{}) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second * time.Duration(attempt)):
			}
		}

		resp, err := c.doSingleRequest(ctx, method, path, body)
		if err != nil {
			lastErr = err
			// Check if it's a client error (don't retry)
			if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
				return nil, err
			}
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.maxRetries, lastErr)
}

// doSingleRequest makes a single HTTP request.
func (c *APIClient) doSingleRequest(ctx context.Context, method string, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	return respBody, nil
}

// HTTPError represents an HTTP error response.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}
