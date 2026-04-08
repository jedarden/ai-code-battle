package engine

import (
	"container/list"
	"math"
	"math/rand"
)

// GathererBot prioritizes energy collection while avoiding combat.
type GathererBot struct {
	rng *rand.Rand
}

// NewGathererBot creates a new gatherer bot.
func NewGathererBot(seed int64) *GathererBot {
	return &GathererBot{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// GetMoves returns moves focused on gathering energy.
func (b *GathererBot) GetMoves(state *VisibleState) ([]Move, error) {
	if len(state.Bots) == 0 {
		return nil, nil
	}

	myID := state.You.ID
	config := state.Config

	// Separate my bots from enemies
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

	energyPositions := make(map[Position]bool)
	for _, e := range state.Energy {
		energyPositions[e] = true
	}

	wallPositions := make(map[Position]bool)
	for _, w := range state.Walls {
		wallPositions[w] = true
	}

	moves := make([]Move, 0, len(myBots))
	usedEnergy := make(map[Position]bool)

	for _, bot := range myBots {
		move := b.computeBotMove(bot, myBots, enemyPositions, energyPositions, usedEnergy, wallPositions, config)
		if move != nil {
			moves = append(moves, *move)
		}
	}

	return moves, nil
}

func (b *GathererBot) computeBotMove(
	bot VisibleBot,
	myBots []VisibleBot,
	enemyPositions, energyPositions, usedEnergy, wallPositions map[Position]bool,
	config Config,
) *Move {
	// Check if we should flee from enemies
	if b.shouldFlee(bot.Position, enemyPositions, config) {
		fleeDir := b.getFleeDirection(bot.Position, enemyPositions, wallPositions, config)
		if fleeDir != DirNone {
			return &Move{
				Position:  bot.Position,
				Direction: fleeDir,
			}
		}
	}

	// Find nearest untargeted energy
	_, path := b.findNearestEnergy(bot.Position, energyPositions, usedEnergy, enemyPositions, wallPositions, config)
	if path != nil && len(path) > 0 {
		return &Move{
			Position:  bot.Position,
			Direction: path[0],
		}
	}

	// No energy visible - spread out to explore
	return b.getExploreMove(bot.Position, myBots, enemyPositions, wallPositions, config)
}

func (b *GathererBot) shouldFlee(pos Position, enemyPositions map[Position]bool, config Config) bool {
	for enemyPos := range enemyPositions {
		dist2 := distance2(pos, enemyPos, config.Rows, config.Cols)
		if dist2 <= config.AttackRadius2+4 {
			return true
		}
	}
	return false
}

func (b *GathererBot) getFleeDirection(pos Position, enemyPositions, wallPositions map[Position]bool, config Config) Direction {
	// Calculate center of mass of enemies
	enemyCenter := Position{Row: 0, Col: 0}
	count := 0
	for enemyPos := range enemyPositions {
		enemyCenter.Row += enemyPos.Row
		enemyCenter.Col += enemyPos.Col
		count++
	}
	if count > 0 {
		enemyCenter.Row /= count
		enemyCenter.Col /= count
	}

	// Move away from enemy center
	directions := []Direction{DirN, DirE, DirS, DirW}
	bestDir := DirN
	bestDist := -1

	for _, dir := range directions {
		newPos := simulateMove(pos, dir, config.Rows, config.Cols)
		if wallPositions[newPos] {
			continue
		}
		dist := distance2(newPos, enemyCenter, config.Rows, config.Cols)
		if dist > bestDist {
			bestDist = dist
			bestDir = dir
		}
	}

	return bestDir
}

func (b *GathererBot) findNearestEnergy(
	start Position,
	energyPositions, usedEnergy, enemyPositions, wallPositions map[Position]bool,
	config Config,
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
		if len(path) > 0 && b.isNearEnemy(pos, enemyPositions, config) {
			continue
		}

		// Explore neighbors
		directions := []Direction{DirN, DirE, DirS, DirW}
		for _, dir := range directions {
			nextPos := simulateMove(pos, dir, config.Rows, config.Cols)
			if !visited[nextPos] && !wallPositions[nextPos] {
				newPath := make([]Direction, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = dir
				queue.PushBack(queueItem{pos: nextPos, path: newPath})
			}
		}
	}

	return nearestEnergy, bestPath
}

func (b *GathererBot) isNearEnemy(pos Position, enemyPositions map[Position]bool, config Config) bool {
	directions := []Direction{DirN, DirE, DirS, DirW}
	for _, dir := range directions {
		adj := simulateMove(pos, dir, config.Rows, config.Cols)
		if enemyPositions[adj] {
			return true
		}
	}
	return false
}

func (b *GathererBot) getExploreMove(
	pos Position,
	myBots []VisibleBot,
	enemyPositions, wallPositions map[Position]bool,
	config Config,
) *Move {
	// Explore toward map center — that's where energy and enemies are
	center := Position{Row: config.Rows / 2, Col: config.Cols / 2}

	bestDir := DirNone
	bestScore := -999999.0

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		newPos := simulateMove(pos, dir, config.Rows, config.Cols)

		if wallPositions[newPos] {
			continue
		}

		if b.isNearEnemy(newPos, enemyPositions, config) {
			continue
		}

		score := 0.0

		// Move toward center
		distToCenter := float64(distance2(newPos, center, config.Rows, config.Cols))
		currentDist := float64(distance2(pos, center, config.Rows, config.Cols))
		score += (currentDist - distToCenter) * 5

		// Spread out from other bots
		for _, other := range myBots {
			if other.Position != pos {
				dist := float64(distance2(newPos, other.Position, config.Rows, config.Cols))
				score += dist * 0.5
			}
		}

		// Add slight randomness to avoid getting stuck
		score += b.rng.Float64() * 2

		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}

	if bestDir != DirNone {
		return &Move{Position: pos, Direction: bestDir}
	}
	return nil
}

// RusherBot aggressively rushes toward enemy cores.
type RusherBot struct {
	rng              *rand.Rand
	knownEnemyCores  map[Position]bool
}

// NewRusherBot creates a new rusher bot.
func NewRusherBot(seed int64) *RusherBot {
	return &RusherBot{
		rng:             rand.New(rand.NewSource(seed)),
		knownEnemyCores: make(map[Position]bool),
	}
}

// GetMoves returns moves rushing toward enemy cores.
func (b *RusherBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	// Update known enemy cores
	for _, core := range state.Cores {
		if core.Owner != myID && core.Active {
			b.knownEnemyCores[core.Position] = true
		}
	}

	// Separate my bots from enemies
	myBots := make([]VisibleBot, 0)
	enemyBots := make([]VisibleBot, 0)
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		} else {
			enemyBots = append(enemyBots, bot)
		}
	}

	if len(myBots) == 0 {
		return nil, nil
	}

	// Build lookup maps
	enemyPositions := make(map[Position]bool)
	for _, enemy := range enemyBots {
		enemyPositions[enemy.Position] = true
	}

	wallPositions := make(map[Position]bool)
	for _, w := range state.Walls {
		wallPositions[w] = true
	}

	energyPositions := make(map[Position]bool)
	for _, e := range state.Energy {
		energyPositions[e] = true
	}

	// Find targets to rush
	targets := b.getRushTargets(state, myID)

	moves := make([]Move, 0, len(myBots))

	for _, bot := range myBots {
		// Opportunistic: grab adjacent energy while rushing
		if len(myBots) <= 2 {
			for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
				adj := simulateMove(bot.Position, dir, config.Rows, config.Cols)
				if energyPositions[adj] && !wallPositions[adj] {
					moves = append(moves, Move{Position: bot.Position, Direction: dir})
					delete(energyPositions, adj)
					goto nextBot
				}
			}
		}

		if dir := b.findBestMove(bot.Position, targets, enemyPositions, wallPositions, config); dir != DirNone {
			moves = append(moves, Move{
				Position:  bot.Position,
				Direction: dir,
			})
		}
	nextBot:
	}

	return moves, nil
}

