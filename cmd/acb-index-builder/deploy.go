package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// B2Client defines the interface for B2 operations needed by bundling functions.
// This allows both real S3Client and mock clients to be used.
type B2Client interface {
	downloadObject(ctx context.Context, key string) (io.ReadCloser, error)
}

// fetchExemptMatchIDs retrieves match IDs that should never be pruned (from
// series, seasons, and playlists).
func fetchExemptMatchIDs(ctx context.Context, db *sql.DB, outputDir string) (map[string]bool, error) {
	exempt := make(map[string]bool)

	if db != nil {
		// Matches in active/pending series (series_games, not series_matches)
		seriesQuery := `
				SELECT DISTINCT sg.match_id
				FROM series_games sg
				JOIN series s ON sg.series_id = s.id
				WHERE s.status IN ('active', 'pending')
				  AND sg.match_id IS NOT NULL
				LIMIT 10000
			`
		rows, err := db.QueryContext(ctx, seriesQuery)
		if err == nil {
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					exempt[id] = true
				}
			}
			rows.Close()
		}

		// Matches in active seasons (via series → series_games)
		seasonQuery := `
				SELECT DISTINCT sg.match_id
				FROM series_games sg
				JOIN series s ON sg.series_id = s.id
				WHERE s.season_id IN (
					SELECT id FROM seasons WHERE ends_at IS NULL OR ends_at > NOW()
				)
				AND sg.match_id IS NOT NULL
				LIMIT 10000
			`
		rows, err = db.QueryContext(ctx, seasonQuery)
		if err == nil {
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					exempt[id] = true
				}
			}
			rows.Close()
		}

		// Matches in persisted playlists (playlist_matches table)
		playlistQuery := `SELECT DISTINCT match_id FROM playlist_matches LIMIT 10000`
		rows, err = db.QueryContext(ctx, playlistQuery)
		if err == nil {
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					exempt[id] = true
				}
			}
			rows.Close()
		}
	}

	// Also read from generated playlist files (covers cases where DB persist failed)
	playlistMatchIDs := fetchPlaylistMatchIDsFromFiles(outputDir)
	for id := range playlistMatchIDs {
		exempt[id] = true
	}

	slog.Debug("Fetched exempt match IDs for pruning", "count", len(exempt))
	return exempt, nil
}

// verifyMergedOutput checks that the output directory contains both the SPA
// shell (index.html) and generated data files before deploying to Pages.
func verifyMergedOutput(cfg *Config) error {
	// Check for SPA shell
	indexPath := filepath.Join(cfg.OutputDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return fmt.Errorf("index.html not found: %w", err)
	}

	// Check for data directory
	dataDir := filepath.Join(cfg.OutputDir, "data")
	if _, err := os.Stat(dataDir); err != nil {
		return fmt.Errorf("data directory not found: %w", err)
	}

	// Check for leaderboard.json (canonical data file). Missing on a first
	// build is a warning, not an error — it may not have been generated yet.
	leaderboardPath := filepath.Join(dataDir, "leaderboard.json")
	if _, err := os.Stat(leaderboardPath); err != nil {
		slog.Warn("leaderboard.json not found in output directory", "path", leaderboardPath)
	}

	return nil
}

// deployToPages uploads the generated index to Cloudflare Pages using wrangler.
func deployToPages(cfg *Config) error {
	if err := verifyMergedOutput(cfg); err != nil {
		return fmt.Errorf("output verification failed: %w", err)
	}

	// Build wrangler command
	args := []string{"pages", "deploy", cfg.OutputDir, "--project-name", cfg.PagesProjectName}
	if cfg.CloudflareAccountID != "" {
		args = append(args, "--compatibility-date=2024-01-01")
	}

	cmd := exec.Command("wrangler", args...)
	cmd.Env = append(os.Environ(),
		"CLOUDFLARE_API_TOKEN="+cfg.CloudflareAPIToken,
		"CLOUDFLARE_ACCOUNT_ID="+cfg.CloudflareAccountID,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wrangler deploy failed: %w\nOutput: %s", err, output)
	}

	slog.Info("Deployed to Cloudflare Pages", "output", string(output))
	return nil
}

// bundleWarmReplay downloads replays from B2 and places them in the deploy directory.
func bundleWarmReplay(ctx context.Context, cfg *Config, b2Client B2Client, matchID string) error {
	// Replays are stored gzipped in B2 (standard format, see downloadReplayFromB2).
	key := fmt.Sprintf("replays/%s.json.gz", matchID)
	rc, err := b2Client.downloadObject(ctx, key)
	if err != nil {
		return fmt.Errorf("download replay: %w", err)
	}
	defer rc.Close()

	destPath := filepath.Join(cfg.OutputDir, "data", "replays", matchID+".json.gz")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create replay dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create replay file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("write replay: %w", err)
	}

	return nil
}

