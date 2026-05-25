// Package main implements acb-enrichment, a service that generates AI commentary
// for featured AI Code Battle matches. It polls for matches without commentary,
// downloads replays from B2/R2, generates turn-by-turn narrative highlights via
// LLM, and stores results back to R2.
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aicodebattle/acb/metrics"
	_ "github.com/lib/pq"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := LoadConfig()

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DatabaseURL())
	if err != nil {
		slog.Error("Failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.PingContext(ctx); err != nil {
		cancel()
		slog.Error("Failed to ping PostgreSQL", "error", err)
		os.Exit(1)
	}
	cancel()

	slog.Info("Connected to PostgreSQL", "database", cfg.DatabaseName)

	// Create enrichment service
	svc := NewEnrichmentService(db, cfg)

	// Verify storage configuration
	if err := svc.CheckStorage(ctx); err != nil {
		slog.Error("Storage check failed", "error", err)
		os.Exit(1)
	}

	// Verify LLM configuration
	if err := svc.CheckLLM(ctx); err != nil {
		slog.Error("LLM check failed", "error", err)
		os.Exit(1)
	}

	// Start internal metrics server
	metricsSrv := metrics.StartServer()
	defer metricsSrv.Close()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	startTime := time.Now()
	cycleCount := 0

	// Main enrichment loop
	for {
		// Check lifetime
		if time.Since(startTime) > cfg.MaxLifetime {
			slog.Info("Max lifetime reached, exiting", "lifetime", cfg.MaxLifetime)
			os.Exit(0)
		}

		// Check for shutdown signal
		select {
		case sig := <-sigChan:
			slog.Info("Received signal, shutting down", "signal", sig)
			os.Exit(0)
		default:
		}

		// Run enrichment cycle
		cycleCount++
		slog.Info("Starting enrichment cycle", "count", cycleCount)

		cycleStart := time.Now()
		cycleCtx, cycleCancel := context.WithTimeout(context.Background(), cfg.CycleTimeout)

		results, err := svc.RunCycle(cycleCtx)
		cycleCancel()

		if err != nil {
			slog.Error("Enrichment cycle failed", "error", err)
			metrics.EnrichmentCycles.WithLabelValues("error").Inc()
		} else {
			slog.Info("Enrichment cycle completed",
				"processed", results.Processed,
				"enriched", results.Enriched,
				"skipped", results.Skipped,
				"failed", results.Failed,
			)
			metrics.EnrichmentCycles.WithLabelValues("success").Inc()
			metrics.EnrichmentProcessed.Add(float64(results.Processed))
			metrics.EnrichmentGenerated.Add(float64(results.Enriched))
		}

		metrics.EnrichmentCycleDuration.Observe(time.Since(cycleStart).Seconds())

		// Sleep until next cycle
		slog.Info("Sleeping until next enrichment cycle", "duration", cfg.CycleInterval)
		time.Sleep(cfg.CycleInterval)
	}
}

// CycleResults holds statistics from a single enrichment cycle.
type CycleResults struct {
	Processed int
	Enriched  int
	Skipped   int
	Failed    int
}
