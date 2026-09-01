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
	"sort"
	"strings"
	"time"
)

// Client is an S3-compatible storage client.
type Client struct {
	accessKey  string
	secretKey  string
	endpoint   string
	bucket     string
	httpClient *http.Client
}

// ClientInterface defines storage operations for enrichment.
type ClientInterface interface {
	FetchReplay(ctx context.Context, matchID string) (map[string]interface{}, error)
	FetchMatchMetadata(ctx context.Context, matchID string) (map[string]interface{}, error)
	UploadCommentary(ctx context.Context, matchID string, commentary map[string]interface{}) error
	HasCredentials() bool
}

// Ensure Client implements ClientInterface
var _ ClientInterface = (*Client)(nil)

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

// UploadCommentary uploads commentary in both JSON (for frontend) and Markdown formats.
func (c *Client) UploadCommentary(ctx context.Context, matchID string, commentary map[string]interface{}) error {
	if !c.HasCredentials() {
		return fmt.Errorf("storage credentials not configured")
	}

	// 1. Upload JSON format for frontend replay viewer
	jsonData, err := c.generateJSONCommentary(matchID, commentary)
	if err != nil {
		return fmt.Errorf("generate JSON commentary: %w", err)
	}

	jsonKey := fmt.Sprintf("commentary/%s.json", matchID)
	if err := c.putObject(ctx, jsonKey, jsonData, "application/json"); err != nil {
		return fmt.Errorf("upload JSON commentary: %w", err)
	}

	// 2. Upload Markdown format for human reading
	markdown := c.generateMarkdownCommentary(commentary)
	mdKey := fmt.Sprintf("commentary/%s-commentary.md", matchID)
	data := []byte(markdown)

	if err := c.putObject(ctx, mdKey, data, "text/markdown; charset=utf-8"); err != nil {
		return fmt.Errorf("upload Markdown commentary: %w", err)
	}

	return nil
}

// generateMarkdownCommentary converts the commentary structure to Markdown format.
func (c *Client) generateMarkdownCommentary(cm map[string]interface{}) string {
	var sb strings.Builder

	// Header
	matchID := ""
	if id, ok := cm["match_id"].(string); ok {
		matchID = id
		sb.WriteString(fmt.Sprintf("# AI Commentary: Match %s\n\n", matchID))
	}

	// Generated timestamp
	if genAt, ok := cm["generated_at"].(string); ok {
		sb.WriteString(fmt.Sprintf("*Generated at %s*\n\n", genAt))
	}

	// Summary
	if summary, ok := cm["summary"].(string); ok {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}

	// Narrative
	if narrative, ok := cm["narrative"].(string); ok {
		sb.WriteString("## Narrative\n\n")
		sb.WriteString(narrative)
		sb.WriteString("\n\n")
	}

	// Key Moments
	if keyMoments, ok := cm["key_moments"].([]interface{}); ok && len(keyMoments) > 0 {
		sb.WriteString("## Key Moments\n\n")
		for _, km := range keyMoments {
			if kmMap, ok := km.(map[string]interface{}); ok {
				turn := 0
				if t, ok := kmMap["turn"].(float64); ok {
					turn = int(t)
				}
				description := ""
				if desc, ok := kmMap["description"].(string); ok {
					description = desc
				}
				significance := "medium"
				if sig, ok := kmMap["significance"].(string); ok {
					significance = sig
				}

				sb.WriteString(fmt.Sprintf("### Turn %d (%s)\n\n", turn, strings.ToUpper(significance)))
				sb.WriteString(description)
				sb.WriteString("\n\n")

				// Tags
				if tags, ok := kmMap["tags"].([]interface{}); ok && len(tags) > 0 {
					sb.WriteString("**Tags:** ")
					for i, tag := range tags {
						if i > 0 {
							sb.WriteString(", ")
						}
						if tagStr, ok := tag.(string); ok {
							sb.WriteString(fmt.Sprintf("`%s`", tagStr))
						}
					}
					sb.WriteString("\n\n")
				}
			}
		}
	}

	return sb.String()
}

