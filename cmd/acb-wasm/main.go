//go:build js && wasm

// Package main is compiled with GOOS=js GOARCH=wasm to produce engine.wasm.
// It exposes three functions on the global acbEngine object:
//
//	acbEngine.loadState(stateJSON)    – load a serialised GameState
//	acbEngine.step(movesJSON)         – advance one turn; returns {state,result}
//	acbEngine.runMatch(configJSON)    – run a full match; returns {replay,result}
//
// Example (JavaScript):
//
//	const go = new Go();
//	WebAssembly.instantiateStreaming(fetch('/wasm/engine.wasm'), go.importObject)
//	  .then(({instance}) => { go.run(instance); });
//	// acbEngine is now available
package main

import (
	"encoding/json"
	"math/rand"
	"syscall/js"
	"time"

	"github.com/aicodebattle/acb/engine"
)

// matchSession holds a running match for turn-by-turn access.
type matchSession struct {
	gs      *engine.GameState
	bots    []engine.BotInterface
	recorder *engine.ReplayWriter
}

var session *matchSession

// jsLoadState parses a serialised GameState JSON and stores it as the active session.
// Signature: acbEngine.loadState(stateJSON: string) => {ok:bool, error?:string}
func jsLoadState(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsErr("stateJSON argument required")
	}
	// For now, we expect an initialisation config rather than a full state dump.
	type initRequest struct {
		Config     engine.Config `json:"config"`
		Seed       int64         `json:"seed"`
		Bot1       string        `json:"bot1"` // strategy name
		Bot2       string        `json:"bot2"` // strategy name
	}
	var req initRequest
	if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
		return jsErr("parse error: " + err.Error())
	}

	cfg := req.Config
	if cfg.Rows == 0 {
		cfg = engine.DefaultConfig()
		// Smaller default for in-browser matches
		cfg.Rows = 30
		cfg.Cols = 30
		cfg.MaxTurns = 200
	}

	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	gs := engine.NewGameState(cfg, rng)

	bot1 := newBuiltinBot(req.Bot1, rng)
	bot2 := newBuiltinBot(req.Bot2, rng)

	mr := engine.NewMatchRunner(cfg, engine.WithRNG(rand.New(rand.NewSource(seed))))
	mr.AddBot(bot1, req.Bot1)
	mr.AddBot(bot2, req.Bot2)

	_ = gs // session setup done via match runner below

	session = &matchSession{
		gs:   gs,
		bots: []engine.BotInterface{bot1, bot2},
	}

	return map[string]interface{}{"ok": true}
}

// jsStep advances one turn.
// Signature: acbEngine.step(movesJSON: string) => {state, events, result?}
func jsStep(_ js.Value, args []js.Value) interface{} {
	if session == nil {
		return jsErr("no active session; call loadState first")
	}
	gs := session.gs

	// Parse moves from caller (if provided)
	if len(args) > 0 && args[0].String() != "" {
		var moves []engine.Move
		if err := json.Unmarshal([]byte(args[0].String()), &moves); err != nil {
			return jsErr("parse moves: " + err.Error())
		}
		for _, m := range moves {
			// Find bot at position and submit move
			for _, b := range gs.Bots {
				if b.Alive && b.Position == m.Position {
					gs.Moves[b.ID] = m
				}
			}
		}
	}

	result := gs.ExecuteTurn()

	stateJSON, _ := json.Marshal(gs)
	eventsJSON, _ := json.Marshal(gs.Events)

	out := map[string]interface{}{
		"state":  string(stateJSON),
		"events": string(eventsJSON),
		"turn":   gs.Turn,
	}
	if result != nil {
		resultJSON, _ := json.Marshal(result)
		out["result"] = string(resultJSON)
	}
	return out
}

// jsRunMatch executes a complete match and returns the replay.
// Signature: acbEngine.runMatch(configJSON: string) => {replay, result}
func jsRunMatch(_ js.Value, args []js.Value) interface{} {
	type runRequest struct {
		Config engine.Config `json:"config"`
		Bot1   string        `json:"bot1"`
		Bot2   string        `json:"bot2"`
		Seed   int64         `json:"seed"`
	}

	var req runRequest
	if len(args) > 0 && args[0].String() != "" {
		if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
			return jsErr("parse config: " + err.Error())
		}
	}

	cfg := req.Config
	if cfg.Rows == 0 {
		cfg = engine.DefaultConfig()
		cfg.Rows = 30
		cfg.Cols = 30
		cfg.MaxTurns = 200
	}

	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	bot1Name := req.Bot1
	if bot1Name == "" {
		bot1Name = "random"
	}
	bot2Name := req.Bot2
	if bot2Name == "" {
		bot2Name = "random"
	}

	mr := engine.NewMatchRunner(cfg,
		engine.WithRNG(rng),
		engine.WithTimeout(500*time.Millisecond),
	)
	mr.AddBot(newBuiltinBot(bot1Name, rand.New(rand.NewSource(seed))), bot1Name)
	mr.AddBot(newBuiltinBot(bot2Name, rand.New(rand.NewSource(seed+1))), bot2Name)

	result, replay, err := mr.Run()
	if err != nil {
		return jsErr("run match: " + err.Error())
	}

	replayJSON, _ := json.Marshal(replay)
	resultJSON, _ := json.Marshal(result)
	return map[string]interface{}{
		"replay": string(replayJSON),
		"result": string(resultJSON),
	}
}

func jsErr(msg string) map[string]interface{} {
	return map[string]interface{}{"ok": false, "error": msg}
}

func main() {
	done := make(chan struct{})

	js.Global().Set("acbEngine", js.ValueOf(map[string]interface{}{
		"loadState": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsLoadState(this, args)
		}),
		"step": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsStep(this, args)
		}),
		"runMatch": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsRunMatch(this, args)
		}),
		"version": "1.0.0",
	}))

	<-done
}
