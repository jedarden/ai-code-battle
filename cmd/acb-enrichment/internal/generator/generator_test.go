package generator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/llm"
	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/storage"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxConcurrent != 3 {
		t.Errorf("Expected MaxConcurrent 3, got %d", cfg.MaxConcurrent)
	}
}

func TestNewGenerator_Defaults(t *testing.T) {
	mockStorage := &mockStorageClient{}
	mockLLM := &mockLLMClient{}
	mockDB := &mockDBStore{}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{})

	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if gen.maxConcurrent != 3 {
		t.Errorf("Expected maxConcurrent 3, got %d", gen.maxConcurrent)
	}
}

func TestNewGenerator_CustomConfig(t *testing.T) {
	mockStorage := &mockStorageClient{}
	mockLLM := &mockLLMClient{}
	mockDB := &mockDBStore{}

	cfg := Config{MaxConcurrent: 5}
	gen := NewGenerator(mockStorage, mockLLM, mockDB, cfg)

	if gen.maxConcurrent != 5 {
		t.Errorf("Expected maxConcurrent 5, got %d", gen.maxConcurrent)
	}
}

func TestEnrichMatches_Success(t *testing.T) {
	mockStorage := &mockStorageClient{
		replayData: map[string]interface{}{
			"turns":        []interface{}{},
			"win_probs":    []float64{0.5, 0.6, 0.4},
			"critical_moments": []interface{}{
				map[string]interface{}{"turn": 50.0, "description": "Core capture", "delta": 0.3},
			},
		},
	}
	mockLLM := &mockLLMClient{
		commentary: &llm.GenerateCommentaryResponse{
			KeyMoments: []llm.KeyMomentCommentary{
				{Turn: 50, Description: "Core capture", Significance: "high", Tags: []string{"combat", "core_capture"}},
			},
			Summary:   "An intense match with multiple lead changes",
			Narrative: "Bot1 established early control but Bot2 mounted a comeback in the mid-game",
		},
	}
	mockDB := &mockDBStore{}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{MaxConcurrent: 2})

	matches := []db.CandidateMatch{
		{
			MatchID:       "match-1",
			TurnCount:     150,
			Winner:        0,
			Condition:     "elimination",
			FinalScores:   []int{100, 95},
			IsUpset:       false,
			IsCloseFinish: true,
			Players: []db.PlayerData{
				{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500},
				{ID: 1, BotID: "bot-2", Name: "Bot2", Rating: 1400},
			},
		},
		{
			MatchID:       "match-2",
			TurnCount:     200,
			Winner:        1,
			Condition:     "elimination",
			FinalScores:   []int{95, 100},
			IsUpset:       true,
			IsCloseFinish: false,
			Players: []db.PlayerData{
				{ID: 0, BotID: "bot-3", Name: "Bot3", Rating: 1300},
				{ID: 1, BotID: "bot-4", Name: "Bot4", Rating: 1500},
			},
		},
	}

	results := gen.EnrichMatches(context.Background(), matches)

	if len(results) != 2 {
		t.Fatalf("EnrichMatches() returned %d results, want 2", len(results))
	}

	// Check first result
	if results[0].MatchID != "match-1" {
		t.Errorf("Results[0].MatchID = %s, want match-1", results[0].MatchID)
	}
	if !results[0].Success {
		t.Errorf("Results[0].Success = false, want true")
	}
	if results[0].Error != nil {
		t.Errorf("Results[0].Error = %v, want nil", results[0].Error)
	}
	if results[0].Duration == 0 {
		t.Error("Results[0].Duration = 0, want > 0")
	}

	// Check second result
	if results[1].MatchID != "match-2" {
		t.Errorf("Results[1].MatchID = %s, want match-2", results[1].MatchID)
	}
	if !results[1].Success {
		t.Errorf("Results[1].Success = false, want true")
	}
}