// generateJSONCommentary converts the commentary structure to JSON format for the frontend.
// The frontend expects EnrichedCommentary format: {match_id, generated_at, criteria, entries}
// where entries are turn-specific commentary with type classification.
func (c *Client) generateJSONCommentary(matchID string, cm map[string]interface{}) ([]byte, error) {
	// Build the JSON structure matching frontend expectations
	result := map[string]interface{}{
		"match_id":     matchID,
		"generated_at": cm["generated_at"],
		"criteria":     c.extractCriteria(cm),
		"entries":      c.buildCommentaryEntries(cm),
	}

	return json.Marshal(result)
}

// extractCriteria extracts enrichment criteria from commentary metadata.
func (c *Client) extractCriteria(cm map[string]interface{}) []string {
	var criteria []string

	// Extract from metadata if available
	if metadata, ok := cm["metadata"].(map[string]interface{}); ok {
		if isUpset, ok := metadata["IsUpset"].(bool); ok && isUpset {
			criteria = append(criteria, "upset")
		}
		if isClose, ok := metadata["IsCloseFinish"].(bool); ok && isClose {
			criteria = append(criteria, "close_finish")
		}
		if isFeatured, ok := metadata["IsFeatured"].(bool); ok && isFeatured {
			criteria = append(criteria, "featured")
		}
	}

	// Default to featured if no criteria set
	if len(criteria) == 0 {
		criteria = append(criteria, "featured")
	}

	return criteria
}

// buildCommentaryEntries converts key moments into turn-specific commentary entries.
func (c *Client) buildCommentaryEntries(cm map[string]interface{}) []map[string]interface{} {
	var entries []map[string]interface{}

	// Add summary as setup entry at turn 0
	if summary, ok := cm["summary"].(string); ok && summary != "" {
		entries = append(entries, map[string]interface{}{
			"turn": 0,
			"text": summary,
			"type": "setup",
		})
	}

	// Convert key moments to entries
	if keyMoments, ok := cm["key_moments"].([]interface{}); ok {
		for _, km := range keyMoments {
			if kmMap, ok := km.(map[string]interface{}); ok {
				turn := 0
				if t, ok := kmMap["turn"].(float64); ok {
					turn = int(t)
				}
				description := ""
				if desc, ok := kmMap["description"].(string); ok {
					description = desc
				}
				significance := "medium"
				if sig, ok := kmMap["significance"].(string); ok {
					significance = sig
				}

				// Map significance to entry type
				entryType := "action"
				switch significance {
				case "high":
					entryType = "climax"
				case "low":
					entryType = "setup"
				default:
					// Check tags for more specific typing
					if tags, ok := kmMap["tags"].([]interface{}); ok {
						for _, tag := range tags {
							if tagStr, ok := tag.(string); ok {
								if tagStr == "turning_point" || tagStr == "comeback" {
									entryType = "climax"
								} else if tagStr == "core_capture" {
									entryType = "reaction"
								}
							}
						}
					}
				}

				entries = append(entries, map[string]interface{}{
					"turn": turn,
					"text": description,
					"type": entryType,
				})
			}
		}
	}

	// Add narrative as denouement at final turn if available
	if narrative, ok := cm["narrative"].(string); ok && narrative != "" {
		// Find the highest turn number from key moments, or default to 999
		maxTurn := 999
		if len(entries) > 0 {
			for _, entry := range entries {
				if turn, ok := entry["turn"].(int); ok && turn > maxTurn {
					maxTurn = turn
				}
			}
		}
		entries = append(entries, map[string]interface{}{
			"turn": maxTurn + 1,
			"text": narrative,
			"type": "denouement",
		})
	}

	// Sort entries by turn
	sort.Slice(entries, func(i, j int) bool {
		turnI, _ := entries[i]["turn"].(int)
		turnJ, _ := entries[j]["turn"].(int)
		return turnI < turnJ
	})

	return entries
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
