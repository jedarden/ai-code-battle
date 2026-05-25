//go:build js && wasm

// Package main compiles to random.wasm for the browser sandbox.
// RandomBot makes uniformly random valid moves.
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

// jsInit initializes the bot with game config.
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

// jsComputeMoves returns random moves.
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

// jsFreeResult is a no-op.
func jsFreeResult(_ js.Value, _ []js.Value) interface{} {
	return nil
}

// getMoves implements RandomBot strategy.
func getMoves(state *engine.VisibleState) []engine.Move {
	myID := state.You.ID
	allDirs := []engine.Direction{engine.DirN, engine.DirE, engine.DirS, engine.DirW}
	var moves []engine.Move
	for _, bot := range state.Bots {
		if bot.Owner != myID {
			continue
		}
		dir := allDirs[rng.Intn(4)]
		moves = append(moves, engine.Move{Position: bot.Position, Direction: dir})
	}
	return moves
}

func main() {
	js.Global().Set("randomBot", js.ValueOf(map[string]interface{}{
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
