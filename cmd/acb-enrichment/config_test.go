package main

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all env vars
	clearEnvs()

	cfg := LoadConfig()

	// Check defaults
	if cfg.PostgresHost != "localhost" {
		t.Errorf("Expected PostgresHost 'localhost', got %s", cfg.PostgresHost)
	}
	if cfg.PostgresPort != 5432 {
		t.Errorf("Expected PostgresPort 5432, got %d", cfg.PostgresPort)
	}
	if cfg.PostgresUser != "acb" {
		t.Errorf("Expected PostgresUser 'acb', got %s", cfg.PostgresUser)
	}
	if cfg.PostgresDatabase != "ai_code_battle" {
		t.Errorf("Expected PostgresDatabase 'ai_code_battle', got %s", cfg.PostgresDatabase)
	}
	if cfg.LLMBaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected LLMBaseURL 'https://api.openai.com/v1', got %s", cfg.LLMBaseURL)
	}
	if cfg.LLMModel != "gpt-4o-mini" {
		t.Errorf("Expected LLMModel 'gpt-4o-mini', got %s", cfg.LLMModel)
	}
	if cfg.LLMMaxTokens != 2000 {
		t.Errorf("Expected LLMMaxTokens 2000, got %d", cfg.LLMMaxTokens)
	}
	if cfg.LLMTemperature != 0.7 {
		t.Errorf("Expected LLMTemperature 0.7, got %f", cfg.LLMTemperature)
	}
	if cfg.MaxEnrichmentsPerHour != 20 {
		t.Errorf("Expected MaxEnrichmentsPerHour 20, got %d", cfg.MaxEnrichmentsPerHour)
	}
	if cfg.MaxConcurrentRequests != 3 {
		t.Errorf("Expected MaxConcurrentRequests 3, got %d", cfg.MaxConcurrentRequests)
	}
	if cfg.MinTurnCount != 100 {
		t.Errorf("Expected MinTurnCount 100, got %d", cfg.MinTurnCount)
	}
	if cfg.MinWinProbCrossings != 3 {
		t.Errorf("Expected MinWinProbCrossings 3, got %d", cfg.MinWinProbCrossings)
	}
	if cfg.UpsetThreshold != 150.0 {
		t.Errorf("Expected UpsetThreshold 150.0, got %f", cfg.UpsetThreshold)
	}
	if cfg.CycleInterval != 30*time.Minute {
		t.Errorf("Expected CycleInterval 30m, got %v", cfg.CycleInterval)
	}
	if cfg.CycleTimeout != 25*time.Minute {
		t.Errorf("Expected CycleTimeout 25m, got %v", cfg.CycleTimeout)
	}
	if cfg.MaxLifetime != 4*time.Hour {
		t.Errorf("Expected MaxLifetime 4h, got %v", cfg.MaxLifetime)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	// Set custom env vars
	os.Setenv("ACB_POSTGRES_HOST", "db.example.com")
	os.Setenv("ACB_POSTGRES_PORT", "5433")
	os.Setenv("ACB_POSTGRES_USER", "testuser")
	os.Setenv("ACB_POSTGRES_PASSWORD", "secret")
	os.Setenv("ACB_POSTGRES_DATABASE", "testdb")
	os.Setenv("ACB_DATABASE_NAME", "test_acb")
	os.Setenv("ACB_LLM_BASE_URL", "https://api.example.com/v1")
	os.Setenv("ACB_LLM_API_KEY", "sk-test-key")
	os.Setenv("ACB_LLM_MODEL", "gpt-4")
	os.Setenv("ACB_LLM_MAX_TOKENS", "4000")
	os.Setenv("ACB_LLM_TEMPERATURE", "0.5")
	os.Setenv("ACB_ENRICHMENT_MAX_PER_HOUR", "50")
	os.Setenv("ACB_ENRICHMENT_MAX_CONCURRENT", "5")
	defer clearEnvs()

	cfg := LoadConfig()

	// Check env overrides
	if cfg.PostgresHost != "db.example.com" {
		t.Errorf("Expected PostgresHost 'db.example.com', got %s", cfg.PostgresHost)
	}
	if cfg.PostgresPort != 5433 {
		t.Errorf("Expected PostgresPort 5433, got %d", cfg.PostgresPort)
	}
	if cfg.PostgresUser != "testuser" {
		t.Errorf("Expected PostgresUser 'testuser', got %s", cfg.PostgresUser)
	}
	if cfg.PostgresPassword != "secret" {
		t.Errorf("Expected PostgresPassword 'secret', got %s", cfg.PostgresPassword)
	}
	if cfg.PostgresDatabase != "testdb" {
		t.Errorf("Expected PostgresDatabase 'testdb', got %s", cfg.PostgresDatabase)
	}
	if cfg.DatabaseName != "test_acb" {
		t.Errorf("Expected DatabaseName 'test_acb', got %s", cfg.DatabaseName)
	}
	if cfg.LLMBaseURL != "https://api.example.com/v1" {
		t.Errorf("Expected LLMBaseURL 'https://api.example.com/v1', got %s", cfg.LLMBaseURL)
	}
	if cfg.LLMAPIKey != "sk-test-key" {
		t.Errorf("Expected LLMAPIKey 'sk-test-key', got %s", cfg.LLMAPIKey)
	}
	if cfg.LLMModel != "gpt-4" {
		t.Errorf("Expected LLMModel 'gpt-4', got %s", cfg.LLMModel)
	}
	if cfg.LLMMaxTokens != 4000 {
		t.Errorf("Expected LLMMaxTokens 4000, got %d", cfg.LLMMaxTokens)
	}
	if cfg.LLMTemperature != 0.5 {
		t.Errorf("Expected LLMTemperature 0.5, got %f", cfg.LLMTemperature)
	}
	if cfg.MaxEnrichmentsPerHour != 50 {
		t.Errorf("Expected MaxEnrichmentsPerHour 50, got %d", cfg.MaxEnrichmentsPerHour)
	}
	if cfg.MaxConcurrentRequests != 5 {
		t.Errorf("Expected MaxConcurrentRequests 5, got %d", cfg.MaxConcurrentRequests)
	}
}

