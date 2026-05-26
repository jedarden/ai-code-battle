package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// MockS3Client implements S3ClientInterface for testing
type MockS3Client struct {
	Objects      map[string]MockObject
	UploadCalls  []UploadCall
	DeleteCalls  []string
	CopyCalls    []CopyCall
	ShouldFailOn string // Set to make specific operations fail
}

type MockObject struct {
	Content      []byte
	LastModified time.Time
}

type UploadCall struct {
	Key  string
	Data []byte
}

type CopyCall struct {
	SourceKey string
	DestKey   string
}

func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		Objects:     make(map[string]MockObject),
		UploadCalls: []UploadCall{},
		DeleteCalls: []string{},
		CopyCalls:   []CopyCall{},
	}
}

func (m *MockS3Client) listObjects(ctx context.Context, prefix string) ([]R2Object, error) {
	if m.ShouldFailOn == "list" {
		return nil, context.DeadlineExceeded
	}

	var objects []R2Object
	for key, obj := range m.Objects {
		if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			objects = append(objects, R2Object{
				Key:          key,
				Size:         int64(len(obj.Content)),
				LastModified: obj.LastModified,
			})
		}
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(objects[j].LastModified)
	})
	return objects, nil
}

func (m *MockS3Client) deleteObject(ctx context.Context, key string) error {
	if m.ShouldFailOn == "delete" {
		return context.DeadlineExceeded
	}

	m.DeleteCalls = append(m.DeleteCalls, key)
	delete(m.Objects, key)
	return nil
}

func (m *MockS3Client) objectExists(ctx context.Context, key string) (bool, error) {
	if m.ShouldFailOn == "exists" {
		return false, context.DeadlineExceeded
	}

	_, exists := m.Objects[key]
	return exists, nil
}

func (m *MockS3Client) uploadFile(ctx context.Context, key string, body io.Reader, contentType string) error {
	if m.ShouldFailOn == "upload" {
		return context.DeadlineExceeded
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	m.UploadCalls = append(m.UploadCalls, UploadCall{Key: key, Data: data})
	m.Objects[key] = MockObject{
		Content:      data,
		LastModified: time.Now(),
	}
	return nil
}

func (m *MockS3Client) copyObject(ctx context.Context, sourceBucket, sourceKey, destKey string) error {
	if m.ShouldFailOn == "copy" {
		return context.DeadlineExceeded
	}

	m.CopyCalls = append(m.CopyCalls, CopyCall{SourceKey: sourceKey, DestKey: destKey})

	// Simulate copy by reading from source and writing to dest
	if obj, exists := m.Objects[sourceKey]; exists {
		m.Objects[destKey] = MockObject{
			Content:      obj.Content,
			LastModified: time.Now(),
		}
	}
	return nil
}

func (m *MockS3Client) downloadObject(ctx context.Context, key string) (io.ReadCloser, error) {
	if m.ShouldFailOn == "download" {
		return nil, context.DeadlineExceeded
	}

	obj, exists := m.Objects[key]
	if !exists {
		return nil, context.DeadlineExceeded
	}

	return io.NopCloser(bytes.NewReader(obj.Content)), nil
}

// Test GetS3ContentType
func TestGetS3ContentType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"replay.json.gz", "application/gzip"},
		{"data.json", "application/json"},
		{"card.png", "image/png"},
		{"file.unknown", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := getS3ContentType(tt.filename)
		if result != tt.expected {
			t.Errorf("getS3ContentType(%q) = %q, want %q", tt.filename, result, tt.expected)
		}
	}
}

// Test ExtractMatchIDFromKey
func TestExtractMatchIDFromKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"replays/abc123.json.gz", "abc123"},
		{"replays/match-456-def.json.gz", "match-456-def"},
		{"replays/test.json.gz", "test"},
		{"replays/", ""},
		{"invalid", ""},
	}

	for _, tt := range tests {
		result := extractMatchIDFromKey(tt.key)
		if result != tt.expected {
			t.Errorf("extractMatchIDFromKey(%q) = %q, want %q", tt.key, result, tt.expected)
		}
	}
}

// Test MockS3Client operations
func TestMockS3ClientUpload(t *testing.T) {
	ctx := context.Background()
	client := NewMockS3Client()

	content := []byte("test content")
	err := client.uploadFile(ctx, "test.txt", bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("uploadFile failed: %v", err)
	}

	if len(client.UploadCalls) != 1 {
		t.Errorf("expected 1 upload call, got %d", len(client.UploadCalls))
	}

	exists, err := client.objectExists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("objectExists failed: %v", err)
	}
	if !exists {
		t.Error("expected object to exist")
	}
}

func TestMockS3ClientDelete(t *testing.T) {
	ctx := context.Background()
	client := NewMockS3Client()

	// Add an object
	client.Objects["test.txt"] = MockObject{
		Content:      []byte("test"),
		LastModified: time.Now(),
	}

	// Delete it
	err := client.deleteObject(ctx, "test.txt")
	if err != nil {
		t.Fatalf("deleteObject failed: %v", err)
	}

	// Verify it's gone
	exists, _ := client.objectExists(ctx, "test.txt")
	if exists {
		t.Error("expected object to be deleted")
	}
}

