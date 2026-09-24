package engine

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/aicodebattle/acb/internal/tier"
)

// slowBot sleeps before answering, standing in for a bot whose host is slow
// or far away. Used to pin that the per-turn timeout is actually enforced.
type slowBot struct {
	delay time.Duration
}

func (b *slowBot) GetMoves(state *VisibleState) ([]Move, error) {
	time.Sleep(b.delay)
	return []Move{{Direction: DirNone}}, nil
}

func TestNewMatchRunner_TurnTimeoutSelection(t *testing.T) {
	tests := []struct {
		name       string
		cfgTimeout time.Duration
		optTimeout time.Duration
		want       time.Duration
	}{
		{name: "positive config wins over option", cfgTimeout: 5 * time.Second, optTimeout: 30 * time.Second, want: 5 * time.Second},
		{name: "zero config uses historical default", cfgTimeout: 0, want: 3 * time.Second},
		{name: "negative config uses historical default", cfgTimeout: -time.Second, want: 3 * time.Second},
		{name: "unset config preserves option", optTimeout: 30 * time.Second, want: 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConfigForPlayers(2, 1)
			cfg.TurnTimeout = tt.cfgTimeout
			var mr *MatchRunner
			if tt.optTimeout > 0 {
				mr = NewMatchRunner(cfg, WithTimeout(tt.optTimeout))
			} else {
				mr = NewMatchRunner(cfg)
			}
			if mr.timeout != tt.want {
				t.Errorf("runner timeout = %v, want %v", mr.timeout, tt.want)
			}
		})
	}
}

func TestConfigTurnTimeoutJSON(t *testing.T) {
	data, err := json.Marshal(Config{TurnTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		TurnTimeout time.Duration `json:"turn_timeout"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.TurnTimeout != time.Second {
		t.Errorf("turn_timeout = %v, want 1s", got.TurnTimeout)
	}
}

// TestNewMatchRunner_TierConfigApplied pins that every defined tournament
// tier, resolved into a Config the way the matchmaker does, lands on the
// runner's per-turn budget.
func TestNewMatchRunner_TierConfigApplied(t *testing.T) {
	for _, tr := range []tier.Tier{tier.Casual, tier.Competitive, tier.Speed} {
		cfg := ConfigForPlayers(2, 1)
		cfg.TurnTimeout = tr.TurnTimeout()
		mr := NewMatchRunner(cfg, WithTimeout(30*time.Second)) // decoy fallback
		if got := mr.timeout; got != tr.TurnTimeout() {
			t.Errorf("tier %q: runner timeout = %v, want %v", tr, got, tr.TurnTimeout())
		}
	}
}

// testTurnTimeoutState builds a two-player game state to drive
// getMovesFromBots without running a full match.
func testTurnTimeoutState(t *testing.T, cfg Config) *GameState {
	t.Helper()
	gs := NewGameState(cfg, rand.New(rand.NewSource(1)))
	gs.AddPlayer()
	gs.AddPlayer()
	return gs
}

// TestGetMoves_TimeoutDiscardsSlowResponse pins enforcement itself: a bot
// that answers inside the budget has its moves accepted; the same bot
// answering outside it has that turn's response discarded (units hold
// position), without killing the bot.
func TestGetMoves_TimeoutDiscardsSlowResponse(t *testing.T) {
	cfg := ConfigForPlayers(2, 1)
	cfg.TurnTimeout = 150 * time.Millisecond
	gs := testTurnTimeoutState(t, cfg)

	// Player 0 answers in time, player 1 sleeps past the budget.
	mr := NewMatchRunner(cfg)
	mr.AddBot(&slowBot{delay: 0}, "fast")
	mr.AddBot(&slowBot{delay: 400 * time.Millisecond}, "slow")

	moves := mr.getMovesFromBots(gs)

	if _, ok := moves[0]; !ok {
		t.Error("bot answering within the budget should have its moves accepted")
	}
	if _, ok := moves[1]; ok {
		t.Error("bot answering past the budget should have its response discarded")
	}
}

func TestGetMoves_TierBudgetEnforced(t *testing.T) {
	tests := []struct {
		name string
		tier tier.Tier
		want time.Duration
	}{
		{name: "casual", tier: tier.Casual, want: 5 * time.Second},
		{name: "competitive", tier: tier.Competitive, want: 3 * time.Second},
		{name: "speed", tier: tier.Speed, want: 1 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := ConfigForPlayers(2, 1)
			cfg.TurnTimeout = tt.tier.TurnTimeout()
			if cfg.TurnTimeout != tt.want {
				t.Fatalf("tier budget = %v, want %v", cfg.TurnTimeout, tt.want)
			}
			gs := testTurnTimeoutState(t, cfg)

			mr := NewMatchRunner(cfg)
			mr.AddBot(&slowBot{delay: tt.want + 200*time.Millisecond}, "misses-deadline")
			mr.AddBot(&slowBot{delay: 0}, "in-time")

			moves := mr.getMovesFromBots(gs)

			if _, ok := moves[0]; ok {
				t.Errorf("bot missing the %v deadline should have its response discarded", tt.want)
			}
			if _, ok := moves[1]; !ok {
				t.Error("bot answering within the tier budget should have its moves accepted")
			}
		})
	}
}

// TestGetMoves_ConfigBudgetBeatsOption pins that the per-match config budget
// is what is actually enforced — a generous deployment fallback must not
// rescue a bot that missed its tier's deadline.
func TestGetMoves_ConfigBudgetBeatsOption(t *testing.T) {
	cfg := ConfigForPlayers(2, 1)
	cfg.TurnTimeout = 100 * time.Millisecond
	gs := testTurnTimeoutState(t, cfg)

	mr := NewMatchRunner(cfg, WithTimeout(30*time.Second))
	mr.AddBot(&slowBot{delay: 300 * time.Millisecond}, "misses-tier")
	mr.AddBot(&slowBot{delay: 0}, "in-time")

	moves := mr.getMovesFromBots(gs)

	if _, ok := moves[0]; ok {
		t.Error("deployment fallback must not override the per-match tier budget")
	}
	if _, ok := moves[1]; !ok {
		t.Error("in-time bot should keep its moves")
	}
}
