package engine

import (
	"encoding/json"
	"fmt"
	"math/rand"
)

// GameState represents the complete state of a match.
type GameState struct {
	Config    Config
	Grid      *Grid
	Bots      []*Bot
	Cores     []*Core
	Energy    []*EnergyNode
	Players   []*Player
	Turn      int
	MatchID   string
	NextBotID int
	rng       *rand.Rand

	// Turn state
	Moves       map[int]Move       // bot ID -> move
	DeadBots    []*Bot             // bots that died this turn (for fog display)
	Events      []Event            // events that occurred this turn
	Dominance   map[int]int        // player -> consecutive turns with 80%+ bots

	// Stalemate detection
	StalemateTurns  int // consecutive turns with no progress
	LastTotalEnergy int // total energy held by all players at last progress
	LastTotalBots   int // total living bots at last progress
}

// Event represents something that happened during a turn.
type Event struct {
	Type    string      `json:"type"`
	Turn    int         `json:"turn"`
	Details interface{} `json:"details"`
}

// Event types
const (
	EventBotSpawned    = "bot_spawned"
	EventBotDied       = "bot_died"
	EventEnergyCollected = "energy_collected"
	EventCoreCaptured  = "core_captured"
	EventCombatDeath   = "combat_death"
	EventCollisionDeath = "collision_death"
)

// NewGameState creates a new game state with the given configuration.
func NewGameState(config Config, rng *rand.Rand) *GameState {
	return &GameState{
		Config:    config,
		Grid:      NewGrid(config.Rows, config.Cols),
		Bots:      make([]*Bot, 0),
		Cores:     make([]*Core, 0),
		Energy:    make([]*EnergyNode, 0),
		Players:   make([]*Player, 0),
		Turn:      0,
		MatchID:   generateMatchID(rng),
		NextBotID: 0,
		rng:       rng,
		Moves:     make(map[int]Move),
		DeadBots:  make([]*Bot, 0),
		Events:    make([]Event, 0),
		Dominance: make(map[int]int),
	}
}

// generateMatchID creates a random match identifier.
func generateMatchID(rng *rand.Rand) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return "m_" + string(b)
}

// AddPlayer adds a new player to the game.
func (gs *GameState) AddPlayer() *Player {
	p := &Player{
		ID:     len(gs.Players),
		Energy: 0,
		Score:  0,
	}
	gs.Players = append(gs.Players, p)
	gs.Dominance[p.ID] = 0
	return p
}

// AddCore adds a core for a player at the given position.
func (gs *GameState) AddCore(owner int, pos Position) *Core {
	c := &Core{
		Position: gs.Grid.WrapPos(pos),
		Owner:    owner,
		Active:   true,
	}
	gs.Cores = append(gs.Cores, c)
	gs.Grid.SetPos(c.Position, TileCore)

	// Player starts with 1 point per core
	if owner < len(gs.Players) {
		gs.Players[owner].Score++
	}

	return c
}

// AddEnergyNode adds an energy node at the given position.
func (gs *GameState) AddEnergyNode(pos Position) *EnergyNode {
	en := &EnergyNode{
		Position:  gs.Grid.WrapPos(pos),
		HasEnergy: false,
		Tick:      0,
	}
	gs.Energy = append(gs.Energy, en)
	return en
}

// SpawnBot spawns a new bot for a player at the given position.
func (gs *GameState) SpawnBot(owner int, pos Position) *Bot {
	b := &Bot{
		ID:       gs.NextBotID,
		Owner:    owner,
		Position: gs.Grid.WrapPos(pos),
		Alive:    true,
	}
	gs.NextBotID++
	gs.Bots = append(gs.Bots, b)

	if owner < len(gs.Players) {
		gs.Players[owner].BotCount++
	}

	gs.Events = append(gs.Events, Event{
		Type: EventBotSpawned,
		Turn: gs.Turn,
		Details: map[string]interface{}{
			"bot_id": b.ID,
			"owner":  owner,
			"pos":    b.Position,
		},
	})

	return b
}

// GetPlayerBots returns all living bots for a player.
func (gs *GameState) GetPlayerBots(playerID int) []*Bot {
	var bots []*Bot
	for _, b := range gs.Bots {
		if b.Alive && b.Owner == playerID {
			bots = append(bots, b)
		}
	}
	return bots
}

// GetLivingBotCount returns the count of living bots.
func (gs *GameState) GetLivingBotCount() int {
	count := 0
	for _, b := range gs.Bots {
		if b.Alive {
			count++
		}
	}
	return count
}

// GetPlayerLivingBotCount returns the count of living bots for a player.
func (gs *GameState) GetPlayerLivingBotCount(playerID int) int {
	count := 0
	for _, b := range gs.Bots {
		if b.Alive && b.Owner == playerID {
			count++
		}
	}
	return count
}

// GetLivingPlayers returns IDs of players with at least one living bot.
func (gs *GameState) GetLivingPlayers() []int {
	alive := make(map[int]bool)
	for _, b := range gs.Bots {
		if b.Alive {
			alive[b.Owner] = true
		}
	}
	var result []int
	for pid := range alive {
		result = append(result, pid)
	}
	return result
}

// KillBot marks a bot as dead and records the event.
func (gs *GameState) KillBot(bot *Bot, reason string) {
	if !bot.Alive {
		return
	}
	bot.Alive = false
	gs.DeadBots = append(gs.DeadBots, bot)

	if bot.Owner < len(gs.Players) {
		gs.Players[bot.Owner].BotCount--
	}

	gs.Events = append(gs.Events, Event{
		Type: EventBotDied,
		Turn: gs.Turn,
		Details: map[string]interface{}{
			"bot_id": bot.ID,
			"owner":  bot.Owner,
			"pos":    bot.Position,
			"reason": reason,
		},
	})
}

