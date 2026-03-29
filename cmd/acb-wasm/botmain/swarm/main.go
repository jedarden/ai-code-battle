//go:build js && wasm

// swarm.wasm implements the ACB WASM bot interface for the swarm strategy.
//
//	acbBot.init(configJSON)          – initialise for a new match
//	acbBot.compute_moves(stateJSON)  – return movesJSON for the current turn
package main

import (
	"encoding/json"
	"math/rand"
	"syscall/js"
	"time"

	"github.com/aicodebattle/acb/cmd/acb-wasm/strategies"
	"github.com/aicodebattle/acb/engine"
)

var rng *rand.Rand

func jsInit(_ js.Value, args []js.Value) interface{} {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	return map[string]interface{}{"ok": true}
}

func jsComputeMoves(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{"ok": false, "error": "stateJSON required"}
	}
	var state engine.VisibleState
	if err := json.Unmarshal([]byte(args[0].String()), &state); err != nil {
		return map[string]interface{}{"ok": false, "error": "parse: " + err.Error()}
	}
	bot := strategies.New("swarm", rng)
	moves, _ := bot.GetMoves(&state)
	b, _ := json.Marshal(moves)
	return string(b)
}

func main() {
	done := make(chan struct{})
	js.Global().Set("acbBot", js.ValueOf(map[string]interface{}{
		"init":          js.FuncOf(jsInit),
		"compute_moves": js.FuncOf(jsComputeMoves),
	}))
	<-done
}
