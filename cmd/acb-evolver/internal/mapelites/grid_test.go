package mapelites

import "testing"

func TestBehaviorToCell(t *testing.T) {
	// 3×3×3×3 grid per §10.2
	g := New(3)

	cases := []struct {
		agg, eco, expl, form float64
		wantX, wantY, wantZ, wantW int
	}{
		{0.0, 0.0, 0.0, 0.0, 0, 0, 0, 0},
		{1.0, 1.0, 1.0, 1.0, 2, 2, 2, 2},
		{0.5, 0.5, 0.5, 0.5, 1, 1, 1, 1},
		{0.15, 0.85, 0.33, 0.66, 0, 2, 0, 1},
		{0.99, 0.01, 0.99, 0.01, 2, 0, 2, 0},
		{0.09, 0.09, 0.09, 0.09, 0, 0, 0, 0},
		{0.34, 0.67, 0.34, 0.67, 1, 2, 1, 2},
	}

	for _, tc := range cases {
		x, y, z, w := g.BehaviorToCell(tc.agg, tc.eco, tc.expl, tc.form)
		if x != tc.wantX || y != tc.wantY || z != tc.wantZ || w != tc.wantW {
			t.Errorf("BehaviorToCell(%.2f, %.2f, %.2f, %.2f) = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
				tc.agg, tc.eco, tc.expl, tc.form, x, y, z, w, tc.wantX, tc.wantY, tc.wantZ, tc.wantW)
		}
	}
}

func TestTotalCells(t *testing.T) {
	g := New(3)
	if g.TotalCells() != 81 {
		t.Errorf("3⁴ = 81 cells, got %d", g.TotalCells())
	}
	g10 := New(10)
	if g10.TotalCells() != 10000 {
		t.Errorf("10⁴ = 10000 cells, got %d", g10.TotalCells())
	}
}

func TestTryPlace_EmptyCell(t *testing.T) {
	g := New(3)
	p, placed := g.TryPlace(1, 10.0, 0.1, 0.9, 0.5, 0.5)
	if !placed {
		t.Fatal("expected placement into empty cell")
	}
	if p.X != 0 || p.Y != 2 || p.Z != 1 || p.W != 1 {
		t.Errorf("expected cell (0,2,1,1), got (%d,%d,%d,%d)", p.X, p.Y, p.Z, p.W)
	}
	cell := g.Get(0, 2, 1, 1)
	if !cell.Occupied || cell.ProgramID != 1 || cell.Fitness != 10.0 {
		t.Errorf("unexpected cell state: %+v", cell)
	}
}

func TestTryPlace_LowerFitnessDoesNotReplace(t *testing.T) {
	g := New(3)
	g.TryPlace(1, 10.0, 0.5, 0.5, 0.5, 0.5)

	_, placed := g.TryPlace(2, 5.0, 0.5, 0.5, 0.5, 0.5)
	if placed {
		t.Fatal("lower fitness should not replace incumbent")
	}
	if g.Get(1, 1, 1, 1).ProgramID != 1 {
		t.Error("incumbent program 1 should still hold the cell")
	}
}

func TestTryPlace_HigherFitnessReplaces(t *testing.T) {
	g := New(3)
	g.TryPlace(1, 10.0, 0.5, 0.5, 0.5, 0.5)

	_, placed := g.TryPlace(2, 20.0, 0.5, 0.5, 0.5, 0.5)
	if !placed {
		t.Fatal("higher fitness should replace incumbent")
	}
	cell := g.Get(1, 1, 1, 1)
	if cell.ProgramID != 2 || cell.Fitness != 20.0 {
		t.Errorf("expected program 2 with fitness 20, got %+v", cell)
	}
}

func TestTryPlace_EqualFitnessDoesNotReplace(t *testing.T) {
	g := New(3)
	g.TryPlace(1, 10.0, 0.5, 0.5, 0.5, 0.5)
	_, placed := g.TryPlace(2, 10.0, 0.5, 0.5, 0.5, 0.5)
	if placed {
		t.Fatal("equal fitness should not replace incumbent")
	}
}

func TestOccupiedCount(t *testing.T) {
	g := New(3)
	if g.OccupiedCount() != 0 {
		t.Error("new grid should have 0 occupied cells")
	}
	g.TryPlace(1, 1.0, 0.1, 0.1, 0.1, 0.1)
	g.TryPlace(2, 1.0, 0.9, 0.9, 0.9, 0.9)
	g.TryPlace(3, 1.0, 0.5, 0.5, 0.5, 0.5)
	if g.OccupiedCount() != 3 {
		t.Errorf("expected 3 occupied cells, got %d", g.OccupiedCount())
	}
	// Same cell should not increase count
	g.TryPlace(4, 99.0, 0.5, 0.5, 0.5, 0.5)
	if g.OccupiedCount() != 3 {
		t.Errorf("expected still 3 occupied cells after same-cell update, got %d", g.OccupiedCount())
	}
}

func TestElite_EmptyGrid(t *testing.T) {
	g := New(3)
	_, found := g.Elite()
	if found {
		t.Fatal("empty grid should have no elite")
	}
}

func TestElite(t *testing.T) {
	g := New(3)
	g.TryPlace(1, 5.0, 0.1, 0.1, 0.1, 0.1)
	g.TryPlace(2, 15.0, 0.9, 0.9, 0.9, 0.9)
	g.TryPlace(3, 10.0, 0.5, 0.5, 0.5, 0.5)

	elite, found := g.Elite()
	if !found {
		t.Fatal("expected an elite in non-empty grid")
	}
	if elite.ProgramID != 2 || elite.Fitness != 15.0 {
		t.Errorf("expected elite program 2 (fitness 15), got %+v", elite)
	}
}

