package engine

import (
	"math"
	"math/rand"
	"sort"
)

// ────────────────────────────────────────────────────────────────────────────
// Phase 13 expansion bots — 10 new strategy archetypes
// ────────────────────────────────────────────────────────────────────────────

// DefenderBot hugs own cores and intercepts enemies within a perimeter.
type DefenderBot struct{ rng *rand.Rand }

func NewDefenderBot(seed int64) *DefenderBot {
	return &DefenderBot{rng: rand.New(rand.NewSource(seed))}
}

func (b *DefenderBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	myCores := myActiveCores(state.Cores, myID)
	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)
	energySet := posSetFromPositions(state.Energy)
	coreSet := posSetFromCorePositions(myCores)

	const perimeterRadius2 = 25

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	for _, bot := range myBots {
		nearestEnemy, enemyDist := findNearestPos(bot.Position, enemySet, config)
		_, coreDist := findNearestPos(bot.Position, coreSet, config)

		var dir Direction

		// Priority 1: Intercept nearby enemies
		if nearestEnemy != nil && enemyDist <= 50 {
			dir = moveToward(bot.Position, *nearestEnemy, wallSet, claimed, config)
		}

		// Priority 2: Return to core perimeter if too far
		if dir == DirNone && coreDist > perimeterRadius2 {
			nearestCore, _ := findNearestPos(bot.Position, coreSet, config)
			if nearestCore != nil {
				dir = moveToward(bot.Position, *nearestCore, wallSet, claimed, config)
			}
		}

		// Priority 3: Gather energy within perimeter
		if dir == DirNone && len(energySet) > 0 {
			nearestEnergy, _ := findNearestPos(bot.Position, energySet, config)
			if nearestEnergy != nil {
				dir = moveToward(bot.Position, *nearestEnergy, wallSet, claimed, config)
			}
		}

		// Priority 4: Patrol near core
		if dir == DirNone && len(coreSet) > 0 {
			nearestCore, _ := findNearestPos(bot.Position, coreSet, config)
			if nearestCore != nil {
				dir = moveToward(bot.Position, *nearestCore, wallSet, claimed, config)
			}
		}

		if dir == DirNone {
			dir = randDirection(b.rng)
		}

		dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if !claimed[dest] {
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// ScoutBot maximizes map coverage, avoids combat.
type ScoutBot struct {
	rng  *rand.Rand
	seen map[Position]int
}

func NewScoutBot(seed int64) *ScoutBot {
	return &ScoutBot{
		rng:  rand.New(rand.NewSource(seed)),
		seen: make(map[Position]int),
	}
}

func (b *ScoutBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)

	vr := intSqrt(config.VisionRadius2) + 1
	for _, bot := range myBots {
		for dr := -vr; dr <= vr; dr++ {
			for dc := -vr; dc <= vr; dc++ {
				if dr*dr+dc*dc > config.VisionRadius2 {
					continue
				}
				r := (bot.Position.Row + dr + config.Rows) % config.Rows
				c := (bot.Position.Col + dc + config.Cols) % config.Cols
				b.seen[Position{Row: r, Col: c}] = state.Turn
			}
		}
	}

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	for _, bot := range myBots {
		if shouldFleeFromEnemies(bot.Position, enemySet, config) {
			dir := fleeDirection(bot.Position, enemySet, wallSet, config)
			if dir != DirNone {
				dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
				if !claimed[dest] {
					claimed[dest] = true
					moves = append(moves, Move{Position: bot.Position, Direction: dir})
					continue
				}
			}
		}

		dir := b.bestExploreDir(bot.Position, config, state.Turn, claimed, wallSet)
		if dir == DirNone {
			dir = randDirection(b.rng)
		}

		dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if !claimed[dest] {
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

func (b *ScoutBot) bestExploreDir(pos Position, config Config, turn int, claimed, wallSet map[Position]bool) Direction {
	bestDir := DirNone
	bestScore := -1

	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		score := 0
		for step := 1; step <= 8; step++ {
			dr, dc := directionDelta(dir)
			r := (pos.Row + dr*step + config.Rows) % config.Rows
			c := (pos.Col + dc*step + config.Cols) % config.Cols
			p := Position{Row: r, Col: c}
			if wallSet[p] {
				continue
			}
			lastSeen, ok := b.seen[p]
			if !ok {
				score += turn + 1
			} else {
				staleness := turn - lastSeen
				if staleness > 0 {
					score += staleness
				}
			}
		}
		dest := simulateMove(pos, dir, config.Rows, config.Cols)
		if claimed[dest] || wallSet[dest] {
			score = -1
		}
		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}

	return bestDir
}

// FarmerBot maximizes energy collection, avoids combat entirely.
type FarmerBot struct{ rng *rand.Rand }

func NewFarmerBot(seed int64) *FarmerBot {
	return &FarmerBot{rng: rand.New(rand.NewSource(seed))}
}

func (b *FarmerBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)
	energySet := posSetFromPositions(state.Energy)
	coreSet := posSetFromCorePositions(myActiveCores(state.Cores, myID))

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)
	usedEnergy := make(map[Position]bool)

	for _, bot := range myBots {
		var dir Direction

		if shouldFleeFromEnemies(bot.Position, enemySet, config) {
			dir = fleeDirection(bot.Position, enemySet, wallSet, config)
		}

		if dir == DirNone && len(energySet) > 0 {
			var bestE *Position
			bestDist := int(1e9)
			for e := range energySet {
				if usedEnergy[e] {
					continue
				}
				d := distance2(bot.Position, e, config.Rows, config.Cols)
				if d < bestDist {
					bestDist = d
					eCopy := e
					bestE = &eCopy
				}
			}
			if bestE != nil {
				usedEnergy[*bestE] = true
				dir = moveToward(bot.Position, *bestE, wallSet, claimed, config)
			}
		}

		if dir == DirNone && len(coreSet) > 0 {
			nearestCore, coreDist := findNearestPos(bot.Position, coreSet, config)
			if nearestCore != nil && coreDist > 9 {
				dir = moveToward(bot.Position, *nearestCore, wallSet, claimed, config)
			}
		}

		if dir == DirNone {
			dir = randDirection(b.rng)
		}

		dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if !claimed[dest] && !wallSet[dest] {
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// PacifistBot maximizes distance from enemies, never initiates combat.
type PacifistBot struct{ rng *rand.Rand }

func NewPacifistBot(seed int64) *PacifistBot {
	return &PacifistBot{rng: rand.New(rand.NewSource(seed))}
}

func (b *PacifistBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)
	coreSet := posSetFromCorePositions(myActiveCores(state.Cores, myID))

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	sortBotsByEnemyDist(myBots, enemySet, config)

	for _, bot := range myBots {
		bestDir := DirNone
		bestScore := float64(math.MinInt64)

		for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
			dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
			if wallSet[dest] || claimed[dest] || enemySet[dest] {
				continue
			}

			score := 0.0
			if len(enemySet) > 0 {
				minDist := float64(math.MaxInt32)
				for ep := range enemySet {
					d := float64(distance2(dest, ep, config.Rows, config.Cols))
					if d < minDist {
						minDist = d
					}
				}
				score += minDist * 10
			}

			if isInDanger(bot.Position, enemySet, config) && len(coreSet) > 0 {
				coreDist := float64(distToSet(dest, coreSet, config))
				currentCoreDist := float64(distToSet(bot.Position, coreSet, config))
				score += (currentCoreDist - coreDist) * 15
			}

			if score > bestScore {
				bestScore = score
				bestDir = dir
			}
		}

		if bestDir != DirNone {
			dest := simulateMove(bot.Position, bestDir, config.Rows, config.Cols)
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: bestDir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// PhalanxBot moves in tight formation toward enemies.
type PhalanxBot struct{ rng *rand.Rand }

func NewPhalanxBot(seed int64) *PhalanxBot {
	return &PhalanxBot{rng: rand.New(rand.NewSource(seed))}
}

func (b *PhalanxBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)

	center := circularMeanOf(botsToPositions(myBots), config.Rows, config.Cols)

	target := Position{Row: config.Rows / 2, Col: config.Cols / 2}
	if len(enemyBots) > 0 {
		target = circularMeanOf(botsToPositions(enemyBots), config.Rows, config.Cols)
	}

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	for _, bot := range myBots {
		bestDir := DirNone
		bestScore := float64(math.MinInt64)

		for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
			dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
			if wallSet[dest] || enemySet[dest] || claimed[dest] {
				continue
			}

			score := 0.0
			distToTarget := float64(distance2(dest, target, config.Rows, config.Cols))
			currentDist := float64(distance2(bot.Position, target, config.Rows, config.Cols))
			score += (currentDist - distToTarget) * 10

			distToCenter := float64(distance2(dest, center, config.Rows, config.Cols))
			score -= distToCenter * 0.5

			for ep := range enemySet {
				if distance2(dest, ep, config.Rows, config.Cols) <= config.AttackRadius2 {
					score += 50
					break
				}
			}

			if score > bestScore {
				bestScore = score
				bestDir = dir
			}
		}

		if bestDir != DirNone {
			dest := simulateMove(bot.Position, bestDir, config.Rows, config.Cols)
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: bestDir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// RaiderBot performs hit-and-run attacks on isolated enemies.
type RaiderBot struct {
	rng             *rand.Rand
	engagementTurns map[Position]int
}

func NewRaiderBot(seed int64) *RaiderBot {
	return &RaiderBot{
		rng:             rand.New(rand.NewSource(seed)),
		engagementTurns: make(map[Position]int),
	}
}

func (b *RaiderBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	wallSet := posSetFromPositions(state.Walls)
	energySet := posSetFromPositions(state.Energy)
	isolated := findIsolatedEnemies(enemyBots, config)

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)
	assigned := make(map[Position]bool)

	for _, target := range isolated {
		for _, bot := range myBots {
			if assigned[bot.Position] {
				continue
			}
			d := distance2(bot.Position, target.Position, config.Rows, config.Cols)
			if d < 400 {
				turns := b.engagementTurns[bot.Position]
				var dir Direction
				if turns >= 2 {
					dir = fleeDirection(bot.Position, map[Position]bool{target.Position: true}, wallSet, config)
					if turns >= 4 {
						b.engagementTurns[bot.Position] = 0
					}
				} else {
					dir = moveToward(bot.Position, target.Position, wallSet, claimed, config)
					b.engagementTurns[bot.Position] = turns + 1
				}
				if dir != DirNone {
					dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
					if !claimed[dest] {
						claimed[dest] = true
						moves = append(moves, Move{Position: bot.Position, Direction: dir})
						assigned[bot.Position] = true
						goto nextRaider
					}
				}
			}
		}
	nextRaider:
	}

	for _, bot := range myBots {
		if assigned[bot.Position] {
			continue
		}
		dir := DirNone
		if len(energySet) > 0 {
			nearest, _ := findNearestPos(bot.Position, energySet, config)
			if nearest != nil {
				dir = moveToward(bot.Position, *nearest, wallSet, claimed, config)
			}
		}
		if dir == DirNone {
			center := Position{Row: config.Rows / 2, Col: config.Cols / 2}
			dir = moveToward(bot.Position, center, wallSet, claimed, config)
		}
		if dir == DirNone {
			dir = randDirection(b.rng)
		}
		dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if !claimed[dest] {
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// NomadBot constantly relocates to new regions of the map.
type NomadBot struct {
	rng        *rand.Rand
	target     *Position
	targetTurn int
	arrived    bool
	arriveTurn int
}

func NewNomadBot(seed int64) *NomadBot {
	return &NomadBot{rng: rand.New(rand.NewSource(seed))}
}

func (b *NomadBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)

	centroid := circularMeanOf(botsToPositions(myBots), config.Rows, config.Cols)
	arriveRadius := int(float64(min(config.Rows, config.Cols)) * 0.15)

	needNew := b.target == nil
	if b.arrived && state.Turn-b.arriveTurn >= 10 {
		needNew = true
	}
	if !b.arrived && state.Turn-b.targetTurn >= 40 {
		needNew = true
	}

	if needNew {
		b.target = pickNomadTarget(centroid, config, b.rng)
		b.targetTurn = state.Turn
		b.arrived = false
	}

	if !b.arrived && b.target != nil {
		d := distance2(centroid, *b.target, config.Rows, config.Cols)
		if d <= arriveRadius*arriveRadius {
			b.arrived = true
			b.arriveTurn = state.Turn
		}
	}

	target := b.target
	if target == nil {
		t := Position{Row: config.Rows / 2, Col: config.Cols / 2}
		target = &t
	}

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	for _, bot := range myBots {
		var dir Direction

		if shouldFleeFromEnemies(bot.Position, enemySet, config) {
			dir = fleeDirection(bot.Position, enemySet, wallSet, config)
		}

		if dir == DirNone && b.arrived && len(enemySet) > 0 {
			nearest, dist := findNearestPos(bot.Position, enemySet, config)
			if nearest != nil && dist <= config.AttackRadius2*4 {
				dir = moveToward(bot.Position, *nearest, wallSet, claimed, config)
			}
		}

		if dir == DirNone {
			dir = moveToward(bot.Position, *target, wallSet, claimed, config)
		}

		if dir == DirNone {
			dir = randDirection(b.rng)
		}

		dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if !claimed[dest] && !wallSet[dest] {
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// OpportunistBot targets weakest visible enemy, fights only with numerical advantage.
type OpportunistBot struct{ rng *rand.Rand }

func NewOpportunistBot(seed int64) *OpportunistBot {
	return &OpportunistBot{rng: rand.New(rand.NewSource(seed))}
}

func (b *OpportunistBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)
	energySet := posSetFromPositions(state.Energy)
	coreSet := posSetFromCorePositions(myActiveCores(state.Cores, myID))

	// Find best target: most isolated enemy where we have numerical advantage
	var bestTarget *Position
	bestScore := -1.0
	for _, enemy := range enemyBots {
		isolation := 0.0
		minFriendDist := float64(1e9)
		for _, other := range enemyBots {
			if other.Position == enemy.Position {
				continue
			}
			d := float64(distance2(enemy.Position, other.Position, config.Rows, config.Cols))
			if d < minFriendDist {
				minFriendDist = d
			}
		}
		if minFriendDist == 1e9 {
			isolation = 10.0
		} else {
			isolation = math.Sqrt(minFriendDist)
		}

		localAlly := 0
		localEnemy := 0
		for _, mb := range myBots {
			if distance2(mb.Position, enemy.Position, config.Rows, config.Cols) <= 25 {
				localAlly++
			}
		}
		for _, oe := range enemyBots {
			if distance2(oe.Position, enemy.Position, config.Rows, config.Cols) <= 25 {
				localEnemy++
			}
		}

		vulnerability := 1.0
		if localEnemy > 0 {
			vulnerability = 1.0 / float64(localEnemy)
		}

		score := isolation * vulnerability
		if localAlly >= localEnemy && score > bestScore {
			bestScore = score
			p := enemy.Position
			bestTarget = &p
		}
	}

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	for _, bot := range myBots {
		var dir Direction

		if bestTarget != nil {
			dir = moveToward(bot.Position, *bestTarget, wallSet, claimed, config)
		}

		if dir == DirNone && shouldFleeFromEnemies(bot.Position, enemySet, config) {
			mySet := posSetFromBots(myBots)
			nearestAlly, _ := findNearestPos(bot.Position, mySet, config)
			if nearestAlly != nil {
				dir = moveToward(bot.Position, *nearestAlly, wallSet, claimed, config)
			}
		}

		if dir == DirNone && len(energySet) > 0 {
			nearest, _ := findNearestPos(bot.Position, energySet, config)
			if nearest != nil {
				dir = moveToward(bot.Position, *nearest, wallSet, claimed, config)
			}
		}

		if dir == DirNone && len(coreSet) > 0 {
			nearestCore, _ := findNearestPos(bot.Position, coreSet, config)
			if nearestCore != nil {
				dir = moveToward(bot.Position, *nearestCore, wallSet, claimed, config)
			}
		}

		if dir == DirNone {
			dir = randDirection(b.rng)
		}

		dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if !claimed[dest] && !wallSet[dest] {
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// AssassinBot rushes enemy cores exclusively, ignoring enemy bots.
type AssassinBot struct {
	rng        *rand.Rand
	knownCores map[Position]bool
}

func NewAssassinBot(seed int64) *AssassinBot {
	return &AssassinBot{
		rng:        rand.New(rand.NewSource(seed)),
		knownCores: make(map[Position]bool),
	}
}

func (b *AssassinBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots := part.friendly
	if len(myBots) == 0 {
		return nil, nil
	}

	wallSet := posSetFromPositions(state.Walls)

	for _, core := range state.Cores {
		if core.Owner != myID && core.Active {
			b.knownCores[core.Position] = true
		}
	}

	var target *Position
	if len(b.knownCores) > 0 {
		center := circularMeanOf(botsToPositions(myBots), config.Rows, config.Cols)
		bestDist := int(1e9)
		for core := range b.knownCores {
			d := distance2(center, core, config.Rows, config.Cols)
			if d < bestDist {
				bestDist = d
				p := core
				target = &p
			}
		}
	}

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	for _, bot := range myBots {
		var dir Direction
		if target != nil {
			dir = moveToward(bot.Position, *target, wallSet, claimed, config)
		}
		if dir == DirNone {
			opposite := Position{Row: config.Rows - bot.Position.Row, Col: config.Cols - bot.Position.Col}
			dir = moveToward(bot.Position, opposite, wallSet, claimed, config)
		}
		if dir == DirNone {
			dir = randDirection(b.rng)
		}

		dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
		if !claimed[dest] {
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// KamikazeBot aggressively attacks nearest enemies at all costs.
type KamikazeBot struct{ rng *rand.Rand }

func NewKamikazeBot(seed int64) *KamikazeBot {
	return &KamikazeBot{rng: rand.New(rand.NewSource(seed))}
}

func (b *KamikazeBot) GetMoves(state *VisibleState) ([]Move, error) {
	myID := state.You.ID
	config := state.Config

	part := partitionBots(state.Bots, myID)
	myBots, enemyBots := part.friendly, part.enemy
	if len(myBots) == 0 {
		return nil, nil
	}

	enemySet := posSetFromBots(enemyBots)
	wallSet := posSetFromPositions(state.Walls)
	energySet := posSetFromPositions(state.Energy)

	moves := make([]Move, 0, len(myBots))
	claimed := make(map[Position]bool)

	sortBotsByEnemyDist(myBots, enemySet, config)

	for _, bot := range myBots {
		nearestEnemy, _ := findNearestPos(bot.Position, enemySet, config)

		bestDir := DirNone
		bestScore := float64(math.MinInt64)

		for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
			dest := simulateMove(bot.Position, dir, config.Rows, config.Cols)
			if wallSet[dest] || claimed[dest] {
				continue
			}

			score := 0.0

			if nearestEnemy != nil {
				distToEnemy := float64(distance2(dest, *nearestEnemy, config.Rows, config.Cols))
				currentDist := float64(distance2(bot.Position, *nearestEnemy, config.Rows, config.Cols))
				score -= distToEnemy * 100
				if distToEnemy <= float64(config.AttackRadius2) {
					score += 200
				}
				score += (currentDist - distToEnemy) * 50
			} else {
				enemyCores := enemyActiveCorePositions(state.Cores, myID)
				if len(enemyCores) > 0 {
					nearestCore, _ := findNearestPos(dest, enemyCores, config)
					if nearestCore != nil {
						coreDist := float64(distance2(dest, *nearestCore, config.Rows, config.Cols))
						score -= coreDist * 100
					}
				}
			}

			if energySet[dest] {
				score += 5
			}

			if score > bestScore {
				bestScore = score
				bestDir = dir
			}
		}

		if bestDir != DirNone {
			dest := simulateMove(bot.Position, bestDir, config.Rows, config.Cols)
			claimed[dest] = true
			moves = append(moves, Move{Position: bot.Position, Direction: bestDir})
		} else {
			claimed[bot.Position] = true
		}
	}

	return moves, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Phase 13 shared helpers
// ────────────────────────────────────────────────────────────────────────────

type botPartition struct {
	friendly []VisibleBot
	enemy    []VisibleBot
}

func partitionBots(bots []VisibleBot, myID int) botPartition {
	var friendly, enemy []VisibleBot
	for _, b := range bots {
		if b.Owner == myID {
			friendly = append(friendly, b)
		} else {
			enemy = append(enemy, b)
		}
	}
	return botPartition{friendly: friendly, enemy: enemy}
}

func myActiveCores(cores []VisibleCore, myID int) []VisibleCore {
	var result []VisibleCore
	for _, c := range cores {
		if c.Owner == myID && c.Active {
			result = append(result, c)
		}
	}
	return result
}

func posSetFromBots(bots []VisibleBot) map[Position]bool {
	m := make(map[Position]bool, len(bots))
	for _, b := range bots {
		m[b.Position] = true
	}
	return m
}

func posSetFromPositions(positions []Position) map[Position]bool {
	m := make(map[Position]bool, len(positions))
	for _, p := range positions {
		m[p] = true
	}
	return m
}

func posSetFromCorePositions(cores []VisibleCore) map[Position]bool {
	m := make(map[Position]bool, len(cores))
	for _, c := range cores {
		m[c.Position] = true
	}
	return m
}

func enemyActiveCorePositions(cores []VisibleCore, myID int) map[Position]bool {
	m := make(map[Position]bool)
	for _, c := range cores {
		if c.Owner != myID && c.Active {
			m[c.Position] = true
		}
	}
	return m
}

func findNearestPos(pos Position, targets map[Position]bool, config Config) (*Position, int) {
	var best *Position
	bestDist := int(1e9)
	for t := range targets {
		d := distance2(pos, t, config.Rows, config.Cols)
		if d < bestDist {
			bestDist = d
			p := t
			best = &p
		}
	}
	return best, bestDist
}

func moveToward(from, to Position, wallSet, claimed map[Position]bool, config Config) Direction {
	bestDir := DirNone
	bestDist := int(1e9)
	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		dest := simulateMove(from, dir, config.Rows, config.Cols)
		if wallSet[dest] || claimed[dest] {
			continue
		}
		d := distance2(dest, to, config.Rows, config.Cols)
		if d < bestDist {
			bestDist = d
			bestDir = dir
		}
	}
	return bestDir
}

func shouldFleeFromEnemies(pos Position, enemySet map[Position]bool, config Config) bool {
	for ep := range enemySet {
		if distance2(pos, ep, config.Rows, config.Cols) <= config.AttackRadius2+9 {
			return true
		}
	}
	return false
}

func isInDanger(pos Position, enemySet map[Position]bool, config Config) bool {
	for ep := range enemySet {
		if distance2(pos, ep, config.Rows, config.Cols) <= config.AttackRadius2 {
			return true
		}
	}
	return false
}

func fleeDirection(pos Position, enemySet, wallSet map[Position]bool, config Config) Direction {
	bestDir := DirNone
	bestMinDist := -1
	for _, dir := range []Direction{DirN, DirE, DirS, DirW} {
		dest := simulateMove(pos, dir, config.Rows, config.Cols)
		if wallSet[dest] {
			continue
		}
		minDist := int(1e9)
		for ep := range enemySet {
			d := distance2(dest, ep, config.Rows, config.Cols)
			if d < minDist {
				minDist = d
			}
		}
		if minDist > bestMinDist {
			bestMinDist = minDist
			bestDir = dir
		}
	}
	return bestDir
}

func randDirection(rng *rand.Rand) Direction {
	dirs := []Direction{DirN, DirE, DirS, DirW}
	return dirs[rng.Intn(4)]
}

func circularMeanOf(positions []Position, rows, cols int) Position {
	if len(positions) == 0 {
		return Position{Row: rows / 2, Col: cols / 2}
	}
	rowScale := 2.0 * math.Pi / float64(rows)
	colScale := 2.0 * math.Pi / float64(cols)
	var sumSinR, sumCosR, sumSinC, sumCosC float64
	for _, p := range positions {
		sumSinR += math.Sin(float64(p.Row) * rowScale)
		sumCosR += math.Cos(float64(p.Row) * rowScale)
		sumSinC += math.Sin(float64(p.Col) * colScale)
		sumCosC += math.Cos(float64(p.Col) * colScale)
	}
	n := float64(len(positions))
	r := math.Atan2(sumSinR/n, sumCosR/n) / rowScale
	c := math.Atan2(sumSinC/n, sumCosC/n) / colScale
	return Position{
		Row: int(math.Mod(math.Mod(r, float64(rows))+float64(rows), float64(rows))),
		Col: int(math.Mod(math.Mod(c, float64(cols))+float64(cols), float64(cols))),
	}
}

func botsToPositions(bots []VisibleBot) []Position {
	positions := make([]Position, len(bots))
	for i, b := range bots {
		positions[i] = b.Position
	}
	return positions
}

func findIsolatedEnemies(enemies []VisibleBot, config Config) []VisibleBot {
	var isolated []VisibleBot
	for _, bot := range enemies {
		minDist := int(1e9)
		for _, other := range enemies {
			if bot.Position == other.Position {
				continue
			}
			d := distance2(bot.Position, other.Position, config.Rows, config.Cols)
			if d < minDist {
				minDist = d
			}
		}
		if minDist >= 16 || len(enemies) == 1 {
			isolated = append(isolated, bot)
		}
	}
	return isolated
}

func pickNomadTarget(centroid Position, config Config, rng *rand.Rand) *Position {
	candidates := []Position{
		{Row: config.Rows / 5, Col: config.Cols / 5},
		{Row: config.Rows / 5, Col: 4 * config.Cols / 5},
		{Row: 4 * config.Rows / 5, Col: config.Cols / 5},
		{Row: 4 * config.Rows / 5, Col: 4 * config.Cols / 5},
		{Row: (centroid.Row + config.Rows/2) % config.Rows, Col: (centroid.Col + config.Cols/2) % config.Cols},
		{Row: 0, Col: config.Cols / 2},
		{Row: config.Rows - 1, Col: config.Cols / 2},
		{Row: config.Rows / 2, Col: 0},
		{Row: config.Rows / 2, Col: config.Cols - 1},
	}
	p := candidates[rng.Intn(len(candidates))]
	return &p
}

func sortBotsByEnemyDist(bots []VisibleBot, enemySet map[Position]bool, config Config) {
	sort.Slice(bots, func(i, j int) bool {
		di := distToSet(bots[i].Position, enemySet, config)
		dj := distToSet(bots[j].Position, enemySet, config)
		return di < dj
	})
}

func distToSet(pos Position, targets map[Position]bool, config Config) int {
	minDist := int(1e9)
	for t := range targets {
		d := distance2(pos, t, config.Rows, config.Cols)
		if d < minDist {
			minDist = d
		}
	}
	return minDist
}

func intSqrt(x int) int {
	if x <= 0 {
		return 0
	}
	r := 0
	for r*r <= x {
		r++
	}
	return r - 1
}

func directionDelta(d Direction) (int, int) {
	switch d {
	case DirN:
		return -1, 0
	case DirS:
		return 1, 0
	case DirE:
		return 0, 1
	case DirW:
		return 0, -1
	default:
		return 0, 0
	}
}
