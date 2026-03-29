//go:build js && wasm

// random.wasm – random-strategy bot implementing the ACB WASM bot interface.
//
// Exported JS object (global acbBot):
//
//	acbBot.init(configJSON)          – initialise; resets RNG seed
//	acbBot.compute_moves(stateJSON)  – returns JSON move array
package main

import (
	"encoding/json"
	"math/rand"
	"syscall/js"
	"time"

	"github.com/aicodebattle/acb/engine"
)

var rng *rand.Rand

func jsInit(_ js.Value, args []js.Value) interface{} {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	return map[string]interface{}{"ok": true}
}

func jsComputeMoves(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsErr("stateJSON required")
	}
	var state engine.VisibleState
	if err := json.Unmarshal([]byte(args[0].String()), &state); err != nil {
		return jsErr("parse: " + err.Error())
	}
	bot := engine.NewRandomBot(rng.Int63())
	moves, _ := bot.GetMoves(&state)
	b, _ := json.Marshal(moves)
	return string(b)
}

func jsErr(msg string) map[string]interface{} {
	return map[string]interface{}{"ok": false, "error": msg}
}

func main() {
	done := make(chan struct{})
	js.Global().Set("acbBot", js.ValueOf(map[string]interface{}{
		"init":          js.FuncOf(jsInit),
		"compute_moves": js.FuncOf(jsComputeMoves),
	}))
	<-done
}
