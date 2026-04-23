// Package meta builds meta-game descriptions for the evolution prompt.
//
// The meta builder aggregates data about the current competitive landscape:
//   - Leaderboard: top-rated bots and their ratings
//   - Dominant strategies: what tactics are currently winning
//   - Island population stats: fitness and diversity per island
package meta

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

// BotInfo represents a bot's competitive summary.
type BotInfo struct {
	Name    string
	Rating  float64
	Island  string
	Evolved bool
}

// IslandStats captures population metrics for a single island.
type IslandStats struct {
	Count      int
	AvgFitness float64
	TopFitness float64
	Diversity  float64 // behavioral diversity (0-1)
}

// Description holds the complete meta-game snapshot.
type Description struct {
	// TotalBots is the number of active bots on the ladder.
	TotalBots int
	// TopBots lists the highest-rated bots.
	TopBots []BotInfo
	// DominantStrategy describes the current meta.
	DominantStrategy string
	// NashMixture describes the Nash equilibrium mixture over the population
	// (per plan §10.3 PSRO). Example: "40% swarm, 30% hunter, 30% gatherer".
	NashMixture string
	// MetaWeaknesses lists exploitable gaps in the current population.
	MetaWeaknesses []string
	// IslandStats holds population metrics per island.
	IslandStats map[string]IslandStats
}

// Builder creates meta descriptions from database state.
type Builder struct {
	store *evolverdb.Store
}

// NewBuilder creates a meta builder backed by the program store.
func NewBuilder(store *evolverdb.Store) *Builder {
	return &Builder{store: store}
}

// Build constructs a meta description from current database state.
func (b *Builder) Build(ctx context.Context, topBotLimit int) (*Description, error) {
	desc := &Description{
		IslandStats: make(map[string]IslandStats),
	}

	// Gather island population stats
	for _, island := range evolverdb.AllIslands {
		programs, err := b.store.ListByIsland(ctx, island)
		if err != nil {
			return nil, err
		}

		stats := IslandStats{
			Count: len(programs),
		}

		if len(programs) > 0 {
			// Calculate average and top fitness
			var sum float64
			for _, p := range programs {
				sum += p.Fitness
			}
			stats.AvgFitness = sum / float64(len(programs))
			stats.TopFitness = programs[0].Fitness // Already sorted by fitness DESC

			// Calculate behavioral diversity using behavior vectors
			stats.Diversity = calculateDiversity(programs)
		}

		desc.IslandStats[island] = stats
	}

	// Get promoted programs to represent the live ladder
	promoted, err := b.store.ListPromoted(ctx)
	if err != nil {
		return nil, err
	}

	desc.TotalBots = len(promoted)

	// Convert to BotInfo and sort by fitness (as proxy for rating)
	topBots := make([]BotInfo, 0, len(promoted))
	for _, p := range promoted {
		topBots = append(topBots, BotInfo{
			Name:    p.BotName,
			Rating:  1500 + p.Fitness*100, // Approximate rating from fitness
			Island:  p.Island,
			Evolved: true,
		})
	}

	// Sort by rating descending
	sort.Slice(topBots, func(i, j int) bool {
		return topBots[i].Rating > topBots[j].Rating
	})

	// Limit to top N bots
	if len(topBots) > topBotLimit {
		topBots = topBots[:topBotLimit]
	}
	desc.TopBots = topBots

	// Infer dominant strategy from top performers
	desc.DominantStrategy = b.inferDominantStrategy(desc)

	// Compute Nash mixture description from island population proportions
	desc.NashMixture = b.computeNashMixture(desc)

	// Infer meta weaknesses from under-represented strategies
	desc.MetaWeaknesses = b.inferMetaWeaknesses(desc)

	return desc, nil
}

// calculateDiversity computes behavioral diversity from behavior vectors.
// Returns a value between 0 (all identical) and 1 (maximally diverse).
func calculateDiversity(programs []*evolverdb.Program) float64 {
	if len(programs) < 2 {
		return 0
	}

	// Calculate average pairwise distance
	var totalDist float64
	var pairs int

	for i := 0; i < len(programs); i++ {
		for j := i + 1; j < len(programs); j++ {
			dist := behaviorDistance(programs[i].BehaviorVector, programs[j].BehaviorVector)
			totalDist += dist
			pairs++
		}
	}

	if pairs == 0 {
		return 0
	}

	avgDist := totalDist / float64(pairs)
	// Normalize: max distance in 4D unit hypercube is sqrt(4) = 2.0
	return avgDist / 2.0
}

