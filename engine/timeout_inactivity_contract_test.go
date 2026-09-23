package engine

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type timeoutInactivityOutcome struct {
	err   error
	delay time.Duration
}

type timeoutInactivityScriptBot struct {
	mu       sync.Mutex
	outcomes []timeoutInactivityOutcome
	calls    int
}

func (b *timeoutInactivityScriptBot) GetMoves(_ *VisibleState) ([]Move, error) {
	b.mu.Lock()
	call := b.calls
	b.calls++
	if call >= len(b.outcomes) {
		b.mu.Unlock()
		return []Move{}, nil
	}
	outcome := b.outcomes[call]
	b.mu.Unlock()
	if outcome.delay > 0 {
		time.Sleep(outcome.delay)
	}
	return []Move{}, outcome.err
}

func (b *timeoutInactivityScriptBot) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func timeoutInactivityConfig(maxTurns int) Config {
	cfg := DefaultConfig()
	cfg.Rows = 20
	cfg.Cols = 20
	cfg.MaxTurns = maxTurns
	cfg.ZoneEnabled = false
	cfg.CoresPerPlayer = 1
	return cfg
}

func timeoutInactivityState(cfg Config) *GameState {
	gs := NewGameState(cfg, rand.New(rand.NewSource(17)))
	gs.AddPlayer()
	gs.AddPlayer()
	return gs
}

func timeoutInactivityEventCount(replay *Replay) int {
	count := 0
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == EventBotInactive {
				count++
			}
		}
	}
	return count
}

func TestTimeoutInactivity_DefaultResponseBudgetIsThreeSeconds(t *testing.T) {
	runner := NewMatchRunner(DefaultConfig())
	if runner.timeout != 3*time.Second {
		t.Fatalf("runner timeout = %v, want 3s", runner.timeout)
	}
	bot := NewHTTPBot("http://127.0.0.1:1", AuthConfig{})
	if bot.client.Timeout != 3*time.Second {
		t.Fatalf("HTTP client timeout = %v, want 3s", bot.client.Timeout)
	}
}

func TestTimeoutInactivity_ConsecutiveFailuresResetAfterSuccess(t *testing.T) {
	cfg := timeoutInactivityConfig(4)
	failure := errors.New("bot failure")
	bot := &timeoutInactivityScriptBot{outcomes: []timeoutInactivityOutcome{
		{err: failure},
		{delay: 25 * time.Millisecond},
		{err: failure},
		{},
	}}
	runner := NewMatchRunner(cfg, WithTimeout(5*time.Millisecond))
	runner.AddBot(bot, "scripted")
	runner.AddBot(NewIdleBot(), "idle")
	gs := timeoutInactivityState(cfg)

	wantStreaks := []int{1, 2, 3, 0}
	for turn, wantStreak := range wantStreaks {
		gs.ClearTurnState()
		moves := runner.getMovesFromBots(gs)
		if got := runner.failureStreak[0]; got != wantStreak {
			t.Fatalf("turn %d failure streak = %d, want %d", turn+1, got, wantStreak)
		}
		_, accepted := moves[0]
		if accepted != (wantStreak == 0) {
			t.Fatalf("turn %d accepted = %v, want %v", turn+1, accepted, wantStreak == 0)
		}
	}
	if got := timeoutInactivityEventCount(&Replay{Turns: []ReplayTurn{{Events: gs.Events}}}); got != 0 {
		t.Fatalf("got %d inactivity events before threshold, want 0", got)
	}
}

func TestTimeoutInactivity_MatchSuccessResetsFailureStreak(t *testing.T) {
	cfg := timeoutInactivityConfig(20)
	failure := errors.New("protocol failure")
	outcomes := make([]timeoutInactivityOutcome, 20)
	for i := 0; i < 20; i++ {
		switch {
		case i < 9, i >= 10 && i < 19:
			outcomes[i].err = failure
		default:
			outcomes[i] = timeoutInactivityOutcome{}
		}
	}
	bot := &timeoutInactivityScriptBot{outcomes: outcomes}
	runner := NewMatchRunner(cfg, WithRNG(rand.New(rand.NewSource(18))), WithTimeout(100*time.Millisecond))
	runner.AddBot(bot, "recovering")
	runner.AddBot(NewIdleBot(), "idle")

	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("run match: %v", err)
	}
	if result.Crashed[0] {
		t.Fatal("successful responses did not reset the failure streak")
	}
	if got := bot.callCount(); got != cfg.MaxTurns {
		t.Fatalf("bot received %d requests, want %d", got, cfg.MaxTurns)
	}
	if got := timeoutInactivityEventCount(replay); got != 0 {
		t.Fatalf("got %d inactivity events, want 0", got)
	}
}

