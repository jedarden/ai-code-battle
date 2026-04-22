package crosspoll

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/llm"
)

// ── Mock implementations ──────────────────────────────────────────────────

// mockStore implements programStore for testing.
type mockStore struct {
	mu           sync.Mutex
	programs     []*evolverdb.Program
	nextID       int64
	createdCalls [] *evolverdb.Program // captures programs passed to Create
}

func newMockStore(programs ...*evolverdb.Program) *mockStore {
	return &mockStore{
		programs: programs,
		nextID:   int64(len(programs)) + 1,
	}
}

func (m *mockStore) MaxGenerationByIsland(ctx context.Context) (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]int)
	for _, p := range m.programs {
		if p.Generation > result[p.Island] {
			result[p.Island] = p.Generation
		}
	}
	return result, nil
}

func (m *mockStore) ListTopByIsland(ctx context.Context, island string, limit int) ([]*evolverdb.Program, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Filter by island, already ordered by fitness desc in test data.
	var filtered []*evolverdb.Program
	for _, p := range m.programs {
		if p.Island == island {
			filtered = append(filtered, p)
		}
	}
	if limit > len(filtered) {
		limit = len(filtered)
	}
	return filtered[:limit], nil
}

func (m *mockStore) Create(ctx context.Context, p *evolverdb.Program) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	p.ID = m.nextID
	m.programs = append(m.programs, p)
	m.createdCalls = append(m.createdCalls, p)
	return m.nextID, nil
}

// mockLLM implements llmGenerator for testing.
type mockLLM struct {
	generateCalled bool
	generateFunc   func(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error)
}

func (m *mockLLM) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	m.generateCalled = true
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req)
	}
	return &llm.GenerateResponse{
		Candidate: &llm.Candidate{Code: "translated_code"},
	}, nil
}

// ── Existing tests ────────────────────────────────────────────────────────

func TestBuildTranslationPrompt_containsBothLanguages(t *testing.T) {
	got := buildTranslationPrompt("func main() {}", "go", "python")
	if !strings.Contains(got, "go") {
		t.Error("expected source language in prompt")
	}
	if !strings.Contains(got, "python") {
		t.Error("expected target language in prompt")
	}
	if !strings.Contains(got, "func main() {}") {
		t.Error("expected source code in prompt")
	}
}

func TestBuildTranslationPrompt_containsHTTPSpec(t *testing.T) {
	got := buildTranslationPrompt("code", "python", "rust")
	for _, want := range []string{"port 8080", "GET /health", "POST /turn", "HMAC"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in translation prompt", want)
		}
	}
}

func TestBuildTranslationPrompt_fencedCodeBlock(t *testing.T) {
	got := buildTranslationPrompt("print('hi')", "python", "go")
	if !strings.Contains(got, "```") {
		t.Error("expected fenced code block in prompt")
	}
}

func TestPickTargetIsland_neverSource(t *testing.T) {
	rng := newRandZero()
	c := &Checker{rng: rng}

	for _, source := range evolverdb.AllIslands {
		for i := 0; i < 20; i++ {
			target := c.pickTargetIsland(source)
			if target == source {
				t.Errorf("pickTargetIsland(%q) returned same island", source)
			}
		}
	}
}

func TestPickTargetIsland_validIsland(t *testing.T) {
	c := &Checker{rng: newRandZero()}
	valid := map[string]bool{}
	for _, island := range evolverdb.AllIslands {
		valid[island] = true
	}

	for _, source := range evolverdb.AllIslands {
		target := c.pickTargetIsland(source)
		if !valid[target] {
			t.Errorf("pickTargetIsland(%q) returned invalid island %q", source, target)
		}
	}
}

func TestCheckAndPollinate_noBoundary_noop(t *testing.T) {
	nextBoundary := ((0 / generationInterval) + 1) * generationInterval
	if nextBoundary <= 49 {
		t.Error("generation 49 should not trigger pollination")
	}
	if nextBoundary > 50 {
		t.Error("generation 50 should trigger pollination")
	}
}

