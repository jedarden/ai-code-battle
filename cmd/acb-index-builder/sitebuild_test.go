package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractRegistry(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{"forgejo.ardenone.com/ai-code-battle/acb-site-build:latest", "forgejo.ardenone.com"},
		{"forgejo.ardenone.com/ai-code-battle/acb-site-build", "forgejo.ardenone.com"},
		{"docker.io/library/nginx:latest", "docker.io"},
		{"nginx:latest", "https://index.docker.io/v1/"},
		{"localhost:5000/myimage:tag", "https://index.docker.io/v1/"},
		{"", "https://index.docker.io/v1/"},
	}
	for _, tt := range tests {
		got := extractRegistry(tt.image)
		if got != tt.want {
			t.Errorf("extractRegistry(%q) = %q, want %q", tt.image, got, tt.want)
		}
	}
}

func TestReadWriteCachedDigest(t *testing.T) {
	dir := t.TempDir()

	if d := readCachedDigest(dir); d != "" {
		t.Errorf("expected empty digest for missing file, got %q", d)
	}

	writeCachedDigest(dir, "sha256:abc123\n")
	if d := readCachedDigest(dir); d != "sha256:abc123" {
		t.Errorf("readCachedDigest = %q, want %q", d, "sha256:abc123")
	}

	writeCachedDigest(dir, "sha256:def456\n")
	if d := readCachedDigest(dir); d != "sha256:def456" {
		t.Errorf("readCachedDigest after overwrite = %q, want %q", d, "sha256:def456")
	}
}

func TestReadCachedDigest_InvalidPath(t *testing.T) {
	d := readCachedDigest("/nonexistent/path/that/does/not/exist")
	if d != "" {
		t.Errorf("expected empty digest for nonexistent dir, got %q", d)
	}
}

func TestWriteCachedDigest_InvalidPath(t *testing.T) {
	// Should not panic, just log a warning
	writeCachedDigest("/nonexistent/path", "sha256:abc")
}

func TestExtractedDistPath(t *testing.T) {
	cfg := &Config{SiteBuildPath: "dist"}
	got := extractedDistPath(cfg)
	want := filepath.Join(siteBuildExtractDir, "dist")
	if got != want {
		t.Errorf("extractedDistPath() = %q, want %q", got, want)
	}

	cfg2 := &Config{SiteBuildPath: "build/output"}
	got2 := extractedDistPath(cfg2)
	want2 := filepath.Join(siteBuildExtractDir, "build/output")
	if got2 != want2 {
		t.Errorf("extractedDistPath() = %q, want %q", got2, want2)
	}
}

func TestFallbackWebDir_NothingExists(t *testing.T) {
	oldBakedIn := bakedInWebDist
	bakedInWebDist = filepath.Join(t.TempDir(), "baked-in")
	defer func() { bakedInWebDist = oldBakedIn }()

	cfg := &Config{
		SiteBuildPath: "dist",
		OutputDir:     t.TempDir(),
	}
	got := fallbackWebDir(cfg)
	if got != bakedInWebDist {
		t.Errorf("fallbackWebDir() = %q, want %q (baked-in)", got, bakedInWebDist)
	}
}

func TestFallbackWebDir_ExtractedExists(t *testing.T) {
	oldExtractDir := siteBuildExtractDir
	siteBuildExtractDir = filepath.Join(t.TempDir(), "extract")
	defer func() { siteBuildExtractDir = oldExtractDir }()

	cfg := &Config{
		SiteBuildPath: "dist",
		OutputDir:     t.TempDir(),
	}
	extractedPath := extractedDistPath(cfg)
	if err := os.MkdirAll(extractedPath, 0755); err != nil {
		t.Fatal(err)
	}

	got := fallbackWebDir(cfg)
	if got != extractedPath {
		t.Errorf("fallbackWebDir() = %q, want %q (extracted)", got, extractedPath)
	}
}

func TestFallbackWebDir_BakedInExists(t *testing.T) {
	bakedInDir := filepath.Join(t.TempDir(), "baked-in")
	oldBakedIn := bakedInWebDist
	bakedInWebDist = bakedInDir
	defer func() { bakedInWebDist = oldBakedIn }()

	oldExtractDir := siteBuildExtractDir
	siteBuildExtractDir = filepath.Join(t.TempDir(), "extract")
	defer func() { siteBuildExtractDir = oldExtractDir }()

	if err := os.MkdirAll(bakedInDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		SiteBuildPath: "dist",
		OutputDir:     t.TempDir(),
	}
	got := fallbackWebDir(cfg)
	if got != bakedInDir {
		t.Errorf("fallbackWebDir() = %q, want %q (baked-in)", got, bakedInDir)
	}
}

