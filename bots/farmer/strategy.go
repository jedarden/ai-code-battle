package main

import "math"

const (
	fleeRadius2       = 9  // flee if enemy within 3 cells (squared = 9)
	dangerBuffer      = 20 // extra buffer beyond attack radius for avoidance
)

// FarmerStrategy maximizes energy collection and spawn rate while avoiding combat.
type FarmerStrategy struct{}

func NewFarmerStrategy() *FarmerStrategy {
	return &FarmerStrategy{}
}

// ComputeMoves assigns each owned bot to seek energy, flee enemies, or
// stay near core for spawning.
func (s *FarmerStrategy) ComputeMoves(state *GameState) []Move {
	rows := state.Config.Rows
	cols := state.Config.Cols
	attackR2 := state.Config.AttackRadius2
	myID := state.You.ID

	// Build lookup maps
	wallSet := make(map[Position]bool, len(state.Walls))
	for _, w := range state.Walls {
		wallSet[w] = true
	}

	enemySet := make(map[Position]bool)
	enemyPositions := make([]Position, 0)
	for _, b := range state.Bots {
		if b.Owner != myID {
			enemySet[b.Position] = true
			enemyPositions = append(enemyPositions, b.Position)
		}
	}

	// Identify my active cores
	myCores := make([]Position, 0)
	for _, c := range state.Cores {
		if c.Owner == myID && c.Active {
			myCores = append(myCores, c.Position)
		}
	}

	// Determine which energy tiles are contested (enemy adjacent)
	contestedEnergy := make(map[Position]bool)
	for _, e := range state.Energy {
		for _, ep := range enemyPositions {
			if distance2(e, ep, rows, cols) <= 2 {
				contestedEnergy[e] = true
				break
			}
		}
	}

	// Separate my bots
	myBots := make([]VisibleBot, 0, len(state.Bots))
	for _, b := range state.Bots {
		if b.Owner == myID {
			myBots = append(myBots, b)
		}
	}

	// Track assigned energy targets to avoid duplicate assignments
	assignedEnergy := make(map[Position]bool)
	// Track claimed destinations to prevent self-collision
	claimedDests := make(map[Position]bool)

	// Sort bots: prioritize bots closest to uncontested energy
	botScores := make([]int, len(myBots))
	for i, b := range myBots {
		bestDist := math.MaxInt32
		for _, e := range state.Energy {
			if !contestedEnergy[e] {
				d := distance2(b.Position, e, rows, cols)
				if d < bestDist {
					bestDist = d
				}
			}
		}
		botScores[i] = bestDist
	}

	// Simple selection sort for small arrays
	sorted := make([]int, len(myBots))
	for i := range sorted {
		sorted[i] = i
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if botScores[sorted[j]] < botScores[sorted[i]] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	moves := make([]Move, 0, len(myBots))

	for _, idx := range sorted {
		bot := myBots[idx]
		dir := s.computeBotMove(bot, state, wallSet, enemyPositions, enemySet,
			myCores, contestedEnergy, assignedEnergy, claimedDests, rows, cols, attackR2)

		dest := bot.Position
		if dir != "" {
			dest = simulateMove(bot.Position, dir, rows, cols)
		}

		// If destination is already claimed by another bot, hold position
		if dir != "" && claimedDests[dest] {
			dir = ""
			dest = bot.Position
		}

		claimedDests[dest] = true
		if dir != "" {
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		}
	}

	return moves
}

func (s *FarmerStrategy) computeBotMove(
	bot VisibleBot,
	state *GameState,
	wallSet map[Position]bool,
	enemyPositions []Position,
	enemySet map[Position]bool,
	myCores []Position,
	contestedEnergy map[Position]bool,
	assignedEnergy map[Position]bool,
	claimedDests map[Position]bool,
	rows, cols, attackR2 int,
) string {
	pos := bot.Position

	// Priority 1: FLEE if any enemy within flee radius
	if len(enemyPositions) > 0 {
		minEnemyDist2 := math.MaxInt32
		for _, ep := range enemyPositions {
			d := distance2(pos, ep, rows, cols)
			if d < minEnemyDist2 {
				minEnemyDist2 = d
			}
		}

		if minEnemyDist2 <= fleeRadius2 {
			dir := s.fleeDirection(pos, enemyPositions, wallSet, enemySet, rows, cols)
			if dir != "" {
				return dir
			}
		}

		// Also flee if enemy within attack radius + buffer
		if minEnemyDist2 <= attackR2+dangerBuffer {
			dir := s.fleeDirection(pos, enemyPositions, wallSet, enemySet, rows, cols)
			if dir != "" {
				return dir
			}
		}
	}

	passable := func(p Position) bool {
		if wallSet[p] {
			return false
		}
		// Avoid stepping directly onto enemy positions
		if enemySet[p] {
			return false
		}
		return true
	}

	// Priority 2: Seek nearest uncontested, unassigned energy
	var bestEnergyTarget *Position
	bestDist := math.MaxInt32
	for i, e := range state.Energy {
		if contestedEnergy[e] || assignedEnergy[e] {
			continue
		}
		d := distance2(pos, e, rows, cols)
		if d < bestDist {
			bestDist = d
			eCopy := state.Energy[i]
			bestEnergyTarget = &eCopy
		}
	}

	if bestEnergyTarget != nil {
		assignedEnergy[*bestEnergyTarget] = true
		dir := BFS(pos, *bestEnergyTarget, passable, rows, cols)
		if dir != "" {
			return dir
		}
	}

	// Priority 3: If on or adjacent to energy, collect it (hold or step onto)
	for _, e := range state.Energy {
		if e == pos {
			// Already on energy, hold to collect
			return ""
		}
	}

	// Priority 4: Move toward nearest energy (even contested)
	if len(state.Energy) > 0 {
		bestDist = math.MaxInt32
		var target Position
		for _, e := range state.Energy {
			d := distance2(pos, e, rows, cols)
			if d < bestDist {
				bestDist = d
				target = e
			}
		}
		dir := BFS(pos, target, passable, rows, cols)
		if dir != "" {
			return dir
		}
	}

	// Priority 5: Stay near active core for spawning
	if len(myCores) > 0 {
		nearestCoreDist := math.MaxInt32
		var nearestCore Position
		for _, c := range myCores {
			d := distance2(pos, c, rows, cols)
			if d < nearestCoreDist {
				nearestCoreDist = d
				nearestCore = c
			}
		}

		// If far from core, move toward it
		if nearestCoreDist > 4 {
			dir := BFS(pos, nearestCore, passable, rows, cols)
			if dir != "" {
				return dir
			}
		}
	}

	// Priority 6: Spread out from other friendly bots to avoid self-collision
	return s.spreadMove(pos, state, claimedDests, rows, cols)
}

// fleeDirection picks the cardinal direction that maximizes distance from
// the nearest enemy.
func (s *FarmerStrategy) fleeDirection(
	pos Position,
	enemies []Position,
	wallSet, enemySet map[Position]bool,
	rows, cols int,
) string {
	bestDir := ""
	bestMinDist := -1

	for _, step := range cardinalSteps(pos, rows, cols) {
		if wallSet[step.pos] || enemySet[step.pos] {
			continue
		}

		minDist := math.MaxInt32
		for _, ep := range enemies {
			d := distance2(step.pos, ep, rows, cols)
			if d < minDist {
				minDist = d
			}
		}

		if minDist > bestMinDist {
			bestMinDist = minDist
			bestDir = step.dir
		}
	}

	return bestDir
}

// spreadMove picks a direction that moves away from the densest cluster
// of friendly bots.
func (s *FarmerStrategy) spreadMove(
	pos Position,
	state *GameState,
	claimedDests map[Position]bool,
	rows, cols int,
) string {
	myID := state.You.ID

	bestDir := ""
	bestScore := -1

	for _, step := range cardinalSteps(pos, rows, cols) {
		if claimedDests[step.pos] {
			continue
		}

		// Score = minimum distance to any friendly bot (maximize spacing)
		minDist := math.MaxInt32
		for _, b := range state.Bots {
			if b.Owner != myID {
				continue
			}
			d := distance2(step.pos, b.Position, rows, cols)
			if d < minDist {
				minDist = d
			}
		}

		if minDist > bestScore {
			bestScore = minDist
			bestDir = step.dir
		}
	}

	return bestDir
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
