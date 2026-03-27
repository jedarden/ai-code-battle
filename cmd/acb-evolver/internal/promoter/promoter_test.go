package promoter

import (
	"strings"
	"testing"
)

// ── dockerfileFor ─────────────────────────────────────────────────────────────

func TestDockerfileFor_AllSupportedLanguages(t *testing.T) {
	languages := []string{"go", "python", "rust", "typescript", "java", "php"}
	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			df, err := dockerfileFor(lang)
			if err != nil {
				t.Fatalf("dockerfileFor(%q) error: %v", lang, err)
			}
			if !strings.Contains(df, "FROM ") {
				t.Errorf("Dockerfile for %q missing FROM instruction", lang)
			}
			if !strings.Contains(df, "BOT_PORT") {
				t.Errorf("Dockerfile for %q missing BOT_PORT env var", lang)
			}
			if !strings.Contains(df, "BOT_SECRET") {
				t.Errorf("Dockerfile for %q missing BOT_SECRET env var", lang)
			}
			if !strings.Contains(df, "EXPOSE 8080") {
				t.Errorf("Dockerfile for %q missing EXPOSE 8080", lang)
			}
		})
	}
}

func TestDockerfileFor_UnsupportedLanguage(t *testing.T) {
	_, err := dockerfileFor("cobol")
	if err == nil {
		t.Error("expected error for unsupported language, got nil")
	}
}

func TestDockerfileFor_GoUsesMultistage(t *testing.T) {
	df, _ := dockerfileFor("go")
	if !strings.Contains(df, "AS builder") {
		t.Error("Go Dockerfile should use multi-stage build")
	}
	if !strings.Contains(df, "golang:") {
		t.Error("Go Dockerfile should use a golang base image")
	}
}

func TestDockerfileFor_RustUsesMultistage(t *testing.T) {
	df, _ := dockerfileFor("rust")
	if !strings.Contains(df, "AS builder") {
		t.Error("Rust Dockerfile should use multi-stage build")
	}
}

// ── generateBotID ─────────────────────────────────────────────────────────────

func TestGenerateBotID_Format(t *testing.T) {
	id, err := generateBotID()
	if err != nil {
		t.Fatalf("generateBotID error: %v", err)
	}
	if !strings.HasPrefix(id, "b_") {
		t.Errorf("bot ID %q does not start with 'b_'", id)
	}
	// b_ + 8 hex chars = 10 total
	if len(id) != 10 {
		t.Errorf("bot ID %q has length %d, want 10", id, len(id))
	}
}

func TestGenerateBotID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateBotID()
		if err != nil {
			t.Fatalf("generateBotID error at iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Errorf("duplicate bot ID generated: %s", id)
		}
		seen[id] = true
	}
}

// ── generateSecret ────────────────────────────────────────────────────────────

func TestGenerateSecret_Length(t *testing.T) {
	s, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret error: %v", err)
	}
	// 32 random bytes encoded as 64 hex chars
	if len(s) != 64 {
		t.Errorf("secret %q has length %d, want 64", s, len(s))
	}
}

func TestGenerateSecret_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		s, err := generateSecret()
		if err != nil {
			t.Fatalf("generateSecret error at iteration %d: %v", i, err)
		}
		if seen[s] {
			t.Errorf("duplicate secret generated: %s", s)
		}
		seen[s] = true
	}
}

// ── encryptAESGCM / decryptAESGCM ─────────────────────────────────────────────

func TestEncryptDecryptAESGCM_RoundTrip(t *testing.T) {
	// 32-byte key = 64 hex chars
	key := strings.Repeat("ab", 32) // "abababab..." 64 chars
	plaintext := "my-super-secret-bot-key"

	ct, err := encryptAESGCM(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ct == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}
}

func TestEncryptAESGCM_InvalidKey(t *testing.T) {
	_, err := encryptAESGCM("plaintext", "notahexkey")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

// ── manifest templates ────────────────────────────────────────────────────────

func TestManifestTemplates_Execute(t *testing.T) {
	data := manifestData{
		Name:         "acb-evo-test",
		Namespace:    "ai-code-battle",
		Island:       "alpha",
		Generation:   1,
		Registry:     "registry.example.com/acb",
		Port:         8080,
		SecretBase64: "dGVzdA==",
	}

	for name, tmpl := range map[string]interface{ Execute(interface{}, interface{}) error }{} {
		_ = name
		_ = tmpl
	}

	// Test secret manifest
	var buf strings.Builder
	if err := secretManifestTmpl.Execute(&buf, data); err != nil {
		t.Fatalf("secretManifestTmpl.Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "acb-evo-test-secret") {
		t.Error("secret manifest missing expected name")
	}
	if !strings.Contains(out, "dGVzdA==") {
		t.Error("secret manifest missing base64 secret")
	}

	// Test deployment manifest
	buf.Reset()
	if err := deployManifestTmpl.Execute(&buf, data); err != nil {
		t.Fatalf("deployManifestTmpl.Execute: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "acb-evo-test") {
		t.Error("deployment manifest missing bot name")
	}
	if !strings.Contains(out, "registry.example.com/acb/acb-evo-test:latest") {
		t.Error("deployment manifest missing full image reference")
	}
	if !strings.Contains(out, "acb/island: alpha") {
		t.Error("deployment manifest missing island label")
	}

	// Test service manifest
	buf.Reset()
	if err := svcManifestTmpl.Execute(&buf, data); err != nil {
		t.Fatalf("svcManifestTmpl.Execute: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "ClusterIP") {
		t.Error("service manifest missing ClusterIP type")
	}
}
