// Package selector implements parent sampling strategies for the evolution
// pipeline.  The primary strategy is tournament selection, which balances
// selection pressure (favoring fit individuals) with diversity (random
// sampling prevents premature convergence).
package selector

import (
	"math/rand"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

// TournamentSelect picks the highest-fitness program from k randomly sampled
// candidates drawn without replacement from programs.
//
// When k ≥ len(programs) the function returns the globally best program.
// Returns nil when programs is empty.
func TournamentSelect(programs []*evolverdb.Program, k int, rng *rand.Rand) *evolverdb.Program {
	if len(programs) == 0 {
		return nil
	}

	// If k covers the whole population, just return the global best.
	if k >= len(programs) {
		best := programs[0]
		for _, p := range programs[1:] {
			if p.Fitness > best.Fitness {
				best = p
			}
		}
		return best
	}

	// Sample k distinct indices using a Fisher-Yates partial shuffle.
	indices := rng.Perm(len(programs))[:k]

	best := programs[indices[0]]
	for _, idx := range indices[1:] {
		if programs[idx].Fitness > best.Fitness {
			best = programs[idx]
		}
	}
	return best
}

// SelectParents draws n parents via tournament selection with tournament size k.
// The same program may be selected more than once (sampling with replacement
// across tournaments).
func SelectParents(programs []*evolverdb.Program, n, k int, rng *rand.Rand) []*evolverdb.Program {
	parents := make([]*evolverdb.Program, n)
	for i := range parents {
		parents[i] = TournamentSelect(programs, k, rng)
	}
	return parents
}
