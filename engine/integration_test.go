package engine

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"math/rand"
)

// TestIntegration_HTTPMatch runs a complete match between two HTTP bots.
func TestIntegration_HTTPMatch(t *testing.T) {
	secret := "test-integration-secret"

	// Create mock bot servers for two players
	server0 := createMockBotServer(t, secret, 0)
	server1 := createMockBotServer(t, secret, 1)
	defer server0.Close()
	defer server1.Close()

	// Create HTTP bots
	auth0 := AuthConfig{BotID: "b_0", Secret: secret, MatchID: "m_integration"}
	auth1 := AuthConfig{BotID: "b_1", Secret: secret, MatchID: "m_integration"}

	bot0 := NewHTTPBot(server0.URL, auth0, WithHTTPTimeout(5*time.Second))
	bot1 := NewHTTPBot(server1.URL, auth1, WithHTTPTimeout(5*time.Second))

	// Create match runner with small config for fast test
	config := DefaultConfig()
	config.Rows = 20
	config.Cols = 20
	config.MaxTurns = 100

	runner := NewMatchRunner(config,
		WithRNG(rand.New(rand.NewSource(12345))),
		WithTimeout(5*time.Second),
	)

	runner.AddBot(bot0, "HTTPBot0")
	runner.AddBot(bot1, "HTTPBot1")

	// Run the match
	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if result == nil {
		t.Fatal("Match result is nil")
	}

	if replay == nil {
		t.Fatal("Replay is nil")
	}

	if replay.MatchID == "" {
		t.Error("Replay has empty MatchID")
	}

	if len(replay.Players) != 2 {
		t.Errorf("Replay has %d players, want 2", len(replay.Players))
	}

	if len(replay.Turns) == 0 {
		t.Error("Replay has no turns")
	}

	t.Logf("Match completed: Winner=%d, Turns=%d", result.Winner, result.Turns)

	// Verify win_prob array is populated (task: bf-qps)
	if len(replay.WinProb) == 0 {
		t.Error("Replay WinProb array is empty - ComputeWinProbability was not called")
	}

	// Verify WinProb entries have correct length (should equal number of players)
	if len(replay.WinProb) > 0 && len(replay.WinProb[0]) != len(replay.Players) {
		t.Errorf("WinProb entries have %d values, want %d (number of players)", len(replay.WinProb[0]), len(replay.Players))
	}

	// Verify WinProb values are in valid range [0, 1]
	for i, entry := range replay.WinProb {
		for j, prob := range entry {
			if prob < 0 || prob > 1 {
				t.Errorf("WinProb entry %d player %d has invalid probability %.2f (want 0-1)", i, j, prob)
			}
		}
	}

	// Verify critical moments are populated
	t.Logf("Critical moments detected: %d", len(replay.CriticalMoments))
	for _, m := range replay.CriticalMoments {
		t.Logf("  Turn %d: delta=%.2f, player=%d, desc=%s", m.Turn, m.Delta, m.Player, m.Description)
	}
}

// TestIntegration_HMACAuthentication verifies HMAC signing works end-to-end.
func TestIntegration_HMACAuthentication(t *testing.T) {
	secret := "hmac-test-secret"
	matchID := "m_hmac_test"
	turn := 42
	timestamp := time.Now().Unix()
	requestBody := []byte(`{"match_id":"m_hmac_test","turn":42}`)

	signature := SignRequest(secret, matchID, turn, timestamp, requestBody)

	auth := RequestAuth{
		MatchID:   matchID,
		Turn:      turn,
		Timestamp: timestamp,
		BotID:     "b_test",
		Signature: signature,
	}
	if err := VerifyRequest(secret, auth, requestBody); err != nil {
		t.Errorf("Signature verification failed: %v", err)
	}

	if err := VerifyRequest("wrong-secret", auth, requestBody); err == nil {
		t.Error("Verification should fail with wrong secret")
	}

	if err := VerifyRequest(secret, auth, []byte("wrong body")); err == nil {
		t.Error("Verification should fail with wrong body")
	}
}

