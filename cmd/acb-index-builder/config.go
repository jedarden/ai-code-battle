package main

import (
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the index builder
type Config struct {
	// PostgreSQL connection
	PostgresHost     string
	PostgresPort     int
	PostgresDatabase string
	PostgresUser     string
	PostgresPassword string

	// Build cycle timing
	BuildInterval  time.Duration // How often to rebuild indexes (default: 15m)
	DeployInterval int           // Deploy every N builds (default: 6 = 90min)
	MaxLifetime    time.Duration // Max process lifetime before exit (default: 4h)
	BuildTimeout   time.Duration // Timeout for each build cycle (default: 10m)

	// Cloudflare configuration
	CloudflareAPIToken string
	CloudflareAccountID string
	PagesProjectName   string

	// R2 configuration for warm cache management
	R2AccessKey      string
	R2SecretKey      string
	R2Endpoint       string
	R2BucketName     string

	// B2 configuration for cold archive
	B2AccessKey    string
	B2SecretKey    string
	B2Endpoint     string
	B2BucketName   string

	// Output directory for generated files
	OutputDir string
}

// LoadConfig reads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		PostgresHost:     getEnv("ACB_POSTGRES_HOST", "cnpg-apexalgo-rw.cnpg.svc.cluster.local"),
		PostgresPort:     getEnvInt("ACB_POSTGRES_PORT", 5432),
		PostgresDatabase: getEnv("ACB_POSTGRES_DATABASE", "acb"),
		PostgresUser:     getEnv("ACB_POSTGRES_USER", "acb"),
		PostgresPassword: getEnv("ACB_POSTGRES_PASSWORD", ""),

		BuildInterval:  getEnvDuration("ACB_BUILD_INTERVAL", 15*time.Minute),
		DeployInterval: getEnvInt("ACB_DEPLOY_INTERVAL", 6),
		MaxLifetime:    getEnvDuration("ACB_MAX_LIFETIME", 4*time.Hour),

		CloudflareAPIToken:  os.Getenv("ACB_CLOUDFLARE_API_TOKEN"),
		CloudflareAccountID: os.Getenv("ACB_CLOUDFLARE_ACCOUNT_ID"),
		PagesProjectName:    getEnv("ACB_PAGES_PROJECT", "ai-code-battle"),

		R2AccessKey:  os.Getenv("ACB_R2_ACCESS_KEY"),
		R2SecretKey:  os.Getenv("ACB_R2_SECRET_KEY"),
		R2Endpoint:   getEnv("ACB_R2_ENDPOINT", "https://<account-id>.r2.cloudflarestorage.com"),
		R2BucketName: os.Getenv("ACB_R2_BUCKET"),

		B2AccessKey:  os.Getenv("ACB_B2_ACCESS_KEY"),
		B2SecretKey:  os.Getenv("ACB_B2_SECRET_KEY"),
		B2Endpoint:   getEnv("ACB_B2_ENDPOINT", "https://s3.us-west-004.backblazeb2.com"),
		B2BucketName: os.Getenv("ACB_B2_BUCKET"),

		OutputDir: getEnv("ACB_OUTPUT_DIR", "/tmp/acb-index"),
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultValue
}