func (b *RusherBot) getRushTargets(state *VisibleState, myID int) []Position {
	targets := make([]Position, 0)

	// First priority: visible enemy cores
	for _, core := range state.Cores {
		if core.Owner != myID && core.Active {
			targets = append(targets, core.Position)
		}
	}

	// Add known enemy cores from previous turns
	for pos := range b.knownEnemyCores {
		found := false
		for _, t := range targets {
			if t == pos {
				found = true
				break
			}
		}
		if !found {
			targets = append(targets, pos)
		}
	}

	// If no targets, explore center of map
	if len(targets) == 0 {
		targets = append(targets, Position{Row: state.Config.Rows / 2, Col: state.Config.Cols / 2})
	}

	return targets
}

func (b *RusherBot) findBestMove(
	start Position,
	targets []Position,
	enemyPositions, wallPositions map[Position]bool,
	config Config,
) Direction {
	// BFS to find shortest path to any target
	type queueItem struct {
		pos      Position
		firstDir Direction
	}

	visited := make(map[Position]bool)
	queue := list.New()
	queue.PushBack(queueItem{pos: start, firstDir: DirNone})
	visited[start] = true

	for queue.Len() > 0 {
		item := queue.Remove(queue.Front()).(queueItem)
		pos := item.pos

		// Check if we've reached a target
		for _, target := range targets {
			if pos == target {
				return item.firstDir
			}
		}

		// Explore neighbors
		for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
			next := simulateMove(pos, dir, config.Rows, config.Cols)

			if visited[next] || wallPositions[next] || enemyPositions[next] {
				continue
			}

			visited[next] = true
			firstDir := item.firstDir
			if firstDir == DirNone {
				firstDir = dir
			}
			queue.PushBack(queueItem{pos: next, firstDir: firstDir})
		}
	}

	// No path found - pick any valid direction
	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		next := simulateMove(start, dir, config.Rows, config.Cols)
		if !wallPositions[next] && !enemyPositions[next] {
			return dir
		}
	}

	return DirN
}

