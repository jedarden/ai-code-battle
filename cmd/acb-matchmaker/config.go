package main

import (
	"os"
	"strconv"
	"time"

	"github.com/aicodebattle/acb/internal/tier"
)

type matchBotConfig struct {
	BotID    string `json:"bot_id"`
	Endpoint string `json:"endpoint"`
	Secret   string `json:"secret"`
	Slot     int    `json:"slot"`
}

type jobConfig struct {
	MatchID       string           `json:"match_id"`
	SeriesID      int64            `json:"series_id,omitempty"`
	GameNum       int              `json:"game_num,omitempty"`
	MapSeed       int64            `json:"map_seed"`
	MaxTurns      int              `json:"max_turns"`
	Rows          int              `json:"rows"`
	Cols          int              `json:"cols"`
	Tier          string           `json:"tier"`
	TurnTimeoutMs int64            `json:"turn_timeout_ms"`
	Bots          []matchBotConfig `json:"bots"`
}

func matchTiming(cfg Config) (tier.Tier, time.Duration, error) {
	t, err := tier.Parse(cfg.MatchTier)
	if err != nil {
		return tier.Default, tier.Default.TurnTimeout(), err
	}
	return t, t.TurnTimeout(), nil
}

func validateMatchTiming(cfg Config) error {
	_, _, err := matchTiming(cfg)
	return err
}

func matchTimingForMatch(cfg Config) (tier.Tier, time.Duration, error) {
	if value, ok := os.LookupEnv("ACB_MATCH_TIER"); ok {
		return matchTiming(Config{MatchTier: value})
	}
	return matchTiming(cfg)
}

func (c *jobConfig) applyMatchTiming(cfg Config) error {
	t, timeout, err := matchTimingForMatch(cfg)
	if err != nil {
		return err
	}
	c.Tier = string(t)
	c.TurnTimeoutMs = timeout.Milliseconds()
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}
