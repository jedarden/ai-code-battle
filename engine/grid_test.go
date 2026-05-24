package engine

import (
	"fmt"
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
		{60, 0, Position{0, 0}},    // wrap row
		{0, 60, Position{0, 0}},    // wrap col
		{-1, 0, Position{59, 0}},   // negative wrap row
		{0, -1, Position{0, 59}},   // negative wrap col
		{65, 65, Position{5, 5}},   // both wrap
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
		{Position{0, 0}, Position{59, 0}, 1},  // distance 1 via wrap
		{Position{0, 0}, Position{58, 0}, 4},  // distance 2 via wrap
		{Position{0, 0}, Position{0, 59}, 1},  // distance 1 via wrap col
		{Position{0, 0}, Position{59, 59}, 2}, // distance sqrt(2) via corner wrap
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

// TestINV6_ToroidalBounds is a property-based fuzz test that verifies
// INV-6: No bot, energy, core, or wall position ever has row < 0,
// row >= rows, col < 0, or col >= cols after any movement operation.
// All movement must use (pos + delta + size) % size.
//
// This test runs thousands of random scenarios with various grid sizes
// and operations to enforce the bound invariant.
func TestINV6_ToroidalBounds(t *testing.T) {
	// Test various grid dimensions
	testCases := []struct {
		rows int
		cols int
	}{
		{30, 30},   // Minimum
		{40, 60},   // Rectangular
		{60, 60},   // Standard square
		{100, 100}, // Large
		{120, 80},  // Large rectangular
		{200, 200}, // Maximum
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%dx%d", tc.rows, tc.cols), func(t *testing.T) {
			// Use multiple random seeds for thoroughness
			seeds := []int64{42, 123, 456, 789, 999, 1337, 2024, 0, -1, 1}

			for _, seed := range seeds {
				t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
					rng := rand.New(rand.NewSource(seed))
					testToroidalBoundsScenario(t, rng, tc.rows, tc.cols)
				})
			}
		})
	}
}

// testToroidalBoundsScenario runs a random scenario and verifies all positions stay in bounds.
func testToroidalBoundsScenario(t *testing.T, rng *rand.Rand, rows, cols int) {
	g := NewGrid(rows, cols)

	// Add random walls
	numWalls := rng.Intn(rows * cols / 20) // Up to 5% wall density
	for i := 0; i < numWalls; i++ {
		row := rng.Intn(rows*3) - rows // Can be negative or >= rows
		col := rng.Intn(cols*3) - cols
		g.Set(row, col, TileWall)
	}

	// Test that all wall positions are in bounds
	for pos := range g.Walls {
		if !posInBounds(pos, rows, cols) {
			t.Errorf("Wall position %v out of bounds for %dx%d grid", pos, rows, cols)
		}
	}

	// Test Wrap with random positions (including out of bounds)
	for i := 0; i < 1000; i++ {
		row := rng.Intn(rows*10) - rows*5 // Wide range including negative
		col := rng.Intn(cols*10) - cols*5
		wrapped := g.Wrap(row, col)

		if !posInBounds(wrapped, rows, cols) {
			t.Errorf("Wrap(%d, %d) = %v out of bounds for %dx%d grid", row, col, wrapped, rows, cols)
		}
	}

	// Test Move from random positions in all directions
	testPositions := []Position{
		{0, 0}, {0, cols - 1}, {rows - 1, 0}, {rows - 1, cols - 1}, // Corners
		{rows / 2, cols / 2},                // Center
		{0, cols / 2}, {rows - 1, cols / 2}, // Middle of edges
	}

	directions := []Direction{DirN, DirE, DirS, DirW}

	for _, pos := range testPositions {
		for _, dir := range directions {
			moved := g.Move(pos, dir)
			if !posInBounds(moved, rows, cols) {
				t.Errorf("Move(%v, %v) = %v out of bounds for %dx%d grid", pos, dir, moved, rows, cols)
			}
		}
	}

	// Test Neighbors returns positions within bounds
	center := Position{rows / 2, cols / 2}
	neighbors := g.Neighbors(center, 49) // vision radius
	for _, n := range neighbors {
		if !posInBounds(n, rows, cols) {
			t.Errorf("Neighbors(%v, 49) returned out-of-bounds position %v for %dx%d grid", center, n, rows, cols)
		}
	}

	// Test VisibleFrom returns positions within bounds
	positions := []Position{{0, 0}, {rows - 1, cols - 1}}
	visible := g.VisibleFrom(positions, 49)
	for pos := range visible {
		if !posInBounds(pos, rows, cols) {
			t.Errorf("VisibleFrom returned out-of-bounds position %v for %dx%d grid", pos, rows, cols)
		}
	}
}

// posInBounds checks if a position is within the grid bounds.
func posInBounds(pos Position, rows, cols int) bool {
	return pos.Row >= 0 && pos.Row < rows && pos.Col >= 0 && pos.Col < cols
}