func TestTimeoutInactivity_MatchEmitsReplayEventAndCrashJSON(t *testing.T) {
	cfg := timeoutInactivityConfig(12)
	failure := errors.New("protocol failure")
	outcomes := make([]timeoutInactivityOutcome, BotInactiveAfterFailures)
	for i := range outcomes {
		outcomes[i].err = failure
	}
	bot := &timeoutInactivityScriptBot{outcomes: outcomes}
	runner := NewMatchRunner(cfg, WithRNG(rand.New(rand.NewSource(19))), WithTimeout(100*time.Millisecond))
	runner.AddBot(bot, "inactive")
	runner.AddBot(NewIdleBot(), "idle")

	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("run match: %v", err)
	}
	if len(result.Crashed) != 2 || !result.Crashed[0] || result.Crashed[1] {
		t.Fatalf("result crash state = %v, want [true false]", result.Crashed)
	}
	if result.Reason != "turns" || result.Turns != cfg.MaxTurns {
		t.Fatalf("inactive bot changed match completion: reason=%q turns=%d", result.Reason, result.Turns)
	}
	if got := bot.callCount(); got != BotInactiveAfterFailures {
		t.Fatalf("inactive bot received %d requests, want %d", got, BotInactiveAfterFailures)
	}
	if got := timeoutInactivityEventCount(replay); got != 1 {
		t.Fatalf("got %d inactivity events, want 1", got)
	}

	var inactive Event
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == EventBotInactive {
				inactive = event
			}
		}
	}
	if inactive.Turn != BotInactiveAfterFailures {
		t.Fatalf("inactive event turn = %d, want %d", inactive.Turn, BotInactiveAfterFailures)
	}
	details, ok := inactive.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("inactive event details type = %T, want map", inactive.Details)
	}
	if details["player"] != 0 || details["consecutive_failures"] != BotInactiveAfterFailures || details["reason"] != "error" {
		t.Fatalf("inactive event details = %#v", details)
	}

	lastTurn := replay.Turns[len(replay.Turns)-1]
	for _, botState := range lastTurn.Bots {
		if botState.Owner == 0 && !botState.Alive {
			t.Fatal("inactive bot lost a living unit")
		}
	}

	data, err := ReplayToJSON(replay)
	if err != nil {
		t.Fatalf("serialize replay: %v", err)
	}
	var payload struct {
		Result struct {
			Crashed []bool `json:"crashed"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode replay result: %v", err)
	}
	if len(payload.Result.Crashed) != 2 || !payload.Result.Crashed[0] || payload.Result.Crashed[1] {
		t.Fatalf("serialized crash state = %v, want [true false]", payload.Result.Crashed)
	}
	loaded, err := LoadReplay(data)
	if err != nil {
		t.Fatalf("reload replay: %v", err)
	}
	if loaded.Result == nil || len(loaded.Result.Crashed) != 2 || !loaded.Result.Crashed[0] || loaded.Result.Crashed[1] {
		t.Fatalf("reloaded crash state = %#v, want [true false]", loaded.Result)
	}
}

func TestTimeoutInactivity_DirectTurnResultIncludesCrashArray(t *testing.T) {
	cfg := timeoutInactivityConfig(1)
	gs := timeoutInactivityState(cfg)
	result := gs.createResult(0, "turns")
	if len(result.Crashed) != 2 || result.Crashed[0] || result.Crashed[1] {
		t.Fatalf("direct result crash state = %v, want [false false]", result.Crashed)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var payload struct {
		Crashed []bool `json:"crashed"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(payload.Crashed) != 2 || payload.Crashed[0] || payload.Crashed[1] {
		t.Fatalf("serialized direct result crash state = %v, want [false false]", payload.Crashed)
	}
}
