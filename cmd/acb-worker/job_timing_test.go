package main

import (
	"testing"
	"time"

	"github.com/aicodebattle/acb/engine"
	"github.com/aicodebattle/acb/internal/tier"
)

func TestResolveTurnTimeout(t *testing.T) {
	deployDefault := 3 * time.Second

	tests := []struct {
		name          string
		turnTimeoutMs int64
		fallback      time.Duration
		want          time.Duration
	}{{
		name:          "no job value falls back to deployment default",
		turnTimeoutMs: 0,
		fallback:      deployDefault,
		want:          3 * time.Second,
	}, {
		name:          "casual tier value wins over deployment default",
		turnTimeoutMs: tier.Casual.TurnTimeout().Milliseconds(),
		fallback:      deployDefault,
		want:          5 * time.Second,
	}, {
		name:          "competitive tier value matches deployment default",
		turnTimeoutMs: tier.Competitive.TurnTimeout().Milliseconds(),
		fallback:      deployDefault,
		want:          3 * time.Second,
	}, {
		name:          "speed tier value wins over deployment default",
		turnTimeoutMs: tier.Speed.TurnTimeout().Milliseconds(),
		fallback:      deployDefault,
		want:          1 * time.Second,
	}, {
		name:          "negative job value is treated as unset",
		turnTimeoutMs: -1,
		fallback:      deployDefault,
		want:          3 * time.Second,
	}, {
		name:          "non-default deployment default still honored for pre-tier rows",
		turnTimeoutMs: 0,
		fallback:      10 * time.Second,
		want:          10 * time.Second,
	}, {
		name:          "missing deployment fallback uses engine default",
		turnTimeoutMs: 0,
		fallback:      0,
		want:          engine.DefaultTurnTimeout,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTurnTimeout(tt.turnTimeoutMs, tt.fallback)
			if got != tt.want {
				t.Errorf("resolveTurnTimeout(%d, %v) = %v, want %v", tt.turnTimeoutMs, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestJobConfigJSONDecodesSnakeCase(t *testing.T) {
	in := `{"match_id":"m_abc","max_turns":500,"tier":"speed","turn_timeout_ms":1000,"bots":[]}`

	timing, err := extractJobTiming([]byte(in))
	if err != nil {
		t.Fatalf("extractJobTiming(valid config) unexpected error: %v", err)
	}
	if timing.Tier != "speed" {
		t.Errorf("tier = %q, want %q", timing.Tier, "speed")
	}
	if timing.TurnTimeoutMs != 1000 {
		t.Errorf("turn_timeout_ms = %d, want 1000", timing.TurnTimeoutMs)
	}

	timing, err = extractJobTiming([]byte(`{"match_id":"m_old","max_turns":500,"bots":[]}`))
	if err != nil {
		t.Fatalf("extractJobTiming(pre-tier config) unexpected error: %v", err)
	}
	if timing.Tier != "" || timing.TurnTimeoutMs != 0 {
		t.Errorf("pre-tier row should decode to zero values, got %+v", timing)
	}
	if got := resolveTurnTimeout(timing.TurnTimeoutMs, 3*time.Second); got != 3*time.Second {
		t.Errorf("pre-tier row timeout = %v, want deployment default 3s", got)
	}

	timing, err = extractJobTiming([]byte(`{"match_id":"m_zero","turn_timeout_ms":0,"bots":[]}`))
	if err != nil {
		t.Fatalf("extractJobTiming(zero config) unexpected error: %v", err)
	}
	if got := resolveTurnTimeout(timing.TurnTimeoutMs, 0); got != engine.DefaultTurnTimeout {
		t.Errorf("zero job timeout = %v, want engine default %v", got, engine.DefaultTurnTimeout)
	}
}
