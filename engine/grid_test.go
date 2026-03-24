package engine

import (
	"math/rand"
	"testing"
)

func TestGridWrap(t *testing.T) {
	g := NewGrid(60, 60)

	tests := []struct {
		row, col int
		want     Position
	}{
		{0, 0, Position{0, 0}},
		{59, 59, Position{59, 59}},
		{60, 0, Position{0, 0}},   // wrap row
		{0, 60, Position{0, 0}},   // wrap col
		{-1, 0, Position{59, 0}},  // negative wrap row
		{0, -1, Position{0, 59}},  // negative wrap col
		{65, 65, Position{5, 5}},  // both wrap
		{-5, -5, Position{55, 55}}, // both negative wrap
	}

	for _, tt := range tests {
		got := g.Wrap(tt.row, tt.col)
		if got != tt.want {
			t.Errorf("Wrap(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.want)
		}
	}
}

func TestGridDistance2(t *testing.T) {
	g := NewGrid(60, 60)

	tests := []struct {
		a, b Position
		want int
	}{
		// Direct distances
		{Position{0, 0}, Position{0, 0}, 0},
		{Position{0, 0}, Position{0, 3}, 9},
		{Position{0, 0}, Position{3, 4}, 25}, // 3-4-5 triangle
		{Position{10, 10}, Position{13, 14}, 25},

		// Toroidal wrapping - shorter path across boundary
		{Position{0, 0}, Position{59, 0}, 1},   // distance 1 via wrap
		{Position{0, 0}, Position{58, 0}, 4},   // distance 2 via wrap
		{Position{0, 0}, Position{0, 59}, 1},   // distance 1 via wrap col
		{Position{0, 0}, Position{59, 59}, 2},  // distance sqrt(2) via corner wrap
	}

	for _, tt := range tests {
		got := g.Distance2(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("Distance2(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestGridInRadius(t *testing.T) {
	g := NewGrid(60, 60)

	// Test vision radius 49 (default ~7 tiles)
	center := Position{30, 30}

	// Should be visible
	visible := []Position{
		{30, 30}, // self
		{30, 31}, // adjacent
		{30, 37}, // 7 tiles away (dist^2 = 49)
		{37, 30}, // 7 tiles away (dist^2 = 49)
		{33, 33}, // diagonal (dist^2 = 18)
		{34, 34}, // diagonal (dist^2 = 32)
		{35, 34}, // dist^2 = 25 + 16 = 41
	}
	for _, p := range visible {
		if !g.InRadius(center, p, 49) {
			t.Errorf("Position %v should be within radius 49 of %v", p, center)
		}
	}

	// Should not be visible
	notVisible := []Position{
		{30, 38}, // 8 tiles away, dist^2 = 64 > 49
		{38, 38}, // diagonal 8*2 = 128 > 49
	}
	for _, p := range notVisible {
		if g.InRadius(center, p, 49) {
			t.Errorf("Position %v should NOT be within radius 49 of %v", p, center)
		}
	}
}

func TestGridAttackRadius(t *testing.T) {
	g := NewGrid(60, 60)

	// Attack radius 5 (default ~2.24 tiles)
	center := Position{30, 30}

	// Attack radius includes cardinal, diagonal neighbors, and one more ring
	// dist^2 <= 5 means: 0, 1, 2, 4, 5
	// (1,0) = 1, (1,1) = 2, (2,0) = 4, (2,1) = 5
	inAttack := []Position{
		{30, 30}, // self (dist 0)
		{30, 31}, // cardinal (dist 1)
		{31, 31}, // diagonal (dist 2)
		{32, 30}, // 2 tiles (dist 4)
		{32, 31}, // (dist 5)
	}
	for _, p := range inAttack {
		if !g.InRadius(center, p, 5) {
			t.Errorf("Position %v should be in attack radius of %v", p, center)
		}
	}

	// Outside attack radius
	outAttack := []Position{
		{33, 30}, // 3 tiles (dist 9 > 5)
		{32, 32}, // (dist 8 > 5)
	}
	for _, p := range outAttack {
		if g.InRadius(center, p, 5) {
			t.Errorf("Position %v should NOT be in attack radius of %v", p, center)
		}
	}
}

func TestGridWalls(t *testing.T) {
	g := NewGrid(10, 10)

	// Initially no walls
	if g.IsWall(Position{5, 5}) {
		t.Error("Position should not be a wall initially")
	}

	// Set a wall
	g.Set(5, 5, TileWall)
	if !g.IsWall(Position{5, 5}) {
		t.Error("Position should be a wall after setting")
	}
	if g.IsPassable(Position{5, 5}) {
		t.Error("Wall should not be passable")
	}

	// Remove wall
	g.Set(5, 5, TileOpen)
	if g.IsWall(Position{5, 5}) {
		t.Error("Position should not be a wall after clearing")
	}
}

func TestGridMove(t *testing.T) {
	g := NewGrid(60, 60)

	tests := []struct {
		start Position
		dir   Direction
		want  Position
	}{
		{Position{30, 30}, DirN, Position{29, 30}},
		{Position{30, 30}, DirS, Position{31, 30}},
		{Position{30, 30}, DirE, Position{30, 31}},
		{Position{30, 30}, DirW, Position{30, 29}},
		// Wrap at edges
		{Position{0, 0}, DirN, Position{59, 0}},
		{Position{0, 0}, DirW, Position{0, 59}},
		{Position{59, 59}, DirS, Position{0, 59}},
		{Position{59, 59}, DirE, Position{59, 0}},
	}

	for _, tt := range tests {
		got := g.Move(tt.start, tt.dir)
		if got != tt.want {
			t.Errorf("Move(%v, %v) = %v, want %v", tt.start, tt.dir, got, tt.want)
		}
	}
}

func TestGridVisibleFrom(t *testing.T) {
	g := NewGrid(60, 60)

	// Single bot at center
	positions := []Position{{30, 30}}
	visible := g.VisibleFrom(positions, 49)

	// Should see positions within radius
	if !visible[Position{30, 30}] {
		t.Error("Should see own position")
	}
	if !visible[Position{30, 37}] {
		t.Error("Should see position 7 tiles away (dist^2 = 49)")
	}
	if visible[Position{30, 38}] {
		t.Error("Should NOT see position 8 tiles away (dist^2 = 64 > 49)")
	}

	// Multiple bots - union of visibility
	positions = []Position{{10, 10}, {50, 50}}
	visible = g.VisibleFrom(positions, 49)

	if !visible[Position{10, 10}] {
		t.Error("Should see first bot position")
	}
	if !visible[Position{50, 50}] {
		t.Error("Should see second bot position")
	}
	if !visible[Position{17, 10}] {
		t.Error("Should see 7 tiles from first bot")
	}
	if !visible[Position{43, 50}] {
		t.Error("Should see 7 tiles from second bot (via wrap)")
	}
}

func TestGridRandomPassable(t *testing.T) {
	g := NewGrid(10, 10)

	// Add some walls
	g.Set(5, 5, TileWall)
	g.Set(5, 6, TileWall)

	rng := rand.New(rand.NewSource(42))

	// Get many random positions and verify they're all passable
	for i := 0; i < 100; i++ {
		p := g.RandomPassable(rng)
		if !g.IsPassable(p) {
			t.Errorf("RandomPassable returned impassable position %v", p)
		}
	}
}

func TestSqrtApprox(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 1},
		{4, 2},
		{9, 3},
		{16, 4},
		{25, 5},
		{49, 7},
		{50, 7}, // sqrt(50) ≈ 7.07
		{100, 10},
	}

	for _, tt := range tests {
		got := sqrtApprox(tt.n)
		if got != tt.want {
			t.Errorf("sqrtApprox(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}
