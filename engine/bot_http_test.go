package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPBot_GetMoves(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turn" {
			http.NotFound(w, r)
			return
		}

		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type header")
		}
		if r.Header.Get("X-ACB-Match-Id") == "" {
			t.Error("missing X-ACB-Match-Id header")
		}
		if r.Header.Get("X-ACB-Signature") == "" {
			t.Error("missing X-ACB-Signature header")
		}

		// Read and parse request body
		var state VisibleState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Return moves for owned bots
		moves := make([]Move, 0)
		for _, bot := range state.Bots {
			if bot.Owner == state.You.ID {
				moves = append(moves, Move{
					Position:  bot.Position,
					Direction: DirN,
				})
			}
		}

		resp := MoveResponse{Moves: moves}
		body, _ := json.Marshal(resp)

		// Sign response
		sig := SignResponse("test-secret", state.MatchID, state.Turn, body)
		w.Header().Set("X-ACB-Signature", sig)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	// Create HTTP bot
	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth)

	// Create test game state
	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		Config:  DefaultConfig(),
		You: struct {
			ID     int `json:"id"`
			Energy int `json:"energy"`
			Score  int `json:"score"`
		}{
			ID:     0,
			Energy: 3,
			Score:  1,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 5, Col: 5}, Owner: 0},
			{Position: Position{Row: 10, Col: 10}, Owner: 1},
		},
		Energy: []Position{},
		Cores:  []VisibleCore{},
		Walls:  []Position{},
		Dead:   []VisibleBot{},
	}

	// Get moves
	moves, err := bot.GetMoves(state)
	if err != nil {
		t.Fatalf("GetMoves failed: %v", err)
	}

	// Should have one move for the owned bot
	if len(moves) != 1 {
		t.Errorf("got %d moves, want 1", len(moves))
	}
	if moves[0].Direction != DirN {
		t.Errorf("got direction %v, want DirN", moves[0].Direction)
	}
}

func TestHTTPBot_Timeout(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create HTTP bot with 100ms timeout
	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth, WithHTTPTimeout(100*time.Millisecond))

	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		Config:  DefaultConfig(),
	}

	// Get moves should timeout
	_, err := bot.GetMoves(state)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}

	// Check failure count increased
	if bot.failCount != 1 {
		t.Errorf("failCount = %d, want 1", bot.failCount)
	}
}

func TestHTTPBot_CrashAfter10Failures(t *testing.T) {
	// Create a failing server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth)

	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		Config:  DefaultConfig(),
	}

	// Fail 10 times
	for i := 0; i < 10; i++ {
		bot.GetMoves(state)
	}

	// Bot should be crashed
	if !bot.IsCrashed() {
		t.Error("bot should be marked as crashed after 10 failures")
	}

	// Further calls should return empty moves without making HTTP request
	moves, err := bot.GetMoves(state)
	if err != nil {
		t.Errorf("crashed bot should not return error, got: %v", err)
	}
	if len(moves) != 0 {
		t.Errorf("crashed bot should return empty moves, got %d", len(moves))
	}
}

func TestHTTPBot_ValidateMoves(t *testing.T) {
	// Create a server that returns invalid moves
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var state VisibleState
		json.NewDecoder(r.Body).Decode(&state)

		// Return moves with:
		// 1. Invalid direction
		// 2. Position without owned bot
		// 3. Duplicate position
		// 4. Valid move
		moves := []Move{
			{Position: Position{Row: 0, Col: 0}, Direction: DirNone}, // Invalid direction
			{Position: Position{Row: 99, Col: 99}, Direction: DirN},  // No bot there
			{Position: Position{Row: 5, Col: 5}, Direction: DirN},    // Valid
			{Position: Position{Row: 5, Col: 5}, Direction: DirS},    // Duplicate
		}

		resp := MoveResponse{Moves: moves}
		body, _ := json.Marshal(resp)
		sig := SignResponse("test-secret", state.MatchID, state.Turn, body)
		w.Header().Set("X-ACB-Signature", sig)
		w.Write(body)
	}))
	defer server.Close()

	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth)

	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		Config:  DefaultConfig(),
		You: struct {
			ID     int `json:"id"`
			Energy int `json:"energy"`
			Score  int `json:"score"`
		}{ID: 0},
		Bots: []VisibleBot{
			{Position: Position{Row: 5, Col: 5}, Owner: 0}, // Our bot
			{Position: Position{Row: 10, Col: 10}, Owner: 1}, // Enemy bot
		},
	}

	moves, err := bot.GetMoves(state)
	if err != nil {
		t.Fatalf("GetMoves failed: %v", err)
	}

	// Should only have 1 valid move (duplicate filtered, invalid direction filtered, non-owned filtered)
	if len(moves) != 1 {
		t.Errorf("got %d moves, want 1 (invalid filtered out)", len(moves))
	}
}

