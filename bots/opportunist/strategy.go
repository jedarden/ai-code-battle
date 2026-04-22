package main

import "math"

const (
	engageRadius2    = 25  // ~5 tiles: region considered "local" for numerical advantage
	retreatRadius2   = 9   // flee if enemy within 3 tiles and we're outnumbered
	patrolRadius     = 8   // max distance from core when patrolling
	energySeekRange2 = 100 // ~10 tiles: seek energy within this range
)

// OpportunistStrategy targets the weakest visible enemy — fights only when
// it has local numerical advantage, retreats toward reinforcements otherwise,
// and builds economy during retreats.
type OpportunistStrategy struct{}

func NewOpportunistStrategy() *OpportunistStrategy {
	return &OpportunistStrategy{}
}

// targetInfo describes a scored enemy target.
type targetInfo struct {
	pos        Position
	owner      int
	score      float64 // higher = more attractive
	isolation  float64 // distance to nearest friendly
	localAlly  int     // allies within engageRadius2
	localEnemy int     // enemies within engageRadius2
}

// ComputeMoves assigns each owned bot to attack, retreat, gather energy, or
// patrol near core.
func (s *OpportunistStrategy) ComputeMoves(state *GameState) []Move {
	rows := state.Config.Rows
	cols := state.Config.Cols
	attackR2 := state.Config.AttackRadius2
	myID := state.You.ID

	wallSet := make(map[Position]bool, len(state.Walls))
	for _, w := range state.Walls {
		wallSet[w] = true
	}

	// Separate bots by ownership
	myBots := make([]Position, 0, len(state.Bots))
	myBotSet := make(map[Position]bool)
	enemyBots := make([]VisibleBot, 0)
	enemySet := make(map[Position]bool)

	for _, b := range state.Bots {
		if b.Owner == myID {
			myBots = append(myBots, b.Position)
			myBotSet[b.Position] = true
		} else {
			enemyBots = append(enemyBots, b)
			enemySet[b.Position] = true
		}
	}

	// Identify my active cores
	myCores := make([]Position, 0)
	for _, c := range state.Cores {
		if c.Owner == myID && c.Active {
			myCores = append(myCores, c.Position)
		}
	}

	// Score enemy targets: isolation × low-HP-proxy
	targets := s.scoreTargets(enemyBots, myBots, rows, cols)

	passable := func(p Position) bool {
		return !wallSet[p] && !enemySet[p]
	}

	claimedDests := make(map[Position]bool)
	moves := make([]Move, 0, len(myBots))

	// Assign bots: attackers first (closest to best target), then retreaters, then economy
	attackAssigns := s.assignAttackers(targets, myBots, attackR2, rows, cols)

	for _, bot := range myBots {
		dir := ""

		if assign, ok := attackAssigns[bot]; ok {
			// Attack mode: move toward assigned target
			dir = s.attackMove(bot, assign.targetPos, passable, rows, cols)
		} else if s.shouldFlee(bot, enemyBots, myBots, rows, cols) {
			// Retreat mode: move toward nearest ally cluster
			dir = s.retreatMove(bot, myBots, enemySet, wallSet, rows, cols)
			// Opportunistically grab energy while retreating
			if dir == "" {
				dir = s.energyMove(bot, state.Energy, passable, claimedDests, rows, cols)
			}
		} else {
			// Economy/patrol mode
			dir = s.economyOrPatrol(bot, state.Energy, myCores, passable, claimedDests, rows, cols)
		}

		dest := bot
		if dir != "" {
			dest = simulateMove(bot, dir, rows, cols)
		}

		// Prevent self-collision
		if dir != "" && claimedDests[dest] {
			dir = ""
			dest = bot
		}

		claimedDests[dest] = true
		if dir != "" {
			moves = append(moves, Move{Position: bot, Direction: dir})
		}
	}

	return moves
}

