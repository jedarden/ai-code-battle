// Package game provides shared types and utilities for AI Code Battle bots.
package game

import (
	"fmt"
	"time"
)

// Direction represents a cardinal movement direction.
type Direction string

const (
	DirN Direction = "N"
	DirE Direction = "E"
	DirS Direction = "S"
	DirW Direction = "W"
)

// AllDirections returns all four cardinal directions.
func AllDirections() []Direction {
	return []Direction{DirN, DirE, DirS, DirW}
}

// GameConfig holds the match configuration sent by the engine.
type GameConfig struct {
	Rows           int    `json:"rows"`
	Cols           int    `json:"cols"`
	MaxTurns       int    `json:"max_turns"`
	VisionRadius2  int    `json:"vision_radius2"`
	AttackRadius2  int    `json:"attack_radius2"`
	SpawnCost      int    `json:"spawn_cost"`
	EnergyInterval int    `json:"energy_interval"`
	SeasonID       string `json:"season_id,omitempty"`
	RulesVersion   string `json:"rules_version,omitempty"`
}

// Position represents a grid coordinate.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// VisibleBot represents a bot visible within fog of war.
type VisibleBot struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// VisibleCore represents a core visible within fog of war.
type VisibleCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}

// GameState represents the fog-filtered game state for a single turn.
type GameState struct {
	MatchID string        `json:"match_id"`
	Turn    int           `json:"turn"`
	Config  GameConfig    `json:"config"`
	You     PlayerInfo    `json:"you"`
	Bots    []VisibleBot  `json:"bots"`
	Energy  []Position    `json:"energy"`
	Cores   []VisibleCore `json:"cores"`
	Walls   []Position    `json:"walls"`
	Dead    []VisibleBot  `json:"dead"`
}

// PlayerInfo contains information about the current player.
type PlayerInfo struct {
	ID     int `json:"id"`
	Energy int `json:"energy"`
	Score  int `json:"score"`
}

// Move represents a movement order for a single bot.
type Move struct {
	Position  Position  `json:"position"`
	Direction Direction `json:"direction"`
}

// MoveResponse is the response sent back to the engine.
type MoveResponse struct {
	Moves []Move `json:"moves"`
}

// AuthHeaders contains the authentication headers from a request.
type AuthHeaders struct {
	MatchID   string
	Turn      string
	Timestamp string
	Signature string
}

// VerifyTimestamp checks if the timestamp is within the allowed window.
// Accepts both RFC3339 and Unix timestamp formats.
func VerifyTimestamp(timestamp string) bool {
	// Try RFC3339 first
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		// Try Unix timestamp (seconds since epoch)
		// Parse as integer for Unix timestamp
		var sec int64
		n, err := fmt.Sscanf(timestamp, "%d", &sec)
		if err != nil || n != 1 {
			return false
		}
		ts = time.Unix(sec, 0)
	}

	now := time.Now()
	// Allow ±30 seconds to prevent replay attacks
	diff := now.Sub(ts).Seconds()
	return diff >= -30 && diff <= 30
}