func TestLoadConfig_StorageDefaults(t *testing.T) {
	clearEnvs()

	cfg := LoadConfig()

	// B2 defaults
	if cfg.B2BucketName != "ai-code-battle" {
		t.Errorf("Expected B2BucketName 'ai-code-battle', got %s", cfg.B2BucketName)
	}
	if cfg.B2Endpoint != "https://s3.us-west-004.backblazeb2.com" {
		t.Errorf("Expected B2Endpoint 'https://s3.us-west-004.backblazeb2.com', got %s", cfg.B2Endpoint)
	}

	// R2 defaults
	if cfg.R2BucketName != "ai-code-battle" {
		t.Errorf("Expected R2BucketName 'ai-code-battle', got %s", cfg.R2BucketName)
	}
	if cfg.R2Endpoint != "https://r2.cloudflarestorage.com" {
		t.Errorf("Expected R2Endpoint 'https://r2.cloudflarestorage.com', got %s", cfg.R2Endpoint)
	}
}

func TestLoadConfig_StorageFromEnv(t *testing.T) {
	os.Setenv("ACB_B2_BUCKET", "test-b2-bucket")
	os.Setenv("ACB_B2_ACCESS_KEY_ID", "b2-key-id")
	os.Setenv("ACB_B2_SECRET_ACCESS_KEY", "b2-secret")
	os.Setenv("ACB_B2_ENDPOINT", "https://b2.example.com")
	os.Setenv("ACB_R2_BUCKET", "test-r2-bucket")
	os.Setenv("ACB_R2_ACCESS_KEY_ID", "r2-key-id")
	os.Setenv("ACB_R2_SECRET_ACCESS_KEY", "r2-secret")
	os.Setenv("ACB_R2_ENDPOINT", "https://r2.example.com")
	defer clearEnvs()

	cfg := LoadConfig()

	// B2 env overrides
	if cfg.B2BucketName != "test-b2-bucket" {
		t.Errorf("Expected B2BucketName 'test-b2-bucket', got %s", cfg.B2BucketName)
	}
	if cfg.B2AccessKeyID != "b2-key-id" {
		t.Errorf("Expected B2AccessKeyID 'b2-key-id', got %s", cfg.B2AccessKeyID)
	}
	if cfg.B2SecretAccessKey != "b2-secret" {
		t.Errorf("Expected B2SecretAccessKey 'b2-secret', got %s", cfg.B2SecretAccessKey)
	}
	if cfg.B2Endpoint != "https://b2.example.com" {
		t.Errorf("Expected B2Endpoint 'https://b2.example.com', got %s", cfg.B2Endpoint)
	}

	// R2 env overrides
	if cfg.R2BucketName != "test-r2-bucket" {
		t.Errorf("Expected R2BucketName 'test-r2-bucket', got %s", cfg.R2BucketName)
	}
	if cfg.R2AccessKeyID != "r2-key-id" {
		t.Errorf("Expected R2AccessKeyID 'r2-key-id', got %s", cfg.R2AccessKeyID)
	}
	if cfg.R2SecretAccessKey != "r2-secret" {
		t.Errorf("Expected R2SecretAccessKey 'r2-secret', got %s", cfg.R2SecretAccessKey)
	}
	if cfg.R2Endpoint != "https://r2.example.com" {
		t.Errorf("Expected R2Endpoint 'https://r2.example.com', got %s", cfg.R2Endpoint)
	}
}

