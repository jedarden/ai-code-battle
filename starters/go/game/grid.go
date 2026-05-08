// Package game provides grid utilities for AI Code Battle bots.
package game

// ToroidalManhattan returns the Manhattan distance between two positions
// on a toroidal (wrapping) grid.
func ToroidalManhattan(a, b Position, rows, cols int) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)

	// Apply toroidal wrapping
	if dr > rows/2 {
		dr = rows - dr
	}
	if dc > cols/2 {
		dc = cols - dc
	}

	return dr + dc
}

// ToroidalDistance2 returns the squared Euclidean distance between two
// positions on a toroidal grid.
func ToroidalDistance2(a, b Position, rows, cols int) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)

	// Apply toroidal wrapping
	if dr > rows/2 {
		dr = rows - dr
	}
	if dc > cols/2 {
		dc = cols - dc
	}

	return dr*dr + dc*dc
}

// Neighbors returns the 4 cardinal neighbors of a position on a toroidal grid.
func Neighbors(p Position, rows, cols int) []Position {
	directions := []struct {
		dr, dc int
		dir    Direction
	}{
		{-1, 0, DirN},
		{0, 1, DirE},
		{1, 0, DirS},
		{0, -1, DirW},
	}

	result := make([]Position, 0, 4)
	for _, d := range directions {
		result = append(result, Position{
			Row: (p.Row + d.dr + rows) % rows,
			Col: (p.Col + d.dc + cols) % cols,
		})
	}
	return result
}

// NeighborInDirection returns the position reached by moving one step
// in the given direction on a toroidal grid.
func NeighborInDirection(p Position, dir Direction, rows, cols int) Position {
	switch dir {
	case DirN:
		return Position{Row: (p.Row - 1 + rows) % rows, Col: p.Col}
	case DirE:
		return Position{Row: p.Row, Col: (p.Col + 1) % cols}
	case DirS:
		return Position{Row: (p.Row + 1) % rows, Col: p.Col}
	case DirW:
		return Position{Row: p.Row, Col: (p.Col - 1 + cols) % cols}
	default:
		return p
	}
}

// AllNeighbors returns the 8-directional neighbors (including diagonals)
// of a position on a toroidal grid.
func AllNeighbors(p Position, rows, cols int) []Position {
	offsets := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	result := make([]Position, 0, 8)
	for _, off := range offsets {
		result = append(result, Position{
			Row: (p.Row + off[0] + rows) % rows,
			Col: (p.Col + off[1] + cols) % cols,
		})
	}
	return result
}

// BFSDirection finds the shortest path from start to goal using BFS,
// returning only the first direction to move. Returns empty string if
// no path exists or if start == goal.
//
// The passable function should return true for positions that can be entered.
func BFSDirection(start, goal Position, passable func(Position) bool, rows, cols int) Direction {
	if start == goal {
		return ""
	}

	type node struct {
		pos Position
		dir Direction // first direction taken to reach this node
	}

	visited := make(map[Position]bool)
	visited[start] = true

	queue := []node{{start, ""}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, nextPos := range Neighbors(cur.pos, rows, cols) {
			if nextPos == goal {
				// Found goal - return the first direction
				if cur.dir == "" {
					// Direct neighbor - determine direction
					dr := nextPos.Row - start.Row
					dc := nextPos.Col - start.Col
					// Normalize for wrapping
					if dr < -rows/2 {
						dr += rows
					} else if dr > rows/2 {
						dr -= rows
					}
					if dc < -cols/2 {
						dc += cols
					} else if dc > cols/2 {
						dc -= cols
					}

					switch {
					case dr < 0:
						return DirN
					case dr > 0:
						return DirS
					case dc < 0:
						return DirW
					case dc > 0:
						return DirE
					}
				}
				return cur.dir
			}

			if !visited[nextPos] && passable(nextPos) {
				visited[nextPos] = true
				firstDir := cur.dir
				if firstDir == "" {
					// Determine direction from start to nextPos
					dr := nextPos.Row - start.Row
					dc := nextPos.Col - start.Col
					// Normalize for wrapping
					if dr < -rows/2 {
						dr += rows
					} else if dr > rows/2 {
						dr -= rows
					}
					if dc < -cols/2 {
						dc += cols
					} else if dc > cols/2 {
						dc -= cols
					}

					switch {
					case dr < 0:
						firstDir = DirN
					case dr > 0:
						firstDir = DirS
					case dc < 0:
						firstDir = DirW
					case dc > 0:
						firstDir = DirE
					}
				}
				queue = append(queue, node{nextPos, firstDir})
			}
		}
	}

	return "" // No path found
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
