// Package engine implements the AI Code Battle game simulation.
package engine

// Position represents a coordinate on the toroidal grid.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Tile represents the type of a grid cell.
type Tile int

const (
	TileOpen Tile = iota
	TileWall
	TileEnergy
	TileCore
)

// String returns the symbol representation of a tile.
func (t Tile) String() string {
	switch t {
	case TileOpen:
		return "."
	case TileWall:
		return "#"
	case TileEnergy:
		return "*"
	case TileCore:
		return "C"
	default:
		return "?"
	}
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

// ParseDirection parses a direction string.
func ParseDirection(s string) Direction {
	switch s {
	case "N":
		return DirN
	case "E":
		return DirE
	case "S":
		return DirS
	case "W":
		return DirW
	default:
		return DirNone
	}
}

// Delta returns the row and column delta for a direction.
func (d Direction) Delta() (dr, dc int) {
	switch d {
	case DirN:
		return -1, 0
	case DirE:
		return 0, 1
	case DirS:
		return 1, 0
	case DirW:
		return 0, -1
	default:
		return 0, 0
	}
}

// Bot represents a unit on the grid.
type Bot struct {
	ID       int `json:"id"`
	Owner    int `json:"owner"`
	Position Position `json:"position"`
	Alive    bool `json:"alive"`
}

// Core represents a spawn point owned by a player.
type Core struct {
	Position Position `json:"position"`
	Owner    int `json:"owner"`
	Active   bool `json:"active"` // false if razed
}

// EnergyNode represents an energy spawn location.
type EnergyNode struct {
	Position  Position `json:"position"`
	HasEnergy bool `json:"has_energy"` // true if energy is currently collectible
	Tick      int `json:"tick"`        // turns since last spawn
}

// Player represents a participant in the match.
type Player struct {
	ID       int `json:"id"`
	Energy   int `json:"energy"`
	Score    int `json:"score"`
	BotCount int `json:"bot_count"`
}

// Move represents a bot's movement order.
// Bots are identified by their position in the fog-filtered state.
type Move struct {
	Position  Position  `json:"position"`  // current position of bot to move
	Direction Direction `json:"direction"`
}

// Config holds game configuration parameters.
type Config struct {
	Rows           int `json:"rows"`
	Cols           int `json:"cols"`
	MaxTurns       int `json:"max_turns"`
	VisionRadius2  int `json:"vision_radius2"`  // squared vision distance
	AttackRadius2  int `json:"attack_radius2"`  // squared attack distance
	SpawnCost      int `json:"spawn_cost"`      // energy cost to spawn a bot
	EnergyInterval int `json:"energy_interval"` // turns between energy spawns
}

// DefaultConfig returns the default game configuration.
func DefaultConfig() Config {
	return Config{
		Rows:           60,
		Cols:           60,
		MaxTurns:       500,
		VisionRadius2:  49, // ~7 tiles
		AttackRadius2:  5,  // ~2.24 tiles
		SpawnCost:      3,
		EnergyInterval: 10,
	}
}

// MatchResult represents the outcome of a match.
type MatchResult struct {
	Winner      int    `json:"winner"`       // -1 for draw
	Reason      string `json:"reason"`       // "elimination", "dominance", "turns", "draw"
	Turns       int    `json:"turns"`
	Scores      []int  `json:"scores"`
	Energy      []int  `json:"energy"`       // energy collected per player
	BotsAlive   []int  `json:"bots_alive"`
}

// BotInterface defines the interface for bot decision-making.
// In Phase 1, this is implemented by local bots communicating via stdin/stdout.
type BotInterface interface {
	// GetMoves returns the bot's moves for the current turn.
	// state is the fog-filtered game state visible to this player.
	GetMoves(state *VisibleState) ([]Move, error)
}

// VisibleState represents the game state filtered by fog of war for a specific player.
type VisibleState struct {
	MatchID string `json:"match_id"`
	Turn    int    `json:"turn"`
	Config  Config `json:"config"`
	You     struct {
		ID     int `json:"id"`
		Energy int `json:"energy"`
		Score  int `json:"score"`
	} `json:"you"`
	Bots   []VisibleBot `json:"bots"`
	Energy []Position   `json:"energy"`
	Cores  []VisibleCore `json:"cores"`
	Walls  []Position   `json:"walls"`
	Dead   []VisibleBot `json:"dead"`
}

// VisibleBot represents a bot visible to a player.
type VisibleBot struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// VisibleCore represents a core visible to a player.
type VisibleCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}
