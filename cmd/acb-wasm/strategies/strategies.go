// Package strategies provides the built-in ACB bot strategies for use in
// WASM builds. Each strategy implements engine.BotInterface.
//
// The first six (gatherer, rusher, guardian, swarm, hunter, random) are the
// original sandbox roster. The extended roster ports the ladder bots per plan
// §13.1: farmer, opportunist, siege, and economist from the Go/Python ladder
// sources under bots/, and assassin, phalanx, and zone-driver from the Rust
// ladder sources. The ports are behaviorally equivalent — same BFS, same
// heuristics, same priorities — not identical code.
package strategies

import (
	"math"
	"math/rand"
	"sort"

	"github.com/aicodebattle/acb/engine"
)

// New returns a BotInterface for the named strategy.
// Unknown names fall back to random.
func New(name string, rng *rand.Rand) engine.BotInterface {
	switch name {
	case "gatherer":
		return NewGatherer(rng)
	case "rusher":
		return NewRusher(rng)
	case "guardian":
		return NewGuardian(rng)
	case "swarm":
		return NewSwarm(rng)
	case "hunter":
		return NewHunter(rng)
	case "farmer":
		return NewFarmer()
	case "opportunist":
		return NewOpportunist()
	case "siege":
		return NewSiege()
	case "economist":
		return NewEconomist()
	case "assassin":
		return NewAssassin()
	case "phalanx":
		return NewPhalanx()
	case "zone-driver":
		return NewZoneDriver()
	default:
		return engine.NewRandomBot(rng.Int63())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// GathererBot – energy-focused, avoids combat
// ────────────────────────────────────────────────────────────────────────────

type Gatherer struct{ rng *rand.Rand }

func NewGatherer(rng *rand.Rand) *Gatherer { return &Gatherer{rng: rng} }

func (b *Gatherer) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	myID := state.You.ID
	energySet := posSet(state.Energy)
	enemySet := enemyPositions(state.Bots, myID)
	var moves []engine.Move
	for _, bot := range state.Bots {
		if bot.Owner != myID {
			continue
		}
		dir := fleeDir(bot.Position, enemySet, state.Config)
		if dir == engine.DirNone {
			dir = towardNearest(bot.Position, energySet, state.Config)
		}
		if dir == engine.DirNone {
			dir = randDir(b.rng)
		}
		moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
	}
	return moves, nil
}

// ────────────────────────────────────────────────────────────────────────────
// RusherBot – attacks enemy cores and bots aggressively
// ────────────────────────────────────────────────────────────────────────────

type Rusher struct{ rng *rand.Rand }

func NewRusher(rng *rand.Rand) *Rusher { return &Rusher{rng: rng} }

func (b *Rusher) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	myID := state.You.ID
	coreSet := make(map[engine.Position]bool)
	for _, c := range state.Cores {
		if c.Owner != myID && c.Active {
			coreSet[c.Position] = true
		}
	}
	enemySet := enemyPositions(state.Bots, myID)
	var moves []engine.Move
	for _, bot := range state.Bots {
		if bot.Owner != myID {
			continue
		}
		var dir engine.Direction
		if len(coreSet) > 0 {
			dir = towardNearest(bot.Position, coreSet, state.Config)
		} else {
			dir = towardNearest(bot.Position, enemySet, state.Config)
		}
		if dir == engine.DirNone {
			dir = randDir(b.rng)
		}
		moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
	}
	return moves, nil
}

// ────────────────────────────────────────────────────────────────────────────
// GuardianBot – defends own cores
// ────────────────────────────────────────────────────────────────────────────

type Guardian struct{ rng *rand.Rand }

func NewGuardian(rng *rand.Rand) *Guardian { return &Guardian{rng: rng} }

func (b *Guardian) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	myID := state.You.ID
	myCoreSet := make(map[engine.Position]bool)
	for _, c := range state.Cores {
		if c.Owner == myID && c.Active {
			myCoreSet[c.Position] = true
		}
	}
	enemySet := enemyPositions(state.Bots, myID)
	var moves []engine.Move
	for _, bot := range state.Bots {
		if bot.Owner != myID {
			continue
		}
		var dir engine.Direction
		if isNear(bot.Position, enemySet, state.Config, state.Config.AttackRadius2+4) {
			dir = towardNearest(bot.Position, enemySet, state.Config)
		} else {
			dir = towardNearest(bot.Position, myCoreSet, state.Config)
		}
		if dir == engine.DirNone {
			dir = randDir(b.rng)
		}
		moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
	}
	return moves, nil
}

// ────────────────────────────────────────────────────────────────────────────
// SwarmBot – spreads to maximise map coverage
// ────────────────────────────────────────────────────────────────────────────

type Swarm struct{ rng *rand.Rand }

func NewSwarm(rng *rand.Rand) *Swarm { return &Swarm{rng: rng} }

func (b *Swarm) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	myID := state.You.ID
	dirs := []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW}
	var moves []engine.Move
	for _, bot := range state.Bots {
		if bot.Owner != myID {
			continue
		}
		best, bestScore := engine.DirNone, -1
		for _, d := range dirs {
			np := applyDir(bot.Position, d, state.Config)
			score := 0
			for _, other := range state.Bots {
				if other.Owner == myID {
					score += dist2(np, other.Position, state.Config)
				}
			}
			if best == engine.DirNone || score > bestScore {
				bestScore = score
				best = d
			}
		}
		moves = append(moves, engine.Move{Position: bot.Position, Direction: best})
	}
	return moves, nil
}

// ────────────────────────────────────────────────────────────────────────────
// HunterBot – hunts nearest enemy bot
// ────────────────────────────────────────────────────────────────────────────

type Hunter struct{ rng *rand.Rand }

func NewHunter(rng *rand.Rand) *Hunter { return &Hunter{rng: rng} }

func (b *Hunter) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	myID := state.You.ID
	enemySet := enemyPositions(state.Bots, myID)
	energySet := posSet(state.Energy)
	var moves []engine.Move
	for _, bot := range state.Bots {
		if bot.Owner != myID {
			continue
		}
		var dir engine.Direction
		if len(enemySet) > 0 {
			dir = towardNearest(bot.Position, enemySet, state.Config)
		} else {
			dir = towardNearest(bot.Position, energySet, state.Config)
		}
		if dir == engine.DirNone {
			dir = randDir(b.rng)
		}
		moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
	}
	return moves, nil
}

// ────────────────────────────────────────────────────────────────────────────
// FarmerBot – maximizes energy collection and spawn rate, avoids combat
// (port of bots/farmer/strategy.go)
// ────────────────────────────────────────────────────────────────────────────

type Farmer struct{}

func NewFarmer() *Farmer { return &Farmer{} }

func (b *Farmer) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	cfg := state.Config
	myID := state.You.ID

	wallSet := posSet(state.Walls)
	enemySet := enemyPositions(state.Bots, myID)
	enemyList := make([]engine.Position, 0)
	for _, bot := range state.Bots {
		if bot.Owner != myID {
			enemyList = append(enemyList, bot.Position)
		}
	}

	var myCores []engine.Position
	for _, c := range state.Cores {
		if c.Owner == myID && c.Active {
			myCores = append(myCores, c.Position)
		}
	}

	// Energy tiles adjacent (≤√2) to an enemy are contested — collecting there
	// destroys the energy node instead.
	contestedEnergy := make(map[engine.Position]bool)
	for _, e := range state.Energy {
		for _, ep := range enemyList {
			if dist2(e, ep, cfg) <= 2 {
				contestedEnergy[e] = true
				break
			}
		}
	}

	myBots := make([]engine.VisibleBot, 0, len(state.Bots))
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		}
	}

	assignedEnergy := make(map[engine.Position]bool)
	claimedDests := make(map[engine.Position]bool)

	// Process bots closest to uncontested energy first.
	botScores := make([]int, len(myBots))
	for i, bot := range myBots {
		bestDist := math.MaxInt32
		for _, e := range state.Energy {
			if contestedEnergy[e] {
				continue
			}
			if d := dist2(bot.Position, e, cfg); d < bestDist {
				bestDist = d
			}
		}
		botScores[i] = bestDist
	}
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

	moves := make([]engine.Move, 0, len(myBots))
	for _, idx := range sorted {
		bot := myBots[idx]
		dir := farmerBotMove(bot.Position, state, wallSet, enemyList, enemySet,
			myCores, contestedEnergy, assignedEnergy, claimedDests, cfg)

		dest := bot.Position
		if dir != engine.DirNone {
			dest = applyDir(bot.Position, dir, cfg)
		}
		if dir != engine.DirNone && claimedDests[dest] {
			dir = engine.DirNone
			dest = bot.Position
		}
		claimedDests[dest] = true
		if dir != engine.DirNone {
			moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
		}
	}
	return moves, nil
}

