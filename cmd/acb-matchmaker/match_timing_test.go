package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/aicodebattle/acb/internal/tier"
)

func TestMatchTiming_Tiers(t *testing.T) {
	tests := []struct {
		env      string
		wantTier tier.Tier
		want     time.Duration
		wantErr  bool
	}{
		{env: "", wantTier: tier.Competitive, want: 3 * time.Second},
		{env: "casual", wantTier: tier.Casual, want: 5 * time.Second},
		{env: "competitive", wantTier: tier.Competitive, want: 3 * time.Second},
		{env: "speed", wantTier: tier.Speed, want: 1 * time.Second},
		{env: "turbo", wantErr: true},
	}
	for _, tt := range tests {
		gotTier, gotTimeout, err := matchTiming(Config{MatchTier: tt.env})
		if (err != nil) != tt.wantErr {
			t.Errorf("matchTiming(%q) error = %v, wantErr %v", tt.env, err, tt.wantErr)
			continue
		}
		if tt.wantErr {
			continue
		}
		if gotTier != tt.wantTier {
			t.Errorf("matchTiming(%q) tier = %q, want %q", tt.env, gotTier, tt.wantTier)
		}
		if gotTimeout != tt.want {
			t.Errorf("matchTiming(%q) timeout = %v, want %v", tt.env, gotTimeout, tt.want)
		}
	}
}

func TestMatchTiming_StampsJobConfigJSON(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		wantTier    string
		wantTimeout int64
	}{
		{name: "casual", env: "casual", wantTier: "casual", wantTimeout: 5000},
		{name: "competitive", env: "competitive", wantTier: "competitive", wantTimeout: 3000},
		{name: "speed", env: "speed", wantTier: "speed", wantTimeout: 1000},
		{name: "empty", env: "", wantTier: "competitive", wantTimeout: 3000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ACB_MATCH_TIER", tt.env)
			config := jobConfig{
				MatchID:  "m_test",
				MapSeed:  7,
				MaxTurns: 500,
				Rows:     40,
				Cols:     40,
				Bots:     []matchBotConfig{{BotID: "bot_a", Slot: 0}, {BotID: "bot_b", Slot: 1}},
			}
			if err := config.applyMatchTiming(Config{}); err != nil {
				t.Fatalf("applyMatchTiming: %v", err)
			}
			data, err := json.Marshal(config)
			if err != nil {
				t.Fatalf("marshal job config: %v", err)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal job config: %v", err)
			}
			if _, ok := got["tier"]; !ok {
				t.Fatal("job config is missing tier")
			}
			if _, ok := got["turn_timeout_ms"]; !ok {
				t.Fatal("job config is missing turn_timeout_ms")
			}
			var gotTier string
			if err := json.Unmarshal(got["tier"], &gotTier); err != nil {
				t.Fatalf("decode tier: %v", err)
			}
			var gotTimeout int64
			if err := json.Unmarshal(got["turn_timeout_ms"], &gotTimeout); err != nil {
				t.Fatalf("decode turn_timeout_ms: %v", err)
			}
			if gotTier != tt.wantTier {
				t.Errorf("job config tier = %q, want %q", gotTier, tt.wantTier)
			}
			if gotTimeout != tt.wantTimeout {
				t.Errorf("job config turn_timeout_ms = %d, want %d", gotTimeout, tt.wantTimeout)
			}
		})
	}
}

func TestMatchTiming_UnsetUsesCompetitive(t *testing.T) {
	original, wasSet := os.LookupEnv("ACB_MATCH_TIER")
	if err := os.Unsetenv("ACB_MATCH_TIER"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv("ACB_MATCH_TIER", original)
		} else {
			_ = os.Unsetenv("ACB_MATCH_TIER")
		}
	})

	config := jobConfig{}
	if err := config.applyMatchTiming(Config{}); err != nil {
		t.Fatalf("applyMatchTiming: %v", err)
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal job config: %v", err)
	}
	var got struct {
		Tier          string `json:"tier"`
		TurnTimeoutMs int64  `json:"turn_timeout_ms"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal job config: %v", err)
	}
	if got.Tier != string(tier.Competitive) || got.TurnTimeoutMs != 3000 {
		t.Fatalf("unset tier job config = %s/%dms, want competitive/3000ms", got.Tier, got.TurnTimeoutMs)
	}
}

func TestMatchTiming_ChangedTierAppliesToNextJob(t *testing.T) {
	t.Setenv("ACB_MATCH_TIER", "casual")
	first := jobConfig{}
	if err := first.applyMatchTiming(Config{}); err != nil {
		t.Fatalf("first applyMatchTiming: %v", err)
	}

	t.Setenv("ACB_MATCH_TIER", "speed")
	second := jobConfig{}
	if err := second.applyMatchTiming(Config{}); err != nil {
		t.Fatalf("second applyMatchTiming: %v", err)
	}

	if first.Tier != "casual" || first.TurnTimeoutMs != 5000 {
		t.Fatalf("first job timing = %s/%dms, want casual/5000ms", first.Tier, first.TurnTimeoutMs)
	}
	if second.Tier != "speed" || second.TurnTimeoutMs != 1000 {
		t.Fatalf("second job timing = %s/%dms, want speed/1000ms", second.Tier, second.TurnTimeoutMs)
	}
}

func TestMatchTiming_InvalidTierFailsStartupValidation(t *testing.T) {
	t.Setenv("ACB_MATCH_TIER", "turbo")
	cfg := loadConfig()
	if err := validateMatchTiming(cfg); err == nil {
		t.Fatal("startup validation accepted an unknown tier")
	}
}
