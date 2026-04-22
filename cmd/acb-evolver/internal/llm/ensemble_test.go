package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDefaultEnsembleConfig(t *testing.T) {
	cfg := DefaultEnsembleConfig()
	if cfg.NumCandidates != 3 {
		t.Errorf("expected NumCandidates=3, got %d", cfg.NumCandidates)
	}
	if !cfg.RefineTop {
		t.Error("expected RefineTop=true")
	}
	if cfg.FastTierMaxTokens != 4096 {
		t.Errorf("expected FastTierMaxTokens=4096, got %d", cfg.FastTierMaxTokens)
	}
	if cfg.StrongTierMaxTokens != 8192 {
		t.Errorf("expected StrongTierMaxTokens=8192, got %d", cfg.StrongTierMaxTokens)
	}
	if cfg.Temperature != 0.85 {
		t.Errorf("expected Temperature=0.85, got %f", cfg.Temperature)
	}
}

func TestSelectBestCandidate_Empty(t *testing.T) {
	idx := selectBestCandidate([]Candidate{})
	if idx != -1 {
		t.Errorf("expected -1 for empty candidates, got %d", idx)
	}
}

func TestSelectBestCandidate_Single(t *testing.T) {
	candidates := []Candidate{
		{Code: "short", Language: "go"},
	}
	idx := selectBestCandidate(candidates)
	if idx != 0 {
		t.Errorf("expected 0 for single candidate, got %d", idx)
	}
}

func TestSelectBestCandidate_PrefersLonger(t *testing.T) {
	candidates := []Candidate{
		{Code: "short", Language: "go"},
		{Code: "this is a much longer piece of code that should score higher", Language: "go"},
		{Code: "medium length", Language: "go"},
	}
	idx := selectBestCandidate(candidates)
	if idx != 1 {
		t.Errorf("expected 1 (longest), got %d", idx)
	}
}

func TestSelectBestCandidate_GoHttpBonus(t *testing.T) {
	// Code with HTTP server patterns should score higher
	shortWithHttp := `package main
import "net/http"
func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
func handler(w http.ResponseWriter, r *http.Request) {}`

	longerNoHttp := strings.Repeat("x", 500)

	candidates := []Candidate{
		{Code: longerNoHttp, Language: "go"},
		{Code: shortWithHttp, Language: "go"},
	}

	idx := selectBestCandidate(candidates)
	// Calculate expected scores:
	// shortWithHttp: ~150 chars * 1.5 = 225 (HTTP bonus)
	// longerNoHttp: 500 chars * 1.0 = 500 (no bonus)
	// The longer code wins because 500 > 225
	if idx != 0 {
		t.Errorf("expected 0 (longer code), got %d", idx)
	}
}

func TestScoreCandidate_Bonuses(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		lang     string
		minScore float64
	}{
		{
			name:     "go with HTTP",
			code:     "func main() { http.HandleFunc(); ListenAndServe() }",
			lang:     "go",
			minScore: 60, // 51 chars * 1.5 = 76.5 for HTTP bonus
		},
		{
			name:     "python with Flask",
			code:     "def app(): from flask import Flask; app = Flask(__name__); app.run()",
			lang:     "python",
			minScore: 70, // ~50 chars * 1.5 = 75 for Flask bonus
		},
		{
			name:     "typescript with server",
			code:     "function createServer() { import { createServer } from 'http'; createServer().listen() }",
			lang:     "typescript",
			minScore: 75, // ~53 chars * 1.5 = 80 for server bonus
		},
		{
			name:     "rust with HTTP",
			code:     "fn main() { use hyper::Server; Server::bind().serve() }",
			lang:     "rust",
			minScore: 50, // ~50 chars * 1.5 = 75 for HTTP bonus
		},
		{
			name:     "java with HTTP",
			code:     "public static void main(String[] args) throws Exception { HttpServer.create() }",
			lang:     "java",
			minScore: 60, // ~60 chars * 1.5 = 90 for HTTP bonus
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := scoreCandidate(Candidate{Code: tc.code, Language: tc.lang})
			if score < tc.minScore {
				t.Errorf("expected score >= %f for %s, got %f", tc.minScore, tc.name, score)
			}
		})
	}
}

func TestContains(t *testing.T) {
	if !contains("hello world", "world") {
		t.Error("expected contains to find 'world'")
	}
	if contains("hello world", "mars") {
		t.Error("expected contains to not find 'mars'")
	}
	if !contains("test", "test") {
		t.Error("expected contains to find exact match")
	}
}