func farmerBotMove(pos engine.Position, state *engine.VisibleState,
	wallSet map[engine.Position]bool, enemyList []engine.Position, enemySet map[engine.Position]bool,
	myCores []engine.Position, contestedEnergy, assignedEnergy, claimedDests map[engine.Position]bool,
	cfg engine.Config) engine.Direction {

	passable := func(p engine.Position) bool {
		return !wallSet[p] && !enemySet[p]
	}

	// Priority 1: flee when locally outnumbered within attack range.
	if farmerShouldFlee(pos, state.Bots, state.You.ID, cfg) {
		if dir := maximizeMinDist(pos, enemyList, wallSet, enemySet, cfg); dir != engine.DirNone {
			return dir
		}
	}

	// Priority 2: seek nearest uncontested, unassigned energy.
	bestDist := math.MaxInt32
	var bestEnergy engine.Position
	found := false
	for _, e := range state.Energy {
		if contestedEnergy[e] || assignedEnergy[e] {
			continue
		}
		if d := dist2(pos, e, cfg); d < bestDist {
			bestDist = d
			bestEnergy = e
			found = true
		}
	}
	if found {
		assignedEnergy[bestEnergy] = true
		if dir := bfsDir(pos, bestEnergy, passable, cfg); dir != engine.DirNone {
			return dir
		}
	}

	// Priority 3: already standing on energy — hold to collect.
	for _, e := range state.Energy {
		if e == pos {
			return engine.DirNone
		}
	}

	// Priority 4: move toward nearest energy, even contested.
	if len(state.Energy) > 0 {
		bestDist = math.MaxInt32
		var target engine.Position
		for _, e := range state.Energy {
			if d := dist2(pos, e, cfg); d < bestDist {
				bestDist = d
				target = e
			}
		}
		if dir := bfsDir(pos, target, passable, cfg); dir != engine.DirNone {
			return dir
		}
	}

	// Priority 5: stay near an active core for spawning.
	if len(myCores) > 0 {
		nearestCoreDist := math.MaxInt32
		var nearestCore engine.Position
		for _, c := range myCores {
			if d := dist2(pos, c, cfg); d < nearestCoreDist {
				nearestCoreDist = d
				nearestCore = c
			}
		}
		if nearestCoreDist > 4 {
			if dir := bfsDir(pos, nearestCore, passable, cfg); dir != engine.DirNone {
				return dir
			}
		}
	}

	// Priority 6: spread out from friendly bots.
	return spreadFromBots(pos, state.Bots, state.You.ID, claimedDests, cfg)
}

func farmerShouldFlee(pos engine.Position, bots []engine.VisibleBot, myID int, cfg engine.Config) bool {
	nearbyEnemies, nearbyAllies := 0, 0
	for _, b := range bots {
		if b.Position == pos {
			continue
		}
		if dist2(pos, b.Position, cfg) <= cfg.AttackRadius2 {
			if b.Owner == myID {
				nearbyAllies++
			} else {
				nearbyEnemies++
			}
		}
	}
	return nearbyEnemies > 0 && nearbyAllies < nearbyEnemies
}

// maximizeMinDist picks the cardinal step that maximizes distance from the
// nearest enemy (the farmer/opportunist flee direction).
func maximizeMinDist(pos engine.Position, enemies []engine.Position,
	wallSet, enemySet map[engine.Position]bool, cfg engine.Config) engine.Direction {
	bestDir, bestMinDist := engine.DirNone, -1
	for _, st := range cardinalSteps(pos, cfg) {
		if wallSet[st.pos] || enemySet[st.pos] {
			continue
		}
		minDist := math.MaxInt32
		for _, ep := range enemies {
			if d := dist2(st.pos, ep, cfg); d < minDist {
				minDist = d
			}
		}
		if minDist > bestMinDist {
			bestMinDist = minDist
			bestDir = st.dir
		}
	}
	return bestDir
}

// spreadFromBots picks a direction that maximizes distance to the nearest
// friendly bot (farmer's spread step).
func spreadFromBots(pos engine.Position, bots []engine.VisibleBot, myID int,
	claimedDests map[engine.Position]bool, cfg engine.Config) engine.Direction {
	bestDir, bestScore := engine.DirNone, -1
	for _, st := range cardinalSteps(pos, cfg) {
		if claimedDests[st.pos] {
			continue
		}
		minDist := math.MaxInt32
		for _, b := range bots {
			if b.Owner != myID {
				continue
			}
			if d := dist2(st.pos, b.Position, cfg); d < minDist {
				minDist = d
			}
		}
		if minDist > bestScore {
			bestScore = minDist
			bestDir = st.dir
		}
	}
	return bestDir
}

// ────────────────────────────────────────────────────────────────────────────
// OpportunistBot – targets the weakest visible enemy, fights only with local
// numerical advantage, retreats and farms otherwise
// (port of bots/opportunist/strategy.go)
// ────────────────────────────────────────────────────────────────────────────

const (
	oppEngageRadius2    = 25  // ~5 tiles: region considered "local" for numerical advantage
	oppRetreatRadius2   = 9   // flee if enemy within 3 tiles and we're outnumbered
	oppPatrolRadius     = 8   // max distance from core when patrolling
	oppEnergySeekRange2 = 100 // ~10 tiles: seek energy within this range
)

type Opportunist struct{}

func NewOpportunist() *Opportunist { return &Opportunist{} }

type oppTarget struct {
	pos        engine.Position
	score      float64
	localAlly  int
	localEnemy int
}

func (b *Opportunist) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	cfg := state.Config
	myID := state.You.ID

	wallSet := posSet(state.Walls)
	myBots := make([]engine.Position, 0, len(state.Bots))
	var enemyBots []engine.VisibleBot
	enemySet := make(map[engine.Position]bool)
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot.Position)
		} else {
			enemyBots = append(enemyBots, bot)
			enemySet[bot.Position] = true
		}
	}

	var myCores []engine.Position
	for _, c := range state.Cores {
		if c.Owner == myID && c.Active {
			myCores = append(myCores, c.Position)
		}
	}

	targets := oppScoreTargets(enemyBots, myBots, cfg)
	passable := func(p engine.Position) bool { return !wallSet[p] && !enemySet[p] }

	claimedDests := make(map[engine.Position]bool)
	assignments := oppAssignAttackers(targets, myBots, cfg)
	moves := make([]engine.Move, 0, len(myBots))

	for _, bot := range myBots {
		var dir engine.Direction
		if target, ok := assignments[bot]; ok {
			dir = bfsDir(bot, target, func(p engine.Position) bool {
				return p == target || passable(p)
			}, cfg)
		} else if oppShouldFlee(bot, enemyBots, myBots, cfg) {
			dir = oppRetreatMove(bot, myBots, enemySet, wallSet, cfg)
			if dir == engine.DirNone {
				dir = oppEnergyMove(bot, state.Energy, passable, claimedDests, cfg)
			}
		} else {
			dir = oppEconomyOrPatrol(bot, state.Energy, myCores, passable, claimedDests, cfg)
		}

		dest := bot
		if dir != engine.DirNone {
			dest = applyDir(bot, dir, cfg)
		}
		if dir != engine.DirNone && claimedDests[dest] {
			dir = engine.DirNone
			dest = bot
		}
		claimedDests[dest] = true
		if dir != engine.DirNone {
			moves = append(moves, engine.Move{Position: bot, Direction: dir})
		}
	}
	return moves, nil
}

