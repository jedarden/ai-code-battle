package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
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

	// Start internal metrics server (separate port for Prometheus scraping)
	metricsSrv := metrics.StartServer()
	defer metricsSrv.Close()

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		slog.Error("Failed to create output directory", "error", err, "path", cfg.OutputDir)
		os.Exit(1)
	}

	// Initialize crane auth for pulling site build images from the registry
	if err := initCraneAuth(cfg); err != nil {
		slog.Warn("Failed to initialize crane auth, site builds will use baked-in assets", "error", err)
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

		buildStart := time.Now()
		buildCtx, buildCancel := context.WithTimeout(context.Background(), cfg.BuildTimeout)
		if err := runBuildCycle(buildCtx, db, cfg); err != nil {
			slog.Error("Build cycle failed", "error", err)
		} else {
			slog.Info("Build cycle completed", "count", buildCount)
		}
		buildCancel()
		metrics.IndexBuildDuration.Observe(time.Since(buildStart).Seconds())

		// Deploy every N cycles
		if buildCount%cfg.DeployInterval == 0 {
			slog.Info("Deploy interval reached, deploying to Pages")
			if err := deployToPages(cfg); err != nil {
				slog.Error("Failed to deploy to Pages", "error", err)
			} else {
				slog.Info("Deployed to Cloudflare Pages")
			}

		}

		// Sleep until next cycle
		slog.Info("Sleeping until next build cycle", "duration", cfg.BuildInterval)
		time.Sleep(cfg.BuildInterval)
	}
}

// uploadMetaJSONToR2 uploads the generated static meta JSON files to R2 so
// they are available at the R2 CDN URL in addition to the Pages deploy.
func uploadMetaJSONToR2(ctx context.Context, cfg *Config, outputDir string, data *IndexData) error {
	if cfg.R2AccessKey == "" || cfg.R2BucketName == "" {
		return nil
	}

	static := []string{
		"data/meta/index.json",
		"data/meta/archetypes.json",
		"data/meta/rivalries.json",
		"data/evolution/community_hints.json",
		"data/evolution/meta.json",
		"data/evolution/lineage.json",
	}

	for _, rel := range static {
		localPath := fmt.Sprintf("%s/%s", outputDir, rel)
		if err := uploadFileToR2(ctx, cfg, localPath, rel); err != nil {
			slog.Error("Failed to upload meta JSON to R2", "path", rel, "error", err)
		}
	}

	// Upload per-match feedback files for matches that have annotations.
	matchIDs := make(map[string]bool)
	for _, f := range data.Feedback {
		matchIDs[f.MatchID] = true
	}
	for matchID := range matchIDs {
		rel := fmt.Sprintf("data/matches/%s/feedback.json", matchID)
		localPath := fmt.Sprintf("%s/%s", outputDir, rel)
		if err := uploadFileToR2(ctx, cfg, localPath, rel); err != nil {
			slog.Error("Failed to upload match feedback to R2", "match_id", matchID, "error", err)
		}
	}

	slog.Info("Uploaded meta JSON to R2",
		"static_files", len(static),
		"match_feedback_files", len(matchIDs),
	)
	return nil
}