func TestContainsAll(t *testing.T) {
	if !containsAll("hello world foo", "hello", "world") {
		t.Error("expected containsAll to find both substrings")
	}
	if containsAll("hello world", "hello", "mars") {
		t.Error("expected containsAll to fail on missing substring")
	}
}

func TestBuildRefinementPrompt(t *testing.T) {
	original := "Write a bot"
	candidate := &Candidate{
		Code:     "func main() {}",
		Language: "go",
	}

	prompt := buildRefinementPrompt(original, candidate)

	if !strings.Contains(prompt, "Write a bot") {
		t.Error("expected original prompt to be included")
	}
	if !strings.Contains(prompt, "Previous Candidate") {
		t.Error("expected refinement section header")
	}
	if !strings.Contains(prompt, "func main() {}") {
		t.Error("expected candidate code to be included")
	}
	if !strings.Contains(prompt, "```go") {
		t.Error("expected go code block")
	}
}

func TestEnsembleResult_Empty(t *testing.T) {
	result := &EnsembleResult{}
	if result.Best != nil {
		t.Error("expected nil Best for empty result")
	}
}

func TestNoValidCandidatesError(t *testing.T) {
	err := ErrNoValidCandidates
	if err.Error() != "no valid code candidates were generated" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// Helper function to build JSON response with code content
func buildMockJSONResponse(code string) string {
	escapedCode := strings.ReplaceAll(code, "\n", "\\n")
	return "{\"choices\": [{\"message\": {\"role\": \"assistant\", \"content\": \"```go\\n" + escapedCode + "\\n```\"}}]}"
}

// Integration test with mock server
func TestEnsemble_WithMockServer(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		cc := callCount
		mu.Unlock()
		// Return a valid response with code block
		code := fmt.Sprintf("package main\nfunc main() { /* code %c }", rune('A'+cc))
		response := buildMockJSONResponse(code)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	cfg := EnsembleConfig{
		NumCandidates:     2,
		RefineTop:         false, // Skip refinement for this test
		FastTierMaxTokens: 1024,
		Temperature:       0.7,
	}

	result, err := client.Ensemble(context.Background(), "test prompt", "go", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.AllCandidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(result.AllCandidates))
	}

	if result.Best == nil {
		t.Fatal("expected non-nil Best candidate")
	}

	if result.RefinementApplied {
		t.Error("expected no refinement since RefineTop=false")
	}
}

func TestEnsemble_WithRefinement(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(callCount.Add(1))
		var code string
		if n <= 2 {
			// Fast tier responses
			code = "package main\nfunc main() { /* fast code */ }"
		} else {
			// Strong tier refinement
			code = "package main\nfunc main() { /* refined code */ }"
		}
		response := buildMockJSONResponse(code)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	cfg := EnsembleConfig{
		NumCandidates:       2,
		RefineTop:           true,
		FastTierMaxTokens:   1024,
		StrongTierMaxTokens: 2048,
		Temperature:         0.7,
	}

	result, err := client.Ensemble(context.Background(), "test prompt", "go", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.RefinementApplied {
		t.Error("expected refinement to be applied")
	}

	if result.Best == nil {
		t.Fatal("expected non-nil Best candidate")
	}

	// The refined code should contain "refined"
	if !strings.Contains(result.Best.Code, "refined") {
		t.Errorf("expected refined code, got: %s", result.Best.Code)
	}
}

func TestEnsemble_AllFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return invalid responses (no code blocks)
		response := `{"choices": [{"message": {"role": "assistant", "content": "This is just text with no code blocks."}}]}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	cfg := EnsembleConfig{
		NumCandidates: 2,
		RefineTop:     false,
	}

	result, err := client.Ensemble(context.Background(), "test prompt", "go", cfg)
	if err != ErrNoValidCandidates {
		t.Errorf("expected ErrNoValidCandidates, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}

	if len(result.Errors) == 0 {
		t.Error("expected errors to be recorded")
	}
}

func TestEnsemble_ZeroCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := "{\"choices\": [{\"message\": {\"content\": \"```go\\nx\\n```\"}}]}"
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	cfg := EnsembleConfig{
		NumCandidates: 0, // Should default to 1
		RefineTop:     false,
	}

	result, err := client.Ensemble(context.Background(), "test", "go", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.AllCandidates) != 1 {
		t.Errorf("expected 1 candidate (default), got %d", len(result.AllCandidates))
	}
}