// GuardianBot defends cores with cautious expansion.
type GuardianBot struct {
	rng *rand.Rand
}

// NewGuardianBot creates a new guardian bot.
func NewGuardianBot(seed int64) *GuardianBot {
	return &GuardianBot{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// GetMoves returns moves focused on defense and cautious gathering.
func (b *GuardianBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	// Separate bots
	myBots := make([]VisibleBot, 0)
	enemyBots := make([]VisibleBot, 0)
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		} else {
			enemyBots = append(enemyBots, bot)
		}
	}

	if len(myBots) == 0 {
		return nil, nil
	}

	// Find my cores
	myCores := make([]VisibleCore, 0)
	for _, core := range state.Cores {
		if core.Owner == myID && core.Active {
			myCores = append(myCores, core)
		}
	}

	// Build lookup maps
	enemyPositions := make(map[Position]bool)
	for _, enemy := range enemyBots {
		enemyPositions[enemy.Position] = true
	}

	energyPositions := make(map[Position]bool)
	for _, e := range state.Energy {
		energyPositions[e] = true
	}

	wallPositions := make(map[Position]bool)
	for _, w := range state.Walls {
		wallPositions[w] = true
	}

	moves := make([]Move, 0, len(myBots))
	usedEnergy := make(map[Position]bool)

	for _, bot := range myBots {
		move := b.computeBotMove(bot, myCores, enemyBots, enemyPositions, energyPositions, usedEnergy, wallPositions, config)
		if move != nil {
			moves = append(moves, *move)
		}
	}

	return moves, nil
}

