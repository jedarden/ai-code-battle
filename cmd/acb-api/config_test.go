package main

import (
	"os"
	"testing"
)

func TestEnvOr(t *testing.T) {
	// Unset var should return fallback
	os.Unsetenv("TEST_ENV_OR_VAR")
	if got := envOr("TEST_ENV_OR_VAR", "default"); got != "default" {
		t.Errorf("envOr unset = %q, want 'default'", got)
	}

	// Set var should return value
	os.Setenv("TEST_ENV_OR_VAR", "custom")
	defer os.Unsetenv("TEST_ENV_OR_VAR")
	if got := envOr("TEST_ENV_OR_VAR", "default"); got != "custom" {
		t.Errorf("envOr set = %q, want 'custom'", got)
	}
}

func TestEnvInt(t *testing.T) {
	// Unset var should return fallback
	os.Unsetenv("TEST_ENV_INT_VAR")
	if got := envInt("TEST_ENV_INT_VAR", 42); got != 42 {
		t.Errorf("envInt unset = %d, want 42", got)
	}

	// Valid int
	os.Setenv("TEST_ENV_INT_VAR", "100")
	defer os.Unsetenv("TEST_ENV_INT_VAR")
	if got := envInt("TEST_ENV_INT_VAR", 42); got != 100 {
		t.Errorf("envInt valid = %d, want 100", got)
	}

	// Invalid int should return fallback
	os.Setenv("TEST_ENV_INT_VAR", "notanumber")
	if got := envInt("TEST_ENV_INT_VAR", 42); got != 42 {
		t.Errorf("envInt invalid = %d, want 42", got)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all relevant env vars
	for _, key := range []string{
		"ACB_LISTEN_ADDR", "ACB_DATABASE_URL", "ACB_VALKEY_ADDR",
		"ACB_MATCHMAKER_INTERVAL", "ACB_HEALTHCHECK_INTERVAL", "ACB_REAPER_INTERVAL",
	} {
		os.Unsetenv(key)
	}

	cfg := loadConfig()

	if cfg.ListenAddr != ":8080" {
		t.Errorf("default ListenAddr = %q, want ':8080'", cfg.ListenAddr)
	}
	if cfg.MatchmakerSecs != 60 {
		t.Errorf("default MatchmakerSecs = %d, want 60", cfg.MatchmakerSecs)
	}
	if cfg.HealthCheckSecs != 900 {
		t.Errorf("default HealthCheckSecs = %d, want 900", cfg.HealthCheckSecs)
	}
	if cfg.ReaperSecs != 300 {
		t.Errorf("default ReaperSecs = %d, want 300", cfg.ReaperSecs)
	}
	if cfg.BotTimeoutSecs != 5 {
		t.Errorf("default BotTimeoutSecs = %d, want 5", cfg.BotTimeoutSecs)
	}
	if cfg.StaleJobMinutes != 15 {
		t.Errorf("default StaleJobMinutes = %d, want 15", cfg.StaleJobMinutes)
	}
	if cfg.MaxConsecFails != 3 {
		t.Errorf("default MaxConsecFails = %d, want 3", cfg.MaxConsecFails)
	}
}
