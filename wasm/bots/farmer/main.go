//go:build js && wasm

// Package main compiles to farmer.wasm for the browser sandbox.
// It exports the standard bot WASM interface on the acbBot global:
// init, compute_moves, free_result.
package main

import (
	"encoding/json"
	"math/rand"
	"syscall/js"
	"time"

	"github.com/aicodebattle/acb/cmd/acb-wasm/strategies"
	"github.com/aicodebattle/acb/engine"
)

var (
	bot engine.BotInterface
	rng *rand.Rand
)

// jsInit initializes the bot for a new match.
// Signature: init(configJSON: string) => {ok:bool, error?:string}
func jsInit(_ js.Value, _ []js.Value) interface{} {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	bot = strategies.New("farmer", rng)
	return map[string]interface{}{"ok": true}
}

// jsComputeMoves returns moves for the current turn.
// Signature: compute_moves(stateJSON: string) => string (moves JSON)
func jsComputeMoves(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "[]"
	}
	if bot == nil {
		bot = strategies.New("farmer", rng)
	}
	var state engine.VisibleState
	if err := json.Unmarshal([]byte(args[0].String()), &state); err != nil {
		return "[]"
	}
	moves, _ := bot.GetMoves(&state)
	b, _ := json.Marshal(moves)
	return string(b)
}

// jsFreeResult is a no-op for Go (GC handles memory).
// Signature: free_result(ptr: number) => undefined
func jsFreeResult(_ js.Value, _ []js.Value) interface{} {
	return nil
}

func main() {
	js.Global().Set("acbBot", js.ValueOf(map[string]interface{}{
		"init": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsInit(this, args)
		}),
		"compute_moves": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsComputeMoves(this, args)
		}),
		"free_result": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsFreeResult(this, args)
		}),
		"version": "1.0.0",
	}))
	select {}
}
