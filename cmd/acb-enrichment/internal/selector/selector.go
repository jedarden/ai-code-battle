// Package selector selects matches for enrichment based on various criteria.
package selector

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"github.com/aicodebattle/acb/cmd/acb-enrichment/internal/db"
)

// Selector chooses which matches should be enriched.
type Selector struct {
	store          db.StoreInterface
	minTurnCount   int
	minCrossings   int
	upsetThreshold float64
	maxPerHour     int
}

// Config holds selector configuration.
type Config struct {
	MinTurnCount   int
	MinCrossings   int
	UpsetThreshold float64
	MaxPerHour     int
}

// DefaultConfig returns default selector configuration.
func DefaultConfig() Config {
	return Config{
		MinTurnCount:   100,
		MinCrossings:   3,
		UpsetThreshold: 150,
		MaxPerHour:     20,
	}
}

// NewSelector creates a new match selector.
func NewSelector(store db.StoreInterface, cfg Config) *Selector {
	if cfg.MinTurnCount == 0 {
		cfg.MinTurnCount = DefaultConfig().MinTurnCount
	}
	if cfg.MinCrossings == 0 {
		cfg.MinCrossings = DefaultConfig().MinCrossings
	}
	if cfg.UpsetThreshold == 0 {
		cfg.UpsetThreshold = DefaultConfig().UpsetThreshold
	}
	if cfg.MaxPerHour == 0 {
		cfg.MaxPerHour = DefaultConfig().MaxPerHour
	}

	return &Selector{
		store:          store,
		minTurnCount:   cfg.MinTurnCount,
		minCrossings:   cfg.MinCrossings,
		upsetThreshold: cfg.UpsetThreshold,
		maxPerHour:     cfg.MaxPerHour,
	}
}

// SelectionResult holds the selected matches for enrichment.
type SelectionResult struct {
	Matches []db.CandidateMatch
	Skipped int // Skipped due to rate limit
}

// Select finds matches that qualify for enrichment, respecting rate limits.
func (s *Selector) Select(ctx context.Context) (*SelectionResult, error) {
	// Check rate limit
	since := time.Now().Add(-1 * time.Hour)
	count, err := s.store.GetEnrichmentCount(ctx, since)
	if err != nil {
		return nil, err
	}

	remaining := s.maxPerHour - count
	if remaining <= 0 {
		return &SelectionResult{Skipped: 0}, nil
	}

	// Fetch candidates
	candidates, err := s.store.FindCandidates(ctx, s.minTurnCount, s.minCrossings, s.upsetThreshold)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return &SelectionResult{}, nil
	}

	// Sort by priority (featured criteria)
	s.sortByPriority(candidates)

	// Take top matches up to rate limit
	selected := candidates
	if len(selected) > remaining {
		selected = selected[:remaining]
	}

	return &SelectionResult{
		Matches: selected,
		Skipped: max(0, len(candidates)-remaining),
	}, nil
}

// sortByPriority sorts candidates by enrichment priority.
// Higher priority: upsets, close finishes, then high win prob crossings.
func (s *Selector) sortByPriority(matches []db.CandidateMatch) {
	sort.Slice(matches, func(i, j int) bool {
		return s.priorityScore(matches[i]) > s.priorityScore(matches[j])
	})
}

// priorityScore calculates a priority score for a match.
func (s *Selector) priorityScore(m db.CandidateMatch) float64 {
	score := 0.0

	// Upsets are highest priority
	if m.IsUpset {
		score += 1000
	}

	// Close finishes are high priority
	if m.IsCloseFinish {
		score += 500
	}

	// Win prob crossings add to priority
	score += float64(m.WinProbCrossings) * 50

	// Shorter matches (more action-packed) get slight boost
	if m.TurnCount < 300 {
		score += 20
	}

	return score
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Shuffle randomly shuffles a slice of matches.
func Shuffle(matches []db.CandidateMatch) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(matches) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		matches[i], matches[j] = matches[j], matches[i]
	}
}