// oppScoreTargets evaluates each visible enemy: isolation × vulnerability,
// sorted descending by attractiveness.
func oppScoreTargets(enemies []engine.VisibleBot, myBots []engine.Position, cfg engine.Config) []oppTarget {
	targets := make([]oppTarget, 0, len(enemies))
	for _, e := range enemies {
		isolation := 10.0
		minFriendly := math.MaxFloat64
		for _, other := range enemies {
			if other.Position == e.Position || other.Owner != e.Owner {
				continue
			}
			if d := float64(dist2(e.Position, other.Position, cfg)); d < minFriendly {
				minFriendly = d
			}
		}
		if minFriendly != math.MaxFloat64 {
			isolation = math.Sqrt(minFriendly)
		}

		localAlly, localEnemy := 0, 0
		for _, mb := range myBots {
			if dist2(mb, e.Position, cfg) <= oppEngageRadius2 {
				localAlly++
			}
		}
		for _, oe := range enemies {
			if dist2(oe.Position, e.Position, cfg) <= oppEngageRadius2 {
				localEnemy++
			}
		}

		vulnerability := 1.0
		if localEnemy > 0 {
			vulnerability = 1.0 / float64(localEnemy)
		}

		targets = append(targets, oppTarget{
			pos:        e.Position,
			score:      isolation * vulnerability,
			localAlly:  localAlly,
			localEnemy: localEnemy,
		})
	}
	// Sort by score descending (insertion sort, small arrays).
	for i := 1; i < len(targets); i++ {
		for j := i; j > 0 && targets[j].score > targets[j-1].score; j-- {
			targets[j], targets[j-1] = targets[j-1], targets[j]
		}
	}
	return targets
}

// oppAssignAttackers sends bots toward targets only where we hold local
// numerical advantage; each target gets localEnemy+1 (min 2) attackers.
func oppAssignAttackers(targets []oppTarget, myBots []engine.Position, cfg engine.Config) map[engine.Position]engine.Position {
	assignments := make(map[engine.Position]engine.Position)
	assigned := make(map[engine.Position]bool)

	for _, tgt := range targets {
		if tgt.localAlly < tgt.localEnemy {
			continue
		}
		type cand struct {
			pos  engine.Position
			dist int
		}
		candidates := make([]cand, 0)
		for _, mb := range myBots {
			if assigned[mb] {
				continue
			}
			if d := dist2(mb, tgt.pos, cfg); d <= oppEngageRadius2*2 {
				candidates = append(candidates, cand{mb, d})
			}
		}
		for i := 1; i < len(candidates); i++ {
			for j := i; j > 0 && candidates[j].dist < candidates[j-1].dist; j-- {
				candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
			}
		}

		wantCount := tgt.localEnemy + 1
		if wantCount < 2 {
			wantCount = 2
		}
		for i, c := range candidates {
			if i >= wantCount {
				break
			}
			assignments[c.pos] = tgt.pos
			assigned[c.pos] = true
		}
	}
	return assignments
}

func oppShouldFlee(bot engine.Position, enemies []engine.VisibleBot, myBots []engine.Position, cfg engine.Config) bool {
	nearbyEnemies := 0
	for _, e := range enemies {
		if dist2(bot, e.Position, cfg) <= oppRetreatRadius2 {
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
		if dist2(bot, mb, cfg) <= oppRetreatRadius2 {
			nearbyAllies++
		}
	}
	return nearbyAllies < nearbyEnemies
}

// oppRetreatMove scores each step by attraction to friendly clusters
// (100/Manhattan each) plus squared distance from every enemy.
func oppRetreatMove(bot engine.Position, myBots []engine.Position,
	enemySet, wallSet map[engine.Position]bool, cfg engine.Config) engine.Direction {
	bestDir, bestScore := engine.DirNone, -1
	for _, st := range cardinalSteps(bot, cfg) {
		if wallSet[st.pos] || enemySet[st.pos] {
			continue
		}
		score := 0
		for _, mb := range myBots {
			if mb == bot {
				continue
			}
			if d := torManhattan(st.pos, mb, cfg); d > 0 {
				score += 100 / d
			}
		}
		for ep := range enemySet {
			score += dist2(st.pos, ep, cfg)
		}
		if score > bestScore {
			bestScore = score
			bestDir = st.dir
		}
	}
	return bestDir
}

func oppEconomyOrPatrol(bot engine.Position, energy, cores []engine.Position,
	passable func(engine.Position) bool, claimedDests map[engine.Position]bool, cfg engine.Config) engine.Direction {
	if dir := oppEnergyMove(bot, energy, passable, claimedDests, cfg); dir != engine.DirNone {
		return dir
	}
	if len(cores) > 0 {
		nearestCoreDist := math.MaxInt32
		var nearestCore engine.Position
		for _, c := range cores {
			if d := dist2(bot, c, cfg); d < nearestCoreDist {
				nearestCoreDist = d
				nearestCore = c
			}
		}
		if nearestCoreDist > oppPatrolRadius*oppPatrolRadius {
			if dir := bfsDir(bot, nearestCore, passable, cfg); dir != engine.DirNone {
				return dir
			}
		}
	}
	// Spread out to avoid clustering.
	bestDir, bestScore := engine.DirNone, -1
	for _, st := range cardinalSteps(bot, cfg) {
		if claimedDests[st.pos] {
			continue
		}
		score := 0
		for dest := range claimedDests {
			if d := dist2(st.pos, dest, cfg); d > 0 {
				score += d
			}
		}
		if score > bestScore {
			bestScore = score
			bestDir = st.dir
		}
	}
	return bestDir
}

func oppEnergyMove(bot engine.Position, energy []engine.Position,
	passable func(engine.Position) bool, claimedDests map[engine.Position]bool, cfg engine.Config) engine.Direction {
	bestDist := math.MaxInt32
	var target engine.Position
	found := false
	for _, e := range energy {
		if claimedDests[e] {
			continue
		}
		if d := dist2(bot, e, cfg); d < bestDist && d <= oppEnergySeekRange2 {
			bestDist = d
			target = e
			found = true
		}
	}
	if !found {
		return engine.DirNone
	}
	return bfsDir(bot, target, passable, cfg)
}

// ────────────────────────────────────────────────────────────────────────────
// SiegeBot – spawn-lockout: occupies tiles around enemy cores so they cannot
// respawn; unassigned bots farm energy
// (port of bots/siege/strategy.go)
// ────────────────────────────────────────────────────────────────────────────

type Siege struct{}

func NewSiege() *Siege { return &Siege{} }

func (b *Siege) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	cfg := state.Config
	myID := state.You.ID

	var myBots, enemyBots []engine.VisibleBot
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		} else {
			enemyBots = append(enemyBots, bot)
		}
	}

	enemyPositions := enemyPositions(state.Bots, myID)
	wallPositions := posSet(state.Walls)
	energyPositions := posSet(state.Energy)

	var enemyCores []engine.VisibleCore
	for _, core := range state.Cores {
		if core.Owner != myID && core.Active {
			enemyCores = append(enemyCores, core)
		}
	}

	occupied := make(map[engine.Position]bool)
	for _, bot := range myBots {
		occupied[bot.Position] = true
	}

	moves := make([]engine.Move, 0, len(myBots))
	assignedBots := make(map[engine.Position]bool)

	// PHASE 1: assign bots to lockout rings around enemy cores (greedy by distance).
	lockout := siegeAssignLockout(myBots, enemyCores, enemyPositions, wallPositions, occupied, cfg)
	for botPos, targetPos := range lockout {
		var targetBot *engine.VisibleBot
		for i := range myBots {
			if myBots[i].Position == botPos {
				targetBot = &myBots[i]
				break
			}
		}
		if targetBot == nil {
			continue
		}
		if dir := siegeStepToward(botPos, targetPos, enemyPositions, wallPositions, occupied, cfg); dir != engine.DirNone {
			moves = append(moves, engine.Move{Position: botPos, Direction: dir})
			assignedBots[botPos] = true
			occupied[applyDir(botPos, dir, cfg)] = true
		}
	}

	// PHASE 2: unassigned bots survive the zone, flee, then collect energy.
	usedEnergy := make(map[engine.Position]bool)
	for _, bot := range myBots {
		if assignedBots[bot.Position] {
			continue
		}

		// Zone awareness: survival first.
		if state.Zone != nil && state.Zone.Active {
			d2 := dist2(bot.Position, state.Zone.Center, cfg)
			const safetyMargin2 = 9 // (3 tiles)^2
			if d2 >= state.Zone.Radius*state.Zone.Radius-safetyMargin2 {
				if dir := siegeStepToward(bot.Position, state.Zone.Center, enemyPositions, wallPositions, occupied, cfg); dir != engine.DirNone {
					moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
					occupied[applyDir(bot.Position, dir, cfg)] = true
					continue
				}
			}
		}

		// Flee when locally outnumbered.
		if siegeShouldFlee(bot.Position, myBots, enemyBots, cfg) {
			if dir := siegeFleeDir(bot.Position, enemyBots, wallPositions, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
				occupied[applyDir(bot.Position, dir, cfg)] = true
				continue
			}
		}

		// Collect adjacent energy (immediate gain).
		collected := false
		for _, dir := range allDirs {
			adj := applyDir(bot.Position, dir, cfg)
			if energyPositions[adj] && !usedEnergy[adj] &&
				!wallPositions[adj] && !enemyPositions[adj] && !occupied[adj] {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
				usedEnergy[adj] = true
				occupied[adj] = true
				collected = true
				break
			}
		}
		if collected {
			continue
		}

		// BFS toward nearest untargeted energy.
		if path := siegeNearestEnergyDir(bot.Position, energyPositions, usedEnergy, wallPositions, enemyPositions, occupied, cfg); path != engine.DirNone {
			moves = append(moves, engine.Move{Position: bot.Position, Direction: path})
			occupied[applyDir(bot.Position, path, cfg)] = true
			continue
		}

		// No energy — advance toward the nearest enemy core.
		if len(enemyCores) > 0 {
			nearest := enemyCores[0]
			minDist := dist2(bot.Position, nearest.Position, cfg)
			for _, core := range enemyCores[1:] {
				if d := dist2(bot.Position, core.Position, cfg); d < minDist {
					minDist = d
					nearest = core
				}
			}
			if dir := siegeStepToward(bot.Position, nearest.Position, enemyPositions, wallPositions, occupied, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
				occupied[applyDir(bot.Position, dir, cfg)] = true
			}
		}
	}
	return moves, nil
}

