package engine

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Replay records the complete history of a match for playback.
type Replay struct {
	FormatVersion   string            `json:"format_version"` // semver, e.g. "1.0"
	MatchID         string            `json:"match_id"`
	Config          Config            `json:"config"`
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time"`
	Result          *MatchResult      `json:"result"`
	Players         []ReplayPlayer    `json:"players"`
	Map             ReplayMap         `json:"map"`
	Turns           []ReplayTurn      `json:"turns"`
	WinProb         []WinProbEntry    `json:"win_prob,omitempty"`
	CriticalMoments []CriticalMoment  `json:"critical_moments,omitempty"`
}

// ReplayPlayer represents player info in a replay.
type ReplayPlayer struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
}

// ReplayMap represents the static map data.
type ReplayMap struct {
	Rows     int        `json:"rows"`
	Cols     int        `json:"cols"`
	Walls    []Position `json:"walls"`
	Cores    []ReplayCore `json:"cores"`
	EnergyNodes []Position `json:"energy_nodes"`
}

// ReplayCore represents a core in the replay.
type ReplayCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// ReplayTurn represents the state at a single turn.
type ReplayTurn struct {
	Turn       int                 `json:"turn"`
	Bots       []ReplayBot         `json:"bots"`
	Cores      []ReplayCoreState   `json:"cores"`
	Energy     []Position          `json:"energy"`
	Scores     []int               `json:"scores"`
	EnergyHeld []int               `json:"energy_held"`
	Events     []Event             `json:"events,omitempty"`
	Debug      map[int]*DebugInfo  `json:"debug,omitempty"` // optional bot debug telemetry
}

// ReplayBot represents a bot in a replay turn.
type ReplayBot struct {
	ID       int      `json:"id"`
	Owner    int      `json:"owner"`
	Position Position `json:"position"`
	Alive    bool     `json:"alive"`
}

// ReplayCoreState represents a core's state at a turn.
type ReplayCoreState struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}

// ReplayWriter records a match as it progresses.
type ReplayWriter struct {
	replay    *Replay
	turns     []ReplayTurn
	startTime time.Time
}

// NewReplayWriter creates a new replay writer.
func NewReplayWriter(matchID string, config Config) *ReplayWriter {
	return &ReplayWriter{
		replay: &Replay{
			FormatVersion: "1.0",
			MatchID:       matchID,
			Config:        config,
			StartTime:     time.Now().UTC(),
		},
		turns:     make([]ReplayTurn, 0),
		startTime: time.Now(),
	}
}

// SetPlayers records the players in the match.
func (rw *ReplayWriter) SetPlayers(players []ReplayPlayer) {
	rw.replay.Players = players
}

// SetMap records the static map data.
func (rw *ReplayWriter) SetMap(gs *GameState) {
	rmap := ReplayMap{
		Rows:        gs.Config.Rows,
		Cols:        gs.Config.Cols,
		Walls:       make([]Position, 0),
		Cores:       make([]ReplayCore, 0),
		EnergyNodes: make([]Position, 0),
	}

	// Record walls
	for p := range gs.Grid.Walls {
		rmap.Walls = append(rmap.Walls, p)
	}

	// Record cores
	for _, c := range gs.Cores {
		rmap.Cores = append(rmap.Cores, ReplayCore{
			Position: c.Position,
			Owner:    c.Owner,
		})
	}

	// Record energy node positions
	for _, en := range gs.Energy {
		rmap.EnergyNodes = append(rmap.EnergyNodes, en.Position)
	}

	rw.replay.Map = rmap
}

// RecordTurn records the state at the end of a turn.
// debug is an optional map of player ID -> DebugInfo collected from bot responses.
func (rw *ReplayWriter) RecordTurn(gs *GameState, debug map[int]*DebugInfo) {
	turn := ReplayTurn{
		Turn:       gs.Turn,
		Bots:       make([]ReplayBot, 0),
		Cores:      make([]ReplayCoreState, 0),
		Energy:     make([]Position, 0),
		Scores:     make([]int, len(gs.Players)),
		EnergyHeld: make([]int, len(gs.Players)),
		Events:     gs.Events,
		Debug:      debug,
	}

	// Record all bots (including dead ones for death animation)
	for _, b := range gs.Bots {
		turn.Bots = append(turn.Bots, ReplayBot{
			ID:       b.ID,
			Owner:    b.Owner,
			Position: b.Position,
			Alive:    b.Alive,
		})
	}

	// Record core states
	for _, c := range gs.Cores {
		turn.Cores = append(turn.Cores, ReplayCoreState{
			Position: c.Position,
			Owner:    c.Owner,
			Active:   c.Active,
		})
	}

	// Record energy positions
	for _, en := range gs.Energy {
		if en.HasEnergy {
			turn.Energy = append(turn.Energy, en.Position)
		}
	}

	// Record scores and energy
	for i, p := range gs.Players {
		turn.Scores[i] = p.Score
		turn.EnergyHeld[i] = p.Energy
	}

	rw.turns = append(rw.turns, turn)
}

// SetWinProbability sets the win probability data and critical moments on the replay.
func (rw *ReplayWriter) SetWinProbability(winProb []WinProbEntry, moments []CriticalMoment) {
	rw.replay.WinProb = winProb
	rw.replay.CriticalMoments = moments
}

// Finalize completes the replay with the match result.
func (rw *ReplayWriter) Finalize(result *MatchResult) {
	rw.replay.EndTime = time.Now().UTC()
	rw.replay.Result = result
	rw.replay.Turns = rw.turns
}

// GetReplay returns the completed replay.
func (rw *ReplayWriter) GetReplay() *Replay {
	return rw.replay
}

// WriteJSON writes the replay as JSON to the writer.
func (rw *ReplayWriter) WriteJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rw.replay)
}

// WriteFile writes the replay as JSON to a file.
func (rw *ReplayWriter) WriteFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return rw.WriteJSON(f)
}

// ReplayToJSON converts a replay to JSON bytes.
func ReplayToJSON(replay *Replay) ([]byte, error) {
	return json.MarshalIndent(replay, "", "  ")
}

// LoadReplay loads a replay from JSON bytes.
func LoadReplay(data []byte) (*Replay, error) {
	var replay Replay
	err := json.Unmarshal(data, &replay)
	if err != nil {
		return nil, err
	}
	return &replay, nil
}

// LoadReplayFile loads a replay from a file.
func LoadReplayFile(path string) (*Replay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadReplay(data)
}