// behaviorDistance computes Euclidean distance between behavior vectors.
// Supports 2-D and 4-D vectors (per plan §10.2 MAP-Elites: aggression,
// economy, exploration, formation).
func behaviorDistance(a, b []float64) float64 {
	n := len(a)
	if n > len(b) {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// inferDominantStrategy analyzes the top bots and describes the meta.
func (b *Builder) inferDominantStrategy(desc *Description) string {
	if len(desc.TopBots) == 0 {
		return "unknown (no promoted bots)"
	}

	// Count strategies by island
	islandCounts := make(map[string]int)
	for _, bot := range desc.TopBots {
		islandCounts[bot.Island]++
	}

	// Find dominant island(s)
	var dominantIslands []string
	maxCount := 0
	for island, count := range islandCounts {
		if count > maxCount {
			maxCount = count
			dominantIslands = []string{island}
		} else if count == maxCount {
			dominantIslands = append(dominantIslands, island)
		}
	}

	// Map islands to strategy descriptions
	strategyMap := map[string]string{
		evolverdb.IslandAlpha: "aggressive core-rushing",
		evolverdb.IslandBeta:  "energy-focused economy",
		evolverdb.IslandGamma: "defensive adaptation",
		evolverdb.IslandDelta: "experimental mixed",
	}

	var strategies []string
	for _, island := range dominantIslands {
		if s, ok := strategyMap[island]; ok {
			strategies = append(strategies, s)
		}
	}

	if len(strategies) == 0 {
		return "diverse meta with no clear dominant strategy"
	}

	return strings.Join(strategies, " / ")
}

// computeNashMixture produces a human-readable description of the Nash equilibrium
// mixture over the island population, expressed as proportional strategy weights.
// Per plan §10.3 PSRO: "the candidate should beat this mixture, not just one opponent."
func (b *Builder) computeNashMixture(desc *Description) string {
	if len(desc.IslandStats) == 0 || desc.TotalBots == 0 {
		return ""
	}

	// Approximate Nash mixture from island population proportions.
	// Each island represents a strategy archetype (§10.2).
	strategyMap := map[string]string{
		evolverdb.IslandAlpha: "aggressive",
		evolverdb.IslandBeta:  "economy",
		evolverdb.IslandGamma: "defensive",
		evolverdb.IslandDelta: "mixed",
	}

	type mixEntry struct {
		name   string
		weight float64
	}

	var entries []mixEntry
	totalPop := 0
	for _, island := range evolverdb.AllIslands {
		stats, ok := desc.IslandStats[island]
		if !ok || stats.Count == 0 {
			continue
		}
		totalPop += stats.Count
		entries = append(entries, mixEntry{
			name:   strategyMap[island],
			weight: float64(stats.Count),
		})
	}

	if totalPop == 0 {
		return ""
	}

	// Normalize to percentages
	var parts []string
	for _, e := range entries {
		pct := int(math.Round(e.weight / float64(totalPop) * 100))
		if pct > 0 {
			parts = append(parts, fmt.Sprintf("%d%% %s", pct, e.name))
		}
	}

	return strings.Join(parts, ", ")
}

// inferMetaWeaknesses identifies exploitable gaps in the current population.
// Per plan §10.3: "Weaknesses in the current meta are highlighted."
func (b *Builder) inferMetaWeaknesses(desc *Description) []string {
	var weaknesses []string

	// Check for empty islands — under-explored strategy spaces.
	strategyMap := map[string]string{
		evolverdb.IslandAlpha: "aggression",
		evolverdb.IslandBeta:  "economy",
		evolverdb.IslandGamma: "defense",
		evolverdb.IslandDelta: "mixed/experimental",
	}

	for _, island := range evolverdb.AllIslands {
		stats, ok := desc.IslandStats[island]
		if !ok || stats.Count == 0 {
			weaknesses = append(weaknesses,
				fmt.Sprintf("No bots exploring %s strategies — wide open niche",
					strategyMap[island]))
		} else if stats.Diversity < 0.2 {
			weaknesses = append(weaknesses,
				fmt.Sprintf("Low behavioral diversity in %s island (%.2f) — susceptible to counters",
					strategyMap[island], stats.Diversity))
		}
	}

	// Check for dominant strategy monopoly.
	if desc.DominantStrategy != "" && len(desc.TopBots) >= 3 {
		islandCounts := make(map[string]int)
		for _, bot := range desc.TopBots {
			islandCounts[bot.Island]++
		}
		for island, count := range islandCounts {
			if count >= len(desc.TopBots)*2/3 {
				weaknesses = append(weaknesses,
					fmt.Sprintf("Top bots are %s-heavy (%d/%d) — counter-strategies could exploit this",
						strategyMap[island], count, len(desc.TopBots)))
			}
		}
	}

	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "Meta is relatively balanced — no obvious exploit")
	}

	return weaknesses
}

// BuildSimple creates a meta description without database access.
// This is useful for testing or when database is not available.
func BuildSimple(totalBots int, topBots []BotInfo, islandStats map[string]IslandStats) *Description {
	desc := &Description{
		TotalBots:   totalBots,
		TopBots:     topBots,
		IslandStats: islandStats,
	}

	// Infer dominant strategy
	if len(topBots) == 0 {
		desc.DominantStrategy = "unknown (no promoted bots)"
		return desc
	}

	islandCounts := make(map[string]int)
	for _, bot := range topBots {
		islandCounts[bot.Island]++
	}

	// Find most common island
	maxCount := 0
	dominantIsland := ""
	for island, count := range islandCounts {
		if count > maxCount {
			maxCount = count
			dominantIsland = island
		}
	}

	strategyMap := map[string]string{
		evolverdb.IslandAlpha: "aggressive core-rushing",
		evolverdb.IslandBeta:  "energy-focused economy",
		evolverdb.IslandGamma: "defensive adaptation",
		evolverdb.IslandDelta: "experimental mixed",
	}

	if s, ok := strategyMap[dominantIsland]; ok {
		desc.DominantStrategy = s
	} else {
		desc.DominantStrategy = "diverse meta"
	}

	return desc
}
