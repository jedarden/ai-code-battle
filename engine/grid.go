package engine

import (
	"math/rand"
)

// Grid represents the toroidal game board.
type Grid struct {
	Rows   int
	Cols   int
	Tiles  [][]Tile
	Walls  map[Position]bool // cached wall positions for fast lookup
}

// NewGrid creates a new empty grid with the given dimensions.
func NewGrid(rows, cols int) *Grid {
	tiles := make([][]Tile, rows)
	for i := range tiles {
		tiles[i] = make([]Tile, cols)
	}
	return &Grid{
		Rows:  rows,
		Cols:  cols,
		Tiles: tiles,
		Walls: make(map[Position]bool),
	}
}

// Wrap returns the position wrapped to the toroidal grid.
func (g *Grid) Wrap(row, col int) Position {
	row = ((row % g.Rows) + g.Rows) % g.Rows
	col = ((col % g.Cols) + g.Cols) % g.Cols
	return Position{Row: row, Col: col}
}

// WrapPos wraps a position to the toroidal grid.
func (g *Grid) WrapPos(p Position) Position {
	return g.Wrap(p.Row, p.Col)
}

// Get returns the tile at the given position (with wrapping).
func (g *Grid) Get(row, col int) Tile {
	p := g.Wrap(row, col)
	return g.Tiles[p.Row][p.Col]
}

// GetPos returns the tile at the given position (with wrapping).
func (g *Grid) GetPos(p Position) Tile {
	return g.Get(p.Row, p.Col)
}

// Set sets the tile at the given position (with wrapping).
func (g *Grid) Set(row, col int, t Tile) {
	p := g.Wrap(row, col)
	g.Tiles[p.Row][p.Col] = t
	if t == TileWall {
		g.Walls[p] = true
	} else {
		delete(g.Walls, p)
	}
}

// SetPos sets the tile at the given position.
func (g *Grid) SetPos(p Position, t Tile) {
	g.Set(p.Row, p.Col, t)
}

// IsWall returns true if the position is a wall.
func (g *Grid) IsWall(p Position) bool {
	return g.Walls[p]
}

// IsPassable returns true if a bot can occupy the position.
func (g *Grid) IsPassable(p Position) bool {
	return !g.IsWall(p)
}

// Distance2 returns the squared toroidal distance between two positions.
func (g *Grid) Distance2(a, b Position) int {
	dr := a.Row - b.Row
	dc := a.Col - b.Col

	// Account for wrapping - take the shorter path
	if dr > g.Rows/2 {
		dr -= g.Rows
	} else if dr < -g.Rows/2 {
		dr += g.Rows
	}
	if dc > g.Cols/2 {
		dc -= g.Cols
	} else if dc < -g.Cols/2 {
		dc += g.Cols
	}

	return dr*dr + dc*dc
}

// Distance returns the approximate toroidal distance between two positions.
func (g *Grid) Distance(a, b Position) int {
	d2 := g.Distance2(a, b)
	// Integer square root approximation
	if d2 == 0 {
		return 0
	}
	// Simple approximation - for precise distance use math.Sqrt
	d := 0
	for d*d < d2 {
		d++
	}
	return d
}

// InRadius returns true if b is within radius2 of a.
func (g *Grid) InRadius(a, b Position, radius2 int) bool {
	return g.Distance2(a, b) <= radius2
}

// Neighbors returns all positions within radius2 of the given position.
func (g *Grid) Neighbors(p Position, radius2 int) []Position {
	var result []Position
	radius := sqrtApprox(radius2)

	for dr := -radius; dr <= radius; dr++ {
		for dc := -radius; dc <= radius; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			np := g.Wrap(p.Row+dr, p.Col+dc)
			if g.Distance2(p, np) <= radius2 {
				result = append(result, np)
			}
		}
	}
	return result
}

// VisibleFrom returns all positions visible from the given positions within radius2.
func (g *Grid) VisibleFrom(positions []Position, radius2 int) map[Position]bool {
	visible := make(map[Position]bool)
	radius := sqrtApprox(radius2)

	for _, p := range positions {
		for dr := -radius; dr <= radius; dr++ {
			for dc := -radius; dc <= radius; dc++ {
				np := g.Wrap(p.Row+dr, p.Col+dc)
				if g.Distance2(p, np) <= radius2 {
					visible[np] = true
				}
			}
		}
	}
	return visible
}

// Move applies a direction to a position and returns the new position (with wrapping).
func (g *Grid) Move(p Position, d Direction) Position {
	dr, dc := d.Delta()
	return g.Wrap(p.Row+dr, p.Col+dc)
}

// RandomPassable returns a random passable position.
func (g *Grid) RandomPassable(rng *rand.Rand) Position {
	for {
		row := rng.Intn(g.Rows)
		col := rng.Intn(g.Cols)
		p := Position{Row: row, Col: col}
		if g.IsPassable(p) {
			return p
		}
	}
}

// String returns a string representation of the grid.
func (g *Grid) String() string {
	var result string
	for row := 0; row < g.Rows; row++ {
		for col := 0; col < g.Cols; col++ {
			result += g.Tiles[row][col].String()
		}
		result += "\n"
	}
	return result
}

// sqrtApprox returns an integer approximation of the square root.
func sqrtApprox(n int) int {
	if n <= 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}
