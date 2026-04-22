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
	"time"

	"github.com/aicodebattle/acb/metrics"
)

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
		playlistQuery := `SELECT DISTINCT match_id FROM playlist_matches`
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

// deployToPages deploys the generated files to Cloudflare Pages via wrangler
func deployToPages(cfg *Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Check if wrangler is available
	if _, err := exec.LookPath("wrangler"); err != nil {
		return fmt.Errorf("wrangler not found in PATH: %w", err)
	}

	// Set up environment for wrangler
	env := os.Environ()
	if cfg.CloudflareAPIToken != "" {
		env = append(env, fmt.Sprintf("CLOUDFLARE_API_TOKEN=%s", cfg.CloudflareAPIToken))
	}
	if cfg.CloudflareAccountID != "" {
		env = append(env, fmt.Sprintf("CLOUDFLARE_ACCOUNT_ID=%s", cfg.CloudflareAccountID))
	}

	// Run wrangler pages deploy
	args := []string{
		"pages", "deploy",
		cfg.OutputDir,
		"--project-name", cfg.PagesProjectName,
		"--branch", "main",
	}

	cmd := exec.CommandContext(ctx, "wrangler", args...)
	cmd.Env = env
	cmd.Dir = "/tmp" // wrangler creates .wrangler/tmp relative to CWD; /app is root-owned
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	slog.Info("Running wrangler pages deploy",
		"project", cfg.PagesProjectName,
		"directory", cfg.OutputDir,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wrangler pages deploy failed: %w", err)
	}

	slog.Info("Successfully deployed to Cloudflare Pages")
	return nil
}

// pruneR2Cache removes old replays from R2 warm cache to stay within the 10GB free tier
// It also promotes recent replays from B2 to R2
func pruneR2Cache(ctx context.Context, cfg *Config) error {
	return pruneR2CacheWithDB(ctx, cfg, nil)
}

// pruneR2CacheWithDB removes old replays from R2, respecting exempt matches
func pruneR2CacheWithDB(ctx context.Context, cfg *Config, db *sql.DB) error {
	// R2 max size in bytes (10 GB with 500MB buffer for safety)
	maxSize := int64(10*1024*1024*1024 - 500*1024*1024)

	// List all objects in R2 replays directory
	objects, err := listR2Objects(ctx, cfg, "replays/")
	if err != nil {
		return fmt.Errorf("list R2 objects: %w", err)
	}

	// Calculate total size
	var totalSize int64
	for _, obj := range objects {
		totalSize += obj.Size
	}

	slog.Info("R2 warm cache status",
		"objects", len(objects),
		"total_size_gb", float64(totalSize)/(1024*1024*1024),
		"max_size_gb", float64(maxSize)/(1024*1024*1024),
	)

	// Export R2 cache size metric per §9.9
	metrics.R2BytesUsed.Set(float64(totalSize))

	// If under limit, nothing to prune
	if totalSize <= maxSize {
		slog.Info("R2 cache within limits, no pruning needed")
		return nil
	}

	// Get exempt match IDs if db is provided
	exemptMatchIDs := make(map[string]bool)
	if db != nil {
		exemptMatchIDs, err = fetchExemptMatchIDs(ctx, db, cfg.OutputDir)
		if err != nil {
			slog.Warn("Failed to fetch exempt match IDs, will proceed without exemptions", "error", err)
		}
	}

	// Sort objects by age (oldest first) and delete until under limit
	// Objects are already sorted by LastModified from listR2Objects
	toDelete := int64(0)
	prunedCount := 0
	for _, obj := range objects {
		if totalSize-toDelete <= maxSize {
			break
		}

		// Extract match ID from key (replays/{match_id}.json.gz)
		matchID := extractMatchIDFromKey(obj.Key)
		if exemptMatchIDs[matchID] {
			slog.Debug("Skipping exempt match from pruning", "key", obj.Key, "match_id", matchID)
			continue
		}

		if err := deleteR2Object(ctx, cfg, obj.Key); err != nil {
			slog.Error("Failed to delete R2 object", "key", obj.Key, "error", err)
			continue
		}

		toDelete += obj.Size
		prunedCount++
		slog.Info("Pruned R2 object", "key", obj.Key, "size_mb", obj.Size/(1024*1024))
	}

	slog.Info("R2 pruning complete",
		"pruned_count", prunedCount,
		"pruned_size_gb", float64(toDelete)/(1024*1024*1024),
	)

	return nil
}

// extractMatchIDFromKey extracts the match ID from a replay key
func extractMatchIDFromKey(key string) string {
	// Key format: replays/{match_id}.json.gz
	parts := strings.Split(key, "/")
	if len(parts) < 2 {
		return ""
	}
	filename := parts[len(parts)-1]
	// Remove .json.gz extension
	if strings.HasSuffix(filename, ".json.gz") {
		return filename[:len(filename)-8]
	}
	return filename
}