// runBuildCycle executes one full index build cycle
func runBuildCycle(ctx context.Context, db *sql.DB, cfg *Config) (resultErr error) {
	// Recover from panics and log via slog before re-panicking
	// This prevents silent crashes where panic output (stderr) is lost
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Build cycle panicked", "panic", fmt.Sprintf("%v", r), "stack", string(debug.Stack()))
			resultErr = fmt.Errorf("panic: %v", r)
		}
	}()

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

	// Sync site build from registry (if configured) and copy into output directory.
	// Falls back to baked-in assets when registry is unreachable.
	webDistDir, siteBuildChanged := syncSiteBuild(ctx, cfg)

	// When a new site build is detected, remove old SPA files to prevent
	// stale hashed JS/CSS accumulation toward Pages' 20K file limit.
	if siteBuildChanged {
		slog.Info("Site build changed, cleaning stale web assets before merge")
		if err := cleanStaleWebAssets(cfg); err != nil {
			slog.Error("Failed to clean stale web assets", "error", err)
		}
	}

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
	if err := generateAllIndexes(data, cfg.OutputDir, db, cfg); err != nil {
		return fmt.Errorf("generate indexes: %w", err)
	}

	// Upload new static meta JSON files to R2 warm cache
	if err := uploadMetaJSONToR2(ctx, cfg, cfg.OutputDir, data); err != nil {
		slog.Error("Failed to upload meta JSON to R2", "error", err)
		// Non-fatal
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

	// Upload cards to B2 cold archive (primary storage)
	if err := uploadCardsToB2(ctx, cfg, cfg.OutputDir); err != nil {
		slog.Error("Failed to upload cards to B2", "error", err)
		// Non-fatal
	}

	// Bundle warm-set assets (replays, thumbnails, cards, evolution) from B2 into Pages deploy
	if err := bundleWarmAssetsForCycle(ctx, db, cfg, data); err != nil {
		slog.Error("Failed to bundle warm assets", "error", err)
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

// bundleWarmAssetsForCycle bundles warm-set assets from B2 into the Pages deploy directory.
// This includes replays, thumbnails, bot cards, and evolution live.json.
func bundleWarmAssetsForCycle(ctx context.Context, db *sql.DB, cfg *Config, data *IndexData) error {
	// Get recent match IDs from the last 24 hours
	recentMatchIDs, err := fetchRecentMatchIDs(ctx, db, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("fetch recent match IDs: %w", err)
	}

	if len(recentMatchIDs) == 0 {
		slog.Debug("No recent matches to bundle")
		return nil
	}

	// Get exempt match IDs (playlists, series, seasons) - these are also part of warm set
	exemptMatchIDs, err := fetchExemptMatchIDs(ctx, db, cfg.OutputDir)
	if err != nil {
		slog.Warn("Failed to fetch exempt match IDs, bundling only recent", "error", err)
		exemptMatchIDs = make(map[string]bool)
	}

	// Combine recent and exempt matches for bundling
	matchIDsToBundle := recentMatchIDs
	for matchID := range exemptMatchIDs {
		matchIDsToBundle = append(matchIDsToBundle, matchID)
	}

	// Deduplicate match IDs
	uniqueMatchIDs := make(map[string]bool)
	for _, id := range matchIDsToBundle {
		uniqueMatchIDs[id] = true
	}
	var dedupedMatchIDs []string
	for id := range uniqueMatchIDs {
		dedupedMatchIDs = append(dedupedMatchIDs, id)
	}

	if len(dedupedMatchIDs) == 0 {
		slog.Debug("No matches to bundle")
		return nil
	}

	slog.Info("Bundling warm assets from B2",
		"recent_count", len(recentMatchIDs),
		"exempt_count", len(exemptMatchIDs),
		"total_unique", len(dedupedMatchIDs))

	// Create B2 client for bundling
	b2Client, err := getB2Client(cfg)
	if err != nil {
		return fmt.Errorf("create B2 client: %w", err)
	}

	// Bundle replays
	if err := bundleWarmReplays(ctx, cfg, b2Client, dedupedMatchIDs); err != nil {
		slog.Error("Failed to bundle warm replays", "error", err)
		// Continue with other assets
	}

	// Bundle thumbnails
	if err := bundleWarmThumbnails(ctx, cfg, b2Client, dedupedMatchIDs); err != nil {
		slog.Error("Failed to bundle warm thumbnails", "error", err)
		// Continue
	}

	// Bundle bot cards for all active bots
	var botIDs []string
	for _, bot := range data.Bots {
		botIDs = append(botIDs, bot.ID)
	}
	if len(botIDs) > 0 {
		if err := bundleWarmCards(ctx, cfg, b2Client, botIDs); err != nil {
			slog.Error("Failed to bundle warm cards", "error", err)
			// Continue
		}
	}

	// Bundle evolution live.json
	if err := bundleEvolutionLive(ctx, cfg, b2Client); err != nil {
		slog.Error("Failed to bundle evolution live.json", "error", err)
		// Continue
	}

	return nil
}

// fetchRecentMatchIDs retrieves match IDs from the last duration
func fetchRecentMatchIDs(ctx context.Context, db *sql.DB, since time.Duration) ([]string, error) {
	query := `
		SELECT match_id
		FROM matches
		WHERE status = 'completed'
		  AND completed_at > NOW() - $1::interval
		ORDER BY completed_at DESC
		LIMIT 5000
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
