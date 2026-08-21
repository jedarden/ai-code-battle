package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	dbstore "github.com/aicodebattle/acb/cmd/acb-enrichment/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/generator"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/llm"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/selector"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/storage"
)

func TestNewEnrichmentService(t *testing.T) {
	db, _ := sql.Open("postgres", "dummy")
	cfg := Config{
		PostgresHost:     "localhost",
		PostgresPort:     5432,
		PostgresUser:     "test",
		PostgresPassword: "test",
		PostgresDatabase: "test",
		DatabaseName:     "test",
		LLMBaseURL:       "https://api.test.com",
		LLMAPIKey:        "test-key",
		LLMModel:         "test-model",
		R2AccessKeyID:    "r2-key",
		R2SecretAccessKey: "r2-secret",
		R2Endpoint:       "https://r2.test.com",
		R2BucketName:     "test-bucket",
		B2AccessKeyID:    "b2-key",
		B2SecretAccessKey: "b2-secret",
		B2Endpoint:       "https://b2.test.com",
		B2BucketName:     "test-bucket",
		MinTurnCount:      100,
		MinWinProbCrossings: 3,
		UpsetThreshold:   150.0,
		MaxEnrichmentsPerHour: 20,
		MaxConcurrentRequests: 3,
	}

	svc := NewEnrichmentService(db, cfg)

	if svc == nil {
		t.Fatal("NewEnrichmentService returned nil")
	}
	if svc.db != db {
		t.Errorf("Expected db %v, got %v", db, svc.db)
	}
	if svc.store == nil {
		t.Error("Expected store to be initialized")
	}
	if svc.selector == nil {
		t.Error("Expected selector to be initialized")
	}
	if svc.generator == nil {
		t.Error("Expected generator to be initialized")
	}
	if svc.r2Client == nil {
		t.Error("Expected r2Client to be initialized")
	}
	if svc.b2Client == nil {
		t.Error("Expected b2Client to be initialized")
	}
	if svc.llmClient == nil {
		t.Error("Expected llmClient to be initialized")
	}
}

