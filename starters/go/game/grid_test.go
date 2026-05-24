package game

import (
	"testing"
)

func TestToroidalManhattan(t *testing.T) {
	tests := []struct {
		name       string
		a, b       Position
		rows, cols int
		want       int
	}{
		{
			name: "adjacent",
			a:    Position{Row: 0, Col: 0},
			b:    Position{Row: 0, Col: 1},
			rows: 10, cols: 10,
			want: 1,
		},
		{
			name: "wrapping row",
			a:    Position{Row: 0, Col: 0},
			b:    Position{Row: 9, Col: 0},
			rows: 10, cols: 10,
			want: 1,
		},
		{
			name: "wrapping col",
			a:    Position{Row: 0, Col: 0},
			b:    Position{Row: 0, Col: 9},
			rows: 10, cols: 10,
			want: 1,
		},
		{
			name: "diagonal",
			a:    Position{Row: 0, Col: 0},
			b:    Position{Row: 1, Col: 1},
			rows: 10, cols: 10,
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToroidalManhattan(tt.a, tt.b, tt.rows, tt.cols); got != tt.want {
				t.Errorf("ToroidalManhattan() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToroidalDistance2(t *testing.T) {
	tests := []struct {
		name       string
		a, b       Position
		rows, cols int
		want       int
	}{
		{
			name: "adjacent",
			a:    Position{Row: 0, Col: 0},
			b:    Position{Row: 0, Col: 1},
			rows: 10, cols: 10,
			want: 1,
		},
		{
			name: "diagonal",
			a:    Position{Row: 0, Col: 0},
			b:    Position{Row: 1, Col: 1},
			rows: 10, cols: 10,
			want: 2,
		},
		{
			name: "distance 5",
			a:    Position{Row: 0, Col: 0},
			b:    Position{Row: 3, Col: 4},
			rows: 10, cols: 10,
			want: 25, // 3^2 + 4^2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToroidalDistance2(tt.a, tt.b, tt.rows, tt.cols); got != tt.want {
				t.Errorf("ToroidalDistance2() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeighbors(t *testing.T) {
	p := Position{Row: 0, Col: 0}
	rows, cols := 5, 5

	neighbors := Neighbors(p, rows, cols)

	if len(neighbors) != 4 {
		t.Fatalf("Neighbors() returned %d positions, want 4", len(neighbors))
	}

	// Check that we got all 4 cardinal directions
	expected := []Position{
		{Row: 4, Col: 0}, // N (wraps)
		{Row: 0, Col: 1}, // E
		{Row: 1, Col: 0}, // S
		{Row: 0, Col: 4}, // W (wraps)
	}

	for i, exp := range expected {
		if neighbors[i] != exp {
			t.Errorf("Neighbors()[%d] = %v, want %v", i, neighbors[i], exp)
		}
	}
}

func TestNeighborInDirection(t *testing.T) {
	tests := []struct {
		name       string
		pos        Position
		dir        Direction
		rows, cols int
		want       Position
	}{
		{
			name: "north",
			pos:  Position{Row: 5, Col: 5},
			dir:  DirN,
			rows: 10, cols: 10,
			want: Position{Row: 4, Col: 5},
		},
		{
			name: "north wrapping",
			pos:  Position{Row: 0, Col: 5},
			dir:  DirN,
			rows: 10, cols: 10,
			want: Position{Row: 9, Col: 5},
		},
		{
			name: "east",
			pos:  Position{Row: 5, Col: 5},
			dir:  DirE,
			rows: 10, cols: 10,
			want: Position{Row: 5, Col: 6},
		},
		{
			name: "east wrapping",
			pos:  Position{Row: 5, Col: 9},
			dir:  DirE,
			rows: 10, cols: 10,
			want: Position{Row: 5, Col: 0},
		},
		{
			name: "south",
			pos:  Position{Row: 5, Col: 5},
			dir:  DirS,
			rows: 10, cols: 10,
			want: Position{Row: 6, Col: 5},
		},
		{
			name: "west",
			pos:  Position{Row: 5, Col: 5},
			dir:  DirW,
			rows: 10, cols: 10,
			want: Position{Row: 5, Col: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeighborInDirection(tt.pos, tt.dir, tt.rows, tt.cols); got != tt.want {
				t.Errorf("NeighborInDirection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllNeighbors(t *testing.T) {
	p := Position{Row: 1, Col: 1}
	rows, cols := 5, 5

	neighbors := AllNeighbors(p, rows, cols)

	if len(neighbors) != 8 {
		t.Fatalf("AllNeighbors() returned %d positions, want 8", len(neighbors))
	}
}

func TestBFSDirection(t *testing.T) {
	rows, cols := 10, 10

	// All positions are passable
	passable := func(p Position) bool {
		return true
	}

	tests := []struct {
		name  string
		start Position
		goal  Position
		want  Direction
	}{
		{
			name:  "adjacent north",
			start: Position{Row: 5, Col: 5},
			goal:  Position{Row: 4, Col: 5},
			want:  DirN,
		},
		{
			name:  "adjacent east",
			start: Position{Row: 5, Col: 5},
			goal:  Position{Row: 5, Col: 6},
			want:  DirE,
		},
		{
			name:  "path north",
			start: Position{Row: 5, Col: 5},
			goal:  Position{Row: 2, Col: 5},
			want:  DirN,
		},
		{
			name:  "path with wrapping",
			start: Position{Row: 0, Col: 0},
			goal:  Position{Row: 9, Col: 0},
			want:  DirN, // North wraps to 9
		},
		{
			name:  "same position",
			start: Position{Row: 5, Col: 5},
			goal:  Position{Row: 5, Col: 5},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BFSDirection(tt.start, tt.goal, passable, rows, cols); got != tt.want {
				t.Errorf("BFSDirection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBFSDirectionWithWalls(t *testing.T) {
	rows, cols := 10, 10

	// Create a wall at row 5, cols 3-7
	walls := make(map[Position]bool)
	for c := 3; c <= 7; c++ {
		walls[Position{Row: 5, Col: c}] = true
	}

	passable := func(p Position) bool {
		return !walls[p]
	}

	// Start north of wall, goal south of wall - should go around
	start := Position{Row: 4, Col: 5}
	goal := Position{Row: 6, Col: 5}

	dir := BFSDirection(start, goal, passable, rows, cols)

	// Should not return empty (there is a path around the wall)
	if dir == "" {
		t.Error("BFSDirection() returned empty, expected a path around the wall")
	}

	// Should not try to go south into the wall
	if dir == DirS {
		t.Error("BFSDirection() returned South, which goes into the wall")
	}
}

func TestAllDirections(t *testing.T) {
	dirs := AllDirections()

	if len(dirs) != 4 {
		t.Fatalf("AllDirections() returned %d directions, want 4", len(dirs))
	}

	expected := []Direction{DirN, DirE, DirS, DirW}
	for i, want := range expected {
		if dirs[i] != want {
			t.Errorf("AllDirections()[%d] = %v, want %v", i, dirs[i], want)
		}
	}
}
