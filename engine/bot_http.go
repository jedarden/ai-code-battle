package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPBot is a bot that communicates via HTTP POST requests.
// It implements BotInterface for use with MatchRunner.
type HTTPBot struct {
	client    *http.Client
	baseURL   string // bot's HTTP endpoint (e.g., "http://localhost:8080")
	auth      AuthConfig
	matchID   string
	turn      int
	crashed   bool
	failCount int        // consecutive failures
	lastDebug *DebugInfo // debug info from last response
}

// HTTPOption is a functional option for HTTPBot.
type HTTPOption func(*HTTPBot)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) HTTPOption {
	return func(b *HTTPBot) {
		b.client = client
	}
}

// WithHTTPTimeout sets the HTTP timeout (default 3 seconds).
func WithHTTPTimeout(timeout time.Duration) HTTPOption {
	return func(b *HTTPBot) {
		b.client.Timeout = timeout
	}
}

// NewHTTPBot creates a new HTTP bot.
func NewHTTPBot(baseURL string, auth AuthConfig, options ...HTTPOption) *HTTPBot {
	bot := &HTTPBot{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		baseURL: baseURL,
		auth:    auth,
		matchID: auth.MatchID,
	}

	for _, opt := range options {
		opt(bot)
	}

	return bot
}

// SetMatchID sets the current match ID (called at match start).
func (b *HTTPBot) SetMatchID(matchID string) {
	b.matchID = matchID
	b.auth.MatchID = matchID
	b.turn = 0
	b.crashed = false
	b.failCount = 0
}

// IsCrashed returns true if the bot has been marked as crashed.
func (b *HTTPBot) IsCrashed() bool {
	return b.crashed
}

// markInactive is called by MatchRunner when the match-level failure policy
// is reached. Inactive bots remain alive in the game but no longer receive
// turn requests.
func (b *HTTPBot) markInactive() {
	b.crashed = true
}

// MoveResponse represents the JSON response from a bot.
type MoveResponse struct {
	Moves []Move     `json:"moves"`
	Debug *DebugInfo `json:"debug,omitempty"`
}

// DebugInfo contains optional debug telemetry from the bot.
type DebugInfo struct {
	Reasoning string                 `json:"reasoning,omitempty"`
	Targets   []DebugTarget          `json:"targets,omitempty"`
	Values    map[string]interface{} `json:"values,omitempty"`
	Heatmap   *DebugHeatmap          `json:"heatmap,omitempty"`
}

// DebugTarget represents a debug target marker.
type DebugTarget struct {
	Position Position `json:"position"`
	Label    string   `json:"label"`
	Color    string   `json:"color,omitempty"`
	Priority float64  `json:"priority"`
}

// DebugHeatmap represents a 2D grid overlay for visualization.
type DebugHeatmap struct {
	Name string      `json:"name"` // e.g., "threat", "influence"
	Data [][]float64 `json:"data"` // 2D array of values (row-major)
}

type moveResponseEnvelope struct {
	Moves json.RawMessage `json:"moves"`
	Debug *DebugInfo      `json:"debug"`
}

type moveResponseMove struct {
	Position  *moveResponsePosition `json:"position"`
	Direction *string               `json:"direction"`
}

type moveResponsePosition struct {
	Row *int `json:"row"`
	Col *int `json:"col"`
}

func decodeMoveResponse(responseBody []byte) (MoveResponse, error) {
	var envelope moveResponseEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return MoveResponse{}, fmt.Errorf("invalid move response: %w", err)
	}
	if len(envelope.Moves) == 0 {
		return MoveResponse{}, fmt.Errorf("move response must include moves")
	}

	var wireMoves []moveResponseMove
	if err := json.Unmarshal(envelope.Moves, &wireMoves); err != nil {
		return MoveResponse{}, fmt.Errorf("moves must be an array: %w", err)
	}
	if wireMoves == nil {
		return MoveResponse{}, fmt.Errorf("moves must be an array, not null")
	}

	response := MoveResponse{
		Moves: make([]Move, 0, len(wireMoves)),
		Debug: envelope.Debug,
	}
	for i, wireMove := range wireMoves {
		if wireMove.Position == nil || wireMove.Position.Row == nil || wireMove.Position.Col == nil {
			return MoveResponse{}, fmt.Errorf("move %d must include position.row and position.col", i)
		}
		if *wireMove.Position.Row < 0 || *wireMove.Position.Col < 0 {
			return MoveResponse{}, fmt.Errorf("move %d position must be non-negative", i)
		}
		if wireMove.Direction == nil {
			return MoveResponse{}, fmt.Errorf("move %d must include direction", i)
		}

		direction := ParseDirection(*wireMove.Direction)
		if direction == DirNone && *wireMove.Direction != "stay" {
			return MoveResponse{}, fmt.Errorf("move %d has invalid direction %q", i, *wireMove.Direction)
		}
		response.Moves = append(response.Moves, Move{
			Position: Position{
				Row: *wireMove.Position.Row,
				Col: *wireMove.Position.Col,
			},
			Direction: direction,
		})
	}
	return response, nil
}

