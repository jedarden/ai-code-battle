package main

import "encoding/json"

// Position represents a coordinate on the grid.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Direction represents a movement direction.
type Direction int

const (
	DirNone Direction = iota
	DirN
	DirE
	DirS
	DirW
)

// String returns the string representation of a direction.
func (d Direction) String() string {
	switch d {
	case DirN:
		return "N"
	case DirE:
		return "E"
	case DirS:
		return "S"
	case DirW:
		return "W"
	default:
		return ""
	}
}

// MarshalJSON serializes Direction as a string.
func (d Direction) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// VisibleBot represents a bot visible to this player.
type VisibleBot struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// VisibleCore represents a core visible to this player.
type VisibleCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}

// You contains information about the current player.
type You struct {
	ID     int `json:"id"`
	Energy int `json:"energy"`
	Score  int `json:"score"`
}

// VisibleState represents the fog-filtered game state visible to this player.
type VisibleState struct {
	MatchID string         `json:"match_id"`
	Turn    int            `json:"turn"`
	Config  map[string]any `json:"config"`
	You     You            `json:"you"`
	Bots    []VisibleBot   `json:"bots"`
	Energy  []Position     `json:"energy"`
	Cores   []VisibleCore  `json:"cores"`
	Walls   []Position     `json:"walls"`
	Dead    []VisibleBot   `json:"dead"`
}

// Move represents a bot's movement order.
type Move struct {
	Position  Position  `json:"position"`
	Direction Direction `json:"direction"`
}
