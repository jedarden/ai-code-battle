// Package live provides R2 upload for the evolution live.json feed.
package live

import (
	"strings"
	"testing"
)

func TestR2ConfigFromEnv(t *testing.T) {
	// Test with no env vars
	cfg := R2ConfigFromEnv()
	if cfg.AccessKey != "" {
		t.Error("expected empty AccessKey")
	}
	if cfg.HasCredentials() {
		t.Error("expected no credentials without env vars")
	}
}

func TestR2ConfigHasCredentials(t *testing.T) {
	tests := []struct {
		name      string
		accessKey string
		secretKey string
		endpoint  string
		bucket    string
		want      bool
	}{
		{
			name:      "all set",
			accessKey: "key",
			secretKey: "secret",
			endpoint:  "https://example.com",
			bucket:    "bucket",
			want:      true,
		},
		{
			name:      "missing access key",
			accessKey: "",
			secretKey: "secret",
			endpoint:  "https://example.com",
			bucket:    "bucket",
			want:      false,
		},
		{
			name:      "missing secret key",
			accessKey: "key",
			secretKey: "",
			endpoint:  "https://example.com",
			bucket:    "bucket",
			want:      false,
		},
		{
			name:      "missing endpoint",
			accessKey: "key",
			secretKey: "secret",
			endpoint:  "",
			bucket:    "bucket",
			want:      false,
		},
		{
			name:      "missing bucket",
			accessKey: "key",
			secretKey: "secret",
			endpoint:  "https://example.com",
			bucket:    "",
			want:      false,
		},
		{
			name:      "all empty",
			accessKey: "",
			secretKey: "",
			endpoint:  "",
			bucket:    "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &R2Config{
				AccessKey: tt.accessKey,
				SecretKey: tt.secretKey,
				Endpoint:  tt.endpoint,
				Bucket:    tt.bucket,
			}
			if got := cfg.HasCredentials(); got != tt.want {
				t.Errorf("HasCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewR2Client_MissingCredentials(t *testing.T) {
	cfg := &R2Config{} // no credentials
	_, err := NewR2Client(cfg)
	if err == nil {
		t.Error("expected error for missing credentials")
	}
	if !strings.Contains(err.Error(), "credentials not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	// Test with env var set
	t.Setenv("TEST_VAR", "value")
	if got := getEnvOrDefault("TEST_VAR", "default"); got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}

	// Test with env var not set
	if got := getEnvOrDefault("NONEXISTENT_VAR", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
}
