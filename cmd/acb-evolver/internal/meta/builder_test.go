package meta

import (
	"testing"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

func TestBuildSimple_Basic(t *testing.T) {
	topBots := []BotInfo{
		{Name: "TopBot1", Rating: 1600, Island: evolverdb.IslandAlpha, Evolved: true},
		{Name: "TopBot2", Rating: 1550, Island: evolverdb.IslandBeta, Evolved: true},
		{Name: "TopBot3", Rating: 1500, Island: evolverdb.IslandAlpha, Evolved: true},
	}

	islandStats := map[string]IslandStats{
		evolverdb.IslandAlpha: {Count: 10, AvgFitness: 0.5, TopFitness: 0.9, Diversity: 0.7},
		evolverdb.IslandBeta:  {Count: 8, AvgFitness: 0.4, TopFitness: 0.8, Diversity: 0.6},
	}

	got := BuildSimple(20, topBots, islandStats)

	if got.TotalBots != 20 {
		t.Errorf("expected TotalBots 20, got %d", got.TotalBots)
	}

	if len(got.TopBots) != 3 {
		t.Errorf("expected 3 TopBots, got %d", len(got.TopBots))
	}

	if got.TopBots[0].Name != "TopBot1" {
		t.Errorf("expected TopBot1 as first, got %s", got.TopBots[0].Name)
	}

	if got.DominantStrategy == "" {
		t.Error("expected non-empty DominantStrategy")
	}

	if len(got.IslandStats) != 2 {
		t.Errorf("expected 2 IslandStats, got %d", len(got.IslandStats))
	}
}

func TestBuildSimple_EmptyBots(t *testing.T) {
	got := BuildSimple(0, nil, nil)

	if got.TotalBots != 0 {
		t.Errorf("expected TotalBots 0, got %d", got.TotalBots)
	}

	if len(got.TopBots) != 0 {
		t.Errorf("expected 0 TopBots, got %d", len(got.TopBots))
	}

	if got.DominantStrategy != "unknown (no promoted bots)" {
		t.Errorf("expected unknown meta message, got %q", got.DominantStrategy)
	}
}

func TestBuildSimple_DominantStrategy_Alpha(t *testing.T) {
	// Alpha (aggressive) has most top bots
	topBots := []BotInfo{
		{Name: "A1", Rating: 1600, Island: evolverdb.IslandAlpha, Evolved: true},
		{Name: "A2", Rating: 1550, Island: evolverdb.IslandAlpha, Evolved: true},
		{Name: "B1", Rating: 1500, Island: evolverdb.IslandBeta, Evolved: true},
	}

	got := BuildSimple(10, topBots, nil)

	if got.DominantStrategy != "aggressive core-rushing" {
		t.Errorf("expected 'aggressive core-rushing', got %q", got.DominantStrategy)
	}
}

func TestBuildSimple_DominantStrategy_Beta(t *testing.T) {
	// Beta (economic) has most top bots
	topBots := []BotInfo{
		{Name: "B1", Rating: 1600, Island: evolverdb.IslandBeta, Evolved: true},
		{Name: "B2", Rating: 1550, Island: evolverdb.IslandBeta, Evolved: true},
		{Name: "A1", Rating: 1500, Island: evolverdb.IslandAlpha, Evolved: true},
	}

	got := BuildSimple(10, topBots, nil)

	if got.DominantStrategy != "energy-focused economy" {
		t.Errorf("expected 'energy-focused economy', got %q", got.DominantStrategy)
	}
}

func TestBuildSimple_DominantStrategy_Gamma(t *testing.T) {
	topBots := []BotInfo{
		{Name: "G1", Rating: 1600, Island: evolverdb.IslandGamma, Evolved: true},
		{Name: "G2", Rating: 1550, Island: evolverdb.IslandGamma, Evolved: true},
	}

	got := BuildSimple(10, topBots, nil)

	if got.DominantStrategy != "defensive adaptation" {
		t.Errorf("expected 'defensive adaptation', got %q", got.DominantStrategy)
	}
}

func TestBuildSimple_DominantStrategy_Delta(t *testing.T) {
	topBots := []BotInfo{
		{Name: "D1", Rating: 1600, Island: evolverdb.IslandDelta, Evolved: true},
		{Name: "D2", Rating: 1550, Island: evolverdb.IslandDelta, Evolved: true},
	}

	got := BuildSimple(10, topBots, nil)

	if got.DominantStrategy != "experimental mixed" {
		t.Errorf("expected 'experimental mixed', got %q", got.DominantStrategy)
	}
}

func TestCalculateDiversity_SingleProgram(t *testing.T) {
	programs := []*evolverdb.Program{
		{ID: 1, BehaviorVector: []float64{0.5, 0.5}},
	}

	got := calculateDiversity(programs)

	if got != 0 {
		t.Errorf("expected diversity 0 for single program, got %f", got)
	}
}

func TestCalculateDiversity_IdenticalPrograms(t *testing.T) {
	programs := []*evolverdb.Program{
		{ID: 1, BehaviorVector: []float64{0.5, 0.5}},
		{ID: 2, BehaviorVector: []float64{0.5, 0.5}},
		{ID: 3, BehaviorVector: []float64{0.5, 0.5}},
	}

	got := calculateDiversity(programs)

	if got != 0 {
		t.Errorf("expected diversity 0 for identical programs, got %f", got)
	}
}

func TestCalculateDiversity_DiversePrograms(t *testing.T) {
	programs := []*evolverdb.Program{
		{ID: 1, BehaviorVector: []float64{0.0, 0.0, 0.0, 0.0}},
		{ID: 2, BehaviorVector: []float64{1.0, 1.0, 1.0, 1.0}},
	}

	got := calculateDiversity(programs)

	// Distance between (0,0,0,0) and (1,1,1,1) is sqrt(4) = 2.0
	// Normalized by 2.0 (max distance in 4D unit hypercube is sqrt(4) = 2.0)
	// Expected: 2.0 / 2.0 = 1.0
	if got < 0.9 || got > 1.1 {
		t.Errorf("expected diversity close to 1.0 for maximally diverse programs, got %f", got)
	}
}

func TestCalculateDiversity_EmptyPrograms(t *testing.T) {
	got := calculateDiversity(nil)

	if got != 0 {
		t.Errorf("expected diversity 0 for nil programs, got %f", got)
	}
}

func TestCalculateDiversity_NoBehaviorVector(t *testing.T) {
	programs := []*evolverdb.Program{
		{ID: 1, BehaviorVector: nil},
		{ID: 2, BehaviorVector: []float64{}},
	}

	got := calculateDiversity(programs)

	if got != 0 {
		t.Errorf("expected diversity 0 for programs without behavior vectors, got %f", got)
	}
}

func TestBehaviorDistance(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float64
		expected float64
	}{
		{"same point", []float64{0.5, 0.5}, []float64{0.5, 0.5}, 0},
		{"unit apart x", []float64{0.0, 0.0}, []float64{1.0, 0.0}, 1},
		{"unit apart y", []float64{0.0, 0.0}, []float64{0.0, 1.0}, 1},
		{"diagonal", []float64{0.0, 0.0}, []float64{1.0, 1.0}, 1.414214},
		{"nil vector a", nil, []float64{0.5, 0.5}, 0},
		{"nil vector b", []float64{0.5, 0.5}, nil, 0},
		{"short vector a", []float64{0.5}, []float64{0.5, 0.5}, 0},
		{"short vector b", []float64{0.5, 0.5}, []float64{0.5}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := behaviorDistance(tc.a, tc.b)
			// Use approximate comparison for floating point
			const epsilon = 0.0001
			diff := got - tc.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > epsilon {
				t.Errorf("expected distance %f, got %f", tc.expected, got)
			}
		})
	}
}

func TestIslandStats_Values(t *testing.T) {
	islandStats := map[string]IslandStats{
		evolverdb.IslandAlpha: {Count: 5, AvgFitness: 0.75, TopFitness: 0.95, Diversity: 0.8},
	}

	got := BuildSimple(5, nil, islandStats)

	alphaStats, ok := got.IslandStats[evolverdb.IslandAlpha]
	if !ok {
		t.Fatal("expected alpha island stats")
	}

	if alphaStats.Count != 5 {
		t.Errorf("expected Count 5, got %d", alphaStats.Count)
	}

	if alphaStats.AvgFitness != 0.75 {
		t.Errorf("expected AvgFitness 0.75, got %f", alphaStats.AvgFitness)
	}

	if alphaStats.TopFitness != 0.95 {
		t.Errorf("expected TopFitness 0.95, got %f", alphaStats.TopFitness)
	}

	if alphaStats.Diversity != 0.8 {
		t.Errorf("expected Diversity 0.8, got %f", alphaStats.Diversity)
	}
}
