package mapelites

import "testing"

func TestBehaviorToCell(t *testing.T) {
	g := New(10)

	cases := []struct {
		agg, eco     float64
		wantX, wantY int
	}{
		{0.0, 0.0, 0, 0},
		{1.0, 1.0, 9, 9},
		{0.5, 0.5, 5, 5},
		{0.15, 0.85, 1, 8},
		{0.99, 0.01, 9, 0},
		{0.09, 0.09, 0, 0},
		{0.1, 0.9, 1, 9},
	}

	for _, tc := range cases {
		x, y := g.BehaviorToCell(tc.agg, tc.eco)
		if x != tc.wantX || y != tc.wantY {
			t.Errorf("BehaviorToCell(%.2f, %.2f) = (%d, %d), want (%d, %d)",
				tc.agg, tc.eco, x, y, tc.wantX, tc.wantY)
		}
	}
}

func TestTryPlace_EmptyCell(t *testing.T) {
	g := New(10)
	p, placed := g.TryPlace(1, 10.0, 0.1, 0.9)
	if !placed {
		t.Fatal("expected placement into empty cell")
	}
	if p.X != 1 || p.Y != 9 {
		t.Errorf("expected cell (1,9), got (%d,%d)", p.X, p.Y)
	}
	cell := g.Get(1, 9)
	if !cell.Occupied || cell.ProgramID != 1 || cell.Fitness != 10.0 {
		t.Errorf("unexpected cell state: %+v", cell)
	}
}

func TestTryPlace_LowerFitnessDoesNotReplace(t *testing.T) {
	g := New(10)
	g.TryPlace(1, 10.0, 0.5, 0.5)

	_, placed := g.TryPlace(2, 5.0, 0.5, 0.5)
	if placed {
		t.Fatal("lower fitness should not replace incumbent")
	}
	if g.Get(5, 5).ProgramID != 1 {
		t.Error("incumbent program 1 should still hold the cell")
	}
}

func TestTryPlace_HigherFitnessReplaces(t *testing.T) {
	g := New(10)
	g.TryPlace(1, 10.0, 0.5, 0.5)

	_, placed := g.TryPlace(2, 20.0, 0.5, 0.5)
	if !placed {
		t.Fatal("higher fitness should replace incumbent")
	}
	cell := g.Get(5, 5)
	if cell.ProgramID != 2 || cell.Fitness != 20.0 {
		t.Errorf("expected program 2 with fitness 20, got %+v", cell)
	}
}

func TestTryPlace_EqualFitnessDoesNotReplace(t *testing.T) {
	g := New(10)
	g.TryPlace(1, 10.0, 0.5, 0.5)
	_, placed := g.TryPlace(2, 10.0, 0.5, 0.5)
	if placed {
		t.Fatal("equal fitness should not replace incumbent")
	}
}

func TestOccupiedCount(t *testing.T) {
	g := New(10)
	if g.OccupiedCount() != 0 {
		t.Error("new grid should have 0 occupied cells")
	}
	g.TryPlace(1, 1.0, 0.1, 0.1)
	g.TryPlace(2, 1.0, 0.9, 0.9)
	g.TryPlace(3, 1.0, 0.5, 0.5)
	if g.OccupiedCount() != 3 {
		t.Errorf("expected 3 occupied cells, got %d", g.OccupiedCount())
	}
	// Same cell should not increase count
	g.TryPlace(4, 99.0, 0.5, 0.5)
	if g.OccupiedCount() != 3 {
		t.Errorf("expected still 3 occupied cells after same-cell update, got %d", g.OccupiedCount())
	}
}

func TestElite_EmptyGrid(t *testing.T) {
	g := New(10)
	_, found := g.Elite()
	if found {
		t.Fatal("empty grid should have no elite")
	}
}

func TestElite(t *testing.T) {
	g := New(10)
	g.TryPlace(1, 5.0, 0.1, 0.1)
	g.TryPlace(2, 15.0, 0.9, 0.9)
	g.TryPlace(3, 10.0, 0.5, 0.5)

	elite, found := g.Elite()
	if !found {
		t.Fatal("expected an elite in non-empty grid")
	}
	if elite.ProgramID != 2 || elite.Fitness != 15.0 {
		t.Errorf("expected elite program 2 (fitness 15), got %+v", elite)
	}
}

func TestAllElites(t *testing.T) {
	g := New(10)
	if len(g.AllElites()) != 0 {
		t.Error("empty grid should return no elites")
	}

	g.TryPlace(1, 1.0, 0.0, 0.0)
	g.TryPlace(2, 2.0, 0.5, 0.5)
	g.TryPlace(3, 3.0, 1.0, 1.0)

	elites := g.AllElites()
	if len(elites) != 3 {
		t.Errorf("expected 3 elites, got %d", len(elites))
	}
}

func TestSeedBehaviorVectors(t *testing.T) {
	// Verify that the 6 seed bots land in distinct grid cells on a 10x10 grid.
	g := New(10)

	bots := []struct {
		id         int64
		name       string
		aggression float64
		economy    float64
	}{
		{1, "gatherer", 0.1, 0.9},
		{2, "guardian", 0.2, 0.6},
		{3, "rusher", 0.9, 0.2},
		{4, "swarm", 0.6, 0.5},
		{5, "hunter", 0.7, 0.3},
		{6, "random", 0.3, 0.4},
	}

	placed := 0
	for _, b := range bots {
		_, ok := g.TryPlace(b.id, 1.0, b.aggression, b.economy)
		if ok {
			placed++
		}
	}

	if placed != 6 {
		t.Errorf("expected all 6 seed bots in distinct cells, but only %d placed", placed)
	}
	if g.OccupiedCount() != 6 {
		t.Errorf("expected 6 occupied cells, got %d", g.OccupiedCount())
	}
}
