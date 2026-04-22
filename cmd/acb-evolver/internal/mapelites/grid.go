// Package mapelites implements a 4-D MAP-Elites behavior grid for diversity
// maintenance in the evolution pipeline (plan §10.2).
//
// The four behavior dimensions are:
//
//	X axis – Aggression   (0.0 = pacifist       … 1.0 = full aggressor)
//	Y axis – Economy      (0.0 = ignores energy  … 1.0 = perfect economy)
//	Z axis – Exploration  (0.0 = stays near core … 1.0 = covers >80% of map)
//	W axis – Formation    (0.0 = units scattered  … 1.0 = units always grouped)
//
// Each dimension is binned into Size levels, producing Size⁴ cells.
// Plan §10.2 specifies Size=3 → 3⁴ = 81 cells.
package mapelites

import "math"

// NumDims is the number of behavior dimensions.
const NumDims = 4

// Grid is a 4-D MAP-Elites behavior grid.
type Grid struct {
	size  int
	cells map[[NumDims]int]Cell
}

// Cell is a single niche in the grid.
type Cell struct {
	ProgramID int64
	Fitness   float64
	Occupied  bool
}

// Placement records which grid cell a program was placed into.
type Placement struct {
	X, Y, Z, W int
}

// Key returns the [4]int array key for this placement.
func (p Placement) Key() [NumDims]int {
	return [NumDims]int{p.X, p.Y, p.Z, p.W}
}

// New creates an empty Grid with the given side length per dimension.
// Total cells = size⁴. Use size=3 for the 81-cell grid per §10.2.
func New(size int) *Grid {
	return &Grid{size: size, cells: make(map[[NumDims]int]Cell)}
}

// Size returns the side length of the grid (per dimension).
func (g *Grid) Size() int {
	return g.size
}

// TotalCells returns size⁴.
func (g *Grid) TotalCells() int {
	return g.size * g.size * g.size * g.size
}

// dimBin converts a continuous [0, 1] value to a discrete bin in [0, size-1].
func (g *Grid) dimBin(v float64) int {
	return int(math.Min(math.Floor(v*float64(g.size)), float64(g.size-1)))
}

// BehaviorToCell converts continuous behavior values (each in [0, 1]) to
// discrete grid coordinates clamped to [0, size-1].
func (g *Grid) BehaviorToCell(aggression, economy, exploration, formation float64) (x, y, z, w int) {
	return g.dimBin(aggression), g.dimBin(economy), g.dimBin(exploration), g.dimBin(formation)
}

// TryPlace attempts to place a program in the cell determined by its behavior
// vector. The cell is updated only when it is empty or the new program has
// strictly higher fitness than the incumbent.
// Returns the target cell coordinates and whether the cell was updated.
func (g *Grid) TryPlace(id int64, fitness, aggression, economy, exploration, formation float64) (Placement, bool) {
	x, y, z, w := g.BehaviorToCell(aggression, economy, exploration, formation)
	key := [NumDims]int{x, y, z, w}
	cell, exists := g.cells[key]

	if !exists || fitness > cell.Fitness {
		g.cells[key] = Cell{ProgramID: id, Fitness: fitness, Occupied: true}
		return Placement{X: x, Y: y, Z: z, W: w}, true
	}
	return Placement{X: x, Y: y, Z: z, W: w}, false
}

// Get returns the cell at grid coordinates (x, y, z, w).
func (g *Grid) Get(x, y, z, w int) Cell {
	return g.cells[[NumDims]int{x, y, z, w}]
}

// OccupiedCount returns the number of filled cells.
func (g *Grid) OccupiedCount() int {
	return len(g.cells)
}

// Elite returns the cell with the highest fitness in the grid.
// Returns (zero Cell, false) when the grid is empty.
func (g *Grid) Elite() (Cell, bool) {
	var best Cell
	found := false
	for _, c := range g.cells {
		if c.Occupied && (!found || c.Fitness > best.Fitness) {
			best = c
			found = true
		}
	}
	return best, found
}

// AllElites returns a flat slice of every occupied cell.
func (g *Grid) AllElites() []Cell {
	out := make([]Cell, 0, len(g.cells))
	for _, c := range g.cells {
		if c.Occupied {
			out = append(out, c)
		}
	}
	return out
}

// Slice returns a 2-D snapshot of the grid by fixing two dimensions.
// For example, to view the aggression×economy plane at exploration=1, formation=1:
//
//	slice := grid.Slice(2, 1, 3, 1)  // fix dim 2 (z) to 1, dim 3 (w) to 1
//
// Returns a size×size grid of cells.
func (g *Grid) Slice(fixedDim1, fixedVal1, fixedDim2, fixedVal2 int) [][]Cell {
	result := make([][]Cell, g.size)
	for i := range result {
		result[i] = make([]Cell, g.size)
	}

	// Determine the two free dimensions (the ones not fixed)
	free := [2]int{-1, -1}
	fi := 0
	for d := 0; d < NumDims; d++ {
		if d != fixedDim1 && d != fixedDim2 {
			free[fi] = d
			fi++
		}
	}

	for key, cell := range g.cells {
		if key[fixedDim1] == fixedVal1 && key[fixedDim2] == fixedVal2 {
			result[key[free[0]]][key[free[1]]] = cell
		}
	}
	return result
}

// GridSnapshot is a JSON-serializable snapshot of the grid for the dashboard.
type GridSnapshot struct {
	Size     int             `json:"size"`
	DimNames [NumDims]string `json:"dim_names"`
	Cells    []CellSnapshot  `json:"cells"`
}

// CellSnapshot is one occupied cell in the grid snapshot.
type CellSnapshot struct {
	Pos      [NumDims]int `json:"pos"`
	Program  int64        `json:"program_id"`
	Fitness  float64      `json:"fitness"`
}

// Snapshot returns a JSON-serializable representation of the grid.
func (g *Grid) Snapshot() GridSnapshot {
	snap := GridSnapshot{
		Size:     g.size,
		DimNames: [NumDims]string{"aggression", "economy", "exploration", "formation"},
	}
	for key, cell := range g.cells {
		if cell.Occupied {
			snap.Cells = append(snap.Cells, CellSnapshot{
				Pos:     key,
				Program: cell.ProgramID,
				Fitness: math.Round(cell.Fitness*1000) / 1000,
			})
		}
	}
	return snap
}
