// Package strategies provides the six built-in ACB bot strategies for use in
// WASM builds. Each strategy implements engine.BotInterface.
package strategies

import (
	"math/rand"

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
