package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	dbstore "github.com/aicodebattle/acb/cmd/acb-enrichment/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/generator"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/llm"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/selector"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/storage"
)

// EnrichmentService manages the AI replay enrichment process.
type EnrichmentService struct {
	db        *sql.DB
	cfg       Config
	store     *dbstore.Store
	selector  matchSelector
	generator matchGenerator
	r2Client  *storage.Client
	b2Client  *storage.Client
	llmClient *llm.Client
}

type matchSelector interface {
	Select(context.Context) (*selector.SelectionResult, error)
}

type matchGenerator interface {
	EnrichMatches(context.Context, []dbstore.CandidateMatch) []generator.EnrichmentResult
}

// NewEnrichmentService creates a new enrichment service.
func NewEnrichmentService(db *sql.DB, cfg Config) *EnrichmentService {
	// Initialize database store
	store := dbstore.NewStore(db)

	// Initialize storage clients
	r2Client := storage.NewClient(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Endpoint, cfg.R2BucketName)
	b2Client := storage.NewClient(cfg.B2AccessKeyID, cfg.B2SecretAccessKey, cfg.B2Endpoint, cfg.B2BucketName)

	// Initialize LLM client
	llmClient := llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)

	// Initialize selector
	selCfg := selector.Config{
		MinTurnCount:   cfg.MinTurnCount,
		MinCrossings:   cfg.MinWinProbCrossings,
		UpsetThreshold: cfg.UpsetThreshold,
		MaxPerHour:     cfg.MaxEnrichmentsPerHour,
	}
	sel := selector.NewSelector(store, selCfg)

	// Initialize generator
	genCfg := generator.Config{
		MaxConcurrent: cfg.MaxConcurrentRequests,
	}
	// Prefer R2 for storage, fall back to B2
	storageClient := r2Client
	if !storageClient.HasCredentials() {
		storageClient = b2Client
	}
	gen := generator.NewGenerator(storageClient, llmClient, store, genCfg)

	return &EnrichmentService{
		db:        db,
		cfg:       cfg,
		store:     store,
		selector:  sel,
		generator: gen,
		r2Client:  r2Client,
		b2Client:  b2Client,
		llmClient: llmClient,
	}
}

// RunCycle executes one enrichment cycle.
func (s *EnrichmentService) RunCycle(ctx context.Context) (CycleResults, error) {
	results := CycleResults{}

	slog.Info("Selecting matches for enrichment")

	// Select candidates
	selection, err := s.selector.Select(ctx)
	if err != nil {
		return results, fmt.Errorf("select matches: %w", err)
	}

	results.Skipped = selection.Skipped
	candidates := selection.Matches

	if len(candidates) == 0 {
		slog.Info("No matches qualifying for enrichment")
		return results, nil
	}

	results.Processed = len(candidates)
	slog.Info("Found candidate matches", "count", len(candidates))

	// Enrich matches
	slog.Info("Generating commentary", "matches", len(candidates))
	enrichmentResults := s.generator.EnrichMatches(ctx, candidates)

	// Process results
	for _, er := range enrichmentResults {
		if er.Success {
			results.Enriched++
			slog.Info("Enriched match",
				"match_id", er.MatchID,
				"duration", er.Duration.String())
		} else {
			results.Failed++
			slog.Error("Failed to enrich match",
				"match_id", er.MatchID,
				"error", er.Error)
		}
	}

	return results, nil
}

// CheckStorage verifies that storage credentials are configured.
func (s *EnrichmentService) CheckStorage(ctx context.Context) error {
	// Check R2 first
	if s.r2Client.HasCredentials() {
		slog.Info("Using R2 for enrichment storage")
		return nil
	}

	// Fall back to B2
	if s.b2Client.HasCredentials() {
		slog.Info("Using B2 for enrichment storage")
		return nil
	}

	return fmt.Errorf("no storage credentials configured (R2 or B2 required)")
}

// CheckLLM verifies that LLM credentials are configured.
func (s *EnrichmentService) CheckLLM(ctx context.Context) error {
	if s.cfg.LLMAPIKey == "" && s.cfg.LLMBaseURL == "" {
		return fmt.Errorf("no LLM configuration (ACB_LLM_API_KEY or ACB_LLM_BASE_URL required)")
	}

	slog.Info("LLM configured",
		"base_url", s.cfg.LLMBaseURL,
		"model", s.cfg.LLMModel)

	return nil
}
