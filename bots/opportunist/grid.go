package main

func ToroidalManhattan(a, b Position, rows, cols int) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)
	dr = min(dr, rows-dr)
	dc = min(dc, cols-dc)
	return dr + dc
}

func distance2(a, b Position, rows, cols int) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)
	dr = min(dr, rows-dr)
	dc = min(dc, cols-dc)
	return dr*dr + dc*dc
}

type cardinalStep struct {
	pos Position
	dir string
}

func cardinalSteps(p Position, rows, cols int) []cardinalStep {
	steps := []struct {
		dr, dc int
		dir    string
	}{{-1, 0, "N"}, {0, 1, "E"}, {1, 0, "S"}, {0, -1, "W"}}
	result := make([]cardinalStep, 0, 4)
	for _, s := range steps {
		result = append(result, cardinalStep{
			pos: Position{
				Row: (p.Row + s.dr + rows) % rows,
				Col: (p.Col + s.dc + cols) % cols,
			},
			dir: s.dir,
		})
	}
	return result
}

func BFS(start, goal Position, passable func(Position) bool, rows, cols int) string {
	if start == goal {
		return ""
	}

	type node struct {
		pos Position
		dir string
	}

	visited := map[Position]bool{start: true}
	queue := []node{}

	for _, step := range cardinalSteps(start, rows, cols) {
		if step.pos == goal && passable(step.pos) {
			return step.dir
		}
		if passable(step.pos) && !visited[step.pos] {
			visited[step.pos] = true
			queue = append(queue, node{step.pos, step.dir})
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.pos == goal {
			return cur.dir
		}

		for _, step := range cardinalSteps(cur.pos, rows, cols) {
			if !visited[step.pos] && passable(step.pos) {
				visited[step.pos] = true
				queue = append(queue, node{step.pos, cur.dir})
			}
		}
	}

	return ""
}

func simulateMove(pos Position, dir string, rows, cols int) Position {
	dr, dc := 0, 0
	switch dir {
	case "N":
		dr = -1
	case "S":
		dr = 1
	case "E":
		dc = 1
	case "W":
		dc = -1
	}
	return Position{
		Row: (pos.Row + dr + rows) % rows,
		Col: (pos.Col + dc + cols) % cols,
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