// siegeAssignLockout greedily pairs unassigned bots with ring slots around
// enemy cores (all 8 neighbors), nearest pair first.
func siegeAssignLockout(myBots []engine.VisibleBot, enemyCores []engine.VisibleCore,
	enemyPositions, wallPositions, occupied map[engine.Position]bool, cfg engine.Config) map[engine.Position]engine.Position {

	type slot struct {
		pos     engine.Position
		blocked bool // wall, enemy-occupied, or already taken
	}
	var slots []slot
	for _, core := range enemyCores {
		for _, d := range [8][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}} {
			p := engine.Position{
				Row: (core.Position.Row + d[0] + cfg.Rows) % cfg.Rows,
				Col: (core.Position.Col + d[1] + cfg.Cols) % cfg.Cols,
			}
			if wallPositions[p] || enemyPositions[p] {
				continue
			}
			// A slot already held by a friendly bot is covered — block it from
			// assignment, like the ladder original.
			slots = append(slots, slot{pos: p, blocked: occupied[p]})
		}
	}

	assignments := make(map[engine.Position]engine.Position)
	for {
		bestSlot, bestBot, bestDist := -1, -1, math.MaxInt32
		for bi, bot := range myBots {
			if _, ok := assignments[bot.Position]; ok {
				continue
			}
			for si, s := range slots {
				if s.blocked {
					continue
				}
				targeted := false
				for _, t := range assignments {
					if t == s.pos {
						targeted = true
						break
					}
				}
				if targeted {
					continue
				}
				if d := dist2(bot.Position, s.pos, cfg); d < bestDist {
					bestDist = d
					bestSlot, bestBot = si, bi
				}
			}
		}
		if bestBot == -1 || bestSlot == -1 {
			break
		}
		assignments[myBots[bestBot].Position] = slots[bestSlot].pos
		slots[bestSlot].blocked = true
	}
	return assignments
}

func siegeShouldFlee(pos engine.Position, myBots, enemyBots []engine.VisibleBot, cfg engine.Config) bool {
	nearbyEnemies := 0
	for _, enemy := range enemyBots {
		if dist2(pos, enemy.Position, cfg) <= cfg.AttackRadius2 {
			nearbyEnemies++
		}
	}
	if nearbyEnemies == 0 {
		return false
	}
	nearbyAllies := 0
	for _, ally := range myBots {
		if ally.Position == pos {
			continue
		}
		if dist2(pos, ally.Position, cfg) <= cfg.AttackRadius2 {
			nearbyAllies++
		}
	}
	return nearbyAllies < nearbyEnemies
}

// siegeFleeDir runs away from the enemies' center of mass.
func siegeFleeDir(pos engine.Position, enemies []engine.VisibleBot, wallPositions map[engine.Position]bool, cfg engine.Config) engine.Direction {
	center := engine.Position{}
	for _, enemy := range enemies {
		center.Row += enemy.Position.Row
		center.Col += enemy.Position.Col
	}
	if len(enemies) > 0 {
		center.Row /= len(enemies)
		center.Col /= len(enemies)
	}
	bestDir, bestDist := engine.DirNone, -1
	for _, dir := range allDirs {
		np := applyDir(pos, dir, cfg)
		if wallPositions[np] {
			continue
		}
		if d := dist2(np, center, cfg); d > bestDist {
			bestDist = d
			bestDir = dir
		}
	}
	return bestDir
}

// siegeNearestEnergyDir BFS-searches for the nearest untargeted energy tile
// (depth-limited to 20 steps) and returns the first direction.
func siegeNearestEnergyDir(start engine.Position, energyPositions, usedEnergy, wallPositions, enemyPositions, occupied map[engine.Position]bool, cfg engine.Config) engine.Direction {
	type queueItem struct {
		pos  engine.Position
		path []engine.Direction
	}
	visited := make(map[engine.Position]bool)
	queue := []queueItem{{start, nil}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if visited[item.pos] {
			continue
		}
		visited[item.pos] = true

		if energyPositions[item.pos] && !usedEnergy[item.pos] {
			if len(item.path) == 0 {
				return engine.DirNone
			}
			return item.path[0]
		}
		if len(item.path) > 20 {
			continue
		}
		for _, dir := range allDirs {
			next := applyDir(item.pos, dir, cfg)
			if wallPositions[next] || enemyPositions[next] || occupied[next] {
				continue
			}
			if !visited[next] {
				queue = append(queue, queueItem{next, append(append([]engine.Direction{}, item.path...), dir)})
			}
		}
	}
	return engine.DirNone
}