// scoreTargets evaluates each visible enemy and returns them sorted by
// attractiveness (isolation × vulnerability).
func (s *OpportunistStrategy) scoreTargets(enemies []VisibleBot, myBots []Position, rows, cols int) []targetInfo {
	targets := make([]targetInfo, 0, len(enemies))

	for _, e := range enemies {
		// Isolation: distance to nearest friendly (other enemy owned by same player)
		isolation := 0.0
		minFriendly := math.MaxFloat64
		for _, other := range enemies {
			if other.Position == e.Position {
				continue
			}
			if other.Owner == e.Owner {
				d := float64(distance2(e.Position, other.Position, rows, cols))
				if d < minFriendly {
					minFriendly = d
				}
			}
		}
		if minFriendly == math.MaxFloat64 {
			isolation = 10.0
		} else {
			isolation = math.Sqrt(minFriendly)
		}

		// Count local allies and enemies around this target
		localAlly := 0
		localEnemy := 0
		for _, mb := range myBots {
			if distance2(mb, e.Position, rows, cols) <= engageRadius2 {
				localAlly++
			}
		}
		for _, oe := range enemies {
			if distance2(oe.Position, e.Position, rows, cols) <= engageRadius2 {
				localEnemy++
			}
		}

		// Low-HP-proxy: bots that are more isolated are "weaker" targets.
		// If the enemy has few local allies, it's more vulnerable.
		vulnerability := 1.0
		if localEnemy > 0 {
			vulnerability = 1.0 / float64(localEnemy)
		}

		score := isolation * vulnerability

		targets = append(targets, targetInfo{
			pos:        e.Position,
			owner:      e.Owner,
			score:      score,
			isolation:  isolation,
			localAlly:  localAlly,
			localEnemy: localEnemy,
		})
	}

	// Sort by score descending
	for i := 1; i < len(targets); i++ {
		for j := i; j > 0 && targets[j].score > targets[j-1].score; j-- {
			targets[j], targets[j-1] = targets[j-1], targets[j]
		}
	}

	return targets
}

// attackAssign holds the assignment of a bot to an attack target.
type attackAssign struct {
	targetPos Position
}

// assignAttackers determines which bots should attack which targets.
// Only assigns bots when we have local numerical advantage (allies >= enemies)
// in the target's region.
func (s *OpportunistStrategy) assignAttackers(targets []targetInfo, myBots []Position, attackR2 int, rows, cols int) map[Position]attackAssign {
	assignments := make(map[Position]attackAssign)
	assignedBots := make(map[Position]bool)

	for _, tgt := range targets {
		// Only attack if we have numerical advantage in the region
		if tgt.localAlly < tgt.localEnemy {
			continue
		}

		// Find closest unassigned bots to send toward this target
		type botDist struct {
			pos  Position
			dist int
		}
		candidates := make([]botDist, 0)
		for _, mb := range myBots {
			if assignedBots[mb] {
				continue
			}
			d := distance2(mb, tgt.pos, rows, cols)
			// Only consider bots within a reasonable engagement range
			if d <= engageRadius2*2 {
				candidates = append(candidates, botDist{mb, d})
			}
		}

		// Sort candidates by distance (closest first)
		for i := 1; i < len(candidates); i++ {
			for j := i; j > 0 && candidates[j].dist < candidates[j-1].dist; j-- {
				candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
			}
		}

		// Assign enough bots to ensure advantage (send 2 for each enemy in region)
		wantCount := tgt.localEnemy + 1
		if wantCount < 2 {
			wantCount = 2
		}

		assigned := 0
		for _, c := range candidates {
			if assigned >= wantCount {
				break
			}
			assignments[c.pos] = attackAssign{targetPos: tgt.pos}
			assignedBots[c.pos] = true
			assigned++
		}
	}

	return assignments
}

// attackMove moves a bot toward the assigned target position.
// The target itself is treated as passable so BFS can path to it.
func (s *OpportunistStrategy) attackMove(bot, target Position, passable func(Position) bool, rows, cols int) string {
	attackPassable := func(p Position) bool {
		if p == target {
			return true
		}
		return passable(p)
	}
	return BFS(bot, target, attackPassable, rows, cols)
}

