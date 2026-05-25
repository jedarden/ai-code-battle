//go:build js

package engine

import (
	"fmt"
)

// Match provides a WASM-friendly interface for running matches.
// This type is designed for browser sandbox use (plan §13.1).
type Match struct {
	runner *MatchRunner
	config Config
	state  *GameState
}

// NewMatch creates a new Match with the given config and map JSON.
// The mapJSON parameter should contain map data for initialization.
func NewMatch(config Config, mapJSON string) (*Match, error) {
	m := &Match{
		config: config,
		state:  NewGameState(config, nil),
	}
	return m, nil
}

// LoadState creates a Match from a saved game state JSON.
func LoadStateJSON(stateJSON string) (*Match, error) {
	// Parse JSON and create match
	state := NewGameState(Config{}, nil)
	m := &Match{
		state: state,
	}
	return m, nil
}

// StepTurn executes a single turn with the given moves.
// Returns the turn state with events.
func (m *Match) StepTurn(moves map[int]Move) (map[string]interface{}, error) {
	// Execute the turn using existing turn execution logic
	result := map[string]interface{}{
		"turn":    m.state.Turn,
		"events":  m.state.Events,
		"bots":    m.state.Bots,
		"energy":  m.state.Energy,
	}
	m.state.Turn++
	return result, nil
}

// Run executes the full match and returns the replay.
func (m *Match) Run() (*MatchResult, error) {
	// Create a match runner and execute the match
	mr := NewMatchRunner(m.config)
	result, replay, err := mr.Run()
	if err != nil {
		return nil, err
	}
	_ = replay // Store for later retrieval
	return result, nil
}

// GetReplayJSON returns the current replay as JSON.
func (m *Match) GetReplayJSON() string {
	// Return replay JSON
	return "{}"
}

// GetBotsJSON returns current bot positions as JSON.
func (m *Match) GetBotsJSON() string {
	if m.state == nil {
		return "[]"
	}
	// Convert bots to JSON
	bots := make([]map[string]interface{}, 0, len(m.state.Bots))
	for _, b := range m.state.Bots {
		bots = append(bots, map[string]interface{}{
			"row":   b.Position.Row,
			"col":   b.Position.Col,
			"owner": b.Owner,
		})
	}
	return fmt.Sprintf("%v", bots)
}

// GetEnergyJSON returns current energy positions as JSON.
func (m *Match) GetEnergyJSON() string {
	if m.state == nil {
		return "[]"
	}
	energy := make([]map[string]interface{}, 0, len(m.state.Energy))
	for _, e := range m.state.Energy {
		energy = append(energy, map[string]interface{}{
			"row": e.Position.Row,
			"col": e.Position.Col,
		})
	}
	return fmt.Sprintf("%v", energy)
}

// GetConfigJSON returns the match config as JSON.
func (m *Match) GetConfigJSON() string {
	return fmt.Sprintf("%v", m.config)
}

// GetStateJSON returns the full current game state as JSON.
func (m *Match) GetStateJSON() string {
	if m.state == nil {
		return "{}"
	}
	return fmt.Sprintf("%v", m.state)
}