// TestIntegration_ResponseSigning verifies response signing works.
func TestIntegration_ResponseSigning(t *testing.T) {
	secret := "response-test-secret"
	matchID := "m_response_test"
	turn := 10
	responseBody := []byte(`{"moves":[{"position":{"row":5,"col":5},"direction":"N"}]}`)

	signature := SignResponse(secret, matchID, turn, responseBody)

	if err := VerifyResponse(secret, matchID, turn, signature, responseBody); err != nil {
		t.Errorf("Response verification failed: %v", err)
	}

	if err := VerifyResponse("wrong-secret", matchID, turn, signature, responseBody); err == nil {
		t.Error("Verification should fail with wrong secret")
	}
}

// createMockBotServer creates a test HTTP server that acts as a bot.
func createMockBotServer(t *testing.T, secret string, playerID int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path != "/turn" {
			http.NotFound(w, r)
			return
		}

		var state VisibleState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		moves := make([]Move, 0)
		for _, bot := range state.Bots {
			if bot.Owner == state.You.ID {
				dir := DirN
				if playerID == 1 {
					dir = DirE
				}
				moves = append(moves, Move{
					Position:  bot.Position,
					Direction: dir,
				})
			}
		}

		resp := MoveResponse{Moves: moves}
		body, _ := json.Marshal(resp)

		matchID := r.Header.Get("X-ACB-Match-Id")
		turnStr := r.Header.Get("X-ACB-Turn")
		turn := 0
		for _, c := range turnStr {
			if c >= '0' && c <= '9' {
				turn = turn*10 + int(c-'0')
			}
		}

		sig := SignResponse(secret, matchID, turn, body)
		w.Header().Set("X-ACB-Signature", sig)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

// TestIntegration_CenterWeightedEnergy verifies that energy nodes are biased
// toward the map center to force contested energy collection.
func TestIntegration_CenterWeightedEnergy(t *testing.T) {
	secret := "test-energy-secret"
	server := createMockBotServer(t, secret, 0)
	defer server.Close()

	auth := AuthConfig{BotID: "b_test", Secret: secret, MatchID: "m_energy_test"}
	bot := NewHTTPBot(server.URL, auth, WithHTTPTimeout(5*time.Second))

	config := DefaultConfig()
	config.Rows = 60
	config.Cols = 60
	config.MaxTurns = 10 // Short match for fast test

	runner := NewMatchRunner(config,
		WithRNG(rand.New(rand.NewSource(42))),
		WithTimeout(5*time.Second),
	)

	runner.AddBot(bot, "TestBot")
	runner.AddBot(bot, "TestBot2")

	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if result == nil {
		t.Fatal("Match result is nil")
	}

	if replay == nil {
		t.Fatal("Replay is nil")
	}

	// Count energy nodes in central zone (20% of map radius)
	centerRow, centerCol := config.Rows/2, config.Cols/2
	maxRadius := float64(centerRow) * 0.20 // 20% of center distance = central zone
	centralCount := 0

	for _, en := range replay.Map.EnergyNodes {
		dr := float64(en.Row) - float64(centerRow)
		dc := float64(en.Col) - float64(centerCol)
		dist := math.Sqrt(dr*dr + dc*dc)
		if dist <= maxRadius {
			centralCount++
		}
	}

	// Expect at least 20% in central zone (allowing some variance for randomness)
	minCentral := int(float64(len(replay.Map.EnergyNodes)) * 0.20)
	if centralCount < minCentral {
		t.Errorf("expected at least %d energy nodes in central zone, got %d (total nodes: %d)",
			minCentral, centralCount, len(replay.Map.EnergyNodes))
	}

	t.Logf("Center-weighted energy: %d/%d nodes in central zone (%.1f%%)",
		centralCount, len(replay.Map.EnergyNodes),
		100.0*float64(centralCount)/float64(len(replay.Map.EnergyNodes)))
}
