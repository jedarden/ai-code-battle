//go:build js && wasm

// Package main compiles to hunter.wasm for the browser sandbox.
// HunterBot hunts nearest enemy bot.
package main

import (
	"encoding/json"
	"math/rand"
	"syscall/js"

	"github.com/aicodebattle/acb/engine"
)

var (
	cfg     engine.Config
	rng     *rand.Rand
	visible *engine.VisibleState
)

func jsInit(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsErr("configJSON argument required")
	}
	if err := json.Unmarshal([]byte(args[0].String()), &cfg); err != nil {
		return jsErr("parse config: " + err.Error())
	}
	rng = rand.New(rand.NewSource(42))
	visible = &engine.VisibleState{}
	return map[string]interface{}{"ok": true}
}

func jsComputeMoves(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "[]"
	}
	if err := json.Unmarshal([]byte(args[0].String()), visible); err != nil {
		return "[]"
	}
	moves := getMoves(visible)
	json, _ := json.Marshal(moves)
	return string(json)
}

func jsFreeResult(_ js.Value, _ []js.Value) interface{} {
	return nil
}

// getMoves implements HunterBot: hunt nearest enemy.
func getMoves(state *engine.VisibleState) []engine.Move {
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
			dir = towardNearest(bot.Position, enemySet)
		} else {
			dir = towardNearest(bot.Position, energySet)
		}
		if dir == engine.DirNone {
			dir = randDir()
		}
		moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
	}
	return moves
}

func main() {
	js.Global().Set("hunterBot", js.ValueOf(map[string]interface{}{
		"init":          js.FuncOf(jsInit),
		"compute_moves": js.FuncOf(jsComputeMoves),
		"free_result":   js.FuncOf(jsFreeResult),
		"version":       "1.0.0",
	}))
	select {}
}

func jsErr(msg string) map[string]interface{} {
	return map[string]interface{}{"ok": false, "error": msg}
}

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

func applyDir(p engine.Position, d engine.Direction) engine.Position {
	dr, dc := d.Delta()
	row := ((p.Row+dr)%cfg.Rows + cfg.Rows) % cfg.Rows
	col := ((p.Col+dc)%cfg.Cols + cfg.Cols) % cfg.Cols
	return engine.Position{Row: row, Col: col}
}

func dist2(a, b engine.Position) int {
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

func towardNearest(from engine.Position, targets map[engine.Position]bool) engine.Direction {
	if len(targets) == 0 {
		return engine.DirNone
	}
	allDirs := []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW}
	best, bestD := engine.DirNone, 1<<31-1
	for _, d := range allDirs {
		np := applyDir(from, d)
		for t := range targets {
			if d2 := dist2(np, t); d2 < bestD {
				bestD = d2
				best = d
			}
		}
	}
	return best
}

func randDir() engine.Direction {
	allDirs := []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW}
	return allDirs[rng.Intn(4)]
}