func TestCheckAndPollinate_boundaryExactly50(t *testing.T) {
	prev := 0
	cur := 50
	nextBoundary := ((prev / generationInterval) + 1) * generationInterval
	if nextBoundary != 50 {
		t.Fatalf("expected first boundary at 50, got %d", nextBoundary)
	}
	if nextBoundary > cur {
		t.Error("50 should trigger at gen 50")
	}
}

func TestCheckAndPollination_multipleBoundaries(t *testing.T) {
	prev := 49
	cur := 102

	count := 0
	nextBoundary := ((prev / generationInterval) + 1) * generationInterval
	for nextBoundary <= cur && nextBoundary > 0 {
		count++
		nextBoundary += generationInterval
	}

	if count != 2 {
		t.Errorf("expected 2 pollinations for prev=49 cur=102, got %d", count)
	}
}

func TestCheckAndPollination_noDuplicateOnRecheck(t *testing.T) {
	prev := 50
	cur := 50

	nextBoundary := ((prev / generationInterval) + 1) * generationInterval
	if nextBoundary <= cur {
		t.Error("should not re-trigger at gen 50 when prev=50")
	}
}

func TestCheckAndPollination_skipsFrom0to100(t *testing.T) {
	prev := 0
	cur := 100

	count := 0
	nextBoundary := ((prev / generationInterval) + 1) * generationInterval
	for nextBoundary <= cur && nextBoundary > 0 {
		count++
		nextBoundary += generationInterval
	}

	if count != 2 {
		t.Errorf("expected 2 pollinations for prev=0 cur=100, got %d", count)
	}
}

// ── Integration tests with mock store ─────────────────────────────────────

func seedIsland(store *mockStore, island, lang string, fitness float64, generation int) {
	store.programs = append(store.programs, &evolverdb.Program{
		ID:             store.nextID,
		Code:           fmt.Sprintf("code_%s_gen%d", island, generation),
		Language:       lang,
		Island:         island,
		Generation:     generation,
		ParentIDs:      []int64{},
		BehaviorVector: []float64{0.5, 0.5},
		Fitness:        fitness,
	})
	store.nextID++
}

func TestCheckAndPollinate_fiftyGenerations_oneEventPerIsland(t *testing.T) {
	// Seed all 4 islands at generation 50 with top programs.
	store := newMockStore()
	for _, island := range evolverdb.AllIslands {
		seedIsland(store, island, "go", 100.0, 50)
		// Add a lower-fitness program so ListTopByIsland returns the right one
		seedIsland(store, island, "go", 50.0, 30)
	}

	llmClient := &mockLLM{}
	rng := rand.New(rand.NewSource(42))
	checker := &Checker{store: store, client: llmClient, rng: rng}

	prevGens := make(map[string]int) // all start at 0
	results, err := checker.CheckAndPollinate(context.Background(), prevGens, false)
	if err != nil {
		t.Fatalf("CheckAndPollinate: %v", err)
	}

	// Expect exactly 4 events — one per island.
	if len(results) != 4 {
		t.Fatalf("expected 4 pollination events (one per island), got %d", len(results))
	}

	// Verify each island triggered exactly once.
	triggered := make(map[string]int)
	for _, r := range results {
		triggered[r.SourceIsland]++
	}
	for _, island := range evolverdb.AllIslands {
		if triggered[island] != 1 {
			t.Errorf("island %s: expected 1 event, got %d", island, triggered[island])
		}
	}

	// Verify no translation needed (all same language).
	for _, r := range results {
		if r.Translated {
			t.Errorf("event from %s→%s should not have translation (same lang)", r.SourceIsland, r.TargetIsland)
		}
	}

	// Verify prevGens was updated.
	for _, island := range evolverdb.AllIslands {
		if prevGens[island] != 50 {
			t.Errorf("prevGens[%s] = %d, want 50", island, prevGens[island])
		}
	}
}

func TestCheckAndPollinate_underFifty_noEvents(t *testing.T) {
	store := newMockStore()
	for _, island := range evolverdb.AllIslands {
		seedIsland(store, island, "go", 100.0, 49)
	}

	llmClient := &mockLLM{}
	rng := rand.New(rand.NewSource(42))
	checker := &Checker{store: store, client: llmClient, rng: rng}

	prevGens := make(map[string]int)
	results, err := checker.CheckAndPollinate(context.Background(), prevGens, false)
	if err != nil {
		t.Fatalf("CheckAndPollinate: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 events for gen 49, got %d", len(results))
	}

	if llmClient.generateCalled {
		t.Error("LLM should not have been called")
	}
}

