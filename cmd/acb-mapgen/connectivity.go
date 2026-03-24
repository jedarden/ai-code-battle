// Command acb-mapgen generates symmetric maps for AI Code Battle.
package main

import (
	"container/list"
	"math/rand"
)

// PositionSet is a set of positions for fast lookup.
type PositionSet map[Position]bool

// CheckConnectivity verifies that all passable tiles in the map are reachable
// from each player's core. This is critical for ensuring fair gameplay.
//
// For toroidal grids, we use BFS from a starting point and verify all
// passable tiles are visited.
func CheckConnectivity(m *Map) bool {
	if len(m.Cores) == 0 {
		return true // No cores, nothing to validate
	}

	// Build a set of passable positions
	passable, totalPassable := m.passablePositions()
	if totalPassable == 0 {
		return false // No passable tiles at all
	}

	// BFS from first core position
	start := m.Cores[0].Position
	if !passable[start] {
		return false // Core itself is not passable
	}

	visited := make(PositionSet)
	queue := list.New()
	queue.PushBack(start)
	visited[start] = true
	count := 1

	// Direction deltas for 4-connected neighbors (cardinal directions)
	dirs := []Position{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

	for queue.Len() > 0 {
		front := queue.Front()
		queue.Remove(front)
		curr := front.Value.(Position)

		for _, d := range dirs {
			// Toroidal wrapping
			nr := ((curr.Row + d.Row) % m.Rows + m.Rows) % m.Rows
			nc := ((curr.Col + d.Col) % m.Cols + m.Cols) % m.Cols
			np := Position{Row: nr, Col: nc}

			if passable[np] && !visited[np] {
				visited[np] = true
				queue.PushBack(np)
				count++
			}
		}
	}

	// All passable tiles must be visited
	return count == totalPassable
}

// passablePositions returns all positions that are not walls and the count.
func (m *Map) passablePositions() (PositionSet, int) {
	result := make(PositionSet)

	// Build wall set for fast lookup
	wallSet := make(map[Position]bool)
	for _, w := range m.Walls {
		wallSet[w] = true
	}

	for r := 0; r < m.Rows; r++ {
		for c := 0; c < m.Cols; c++ {
			p := Position{Row: r, Col: c}
			if !wallSet[p] {
				result[p] = true
			}
		}
	}
	return result, len(result)
}

// EnsureConnectivity generates walls while maintaining full connectivity.
// It uses a retry mechanism: if walls disconnect the map, regenerate and try again.
func EnsureConnectivity(numPlayers, rows, cols int, wallDensity float64, numEnergyNodes int, rng *rand.Rand, maxAttempts int) *Map {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		m := generateMap(numPlayers, rows, cols, wallDensity, numEnergyNodes, rng)

		if CheckConnectivity(m) {
			return m
		}
	}

	// If all attempts failed, return nil
	return nil
}