// siegeStepToward picks the cardinal step that best reduces distance to the
// target while avoiding walls, enemies, and own occupied tiles.
func siegeStepToward(pos, target engine.Position, enemyPositions, wallPositions, occupied map[engine.Position]bool, cfg engine.Config) engine.Direction {
	bestDir, bestDist := engine.DirNone, math.MaxInt32
	for _, dir := range allDirs {
		np := applyDir(pos, dir, cfg)
		if wallPositions[np] || enemyPositions[np] || occupied[np] {
			continue
		}
		if d := dist2(np, target, cfg); d < bestDist {
			bestDist = d
			bestDir = dir
		}
	}
	return bestDir
}

// ────────────────────────────────────────────────────────────────────────────
// EconomistBot – wins by energy starvation: contests nodes enemies are
// approaching so the energy is destroyed, harvests uncontested nodes
// (Go port of bots/economist/bot.py)
// ────────────────────────────────────────────────────────────────────────────

const econAdjacentRadius2 = 2 // sqrt(2)^2 — adjacency for energy contesting

type Economist struct {
	// Persistent per-match state; reset when the match ID changes.
	matchID            string
	contestAssignments map[engine.Position]engine.Position // bot pos -> energy pos
}

func NewEconomist() *Economist {
	return &Economist{contestAssignments: make(map[engine.Position]engine.Position)}
}

func (b *Economist) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	cfg := state.Config
	if b.matchID != state.MatchID {
		b.matchID = state.MatchID
		b.contestAssignments = make(map[engine.Position]engine.Position)
	}
	myID := state.You.ID

	var myBots []engine.VisibleBot
	enemyPositions := make(map[engine.Position]bool)
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		} else {
			enemyPositions[bot.Position] = true
		}
	}
	if len(myBots) == 0 {
		return nil, nil
	}

	myPositions := make(map[engine.Position]bool, len(myBots))
	for _, bot := range myBots {
		myPositions[bot.Position] = true
	}
	energyPositions := posSet(state.Energy)

	// Drop assignments whose bot moved/died or whose energy was consumed.
	for pos, contest := range b.contestAssignments {
		if !myPositions[pos] || !energyPositions[contest] {
			delete(b.contestAssignments, pos)
		}
	}

	var moves []engine.Move
	usedPositions := make(map[engine.Position]bool)

	// Priority 1: maintain existing contest assignments — stay put once adjacent.
	for _, bot := range myBots {
		contest, ok := b.contestAssignments[bot.Position]
		if !ok || !energyPositions[contest] {
			continue
		}
		usedPositions[bot.Position] = true
		if dist2(bot.Position, contest, cfg) > econAdjacentRadius2 {
			if dir := towardDir(bot.Position, contest, enemyPositions, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
			}
		}
	}

	// Priority 2: contest visible energy nodes enemies can also reach.
	type energyPriority struct {
		pos      engine.Position
		priority float64
		myDist   int
	}
	priorities := make([]energyPriority, 0, len(state.Energy))
	for _, e := range state.Energy {
		myReachable, nearestMy := 0, math.MaxInt32
		for _, bot := range myBots {
			if usedPositions[bot.Position] {
				continue
			}
			if d := dist2(bot.Position, e, cfg); d < nearestMy {
				nearestMy = d
			}
			if dist2(bot.Position, e, cfg) <= 64 {
				myReachable++
			}
		}
		_ = myReachable

		enemyReachable, nearestEnemy := 0, math.MaxInt32
		if len(enemyPositions) > 0 {
			for ep := range enemyPositions {
				if d := dist2(ep, e, cfg); d < nearestEnemy {
					nearestEnemy = d
				}
				if dist2(ep, e, cfg) <= 64 {
					enemyReachable++
				}
			}
		} else {
			// No enemies visible — use distance to map center as a proxy.
			center := engine.Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2}
			nearestEnemy = dist2(e, center, cfg)
			if nearestEnemy < 100 {
				enemyReachable = 1
			}
		}

		var priority float64
		switch {
		case enemyReachable > 0:
			priority = 10000.0 / float64(nearestEnemy+nearestMy+1)
		case nearestEnemy < 100:
			priority = 1000.0 / float64(nearestMy+1)
		default:
			priority = 100.0 / float64(nearestMy+1)
		}
		priorities = append(priorities, energyPriority{e, priority, nearestMy})
	}
	sort.SliceStable(priorities, func(i, j int) bool { return priorities[i].priority > priorities[j].priority })

	for _, ep := range priorities {
		// Nearest unassigned bot claims this node.
		nearestBot, nearestDist := engine.Position{}, math.MaxInt32
		found := false
		for _, bot := range myBots {
			if usedPositions[bot.Position] {
				continue
			}
			if d := dist2(bot.Position, ep.pos, cfg); d < nearestDist {
				nearestDist = d
				nearestBot = bot.Position
				found = true
			}
		}
		if !found {
			continue
		}
		usedPositions[nearestBot] = true
		b.contestAssignments[nearestBot] = ep.pos
		if nearestDist > econAdjacentRadius2 {
			if dir := towardDir(nearestBot, ep.pos, enemyPositions, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: nearestBot, Direction: dir})
			}
		}
	}

	// Priority 3: remaining bots drift toward the map center to find energy.
	center := engine.Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2}
	for _, bot := range myBots {
		if !usedPositions[bot.Position] {
			if dir := towardDir(bot.Position, center, enemyPositions, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
			}
		}
	}
	return moves, nil
}

// towardDir steps greedily toward a target (the economist's _move_toward):
// best squared-distance neighbor; DirNone only when no neighbor improves.
func towardDir(from, to engine.Position, enemyPositions map[engine.Position]bool, cfg engine.Config) engine.Direction {
	bestDir, bestDist := engine.DirNone, math.MaxInt32
	for _, st := range cardinalSteps(from, cfg) {
		if d := dist2(st.pos, to, cfg); d < bestDist {
			bestDist = d
			bestDir = st.dir
		}
	}
	return bestDir
}

// ────────────────────────────────────────────────────────────────────────────
// AssassinBot – decapitation archetype: every unit rushes the enemy core,
// ignoring enemies and economy; relies on speed and mass
// (Go port of bots/assassin/src/strategy.rs)
// ────────────────────────────────────────────────────────────────────────────

type Assassin struct {
	// Persistent per-match state; reset when the match ID changes.
	matchID      string
	knownTargets map[engine.Position]bool // enemy cores ever seen; value = last-known active
}

func NewAssassin() *Assassin {
	return &Assassin{knownTargets: make(map[engine.Position]bool)}
}

func (b *Assassin) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	cfg := state.Config
	myID := state.You.ID
	if b.matchID != state.MatchID {
		b.matchID = state.MatchID
		b.knownTargets = make(map[engine.Position]bool)
	}

	for _, core := range state.Cores {
		if core.Owner != myID {
			b.knownTargets[core.Position] = core.Active
		}
	}

	var myBots []engine.VisibleBot
	for _, bot := range state.Bots {
		if bot.Owner == myID {
			myBots = append(myBots, bot)
		}
	}
	if len(myBots) == 0 {
		return nil, nil
	}
	walls := posSet(state.Walls)

	// Active targets sorted by distance from our center of mass.
	center := engine.Position{}
	for _, bot := range myBots {
		center.Row += bot.Position.Row
		center.Col += bot.Position.Col
	}
	center.Row /= len(myBots)
	center.Col /= len(myBots)

	var targets []engine.Position
	for pos, active := range b.knownTargets {
		if active {
			targets = append(targets, pos)
		}
	}
	// Stable, matching Rust's sort_by_key — ties keep discovery order.
	sort.SliceStable(targets, func(i, j int) bool {
		return dist2(center, targets[i], cfg) < dist2(center, targets[j], cfg)
	})

	if len(targets) == 0 {
		// Explore outward to find enemy cores.
		var moves []engine.Move
		for i, bot := range myBots {
			targetRow := cfg.Rows - 1
			if i%3 == 0 {
				targetRow = cfg.Rows / 2
			}
			targetCol := 0
			if i%2 == 0 {
				targetCol = cfg.Cols - 1
			}
			target := engine.Position{Row: targetRow, Col: targetCol}
			if dir := closestStepDir(bot.Position, target, walls, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
			}
		}
		return moves, nil
	}

	primary := targets[0]
	claimed := make(map[engine.Position]bool)
	moves := make([]engine.Move, 0, len(myBots))
	for _, bot := range myBots {
		// Unlike rusher, walk straight through enemies — only walls block.
		if dir := assassinBFSDir(bot.Position, primary, walls, claimed, cfg); dir != engine.DirNone {
			dest := applyDir(bot.Position, dir, cfg)
			claimed[dest] = true
			moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
		}
	}
	return moves, nil
}