func TestLoadConfig_EnrichmentCriteria(t *testing.T) {
	os.Setenv("ACB_ENRICHMENT_MIN_TURNS", "200")
	os.Setenv("ACB_ENRICHMENT_MIN_CROSSINGS", "5")
	os.Setenv("ACB_ENRICHMENT_UPSET_THRESHOLD", "200")
	defer clearEnvs()

	cfg := LoadConfig()

	if cfg.MinTurnCount != 200 {
		t.Errorf("Expected MinTurnCount 200, got %d", cfg.MinTurnCount)
	}
	if cfg.MinWinProbCrossings != 5 {
		t.Errorf("Expected MinWinProbCrossings 5, got %d", cfg.MinWinProbCrossings)
	}
	if cfg.UpsetThreshold != 200.0 {
		t.Errorf("Expected UpsetThreshold 200.0, got %f", cfg.UpsetThreshold)
	}
}

func TestLoadConfig_Durations(t *testing.T) {
	os.Setenv("ACB_ENRICHMENT_INTERVAL", "15m")
	os.Setenv("ACB_ENRICHMENT_TIMEOUT", "10m")
	os.Setenv("ACB_ENRICHMENT_MAX_LIFETIME", "2h")
	defer clearEnvs()

	cfg := LoadConfig()

	if cfg.CycleInterval != 15*time.Minute {
		t.Errorf("Expected CycleInterval 15m, got %v", cfg.CycleInterval)
	}
	if cfg.CycleTimeout != 10*time.Minute {
		t.Errorf("Expected CycleTimeout 10m, got %v", cfg.CycleTimeout)
	}
	if cfg.MaxLifetime != 2*time.Hour {
		t.Errorf("Expected MaxLifetime 2h, got %v", cfg.MaxLifetime)
	}
}

func TestLoadConfig_InvalidEnvValues(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		want     interface{}
		validate func(*testing.T, interface{})
	}{
		{
			name:     "invalid port uses default",
			envKey:   "ACB_POSTGRES_PORT",
			envValue: "invalid",
			want:     5432,
			validate: func(t *testing.T, got interface{}) {
				port := got.(int)
				if port != 5432 {
					t.Errorf("Expected default port 5432 for invalid input, got %d", port)
				}
			},
		},
		{
			name:     "invalid float uses default",
			envKey:   "ACB_LLM_TEMPERATURE",
			envValue: "invalid",
			want:     0.7,
			validate: func(t *testing.T, got interface{}) {
				temp := got.(float64)
				if temp != 0.7 {
					t.Errorf("Expected default temp 0.7 for invalid input, got %f", temp)
				}
			},
		},
		{
			name:     "invalid duration uses default",
			envKey:   "ACB_ENRICHMENT_INTERVAL",
			envValue: "invalid",
			want:     30 * time.Minute,
			validate: func(t *testing.T, got interface{}) {
				dur := got.(time.Duration)
				if dur != 30*time.Minute {
					t.Errorf("Expected default 30m for invalid input, got %v", dur)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvs()
			os.Setenv(tt.envKey, tt.envValue)
			cfg := LoadConfig()

			switch tt.want.(type) {
			case int:
				var port int
				if tt.envKey == "ACB_POSTGRES_PORT" {
					port = cfg.PostgresPort
				}
				tt.validate(t, port)
			case float64:
				tt.validate(t, cfg.LLMTemperature)
			case time.Duration:
				tt.validate(t, cfg.CycleInterval)
			}
		})
	}
}

func TestDatabaseURL(t *testing.T) {
	clearEnvs()
	os.Setenv("ACB_POSTGRES_PASSWORD", "testpass")

	cfg := LoadConfig()
	url := cfg.DatabaseURL()

	expected := "host=localhost port=5432 user=acb password=testpass dbname=ai_code_battle sslmode=disable"
	if url != expected {
		t.Errorf("Expected DatabaseURL '%s', got '%s'", expected, url)
	}
}

func clearEnvs() {
	envs := []string{
		"ACB_POSTGRES_HOST", "ACB_POSTGRES_PORT", "ACB_POSTGRES_USER", "ACB_POSTGRES_PASSWORD", "ACB_POSTGRES_DATABASE", "ACB_DATABASE_NAME",
		"ACB_LLM_BASE_URL", "ACB_LLM_API_KEY", "ACB_LLM_MODEL", "ACB_LLM_MAX_TOKENS", "ACB_LLM_TEMPERATURE",
		"ACB_ENRICHMENT_MAX_PER_HOUR", "ACB_ENRICHMENT_MAX_CONCURRENT", "ACB_ENRICHMENT_MIN_TURNS", "ACB_ENRICHMENT_MIN_CROSSINGS", "ACB_ENRICHMENT_UPSET_THRESHOLD",
		"ACB_ENRICHMENT_INTERVAL", "ACB_ENRICHMENT_TIMEOUT", "ACB_ENRICHMENT_MAX_LIFETIME",
		"ACB_B2_BUCKET", "ACB_B2_ACCESS_KEY_ID", "ACB_B2_SECRET_ACCESS_KEY", "ACB_B2_ENDPOINT",
		"ACB_R2_BUCKET", "ACB_R2_ACCESS_KEY_ID", "ACB_R2_SECRET_ACCESS_KEY", "ACB_R2_ENDPOINT",
	}
	for _, env := range envs {
		os.Unsetenv(env)
	}
}
