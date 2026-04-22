// Package main implements the AI Code Battle API server.
// It provides bot registration, job coordination, matchmaking,
// health checks, and rating updates. Connects to PostgreSQL
// for persistent storage and Valkey (Redis-compatible) for
// the job queue.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aicodebattle/acb/metrics"
	"github.com/aicodebattle/acb/ratelimit"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	ListenAddr       string
	DatabaseURL      string
	ValkeyAddr       string
	ValkeyPassword   string
	WorkerAPIKey     string // API key workers use to submit results
	EncryptionKey    string // AES-256-GCM key for shared secret encryption
	DiscordWebhook   string // Discord webhook URL for alerts
	SlackWebhook     string // Slack webhook URL for alerts
	MatchmakerSecs   int
	HealthCheckSecs  int
	ReaperSecs       int
	BotTimeoutSecs   int
	StaleJobMinutes  int
	MaxConsecFails   int
}

func loadConfig() Config {
	return Config{
		ListenAddr:      envOr("ACB_LISTEN_ADDR", ":8080"),
		DatabaseURL:     envOr("ACB_DATABASE_URL", "postgres://localhost:5432/acb?sslmode=disable"),
		ValkeyAddr:      envOr("ACB_VALKEY_ADDR", "localhost:6379"),
		ValkeyPassword:  os.Getenv("ACB_VALKEY_PASSWORD"),
		WorkerAPIKey:    os.Getenv("ACB_WORKER_API_KEY"),
		EncryptionKey:   os.Getenv("ACB_ENCRYPTION_KEY"),
		DiscordWebhook:  os.Getenv("ACB_DISCORD_WEBHOOK"),
		SlackWebhook:    os.Getenv("ACB_SLACK_WEBHOOK"),
		MatchmakerSecs:  envInt("ACB_MATCHMAKER_INTERVAL", 60),
		HealthCheckSecs: envInt("ACB_HEALTHCHECK_INTERVAL", 900),
		ReaperSecs:      envInt("ACB_REAPER_INTERVAL", 300),
		BotTimeoutSecs:  envInt("ACB_BOT_TIMEOUT", 5),
		StaleJobMinutes: envInt("ACB_STALE_JOB_MINUTES", 15),
		MaxConsecFails:  envInt("ACB_MAX_CONSEC_FAILS", 3),
	}
}

func main() {
	cfg := loadConfig()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.ValkeyAddr,
		Password: cfg.ValkeyPassword,
	})
	defer rdb.Close()

	srv := &Server{
		cfg:         cfg,
		db:          db,
		rdb:         rdb,
		regLimiter:  ratelimit.NewLimiter(5, 5.0/3600),     // 5/hour per IP
		feedbackLtr: ratelimit.NewLimiter(20, 20.0/3600),   // 20/hour per IP
		predictLtr:  ratelimit.NewLimiter(60, 60.0/3600),   // 60/hour per IP
		submitLtr:   ratelimit.NewLimiter(5, 5.0/86400),    // 5/day per key
		voteLtr:     ratelimit.NewLimiter(10, 10.0/3600),   // 10/hour per IP
	}

	// Periodically purge stale rate-limit buckets (every 10 min)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			srv.regLimiter.Cleanup(time.Hour)
			srv.feedbackLtr.Cleanup(time.Hour)
			srv.predictLtr.Cleanup(time.Hour)
			srv.submitLtr.Cleanup(24 * time.Hour)
			srv.voteLtr.Cleanup(time.Hour)
		}
	}()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Start internal metrics server (separate port for Prometheus scraping)
	metricsSrv := metrics.StartServer()
	defer metricsSrv.Close()

	httpSrv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      metrics.HTTPMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Note: Background tickers (matchmaker, health-checker, stale-reaper) are now
	// handled by the separate acb-matchmaker deployment per plan §12 Phase 4.
	// This API server only handles HTTP endpoints for bot registration, job
	// coordination, and bot status.

	_ = ctx // ctx no longer needed since tickers moved to acb-matchmaker

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("acb-api listening on %s", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-sigCh
	log.Println("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
	log.Println("shutdown complete")
}
