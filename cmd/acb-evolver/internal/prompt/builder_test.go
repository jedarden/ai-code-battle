package prompt

import (
	"strings"
	"testing"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

func TestAssemble_containsGameRules(t *testing.T) {
	r := Request{
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)
	for _, want := range []string{"toroidal", "energy", "spawn", "focus-fire"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected prompt to contain %q", want)
		}
	}
}

func TestAssemble_islandContext(t *testing.T) {
	tests := []struct {
		island  string
		keyword string
	}{
		{evolverdb.IslandAlpha, "aggressive"},
		{evolverdb.IslandBeta, "energy-focused"},
		{evolverdb.IslandGamma, "defensive"},
		{evolverdb.IslandDelta, "experimental"},
	}
	for _, tc := range tests {
		r := Request{Island: tc.island, TargetLang: "go", Generation: 2}
		got := Assemble(r)
		if !strings.Contains(got, tc.keyword) {
			t.Errorf("island %s: expected %q in prompt", tc.island, tc.keyword)
		}
	}
}

func TestAssemble_targetLanguageAppears(t *testing.T) {
	for _, lang := range []string{"go", "python", "rust", "typescript", "java", "php"} {
		r := Request{Island: evolverdb.IslandDelta, TargetLang: lang, Generation: 0}
		got := Assemble(r)
		if !strings.Contains(got, "```"+lang) {
			t.Errorf("lang %s: expected fenced block in prompt", lang)
		}
	}
}

func TestAssemble_parentCodeEmbedded(t *testing.T) {
	parents := []*evolverdb.Program{
		{
			ID:             42,
			Code:           "func main() { /* gatherer */ }",
			Language:       "go",
			Fitness:        0.75,
			BehaviorVector: []float64{0.1, 0.9},
		},
	}
	r := Request{
		Parents:    parents,
		Island:     evolverdb.IslandBeta,
		TargetLang: "go",
		Generation: 3,
	}
	got := Assemble(r)
	if !strings.Contains(got, "func main() { /* gatherer */ }") {
		t.Error("expected parent code to be embedded in the prompt")
	}
	if !strings.Contains(got, "fitness: 0.750") {
		t.Error("expected parent fitness to appear in the prompt")
	}
	if !strings.Contains(got, "aggression=0.10") {
		t.Error("expected behavior vector to appear in the prompt")
	}
}

func TestAssemble_replayAnalysis(t *testing.T) {
	replays := []MatchSummary{
		{
			MatchID:    "match-001",
			WinnerName: "rusher",
			LoserName:  "gatherer",
			Condition:  "elimination",
			TurnCount:  123,
			Scores:     []int{42, 10},
			Strategies: []string{"core rush", "aggressive spawn"},
			Weaknesses: []string{"exposed energy lines", "slow response"},
			KeyMoments: []string{"Turn 50: rusher surrounded gatherer core"},
		},
	}
	r := Request{
		Replays:    replays,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "rust",
		Generation: 1,
	}
	got := Assemble(r)
	if !strings.Contains(got, "match-001") {
		t.Error("expected match ID in prompt")
	}
	if !strings.Contains(got, "rusher defeated gatherer") {
		t.Error("expected match result in prompt")
	}
	if !strings.Contains(got, "core rush") {
		t.Error("expected strategies in prompt")
	}
	if !strings.Contains(got, "Turn 50: rusher surrounded gatherer core") {
		t.Error("expected key moment in prompt")
	}
}

func TestAssemble_metaDescription(t *testing.T) {
	meta := MetaDescription{
		TotalBots:        12,
		DominantStrategy: "energy-focused economy",
		TopBots: []BotSummary{
			{Name: "gatherer", Rating: 1600, Island: "beta", Evolved: false},
			{Name: "evo-001", Rating: 1550, Island: "alpha", Evolved: true},
		},
		IslandStats: map[string]IslandStat{
			"alpha": {Count: 3, AvgFitness: 0.5, TopFitness: 0.9},
		},
	}
	r := Request{
		Meta:       meta,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 5,
	}
	got := Assemble(r)
	if !strings.Contains(got, "12") {
		t.Error("expected total bot count in prompt")
	}
	if !strings.Contains(got, "energy-focused economy") {
		t.Error("expected dominant strategy in prompt")
	}
	if !strings.Contains(got, "gatherer") {
		t.Error("expected top bot name in prompt")
	}
	if !strings.Contains(got, "evolved") {
		t.Error("expected evolved flag for evo-001 in prompt")
	}
}

func TestAssemble_emptyMeta_noMetaSection(t *testing.T) {
	r := Request{
		Island:     evolverdb.IslandDelta,
		TargetLang: "python",
		Generation: 0,
	}
	got := Assemble(r)
	// Meta section heading should not appear when meta is empty.
	if strings.Contains(got, "## Current Meta") {
		t.Error("expected no meta section when meta is empty")
	}
}