// GetMoves sends the game state to the bot and returns its moves.
// Implements BotInterface.
func (b *HTTPBot) GetMoves(state *VisibleState) ([]Move, error) {
	b.lastDebug = nil

	// If crashed, return no moves (bots hold position)
	if b.crashed {
		return []Move{}, nil
	}

	if state == nil || state.MatchID == "" {
		b.recordFailure()
		return nil, fmt.Errorf("visible state must include match_id")
	}

	// Update request identity from the exact state that will be signed and sent.
	b.matchID = state.MatchID
	b.auth.MatchID = state.MatchID
	b.turn = state.Turn

	// Serialize state
	requestBody, err := json.Marshal(state)
	if err != nil {
		b.recordFailure()
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	// Build request
	url := fmt.Sprintf("%s/turn", b.baseURL)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		b.recordFailure()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	timestamp := time.Now().Unix()
	signature := SignRequest(b.auth.Secret, b.matchID, b.turn, timestamp, requestBody)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ACB-Match-Id", b.matchID)
	req.Header.Set("X-ACB-Turn", fmt.Sprintf("%d", b.turn))
	req.Header.Set("X-ACB-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-ACB-Bot-Id", b.auth.BotID)
	req.Header.Set("X-ACB-Signature", signature)

	// Send request
	resp, err := b.client.Do(req)
	if err != nil {
		b.recordFailure()
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		b.recordFailure()
		return nil, fmt.Errorf("bot returned status %d", resp.StatusCode)
	}

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		b.recordFailure()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Verify response signature (strict — per §4.4)
	responseSig := resp.Header.Get("X-ACB-Signature")
	if responseSig == "" {
		b.recordFailure()
		return nil, fmt.Errorf("missing response signature")
	}
	if err := VerifyResponse(b.auth.Secret, b.matchID, b.turn, responseSig, responseBody); err != nil {
		b.recordFailure()
		return nil, fmt.Errorf("response signature verification failed: %w", err)
	}

	moveResp, err := decodeMoveResponse(responseBody)
	if err != nil {
		b.recordFailure()
		return nil, fmt.Errorf("failed to validate response: %w", err)
	}

	// Validate debug size (max 10 KB per turn per §14.1)
	if moveResp.Debug != nil {
		debugJSON, err := json.Marshal(moveResp.Debug)
		if err != nil {
			b.recordFailure()
			return nil, fmt.Errorf("failed to marshal debug data: %w", err)
		}
		if len(debugJSON) > 10*1024 { // 10 KB limit
			b.recordFailure()
			return nil, fmt.Errorf("debug data exceeds 10 KB limit (%d bytes)", len(debugJSON))
		}
	}

	// Validate moves (basic validation)
	moves := b.validateMoves(moveResp.Moves, state)

	// Store debug info for replay
	b.lastDebug = moveResp.Debug

	// Reset failure count on success
	b.failCount = 0

	return moves, nil
}

// LastDebug returns the debug info from the most recent response, or nil.
func (b *HTTPBot) LastDebug() *DebugInfo {
	return b.lastDebug
}

// validateMoves validates and filters moves against the current state.
func (b *HTTPBot) validateMoves(moves []Move, state *VisibleState) []Move {
	// Build set of owned bot positions
	ownedPositions := make(map[Position]bool)
	for _, bot := range state.Bots {
		if bot.Owner == state.You.ID {
			ownedPositions[bot.Position] = true
		}
	}

	// Filter to valid moves
	validMoves := make([]Move, 0, len(moves))
	seen := make(map[Position]bool)

	for _, move := range moves {
		// Check direction is valid
		if move.Direction < DirN || move.Direction > DirW {
			continue
		}

		// Check position has an owned bot
		if !ownedPositions[move.Position] {
			continue
		}

		// Check for duplicate positions (first wins)
		if seen[move.Position] {
			continue
		}
		seen[move.Position] = true

		validMoves = append(validMoves, move)
	}

	return validMoves
}

// recordFailure tracks consecutive failures and marks the bot as crashed at
// the match policy threshold. MatchRunner also tracks this boundary so bots
// implementing BotInterface without HTTPBot receive the same treatment.
func (b *HTTPBot) recordFailure() {
	b.failCount++
	if b.failCount >= BotInactiveAfterFailures {
		b.crashed = true
	}
}

// Health checks the bot's health endpoint.
func (b *HTTPBot) Health() error {
	url := fmt.Sprintf("%s/health", b.baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}