func TestCheckAndPollinate_copiedProgramAttributes(t *testing.T) {
	// Only alpha at gen 50 — other islands below boundary.
	store := newMockStore()
	seedIsland(store, evolverdb.IslandAlpha, "go", 100.0, 50)
	seedIsland(store, evolverdb.IslandBeta, "go", 80.0, 10)
	seedIsland(store, evolverdb.IslandGamma, "go", 70.0, 10)
	seedIsland(store, evolverdb.IslandDelta, "go", 60.0, 10)

	llmClient := &mockLLM{}
	rng := rand.New(rand.NewSource(42))
	checker := &Checker{store: store, client: llmClient, rng: rng}

	prevGens := make(map[string]int)
	results, err := checker.CheckAndPollinate(context.Background(), prevGens, false)
	if err != nil {
		t.Fatalf("CheckAndPollinate: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 event (alpha only at gen 50), got %d", len(results))
	}

	r := results[0]
	if r.SourceIsland != evolverdb.IslandAlpha {
		t.Errorf("source island: got %q, want %q", r.SourceIsland, evolverdb.IslandAlpha)
	}
	if r.SourceIsland == r.TargetIsland {
		t.Error("target island should differ from source")
	}

	// Verify the created program has the fitness penalty (0.9x).
	if len(store.createdCalls) != 1 {
		t.Fatalf("expected 1 Create call, got %d", len(store.createdCalls))
	}
	created := store.createdCalls[0]
	if created.Fitness != 90.0 {
		t.Errorf("migrated program fitness: got %f, want 90.0 (0.9*100)", created.Fitness)
	}
	if created.Island != r.TargetIsland {
		t.Errorf("migrated program island: got %q, want %q", created.Island, r.TargetIsland)
	}
	if len(created.ParentIDs) != 1 {
		t.Fatalf("migrated program should have 1 parent, got %d", len(created.ParentIDs))
	}
	// Parent should be the top program on alpha (ID=1).
	if created.ParentIDs[0] != 1 {
		t.Errorf("parent ID: got %d, want 1 (top alpha program)", created.ParentIDs[0])
	}
}

func TestCheckAndPollinate_translationTriggered_differentLanguages(t *testing.T) {
	// Alpha speaks Go; all other islands speak Python so any target needs translation.
	store := newMockStore()
	seedIsland(store, evolverdb.IslandAlpha, "go", 100.0, 50)
	seedIsland(store, evolverdb.IslandBeta, "python", 80.0, 10)
	seedIsland(store, evolverdb.IslandGamma, "python", 70.0, 10)
	seedIsland(store, evolverdb.IslandDelta, "python", 60.0, 10)

	translatedCode := "def handle_turn(): pass"
	llmClient := &mockLLM{
		generateFunc: func(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
			return &llm.GenerateResponse{
				Candidate: &llm.Candidate{Code: translatedCode},
			}, nil
		},
	}

	rng := rand.New(rand.NewSource(42))
	checker := &Checker{store: store, client: llmClient, rng: rng}

	prevGens := make(map[string]int)
	results, err := checker.CheckAndPollinate(context.Background(), prevGens, false)
	if err != nil {
		t.Fatalf("CheckAndPollinate: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 event, got %d", len(results))
	}

	r := results[0]
	if !r.Translated {
		t.Error("expected translation for go→python")
	}
	if !llmClient.generateCalled {
		t.Error("expected LLM to be called for translation")
	}
	if r.SourceLang != "go" {
		t.Errorf("source lang: got %q, want go", r.SourceLang)
	}
	if r.TargetLang != "python" {
		t.Errorf("target lang: got %q, want python", r.TargetLang)
	}

	// Verify the translated code was stored.
	if len(store.createdCalls) != 1 {
		t.Fatal("expected 1 Create call")
	}
	if store.createdCalls[0].Code != translatedCode {
		t.Errorf("stored code: got %q, want %q", store.createdCalls[0].Code, translatedCode)
	}
	if store.createdCalls[0].Language != "python" {
		t.Errorf("stored language: got %q, want python", store.createdCalls[0].Language)
	}
}

