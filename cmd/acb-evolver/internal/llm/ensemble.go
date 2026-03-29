// Package llm provides an OpenAI-compatible LLM client and utilities for
// extracting bot code from model responses.
package llm

import (
	"context"
	"sync"
)

// EnsembleConfig configures the ensemble generation behavior.
type EnsembleConfig struct {
	// NumCandidates is the number of candidates to generate in parallel.
	// Default: 3
	NumCandidates int
	// RefineTop indicates whether to refine the best candidate with the strong tier.
	// Default: true
	RefineTop bool
	// FastTierMaxTokens is the max tokens for fast tier generation.
	// Default: 4096
	FastTierMaxTokens int
	// StrongTierMaxTokens is the max tokens for strong tier refinement.
	// Default: 8192
	StrongTierMaxTokens int
	// Temperature for generation. Default: 0.85
	Temperature float64
}

// DefaultEnsembleConfig returns a sensible default configuration.
func DefaultEnsembleConfig() EnsembleConfig {
	return EnsembleConfig{
		NumCandidates:       3,
		RefineTop:           true,
		FastTierMaxTokens:   4096,
		StrongTierMaxTokens: 8192,
		Temperature:         0.85,
	}
}

// EnsembleResult holds the results of ensemble generation.
type EnsembleResult struct {
	// Best is the selected best candidate after optional refinement.
	Best *Candidate
	// BestRawText is the raw LLM output for the best candidate.
	BestRawText string
	// AllCandidates contains all generated candidates before selection.
	AllCandidates []Candidate
	// AllRawTexts contains all raw LLM outputs.
	AllRawTexts []string
	// RefinementApplied indicates if strong-tier refinement was applied.
	RefinementApplied bool
	// Errors contains any errors from individual generations.
	Errors []error
}

// Ensemble generates multiple candidates in parallel using the fast tier,
// selects the best one, and optionally refines it with the strong tier.
//
// The selection strategy prefers:
// 1. Longer code blocks (more complete implementations)
// 2. Code that passes basic structural checks
func (c *Client) Ensemble(ctx context.Context, prompt string, targetLang string, cfg EnsembleConfig) (*EnsembleResult, error) {
	if cfg.NumCandidates <= 0 {
		cfg.NumCandidates = 1
	}

	// Generate candidates in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	candidates := make([]Candidate, 0, cfg.NumCandidates)
	rawTexts := make([]string, 0, cfg.NumCandidates)
	errors := make([]error, 0)

	for i := 0; i < cfg.NumCandidates; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			maxTokens := cfg.FastTierMaxTokens
			if maxTokens == 0 {
				maxTokens = defaultMaxTokens
			}
			temp := cfg.Temperature
			if temp == 0 {
				temp = defaultTemperature
			}

			resp, err := c.Generate(ctx, GenerateRequest{
				Prompt:      prompt,
				Tier:        TierFast,
				MaxTokens:   maxTokens,
				Temperature: temp,
				TargetLang:  targetLang,
			})

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors = append(errors, err)
				return
			}

			if resp.Candidate != nil {
				candidates = append(candidates, *resp.Candidate)
				rawTexts = append(rawTexts, resp.RawText)
			}
		}(i)
	}
	wg.Wait()

	// If no candidates were generated, return error
	if len(candidates) == 0 {
		return &EnsembleResult{Errors: errors}, ErrNoValidCandidates
	}

	// Select the best candidate (longest code block as heuristic)
	bestIdx := selectBestCandidate(candidates)
	best := &candidates[bestIdx]
	bestRaw := rawTexts[bestIdx]

	result := &EnsembleResult{
		AllCandidates: candidates,
		AllRawTexts:   rawTexts,
		Errors:        errors,
	}

	// Optionally refine with strong tier
	if cfg.RefineTop {
		refined, refineRaw, err := c.refineCandidate(ctx, prompt, best, targetLang, cfg)
		if err == nil && refined != nil {
			result.Best = refined
			result.BestRawText = refineRaw
			result.RefinementApplied = true
		} else {
			// Refinement failed, use the original best
			result.Best = best
			result.BestRawText = bestRaw
		}
	} else {
		result.Best = best
		result.BestRawText = bestRaw
	}

	return result, nil
}

// refineCandidate uses the strong tier to improve a candidate.
func (c *Client) refineCandidate(ctx context.Context, originalPrompt string, candidate *Candidate, targetLang string, cfg EnsembleConfig) (*Candidate, string, error) {
	refinementPrompt := buildRefinementPrompt(originalPrompt, candidate)

	maxTokens := cfg.StrongTierMaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	resp, err := c.Generate(ctx, GenerateRequest{
		Prompt:      refinementPrompt,
		Tier:        TierStrong,
		MaxTokens:   maxTokens,
		Temperature: 0.5, // Lower temperature for refinement
		TargetLang:  targetLang,
	})
	if err != nil {
		return nil, "", err
	}

	return resp.Candidate, resp.RawText, nil
}

// buildRefinementPrompt creates a prompt that asks the LLM to refine existing code.
func buildRefinementPrompt(originalPrompt string, candidate *Candidate) string {
	return originalPrompt + `

---

## Previous Candidate (needs improvement)

Here is a candidate implementation that needs refinement:

` + "```" + candidate.Language + `
` + candidate.Code + `
` + "```" + `

Please improve this code by:
1. Fixing any bugs or edge cases
2. Improving tactical decision-making
3. Adding any missing functionality
4. Ensuring complete HTTP server implementation

Return only the improved code in a fenced code block.`
}

// selectBestCandidate picks the best candidate using heuristics.
// Currently uses code length as the primary metric.
func selectBestCandidate(candidates []Candidate) int {
	if len(candidates) == 0 {
		return -1
	}

	bestIdx := 0
	bestScore := scoreCandidate(candidates[0])

	for i := 1; i < len(candidates); i++ {
		score := scoreCandidate(candidates[i])
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return bestIdx
}

// scoreCandidate assigns a quality score to a candidate.
// Higher scores are better.
func scoreCandidate(c Candidate) float64 {
	score := float64(len(c.Code))

	// Bonus for having common code structures
	switch c.Language {
	case "go":
		if containsAll(c.Code, "func main(", "http.HandleFunc", "ListenAndServe") {
			score *= 1.5
		}
		if contains(c.Code, "GetMoves") {
			score *= 1.2
		}
	case "python":
		if containsAll(c.Code, "def ", "Flask", "app.run") || containsAll(c.Code, "def ", "HTTPServer") {
			score *= 1.5
		}
	case "rust":
		if containsAll(c.Code, "fn main()", "HttpServer", "bind") {
			score *= 1.5
		}
	case "typescript", "javascript":
		if containsAll(c.Code, "function", "createServer", "listen") {
			score *= 1.5
		}
	case "java":
		if containsAll(c.Code, "public static void main", "HttpServer") {
			score *= 1.5
		}
	case "php":
		if contains(c.Code, "$_POST") || contains(c.Code, "json_decode") {
			score *= 1.3
		}
	}

	return score
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// containsAll checks if s contains all substrings.
func containsAll(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if !contains(s, substr) {
			return false
		}
	}
	return true
}

// ErrNoValidCandidates is returned when ensemble generation produces no valid candidates.
var ErrNoValidCandidates = &NoValidCandidatesError{}

// NoValidCandidatesError indicates that no valid code candidates were generated.
type NoValidCandidatesError struct{}

func (e *NoValidCandidatesError) Error() string {
	return "no valid code candidates were generated"
}
