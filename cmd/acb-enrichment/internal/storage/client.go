// Package storage provides S3-compatible storage clients for B2 and R2.
package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is an S3-compatible storage client.
type Client struct {
	accessKey string
	secretKey string
	endpoint  string
	bucket    string
	httpClient *http.Client
}

// NewClient creates a new S3-compatible storage client.
func NewClient(accessKey, secretKey, endpoint, bucket string) *Client {
	return &Client{
		accessKey: accessKey,
		secretKey: secretKey,
		endpoint:  strings.TrimRight(endpoint, "/"),
		bucket:    bucket,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// HasCredentials returns true if the client has valid credentials configured.
func (c *Client) HasCredentials() bool {
	return c.accessKey != "" && c.secretKey != "" && c.endpoint != "" && c.bucket != ""
}

// FetchReplay fetches and decompresses a replay JSON from storage.
func (c *Client) FetchReplay(ctx context.Context, matchID string) (map[string]interface{}, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("storage credentials not configured")
	}

	// Try gzipped version first
	key := fmt.Sprintf("replays/%s.json.gz", matchID)
	data, err := c.fetchObject(ctx, key)
	if err != nil {
		// Fall back to uncompressed version
		key = fmt.Sprintf("replays/%s.json", matchID)
		data, err = c.fetchObject(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("fetch replay %s: %w", matchID, err)
		}
	}

	// Decompress if gzipped
	if strings.HasSuffix(key, ".gz") {
		data, err = gunzipData(data)
		if err != nil {
			return nil, fmt.Errorf("decompress replay: %w", err)
		}
	}

	var replay map[string]interface{}
	if err := json.Unmarshal(data, &replay); err != nil {
		return nil, fmt.Errorf("parse replay JSON: %w", err)
	}

	return replay, nil
}

// FetchMatchMetadata fetches match metadata from storage.
func (c *Client) FetchMatchMetadata(ctx context.Context, matchID string) (map[string]interface{}, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("storage credentials not configured")
	}

	key := fmt.Sprintf("matches/%s.json", matchID)
	data, err := c.fetchObject(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("fetch match metadata %s: %w", matchID, err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parse match metadata JSON: %w", err)
	}

	return metadata, nil
}

// UploadCommentary uploads commentary JSON to storage.
func (c *Client) UploadCommentary(ctx context.Context, matchID string, commentary map[string]interface{}) error {
	if !c.HasCredentials() {
		return fmt.Errorf("storage credentials not configured")
	}

	key := fmt.Sprintf("commentary/%s.json", matchID)
	data, err := json.MarshalIndent(commentary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal commentary: %w", err)
	}

	if err := c.putObject(ctx, key, data, "application/json"); err != nil {
		return fmt.Errorf("upload commentary: %w", err)
	}

	return nil
}

// fetchObject retrieves an object from S3-compatible storage.
func (c *Client) fetchObject(ctx context.Context, key string) ([]byte, error) {
	// Simple S3 GET request implementation
	url := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Add basic auth (many S3-compatible APIs accept this)
	req.SetBasicAuth(c.accessKey, c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("object not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// putObject uploads an object to S3-compatible storage.
func (c *Client) putObject(ctx context.Context, key string, data []byte, contentType string) error {
	url := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(c.accessKey, c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// gunzipData decompresses gzip data.
func gunzipData(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}