// SubmitMove records a move for a bot at a given position.
func (gs *GameState) SubmitMove(pos Position, dir Direction) {
	// Find the bot at this position
	for _, b := range gs.Bots {
		if b.Alive && b.Position == pos {
			gs.Moves[b.ID] = Move{Position: pos, Direction: dir}
			return
		}
	}
}

// ClearTurnState clears the per-turn state.
func (gs *GameState) ClearTurnState() {
	gs.Moves = make(map[int]Move)
	gs.DeadBots = make([]*Bot, 0)
	gs.Events = make([]Event, 0)
}

// GetVisibleState returns the game state filtered by fog of war for a player.
func (gs *GameState) GetVisibleState(playerID int) *VisibleState {
	vs := &VisibleState{
		MatchID: gs.MatchID,
		Turn:    gs.Turn,
		Config:  gs.Config,
	}
	vs.You.ID = playerID
	vs.You.Energy = gs.Players[playerID].Energy
	vs.You.Score = gs.Players[playerID].Score

	// Get positions of player's bots
	playerPositions := make([]Position, 0)
	for _, b := range gs.Bots {
		if b.Alive && b.Owner == playerID {
			playerPositions = append(playerPositions, b.Position)
		}
	}

	// Calculate visible tiles
	visible := gs.Grid.VisibleFrom(playerPositions, gs.Config.VisionRadius2)

	// Filter bots
	vs.Bots = make([]VisibleBot, 0)
	for _, b := range gs.Bots {
		if b.Alive && visible[b.Position] {
			vs.Bots = append(vs.Bots, VisibleBot{
				Position: b.Position,
				Owner:    b.Owner,
			})
		}
	}

	// Filter dead bots (visible for one turn)
	for _, b := range gs.DeadBots {
		if visible[b.Position] {
			vs.Dead = append(vs.Dead, VisibleBot{
				Position: b.Position,
				Owner:    b.Owner,
			})
		}
	}

	// Filter energy nodes with energy
	vs.Energy = make([]Position, 0)
	for _, en := range gs.Energy {
		if en.HasEnergy && visible[en.Position] {
			vs.Energy = append(vs.Energy, en.Position)
		}
	}

	// Filter cores
	vs.Cores = make([]VisibleCore, 0)
	for _, c := range gs.Cores {
		if visible[c.Position] {
			vs.Cores = append(vs.Cores, VisibleCore{
				Position: c.Position,
				Owner:    c.Owner,
				Active:   c.Active,
			})
		}
	}

	// Filter walls
	vs.Walls = make([]Position, 0)
	for p := range gs.Grid.Walls {
		if visible[p] {
			vs.Walls = append(vs.Walls, p)
		}
	}

	return vs
}

// ToJSON returns a JSON representation of the game state.
func (gs *GameState) ToJSON() ([]byte, error) {
	return json.MarshalIndent(gs, "", "  ")
}

// Clone creates a deep copy of the game state.
func (gs *GameState) Clone() *GameState {
	newGS := &GameState{
		Config:    gs.Config,
		Grid:      NewGrid(gs.Config.Rows, gs.Config.Cols),
		Bots:      make([]*Bot, len(gs.Bots)),
		Cores:     make([]*Core, len(gs.Cores)),
		Energy:    make([]*EnergyNode, len(gs.Energy)),
		Players:   make([]*Player, len(gs.Players)),
		Turn:      gs.Turn,
		MatchID:   gs.MatchID,
		NextBotID: gs.NextBotID,
		rng:       gs.rng,
		Moves:     make(map[int]Move),
		DeadBots:  make([]*Bot, 0),
		Events:    make([]Event, 0),
		Dominance: make(map[int]int),
	}

	// Copy grid
	for p := range gs.Grid.Walls {
		newGS.Grid.Walls[p] = true
	}
	for row := 0; row < gs.Config.Rows; row++ {
		for col := 0; col < gs.Config.Cols; col++ {
			newGS.Grid.Tiles[row][col] = gs.Grid.Tiles[row][col]
		}
	}

	// Copy bots
	for i, b := range gs.Bots {
		newGS.Bots[i] = &Bot{
			ID:       b.ID,
			Owner:    b.Owner,
			Position: b.Position,
			Alive:    b.Alive,
		}
	}

	// Copy cores
	for i, c := range gs.Cores {
		newGS.Cores[i] = &Core{
			Position: c.Position,
			Owner:    c.Owner,
			Active:   c.Active,
		}
	}

	// Copy energy nodes
	for i, en := range gs.Energy {
		newGS.Energy[i] = &EnergyNode{
			Position:  en.Position,
			HasEnergy: en.HasEnergy,
			Tick:      en.Tick,
		}
	}

	// Copy players
	for i, p := range gs.Players {
		newGS.Players[i] = &Player{
			ID:       p.ID,
			Energy:   p.Energy,
			Score:    p.Score,
			BotCount: p.BotCount,
		}
	}

	// Copy dominance
	for k, v := range gs.Dominance {
		newGS.Dominance[k] = v
	}

	return newGS
}

// String returns a string representation of the game state.
func (gs *GameState) String() string {
	return fmt.Sprintf("GameState{Turn: %d, Players: %d, Bots: %d, Living: %d}",
		gs.Turn, len(gs.Players), len(gs.Bots), gs.GetLivingBotCount())
}
