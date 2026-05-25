// Build with: GOOS=js GOARCH=wasm go build -o engine.wasm ./wasm/engine
//go:build js

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/aicodebattle/acb/engine"
)

var (
	match *engine.Match
)

// init registers the WASM exports
func init() {
	c := make(chan struct{})
	js.Global().Set("acbEngine", js.ValueOf(map[string]interface{}{
		"loadState": jsWrapper(loadState),
		"step":      jsWrapper(step),
		"runMatch":  jsWrapper(runMatch),
		"getReplay": jsWrapper(getReplay),
		"getBots":   jsWrapper(getBots),
		"getEnergy": jsWrapper(getEnergy),
		"getConfig": jsWrapper(getConfig),
		"getState":  jsWrapper(getState),
	}))
	fmt.Println("ACB WASM Engine loaded")
	close(c)
}

func main() {
	// Keep the program running
	select {}
}

// jsWrapper converts a Go function to a JS function
func jsWrapper(fnc func(js.Value, []js.Value) interface{}) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		defer func() {
			if r := recover(); r != nil {
				js.Global().Get("console").Call("error", fmt.Sprintf("panic: %v", r))
			}
		}()
		return fnc(this, args)
	})
}

// loadState loads a game state from JSON
func loadState(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResult("loadState requires state JSON argument")
	}

	stateJSON := args[0].String()

	// Create a new match from the state
	var err error
	match, err = engine.LoadStateJSON(stateJSON)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to load state: %v", err))
	}

	return successResult(nil)
}

// step advances one turn with the given moves
func step(_ js.Value, args []js.Value) interface{} {
	if match == nil {
		return errorResult("no match loaded - call loadState or runMatch first")
	}

	if len(args) < 1 {
		return errorResult("step requires moves JSON argument")
	}

	movesJSON := args[0].String()
	var moves map[int]engine.Move
	if err := json.Unmarshal([]byte(movesJSON), &moves); err != nil {
		return errorResult(fmt.Sprintf("invalid moves JSON: %v", err))
	}

	// Execute one turn
	turnState, err := match.StepTurn(moves)
	if err != nil {
		return errorResult(fmt.Sprintf("turn execution failed: %v", err))
	}

	return successResult(turnState)
}

// runMatch runs a full match with the given config and map
func runMatch(_ js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return errorResult("runMatch requires config and map JSON arguments")
	}

	configJSON := args[0].String()
	mapJSON := args[1].String()

	var config engine.Config
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return errorResult(fmt.Sprintf("invalid config JSON: %v", err))
	}

	// Create new match
	var err error
	match, err = engine.NewMatch(config, mapJSON)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create match: %v", err))
	}

	// Run the match
	result, err := match.Run()
	if err != nil {
		return errorResult(fmt.Sprintf("match execution failed: %v", err))
	}

	return successResult(result)
}

// getReplay returns the current replay JSON
func getReplay(_ js.Value, args []js.Value) interface{} {
	if match == nil {
		return errorResult("no match loaded")
	}

	replay := match.GetReplayJSON()
	return successResult(replay)
}

// getBots returns current bot positions
func getBots(_ js.Value, args []js.Value) interface{} {
	if match == nil {
		return errorResult("no match loaded")
	}

	bots := match.GetBotsJSON()
	return successResult(bots)
}

// getEnergy returns current energy positions
func getEnergy(_ js.Value, args []js.Value) interface{} {
	if match == nil {
		return errorResult("no match loaded")
	}

	energy := match.GetEnergyJSON()
	return successResult(energy)
}

// getConfig returns the match config
func getConfig(_ js.Value, args []js.Value) interface{} {
	if match == nil {
		return errorResult("no match loaded")
	}

	config := match.GetConfigJSON()
	return successResult(config)
}

// getState returns the full current game state
func getState(_ js.Value, args []js.Value) interface{} {
	if match == nil {
		return errorResult("no match loaded")
	}

	state := match.GetStateJSON()
	return successResult(state)
}

func successResult(data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"ok":   true,
		"data": data,
	}
}

func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"ok":    false,
		"error": msg,
	}
}