func TestEnrichMatches_WithFailures(t *testing.T) {
	mockStorage := &mockStorageClient{
		failOn: map[string]bool{"match-2": true},
	}
	mockLLM := &mockLLMClient{}
	mockDB := &mockDBStore{}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{MaxConcurrent: 2})

	matches := []db.CandidateMatch{
		{MatchID: "match-1", TurnCount: 150, Winner: 0, Condition: "elimination", FinalScores: []int{100, 95},
			Players: []db.PlayerData{{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500}},
		},
		{MatchID: "match-2", TurnCount: 200, Winner: 1, Condition: "elimination", FinalScores: []int{95, 100},
			Players: []db.PlayerData{{ID: 0, BotID: "bot-3", Name: "Bot3", Rating: 1300}},
		},
	}

	results := gen.EnrichMatches(context.Background(), matches)

	if len(results) != 2 {
		t.Fatalf("EnrichMatches() returned %d results, want 2", len(results))
	}

	// First should succeed
	if results[0].Success != true {
		t.Errorf("Results[0].Success = %v, want true", results[0].Success)
	}

	// Second should fail
	if results[1].Success != false {
		t.Errorf("Results[1].Success = %v, want false", results[1].Success)
	}
	if results[1].Error == nil {
		t.Error("Results[1].Error = nil, want error")
	}
}

func TestEnrichMatches_Concurrency(t *testing.T) {
	callCount := 0
	maxConcurrent := 0
	currentConcurrent := 0
	concurrentMutex := make(chan struct{}, 1)

	mockStorage := &mockStorageClient{
		slowFetch: true,
		fetchFunc: func(matchID string) {
			concurrentMutex <- struct{}{}
			currentConcurrent++
			if currentConcurrent > maxConcurrent {
				maxConcurrent = currentConcurrent
			}
			callCount++
			time.Sleep(50 * time.Millisecond)
			currentConcurrent--
			<-concurrentMutex
		},
	}
	mockLLM := &mockLLMClient{}
	mockDB := &mockDBStore{}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{MaxConcurrent: 2})

	matches := make([]db.CandidateMatch, 5)
	for i := range matches {
		matches[i] = db.CandidateMatch{
			MatchID: generateMatchID(i),
			TurnCount: 150,
			Winner:    0,
			Condition: "elimination",
			FinalScores: []int{100, 95},
			Players: []db.PlayerData{
				{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500},
				{ID: 1, BotID: "bot-2", Name: "Bot2", Rating: 1400},
			},
		}
	}

	start := time.Now()
	results := gen.EnrichMatches(context.Background(), matches)
	elapsed := time.Since(start)

	if len(results) != 5 {
		t.Fatalf("EnrichMatches() returned %d results, want 5", len(results))
	}

	if callCount != 5 {
		t.Errorf("Expected 5 calls, got %d", callCount)
	}

	// With MaxConcurrent=2 and 5 matches taking 50ms each:
	// Without concurrency: 250ms
	// With concurrency: ~150ms (3 batches: 2+2+1)
	if elapsed > 200*time.Millisecond {
		t.Errorf("EnrichMatches() took %v, concurrency may not be working (expected ~150ms)", elapsed)
	}

	// Verify we didn't exceed max concurrent
	if maxConcurrent > 2 {
		t.Errorf("Max concurrent was %d, want <= 2", maxConcurrent)
	}
}

func TestEnrichMatches_Empty(t *testing.T) {
	mockStorage := &mockStorageClient{}
	mockLLM := &mockLLMClient{}
	mockDB := &mockDBStore{}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{})

	results := gen.EnrichMatches(context.Background(), []db.CandidateMatch{})

	if len(results) != 0 {
		t.Errorf("EnrichMatches() returned %d results, want 0", len(results))
	}
}

