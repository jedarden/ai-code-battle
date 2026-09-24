package tier

import (
	"testing"
	"time"
)

// TestTurnTimeout_PerTier pins the timeout budget each tournament tier grants.
// These are the product values from requirements.md — changing one is a
// product decision, not a refactor.
func TestTurnTimeout_PerTier(t *testing.T) {
	tests := []struct {
		tier Tier
		want time.Duration
	}{
		{Casual, 5 * time.Second},
		{Competitive, 3 * time.Second},
		{Speed, 1 * time.Second},
	}
	for _, tt := range tests {
		if got := tt.tier.TurnTimeout(); got != tt.want {
			t.Errorf("%q tier timeout = %v, want %v", tt.tier, got, tt.want)
		}
	}
}

// TestTurnTimeout_UnknownFallsBackToDefault pins that unknown and empty tier
// values resolve to the competitive budget — the timing the platform has
// always enforced — rather than zeroing out or panicking.
func TestTurnTimeout_UnknownFallsBackToDefault(t *testing.T) {
	for _, name := range []string{"", "Turbo", "CASUAL", "speed-tier"} {
		got := Tier(name).TurnTimeout()
		if got != Default.TurnTimeout() {
			t.Errorf("unknown tier %q timeout = %v, want default %v", name, got, Default.TurnTimeout())
		}
	}
	if Default != Competitive {
		t.Errorf("default tier = %q, want competitive", Default)
	}
	if Default.TurnTimeout() != 3*time.Second {
		t.Errorf("default timeout = %v, want 3s (backward compatibility)", Default.TurnTimeout())
	}
}

func TestValid(t *testing.T) {
	for _, name := range []string{"casual", "competitive", "speed"} {
		if !Tier(name).Valid() {
			t.Errorf("tier %q should be valid", name)
		}
	}
	for _, name := range []string{"", "Turbo", "blitz"} {
		if Tier(name).Valid() {
			t.Errorf("tier %q should not be valid", name)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Tier
	}{{
		in:   "",
		want: Competitive, // empty config = default, no error
	}, {
		in:   "casual",
		want: Casual,
	}, {
		in:   "competitive",
		want: Competitive,
	}, {
		in:   "speed",
		want: Speed,
	}}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	got, err := Parse("turbo")
	if err == nil {
		t.Error("Parse(turbo) should error on an unknown tier")
	}
	if got != Default {
		t.Errorf("Parse(turbo) = %q, want default %q", got, Default)
	}
}