func TestAllElites(t *testing.T) {
	g := New(3)
	if len(g.AllElites()) != 0 {
		t.Error("empty grid should return no elites")
	}

	g.TryPlace(1, 1.0, 0.0, 0.0, 0.0, 0.0)
	g.TryPlace(2, 2.0, 0.5, 0.5, 0.5, 0.5)
	g.TryPlace(3, 3.0, 1.0, 1.0, 1.0, 1.0)

	elites := g.AllElites()
	if len(elites) != 3 {
		t.Errorf("expected 3 elites, got %d", len(elites))
	}
}

func TestSeedBehaviorVectors(t *testing.T) {
	// Verify that the 6 seed bots land in distinct grid cells on a 3×3×3×3 grid.
	g := New(3)

	bots := []struct {
		id                      int64
		name                    string
		aggression, economy     float64
		exploration, formation  float64
	}{
		{1, "gatherer", 0.1, 0.9, 0.3, 0.2},
		{2, "guardian", 0.2, 0.6, 0.1, 0.8},
		{3, "rusher", 0.9, 0.2, 0.5, 0.3},
		{4, "swarm", 0.6, 0.5, 0.4, 0.9},
		{5, "hunter", 0.7, 0.3, 0.8, 0.4},
		{6, "random", 0.3, 0.4, 0.5, 0.5},
	}

	placed := 0
	for _, b := range bots {
		_, ok := g.TryPlace(b.id, 1.0, b.aggression, b.economy, b.exploration, b.formation)
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

func TestSlice(t *testing.T) {
	g := New(3)

	// Place bots at known positions
	g.TryPlace(1, 5.0, 0.1, 0.1, 0.1, 0.1) // (0,0,0,0)
	g.TryPlace(2, 8.0, 0.9, 0.9, 0.1, 0.1) // (2,2,0,0)
	g.TryPlace(3, 3.0, 0.5, 0.1, 0.1, 0.1) // (1,0,0,0)

	// Slice: aggression×economy at z=0, w=0 (dims 2,3 fixed to 0)
	slice := g.Slice(2, 0, 3, 0)

	if len(slice) != 3 {
		t.Fatalf("expected 3 rows in slice, got %d", len(slice))
	}
	if len(slice[0]) != 3 {
		t.Fatalf("expected 3 cols in slice, got %d", len(slice[0]))
	}

	// Check (0,0) = program 1
	c00 := slice[0][0]
	if !c00.Occupied || c00.ProgramID != 1 {
		t.Errorf("slice[0][0]: expected program 1, got %+v", c00)
	}
	// Check (2,2) = program 2
	c22 := slice[2][2]
	if !c22.Occupied || c22.ProgramID != 2 {
		t.Errorf("slice[2][2]: expected program 2, got %+v", c22)
	}
	// Check (1,0) = program 3
	c10 := slice[1][0]
	if !c10.Occupied || c10.ProgramID != 3 {
		t.Errorf("slice[1][0]: expected program 3, got %+v", c10)
	}
}

func TestSnapshot(t *testing.T) {
	g := New(3)
	g.TryPlace(1, 5.0, 0.1, 0.1, 0.1, 0.1)
	g.TryPlace(2, 8.0, 0.9, 0.9, 0.9, 0.9)

	snap := g.Snapshot()
	if snap.Size != 3 {
		t.Errorf("expected size 3, got %d", snap.Size)
	}
	if len(snap.Cells) != 2 {
		t.Errorf("expected 2 cells in snapshot, got %d", len(snap.Cells))
	}
	if snap.DimNames[0] != "aggression" || snap.DimNames[3] != "formation" {
		t.Errorf("dim names: %v", snap.DimNames)
	}
}

func TestMigration_2Dto4D(t *testing.T) {
	// Simulate migrating a 2-D archive into a 4-D grid.
	// Old programs had only aggression and economy.
	// They should project into the 4-D grid at z=middle, w=middle.
	g := New(3)

	// Old 2-D program: aggression=0.5, economy=0.5
	// Migrate to 4-D: exploration=0.5 (middle), formation=0.5 (middle)
	middle := 0.5
	g.TryPlace(1, 10.0, 0.5, 0.5, middle, middle)

	cell := g.Get(1, 1, 1, 1)
	if !cell.Occupied {
		t.Error("migrated program should occupy (1,1,1,1)")
	}
	if cell.ProgramID != 1 {
		t.Errorf("expected program 1, got %d", cell.ProgramID)
	}

	// A new 4-D program with different exploration/formation should go to a different cell
	_, placed := g.TryPlace(2, 15.0, 0.5, 0.5, 0.9, 0.1)
	if !placed {
		t.Error("different 4-D coords should be a new cell")
	}
	cell2 := g.Get(1, 1, 2, 0)
	if !cell2.Occupied || cell2.ProgramID != 2 {
		t.Error("new program should be at (1,1,2,0)")
	}
}

func TestPlacementKey(t *testing.T) {
	p := Placement{X: 1, Y: 2, Z: 0, W: 2}
	key := p.Key()
	if key != [NumDims]int{1, 2, 0, 2} {
		t.Errorf("expected [1 2 0 2], got %v", key)
	}
}
