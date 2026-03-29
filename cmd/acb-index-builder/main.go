package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := LoadConfig()

	// Connect to PostgreSQL
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresDatabase,
	)

	db, err := sql.Open("postgres", connStr)
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

	slog.Info("Connected to PostgreSQL",
		"host", cfg.PostgresHost,
		"database", cfg.PostgresDatabase,
	)

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		slog.Error("Failed to create output directory", "error", err, "path", cfg.OutputDir)
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	startTime := time.Now()
	buildCount := 0

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

		// Run build cycle with timeout
		buildCount++
		slog.Info("Starting build cycle", "count", buildCount)

		buildCtx, buildCancel := context.WithTimeout(context.Background(), cfg.BuildTimeout)
		if err := runBuildCycle(buildCtx, db, cfg); err != nil {
			slog.Error("Build cycle failed", "error", err)
		} else {
			slog.Info("Build cycle completed", "count", buildCount)
		}
		buildCancel()

		// Deploy every N cycles
		if buildCount%cfg.DeployInterval == 0 {
			slog.Info("Deploy interval reached, deploying to Pages")
			if err := deployToPages(cfg); err != nil {
				slog.Error("Failed to deploy to Pages", "error", err)
			} else {
				slog.Info("Deployed to Cloudflare Pages")
			}

			// Run R2 pruning once per week (Monday)
			if time.Now().Weekday() == time.Monday {
				slog.Info("Running weekly R2 pruning")
				if err := pruneR2Cache(context.Background(), cfg); err != nil {
					slog.Error("R2 pruning failed", "error", err)
				} else {
					slog.Info("R2 pruning completed")
				}
			}
		}

		// Sleep until next cycle
		slog.Info("Sleeping until next build cycle", "duration", cfg.BuildInterval)
		time.Sleep(cfg.BuildInterval)
	}
}

// runBuildCycle executes one full index build cycle
func runBuildCycle(ctx context.Context, db *sql.DB, cfg *Config) error {
	// Create data directories
	dirs := []string{
		cfg.OutputDir + "/data",
		cfg.OutputDir + "/data/bots",
		cfg.OutputDir + "/data/matches",
		cfg.OutputDir + "/data/series",
		cfg.OutputDir + "/data/seasons",
		cfg.OutputDir + "/data/playlists",
		cfg.OutputDir + "/data/predictions",
		cfg.OutputDir + "/data/meta",
		cfg.OutputDir + "/data/evolution",
		cfg.OutputDir + "/data/blog",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	// Fetch all data from PostgreSQL
	data, err := fetchAllData(ctx, db)
	if err != nil {
		return fmt.Errorf("fetch data: %w", err)
	}

	// Generate all index files
	if err := generateAllIndexes(data, cfg.OutputDir); err != nil {
		return fmt.Errorf("generate indexes: %w", err)
	}

	return nil
}
