package main

import (
	"fmt"
	"os"
	"time"
)

// Config holds all configuration for the enrichment service.
type Config struct {
	// Database
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDatabase string
	DatabaseName     string

	// LLM
	LLMBaseURL     string
	LLMAPIKey      string
	LLMModel       string // Model to use for commentary (e.g., "gpt-4o-mini", "claude-3-haiku")
	LLMMaxTokens   int
	LLMTemperature float64

	// Rate limiting
	MaxEnrichmentsPerHour int // Maximum number of enrichments per hour
	MaxConcurrentRequests int // Maximum parallel LLM requests

	// Storage (B2/R2)
	B2BucketName      string
	B2AccessKeyID     string
	B2SecretAccessKey string
	B2Endpoint        string // S3-compatible endpoint URL

	R2BucketName      string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Endpoint        string

	// Enrichment criteria
	MinTurnCount        int     // Minimum turn count to consider enrichment
	MinWinProbCrossings int     // Minimum win probability crossings
	UpsetThreshold      float64 // Minimum rating difference for upset consideration

	// Timing
	CycleInterval time.Duration // How often to run enrichment cycles
	CycleTimeout  time.Duration // Max duration per cycle
	MaxLifetime   time.Duration // Process lifetime before restart
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() Config {
	postgresPort := envInt("ACB_POSTGRES_PORT", 5432)
	cfg := Config{
		PostgresHost:     envOr("ACB_POSTGRES_HOST", "localhost"),
		PostgresPort:     postgresPort,
		PostgresUser:     envOr("ACB_POSTGRES_USER", "acb"),
		PostgresPassword: os.Getenv("ACB_POSTGRES_PASSWORD"),
		PostgresDatabase: envOr("ACB_POSTGRES_DATABASE", "ai_code_battle"),
		DatabaseName:     envOr("ACB_DATABASE_NAME", "ai_code_battle"),

		LLMBaseURL:     envOr("ACB_LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:      os.Getenv("ACB_LLM_API_KEY"),
		LLMModel:       envOr("ACB_LLM_MODEL", "gpt-4o-mini"),
		LLMMaxTokens:   envInt("ACB_LLM_MAX_TOKENS", 2000),
		LLMTemperature: envFloat("ACB_LLM_TEMPERATURE", 0.7),

		MaxEnrichmentsPerHour: envInt("ACB_ENRICHMENT_MAX_PER_HOUR", 20),
		MaxConcurrentRequests: envInt("ACB_ENRICHMENT_MAX_CONCURRENT", 3),

		B2BucketName:      envOr("ACB_B2_BUCKET", "ai-code-battle"),
		B2AccessKeyID:     os.Getenv("ACB_B2_ACCESS_KEY_ID"),
		B2SecretAccessKey: os.Getenv("ACB_B2_SECRET_ACCESS_KEY"),
		B2Endpoint:        envOr("ACB_B2_ENDPOINT", "https://s3.us-west-004.backblazeb2.com"),

		R2BucketName:      envOr("ACB_R2_BUCKET", "ai-code-battle"),
		R2AccessKeyID:     os.Getenv("ACB_R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: os.Getenv("ACB_R2_SECRET_ACCESS_KEY"),
		R2Endpoint:        envOr("ACB_R2_ENDPOINT", "https://r2.cloudflarestorage.com"),

		MinTurnCount:        envInt("ACB_ENRICHMENT_MIN_TURNS", 100),
		MinWinProbCrossings: envInt("ACB_ENRICHMENT_MIN_CROSSINGS", 3),
		UpsetThreshold:      envFloat("ACB_ENRICHMENT_UPSET_THRESHOLD", 150),

		CycleInterval: envDuration("ACB_ENRICHMENT_INTERVAL", 30*time.Minute),
		CycleTimeout:  envDuration("ACB_ENRICHMENT_TIMEOUT", 25*time.Minute),
		MaxLifetime:   envDuration("ACB_ENRICHMENT_MAX_LIFETIME", 4*time.Hour),
	}
	return cfg
}

// DatabaseURL returns the PostgreSQL connection string.
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.PostgresHost, c.PostgresPort, c.PostgresUser, c.PostgresPassword, c.PostgresDatabase)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil && i > 0 {
			return i
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
