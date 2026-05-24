// Package generator orchestrates the enrichment process for selected matches.
package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/llm"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/storage"
)

// Generator creates AI commentary for match replays.
type Generator struct {
	storageClient *storage.Client
	llmClient     *llm.Client
	dbStore       *db.Store
	maxConcurrent int
}

// Config holds generator configuration.
type Config struct {
	MaxConcurrent int
}

// DefaultConfig returns default generator configuration.
func DefaultConfig() Config {
	return Config{
		MaxConcurrent: 3,
	}
}

// NewGenerator creates a new enrichment generator.
func NewGenerator(sClient *storage.Client, llmClient *llm.Client, dbStore *db.Store, cfg Config) *Generator {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = DefaultConfig().MaxConcurrent
	}
	return &Generator{
		storageClient: sClient,
		llmClient:     llmClient,
		dbStore:       dbStore,
		maxConcurrent: cfg.MaxConcurrent,
	}
}

// EnrichmentResult holds the result of enriching a single match.
type EnrichmentResult struct {
	MatchID  string
	Success  bool
	Error    error
	Duration time.Duration
}

// EnrichMatches processes multiple matches concurrently and generates commentary.
func (g *Generator) EnrichMatches(ctx context.Context, matches []db.CandidateMatch) []EnrichmentResult {
	results := make([]EnrichmentResult, len(matches))

	// Create semaphore for concurrency control
	sem := make(chan struct{}, g.maxConcurrent)
	var wg sync.WaitGroup

	for i, match := range matches {
		wg.Add(1)
		go func(idx int, m db.CandidateMatch) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			start := time.Now()
			success, err := g.enrichOne(ctx, m)
			results[idx] = EnrichmentResult{
				MatchID:  m.MatchID,
				Success:  success,
				Error:    err,
				Duration: time.Since(start),
			}
		}(i, match)
	}

	wg.Wait()
	return results
}

// enrichOne processes a single match.
func (g *Generator) enrichOne(ctx context.Context, match db.CandidateMatch) (bool, error) {
	// Fetch replay from storage
	replay, err := g.storageClient.FetchReplay(ctx, match.MatchID)
	if err != nil {
		return false, fmt.Errorf("fetch replay: %w", err)
	}

	// Build metadata
	metadata := llm.MatchMetadata{
		Players:       make([]llm.PlayerInfo, len(match.Players)),
		MapSize:       fmt.Sprintf("%dx%d", 60, 60), // Could extract from replay
		TurnCount:     match.TurnCount,
		Winner:        match.Winner,
		Condition:     match.Condition,
		FinalScores:   match.FinalScores,
		IsUpset:       match.IsUpset,
		IsCloseFinish: match.IsCloseFinish,
		IsFeatured:    true, // All selected matches are featured
	}

	for i, p := range match.Players {
		metadata.Players[i] = llm.PlayerInfo{
			ID:     p.ID,
			Name:   p.Name,
			Rating: p.Rating,
		}
	}

	// Build key moments (could extract from replay win_prob data)
	var keyMoments []llm.KeyMoment
	// For now, we'd extract these from the replay's critical_moments array
	if cm, ok := replay["critical_moments"].([]interface{}); ok {
		for _, cmi := range cm {
			if cm, ok := cmi.(map[string]interface{}); ok {
				km := llm.KeyMoment{
					Turn:        int(cm["turn"].(float64)),
					Description: cm["description"].(string),
				}
				if delta, ok := cm["delta"].(float64); ok {
					km.Delta = delta
				}
				keyMoments = append(keyMoments, km)
			}
		}
	}

	// Generate commentary
	req := llm.GenerateCommentaryRequest{
		MatchID:     match.MatchID,
		ReplayJSON:  fmt.Sprintf("%v", replay), // Simplified - would truncate in production
		Metadata:    metadata,
		KeyMoments:  keyMoments,
		MaxTokens:   3000,
		Temperature: 0.7,
	}

	commentary, err := g.llmClient.GenerateCommentary(ctx, req)
	if err != nil {
		return false, fmt.Errorf("generate commentary: %w", err)
	}

	// Store the result
	commentaryMap := map[string]interface{}{
		"match_id":     match.MatchID,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"key_moments":  commentary.KeyMoments,
		"summary":      commentary.Summary,
		"narrative":    commentary.Narrative,
		"metadata":     metadata,
	}

	commentaryJSON, err := json.Marshal(commentaryMap)
	if err != nil {
		return false, fmt.Errorf("marshal commentary: %w", err)
	}

	// Upload to storage
	if err := g.storageClient.UploadCommentary(ctx, match.MatchID, commentaryMap); err != nil {
		// If storage upload fails, try to mark as enriched in DB anyway
		// The commentary JSON contains the full data
	}

	// Mark match as enriched in database
	if err := g.dbStore.MarkEnriched(ctx, match.MatchID, string(commentaryJSON)); err != nil {
		return false, fmt.Errorf("mark enriched: %w", err)
	}

	return true, nil
}
