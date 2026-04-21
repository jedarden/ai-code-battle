//go:build js && wasm

// Package main is compiled with GOOS=js GOARCH=wasm to produce engine.wasm.
// It exposes functions on the global acbEngine object:
//
//	acbEngine.loadState(stateJSON)      – load a serialised GameState
//	acbEngine.step(movesJSON)           – advance one turn; returns {state,result}
//	acbEngine.runMatch(configJSON)      – run a full 2-player match with built-in bots
//	acbEngine.addPlayer(name, fn)       – register a player with a JS callback strategy
//	acbEngine.clearPlayers()            – clear all registered players
//	acbEngine.runMatchMulti(configJSON) – run a match with all registered players
//	acbEngine.version                   – version string
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
	gs       *engine.GameState
	bots     []engine.BotInterface
	recorder *engine.ReplayWriter
}

var session *matchSession

// jsPlayers holds JS callback strategies registered via addPlayer.
var jsPlayers []jsPlayerEntry

type jsPlayerEntry struct {
	name string
	fn   js.Value // JS function: (stateJSON: string) => string (moves JSON)
}

// jsBot wraps a JS callback as an engine.BotInterface.
type jsBot struct {
	fn js.Value
}

func (b *jsBot) GetMoves(state *engine.VisibleState) ([]engine.Move, error) {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	result := b.fn.Invoke(string(stateJSON))
	if result.IsUndefined() || result.IsNull() {
		return []engine.Move{}, nil
	}

	var moves []engine.Move
	if err := json.Unmarshal([]byte(result.String()), &moves); err != nil {
		return []engine.Move{}, nil
	}
	return moves, nil
}

// jsLoadState parses a serialised GameState JSON and stores it as the active session.
// Signature: acbEngine.loadState(stateJSON: string) => {ok:bool, error?:string}
func jsLoadState(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return jsErr("stateJSON argument required")
	}
	type initRequest struct {
		Config engine.Config `json:"config"`
		Seed   int64         `json:"seed"`
		Bot1   string        `json:"bot1"`
		Bot2   string        `json:"bot2"`
	}
	var req initRequest
	if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
		return jsErr("parse error: " + err.Error())
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

	gs := engine.NewGameState(cfg, rng)

	bot1 := newBuiltinBot(req.Bot1, rng)
	bot2 := newBuiltinBot(req.Bot2, rng)

	mr := engine.NewMatchRunner(cfg, engine.WithRNG(rand.New(rand.NewSource(seed))))
	mr.AddBot(bot1, req.Bot1)
	mr.AddBot(bot2, req.Bot2)

	_ = gs

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

	if len(args) > 0 && args[0].String() != "" {
		var moves []engine.Move
		if err := json.Unmarshal([]byte(args[0].String()), &moves); err != nil {
			return jsErr("parse moves: " + err.Error())
		}
		for _, m := range moves {
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

// jsRunMatch executes a complete 2-player match with built-in bots.
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

// jsAddPlayer registers a player with a JS callback strategy.
// Signature: acbEngine.addPlayer(name: string, fn: (stateJSON: string) => string) => {ok:bool}
func jsAddPlayer(_ js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return jsErr("addPlayer requires (name, fn) arguments")
	}
	name := args[0].String()
	fn := args[1]

	jsPlayers = append(jsPlayers, jsPlayerEntry{
		name: name,
		fn:   fn,
	})

	return map[string]interface{}{
		"ok":   true,
		"index": len(jsPlayers) - 1,
	}
}

// jsClearPlayers clears all registered players.
// Signature: acbEngine.clearPlayers() => {ok:bool}
func jsClearPlayers(_ js.Value, _ []js.Value) interface{} {
	jsPlayers = jsPlayers[:0]
	return map[string]interface{}{"ok": true}
}

// jsRunMatchMulti runs a match with all registered JS callback players.
// Signature: acbEngine.runMatchMulti(configJSON: string) => {replay, result}
func jsRunMatchMulti(_ js.Value, args []js.Value) interface{} {
	if len(jsPlayers) < 2 {
		return jsErr("need at least 2 registered players; use addPlayer() first")
	}

	type configRequest struct {
		Config engine.Config `json:"config"`
		Seed   int64         `json:"seed"`
	}

	var req configRequest
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

	mr := engine.NewMatchRunner(cfg,
		engine.WithRNG(rng),
		engine.WithTimeout(2*time.Second),
	)

	for i, p := range jsPlayers {
		mr.AddBot(&jsBot{fn: p.fn}, p.name)
		_ = i
	}

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
		"addPlayer": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsAddPlayer(this, args)
		}),
		"clearPlayers": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsClearPlayers(this, args)
		}),
		"runMatchMulti": js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			return jsRunMatchMulti(this, args)
		}),
		"version": "2.0.0",
	}))

	<-done
}
