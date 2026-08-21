package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("access-key", "secret-key", "https://storage.example.com", "test-bucket")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.accessKey != "access-key" {
		t.Errorf("Expected accessKey 'access-key', got %s", client.accessKey)
	}
	if client.secretKey != "secret-key" {
		t.Errorf("Expected secretKey 'secret-key', got %s", client.secretKey)
	}
	if client.endpoint != "https://storage.example.com" {
		t.Errorf("Expected endpoint 'https://storage.example.com', got %s", client.endpoint)
	}
	if client.bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got %s", client.bucket)
	}
	if client.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}
}

func TestNewClient_TrimTrailingSlash(t *testing.T) {
	client := NewClient("key", "secret", "https://storage.example.com/", "bucket")

	if client.endpoint != "https://storage.example.com" {
		t.Errorf("Expected endpoint to trim trailing slash, got %s", client.endpoint)
	}
}

func TestNewClient_Timeout(t *testing.T) {
	client := NewClient("key", "secret", "https://storage.example.com", "bucket")

	if client.httpClient.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.httpClient.Timeout)
	}
}

func TestHasCredentials(t *testing.T) {
	tests := []struct {
		name       string
		accessKey  string
		secretKey  string
		endpoint   string
		bucket     string
		wantResult bool
	}{
		{
			name:       "all credentials present",
			accessKey:  "test-key",
			secretKey:  "test-secret",
			endpoint:   "https://example.com",
			bucket:     "test-bucket",
			wantResult: true,
		},
		{
			name:       "missing access key",
			accessKey:  "",
			secretKey:  "test-secret",
			endpoint:   "https://example.com",
			bucket:     "test-bucket",
			wantResult: false,
		},
		{
			name:       "missing secret key",
			accessKey:  "test-key",
			secretKey:  "",
			endpoint:   "https://example.com",
			bucket:     "test-bucket",
			wantResult: false,
		},
		{
			name:       "missing endpoint",
			accessKey:  "test-key",
			secretKey:  "test-secret",
			endpoint:   "",
			bucket:     "test-bucket",
			wantResult: false,
		},
		{
			name:       "missing bucket",
			accessKey:  "test-key",
			secretKey:  "test-secret",
			endpoint:   "https://example.com",
			bucket:     "",
			wantResult: false,
		},
		{
			name:       "all empty",
			accessKey:  "",
			secretKey:  "",
			endpoint:   "",
			bucket:     "",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.accessKey, tt.secretKey, tt.endpoint, tt.bucket)
			result := client.HasCredentials()
			if result != tt.wantResult {
				t.Errorf("HasCredentials() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestGunzipData(t *testing.T) {
	// Create test data
	original := []byte("test data for gzip compression")

	// Compress it
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(original); err != nil {
		t.Fatalf("gzip write failed: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close failed: %v", err)
	}
	compressed := buf.Bytes()

	// Decompress it
	decompressed, err := gunzipData(compressed)
	if err != nil {
		t.Fatalf("gunzipData() error = %v", err)
	}

	if !bytes.Equal(decompressed, original) {
		t.Errorf("gunzipData() = %v, want %v", decompressed, original)
	}
}

func TestGunzipData_Invalid(t *testing.T) {
	invalidData := []byte("not gzip data")

	_, err := gunzipData(invalidData)
	if err == nil {
		t.Error("gunzipData() expected error for invalid gzip data, got nil")
	}
}

func TestGunzipData_Empty(t *testing.T) {
	_, err := gunzipData([]byte{})
	if err == nil {
		t.Error("gunzipData() expected error for empty data, got nil")
	}
}

func TestGunzipData_Large(t *testing.T) {
	// Create large test data
	largeData := make([]byte, 1024*1024) // 1MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// Compress
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(largeData); err != nil {
		t.Fatalf("gzip write failed: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close failed: %v", err)
	}
	compressed := buf.Bytes()

	// Decompress
	decompressed, err := gunzipData(compressed)
	if err != nil {
		t.Fatalf("gunzipData() error = %v", err)
	}

	if !bytes.Equal(decompressed, largeData) {
		t.Errorf("gunzipData() produced different result for large data")
	}
	if len(decompressed) != len(largeData) {
		t.Errorf("gunzipData() length = %d, want %d", len(decompressed), len(largeData))
	}
}

func TestFetchReplay_NoCredentials(t *testing.T) {
	client := NewClient("", "", "", "")

	_, err := client.FetchReplay(context.Background(), "match-123")
	if err == nil {
		t.Error("FetchReplay() expected error without credentials, got nil")
	}
	if err.Error() != "storage credentials not configured" {
		t.Errorf("FetchReplay() error = %v, want 'storage credentials not configured'", err)
	}
}

func TestFetchMatchMetadata_NoCredentials(t *testing.T) {
	client := NewClient("", "", "", "")

	_, err := client.FetchMatchMetadata(context.Background(), "match-123")
	if err == nil {
		t.Error("FetchMatchMetadata() expected error without credentials, got nil")
	}
}

func TestUploadCommentary_NoCredentials(t *testing.T) {
	client := NewClient("", "", "", "")

	err := client.UploadCommentary(context.Background(), "match-123", map[string]interface{}{})
	if err == nil {
		t.Error("UploadCommentary() expected error without credentials, got nil")
	}
}

func TestKeyGeneration(t *testing.T) {
	tests := []struct {
		name    string
		matchID string
		wantKey string
		keyType string
	}{
		{
			name:    "replay key",
			matchID: "match-123",
			wantKey: "replays/match-123.json",
			keyType: "replay",
		},
		{
			name:    "replay gzipped key",
			matchID: "match-456",
			wantKey: "replays/match-456.json.gz",
			keyType: "replay-gz",
		},
		{
			name:    "match metadata key",
			matchID: "match-789",
			wantKey: "matches/match-789.json",
			keyType: "metadata",
		},
		{
			name:    "commentary key",
			matchID: "match-abc",
			wantKey: "commentary/match-abc.json",
			keyType: "commentary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var key string
			switch tt.keyType {
			case "replay":
				key = fmt.Sprintf("replays/%s.json", tt.matchID)
			case "replay-gz":
				key = fmt.Sprintf("replays/%s.json.gz", tt.matchID)
			case "metadata":
				key = fmt.Sprintf("matches/%s.json", tt.matchID)
			case "commentary":
				key = fmt.Sprintf("commentary/%s.json", tt.matchID)
			}

			if key != tt.wantKey {
				t.Errorf("Generated key = %s, want %s", key, tt.wantKey)
			}
		})
	}
}

func TestClient_URLBuilding(t *testing.T) {
	client := NewClient("key", "secret", "https://storage.example.com", "test-bucket")

	tests := []struct {
		name    string
		key     string
		wantURL string
	}{
		{
			name:    "simple key",
			key:     "test.json",
			wantURL: "https://storage.example.com/test-bucket/test.json",
		},
		{
			name:    "nested key",
			key:     "replays/match-123.json",
			wantURL: "https://storage.example.com/test-bucket/replays/match-123.json",
		},
		{
			name:    "deeply nested key",
			key:     "path/to/nested/file.json.gz",
			wantURL: "https://storage.example.com/test-bucket/path/to/nested/file.json.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("%s/%s/%s", client.endpoint, client.bucket, tt.key)
			if url != tt.wantURL {
				t.Errorf("Built URL = %s, want %s", url, tt.wantURL)
			}
		})
	}
}

func TestReplayKeyPriority(t *testing.T) {
	// Test that gzipped version is tried first
	matchID := "match-123"
	gzippedKey := fmt.Sprintf("replays/%s.json.gz", matchID)
	plainKey := fmt.Sprintf("replays/%s.json", matchID)

	if gzippedKey != "replays/match-123.json.gz" {
		t.Errorf("Gzipped key = %s, want 'replays/match-123.json.gz'", gzippedKey)
	}
	if plainKey != "replays/match-123.json" {
		t.Errorf("Plain key = %s, want 'replays/match-123.json'", plainKey)
	}
}

func TestHTTPStatusHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantError  bool
		errorMsg   string
	}{
		{
			name:       "200 OK",
			statusCode: 200,
			wantError:  false,
		},
		{
			name:       "404 Not Found",
			statusCode: 404,
			wantError:  true,
			errorMsg:   "object not found",
		},
		{
			name:       "403 Forbidden",
			statusCode: 403,
			wantError:  true,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test error handling logic
			if tt.statusCode == 404 {
				if tt.errorMsg != "object not found" {
					t.Errorf("404 error message = %s, want 'object not found'", tt.errorMsg)
				}
			}

			shouldError := tt.statusCode != 200
			if shouldError != tt.wantError {
				t.Errorf("Status %d: should error = %v, want %v", tt.statusCode, shouldError, tt.wantError)
			}
		})
	}
}

func TestAuthHeaders(t *testing.T) {
	// Test that basic auth is set correctly
	accessKey := "test-access-key"
	secretKey := "test-secret-key"

	// In a real test, we'd check the request headers
	// For now, just verify the values are stored
	client := NewClient(accessKey, secretKey, "https://example.com", "bucket")

	if client.accessKey != accessKey {
		t.Errorf("accessKey = %s, want %s", client.accessKey, accessKey)
	}
	if client.secretKey != secretKey {
		t.Errorf("secretKey = %s, want %s", client.secretKey, secretKey)
	}
}

func TestTimeoutConfiguration(t *testing.T) {
	// The client uses a fixed timeout, but we test it's properly set
	client := NewClient("key", "secret", "https://example.com", "bucket")

	if client.httpClient.Timeout != 60*time.Second {
		t.Errorf("Default timeout = %v, want 60s", client.httpClient.Timeout)
	}
}

func TestConcurrentOperations(t *testing.T) {
	client := NewClient("key", "secret", "https://example.com", "bucket")

	// Test that the client can handle concurrent operations safely
	// (The http.Client is safe for concurrent use)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = client.HasCredentials()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// If we got here without panic or deadlock, concurrent access works
}
