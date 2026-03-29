//go:build js && wasm

package main

import (
	"math/rand"

	"github.com/aicodebattle/acb/cmd/acb-wasm/strategies"
	"github.com/aicodebattle/acb/engine"
)

// newBuiltinBot creates one of the six built-in strategy bots by name.
func newBuiltinBot(name string, rng *rand.Rand) engine.BotInterface {
	return strategies.New(name, rng)
}