// assassinBFSDir paths toward the goal through enemies (only walls block);
// falls back to the step that minimizes toroidal Manhattan distance.
func assassinBFSDir(start, goal engine.Position, walls map[engine.Position]bool, claimed map[engine.Position]bool, cfg engine.Config) engine.Direction {
	if start == goal {
		return engine.DirNone
	}
	passable := func(p engine.Position) bool { return !walls[p] }

	if dir := bfsDir(start, goal, passable, cfg); dir != engine.DirNone {
		return dir
	}
	// No path — pick the direction that gets closest (skip claimed tiles).
	return closestStepDir(start, goal, walls, cfg, claimed)
}

// closestStepDir steps toward the target by toroidal Manhattan distance.
func closestStepDir(start, target engine.Position, walls map[engine.Position]bool, cfg engine.Config, claimed ...map[engine.Position]bool) engine.Direction {
	bestDir, bestDist := engine.DirNone, math.MaxInt32
	for _, st := range cardinalSteps(start, cfg) {
		if walls[st.pos] {
			continue
		}
		blocked := false
		for _, c := range claimed {
			if c[st.pos] {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		if d := torManhattan(st.pos, target, cfg); d < bestDist {
			bestDist = d
			bestDir = st.dir
		}
	}
	return bestDir
}

// ────────────────────────────────────────────────────────────────────────────
// PhalanxBot – tight formation combat: circular-mean centroid, hex formation
// slots, rally when cohesion breaks, advance on enemy concentration otherwise
// (Go port of bots/phalanx/src/strategy.rs)
// ────────────────────────────────────────────────────────────────────────────

const (
	phxFormationRadius2 = 9.0 // max mean squared distance from centroid before rally
	phxAdvanceWeight    = 10.0
	phxFormationWeight  = 8.0
	phxAttackRangeBonus = 50.0
)

type Phalanx struct {
	// Persistent per-match state; reset when the match ID changes.
	matchID  string
	centroid *engine.Position
}

func NewPhalanx() *Phalanx { return &Phalanx{} }

func (b *Phalanx) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	cfg := state.Config
	myID := state.You.ID
	if b.matchID != state.MatchID {
		b.matchID = state.MatchID
		b.centroid = nil
	}

	var myBots, enemyBots []engine.VisibleBot
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

	myPositions := make([]engine.Position, 0, len(myBots))
	for _, bot := range myBots {
		myPositions = append(myPositions, bot.Position)
	}
	walls := posSet(state.Walls)
	enemySet := enemyPositions(state.Bots, myID)

	// Circular-mean centroid, smoothed with last turn's value (70% new).
	centroid := circularMean(myPositions, cfg)
	if b.centroid != nil {
		centroid = smoothCentroid(*b.centroid, centroid, cfg)
	}
	b.centroid = &centroid

	meanDist := meanDistance2From(myPositions, centroid, cfg)
	rallying := meanDist > phxFormationRadius2

	advanceTarget := centroid
	if !rallying {
		if len(enemyBots) > 0 {
			enemyPositionsList := make([]engine.Position, 0, len(enemyBots))
			for _, e := range enemyBots {
				enemyPositionsList = append(enemyPositionsList, e.Position)
			}
			advanceTarget = circularMean(enemyPositionsList, cfg)
		} else {
			advanceTarget = engine.Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2}
		}
	}

	slots := phxFormationSlots(centroid, len(myPositions), cfg)
	assignments := phxAssignSlots(myPositions, slots, cfg)

	claimed := make(map[engine.Position]bool)
	moves := make([]engine.Move, 0, len(myBots))
	for _, bot := range myBots {
		slot, hasSlot := assignments[bot.Position]
		if dir, ok := phxScoredDir(bot.Position, hasSlot, slot, advanceTarget, centroid, enemySet, walls, claimed, rallying, cfg); ok {
			dest := applyDir(bot.Position, dir, cfg)
			claimed[dest] = true
			moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
		} else {
			claimed[bot.Position] = true
		}
	}
	return moves, nil
}

// phxScoredDir scores each candidate step by slot cohesion, centroid
// proximity, advance toward the target, and attack-range presence.
func phxScoredDir(pos engine.Position, hasSlot bool, slot, advanceTarget, centroid engine.Position,
	enemies, walls, claimed map[engine.Position]bool, rallying bool, cfg engine.Config) (engine.Direction, bool) {

	bestDir, bestScore, any := engine.DirNone, math.Inf(-1), false
	for _, st := range cardinalSteps(pos, cfg) {
		if walls[st.pos] || enemies[st.pos] || claimed[st.pos] {
			continue
		}
		score := 0.0

		if hasSlot {
			score += (float64(dist2(pos, slot, cfg)) - float64(dist2(st.pos, slot, cfg))) * phxFormationWeight
		}
		score += (float64(dist2(pos, centroid, cfg)) - float64(dist2(st.pos, centroid, cfg))) * (phxFormationWeight * 0.3)

		advance := phxAdvanceWeight
		if rallying {
			advance *= 2.0
		}
		score += (float64(dist2(pos, advanceTarget, cfg)) - float64(dist2(st.pos, advanceTarget, cfg))) * advance

		if !rallying {
			for ep := range enemies {
				if float64(dist2(st.pos, ep, cfg)) <= float64(cfg.AttackRadius2) {
					score += phxAttackRangeBonus
				}
			}
		}

		if !any || score > bestScore {
			bestScore = score
			bestDir = st.dir
			any = true
		}
	}
	return bestDir, any
}

// circularMean computes the toroidally-correct center of mass.
func circularMean(positions []engine.Position, cfg engine.Config) engine.Position {
	if len(positions) == 0 {
		return engine.Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2}
	}
	rowScale := 2.0 * math.Pi / float64(cfg.Rows)
	colScale := 2.0 * math.Pi / float64(cfg.Cols)
	n := float64(len(positions))

	var sinR, cosR, sinC, cosC float64
	for _, p := range positions {
		sinR += math.Sin(float64(p.Row) * rowScale)
		cosR += math.Cos(float64(p.Row) * rowScale)
		sinC += math.Sin(float64(p.Col) * colScale)
		cosC += math.Cos(float64(p.Col) * colScale)
	}

	avgRow := math.Atan2(sinR/n, cosR/n) / rowScale
	avgCol := math.Atan2(sinC/n, cosC/n) / colScale
	wrappedRow := math.Mod(math.Mod(avgRow, float64(cfg.Rows))+float64(cfg.Rows), float64(cfg.Rows))
	wrappedCol := math.Mod(math.Mod(avgCol, float64(cfg.Cols))+float64(cfg.Cols), float64(cfg.Cols))
	return engine.Position{Row: int(math.Round(wrappedRow)), Col: int(math.Round(wrappedCol))}
}