func (b *GuardianBot) computeBotMove(
	bot VisibleBot,
	myCores []VisibleCore,
	enemyBots []VisibleBot,
	enemyPositions, energyPositions, usedEnergy, wallPositions map[Position]bool,
	config Config,
) *Move {
	const perimeterRadius = 5
	const safeZoneRadius = 10

	// Find nearest threatening enemy
	nearestEnemy, nearestEnemyDist := b.findNearestEnemy(bot.Position, enemyBots, config)

	// If enemy is close, intercept
	if nearestEnemy != nil && nearestEnemyDist <= 50 {
		dir := b.getDirectionToward(bot.Position, nearestEnemy.Position, wallPositions, config)
		if dir != DirNone {
			return &Move{Position: bot.Position, Direction: dir}
		}
	}

	// Check if within safe zone of a core
	inSafeZone := false
	var nearestCore *VisibleCore
	nearestCoreDist := math.MaxInt32

	for i := range myCores {
		core := &myCores[i]
		dist := distance2(bot.Position, core.Position, config.Rows, config.Cols)
		if dist < nearestCoreDist {
			nearestCoreDist = dist
			nearestCore = core
		}
		if dist <= safeZoneRadius*safeZoneRadius {
			inSafeZone = true
		}
	}

	// If outside perimeter, move toward nearest core
	if nearestCore != nil && nearestCoreDist > perimeterRadius*perimeterRadius {
		dir := b.getDirectionToward(bot.Position, nearestCore.Position, wallPositions, config)
		if dir != DirNone {
			return &Move{Position: bot.Position, Direction: dir}
		}
	}

	// Gather energy within safe zone
	if inSafeZone {
		// Find nearest energy
		nearestEnergy, nearestEnergyDist := Position{}, math.MaxInt32
		for pos := range energyPositions {
			if usedEnergy[pos] {
				continue
			}
			dist := distance2(bot.Position, pos, config.Rows, config.Cols)
			if dist < nearestEnergyDist {
				nearestEnergyDist = dist
				nearestEnergy = pos
			}
		}

		if nearestEnergyDist < math.MaxInt32 {
			usedEnergy[nearestEnergy] = true
			dir := b.getDirectionToward(bot.Position, nearestEnergy, wallPositions, config)
			if dir != DirNone {
				return &Move{Position: bot.Position, Direction: dir}
			}
		}
	}

	return nil
}

func (b *GuardianBot) findNearestEnemy(pos Position, enemies []VisibleBot, config Config) (*VisibleBot, int) {
	var nearest *VisibleBot
	nearestDist := math.MaxInt32

	for i := range enemies {
		dist := distance2(pos, enemies[i].Position, config.Rows, config.Cols)
		if dist < nearestDist {
			nearestDist = dist
			nearest = &enemies[i]
		}
	}

	return nearest, nearestDist
}

func (b *GuardianBot) getDirectionToward(from, to Position, wallPositions map[Position]bool, config Config) Direction {
	bestDir := DirNone
	bestDist := math.MaxInt32

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		newPos := simulateMove(from, dir, config.Rows, config.Cols)
		if wallPositions[newPos] {
			continue
		}
		dist := distance2(newPos, to, config.Rows, config.Cols)
		if dist < bestDist {
			bestDist = dist
			bestDir = dir
		}
	}

	return bestDir
}

// SwarmBot moves as a coordinated formation.
type SwarmBot struct {
	rng *rand.Rand
}

