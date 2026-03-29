package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

	// If under limit, nothing to prune
	if totalSize <= maxSize {
		slog.Info("R2 cache within limits, no pruning needed")
		return nil
	}

	// Sort objects by age (oldest first) and delete until under limit
	// Objects are already sorted by LastModified from listR2Objects
	toDelete := int64(0)
	for _, obj := range objects {
		if totalSize-toDelete <= maxSize {
			break
		}

		if err := deleteR2Object(ctx, cfg, obj.Key); err != nil {
			slog.Error("Failed to delete R2 object", "key", obj.Key, "error", err)
			continue
		}

		toDelete += obj.Size
		slog.Info("Pruned R2 object", "key", obj.Key, "size_mb", obj.Size/(1024*1024))
	}

	slog.Info("R2 pruning complete",
		"pruned_count", len(objects),
		"pruned_size_gb", float64(toDelete)/(1024*1024*1024),
	)

	return nil
}

// promoteRecentReplays copies recent replays from B2 to R2 warm cache
func promoteRecentReplays(ctx context.Context, cfg *Config, matchIDs []string) error {
	for _, matchID := range matchIDs {
		// Source path in B2
		b2Key := fmt.Sprintf("replays/%s.json.gz", matchID)

		// Check if already in R2
		r2Key := b2Key
		exists, err := checkR2ObjectExists(ctx, cfg, r2Key)
		if err != nil {
			slog.Error("Failed to check R2 object existence", "key", r2Key, "error", err)
			continue
		}
		if exists {
			continue // Already in warm cache
		}

		// Copy from B2 to R2
		if err := copyB2ToR2(ctx, cfg, b2Key, r2Key); err != nil {
			slog.Error("Failed to promote replay to R2", "match_id", matchID, "error", err)
			continue
		}

		slog.Info("Promoted replay to R2 warm cache", "match_id", matchID)
	}

	return nil
}

// R2Object represents an object in R2 storage
type R2Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// listR2Objects lists all objects in R2 under a prefix, sorted by LastModified (oldest first)
func listR2Objects(ctx context.Context, cfg *Config, prefix string) ([]R2Object, error) {
	// This is a simplified implementation
	// In production, use the AWS SDK for Go v2 with S3-compatible API
	//
	// Example using minio client or aws-sdk-go-v2:
	// cfg := aws.NewConfig().
	//     WithEndpoint(cfg.R2Endpoint).
	//     WithCredentials(credentials.NewStaticCredentials(cfg.R2AccessKey, cfg.R2SecretKey, ""))
	//
	// For now, return empty list - actual implementation requires AWS SDK

	slog.Warn("listR2Objects not fully implemented - requires AWS SDK integration")
	return []R2Object{}, nil
}

// deleteR2Object deletes an object from R2
func deleteR2Object(ctx context.Context, cfg *Config, key string) error {
	// Requires AWS SDK integration
	slog.Warn("deleteR2Object not fully implemented - requires AWS SDK integration")
	return nil
}

// checkR2ObjectExists checks if an object exists in R2
func checkR2ObjectExists(ctx context.Context, cfg *Config, key string) (bool, error) {
	// Requires AWS SDK integration
	return false, nil
}

// copyB2ToR2 copies an object from B2 to R2
func copyB2ToR2(ctx context.Context, cfg *Config, b2Key, r2Key string) error {
	// Requires AWS SDK integration for both B2 and R2
	slog.Warn("copyB2ToR2 not fully implemented - requires AWS SDK integration")
	return nil
}

// copyWebAssets copies the built web SPA to the output directory
func copyWebAssets(cfg *Config, webDistDir string) error {
	// Copy all files from web/dist to output directory
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

		// Read source file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Write to destination
		return os.WriteFile(destPath, data, 0644)
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

// ensure valid function references
var _ = strings.Join