// smoothCentroid blends 70% of the delta into the previous centroid.
func smoothCentroid(prev, current engine.Position, cfg engine.Config) engine.Position {
	dr := toroidalDelta(prev.Row, current.Row, cfg.Rows)
	dc := toroidalDelta(prev.Col, current.Col, cfg.Cols)
	return engine.Position{
		Row: wrapInt(int(math.Round(float64(prev.Row)+0.7*float64(dr))), cfg.Rows),
		Col: wrapInt(int(math.Round(float64(prev.Col)+0.7*float64(dc))), cfg.Cols),
	}
}

// toroidalDelta returns the signed shortest delta from a to b on a ring of
// size n (|delta| ≤ n/2).
func toroidalDelta(a, b, n int) int {
	d := b - a
	if d > n/2 {
		d -= n
	} else if d < -n/2 {
		d += n
	}
	return d
}

func wrapInt(v, n int) int {
	return ((v % n) + n) % n
}

func meanDistance2From(positions []engine.Position, center engine.Position, cfg engine.Config) float64 {
	if len(positions) == 0 {
		return 0
	}
	total := 0
	for _, p := range positions {
		total += dist2(p, center, cfg)
	}
	return float64(total) / float64(len(positions))
}

// phxFormationSlots lays out hex-ring packing slots around the centroid.
func phxFormationSlots(centroid engine.Position, count int, cfg engine.Config) []engine.Position {
	if count == 0 {
		return nil
	}
	slots := []engine.Position{centroid}
	for ring := 1; len(slots) < count && ring <= 20; ring++ {
		for _, d := range hexRing(ring) {
			if len(slots) >= count {
				break
			}
			slots = append(slots, engine.Position{
				Row: wrapInt(centroid.Row+d[0], cfg.Rows),
				Col: wrapInt(centroid.Col+d[1], cfg.Cols),
			})
		}
	}
	return slots
}

// hexRing generates the 6*ring offsets of a hex ring in offset coordinates
// (axial hex → offset_col = q + r/2).
func hexRing(ring int) [][2]int {
	if ring == 0 {
		return [][2]int{{0, 0}}
	}
	hexDirs := [6][2]int{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}}
	result := make([][2]int, 0, 6*ring)
	q, r := ring, 0
	for _, d := range hexDirs {
		for i := 0; i < ring; i++ {
			result = append(result, [2]int{r, q + r/2})
			q += d[0]
			r += d[1]
		}
	}
	return result
}

// phxAssignSlots greedily assigns each bot its nearest unused slot.
func phxAssignSlots(bots, slots []engine.Position, cfg engine.Config) map[engine.Position]engine.Position {
	assignments := make(map[engine.Position]engine.Position, len(bots))
	used := make([]bool, len(slots))
	for _, bot := range bots {
		bestSlot, bestDist := 0, math.MaxInt32
		for si, slot := range slots {
			if used[si] {
				continue
			}
			if d := dist2(bot, slot, cfg); d < bestDist {
				bestDist = d
				bestSlot = si
			}
		}
		if bestSlot < len(slots) {
			used[bestSlot] = true
			assignments[bot] = slots[bestSlot]
		}
	}
	return assignments
}

// ────────────────────────────────────────────────────────────────────────────
// ZoneDriverBot – weaponizes the shrinking zone: saves own bots near the
// edge, blocks enemy escape routes from the kill band, sweeps to herd enemies
// (Go port of bots/zone-driver/src/strategy.rs)
// ────────────────────────────────────────────────────────────────────────────

type ZoneDriver struct{}

func NewZoneDriver() *ZoneDriver { return &ZoneDriver{} }

func (b *ZoneDriver) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	cfg := state.Config
	myID := state.You.ID

	var myBots, enemyBots []engine.VisibleBot
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

	enemySet := enemyPositions(state.Bots, myID)
	walls := posSet(state.Walls)

	if state.Zone == nil || !state.Zone.Active {
		// No active zone — play conservatively.
		return zdDefensiveFallback(myBots, enemySet, walls, cfg), nil
	}
	zone := *state.Zone

	moves := make([]engine.Move, 0, len(myBots))
	assigned := make(map[engine.Position]bool)

	// PRIORITY 1: save own bots outside or on the zone edge.
	for _, bot := range myBots {
		if zdDistanceToZoneEdge(bot.Position, zone, cfg) <= 0 {
			if dir := zdRetreatDir(bot.Position, zone, walls, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
				assigned[bot.Position] = true
			}
		}
	}

	// PRIORITY 2: block escape routes of enemies in the kill band (the ring
	// just inside the zone edge where enemies die next shrink).
	killBandInner := zone.Radius - 2
	if killBandInner < 0 {
		killBandInner = 0
	}
	killBandOuter := zone.Radius
	for _, bot := range myBots {
		if assigned[bot.Position] {
			continue
		}
		if target, ok := zdEnemyInKillBand(bot.Position, enemyBots, zone, killBandInner, killBandOuter, cfg); ok {
			if dir := zdBlockEscapeDir(bot.Position, target, zone, walls, cfg); dir != engine.DirNone {
				moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
				assigned[bot.Position] = true
			}
		}
	}

	// PRIORITY 3: sweep to apply pressure.
	for _, bot := range myBots {
		if assigned[bot.Position] {
			continue
		}
		if dir, ok := zdAdvancePressureDir(bot.Position, enemySet, zone, walls, cfg); ok {
			moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
			assigned[bot.Position] = true
		}
	}

	// Remaining bots hold position (the ladder bot emits a nominal N here).
	for _, bot := range myBots {
		if !assigned[bot.Position] {
			moves = append(moves, engine.Move{Position: bot.Position, Direction: engine.DirN})
		}
	}
	return moves, nil
}

// zdDistanceToZoneEdge is positive inside the zone, 0 on the edge, negative outside.
func zdDistanceToZoneEdge(pos engine.Position, zone engine.ZoneBounds, cfg engine.Config) int {
	dist := math.Sqrt(float64(dist2(pos, zone.Center, cfg)))
	return int(float64(zone.Radius) - dist)
}

func zdRetreatDir(pos engine.Position, zone engine.ZoneBounds, walls map[engine.Position]bool, cfg engine.Config) engine.Direction {
	bestDir, bestReduction := engine.DirNone, math.MinInt32
	current := dist2(pos, zone.Center, cfg)
	for _, st := range cardinalSteps(pos, cfg) {
		if walls[st.pos] {
			continue
		}
		if next := dist2(st.pos, zone.Center, cfg); next < current {
			if reduction := current - next; reduction > bestReduction {
				bestReduction = reduction
				bestDir = st.dir
			}
		}
	}
	return bestDir
}

func zdEnemyInKillBand(myPos engine.Position, enemies []engine.VisibleBot, zone engine.ZoneBounds, inner, outer int, cfg engine.Config) (engine.Position, bool) {
	var best engine.Position
	bestDist, found := math.MaxInt32, false
	for _, bot := range enemies {
		dist := math.Sqrt(float64(dist2(bot.Position, zone.Center, cfg)))
		if dist < float64(inner) || dist > float64(outer) {
			continue
		}
		if d := dist2(myPos, bot.Position, cfg); d < bestDist {
			bestDist = d
			best = bot.Position
			found = true
		}
	}
	return best, found
}

// zdBlockEscapeDir moves toward the tile one step inward of the enemy
// (between it and the zone center).
func zdBlockEscapeDir(myPos, enemyPos engine.Position, zone engine.ZoneBounds, walls map[engine.Position]bool, cfg engine.Config) engine.Direction {
	dr := float64(zone.Center.Row - enemyPos.Row)
	dc := float64(zone.Center.Col - enemyPos.Col)
	length := math.Sqrt(dr*dr + dc*dc)
	if length < 0.1 {
		return engine.DirNone
	}
	idealRow := float64(enemyPos.Row) + dr/length
	idealCol := float64(enemyPos.Col) + dc/length

	bestDir, bestDist := engine.DirNone, math.MaxFloat64
	for _, st := range cardinalSteps(myPos, cfg) {
		if walls[st.pos] {
			continue
		}
		fdr := idealRow - float64(st.pos.Row)
		fdc := idealCol - float64(st.pos.Col)
		if d := math.Sqrt(fdr*fdr + fdc*fdc); d < bestDist {
			bestDist = d
			bestDir = st.dir
		}
	}
	return bestDir
}