// NewSwarmBot creates a new swarm bot.
func NewSwarmBot(seed int64) *SwarmBot {
	return &SwarmBot{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// GetMoves returns formation-based moves toward enemies.
func (b *SwarmBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	// Separate bots
	myBots := make([]VisibleBot, 0)
	enemyBots := make([]VisibleBot, 0)
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		} else {
			enemyBots = append(enemyBots, bot)
		}
	}

	if len(myBots) == 0 {
		return nil, nil
	}

	// Build lookup maps
	enemyPositions := make(map[Position]bool)
	for _, enemy := range enemyBots {
		enemyPositions[enemy.Position] = true
	}

	wallPositions := make(map[Position]bool)
	for _, w := range state.Walls {
		wallPositions[w] = true
	}

	myBotPositions := make(map[Position]bool)
	for _, bot := range myBots {
		myBotPositions[bot.Position] = true
	}

	energyPositions := make(map[Position]bool)
	for _, e := range state.Energy {
		energyPositions[e] = true
	}

	// Calculate swarm center
	swarmCenter := b.calculateCenter(myBots, config)

	// Calculate enemy center if visible
	var enemyCenter *Position
	if len(enemyBots) > 0 {
		center := b.calculateCenter(enemyBots, config)
		enemyCenter = &center
	}

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool) // destinations already claimed by a friendly bot this turn

	for _, bot := range myBots {
		move := b.computeBotMove(bot, myBotPositions, enemyPositions, energyPositions, swarmCenter, enemyCenter, wallPositions, claimed, config, len(myBots))
		if move != nil {
			dest := simulateMove(bot.Position, move.Direction, config.Rows, config.Cols)
			claimed[dest] = true
			moves = append(moves, *move)
		} else {
			// Bot holds position — claim its current tile
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

const cohesionRadius2 = 9 // 3 tiles squared

func (b *SwarmBot) calculateCenter(bots []VisibleBot, config Config) Position {
	if len(bots) == 0 {
		return Position{Row: config.Rows / 2, Col: config.Cols / 2}
	}

	// Use circular mean for toroidal coordinates
	sumSinRow, sumCosRow := 0.0, 0.0
	sumSinCol, sumCosCol := 0.0, 0.0

	rowScale := (2 * math.Pi) / float64(config.Rows)
	colScale := (2 * math.Pi) / float64(config.Cols)

	for _, bot := range bots {
		sumSinRow += math.Sin(float64(bot.Position.Row) * rowScale)
		sumCosRow += math.Cos(float64(bot.Position.Row) * rowScale)
		sumSinCol += math.Sin(float64(bot.Position.Col) * colScale)
		sumCosCol += math.Cos(float64(bot.Position.Col) * colScale)
	}

	n := float64(len(bots))
	avgRow := math.Atan2(sumSinRow/n, sumCosRow/n) / rowScale
	avgCol := math.Atan2(sumSinCol/n, sumCosCol/n) / colScale

	row := int(math.Mod(math.Mod(avgRow, float64(config.Rows))+float64(config.Rows), float64(config.Rows)))
	col := int(math.Mod(math.Mod(avgCol, float64(config.Cols))+float64(config.Cols), float64(config.Cols)))

	return Position{Row: row, Col: col}
}

func (b *SwarmBot) computeBotMove(
	bot VisibleBot,
	myBotPositions, enemyPositions, energyPositions map[Position]bool,
	swarmCenter Position,
	enemyCenter *Position,
	wallPositions, claimed map[Position]bool,
	config Config,
	friendlyCount int,
) *Move {
	// Solo mode: when alone or with very few units, gather energy to build the swarm
	if friendlyCount <= 2 {
		return b.soloMove(bot, energyPositions, enemyPositions, wallPositions, config)
	}

	// Target is enemy center if visible, otherwise map center
	target := Position{Row: config.Rows / 2, Col: config.Cols / 2}
	if enemyCenter != nil {
		target = *enemyCenter
	}

	bestDir := DirNone
	bestScore := -math.MaxFloat64

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		newPos := simulateMove(bot.Position, dir, config.Rows, config.Cols)

		// Can't move into walls or enemies
		if wallPositions[newPos] || enemyPositions[newPos] {
			continue
		}

		// CRITICAL: avoid tiles claimed by another friendly bot this turn (prevents self-collision)
		if claimed[newPos] {
			continue
		}

		// Also avoid moving onto a tile occupied by a friendly bot (they might not move)
		if myBotPositions[newPos] && newPos != bot.Position {
			continue
		}

		// Check cohesion: must stay within cohesion radius of at least one friendly bot
		if !b.maintainsCohesion(newPos, bot.Position, myBotPositions, config) {
			continue
		}

		// Score this move
		score := 0.0

		// Prefer moving toward enemy
		distToTarget := float64(distance2(newPos, target, config.Rows, config.Cols))
		currentDistToTarget := float64(distance2(bot.Position, target, config.Rows, config.Cols))
		score += (currentDistToTarget - distToTarget) * 10

		// Prefer staying near swarm center
		distToSwarmCenter := float64(distance2(newPos, swarmCenter, config.Rows, config.Cols))
		score -= distToSwarmCenter * 0.5

		// Bonus for being in attack range
		for enemyPos := range enemyPositions {
			dist := distance2(newPos, enemyPos, config.Rows, config.Cols)
			if dist <= config.AttackRadius2 {
				score += 50
				break
			}
		}

		// Small bonus for energy on the way
		if energyPositions[newPos] {
			score += 15
		}

		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}

	if bestDir != DirNone {
		return &Move{Position: bot.Position, Direction: bestDir}
	}

	return nil
}

// soloMove handles movement when the swarm is too small for formation tactics.
// Gathers energy to spawn more units, avoids enemies.
func (b *SwarmBot) soloMove(
	bot VisibleBot,
	energyPositions, enemyPositions, wallPositions map[Position]bool,
	config Config,
) *Move {
	bestDir := DirNone
	bestScore := -math.MaxFloat64

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		newPos := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if wallPositions[newPos] || enemyPositions[newPos] {
			continue
		}

		score := 0.0

		// Strong bonus for energy
		if energyPositions[newPos] {
			score += 100
		}

		// Move toward nearest energy
		for ePos := range energyPositions {
			dist := float64(distance2(newPos, ePos, config.Rows, config.Cols))
			currentDist := float64(distance2(bot.Position, ePos, config.Rows, config.Cols))
			if dist < currentDist {
				score += 20.0 / (dist + 1)
			}
		}

		// Avoid enemies
		for ePos := range enemyPositions {
			dist := distance2(newPos, ePos, config.Rows, config.Cols)
			if dist <= config.AttackRadius2+4 {
				score -= 200
			}
		}

		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}

	if bestDir != DirNone {
		return &Move{Position: bot.Position, Direction: bestDir}
	}
	return nil
}

