package engine

import (
	"sync"
	"testing"
	"time"

	"math/rand"
)

// delayedTurnBot lets the runner deadline, rather than the bot itself, decide
// whether a turn is missed. Calls are counted before sleeping so late
// responses from timed-out turns cannot hide a request from the boundary test.
type delayedTurnBot struct {
	mu           sync.Mutex
	calls        int
	timeoutCalls int
	delay        time.Duration
}

func (b *delayedTurnBot) GetMoves(_ *VisibleState) ([]Move, error) {
	b.mu.Lock()
	b.calls++
	call := b.calls
	b.mu.Unlock()
	if call <= b.timeoutCalls {
		time.Sleep(b.delay)
	}
	return []Move{}, nil
}

func (b *delayedTurnBot) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func timeoutPolicyConfig() Config {
	cfg := DefaultConfig()
	cfg.MaxTurns = BotInactiveAfterFailures + 1
	cfg.ZoneEnabled = false
	cfg.CoresPerPlayer = 1
	return cfg
}

func TestMatchRunner_InactivePolicyThresholdBoundary(t *testing.T) {
	t.Run("nine failures recover and remain active", func(t *testing.T) {
		cfg := timeoutPolicyConfig()
		bot := &delayedTurnBot{
			timeoutCalls: BotInactiveAfterFailures - 1,
			delay:        100 * time.Millisecond,
		}
		runner := NewMatchRunner(cfg,
			WithRNG(rand.New(rand.NewSource(1))),
			WithTimeout(5*time.Millisecond),
		)
		runner.AddBot(bot, "recovering")
		runner.AddBot(NewIdleBot(), "idle")

		result, replay, err := runner.Run()
		if err != nil {
			t.Fatalf("run match: %v", err)
		}
		if result.Crashed[0] {
			t.Fatal("bot marked crashed before reaching the threshold")
		}
		if bot.callCount() != cfg.MaxTurns {
			t.Errorf("bot received %d turns, want %d after recovery", bot.callCount(), cfg.MaxTurns)
		}
		for _, turn := range replay.Turns {
			for _, event := range turn.Events {
				if event.Type == EventBotInactive {
					t.Fatalf("recovered bot emitted inactive event on turn %d", turn.Turn)
				}
			}
		}
	})

	t.Run("tenth failure becomes inactive without ending match", func(t *testing.T) {
		cfg := timeoutPolicyConfig()
		bot := &delayedTurnBot{
			timeoutCalls: BotInactiveAfterFailures,
			delay:        100 * time.Millisecond,
		}
		peer := &delayedTurnBot{}
		runner := NewMatchRunner(cfg,
			WithRNG(rand.New(rand.NewSource(2))),
			WithTimeout(5*time.Millisecond),
		)
		runner.AddBot(bot, "inactive")
		runner.AddBot(peer, "peer")

		result, replay, err := runner.Run()
		if err != nil {
			t.Fatalf("run match: %v", err)
		}
		if !result.Crashed[0] {
			t.Fatal("bot was not marked crashed at the threshold")
		}
		if result.Turns != cfg.MaxTurns || result.Reason != "turns" {
			t.Fatalf("inactive bot short-circuited match: reason=%q turns=%d", result.Reason, result.Turns)
		}
		if got := bot.callCount(); got != BotInactiveAfterFailures {
			t.Errorf("inactive bot received %d requests, want %d", got, BotInactiveAfterFailures)
		}
		if got := peer.callCount(); got != cfg.MaxTurns {
			t.Errorf("peer received %d turns, want %d", got, cfg.MaxTurns)
		}

		var inactiveEvents []Event
		for _, turn := range replay.Turns {
			for _, event := range turn.Events {
				if event.Type == EventBotInactive {
					inactiveEvents = append(inactiveEvents, event)
				}
			}
		}
		if len(inactiveEvents) != 1 {
			t.Fatalf("replay contains %d bot_inactive events, want 1", len(inactiveEvents))
		}
		event := inactiveEvents[0]
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

		lastTurn := replay.Turns[len(replay.Turns)-1]
		for _, botState := range lastTurn.Bots {
			if botState.Owner == 0 && !botState.Alive {
				t.Fatal("marking a bot inactive must not kill its living units")
			}
		}
	})
}