func TestHTTPBot_MissingSignature(t *testing.T) {
	// Server that returns valid moves but omits X-ACB-Signature header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := MoveResponse{Moves: []Move{
			{Position: Position{Row: 5, Col: 5}, Direction: DirN},
		}}
		body, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		// Intentionally no X-ACB-Signature header
		w.Write(body)
	}))
	defer server.Close()

	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth)

	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		Config:  DefaultConfig(),
		You: struct {
			ID     int `json:"id"`
			Energy int `json:"energy"`
			Score  int `json:"score"`
		}{ID: 0},
		Bots: []VisibleBot{
			{Position: Position{Row: 5, Col: 5}, Owner: 0},
		},
	}

	_, err := bot.GetMoves(state)
	if err == nil {
		t.Fatal("expected error for missing signature, got nil")
	}
	if bot.failCount != 1 {
		t.Errorf("failCount = %d, want 1", bot.failCount)
	}
}

func TestHTTPBot_BadSignature(t *testing.T) {
	// Server that returns moves with a wrong-key signature
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var state VisibleState
		json.NewDecoder(r.Body).Decode(&state)

		resp := MoveResponse{Moves: []Move{
			{Position: Position{Row: 5, Col: 5}, Direction: DirN},
		}}
		body, _ := json.Marshal(resp)

		// Sign with wrong secret
		sig := SignResponse("wrong-secret", state.MatchID, state.Turn, body)
		w.Header().Set("X-ACB-Signature", sig)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth)

	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		Config:  DefaultConfig(),
		You: struct {
			ID     int `json:"id"`
			Energy int `json:"energy"`
			Score  int `json:"score"`
		}{ID: 0},
		Bots: []VisibleBot{
			{Position: Position{Row: 5, Col: 5}, Owner: 0},
		},
	}

	_, err := bot.GetMoves(state)
	if err == nil {
		t.Fatal("expected error for bad signature, got nil")
	}
	if bot.failCount != 1 {
		t.Errorf("failCount = %d, want 1", bot.failCount)
	}
}

func TestHTTPBot_BadSignatureCrashes(t *testing.T) {
	// Verify that 10 consecutive bad-signature responses crashes the bot
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var state VisibleState
		json.NewDecoder(r.Body).Decode(&state)

		resp := MoveResponse{Moves: []Move{}}
		body, _ := json.Marshal(resp)
		sig := SignResponse("wrong-secret", state.MatchID, state.Turn, body)
		w.Header().Set("X-ACB-Signature", sig)
		w.Write(body)
	}))
	defer server.Close()

	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth)

	state := &VisibleState{
		MatchID: "m_test",
		Turn:    1,
		Config:  DefaultConfig(),
	}

	for i := 0; i < 10; i++ {
		bot.GetMoves(state)
	}

	if !bot.IsCrashed() {
		t.Error("bot should be crashed after 10 consecutive bad-signature failures")
	}
}

func TestHTTPBot_Health(t *testing.T) {
	// Create a server with health endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	auth := AuthConfig{
		BotID:   "b_test",
		Secret:  "test-secret",
		MatchID: "m_test",
	}
	bot := NewHTTPBot(server.URL, auth)

	if err := bot.Health(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}