func TestCheckAndPollinate_sameLanguage_noTranslation(t *testing.T) {
	// All islands speak Go.
	store := newMockStore()
	seedIsland(store, evolverdb.IslandAlpha, "go", 100.0, 50)
	seedIsland(store, evolverdb.IslandBeta, "go", 80.0, 10)
	seedIsland(store, evolverdb.IslandGamma, "go", 70.0, 10)
	seedIsland(store, evolverdb.IslandDelta, "go", 60.0, 10)

	llmClient := &mockLLM{}
	rng := rand.New(rand.NewSource(42))
	checker := &Checker{store: store, client: llmClient, rng: rng}

	prevGens := make(map[string]int)
	results, err := checker.CheckAndPollinate(context.Background(), prevGens, false)
	if err != nil {
		t.Fatalf("CheckAndPollinate: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 event, got %d", len(results))
	}

	if results[0].Translated {
		t.Error("should not translate when languages match")
	}
	if llmClient.generateCalled {
		t.Error("LLM should not be called when languages match")
	}
}

func TestCheckAndPollinate_hundredGenerations_twoEvents(t *testing.T) {
	// Alpha at gen 100, beta at gen 50, others below.
	store := newMockStore()
	seedIsland(store, evolverdb.IslandAlpha, "go", 100.0, 100)
	seedIsland(store, evolverdb.IslandBeta, "go", 80.0, 50)
	seedIsland(store, evolverdb.IslandGamma, "go", 70.0, 30)
	seedIsland(store, evolverdb.IslandDelta, "go", 60.0, 20)

	llmClient := &mockLLM{}
	rng := rand.New(rand.NewSource(42))
	checker := &Checker{store: store, client: llmClient, rng: rng}

	prevGens := make(map[string]int)
	results, err := checker.CheckAndPollinate(context.Background(), prevGens, false)
	if err != nil {
		t.Fatalf("CheckAndPollinate: %v", err)
	}

	// Alpha crosses 50 and 100 = 2 events. Beta crosses 50 = 1 event. Total = 3.
	if len(results) != 3 {
		t.Fatalf("expected 3 events (alpha×2 + beta×1), got %d", len(results))
	}

	alphaCount := 0
	betaCount := 0
	for _, r := range results {
		if r.SourceIsland == evolverdb.IslandAlpha {
			alphaCount++
		}
		if r.SourceIsland == evolverdb.IslandBeta {
			betaCount++
		}
	}
	if alphaCount != 2 {
		t.Errorf("alpha: expected 2 events (gen 50 and 100), got %d", alphaCount)
	}
	if betaCount != 1 {
		t.Errorf("beta: expected 1 event (gen 50), got %d", betaCount)
	}
}

func TestCheckAndPollinate_emptyIsland_noEvent(t *testing.T) {
	store := newMockStore()
	seedIsland(store, evolverdb.IslandAlpha, "go", 100.0, 50)
	// Beta, gamma, delta have no programs.

	llmClient := &mockLLM{}
	rng := rand.New(rand.NewSource(42))
	checker := &Checker{store: store, client: llmClient, rng: rng}

	prevGens := make(map[string]int)
	results, err := checker.CheckAndPollinate(context.Background(), prevGens, false)
	if err != nil {
		t.Fatalf("CheckAndPollinate: %v", err)
	}

	// Only alpha triggers (it has gen 50). The others have gen 0, no boundary.
	if len(results) != 1 {
		t.Fatalf("expected 1 event (alpha only), got %d", len(results))
	}
	if results[0].SourceIsland != evolverdb.IslandAlpha {
		t.Errorf("source: got %q, want alpha", results[0].SourceIsland)
	}
}

// newRandZero creates a deterministic RNG for reproducible tests.
func newRandZero() *rand.Rand {
	return rand.New(rand.NewSource(0))
}
