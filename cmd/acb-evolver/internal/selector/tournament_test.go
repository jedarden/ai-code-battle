package selector

import (
	"math/rand"
	"testing"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

func makePrograms(fitnesses ...float64) []*evolverdb.Program {
	out := make([]*evolverdb.Program, len(fitnesses))
	for i, f := range fitnesses {
		out[i] = &evolverdb.Program{ID: int64(i + 1), Fitness: f}
	}
	return out
}

func TestTournamentSelect_empty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	got := TournamentSelect(nil, 3, rng)
	if got != nil {
		t.Fatalf("expected nil for empty population, got %+v", got)
	}
}

func TestTournamentSelect_singleProgram(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	programs := makePrograms(5.0)
	got := TournamentSelect(programs, 3, rng)
	if got != programs[0] {
		t.Fatalf("expected sole program to be returned")
	}
}

func TestTournamentSelect_kLargerThanPopulation(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	programs := makePrograms(1.0, 3.0, 2.0)
	// k=10 > len=3, so the global best (fitness=3.0) must be returned.
	got := TournamentSelect(programs, 10, rng)
	if got.Fitness != 3.0 {
		t.Fatalf("expected global best (fitness=3.0), got %.1f", got.Fitness)
	}
}

func TestTournamentSelect_selectsBestAmongSampled(t *testing.T) {
	// Use a fixed seed so the test is deterministic.
	rng := rand.New(rand.NewSource(1))
	programs := makePrograms(1.0, 5.0, 2.0, 4.0, 3.0)

	// Run many tournaments; the highest-fitness program (5.0) should win
	// significantly more often than any other.
	wins := make(map[float64]int)
	const rounds = 200
	for i := 0; i < rounds; i++ {
		p := TournamentSelect(programs, 3, rng)
		wins[p.Fitness]++
	}
	if wins[5.0] == 0 {
		t.Fatalf("best program (fitness=5.0) never won in %d rounds", rounds)
	}
	// It should win the most.
	for f, w := range wins {
		if f != 5.0 && w >= wins[5.0] {
			t.Errorf("program with fitness=%.1f won %d times, >= best program %d times", f, w, wins[5.0])
		}
	}
}

func TestSelectParents_count(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	programs := makePrograms(1.0, 2.0, 3.0, 4.0, 5.0)
	parents := SelectParents(programs, 4, 2, rng)
	if len(parents) != 4 {
		t.Fatalf("expected 4 parents, got %d", len(parents))
	}
	for i, p := range parents {
		if p == nil {
			t.Errorf("parent[%d] is nil", i)
		}
	}
}

func TestSelectParents_nEqualsOne(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	programs := makePrograms(1.0, 2.0, 3.0)
	parents := SelectParents(programs, 1, 2, rng)
	if len(parents) != 1 {
		t.Fatalf("expected 1 parent, got %d", len(parents))
	}
}
