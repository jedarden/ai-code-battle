package main

// ToroidalManhattan returns Manhattan distance with wrap-around.
func ToroidalManhattan(a, b Position, rows, cols int) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)
	dr = min(dr, rows-dr)
	dc = min(dc, cols-dc)
	return dr + dc
}

// ToroidalChebyshev returns Chebyshev distance with wrap-around.
func ToroidalChebyshev(a, b Position, rows, cols int) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)
	dr = min(dr, rows-dr)
	dc = min(dc, cols-dc)
	return max(dr, dc)
}

// Neighbors returns 8-directional neighbors with wrap-around.
func Neighbors(p Position, rows, cols int) []Position {
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

// BFS finds the shortest path from start to goal on a toroidal grid.
// passable returns true if a cell can be entered.
// Returns the path (excluding start) or nil if unreachable.
func BFS(start, goal Position, passable func(Position) bool, rows, cols int) []Position {
	if start == goal {
		return []Position{}
	}

	type node struct {
		pos  Position
		path []Position
	}

	visited := map[Position]bool{start: true}
	queue := []node{{start, nil}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, n := range Neighbors(cur.pos, rows, cols) {
			newPath := make([]Position, len(cur.path), len(cur.path)+1)
			copy(newPath, cur.path)
			newPath = append(newPath, n)

			if n == goal {
				return newPath
			}
			if !visited[n] && passable(n) {
				visited[n] = true
				queue = append(queue, node{n, newPath})
			}
		}
	}

	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
