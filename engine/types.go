// Package engine implements the AI Code Battle game simulation.
package engine

import (
	"encoding/json"
	"fmt"
	"math"
)

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

// MarshalJSON serializes Direction as a string ("N", "E", "S", "W", or "").
func (d Direction) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts both string ("N") and integer (1) representations.
func (d *Direction) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*d = ParseDirection(s)
		return nil
	}
	var i int
	if err := json.Unmarshal(data, &i); err != nil {
		return fmt.Errorf("direction must be a string or integer: %w", err)
	}
	*d = Direction(i)
	return nil
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
	ID       int      `json:"id"`
	Owner    int      `json:"owner"`
	Position Position `json:"position"`
	Alive    bool     `json:"alive"`
}

// Core represents a spawn point owned by a player.
type Core struct {
	Position        Position `json:"position"`
	Owner           int      `json:"owner"`
	Active          bool     `json:"active"`            // false if razed
	ID              int      `json:"id"`                // unique core identifier
	LastSpawnedTurn int      `json:"last_spawned_turn"` // turn when this core last spawned a bot
}

// EnergyNode represents an energy spawn location.
type EnergyNode struct {
	Position  Position `json:"position"`
	HasEnergy bool     `json:"has_energy"` // true if energy is currently collectible
	Tick      int      `json:"tick"`       // turns since last spawn
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
	Position  Position  `json:"position"` // current position of bot to move
	Direction Direction `json:"direction"`
}

// Config holds game configuration parameters.
type Config struct {
	Rows           int    `json:"rows"`
	Cols           int    `json:"cols"`
	MaxTurns       int    `json:"max_turns"`
	VisionRadius2  int    `json:"vision_radius2"`   // squared vision distance
	AttackRadius2  int    `json:"attack_radius2"`   // squared attack distance
	SpawnCost      int    `json:"spawn_cost"`       // energy cost to spawn a bot
	EnergyInterval int    `json:"energy_interval"`  // turns between energy spawns
	CoresPerPlayer int    `json:"cores_per_player"` // starting cores per player
	MapID          string `json:"map_id,omitempty"`
	SeasonID       string `json:"season_id,omitempty"`
	RulesVersion   string `json:"rules_version,omitempty"`

	// Zone (storm) configuration
	ZoneEnabled        bool `json:"zone_enabled"`         // whether the shrinking zone is active
	ZoneStartTurn      int  `json:"zone_start_turn"`      // turn when zone starts shrinking
	ZoneShrinkInterval int  `json:"zone_shrink_interval"` // turns between shrink steps
	ZoneShrinkStep     int  `json:"zone_shrink_step"`     // tiles to shrink each step
	ZoneMinRadius      int  `json:"zone_min_radius"`      // minimum zone radius (stops here)
}

// DefaultConfig returns the default game configuration.
func DefaultConfig() Config {
	return Config{
		Rows:               40,
		Cols:               40,
		MaxTurns:           500,
		VisionRadius2:      49, // ~7 tiles
		AttackRadius2:      9,  // 3 tiles (increased from 5 for better combat trigger)
		SpawnCost:          3,
		EnergyInterval:     10,
		CoresPerPlayer:     2,
		ZoneEnabled:        true,
		ZoneStartTurn:      50,
		ZoneShrinkInterval: 5,
		ZoneShrinkStep:     2,
		ZoneMinRadius:      3,
	}
}

// ConfigForPlayers returns a config scaled for the given player count and cores per player.
// For 2 players, uses 40x40 (800 tiles per player) to increase encounter frequency.
// For 3+ players, uses ~2000 tiles per player (following aichallenge Ants sizing).
func ConfigForPlayers(numPlayers, coresPerPlayer int) Config {
	cfg := DefaultConfig()
	cfg.CoresPerPlayer = coresPerPlayer
	if coresPerPlayer < 1 {
		cfg.CoresPerPlayer = 1
	}

	// Scale grid: smaller maps for 2-player, ~1000 tiles/player for 3+ (high combat density)
	var areaPerPlayer int
	if numPlayers == 2 {
		areaPerPlayer = 800 // 40x40 for 2 players
	} else {
		areaPerPlayer = 1000 // Reduced from 2000 to force more contact
	}
	totalArea := areaPerPlayer * numPlayers
	side := int(math.Sqrt(float64(totalArea)))

	// Clamp to valid range
	if side < 30 {
		side = 30
	}
	if side > 200 {
		side = 200
	}

	cfg.Rows = side
	cfg.Cols = side

	// Scale max turns with map size
	cfg.MaxTurns = side * 8 // larger maps get more turns

	// Scale energy nodes with player count
	cfg.EnergyInterval = 10

	// Scale zone parameters to force combat contact
	// Zone must start early to force combat before energy farming wins
	// ZoneMinRadius must be >= spawn radius so bots aren't killed before they can reach attack range
	// Faster shrink (interval 2) forces quicker engagement
	if numPlayers == 2 {
		cfg.ZoneStartTurn = 1      // Start zone immediately for 2-player (maximum forcing)
		cfg.ZoneShrinkInterval = 2 // Shrink every 2 turns (faster pressure)
		cfg.ZoneShrinkStep = 1     // Shrink 1 tile per interval (0.5 tiles/turn, bots can keep up)
		cfg.ZoneMinRadius = 3      // Final zone diameter (6) is closer to attack radius (3.5)
		cfg.AttackRadius2 = 36     // 6 tiles (maximum for 2-player random bots)
	} else {
		cfg.ZoneStartTurn = 15     // Start zone early for 3+ players (larger gap to close)
		cfg.ZoneShrinkInterval = 2 // Shrink every 2 turns (faster pressure)
		cfg.ZoneShrinkStep = 1     // Shrink 1 tile per interval (0.5 tiles/turn, bots can keep up)
		cfg.ZoneMinRadius = 3      // Final zone diameter (6) forces bots into attack range (3.5)
		cfg.AttackRadius2 = 12     // 3.5 tiles (same as 2-player for better combat trigger)
	}

	return cfg
}

// MatchResult represents the outcome of a match.
type MatchResult struct {
	Winner       int    `json:"winner"` // -1 for draw
	Reason       string `json:"reason"` // "elimination", "dominance", "turns", "draw"
	Turns        int    `json:"turns"`
	Scores       []int  `json:"scores"`
	Energy       []int  `json:"energy"` // energy collected per player
	BotsAlive    []int  `json:"bots_alive"`
	Crashed      []bool `json:"crashed"`       // per-player: true if bot was marked crashed during match
	CombatDeaths []int  `json:"combat_deaths"` // bots killed in combat per player (focus-fire)
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
	Bots   []VisibleBot  `json:"bots"`
	Energy []Position    `json:"energy"`
	Cores  []VisibleCore `json:"cores"`
	Walls  []Position    `json:"walls"`
	Dead   []VisibleBot  `json:"dead"`
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