// shouldFlee returns true if the bot is near enemies and locally outnumbered.
func (s *OpportunistStrategy) shouldFlee(bot Position, enemies []VisibleBot, myBots []Position, rows, cols int) bool {
	nearbyEnemies := 0
	for _, e := range enemies {
		if distance2(bot, e.Position, rows, cols) <= retreatRadius2 {
			nearbyEnemies++
		}
	}

	if nearbyEnemies == 0 {
		return false
	}

	nearbyAllies := 0
	for _, mb := range myBots {
		if mb == bot {
			continue
		}
		if distance2(bot, mb, rows, cols) <= retreatRadius2 {
			nearbyAllies++
		}
	}

	return nearbyAllies < nearbyEnemies
}

// retreatMove moves toward the nearest cluster of friendly bots while
// maximizing distance from enemies.
func (s *OpportunistStrategy) retreatMove(bot Position, myBots []Position, enemySet, wallSet map[Position]bool, rows, cols int) string {
	bestDir := ""
	bestScore := -1

	for _, step := range cardinalSteps(bot, rows, cols) {
		if wallSet[step.pos] || enemySet[step.pos] {
			continue
		}

		score := 0

		// Move toward nearest friendly bot cluster
		for _, mb := range myBots {
			if mb == bot {
				continue
			}
			d := ToroidalManhattan(step.pos, mb, rows, cols)
			if d > 0 {
				score += 100 / d
			}
		}

		// Maximize distance from all enemies (further is safer)
		for ep := range enemySet {
			d := distance2(step.pos, ep, rows, cols)
			score += d
		}

		if score > bestScore {
			bestScore = score
			bestDir = step.dir
		}
	}

	return bestDir
}

// economyOrPatrol seeks nearby energy or patrols near core.
func (s *OpportunistStrategy) economyOrPatrol(bot Position, energy []Position, cores []Position, passable func(Position) bool, claimedDests map[Position]bool, rows, cols int) string {
	// Try to gather nearby uncontested energy
	dir := s.energyMove(bot, energy, passable, claimedDests, rows, cols)
	if dir != "" {
		return dir
	}

	// Patrol near core
	if len(cores) > 0 {
		nearestCoreDist := math.MaxInt32
		var nearestCore Position
		for _, c := range cores {
			d := distance2(bot, c, rows, cols)
			if d < nearestCoreDist {
				nearestCoreDist = d
				nearestCore = c
			}
		}

		// If far from core, move toward it
		if nearestCoreDist > patrolRadius*patrolRadius {
			dir := BFS(bot, nearestCore, passable, rows, cols)
			if dir != "" {
				return dir
			}
		}
	}

	// Spread out to avoid clustering
	return s.spreadMove(bot, claimedDests, rows, cols)
}

// energyMove seeks the nearest unclaimed, uncontested energy tile.
func (s *OpportunistStrategy) energyMove(bot Position, energy []Position, passable func(Position) bool, claimedDests map[Position]bool, rows, cols int) string {
	bestDist := math.MaxInt32
	var target Position
	found := false

	for _, e := range energy {
		if claimedDests[e] {
			continue
		}
		d := distance2(bot, e, rows, cols)
		if d < bestDist && d <= energySeekRange2 {
			bestDist = d
			target = e
			found = true
		}
	}

	if found {
		return BFS(bot, target, passable, rows, cols)
	}
	return ""
}

// spreadMove picks a direction that maximizes distance from claimed destinations.
func (s *OpportunistStrategy) spreadMove(bot Position, claimedDests map[Position]bool, rows, cols int) string {
	bestDir := ""
	bestScore := -1

	for _, step := range cardinalSteps(bot, rows, cols) {
		if claimedDests[step.pos] {
			continue
		}

		score := 0
		for dest := range claimedDests {
			d := distance2(step.pos, dest, rows, cols)
			if d > 0 {
				score += d
			}
		}

		if score > bestScore {
			bestScore = score
			bestDir = step.dir
		}
	}

	return bestDir
}
