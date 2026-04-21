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
		cfg.OutputDir + "/data/commentary",
		cfg.OutputDir + "/cards",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	// Copy web frontend assets into output directory
	const webDistDir = "/app/web/dist"
	if _, err := os.Stat(webDistDir); err == nil {
		if err := copyWebAssets(cfg, webDistDir); err != nil {
			slog.Error("Failed to copy web assets", "error", err)
			// Non-fatal - continue
		}
	}

	// Fetch all data from PostgreSQL
	data, err := fetchAllData(ctx, db)
	if err != nil {
		return fmt.Errorf("fetch data: %w", err)
	}

	// Generate all index files
	if err := generateAllIndexes(data, cfg.OutputDir, db); err != nil {
		return fmt.Errorf("generate indexes: %w", err)
	}

	// Generate blog posts (weekly meta reports and chronicles)
	var llmClient *LLMClient
	if cfg.LLMBaseURL != "" {
		llmClient = NewLLMClient(cfg.LLMBaseURL, cfg.LLMAPIKey)
	}
	if err := generateBlog(data, cfg.OutputDir, llmClient, cfg); err != nil {
		slog.Error("Failed to generate blog", "error", err)
		// Non-fatal - continue with rest of build
	}

	// Generate bot profile cards (PNG images for social sharing)
	if err := generateAllBotCards(data, cfg.OutputDir); err != nil {
		slog.Error("Failed to generate bot cards", "error", err)
		// Non-fatal - continue with rest of build
	}

	// Upload cards to R2 warm cache
	if err := uploadCardsToR2(ctx, cfg, cfg.OutputDir); err != nil {
		slog.Error("Failed to upload cards to R2", "error", err)
		// Non-fatal
	}

	// Upload cards to B2 cold archive
	if err := uploadCardsToB2(ctx, cfg, cfg.OutputDir); err != nil {
		slog.Error("Failed to upload cards to B2", "error", err)
		// Non-fatal
	}

	// Promote recent replays from B2 to R2 warm cache
	if err := promoteRecentReplaysForCycle(ctx, db, cfg); err != nil {
		slog.Error("Failed to promote recent replays", "error", err)
		// Non-fatal
	}

	// Enrich featured replays with AI commentary (§13.3)
	if err := enrichReplays(ctx, data, cfg, llmClient); err != nil {
		slog.Error("Failed to enrich replays", "error", err)
		// Non-fatal - unenriched replays are still valid
	}

	// Generate enriched commentary index (list of matches with AI commentary)
	if err := generateEnrichedIndex(ctx, data, cfg, cfg.OutputDir); err != nil {
		slog.Error("Failed to generate enriched index", "error", err)
		// Non-fatal
	}

	return nil
}

// promoteRecentReplaysForCycle promotes recent replays from B2 to R2
func promoteRecentReplaysForCycle(ctx context.Context, db *sql.DB, cfg *Config) error {
	// Get recent match IDs from the last 24 hours
	recentMatchIDs, err := fetchRecentMatchIDs(ctx, db, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("fetch recent match IDs: %w", err)
	}

	if len(recentMatchIDs) == 0 {
		slog.Debug("No recent matches to promote")
		return nil
	}

	// Get exempt match IDs (playlists, series, seasons)
	exemptMatchIDs, err := fetchExemptMatchIDs(ctx, db, cfg.OutputDir)
	if err != nil {
		slog.Warn("Failed to fetch exempt match IDs, promoting all", "error", err)
		exemptMatchIDs = make(map[string]bool)
	}

	// Combine recent and exempt matches for promotion
	matchIDsToPromote := recentMatchIDs
	for matchID := range exemptMatchIDs {
		matchIDsToPromote = append(matchIDsToPromote, matchID)
	}

	if len(matchIDsToPromote) == 0 {
		slog.Debug("No matches to promote")
		return nil
	}

	slog.Info("Promoting replays to R2",
		"recent_count", len(recentMatchIDs),
		"exempt_count", len(exemptMatchIDs),
		"total", len(matchIDsToPromote))

	return promoteRecentReplays(ctx, cfg, matchIDsToPromote)
}

// fetchRecentMatchIDs retrieves match IDs from the last duration
func fetchRecentMatchIDs(ctx context.Context, db *sql.DB, since time.Duration) ([]string, error) {
	query := `
		SELECT match_id
		FROM matches
		WHERE status = 'completed'
		  AND completed_at > NOW() - $1::interval
		ORDER BY completed_at DESC
	`

	intervalStr := fmt.Sprintf("%.0f seconds", since.Seconds())
	rows, err := db.QueryContext(ctx, query, intervalStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matchIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		matchIDs = append(matchIDs, id)
	}

	return matchIDs, nil
}
