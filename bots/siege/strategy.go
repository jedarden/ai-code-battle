package main

import (
	"container/list"
	"math"
)

// SiegeStrategy implements systematic spawn-lockout by occupying enemy core positions.
// A core cannot spawn if its position is occupied by any bot.
type SiegeStrategy struct{}

// NewSiegeStrategy creates a new siege strategy.
func NewSiegeStrategy() *SiegeStrategy {
	return &SiegeStrategy{}
}

// ComputeMoves calculates the best moves for the current turn.
func (s *SiegeStrategy) ComputeMoves(state *GameState) []Move {
	if len(state.Bots) == 0 {
		return nil
	}

	myID := state.You.ID
	config := state.Config

	// Separate my bots from enemy bots
	myBots := make([]VisibleBot, 0)
	enemyBots := make([]VisibleBot, 0)
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		} else {
			enemyBots = append(enemyBots, bot)
		}
	}

	// Build lookup maps
	enemyPositions := make(map[Position]bool)
	for _, enemy := range enemyBots {
		enemyPositions[enemy.Position] = true
	}

	wallPositions := make(map[Position]bool)
	for _, wall := range state.Walls {
		wallPositions[wall] = true
	}

	energyPositions := make(map[Position]bool)
	for _, e := range state.Energy {
		energyPositions[e] = true
	}

	// Find all enemy cores
	enemyCores := make([]VisibleCore, 0)
	for _, core := range state.Cores {
		if core.Owner != myID && core.Active {
			enemyCores = append(enemyCores, core)
		}
	}

	// Track occupied positions
	occupiedPositions := make(map[Position]bool)
	for _, bot := range myBots {
		occupiedPositions[bot.Position] = true
	}

	moves := make([]Move, 0, len(myBots))
	assignedBots := make(map[Position]bool) // Track which bots have been assigned

	// PHASE 1: Assign bots to lockout rings around enemy cores
	lockoutAssignments := s.assignLockoutBots(myBots, enemyCores, enemyPositions, wallPositions, occupiedPositions, config)

	// Execute lockout assignments
	for botPos, targetPos := range lockoutAssignments {
		// Find the bot at botPos
		var targetBot *VisibleBot
		for i := range myBots {
			if myBots[i].Position == botPos {
				targetBot = &myBots[i]
				break
			}
		}
		if targetBot != nil {
			move := s.moveTowardPosition(*targetBot, targetPos, enemyPositions, wallPositions, occupiedPositions, config)
			if move != nil {
				moves = append(moves, *move)
				assignedBots[botPos] = true
				// Update occupied position for next moves
				dest := simulateMove(targetBot.Position, move.Direction, config)
				occupiedPositions[dest] = true
			}
		}
	}

	// PHASE 2: Unassigned bots collect energy
	usedEnergy := make(map[Position]bool)
	for _, bot := range myBots {
		if assignedBots[bot.Position] {
			continue
		}

		// Zone awareness: survival first
		if state.Zone != nil && state.Zone.Active {
			dist2 := distance2(bot.Position, state.Zone.Center, config)
			safetyMargin2 := 9 // (3 tiles)^2
			if dist2 >= state.Zone.Radius*state.Zone.Radius-safetyMargin2 {
				move := s.moveTowardPosition(bot, state.Zone.Center, enemyPositions, wallPositions, occupiedPositions, config)
				if move != nil {
					moves = append(moves, *move)
					dest := simulateMove(bot.Position, move.Direction, config)
					occupiedPositions[dest] = true
					continue
				}
			}
		}

		// Flee from nearby enemies
		if s.shouldFlee(bot.Position, myBots, enemyBots, config) {
			fleeDir := s.getFleeDirection(bot.Position, enemyBots, wallPositions, config)
			if fleeDir != DirNone {
				moves = append(moves, Move{
					Position:  bot.Position,
					Direction: fleeDir,
				})
				dest := simulateMove(bot.Position, fleeDir, config)
				occupiedPositions[dest] = true
				continue
			}
		}

		// Collect adjacent energy (immediate gain)
		collected := false
		for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
			adj := simulateMove(bot.Position, dir, config)
			if energyPositions[adj] && !usedEnergy[adj] &&
				!wallPositions[adj] && !enemyPositions[adj] && !occupiedPositions[adj] {
				moves = append(moves, Move{
					Position:  bot.Position,
					Direction: dir,
				})
				usedEnergy[adj] = true
				occupiedPositions[adj] = true
				collected = true
				break
			}
		}
		if collected {
			continue
		}

		// Find nearest energy
		_, path := s.findNearestEnergy(bot.Position, energyPositions, usedEnergy, wallPositions, enemyPositions, occupiedPositions, config)
		if path != nil && len(path) > 0 {
			moves = append(moves, Move{
				Position:  bot.Position,
				Direction: path[0],
			})
			dest := simulateMove(bot.Position, path[0], config)
			occupiedPositions[dest] = true
			continue
		}

		// No energy found - advance toward nearest enemy core
		if len(enemyCores) > 0 {
			nearestCore := s.findNearestCore(bot.Position, enemyCores, config)
			move := s.moveTowardPosition(bot, nearestCore.Position, enemyPositions, wallPositions, occupiedPositions, config)
			if move != nil {
				moves = append(moves, *move)
				dest := simulateMove(bot.Position, move.Direction, config)
				occupiedPositions[dest] = true
			}
		}
	}

	return moves
}

