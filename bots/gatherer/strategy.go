package main

import (
	"container/list"
)

// GathererStrategy implements energy-focused gameplay with combat avoidance.
type GathererStrategy struct {
	// No persistent state needed - strategy is stateless per turn
}

// NewGathererStrategy creates a new gatherer strategy.
func NewGathererStrategy() *GathererStrategy {
	return &GathererStrategy{}
}

// ComputeMoves calculates the best moves for the current turn.
func (s *GathererStrategy) ComputeMoves(state *GameState) []Move {
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

	// Build enemy positions map for quick lookup
	enemyPositions := make(map[Position]bool)
	for _, enemy := range enemyBots {
		enemyPositions[enemy.Position] = true
	}

	// Build energy positions map
	energyPositions := make(map[Position]bool)
	for _, e := range state.Energy {
		energyPositions[e] = true
	}

	// For each of my bots, find the best move
	moves := make([]Move, 0, len(myBots))
	usedEnergy := make(map[Position]bool) // Track energy already targeted

	for _, bot := range myBots {
		move := s.computeBotMove(bot, myBots, enemyBots, enemyPositions,
			energyPositions, usedEnergy, config)
		if move != nil {
			moves = append(moves, *move)
			// Mark energy as targeted if bot will collect it
			if energyPositions[move.Position] || energyPositions[simulateMove(bot.Position, move.Direction, config)] {
				usedEnergy[simulateMove(bot.Position, move.Direction, config)] = true
			}
		}
	}

	return moves
}

// computeBotMove calculates the best move for a single bot.
func (s *GathererStrategy) computeBotMove(
	bot VisibleBot,
	myBots, enemyBots []VisibleBot,
	enemyPositions, energyPositions, usedEnergy map[Position]bool,
	config GameConfig,
) *Move {
	// First check if we should flee from enemies
	if s.shouldFlee(bot.Position, enemyBots, config) {
		fleeDir := s.getFleeDirection(bot.Position, enemyBots, config)
		if fleeDir != "" {
			return &Move{
				Position:  bot.Position,
				Direction: fleeDir,
			}
		}
	}

	// Try to find nearest untargeted energy
	_, path := s.findNearestEnergy(bot.Position, energyPositions, usedEnergy, enemyPositions, config)
	if path != nil && len(path) > 0 {
		// Move towards the energy
		return &Move{
			Position:  bot.Position,
			Direction: path[0],
		}
	}

	// No energy visible or reachable - spread out to explore
	return s.getExploreMove(bot.Position, myBots, enemyPositions, config)
}

// shouldFlee returns true if the bot should flee from nearby enemies.
func (s *GathererStrategy) shouldFlee(pos Position, enemies []VisibleBot, config GameConfig) bool {
	for _, enemy := range enemies {
		dist2 := distance2(pos, enemy.Position, config)
		// Flee if enemy is within attack range + 2 tiles buffer
		if dist2 <= config.AttackRadius2+4 {
			return true
		}
	}
	return false
}

// getFleeDirection returns the best direction to flee from enemies.
func (s *GathererStrategy) getFleeDirection(pos Position, enemies []VisibleBot, config GameConfig) Direction {
	// Calculate the center of mass of enemies
	enemyCenter := Position{Row: 0, Col: 0}
	for _, enemy := range enemies {
		enemyCenter.Row += enemy.Position.Row
		enemyCenter.Col += enemy.Position.Col
	}
	if len(enemies) > 0 {
		enemyCenter.Row /= len(enemies)
		enemyCenter.Col /= len(enemies)
	}

	// Move away from enemy center
	dr := pos.Row - enemyCenter.Row
	dc := pos.Col - enemyCenter.Col

	// Normalize direction
	if dr > 0 {
		return DirS
	} else if dr < 0 {
		return DirN
	} else if dc > 0 {
		return DirE
	} else if dc < 0 {
		return DirW
	}

	// Default: move North
	return DirN
}

// findNearestEnergy finds the nearest untargeted energy using BFS.
func (s *GathererStrategy) findNearestEnergy(
	start Position,
	energyPositions, usedEnergy, enemyPositions map[Position]bool,
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

		// Don't path through enemy-adjacent tiles
		if len(path) > 0 && s.isNearEnemy(pos, enemyPositions, config) {
			continue
		}

		// Explore neighbors
		directions := []Direction{DirN, DirE, DirS, DirW}
		for _, dir := range directions {
			nextPos := simulateMove(pos, dir, config)
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

// isNearEnemy checks if a position is adjacent to any enemy.
func (s *GathererStrategy) isNearEnemy(pos Position, enemyPositions map[Position]bool, config GameConfig) bool {
	directions := []Direction{DirN, DirE, DirS, DirW}
	for _, dir := range directions {
		adj := simulateMove(pos, dir, config)
		if enemyPositions[adj] {
			return true
		}
	}
	return false
}

// getExploreMove returns a move for exploring when no energy is visible.
func (s *GathererStrategy) getExploreMove(
	pos Position,
	myBots []VisibleBot,
	enemyPositions map[Position]bool,
	config GameConfig,
) *Move {
	// Calculate direction away from other friendly bots (spread out)
	directions := []Direction{DirN, DirE, DirS, DirW}
	bestDir := DirN
	bestScore := -999999

	for _, dir := range directions {
		newPos := simulateMove(pos, dir, config)

		// Skip if moving towards enemy
		if s.isNearEnemy(newPos, enemyPositions, config) {
			continue
		}

		// Score based on distance from other bots (prefer spreading out)
		score := 0
		for _, other := range myBots {
			if other.Position != pos {
				dist := distance2(newPos, other.Position, config)
				score += int(dist) // Higher is better (further from others)
			}
		}

		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}

	return &Move{
		Position:  pos,
		Direction: bestDir,
	}
}

// distance2 calculates squared Euclidean distance with toroidal wrapping.
func distance2(a, b Position, config GameConfig) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)

	// Apply toroidal wrapping
	if dr > config.Rows/2 {
		dr = config.Rows - dr
	}
	if dc > config.Cols/2 {
		dc = config.Cols - dc
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

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