func TestEnrichmentService_CheckStorage(t *testing.T) {
	tests := []struct {
		name          string
		r2KeyID       string
		r2Secret      string
		r2Endpoint    string
		r2Bucket      string
		b2KeyID       string
		b2Secret      string
		b2Endpoint    string
		b2Bucket      string
		wantErr       bool
		expectedUse   string
	}{
		{
			name:        "R2 configured",
			r2KeyID:     "r2-key",
			r2Secret:    "r2-secret",
			r2Endpoint:  "https://r2.test.com",
			r2Bucket:    "test-bucket",
			wantErr:     false,
			expectedUse: "R2",
		},
		{
			name:        "B2 configured as fallback",
			b2KeyID:     "b2-key",
			b2Secret:    "b2-secret",
			b2Endpoint:  "https://b2.test.com",
			b2Bucket:    "test-bucket",
			wantErr:     false,
			expectedUse: "B2",
		},
		{
			name:   "no storage configured",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				R2AccessKeyID:     tt.r2KeyID,
				R2SecretAccessKey: tt.r2Secret,
				R2Endpoint:        tt.r2Endpoint,
				R2BucketName:      tt.r2Bucket,
				B2AccessKeyID:     tt.b2KeyID,
				B2SecretAccessKey: tt.b2Secret,
				B2Endpoint:        tt.b2Endpoint,
				B2BucketName:      tt.b2Bucket,
			}

			db, _ := sql.Open("postgres", "dummy")
			svc := NewEnrichmentService(db, cfg)

			err := svc.CheckStorage(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckStorage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnrichmentService_CheckLLM(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		apiKey       string
		wantErr      bool
	}{
		{
			name:    "API key configured",
			apiKey:  "test-key",
			wantErr: false,
		},
		{
			name:    "Base URL configured",
			baseURL: "https://api.test.com",
			wantErr: false,
		},
		{
			name:    "both configured",
			baseURL: "https://api.test.com",
			apiKey:  "test-key",
			wantErr: false,
		},
		{
			name:    "neither configured",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				LLMBaseURL: tt.baseURL,
				LLMAPIKey:  tt.apiKey,
			}

			db, _ := sql.Open("postgres", "dummy")
			svc := NewEnrichmentService(db, cfg)

			err := svc.CheckLLM(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckLLM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnrichmentService_RunCycle(t *testing.T) {
	tests := []struct {
		name         string
		selectorFunc func(context.Context) (*selector.SelectionResult, error)
		genFunc      func(context.Context, []dbstore.CandidateMatch) []generator.EnrichmentResult
		wantProcessed int
		wantEnriched  int
		wantSkipped   int
		wantFailed    int
		wantErr       bool
	}{
		{
			name: "successful cycle with enrichments",
			selectorFunc: func(ctx context.Context) (*selector.SelectionResult, error) {
				return &selector.SelectionResult{
					Matches: []dbstore.CandidateMatch{
						{MatchID: "match-1", TurnCount: 150},
						{MatchID: "match-2", TurnCount: 200},
					},
					Skipped: 1,
				}, nil
			},
			genFunc: func(ctx context.Context, matches []dbstore.CandidateMatch) []generator.EnrichmentResult {
				return []generator.EnrichmentResult{
					{MatchID: "match-1", Success: true, Duration: 10 * time.Second},
					{MatchID: "match-2", Success: true, Duration: 15 * time.Second},
				}
			},
			wantProcessed: 2,
			wantEnriched:  2,
			wantSkipped:   1,
			wantFailed:    0,
			wantErr:       false,
		},
		{
			name: "cycle with failures",
			selectorFunc: func(ctx context.Context) (*selector.SelectionResult, error) {
				return &selector.SelectionResult{
					Matches: []dbstore.CandidateMatch{
						{MatchID: "match-1", TurnCount: 150},
						{MatchID: "match-2", TurnCount: 200},
					},
					Skipped: 0,
				}, nil
			},
			genFunc: func(ctx context.Context, matches []dbstore.CandidateMatch) []generator.EnrichmentResult {
				return []generator.EnrichmentResult{
					{MatchID: "match-1", Success: true, Duration: 10 * time.Second},
					{MatchID: "match-2", Success: false, Error: errors.New("LLM error")},
				}
			},
			wantProcessed: 2,
			wantEnriched:  1,
			wantSkipped:   0,
			wantFailed:    1,
			wantErr:       false,
		},
		{
			name: "no candidates",
			selectorFunc: func(ctx context.Context) (*selector.SelectionResult, error) {
				return &selector.SelectionResult{
					Matches: []dbstore.CandidateMatch{},
					Skipped: 0,
				}, nil
			},
			genFunc: func(ctx context.Context, matches []dbstore.CandidateMatch) []generator.EnrichmentResult {
				return nil
			},
			wantProcessed: 0,
			wantEnriched:  0,
			wantSkipped:   0,
			wantFailed:    0,
			wantErr:       false,
		},
		{
			name: "selector error",
			selectorFunc: func(ctx context.Context) (*selector.SelectionResult, error) {
				return nil, errors.New("database error")
			},
			genFunc: func(ctx context.Context, matches []dbstore.CandidateMatch) []generator.EnrichmentResult {
				return nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock selector
			mockSelector := &mockSelector{
				selectFunc: tt.selectorFunc,
			}

			// Create a mock generator
			mockGen := &mockGenerator{
				enrichFunc: tt.genFunc,
			}

			svc := &EnrichmentService{
				selector:  mockSelector,
				generator: mockGen,
			}

			results, err := svc.RunCycle(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("RunCycle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if results.Processed != tt.wantProcessed {
				t.Errorf("RunCycle() Processed = %d, want %d", results.Processed, tt.wantProcessed)
			}
			if results.Enriched != tt.wantEnriched {
				t.Errorf("RunCycle() Enriched = %d, want %d", results.Enriched, tt.wantEnriched)
			}
			if results.Skipped != tt.wantSkipped {
				t.Errorf("RunCycle() Skipped = %d, want %d", results.Skipped, tt.wantSkipped)
			}
			if results.Failed != tt.wantFailed {
				t.Errorf("RunCycle() Failed = %d, want %d", results.Failed, tt.wantFailed)
			}
		})
	}
}

// Mock implementations for testing

type mockSelector struct {
	selectFunc func(context.Context) (*selector.SelectionResult, error)
}

func (m *mockSelector) Select(ctx context.Context) (*selector.SelectionResult, error) {
	if m.selectFunc != nil {
		return m.selectFunc(ctx)
	}
	return &selector.SelectionResult{}, nil
}

type mockGenerator struct {
	enrichFunc func(context.Context, []dbstore.CandidateMatch) []generator.EnrichmentResult
}

func (m *mockGenerator) EnrichMatches(ctx context.Context, matches []dbstore.CandidateMatch) []generator.EnrichmentResult {
	if m.enrichFunc != nil {
		return m.enrichFunc(ctx, matches)
	}
	return nil
}

// Test helper to create mock selector
func newMockSelector(fn func(context.Context) (*selector.SelectionResult, error)) *mockSelector {
	return &mockSelector{selectFunc: fn}
}

// Test helper to create mock generator
func newMockGenerator(fn func(context.Context, []dbstore.CandidateMatch) []generator.EnrichmentResult) *mockGenerator {
	return &mockGenerator{enrichFunc: fn}
}