// assignLockoutBots assigns bots to tiles adjacent to enemy cores (greedy by distance).
func (s *SiegeStrategy) assignLockoutBots(
	myBots []VisibleBot,
	enemyCores []VisibleCore,
	enemyPositions, wallPositions, occupiedPositions map[Position]bool,
	config GameConfig,
) map[Position]Position {
	// For each enemy core, build its lockout ring (all 8 neighbors)
	type LockoutSlot struct {
		core      VisibleCore
		position  Position
		occupied  bool
		distance int // distance from nearest bot
	}

	var allSlots []LockoutSlot

	for _, core := range enemyCores {
		neighbors := getAllNeighbors(core.Position, config)
		for _, neighbor := range neighbors {
			// Check if this slot is valid (not wall, not enemy-occupied)
			if wallPositions[neighbor] || enemyPositions[neighbor] {
				continue
			}

			allSlots = append(allSlots, LockoutSlot{
				core:      core,
				position:  neighbor,
				occupied:  occupiedPositions[neighbor],
				distance:  -1, // Will be computed
			})
		}
	}

	// Greedy assignment: nearest bot -> nearest available slot
	assignments := make(map[Position]Position) // bot position -> target position

	// Keep assigning until we run out of bots or slots
	for {
		bestSlot := -1
		bestBot := -1
		bestDist := math.MaxInt32

		// For each unassigned bot, find the nearest available slot
		for bi, bot := range myBots {
			if _, assigned := assignments[bot.Position]; assigned {
				continue
			}

			for si, slot := range allSlots {
				if slot.occupied {
					continue
				}

				// Check if this slot is already targeted by another bot
				alreadyTargeted := false
				for _, target := range assignments {
					if target == slot.position {
						alreadyTargeted = true
						break
					}
				}
				if alreadyTargeted {
					continue
				}

				dist := distance2(bot.Position, slot.position, config)
				if dist < bestDist {
					bestDist = dist
					bestSlot = si
					bestBot = bi
				}
			}
		}

		// No more assignments possible
		if bestBot == -1 || bestSlot == -1 {
			break
		}

		// Make the assignment
		assignments[myBots[bestBot].Position] = allSlots[bestSlot].position
		allSlots[bestSlot].occupied = true
	}

	return assignments
}

// findNearestCore finds the nearest enemy core to a position.
func (s *SiegeStrategy) findNearestCore(pos Position, cores []VisibleCore, config GameConfig) VisibleCore {
	nearest := cores[0]
	minDist := distance2(pos, nearest.Position, config)

	for _, core := range cores[1:] {
		dist := distance2(pos, core.Position, config)
		if dist < minDist {
			minDist = dist
			nearest = core
		}
	}

	return nearest
}

// shouldFlee returns true if the bot should flee from nearby enemies.
// Only flees when locally outnumbered (nearbyAllies < nearbyEnemies).
func (s *SiegeStrategy) shouldFlee(pos Position, myBots, enemyBots []VisibleBot, config GameConfig) bool {
	// Count nearby enemies within attack radius only (no buffer)
	nearbyEnemies := 0
	for _, enemy := range enemyBots {
		dist2 := distance2(pos, enemy.Position, config)
		if dist2 <= config.AttackRadius2 {
			nearbyEnemies++
		}
	}

	if nearbyEnemies == 0 {
		return false
	}

	// Count nearby allies within the same radius (attack radius only)
	nearbyAllies := 0
	for _, ally := range myBots {
		if ally.Position == pos {
			continue // Don't count self
		}
		dist2 := distance2(pos, ally.Position, config)
		if dist2 <= config.AttackRadius2 {
			nearbyAllies++
		}
	}

	// Only flee if outnumbered
	return nearbyAllies < nearbyEnemies
}

