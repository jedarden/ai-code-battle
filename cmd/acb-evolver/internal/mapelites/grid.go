// Package mapelites implements a 2-D MAP-Elites behavior grid for diversity
// maintenance in the evolution pipeline.
//
// The two behavior dimensions are:
//
//	X axis – aggression (0.0 = pacifist … 1.0 = full aggressor)
//	Y axis – economy   (0.0 = ignores energy … 1.0 = perfect economy)
//
// Each cell in the Size×Size grid holds the ID and fitness of the single best
// program discovered in that behavioral niche.
package mapelites

import "math"

// Grid is a 2-D MAP-Elites behavior grid.
type Grid struct {
	size  int
	cells [][]Cell
}

// Cell is a single niche in the grid.
type Cell struct {
	ProgramID int64
	Fitness   float64
	Occupied  bool
}

// Placement records which grid cell a program was placed into.
type Placement struct {
	X, Y int
}

// New creates an empty Grid with the given side length.
func New(size int) *Grid {
	cells := make([][]Cell, size)
	for i := range cells {
		cells[i] = make([]Cell, size)
	}
	return &Grid{size: size, cells: cells}
}

// BehaviorToCell converts continuous behavior values (each in [0, 1]) to
// discrete grid coordinates clamped to [0, size-1].
func (g *Grid) BehaviorToCell(aggression, economy float64) (x, y int) {
	x = int(math.Min(math.Floor(aggression*float64(g.size)), float64(g.size-1)))
	y = int(math.Min(math.Floor(economy*float64(g.size)), float64(g.size-1)))
	return
}

// TryPlace attempts to place a program in the cell determined by its behavior
// vector.  The cell is updated only when it is empty or the new program has
// strictly higher fitness than the incumbent.
// Returns the target cell coordinates and whether the cell was updated.
func (g *Grid) TryPlace(id int64, fitness, aggression, economy float64) (Placement, bool) {
	x, y := g.BehaviorToCell(aggression, economy)
	cell := &g.cells[x][y]

	if !cell.Occupied || fitness > cell.Fitness {
		*cell = Cell{ProgramID: id, Fitness: fitness, Occupied: true}
		return Placement{X: x, Y: y}, true
	}
	return Placement{X: x, Y: y}, false
}

// Get returns the cell at grid coordinates (x, y).
func (g *Grid) Get(x, y int) Cell {
	return g.cells[x][y]
}

// Size returns the side length of the grid.
func (g *Grid) Size() int {
	return g.size
}

// OccupiedCount returns the number of filled cells.
func (g *Grid) OccupiedCount() int {
	n := 0
	for _, row := range g.cells {
		for _, c := range row {
			if c.Occupied {
				n++
			}
		}
	}
	return n
}

// Elite returns the cell with the highest fitness in the grid.
// Returns (zero Cell, false) when the grid is empty.
func (g *Grid) Elite() (Cell, bool) {
	var best Cell
	found := false
	for _, row := range g.cells {
		for _, c := range row {
			if c.Occupied && (!found || c.Fitness > best.Fitness) {
				best = c
				found = true
			}
		}
	}
	return best, found
}

// AllElites returns a flat slice of every occupied cell.
func (g *Grid) AllElites() []Cell {
	var out []Cell
	for _, row := range g.cells {
		for _, c := range row {
			if c.Occupied {
				out = append(out, c)
			}
		}
	}
	return out
}