func (b *SwarmBot) maintainsCohesion(newPos, oldPos Position, myBotPositions map[Position]bool, config Config) bool {
	for botPos := range myBotPositions {
		if botPos == oldPos {
			continue
		}
		dist := distance2(newPos, botPos, config.Rows, config.Cols)
		if dist <= cohesionRadius2 {
			return true
		}
	}
	return false
}

// HunterBot targets isolated enemy units.
type HunterBot struct {
	rng            *rand.Rand
	enemyTrackers  map[Position]*enemyTracker
}

type enemyTracker struct {
	lastPos    *Position
	currentPos Position
}

// NewHunterBot creates a new hunter bot.
func NewHunterBot(seed int64) *HunterBot {
	return &HunterBot{
		rng:           rand.New(rand.NewSource(seed)),
		enemyTrackers: make(map[Position]*enemyTracker),
	}
}

// GetMoves returns moves targeting isolated enemies.
func (b *HunterBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	// Separate bots
	myBots := make([]VisibleBot, 0)
	enemyBots := make([]VisibleBot, 0)
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		} else {
			enemyBots = append(enemyBots, bot)
		}
	}

	if len(myBots) == 0 {
		return nil, nil
	}

	// Update enemy trackers
	for _, enemy := range enemyBots {
		tracker, exists := b.enemyTrackers[enemy.Position]
		if !exists {
			tracker = &enemyTracker{}
			b.enemyTrackers[enemy.Position] = tracker
		}
		tracker.lastPos = &tracker.currentPos
		tracker.currentPos = enemy.Position
	}

	// Build lookup maps
	enemyPositions := make(map[Position]bool)
	for _, enemy := range enemyBots {
		enemyPositions[enemy.Position] = true
	}

	energyPositions := make(map[Position]bool)
	for _, e := range state.Energy {
		energyPositions[e] = true
	}

	wallPositions := make(map[Position]bool)
	for _, w := range state.Walls {
		wallPositions[w] = true
	}

	myBotPositions := make(map[Position]bool)
	for _, bot := range myBots {
		myBotPositions[bot.Position] = true
	}

	// Find isolated enemies
	isolatedEnemies := b.findIsolatedEnemies(enemyBots, config)

	// Assign hunters to targets
	moves := make([]Move, 0, len(myBots))
	usedEnergy := make(map[Position]bool)
	assignedHunters := make(map[Position]bool)

	// First, assign hunters to isolated enemies
	for _, target := range isolatedEnemies {
		// Assign up to 2 hunters per target
		huntersAssigned := 0
		for i, bot := range myBots {
			if assignedHunters[bot.Position] {
				continue
			}
			if huntersAssigned >= 2 {
				break
			}

			// Check if this bot is close enough to be a hunter
			dist := distance2(bot.Position, target.Position, config.Rows, config.Cols)
			if dist < 400 { // Within ~20 tiles
				predictedPos := b.predictPosition(target)
				dir := b.getDirectionToward(bot.Position, predictedPos, wallPositions, config)
				if dir != DirNone {
					moves = append(moves, Move{Position: bot.Position, Direction: dir})
					assignedHunters[bot.Position] = true
					huntersAssigned++
				}
			}
			_ = i // silence unused variable warning
		}
	}

	// Remaining bots gather or explore
	for _, bot := range myBots {
		if assignedHunters[bot.Position] {
			continue
		}

		// Try to gather energy
		nearestEnergy, nearestDist := Position{}, math.MaxInt32
		for pos := range energyPositions {
			if usedEnergy[pos] {
				continue
			}
			dist := distance2(bot.Position, pos, config.Rows, config.Cols)
			if dist < nearestDist {
				nearestDist = dist
				nearestEnergy = pos
			}
		}

		if nearestDist < math.MaxInt32 {
			usedEnergy[nearestEnergy] = true
			dir := b.getDirectionToward(bot.Position, nearestEnergy, wallPositions, config)
			if dir != DirNone {
				moves = append(moves, Move{Position: bot.Position, Direction: dir})
				continue
			}
		}

		// Explore toward center
		center := Position{Row: config.Rows / 2, Col: config.Cols / 2}
		dir := b.getDirectionToward(bot.Position, center, wallPositions, config)
		if dir != DirNone {
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		}
	}

	return moves, nil
}