func TestEnrichOne_StorageFailure(t *testing.T) {
	mockStorage := &mockStorageClient{
		failOn: map[string]bool{"match-1": true},
	}
	mockLLM := &mockLLMClient{}
	mockDB := &mockDBStore{}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{})

	match := db.CandidateMatch{
		MatchID: "match-1",
		TurnCount: 150,
		Winner:    0,
		Condition: "elimination",
		FinalScores: []int{100, 95},
		Players:   []db.PlayerData{{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500}},
	}

	success, err := gen.enrichOne(context.Background(), match)

	if success {
		t.Error("enrichOne() returned success=true, want false")
	}
	if err == nil {
		t.Error("enrichOne() returned nil error, want error")
	}
}

func TestEnrichOne_LLMFailure(t *testing.T) {
	mockStorage := &mockStorageClient{
		replayData: map[string]interface{}{"turns": []interface{}{}},
	}
	mockLLM := &mockLLMClient{
		failOn: map[string]bool{"match-1": true},
	}
	mockDB := &mockDBStore{}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{})

	match := db.CandidateMatch{
		MatchID: "match-1",
		TurnCount: 150,
		Winner:    0,
		Condition: "elimination",
		FinalScores: []int{100, 95},
		Players:   []db.PlayerData{{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500}},
	}

	success, err := gen.enrichOne(context.Background(), match)

	if success {
		t.Error("enrichOne() returned success=true, want false")
	}
	if err == nil {
		t.Error("enrichOne() returned nil error, want error")
	}
}

func TestEnrichOne_DatabaseFailure(t *testing.T) {
	mockStorage := &mockStorageClient{
		replayData: map[string]interface{}{"turns": []interface{}{}},
	}
	mockLLM := &mockLLMClient{}
	mockDB := &mockDBStore{
		failOn: map[string]bool{"match-1": true},
	}

	gen := NewGenerator(mockStorage, mockLLM, mockDB, Config{})

	match := db.CandidateMatch{
		MatchID: "match-1",
		TurnCount: 150,
		Winner:    0,
		Condition: "elimination",
		FinalScores: []int{100, 95},
		Players:   []db.PlayerData{{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500}},
	}

	success, err := gen.enrichOne(context.Background(), match)

	if success {
		t.Error("enrichOne() returned success=true, want false")
	}
	if err == nil {
		t.Error("enrichOne() returned nil error, want error")
	}
}

// Mock implementations

type mockStorageClient struct {
	replayData map[string]interface{}
	failOn     map[string]bool
	slowFetch  bool
	fetchFunc  func(matchID string)
}

func (m *mockStorageClient) HasCredentials() bool {
	return true
}

func (m *mockStorageClient) FetchReplay(ctx context.Context, matchID string) (map[string]interface{}, error) {
	if m.fetchFunc != nil {
		m.fetchFunc(matchID)
	}
	if m.slowFetch {
		time.Sleep(50 * time.Millisecond)
	}
	if m.failOn != nil && m.failOn[matchID] {
		return nil, errors.New("storage error")
	}
	return m.replayData, nil
}

func (m *mockStorageClient) UploadCommentary(ctx context.Context, matchID string, commentary map[string]interface{}) error {
	return nil
}

type mockLLMClient struct {
	commentary *llm.GenerateCommentaryResponse
	failOn     map[string]bool
}

func (m *mockLLMClient) GenerateCommentary(ctx context.Context, req llm.GenerateCommentaryRequest) (*llm.GenerateCommentaryResponse, error) {
	if m.failOn != nil && m.failOn[req.MatchID] {
		return nil, errors.New("LLM error")
	}
	return m.commentary, nil
}

type mockDBStore struct {
	failOn map[string]bool
}

func (m *mockDBStore) MarkEnriched(ctx context.Context, matchID string, commentaryJSON string) error {
	if m.failOn != nil && m.failOn[matchID] {
		return errors.New("database error")
	}
	return nil
}

// Helper function

func generateMatchID(i int) string {
	return "match-" + string(rune('a'+i%26))
}