func zdAdvancePressureDir(pos engine.Position, enemySet map[engine.Position]bool, zone engine.ZoneBounds, walls map[engine.Position]bool, cfg engine.Config) (engine.Direction, bool) {
	// Enemies visible — advance toward the nearest.
	if len(enemySet) > 0 {
		nearest, nearestDist := engine.Position{}, math.MaxInt32
		for e := range enemySet {
			if d := dist2(pos, e, cfg); d < nearestDist {
				nearestDist = d
				nearest = e
			}
		}
		bestDir, bestDist := engine.DirNone, math.MaxInt32
		for _, st := range cardinalSteps(pos, cfg) {
			if walls[st.pos] {
				continue
			}
			if d := dist2(st.pos, nearest, cfg); d < bestDist {
				bestDist = d
				bestDir = st.dir
			}
		}
		if bestDir != engine.DirNone {
			return bestDir, true
		}
	}

	// No enemies — move out to the pressure ring (radius − 3).
	targetRadius := zone.Radius - 3
	if targetRadius < 0 {
		targetRadius = 0
	}
	targetDist2 := targetRadius * targetRadius
	current := dist2(pos, zone.Center, cfg)
	if current < targetDist2 {
		bestDir, bestIncrease := engine.DirNone, math.MinInt32
		for _, st := range cardinalSteps(pos, cfg) {
			if walls[st.pos] {
				continue
			}
			if increase := dist2(st.pos, zone.Center, cfg) - current; increase > bestIncrease {
				bestIncrease = increase
				bestDir = st.dir
			}
		}
		return bestDir, bestDir != engine.DirNone
	}
	return engine.DirNone, false
}

func zdDefensiveFallback(myBots []engine.VisibleBot, enemySet, walls map[engine.Position]bool, cfg engine.Config) []engine.Move {
	var moves []engine.Move
	for _, bot := range myBots {
		if len(enemySet) == 0 {
			continue
		}
		nearest, nearestDist := engine.Position{}, math.MaxInt32
		for e := range enemySet {
			if d := dist2(bot.Position, e, cfg); d < nearestDist {
				nearestDist = d
				nearest = e
			}
		}
		bestDir, bestDist := engine.DirNone, math.MaxInt32
		for _, st := range cardinalSteps(bot.Position, cfg) {
			if walls[st.pos] || enemySet[st.pos] {
				continue
			}
			if d := dist2(st.pos, nearest, cfg); d < bestDist {
				bestDist = d
				bestDir = st.dir
			}
		}
		if bestDir != engine.DirNone {
			moves = append(moves, engine.Move{Position: bot.Position, Direction: bestDir})
		}
	}
	return moves
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers (unexported)
// ────────────────────────────────────────────────────────────────────────────

var allDirs = []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW}

func randDir(rng *rand.Rand) engine.Direction { return allDirs[rng.Intn(4)] }

func posSet(positions []engine.Position) map[engine.Position]bool {
	m := make(map[engine.Position]bool, len(positions))
	for _, p := range positions {
		m[p] = true
	}
	return m
}

func enemyPositions(bots []engine.VisibleBot, myID int) map[engine.Position]bool {
	m := make(map[engine.Position]bool)
	for _, b := range bots {
		if b.Owner != myID {
			m[b.Position] = true
		}
	}
	return m
}

func applyDir(p engine.Position, d engine.Direction, cfg engine.Config) engine.Position {
	dr, dc := d.Delta()
	row := ((p.Row+dr)%cfg.Rows + cfg.Rows) % cfg.Rows
	col := ((p.Col+dc)%cfg.Cols + cfg.Cols) % cfg.Cols
	return engine.Position{Row: row, Col: col}
}

func dist2(a, b engine.Position, cfg engine.Config) int {
	dr := a.Row - b.Row
	if dr < 0 {
		dr = -dr
	}
	if dr > cfg.Rows/2 {
		dr = cfg.Rows - dr
	}
	dc := a.Col - b.Col
	if dc < 0 {
		dc = -dc
	}
	if dc > cfg.Cols/2 {
		dc = cfg.Cols - dc
	}
	return dr*dr + dc*dc
}

func towardNearest(from engine.Position, targets map[engine.Position]bool, cfg engine.Config) engine.Direction {
	if len(targets) == 0 {
		return engine.DirNone
	}
	best, bestD := engine.DirNone, 1<<31-1
	for _, d := range allDirs {
		np := applyDir(from, d, cfg)
		for t := range targets {
			if d2 := dist2(np, t, cfg); d2 < bestD {
				bestD = d2
				best = d
			}
		}
	}
	return best
}

func fleeDir(from engine.Position, enemies map[engine.Position]bool, cfg engine.Config) engine.Direction {
	thr := cfg.AttackRadius2 + 4
	close := false
	for e := range enemies {
		if dist2(from, e, cfg) <= thr {
			close = true
			break
		}
	}
	if !close {
		return engine.DirNone
	}
	best, bestD := engine.DirNone, -1
	for _, d := range allDirs {
		np := applyDir(from, d, cfg)
		minD := 1<<31 - 1
		for e := range enemies {
			if d2 := dist2(np, e, cfg); d2 < minD {
				minD = d2
			}
		}
		if minD > bestD {
			bestD = minD
			best = d
		}
	}
	return best
}

func isNear(from engine.Position, targets map[engine.Position]bool, cfg engine.Config, r2 int) bool {
	for t := range targets {
		if dist2(from, t, cfg) <= r2 {
			return true
		}
	}
	return false
}

// cardinalStep is one 4-directional neighbor plus the direction reaching it.
type cardinalStep struct {
	pos engine.Position
	dir engine.Direction
}

// cardinalSteps returns the four wrapped neighbors of p in N, E, S, W order.
func cardinalSteps(p engine.Position, cfg engine.Config) []cardinalStep {
	return []cardinalStep{
		{applyDir(p, engine.DirN, cfg), engine.DirN},
		{applyDir(p, engine.DirE, cfg), engine.DirE},
		{applyDir(p, engine.DirS, cfg), engine.DirS},
		{applyDir(p, engine.DirW, cfg), engine.DirW},
	}
}

// torManhattan returns toroidal Manhattan distance (farmer/opportunist pathing
// heuristic).
func torManhattan(a, b engine.Position, cfg engine.Config) int {
	return absInt(toroidalDelta(a.Row, b.Row, cfg.Rows)) + absInt(toroidalDelta(a.Col, b.Col, cfg.Cols))
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// bfsDir returns the first direction of a shortest 4-directional path from
// start to goal on the toroidal grid, or DirNone when unreachable. The goal
// itself must satisfy passable, mirroring the ladder bots' BFS.
func bfsDir(start, goal engine.Position, passable func(engine.Position) bool, cfg engine.Config) engine.Direction {
	if start == goal {
		return engine.DirNone
	}
	type node struct {
		pos engine.Position
		dir engine.Direction
	}
	visited := map[engine.Position]bool{start: true}
	queue := make([]node, 0, 64)
	for _, st := range cardinalSteps(start, cfg) {
		if st.pos == goal && passable(st.pos) {
			return st.dir
		}
		if passable(st.pos) && !visited[st.pos] {
			visited[st.pos] = true
			queue = append(queue, node{st.pos, st.dir})
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.pos == goal {
			return cur.dir
		}
		for _, st := range cardinalSteps(cur.pos, cfg) {
			if !visited[st.pos] && passable(st.pos) {
				visited[st.pos] = true
				queue = append(queue, node{st.pos, cur.dir})
			}
		}
	}
	return engine.DirNone
}