// bundleWarmReplays bundles recent and exempt match replays into the Pages deploy.
func bundleWarmReplays(ctx context.Context, cfg *Config, b2Client B2Client, matchIDs []string) error {
	for _, matchID := range matchIDs {
		if err := bundleWarmReplay(ctx, cfg, b2Client, matchID); err != nil {
			slog.Warn("Failed to bundle replay", "match_id", matchID, "error", err)
			// Continue with other replays
		}
	}

	slog.Info("Bundled warm replays", "count", len(matchIDs))
	return nil
}

// bundleWarmThumbnails downloads thumbnails from B2 and places them in the deploy directory.
func bundleWarmThumbnails(ctx context.Context, cfg *Config, b2Client B2Client, matchIDs []string) error {
	for _, matchID := range matchIDs {
		key := fmt.Sprintf("thumbnails/%s.png", matchID)
		rc, err := b2Client.downloadObject(ctx, key)
		if err != nil {
			slog.Warn("Failed to download thumbnail", "match_id", matchID, "error", err)
			continue
		}
		defer rc.Close()

		destPath := filepath.Join(cfg.OutputDir, "data", "thumbnails", matchID+".png")
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			slog.Warn("Failed to create thumbnail dir", "error", err)
			continue
		}

		f, err := os.Create(destPath)
		if err != nil {
			slog.Warn("Failed to create thumbnail file", "error", err)
			rc.Close()
			continue
		}

		if _, err := io.Copy(f, rc); err != nil {
			slog.Warn("Failed to write thumbnail", "error", err)
			f.Close()
			continue
		}
		f.Close()
	}

	slog.Info("Bundled warm thumbnails", "count", len(matchIDs))
	return nil
}

// bundleWarmCards downloads bot cards from B2 and places them in the deploy directory.
func bundleWarmCards(ctx context.Context, cfg *Config, b2Client B2Client, botIDs []string) error {
	for _, botID := range botIDs {
		key := fmt.Sprintf("cards/%s.png", botID)
		rc, err := b2Client.downloadObject(ctx, key)
		if err != nil {
			slog.Warn("Failed to download card", "bot_id", botID, "error", err)
			continue
		}
		defer rc.Close()

		destPath := filepath.Join(cfg.OutputDir, "data", "cards", botID+".png")
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			slog.Warn("Failed to create card dir", "error", err)
			continue
		}

		f, err := os.Create(destPath)
		if err != nil {
			slog.Warn("Failed to create card file", "error", err)
			rc.Close()
			continue
		}

		if _, err := io.Copy(f, rc); err != nil {
			slog.Warn("Failed to write card", "error", err)
			f.Close()
			continue
		}
		f.Close()
	}

	slog.Info("Bundled warm cards", "count", len(botIDs))
	return nil
}

// bundleEvolutionLive downloads evolution live.json from B2 and places it in the deploy directory.
func bundleEvolutionLive(ctx context.Context, cfg *Config, b2Client B2Client) error {
	key := "evolution/live.json"
	rc, err := b2Client.downloadObject(ctx, key)
	if err != nil {
		slog.Warn("Failed to download evolution live.json", "error", err)
		return nil // Non-fatal
	}
	defer rc.Close()

	destPath := filepath.Join(cfg.OutputDir, "data", "evolution", "live.json")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create evolution dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create live.json file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("write live.json: %w", err)
	}

	slog.Info("Bundled evolution live.json")
	return nil
}

// extractMatchIDFromKey extracts a match ID from a B2/R2 replay object key of
// the form "replays/{matchID}.json.gz". It returns "" for keys that do not use
// the expected replay prefix.
func extractMatchIDFromKey(key string) string {
	const prefix = "replays/"
	if !strings.HasPrefix(key, prefix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(key, prefix), ".json.gz")
}

// fetchPlaylistMatchIDsFromFiles reads playlist JSON files and extracts match IDs.
// This is used as a fallback when the database is unavailable or playlist persistence failed.
func fetchPlaylistMatchIDsFromFiles(outputDir string) map[string]bool {
	matchIDs := make(map[string]bool)

	playlistsDir := filepath.Join(outputDir, "data", "playlists")
	entries, err := os.ReadDir(playlistsDir)
	if err != nil {
		return matchIDs
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Skip index.json
		if entry.Name() == "index.json" {
			continue
		}

		filePath := filepath.Join(playlistsDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var playlist Playlist
		if err := json.Unmarshal(content, &playlist); err != nil {
			continue
		}

		for _, match := range playlist.Matches {
			matchIDs[match.MatchID] = true
		}
	}

	return matchIDs
}
