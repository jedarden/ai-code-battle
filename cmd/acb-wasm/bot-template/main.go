//go:build js && wasm

// Package main implements a WASM bot for the AI Code Battle sandbox.
// Compile with: GOOS=js GOARCH=wasm go build -o mybot.wasm .
//
// The bot exports an 'acbBot' global object with:
//   init(configJSON: string) - called once at match start
//   compute_moves(stateJSON: string) - called each turn, returns moves JSON
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/aicodebattle/acb/engine"
)

// botState holds persistent state across turns (e.g., pathfinding cache).
type botState struct {
	config    engine.Config
	myID      int
	knownPos  map[string]bool // positions we've seen
}

var state = &botState{
	knownPos: make(map[string]bool),
}

// jsInit is called once at match start with the game config.
func jsInit(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{"ok": false, "error": "configJSON required"}
	}

	var cfg engine.Config
	if err := json.Unmarshal([]byte(args[0].String()), &cfg); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}

	state.config = cfg
	return map[string]interface{}{"ok": true}
}

// jsComputeMoves is called each turn with the visible game state.
func jsComputeMoves(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "[]"
	}

	var visible engine.VisibleState
	if err := json.Unmarshal([]byte(args[0].String()), &visible); err != nil {
		return "[]"
	}

	state.myID = visible.You.ID
	moves := computeMoves(&visible)

	jsonBytes, _ := json.Marshal(moves)
	return string(jsonBytes)
}

// computeMoves contains your bot logic. This is a simple example:
// move each bot toward the nearest energy, avoiding enemies if close.
func computeMoves(visible *engine.VisibleState) []engine.Move {
	var moves []engine.Move

	energySet := make(map[engine.Position]bool)
	for _, e := range visible.Energy {
		energySet[e] = true
	}

	enemySet := make(map[engine.Position]bool)
	for _, b := range visible.Bots {
		if b.Owner != state.myID {
			enemySet[b.Position] = true
		}
	}

	for _, bot := range visible.Bots {
		if bot.Owner != state.myID {
			continue
		}

		dir := fleeFromEnemies(bot.Position, enemySet)
		if dir == engine.DirNone {
			dir = towardNearest(bot.Position, energySet)
		}
		if dir == engine.DirNone {
			dir = randomDir()
		}

		moves = append(moves, engine.Move{
			Position: bot.Position,
			Direction: dir,
		})
	}

	return moves
}

func fleeFromEnemies(from engine.Position, enemies map[engine.Position]bool) engine.Direction {
	thr := state.config.AttackRadius2 + 4
	for e := range enemies {
		if dist2(from, e) <= thr {
			return bestFleeDir(from, enemies)
		}
	}
	return engine.DirNone
}

func bestFleeDir(from engine.Position, enemies map[engine.Position]bool) engine.Direction {
	bestDir := engine.DirNone
	bestDist := -1

	for _, d := range []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW} {
		dr, dc := d.Delta()
		np := engine.Position{
			Row: ((from.Row + dr) % state.config.Rows + state.config.Rows) % state.config.Rows,
			Col: ((from.Col + dc) % state.config.Cols + state.config.Cols) % state.config.Cols,
		}

		minDist := 1 << 30
		for e := range enemies {
			if d2 := dist2(np, e); d2 < minDist {
				minDist = d2
			}
		}

		if minDist > bestDist {
			bestDist = minDist
			bestDir = d
		}
	}

	return bestDir
}

func towardNearest(from engine.Position, targets map[engine.Position]bool) engine.Direction {
	if len(targets) == 0 {
		return engine.DirNone
	}

	bestDir := engine.DirNone
	bestDist := 1 << 30

	for _, d := range []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW} {
		dr, dc := d.Delta()
		np := engine.Position{
			Row: ((from.Row + dr) % state.config.Rows + state.config.Rows) % state.config.Rows,
			Col: ((from.Col + dc) % state.config.Cols + state.config.Cols) % state.config.Cols,
		}

		for t := range targets {
			if d2 := dist2(np, t); d2 < bestDist {
				bestDist = d2
				bestDir = d
			}
		}
	}

	return bestDir
}

func dist2(a, b engine.Position) int {
	dr := a.Row - b.Row
	if dr < 0 {
		dr = -dr
	}
	if dr > state.config.Rows/2 {
		dr = state.config.Rows - dr
	}
	dc := a.Col - b.Col
	if dc < 0 {
		dc = -dc
	}
	if dc > state.config.Cols/2 {
		dc = state.config.Cols - dc
	}
	return dr*dr + dc*dc
}

func randomDir() engine.Direction {
	dirs := []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW}
	return dirs[(state.config.Rows+state.config.Cols)%4]
}

func main() {
	done := make(chan struct{})

	js.Global().Set("acbBot", js.ValueOf(map[string]interface{}{
		"init": js.FuncOf(jsInit),
		"compute_moves": js.FuncOf(jsComputeMoves),
	}))

	<-done
}
