// Package crosspoll implements island cross-pollination per §10.2 of the plan.
//
// Every 50 generations, the top program from each island is copied to a random
// other island. If the target island uses a different language, the LLM
// translates the code.
package crosspoll

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/llm"
)

const generationInterval = 50

// PollinationResult records a single cross-pollination event.
type PollinationResult struct {
	SourceIsland string
	TargetIsland string
	ProgramID    int64
	Translated   bool
	SourceLang   string
	TargetLang   string
}

// programStore abstracts the database operations needed by cross-pollination.
type programStore interface {
	MaxGenerationByIsland(ctx context.Context) (map[string]int, error)
	ListTopByIsland(ctx context.Context, island string, limit int) ([]*evolverdb.Program, error)
	Create(ctx context.Context, p *evolverdb.Program) (int64, error)
}

// llmGenerator abstracts the LLM client for code translation.
type llmGenerator interface {
	Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error)
}

// Checker determines which islands need cross-pollination and executes it.
type Checker struct {
	store  programStore
	client llmGenerator
	rng    *rand.Rand
}

// NewChecker creates a Checker backed by the given store and LLM client.
func NewChecker(store *evolverdb.Store, client *llm.Client, rng *rand.Rand) *Checker {
	return &Checker{store: store, client: client, rng: rng}
}

// CheckAndPollinate checks all islands and performs cross-pollination for any
// island whose generation is a multiple of 50. Returns the list of pollination
// events that occurred.
//
// prevGens tracks the last-known max generation per island (from the previous
// check). An island triggers pollination only when it crosses a 50-generation
// boundary since the last check, preventing duplicate events.
func (c *Checker) CheckAndPollinate(ctx context.Context, prevGens map[string]int, verbose bool) ([]PollinationResult, error) {
	curGens, err := c.store.MaxGenerationByIsland(ctx)
	if err != nil {
		return nil, fmt.Errorf("query max generations: %w", err)
	}

	var results []PollinationResult

	for _, island := range evolverdb.AllIslands {
		cur := curGens[island]
		prev := prevGens[island]

		// Find the next 50-boundary after prev that cur has reached or passed.
		nextBoundary := ((prev / generationInterval) + 1) * generationInterval
		for nextBoundary <= cur && nextBoundary > 0 {
			if verbose {
				log.Printf("  Cross-pollination: island %s hit generation %d", island, nextBoundary)
			}
			result, err := c.pollinateIsland(ctx, island, verbose)
			if err != nil {
				log.Printf("  Cross-pollination error for island %s at gen %d: %v", island, nextBoundary, err)
			} else {
				results = append(results, result)
			}
			nextBoundary += generationInterval
		}

		prevGens[island] = cur
	}

	return results, nil
}

// pollinateIsland copies the top program from sourceIsland to a random other
// island, translating if the languages differ.
func (c *Checker) pollinateIsland(ctx context.Context, sourceIsland string, verbose bool) (PollinationResult, error) {
	// Get top program by fitness on the source island.
	topProgs, err := c.store.ListTopByIsland(ctx, sourceIsland, 1)
	if err != nil {
		return PollinationResult{}, fmt.Errorf("list top on %s: %w", sourceIsland, err)
	}
	if len(topProgs) == 0 {
		return PollinationResult{}, fmt.Errorf("no programs on island %s", sourceIsland)
	}
	top := topProgs[0]

	// Pick a random target island (different from source).
	targetIsland := c.pickTargetIsland(sourceIsland)

	// Determine target language from the most-recent program on the target island.
	targetLang := top.Language // default: same language
	targetProgs, err := c.store.ListTopByIsland(ctx, targetIsland, 1)
	if err != nil {
		return PollinationResult{}, fmt.Errorf("list top on target %s: %w", targetIsland, err)
	}
	if len(targetProgs) > 0 {
		targetLang = targetProgs[0].Language
	}

	translated := false
	code := top.Code

	if top.Language != targetLang {
		// Translate via LLM
		translatedCode, err := c.translate(ctx, top.Code, top.Language, targetLang)
		if err != nil {
			log.Printf("  Translation %s→%s failed: %v; copying original code", top.Language, targetLang, err)
		} else {
			code = translatedCode
			translated = true
		}
	}

	// Copy the program to the target island (same generation, new entry).
	behaviorVec := top.BehaviorVector
	if len(behaviorVec) < 2 {
		behaviorVec = []float64{0.5, 0.5}
	}

	newID, err := c.store.Create(ctx, &evolverdb.Program{
		Code:           code,
		Language:       targetLang,
		Island:         targetIsland,
		Generation:     top.Generation,
		ParentIDs:      []int64{top.ID},
		BehaviorVector: behaviorVec,
		Fitness:        top.Fitness * 0.9, // slight fitness penalty for migration
		Promoted:       false,
	})
	if err != nil {
		return PollinationResult{}, fmt.Errorf("insert migrated program: %w", err)
	}

	result := PollinationResult{
		SourceIsland: sourceIsland,
		TargetIsland: targetIsland,
		ProgramID:    newID,
		Translated:   translated,
		SourceLang:   top.Language,
		TargetLang:   targetLang,
	}

	log.Printf("  Cross-pollinated: island %s → %s, program %d → %d, lang %s→%s (translated=%v)",
		sourceIsland, targetIsland, top.ID, newID, top.Language, targetLang, translated)

	return result, nil
}

// pickTargetIsland returns a random island different from source.
func (c *Checker) pickTargetIsland(source string) string {
	others := make([]string, 0, len(evolverdb.AllIslands)-1)
	for _, island := range evolverdb.AllIslands {
		if island != source {
			others = append(others, island)
		}
	}
	return others[c.rng.Intn(len(others))]
}

// translate invokes the LLM to translate bot code from one language to another.
func (c *Checker) translate(ctx context.Context, code, fromLang, toLang string) (string, error) {
	prompt := buildTranslationPrompt(code, fromLang, toLang)

	req := llm.GenerateRequest{
		Prompt:     prompt,
		Tier:       llm.TierStrong, // use strong model for accurate translation
		TargetLang: toLang,
	}

	resp, err := c.client.Generate(ctx, req)
	if err != nil {
		return "", fmt.Errorf("llm translation: %w", err)
	}
	if resp.Candidate == nil {
		return "", fmt.Errorf("llm returned no candidate")
	}
	return resp.Candidate.Code, nil
}

func buildTranslationPrompt(code, fromLang, toLang string) string {
	return fmt.Sprintf(`You are translating a competitive bot for AI Code Battle from %s to %s.
The bot is an HTTP server that:
- Listens on port 8080
- Handles GET /health (returns 200)
- Handles POST /turn with HMAC-SHA256 request verification
- Returns JSON: {"moves": [{"row": N, "col": N, "direction": "N"|"E"|"S"|"W"}]}
- May include optional "debug" field in response

Translate the following bot preserving the EXACT same strategy and behavior.
Use idiomatic %s patterns and standard library only.
The translated bot must be a complete, self-contained HTTP server.

Source code in %s:
`+"```"+`
%s
`+"```"+`

Return ONLY the translated %s code in a single fenced code block:`, fromLang, toLang, toLang, fromLang, code, toLang)
}