func TestAssemble_generationAppearsInIslandContext(t *testing.T) {
	r := Request{
		Island:     evolverdb.IslandGamma,
		TargetLang: "java",
		Generation: 7,
	}
	got := Assemble(r)
	if !strings.Contains(got, "generation 7") {
		t.Error("expected generation number in island context")
	}
}

func TestAssemble_emptyParents_noParentSection(t *testing.T) {
	r := Request{
		Parents:    nil,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)
	if strings.Contains(got, "## Parent Programs") {
		t.Error("expected no parent section when parents is nil")
	}
}

func TestAssemble_emptyReplays_noReplaySection(t *testing.T) {
	r := Request{
		Replays:    nil,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)
	if strings.Contains(got, "## Recent Match Analysis") {
		t.Error("expected no replay section when replays is nil")
	}
}

func TestAssemble_multipleReplays(t *testing.T) {
	replays := []MatchSummary{
		{MatchID: "m1", WinnerName: "w1", Condition: "elimination", TurnCount: 100},
		{MatchID: "m2", WinnerName: "w2", Condition: "dominance", TurnCount: 200},
		{MatchID: "m3", WinnerName: "w3", Condition: "turns", TurnCount: 500},
	}
	r := Request{
		Replays:    replays,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)

	for _, id := range []string{"m1", "m2", "m3"} {
		if !strings.Contains(got, id) {
			t.Errorf("expected match ID %s in prompt", id)
		}
	}
}

func TestAssemble_drawResult(t *testing.T) {
	replays := []MatchSummary{
		{MatchID: "draw-match", Condition: "draw", TurnCount: 500},
	}
	r := Request{
		Replays:    replays,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)
	if !strings.Contains(got, "Draw") {
		t.Error("expected Draw in prompt for draw condition")
	}
}

func TestAssemble_allIslandsHaveContext(t *testing.T) {
	for _, island := range evolverdb.AllIslands {
		r := Request{
			Island:     island,
			TargetLang: "go",
			Generation: 1,
		}
		got := Assemble(r)
		if !strings.Contains(got, island) {
			t.Errorf("expected island %s in prompt", island)
		}
	}
}

func TestAssemble_behaviorVectorDisplay(t *testing.T) {
	parents := []*evolverdb.Program{
		{
			ID:             1,
			Code:           "code",
			Language:       "go",
			Fitness:        0.5,
			BehaviorVector: []float64{0.25, 0.75},
		},
	}
	r := Request{
		Parents:    parents,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)
	if !strings.Contains(got, "aggression=0.25") {
		t.Error("expected aggression value in prompt")
	}
	if !strings.Contains(got, "economy=0.75") {
		t.Error("expected economy value in prompt")
	}
}

func TestAssemble_parentWithoutBehaviorVector(t *testing.T) {
	parents := []*evolverdb.Program{
		{
			ID:             1,
			Code:           "code",
			Language:       "go",
			Fitness:        0.5,
			BehaviorVector: nil, // No behavior vector
		},
	}
	r := Request{
		Parents:    parents,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)
	// Should still include the parent, just without behavior info
	if !strings.Contains(got, "code") {
		t.Error("expected parent code in prompt even without behavior vector")
	}
}

func TestAssemble_codeBlockLanguage(t *testing.T) {
	parents := []*evolverdb.Program{
		{Code: "code", Language: "python", Fitness: 0.5},
	}
	r := Request{
		Parents:    parents,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "python",
		Generation: 1,
	}
	got := Assemble(r)
	if !strings.Contains(got, "```python") {
		t.Error("expected python code block in prompt")
	}
}

func TestAssemble_scoresDisplay(t *testing.T) {
	replays := []MatchSummary{
		{
			MatchID:   "m1",
			Scores:    []int{100, 50, 25},
			Condition: "turns",
			TurnCount: 100,
		},
	}
	r := Request{
		Replays:    replays,
		Island:     evolverdb.IslandAlpha,
		TargetLang: "go",
		Generation: 1,
	}
	got := Assemble(r)
	if !strings.Contains(got, "[100 50 25]") {
		t.Error("expected scores to be displayed")
	}
}

func TestLangDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"go", "Go"},
		{"python", "Python"},
		{"rust", "Rust"},
		{"typescript", "TypeScript"},
		{"java", "Java"},
		{"php", "PHP"},
		{"unknown", "unknown"},
	}

	for _, tc := range tests {
		got := langDisplayName(tc.input)
		if got != tc.expected {
			t.Errorf("langDisplayName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