const isolationThreshold = 16 // 4 tiles squared

func (b *HunterBot) findIsolatedEnemies(enemies []VisibleBot, config Config) []VisibleBot {
	isolated := make([]VisibleBot, 0)

	for _, bot := range enemies {
		nearestDist := math.MaxInt32
		for _, other := range enemies {
			if bot.Position == other.Position {
				continue
			}
			dist := distance2(bot.Position, other.Position, config.Rows, config.Cols)
			if dist < nearestDist {
				nearestDist = dist
			}
		}

		// Isolated if nearest friendly is >= 4 tiles away or only enemy
		if nearestDist >= isolationThreshold || len(enemies) == 1 {
			isolated = append(isolated, bot)
		}
	}

	return isolated
}

func (b *HunterBot) predictPosition(enemy VisibleBot) Position {
	tracker, exists := b.enemyTrackers[enemy.Position]
	if !exists || tracker.lastPos == nil {
		return enemy.Position
	}

	// Simple prediction: continue in same direction
	dr := tracker.currentPos.Row - tracker.lastPos.Row
	dc := tracker.currentPos.Col - tracker.lastPos.Col

	// Handle wrap
	if dr > 30 {
		dr -= 60
	}
	if dr < -30 {
		dr += 60
	}
	if dc > 30 {
		dc -= 60
	}
	if dc < -30 {
		dc += 60
	}

	return Position{
		Row: (tracker.currentPos.Row + dr + 60) % 60,
		Col: (tracker.currentPos.Col + dc + 60) % 60,
	}
}

func (b *HunterBot) getDirectionToward(from, to Position, wallPositions map[Position]bool, config Config) Direction {
	bestDir := DirNone
	bestDist := math.MaxInt32

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		newPos := simulateMove(from, dir, config.Rows, config.Cols)
		if wallPositions[newPos] {
			continue
		}
		dist := distance2(newPos, to, config.Rows, config.Cols)
		if dist < bestDist {
			bestDist = dist
			bestDir = dir
		}
	}

	return bestDir
}

// Helper functions

// distance2 calculates squared Euclidean distance with toroidal wrapping.
func distance2(a, b Position, rows, cols int) int {
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

// simulateMove returns the new position after moving in a direction.
func simulateMove(pos Position, dir Direction, rows, cols int) Position {
	switch dir {
	case DirN:
		return Position{Row: (pos.Row - 1 + rows) % rows, Col: pos.Col}
	case DirE:
		return Position{Row: pos.Row, Col: (pos.Col + 1) % cols}
	case DirS:
		return Position{Row: (pos.Row + 1) % rows, Col: pos.Col}
	case DirW:
		return Position{Row: pos.Row, Col: (pos.Col - 1 + cols) % cols}
	default:
		return pos
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
