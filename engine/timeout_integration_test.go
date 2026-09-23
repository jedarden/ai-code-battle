package engine

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type timeoutIntegrationServer struct {
	mu      sync.Mutex
	calls   []VisibleState
	delay   time.Duration
	status  int
	success map[int]bool
	server  *httptest.Server
}

func newTimeoutIntegrationServer(t *testing.T, secret string, delay time.Duration, status int, success map[int]bool) *timeoutIntegrationServer {
	t.Helper()
	s := &timeoutIntegrationServer{delay: delay, status: status, success: success}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var state VisibleState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		s.calls = append(s.calls, state)
		turn := state.Turn
		s.mu.Unlock()

		if s.delay > 0 {
			time.Sleep(s.delay)
		}
		if s.success != nil && s.success[turn] {
			body, _ := json.Marshal(MoveResponse{Moves: []Move{}})
			w.Header().Set("X-ACB-Signature", SignResponse(secret, state.MatchID, state.Turn, body))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
			return
		}
		if s.status != 0 {
			w.WriteHeader(s.status)
			return
		}
		body, _ := json.Marshal(MoveResponse{Moves: []Move{}})
		w.Header().Set("X-ACB-Signature", SignResponse(secret, state.MatchID, state.Turn, body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *timeoutIntegrationServer) URL() string {
	return s.server.URL
}

func (s *timeoutIntegrationServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func timeoutIntegrationConfig(maxTurns int) Config {
	cfg := DefaultConfig()
	cfg.Rows = 20
	cfg.Cols = 20
	cfg.MaxTurns = maxTurns
	cfg.ZoneEnabled = false
	cfg.CoresPerPlayer = 1
	return cfg
}

func timeoutIntegrationGoodServer(t *testing.T, secret string) *timeoutIntegrationServer {
	t.Helper()
	return newTimeoutIntegrationServer(t, secret, 0, 0, nil)
}

func timeoutIntegrationInactiveEvent(t *testing.T, replay *Replay) Event {
	t.Helper()
	var found Event
	count := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type != EventBotInactive {
				continue
			}
			count++
			found = event
		}
	}
	if count != 1 {
		t.Fatalf("got %d bot_inactive events, want 1", count)
	}
	return found
}

func TestMatchRunner_HTTPTimeoutDeactivatesAfterTenDeadlines(t *testing.T) {
	const secret = "timeout-integration-secret"
	bad := newTimeoutIntegrationServer(t, secret, 80*time.Millisecond, 0, nil)
	good := timeoutIntegrationGoodServer(t, secret)

	config := timeoutIntegrationConfig(BotInactiveAfterFailures + 1)
	runner := NewMatchRunner(config, WithRNG(rand.New(rand.NewSource(91))), WithTimeout(10*time.Millisecond))
	runner.AddBot(NewHTTPBot(bad.URL(), AuthConfig{BotID: "bad", Secret: secret, MatchID: "m_timeout"}, WithHTTPTimeout(200*time.Millisecond)), "bad")
	runner.AddBot(NewHTTPBot(good.URL(), AuthConfig{BotID: "good", Secret: secret, MatchID: "m_timeout"}, WithHTTPTimeout(200*time.Millisecond)), "good")

	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("run match: %v", err)
	}
	if len(result.Crashed) != 2 || !result.Crashed[0] || result.Crashed[1] {
		t.Fatalf("crashed status = %v, want [true false]", result.Crashed)
	}
	if bad.requestCount() != BotInactiveAfterFailures {
		t.Errorf("timed-out bot received %d requests, want %d", bad.requestCount(), BotInactiveAfterFailures)
	}
	if good.requestCount() != config.MaxTurns {
		t.Errorf("healthy bot received %d requests, want %d", good.requestCount(), config.MaxTurns)
	}

	event := timeoutIntegrationInactiveEvent(t, replay)
	if event.Turn != BotInactiveAfterFailures {
		t.Errorf("inactive event turn = %d, want %d", event.Turn, BotInactiveAfterFailures)
	}
	details, ok := event.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("inactive event details type = %T, want map", event.Details)
	}
	if details["player"] != 0 || details["consecutive_failures"] != BotInactiveAfterFailures || details["reason"] != "timeout" {
		t.Errorf("inactive event details = %#v", details)
	}

	last := replay.Turns[len(replay.Turns)-1]
	for _, bot := range last.Bots {
		if bot.Owner == 0 && !bot.Alive {
			t.Fatal("inactive bot had a living unit removed")
		}
	}
}

func TestMatchRunner_HTTPProtocolFailuresDeactivateAfterTenFailures(t *testing.T) {
	const secret = "protocol-integration-secret"
	bad := newTimeoutIntegrationServer(t, secret, 0, http.StatusInternalServerError, nil)
	good := timeoutIntegrationGoodServer(t, secret)

	config := timeoutIntegrationConfig(BotInactiveAfterFailures + 2)
	runner := NewMatchRunner(config, WithRNG(rand.New(rand.NewSource(92))), WithTimeout(time.Second))
	runner.AddBot(NewHTTPBot(bad.URL(), AuthConfig{BotID: "bad", Secret: secret, MatchID: "m_protocol"}, WithHTTPTimeout(time.Second)), "bad")
	runner.AddBot(NewHTTPBot(good.URL(), AuthConfig{BotID: "good", Secret: secret, MatchID: "m_protocol"}, WithHTTPTimeout(time.Second)), "good")

	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("run match: %v", err)
	}
	if len(result.Crashed) != 2 || !result.Crashed[0] || result.Crashed[1] {
		t.Fatalf("crashed status = %v, want [true false]", result.Crashed)
	}
	if bad.requestCount() != BotInactiveAfterFailures {
		t.Errorf("failed bot received %d requests, want %d", bad.requestCount(), BotInactiveAfterFailures)
	}
	if good.requestCount() != config.MaxTurns {
		t.Errorf("healthy bot received %d requests, want %d", good.requestCount(), config.MaxTurns)
	}

	event := timeoutIntegrationInactiveEvent(t, replay)
	details, ok := event.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("inactive event details type = %T, want map", event.Details)
	}
	if details["reason"] != "error" {
		t.Errorf("inactive reason = %#v, want error", details["reason"])
	}
}

func TestMatchRunner_HTTPValidEmptyResponseResetsFailureStreak(t *testing.T) {
	const secret = "reset-integration-secret"
	successTurns := map[int]bool{9: true, 19: true}
	bad := newTimeoutIntegrationServer(t, secret, 0, http.StatusInternalServerError, successTurns)
	good := timeoutIntegrationGoodServer(t, secret)

	config := timeoutIntegrationConfig(20)
	runner := NewMatchRunner(config, WithRNG(rand.New(rand.NewSource(93))), WithTimeout(time.Second))
	runner.AddBot(NewHTTPBot(bad.URL(), AuthConfig{BotID: "bad", Secret: secret, MatchID: "m_reset"}, WithHTTPTimeout(time.Second)), "bad")
	runner.AddBot(NewHTTPBot(good.URL(), AuthConfig{BotID: "good", Secret: secret, MatchID: "m_reset"}, WithHTTPTimeout(time.Second)), "good")

	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("run match: %v", err)
	}
	if result.Crashed[0] {
		t.Fatal("valid empty responses did not reset the consecutive failure streak")
	}
	if bad.requestCount() != config.MaxTurns {
		t.Errorf("recovering bot received %d requests, want %d", bad.requestCount(), config.MaxTurns)
	}
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == EventBotInactive {
				t.Fatalf("recovering bot became inactive on turn %d", turn.Turn)
			}
		}
	}
}
