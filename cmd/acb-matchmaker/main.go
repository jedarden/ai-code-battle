// Package main implements the AI Code Battle matchmaker.
// It is an internal service that runs tickers for bot pairing,
// health checking, and stale job reaping. It has no external
// HTTP exposure - it only connects to PostgreSQL and Valkey.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	DatabaseURL     string
	ValkeyAddr      string
	ValkeyPassword  string
	EncryptionKey   string // AES-256-GCM key for shared secret decryption
	DiscordWebhook  string
	SlackWebhook    string
	MatchmakerSecs    int
	HealthCheckSecs   int
	ReaperSecs        int
	SeriesSchedSecs   int
	SeasonResetSecs   int
	BotTimeoutSecs    int
	StaleJobMinutes   int
	MaxConsecFails    int
	SeasonDecayFactor float64
}

type Matchmaker struct {
	cfg     Config
	db      *sql.DB
	rdb     *redis.Client
	alerter *Alerter
}

func loadConfig() Config {
	return Config{
		DatabaseURL:     envOr("ACB_DATABASE_URL", "postgres://localhost:5432/acb?sslmode=disable"),
		ValkeyAddr:      envOr("ACB_VALKEY_ADDR", "localhost:6379"),
		ValkeyPassword:  os.Getenv("ACB_VALKEY_PASSWORD"),
		EncryptionKey:   os.Getenv("ACB_ENCRYPTION_KEY"),
		DiscordWebhook:  os.Getenv("ACB_DISCORD_WEBHOOK"),
		SlackWebhook:    os.Getenv("ACB_SLACK_WEBHOOK"),
		MatchmakerSecs:    envInt("ACB_MATCHMAKER_INTERVAL", 60),
		HealthCheckSecs:   envInt("ACB_HEALTHCHECK_INTERVAL", 900),
		ReaperSecs:        envInt("ACB_REAPER_INTERVAL", 300),
		SeriesSchedSecs:   envInt("ACB_SERIES_SCHED_INTERVAL", 120),
		SeasonResetSecs:   envInt("ACB_SEASON_RESET_INTERVAL", 300),
		BotTimeoutSecs:    envInt("ACB_BOT_TIMEOUT", 5),
		StaleJobMinutes:   envInt("ACB_STALE_JOB_MINUTES", 15),
		MaxConsecFails:    envInt("ACB_MAX_CONSEC_FAILS", 3),
		SeasonDecayFactor: envFloat("ACB_SEASON_DECAY_FACTOR", 0.7),
	}
}

func main() {
	cfg := loadConfig()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.ValkeyAddr,
		Password: cfg.ValkeyPassword,
	})
	defer rdb.Close()

	// Test connections
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("valkey ping failed: %v", err)
	}

	alerter := NewAlerter(cfg.DiscordWebhook, cfg.SlackWebhook)

	m := &Matchmaker{
		cfg:     cfg,
		db:      db,
		rdb:     rdb,
		alerter: alerter,
	}

	// Start background tickers
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	m.StartTickers(ctx)

	log.Println("acb-matchmaker started - running internal tickers")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	cancel()
	log.Println("shutdown complete")
}
