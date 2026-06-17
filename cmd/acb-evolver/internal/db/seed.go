package db

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed seeds/gatherer_strategy.go.txt
var gathererCode string

//go:embed seeds/rusher_strategy.rs.txt
var rusherCode string

//go:embed seeds/swarm_strategy.ts.txt
var swarmCode string

//go:embed seeds/guardian_strategy.php.txt
var guardianCode string

//go:embed seeds/hunter_strategy.java.txt
var hunterCode string

//go:embed seeds/random_main.py.txt
var randomCode string

//go:embed seeds/phalanx_strategy.rs.txt
var phalanxCode string

//go:embed seeds/assassin_strategy.rs.txt
var assassinCode string

//go:embed seeds/opportunist_strategy.go.txt
var opportunistCode string

//go:embed seeds/defender_strategy.cs.txt
var defenderCode string

// seedProgram describes a built-in strategy bot used to bootstrap the
// programs database.
type seedProgram struct {
	name       string
	language   string
	island     string
	aggression float64 // behavior_vector[0]
	economy    float64 // behavior_vector[1]
	code       string
}

// seeds is the initial population of 9 built-in strategy bots distributed
// across all 4 islands.  Each bot is assigned a behavior vector that captures
// its play-style on the aggression × economy axes.
var seeds = []seedProgram{
	// beta island – economic strategies
	{
		name:       "gatherer",
		language:   "go",
		island:     IslandBeta,
		aggression: 0.1,
		economy:    0.9,
		code:       gathererCode,
	},
	{
		name:       "opportunist",
		language:   "go",
		island:     IslandBeta,
		aggression: 0.55,
		economy:    0.55,
		code:       opportunistCode,
	},
	{
		name:       "guardian",
		language:   "php",
		island:     IslandBeta,
		aggression: 0.2,
		economy:    0.6,
		code:       guardianCode,
	},
	// alpha island – aggressive strategies
	{
		name:       "rusher",
		language:   "rust",
		island:     IslandAlpha,
		aggression: 0.9,
		economy:    0.2,
		code:       rusherCode,
	},
	{
		name:       "phalanx",
		language:   "rust",
		island:     IslandAlpha,
		aggression: 0.85,
		economy:    0.1,
		code:       phalanxCode,
	},
	{
		name:       "assassin",
		language:   "rust",
		island:     IslandAlpha,
		aggression: 1.0,
		economy:    0.0,
		code:       assassinCode,
	},
	{
		name:       "swarm",
		language:   "typescript",
		island:     IslandAlpha,
		aggression: 0.6,
		economy:    0.5,
		code:       swarmCode,
	},
	// gamma island – adaptive / hunting strategies
	{
		name:       "hunter",
		language:   "java",
		island:     IslandGamma,
		aggression: 0.7,
		economy:    0.3,
		code:       hunterCode,
	},
	{
		name:       "defender",
		language:   "csharp",
		island:     IslandGamma,
		aggression: 0.3,
		economy:    0.4,
		code:       defenderCode,
	},
	// delta island – baseline / experimental
	{
		name:       "random",
		language:   "python",
		island:     IslandDelta,
		aggression: 0.3,
		economy:    0.4,
		code:       randomCode,
	},
}

// SeedPopulation inserts the 9 built-in strategy bots as generation-0
// programs if the programs table is empty.  It is idempotent: a second call
// is a no-op.
func SeedPopulation(ctx context.Context, s *Store) (int, error) {
	total, err := s.TotalCount(ctx)
	if err != nil {
		return 0, fmt.Errorf("check existing programs: %w", err)
	}
	if total > 0 {
		return 0, nil
	}

	inserted := 0
	for _, seed := range seeds {
		p := &Program{
			Code:           seed.code,
			Language:       seed.language,
			Island:         seed.island,
			Generation:     0,
			ParentIDs:      []int64{},
			BehaviorVector: []float64{seed.aggression, seed.economy},
			Fitness:        0.0,
			Promoted:       false,
		}
		if _, err := s.Create(ctx, p); err != nil {
			return inserted, fmt.Errorf("seed %s: %w", seed.name, err)
		}
		inserted++
	}
	return inserted, nil
}
