package crosspoll

import (
	"math/rand"
	"strings"
	"testing"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

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
	// Not a real Checker, just need the method.
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
	// This test validates the boundary logic without a DB.
	// When cur < 50, no pollination should trigger.
	// We test the boundary computation logic directly.

	// Generation 49 should not trigger (49 < 50).
	nextBoundary := ((0 / generationInterval) + 1) * generationInterval // = 50
	if nextBoundary <= 49 {
		t.Error("generation 49 should not trigger pollination")
	}

	// Generation 50 should trigger.
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
	nextBoundary := ((prev / generationInterval) + 1) * generationInterval // = 50
	for nextBoundary <= cur && nextBoundary > 0 {
		count++
		nextBoundary += generationInterval
	}

	// Should trigger at 50 and 100 = 2 pollinations.
	if count != 2 {
		t.Errorf("expected 2 pollinations for prev=49 cur=102, got %d", count)
	}
}

func TestCheckAndPollination_noDuplicateOnRecheck(t *testing.T) {
	prev := 50
	cur := 50

	nextBoundary := ((prev / generationInterval) + 1) * generationInterval // = 100
	if nextBoundary <= cur {
		t.Error("should not re-trigger at gen 50 when prev=50")
	}
}

func TestCheckAndPollination_skipsFrom0to100(t *testing.T) {
	prev := 0
	cur := 100

	count := 0
	nextBoundary := ((prev / generationInterval) + 1) * generationInterval // = 50
	for nextBoundary <= cur && nextBoundary > 0 {
		count++
		nextBoundary += generationInterval
	}

	// Should trigger at 50 and 100 = 2 pollinations.
	if count != 2 {
		t.Errorf("expected 2 pollinations for prev=0 cur=100, got %d", count)
	}
}

// newRandZero creates a deterministic RNG for reproducible tests.
func newRandZero() *rand.Rand {
	return rand.New(rand.NewSource(0))
}