// getFleeDirection returns the best direction to flee from enemies.
func (s *SiegeStrategy) getFleeDirection(pos Position, enemies []VisibleBot, wallPositions map[Position]bool, config GameConfig) Direction {
	// Calculate center of mass of enemies
	enemyCenter := Position{Row: 0, Col: 0}
	for _, enemy := range enemies {
		enemyCenter.Row += enemy.Position.Row
		enemyCenter.Col += enemy.Position.Col
	}
	if len(enemies) > 0 {
		enemyCenter.Row /= len(enemies)
		enemyCenter.Col /= len(enemies)
	}

	bestDir := DirNone
	bestDist := -1

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		newPos := simulateMove(pos, dir, config)
		if wallPositions[newPos] {
			continue
		}
		dist := distance2(newPos, enemyCenter, config)
		if dist > bestDist {
			bestDist = dist
			bestDir = dir
		}
	}

	return bestDir
}

// findNearestEnergy finds the nearest untargeted energy using BFS.
func (s *SiegeStrategy) findNearestEnergy(
	start Position,
	energyPositions, usedEnergy, wallPositions, enemyPositions, occupiedPositions map[Position]bool,
	config GameConfig,
) (Position, []Direction) {
	type queueItem struct {
		pos  Position
		path []Direction
	}

	visited := make(map[Position]bool)
	queue := list.New()
	queue.PushBack(queueItem{pos: start, path: []Direction{}})

	var nearestEnergy Position
	var bestPath []Direction

	for queue.Len() > 0 {
		item := queue.Remove(queue.Front()).(queueItem)
		pos := item.pos
		path := item.path

		if visited[pos] {
			continue
		}
		visited[pos] = true

		// Check if this position has untargeted energy
		if energyPositions[pos] && !usedEnergy[pos] {
			nearestEnergy = pos
			bestPath = path
			break
		}

		if len(path) > 20 { // Limit search depth
			continue
		}

		// Explore neighbors
		directions := []Direction{DirN, DirE, DirS, DirW}
		for _, dir := range directions {
			nextPos := simulateMove(pos, dir, config)
			if wallPositions[nextPos] || enemyPositions[nextPos] || occupiedPositions[nextPos] {
				continue
			}
			if !visited[nextPos] {
				newPath := make([]Direction, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = dir
				queue.PushBack(queueItem{pos: nextPos, path: newPath})
			}
		}
	}

	return nearestEnergy, bestPath
}

// moveTowardPosition returns a move that approaches the target position.
func (s *SiegeStrategy) moveTowardPosition(
	bot VisibleBot,
	target Position,
	enemyPositions, wallPositions, occupiedPositions map[Position]bool,
	config GameConfig,
) *Move {
	bestDir := DirNone
	bestDist2 := math.MaxInt32

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		newPos := simulateMove(bot.Position, dir, config)
		if wallPositions[newPos] || enemyPositions[newPos] || occupiedPositions[newPos] {
			continue
		}

		dist2 := distance2(newPos, target, config)
		if dist2 < bestDist2 {
			bestDist2 = dist2
			bestDir = dir
		}
	}

	if bestDir != DirNone {
		return &Move{
			Position:  bot.Position,
			Direction: bestDir,
		}
	}
	return nil
}

// Helper functions

// getAllNeighbors returns all 8 neighbors (including diagonals) of a position.
func getAllNeighbors(pos Position, config GameConfig) []Position {
	neighbors := make([]Position, 0, 8)
	deltas := []struct{ dr, dc int }{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	for _, delta := range deltas {
		newRow := (pos.Row + delta.dr + config.Rows) % config.Rows
		newCol := (pos.Col + delta.dc + config.Cols) % config.Cols
		neighbors = append(neighbors, Position{Row: newRow, Col: newCol})
	}

	return neighbors
}

// distance2 calculates squared toroidal distance.
func distance2(a, b Position, config GameConfig) int {
	dr := a.Row - b.Row
	dc := a.Col - b.Col

	// Apply toroidal wrapping
	if dr > config.Rows/2 {
		dr = config.Rows - dr
	} else if dr < -config.Rows/2 {
		dr = -(config.Rows + dr)
	}
	if dc > config.Cols/2 {
		dc = config.Cols - dc
	} else if dc < -config.Cols/2 {
		dc = -(config.Cols + dc)
	}

	return dr*dr + dc*dc
}

// simulateMove returns the new position after moving in a direction.
func simulateMove(pos Position, dir Direction, config GameConfig) Position {
	var newRow, newCol int

	switch dir {
	case DirN:
		newRow = (pos.Row - 1 + config.Rows) % config.Rows
		newCol = pos.Col
	case DirE:
		newRow = pos.Row
		newCol = (pos.Col + 1) % config.Cols
	case DirS:
		newRow = (pos.Row + 1) % config.Rows
		newCol = pos.Col
	case DirW:
		newRow = pos.Row
		newCol = (pos.Col - 1 + config.Cols) % config.Cols
	default:
		return pos
	}

	return Position{Row: newRow, Col: newCol}
}