func TestMockS3ClientList(t *testing.T) {
	ctx := context.Background()
	client := NewMockS3Client()

	// Add some objects
	now := time.Now()
	client.Objects["replays/match1.json.gz"] = MockObject{
		Content:      []byte("match1"),
		LastModified: now.Add(-2 * time.Hour),
	}
	client.Objects["replays/match2.json.gz"] = MockObject{
		Content:      []byte("match2"),
		LastModified: now.Add(-1 * time.Hour),
	}
	client.Objects["cards/bot1.png"] = MockObject{
		Content:      []byte("card1"),
		LastModified: now,
	}

	// List replay objects
	objects, err := client.listObjects(ctx, "replays/")
	if err != nil {
		t.Fatalf("listObjects failed: %v", err)
	}

	if len(objects) != 2 {
		t.Errorf("expected 2 objects, got %d", len(objects))
	}

	// Verify ordering (oldest first)
	if len(objects) >= 2 && objects[0].LastModified.After(objects[1].LastModified) {
		t.Error("expected objects sorted oldest first")
	}
}

// Test bundleWarmReplays with mock B2 client
func TestBundleWarmReplays(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockS3Client()

	// Add mock replays to B2
	mockClient.Objects["replays/match1.json.gz"] = MockObject{
		Content:      []byte(`{"turn": 1, "events": []}`),
		LastModified: time.Now(),
	}
	mockClient.Objects["replays/match2.json.gz"] = MockObject{
		Content:      []byte(`{"turn": 1, "events": []}`),
		LastModified: time.Now(),
	}

	// Create temporary output directory
	tmpDir, err := os.MkdirTemp("", "bundle-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{OutputDir: tmpDir}
	matchIDs := []string{"match1", "match2"}

	// Bundle replays
	err = bundleWarmReplays(ctx, cfg, mockClient, matchIDs)
	if err != nil {
		t.Fatalf("bundleWarmReplays failed: %v", err)
	}

	// Verify files were created
	for _, matchID := range matchIDs {
		expectedPath := filepath.Join(tmpDir, "data", "replays", matchID+".json.gz")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("expected replay file not created: %s", expectedPath)
		}
	}
}

// Test bundleWarmThumbnails with mock B2 client
func TestBundleWarmThumbnails(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockS3Client()

	// Add mock thumbnails to B2
	mockClient.Objects["thumbnails/match1.png"] = MockObject{
		Content:      []byte("fake-png-data"),
		LastModified: time.Now(),
	}

	// Create temporary output directory
	tmpDir, err := os.MkdirTemp("", "bundle-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{OutputDir: tmpDir}
	matchIDs := []string{"match1"}

	// Bundle thumbnails
	err = bundleWarmThumbnails(ctx, cfg, mockClient, matchIDs)
	if err != nil {
		t.Fatalf("bundleWarmThumbnails failed: %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tmpDir, "data", "thumbnails", "match1.png")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected thumbnail file not created: %s", expectedPath)
	}
}

// Test bundleWarmCards with mock B2 client
func TestBundleWarmCards(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockS3Client()

	// Add mock cards to B2
	mockClient.Objects["cards/bot1.png"] = MockObject{
		Content:      []byte("fake-png-data"),
		LastModified: time.Now(),
	}

	// Create temporary output directory
	tmpDir, err := os.MkdirTemp("", "bundle-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{OutputDir: tmpDir}
	botIDs := []string{"bot1"}

	// Bundle cards
	err = bundleWarmCards(ctx, cfg, mockClient, botIDs)
	if err != nil {
		t.Fatalf("bundleWarmCards failed: %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tmpDir, "data", "cards", "bot1.png")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected card file not created: %s", expectedPath)
	}
}

// Test bundleEvolutionLive with mock B2 client
func TestBundleEvolutionLive(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockS3Client()

	// Add mock live.json to B2
	liveData := `{"updated_at": "2026-05-26T00:00:00Z", "lineage": []}`
	mockClient.Objects["evolution/live.json"] = MockObject{
		Content:      []byte(liveData),
		LastModified: time.Now(),
	}

	// Create temporary output directory
	tmpDir, err := os.MkdirTemp("", "bundle-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{OutputDir: tmpDir}

	// Bundle evolution live.json
	err = bundleEvolutionLive(ctx, cfg, mockClient)
	if err != nil {
		t.Fatalf("bundleEvolutionLive failed: %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tmpDir, "data", "evolution", "live.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected live.json file not created: %s", expectedPath)
	}

	// Verify content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read live.json: %v", err)
	}
	if string(content) != liveData {
		t.Errorf("live.json content mismatch, got %s", string(content))
	}
}

// Test bundleWarmReplays with missing B2 objects (graceful handling)
func TestBundleWarmReplaysMissingObjects(t *testing.T) {
	ctx := context.Background()
	mockClient := NewMockS3Client() // Empty - no objects

	// Create temporary output directory
	tmpDir, err := os.MkdirTemp("", "bundle-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{OutputDir: tmpDir}
	matchIDs := []string{"nonexistent"}

	// Should not error when objects are missing
	err = bundleWarmReplays(ctx, cfg, mockClient, matchIDs)
	if err != nil {
		t.Errorf("bundleWarmReplays should not error on missing objects, got: %v", err)
	}
}