// promoteRecentReplays copies recent replays and thumbnails from B2 to R2 warm cache
func promoteRecentReplays(ctx context.Context, cfg *Config, matchIDs []string) error {
	for _, matchID := range matchIDs {
		// Promote replay
		b2ReplayKey := fmt.Sprintf("replays/%s.json.gz", matchID)
		r2ReplayKey := b2ReplayKey

		exists, err := checkR2ObjectExists(ctx, cfg, r2ReplayKey)
		if err != nil {
			slog.Error("Failed to check R2 object existence", "key", r2ReplayKey, "error", err)
		} else if !exists {
			if err := copyB2ToR2(ctx, cfg, b2ReplayKey, r2ReplayKey); err != nil {
				slog.Error("Failed to promote replay to R2", "match_id", matchID, "error", err)
			} else {
				slog.Info("Promoted replay to R2 warm cache", "match_id", matchID)
			}
		}

		// Promote thumbnail
		b2ThumbKey := fmt.Sprintf("thumbnails/%s.png", matchID)
		r2ThumbKey := b2ThumbKey

		exists, err = checkR2ObjectExists(ctx, cfg, r2ThumbKey)
		if err != nil {
			slog.Error("Failed to check R2 thumbnail existence", "key", r2ThumbKey, "error", err)
		} else if !exists {
			if err := copyB2ToR2(ctx, cfg, b2ThumbKey, r2ThumbKey); err != nil {
				slog.Warn("Failed to promote thumbnail to R2", "match_id", matchID, "error", err)
			} else {
				slog.Info("Promoted thumbnail to R2 warm cache", "match_id", matchID)
			}
		}
	}

	return nil
}

// R2Object represents an object in R2 storage
type R2Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// getR2Client returns an S3 client for R2
func getR2Client(cfg *Config) (*S3Client, error) {
	if cfg.R2AccessKey == "" || cfg.R2SecretKey == "" || cfg.R2BucketName == "" {
		return nil, fmt.Errorf("R2 credentials not configured")
	}
	return NewS3Client(cfg.R2Endpoint, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2BucketName)
}

// getB2Client returns an S3 client for B2
func getB2Client(cfg *Config) (*S3Client, error) {
	if cfg.B2AccessKey == "" || cfg.B2SecretKey == "" || cfg.B2BucketName == "" {
		return nil, fmt.Errorf("B2 credentials not configured")
	}
	return NewS3Client(cfg.B2Endpoint, cfg.B2AccessKey, cfg.B2SecretKey, cfg.B2BucketName)
}

// listR2Objects lists all objects in R2 under a prefix, sorted by LastModified (oldest first)
func listR2Objects(ctx context.Context, cfg *Config, prefix string) ([]R2Object, error) {
	client, err := getR2Client(cfg)
	if err != nil {
		return nil, fmt.Errorf("create R2 client: %w", err)
	}

	return client.listObjects(ctx, prefix)
}

// deleteR2Object deletes an object from R2
func deleteR2Object(ctx context.Context, cfg *Config, key string) error {
	client, err := getR2Client(cfg)
	if err != nil {
		return fmt.Errorf("create R2 client: %w", err)
	}

	return client.deleteObject(ctx, key)
}

// checkR2ObjectExists checks if an object exists in R2
func checkR2ObjectExists(ctx context.Context, cfg *Config, key string) (bool, error) {
	client, err := getR2Client(cfg)
	if err != nil {
		return false, fmt.Errorf("create R2 client: %w", err)
	}

	return client.objectExists(ctx, key)
}

// copyB2ToR2 copies an object from B2 to R2 by downloading from B2 and uploading to R2
func copyB2ToR2(ctx context.Context, cfg *Config, b2Key, r2Key string) error {
	b2Client, err := getB2Client(cfg)
	if err != nil {
		return fmt.Errorf("create B2 client: %w", err)
	}

	r2Client, err := getR2Client(cfg)
	if err != nil {
		return fmt.Errorf("create R2 client: %w", err)
	}

	// Download from B2
	body, err := b2Client.downloadObject(ctx, b2Key)
	if err != nil {
		return fmt.Errorf("download from B2: %w", err)
	}
	defer body.Close()

	// Upload to R2
	contentType := getS3ContentType(r2Key)
	if err := r2Client.uploadFile(ctx, r2Key, body, contentType); err != nil {
		return fmt.Errorf("upload to R2: %w", err)
	}

	slog.Info("Copied object from B2 to R2", "b2_key", b2Key, "r2_key", r2Key)
	return nil
}

// copyWebAssets copies the built web SPA to the output directory
func copyWebAssets(cfg *Config, webDistDir string) error {
	// Copy all files from web/dist to output directory using streaming
	// to avoid loading large files (e.g. demo replays) fully into memory.
	err := filepath.Walk(webDistDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(webDistDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(cfg.OutputDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, src)
		return err
	})

	if err != nil {
		return fmt.Errorf("copy web assets: %w", err)
	}

	slog.Info("Copied web assets to output directory", "source", webDistDir)
	return nil
}

// writeBuildManifest writes a manifest.json with build metadata
func writeBuildManifest(cfg *Config, buildTime time.Time) error {
	manifest := map[string]interface{}{
		"built_at":    buildTime.UTC().Format(time.RFC3339),
		"version":     "1.0.0",
		"environment": getEnvOrDefault("ACB_ENV", "production"),
	}

	manifestPath := filepath.Join(cfg.OutputDir, "data", "manifest.json")
	return writeJSON(manifestPath, manifest)
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// fetchPlaylistMatchIDsFromFiles reads generated playlist JSON files from the
// output directory and returns all match IDs referenced in them. This replaces
// the old approach of querying non-existent playlist_matches DB tables.
func fetchPlaylistMatchIDsFromFiles(outputDir string) map[string]bool {
	ids := make(map[string]bool)

	playlistsDir := filepath.Join(outputDir, "data", "playlists")
	entries, err := os.ReadDir(playlistsDir)
	if err != nil {
		return ids
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		// Skip index.json — only individual playlist files have match lists
		if entry.Name() == "index.json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(playlistsDir, entry.Name()))
		if err != nil {
			continue
		}

		var pl struct {
			Matches []struct {
				MatchID string `json:"match_id"`
			} `json:"matches"`
		}
		if err := json.Unmarshal(data, &pl); err != nil {
			continue
		}
		for _, m := range pl.Matches {
			ids[m.MatchID] = true
		}
	}

	return ids
}

// ensure valid function references
var _ = strings.Join
