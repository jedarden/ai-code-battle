package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	craneConfigDir      = "/tmp/crane-config"
	siteBuildDigestFile = ".site-build-digest"
	siteBuildExtractDir = "/tmp/acb-site-build"
	bakedInWebDist      = "/app/web/dist"
)

// initCraneAuth writes a Docker config.json for crane to authenticate with
// the container registry. No-op if registry auth is not configured.
func initCraneAuth(cfg *Config) error {
	if cfg.RegistryUsername == "" || cfg.SiteBuildImage == "" {
		return nil
	}
	if err := os.MkdirAll(craneConfigDir, 0700); err != nil {
		return fmt.Errorf("create crane config dir: %w", err)
	}

	registry := extractRegistry(cfg.SiteBuildImage)
	auth := base64.StdEncoding.EncodeToString([]byte(cfg.RegistryUsername + ":" + cfg.RegistryPassword))

	config := map[string]interface{}{
		"auths": map[string]interface{}{
			registry: map[string]string{"auth": auth},
		},
	}
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal docker config: %w", err)
	}
	return os.WriteFile(filepath.Join(craneConfigDir, "config.json"), data, 0600)
}

// craneEnviron returns the process environment with DOCKER_CONFIG set if auth
// was configured.
func craneEnviron() []string {
	env := os.Environ()
	if _, err := os.Stat(filepath.Join(craneConfigDir, "config.json")); err == nil {
		env = append(env, "DOCKER_CONFIG="+craneConfigDir)
	}
	return env
}

// syncSiteBuild checks for a newer site build image in the container registry
// and extracts it if available. Returns the path to the web assets directory.
// Falls back to baked-in assets when the registry is unreachable or crane is
// not installed.
func syncSiteBuild(ctx context.Context, cfg *Config) string {
	if cfg.SiteBuildImage == "" {
		return bakedInWebDist
	}
	if _, err := exec.LookPath("crane"); err != nil {
		slog.Warn("crane not found in PATH, using baked-in web assets")
		return bakedInWebDist
	}

	remoteDigest, err := craneDigest(ctx, cfg)
	if err != nil {
		slog.Warn("Failed to query remote site build digest, using cached or baked-in assets", "error", err)
		return fallbackWebDir(cfg)
	}

	cachedDigest := readCachedDigest(cfg.OutputDir)
	if cachedDigest == remoteDigest {
		slog.Debug("Site build image unchanged", "digest", remoteDigest)
		return extractedDistPath(cfg)
	}

	slog.Info("New site build image detected",
		"image", cfg.SiteBuildImage,
		"old_digest", cachedDigest,
		"new_digest", remoteDigest,
	)

	if err := craneExport(ctx, cfg); err != nil {
		slog.Error("Failed to extract site build image", "error", err)
		return fallbackWebDir(cfg)
	}

	writeCachedDigest(cfg.OutputDir, remoteDigest)
	return extractedDistPath(cfg)
}

// extractedDistPath returns the path to the dist directory within the
// extraction staging area.
func extractedDistPath(cfg *Config) string {
	return filepath.Join(siteBuildExtractDir, cfg.SiteBuildPath)
}

// craneDigest uses crane to get the digest of the configured site build image.
func craneDigest(ctx context.Context, cfg *Config) (string, error) {
	cmd := exec.CommandContext(ctx, "crane", "digest", cfg.SiteBuildImage)
	cmd.Env = craneEnviron()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("crane digest %s: %w", cfg.SiteBuildImage, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// craneExport uses crane to export the image filesystem and extracts it into
// the staging directory.
func craneExport(ctx context.Context, cfg *Config) error {
	os.RemoveAll(siteBuildExtractDir)
	if err := os.MkdirAll(siteBuildExtractDir, 0755); err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}

	craneCmd := exec.CommandContext(ctx, "crane", "export", cfg.SiteBuildImage, "-")
	craneCmd.Env = craneEnviron()

	tarCmd := exec.CommandContext(ctx, "tar", "-xf", "-", "-C", siteBuildExtractDir)

	pipe, err := craneCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("crane pipe: %w", err)
	}
	tarCmd.Stdin = pipe

	if err := craneCmd.Start(); err != nil {
		return fmt.Errorf("start crane: %w", err)
	}
	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("extract tar: %w", err)
	}
	if err := craneCmd.Wait(); err != nil {
		return fmt.Errorf("crane export: %w", err)
	}

	slog.Info("Extracted site build image", "path", siteBuildExtractDir)
	return nil
}

// fallbackWebDir returns the best available web asset directory when the
// registry is unreachable.
func fallbackWebDir(cfg *Config) string {
	p := extractedDistPath(cfg)
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		slog.Info("Using previously extracted site build")
		return p
	}
	if _, err := os.Stat(bakedInWebDist); err == nil {
		slog.Info("Using baked-in web assets")
		return bakedInWebDist
	}
	slog.Warn("No web assets available")
	return bakedInWebDist
}

func readCachedDigest(outputDir string) string {
	data, err := os.ReadFile(filepath.Join(outputDir, siteBuildDigestFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func writeCachedDigest(outputDir, digest string) {
	if err := os.WriteFile(filepath.Join(outputDir, siteBuildDigestFile), []byte(digest+"\n"), 0644); err != nil {
		slog.Warn("Failed to cache site build digest", "error", err)
	}
}

// extractRegistry parses the registry host from an image reference.
// "forgejo.example.com/ns/image:tag" → "forgejo.example.com"
func extractRegistry(imageRef string) string {
	parts := strings.SplitN(imageRef, "/", 2)
	if len(parts) == 2 && strings.Contains(parts[0], ".") {
		return parts[0]
	}
	return "https://index.docker.io/v1/"
}