func TestInitCraneAuth_NoAuth(t *testing.T) {
	cfg := &Config{RegistryUsername: "", SiteBuildImage: ""}
	if err := initCraneAuth(cfg); err != nil {
		t.Errorf("initCraneAuth with no auth should be no-op, got %v", err)
	}
}

func TestInitCraneAuth_WithAuth(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir := craneConfigDir
	craneConfigDir = filepath.Join(tmpDir, "crane-cfg")
	defer func() { craneConfigDir = oldDir }()

	cfg := &Config{
		RegistryUsername: "testuser",
		RegistryPassword: "testpass",
		SiteBuildImage:   "forgejo.example.com/ns/image:tag",
	}
	if err := initCraneAuth(cfg); err != nil {
		t.Fatalf("initCraneAuth: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(craneConfigDir, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse config.json: %v", err)
	}

	auths, ok := config["auths"].(map[string]interface{})
	if !ok {
		t.Fatal("config.json missing 'auths' key")
	}
	if _, ok := auths["forgejo.example.com"]; !ok {
		t.Error("config.json missing registry entry for forgejo.example.com")
	}
}

func TestInitCraneAuth_PasswordOnly(t *testing.T) {
	cfg := &Config{RegistryUsername: "", RegistryPassword: "pass", SiteBuildImage: "img"}
	if err := initCraneAuth(cfg); err != nil {
		t.Errorf("initCraneAuth with no username should be no-op, got %v", err)
	}
}

func TestCraneEnviron_NoConfig(t *testing.T) {
	env := craneEnviron()
	hasDockerConfig := false
	for _, e := range env {
		if strings.HasPrefix(e, "DOCKER_CONFIG=") {
			hasDockerConfig = true
		}
	}
	if hasDockerConfig {
		t.Error("craneEnviron should not set DOCKER_CONFIG when config.json doesn't exist")
	}
}

func TestCopyWebAssets(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	os.MkdirAll(filepath.Join(srcDir, "js"), 0755)
	os.MkdirAll(filepath.Join(srcDir, "css"), 0755)
	os.WriteFile(filepath.Join(srcDir, "index.html"), []byte("<html></html>"), 0644)
	os.WriteFile(filepath.Join(srcDir, "js", "app.js"), []byte("// app"), 0644)
	os.WriteFile(filepath.Join(srcDir, "css", "style.css"), []byte("body{}"), 0644)

	cfg := &Config{OutputDir: dstDir}
	if err := copyWebAssets(cfg, srcDir); err != nil {
		t.Fatalf("copyWebAssets: %v", err)
	}

	assertFileContent(t, filepath.Join(dstDir, "index.html"), "<html></html>")
	assertFileContent(t, filepath.Join(dstDir, "js", "app.js"), "// app")
	assertFileContent(t, filepath.Join(dstDir, "css", "style.css"), "body{}")
}

func TestCopyWebAssets_OverlaysOnExistingData(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Pre-existing data files in output directory
	os.MkdirAll(filepath.Join(dstDir, "data"), 0755)
	os.WriteFile(filepath.Join(dstDir, "data", "leaderboard.json"), []byte(`{"old": true}`), 0644)

	// Site build has its own data placeholder
	os.MkdirAll(filepath.Join(srcDir, "data"), 0755)
	os.WriteFile(filepath.Join(srcDir, "index.html"), []byte("<html>"), 0644)
	os.WriteFile(filepath.Join(srcDir, "data", "leaderboard.json"), []byte(`{"placeholder": true}`), 0644)

	cfg := &Config{OutputDir: dstDir}
	if err := copyWebAssets(cfg, srcDir); err != nil {
		t.Fatalf("copyWebAssets: %v", err)
	}

	// Should have the site build's data (will be overwritten by generateAllIndexes later)
	assertFileContent(t, filepath.Join(dstDir, "index.html"), "<html>")
	assertFileContent(t, filepath.Join(dstDir, "data", "leaderboard.json"), `{"placeholder": true}`)
}

func TestCopyWebAssets_EmptySource(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	cfg := &Config{OutputDir: dstDir}
	if err := copyWebAssets(cfg, srcDir); err != nil {
		t.Fatalf("copyWebAssets with empty source: %v", err)
	}
}

func TestCopyWebAssets_NonexistentSource(t *testing.T) {
	cfg := &Config{OutputDir: t.TempDir()}
	err := copyWebAssets(cfg, "/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Errorf("content of %s = %q, want %q", path, string(data), want)
	}
}
