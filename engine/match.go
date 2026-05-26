package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"
)

// MatchRunner orchestrates a match between multiple bots.
type MatchRunner struct {
	config          Config
	bots            []BotInterface
	names           []string
	rng             *rand.Rand
	verbose         bool
	logger          *log.Logger
	timeout         time.Duration    // per-turn timeout
	preGeneratedMap *PreGeneratedMap // pre-generated map from map library (optional)
}

// PreGeneratedMap contains map data loaded from the map library.
type PreGeneratedMap struct {
	WallsJSON string // JSON array of {row, col} positions
	CoresJSON string // JSON array of {position: {row, col}, owner: int}
}

// MatchOption is a functional option for MatchRunner.
type MatchOption func(*MatchRunner)

// WithVerbose enables verbose logging.
func WithVerbose(v bool) MatchOption {
	return func(mr *MatchRunner) {
		mr.verbose = v
	}
}

// WithLogger sets a custom logger.
func WithLogger(l *log.Logger) MatchOption {
	return func(mr *MatchRunner) {
		mr.logger = l
	}
}

// WithTimeout sets the per-turn timeout.
func WithTimeout(d time.Duration) MatchOption {
	return func(mr *MatchRunner) {
		mr.timeout = d
	}
}

// WithRNG sets the random number generator.
func WithRNG(rng *rand.Rand) MatchOption {
	return func(mr *MatchRunner) {
		mr.rng = rng
	}
}

// WithMap sets a pre-generated map from the map library.
// When provided, the match runner uses this map instead of generating one on-the-fly.
func WithMap(preGen PreGeneratedMap) MatchOption {
	return func(mr *MatchRunner) {
		mr.preGeneratedMap = &preGen
	}
}

// NewMatchRunner creates a new match runner.
func NewMatchRunner(config Config, options ...MatchOption) *MatchRunner {
	mr := &MatchRunner{
		config:  config,
		bots:    make([]BotInterface, 0),
		names:   make([]string, 0),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		verbose: false,
		logger:  log.Default(),
		timeout: 3 * time.Second,
	}

	for _, opt := range options {
		opt(mr)
	}

	return mr
}

// AddBot adds a bot to the match.
func (mr *MatchRunner) AddBot(bot BotInterface, name string) {
	mr.bots = append(mr.bots, bot)
	mr.names = append(mr.names, name)
}

// DebugProvider is an optional interface bots may implement to expose debug telemetry.
type DebugProvider interface {
	LastDebug() *DebugInfo
}

// Run executes the match and returns the result and replay.
func (mr *MatchRunner) Run() (*MatchResult, *Replay, error) {
	if len(mr.bots) < 2 {
		return nil, nil, fmt.Errorf("need at least 2 bots, got %d", len(mr.bots))
	}

	// Initialize game state
	gs := NewGameState(mr.config, mr.rng)

	// Add players
	for range mr.bots {
		gs.AddPlayer()
	}

	// Set up replay writer
	replayWriter := NewReplayWriter(gs.MatchID, mr.config)

	// Record players
	replayPlayers := make([]ReplayPlayer, len(mr.bots))
	for i, name := range mr.names {
		replayPlayers[i] = ReplayPlayer{ID: i, Name: name}
	}
	replayWriter.SetPlayers(replayPlayers)

	// Generate a simple symmetric map for 2 players
	mr.generateMap(gs, len(mr.bots))

	// Record initial map state
	replayWriter.SetMap(gs)

	// Collect state snapshots for win probability computation
	snapshots := make([]*GameState, 0, mr.config.MaxTurns+1)
	snapshots = append(snapshots, gs.Clone())

	// Record turn 0 (initial state, no debug yet)
	replayWriter.RecordTurn(gs, nil)

	// Run the match
	var result *MatchResult
	for gs.Turn < mr.config.MaxTurns {
		// Get moves from all bots concurrently
		moves := mr.getMovesFromBots(gs)

		// Submit moves to game state
		gs.ClearTurnState()
		for playerID, playerMoves := range moves {
			for _, move := range playerMoves {
				// Validate bot ownership
				bot := mr.findBotAtPosition(gs, move.Position, playerID)
				if bot != nil && bot.Alive {
					gs.SubmitMove(move.Position, move.Direction)
				}
			}
		}

		// Execute the turn
		result = gs.ExecuteTurn()

		// Collect debug telemetry from bots that support it
		var debug map[int]*DebugInfo
		for i, bot := range mr.bots {
			if dp, ok := bot.(DebugProvider); ok {
				if d := dp.LastDebug(); d != nil {
					if debug == nil {
						debug = make(map[int]*DebugInfo)
					}
					debug[i] = d
				}
			}
		}

		// Record turn state with debug
		replayWriter.RecordTurn(gs, debug)

		// Collect state snapshot for win probability
		snapshots = append(snapshots, gs.Clone())

		if mr.verbose {
			mr.logger.Printf("Turn %d: %d living bots", gs.Turn, gs.GetLivingBotCount())
		}

		if result != nil {
			break
		}
	}

	// Compute win probability via Monte Carlo rollout
	winProbs, criticalMoments := ComputeWinProbability(snapshots, 100, mr.rng)
	replayWriter.SetWinProbability(winProbs, criticalMoments)

	// Populate crash status per player
	result.Crashed = make([]bool, len(mr.bots))
	for i, bot := range mr.bots {
		if hb, ok := bot.(*HTTPBot); ok {
			result.Crashed[i] = hb.IsCrashed()
		}
	}

	// Finalize replay
	replayWriter.Finalize(result)

	return result, replayWriter.GetReplay(), nil
}

// getMovesFromBots gets moves from all bots concurrently.
func (mr *MatchRunner) getMovesFromBots(gs *GameState) map[int][]Move {
	moves := make(map[int][]Move)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for playerID, bot := range mr.bots {
		wg.Add(1)
		go func(pid int, b BotInterface) {
			defer wg.Done()

			// Get visible state for this player
			visibleState := gs.GetVisibleState(pid)

			// Get moves with timeout
			moveChan := make(chan []Move, 1)
			errChan := make(chan error, 1)

			go func() {
				m, err := b.GetMoves(visibleState)
				if err != nil {
					errChan <- err
					return
				}
				moveChan <- m
			}()

			select {
			case m := <-moveChan:
				mu.Lock()
				moves[pid] = m
				mu.Unlock()
			case <-errChan:
				// Bot returned error, no moves
				if mr.verbose {
					mr.logger.Printf("Bot %d returned error", pid)
				}
			case <-time.After(mr.timeout):
				// Timeout, no moves
				if mr.verbose {
					mr.logger.Printf("Bot %d timed out", pid)
				}
			}
		}(playerID, bot)
	}

	wg.Wait()
	return moves
}

// findBotAtPosition finds a bot at a position owned by a player.
func (mr *MatchRunner) findBotAtPosition(gs *GameState, pos Position, playerID int) *Bot {
	for _, b := range gs.Bots {
		if b.Alive && b.Position == pos && b.Owner == playerID {
			return b
		}
	}
	return nil
}

// loadPreGeneratedMap loads a pre-generated map from the map library.
// Returns true if successful, false if the map data is invalid.
func (mr *MatchRunner) loadPreGeneratedMap(gs *GameState) bool {
	if mr.preGeneratedMap == nil {
		return false
	}

	// Parse walls JSON
	type wallPos struct {
		Row int `json:"row"`
		Col int `json:"col"`
	}
	var walls []wallPos
	if err := json.Unmarshal([]byte(mr.preGeneratedMap.WallsJSON), &walls); err != nil {
		mr.logger.Printf("Warning: failed to parse walls JSON: %v — falling back to generated map", err)
		return false
	}

	// Parse cores JSON
	type coreData struct {
		Position Position `json:"position"`
		Owner    int      `json:"owner"`
	}
	var cores []coreData
	if err := json.Unmarshal([]byte(mr.preGeneratedMap.CoresJSON), &cores); err != nil {
		mr.logger.Printf("Warning: failed to parse cores JSON: %v — falling back to generated map", err)
		return false
	}

	// Place walls
	for _, w := range walls {
		if w.Row >= 0 && w.Row < gs.Config.Rows && w.Col >= 0 && w.Col < gs.Config.Cols {
			gs.Grid.SetPos(Position{Row: w.Row, Col: w.Col}, TileWall)
		}
	}

	// Place cores and spawn initial bots
	coresPerPlayer := make(map[int]int)
	for _, c := range cores {
		if c.Owner < 0 || c.Owner >= len(gs.Players) {
			mr.logger.Printf("Warning: core owner %d out of range [0, %d) — skipping", c.Owner, len(gs.Players))
			continue
		}
		if c.Position.Row < 0 || c.Position.Row >= gs.Config.Rows || c.Position.Col < 0 || c.Position.Col >= gs.Config.Cols {
			mr.logger.Printf("Warning: core at (%d, %d) out of grid bounds — skipping", c.Position.Row, c.Position.Col)
			continue
		}
		gs.AddCore(c.Owner, c.Position)
		gs.SpawnBot(c.Owner, c.Position)
		coresPerPlayer[c.Owner]++
	}

	// Verify each player has at least one core
	for p := range gs.Players {
		if coresPerPlayer[p] == 0 {
			mr.logger.Printf("Warning: player %d has no cores in pre-generated map — falling back to generated map", p)
			return false
		}
	}

	// Place energy nodes symmetrically (even with pre-generated walls/cores)
	mr.placeEnergyNodes(gs, len(gs.Players))

	return true
}

// generateMap generates a symmetric map for the given number of players.
// If a pre-generated map is provided via WithMap, it loads that instead.
func (mr *MatchRunner) generateMap(gs *GameState, numPlayers int) {
	// Try to load pre-generated map first
	if mr.loadPreGeneratedMap(gs) {
		return
	}

	// Fall back to generating map on-the-fly
	centerRow := gs.Config.Rows / 2
	centerCol := gs.Config.Cols / 2
	coresPerPlayer := gs.Config.CoresPerPlayer
	if coresPerPlayer < 1 {
		coresPerPlayer = 1
	}

	// Place cores for each player using rotational symmetry.
	// Per plan §3.7.1: zone forces combat by shrinking. Bots must be able to reach
	// the safe zone before it kills them, while also being forced into contact range.
	//
	// Zone parameters: starts at turn 10, shrinks 2 tiles/turn, min radius 2 (2-player)
	// By turn 19, zone reaches min radius of 2 (6-tile diameter, ≤2×attack radius).
	//
	// Spawn radius as percentage of grid half-size:
	// - 2-player: 20% (~4 tiles from center on 40x40 grid, ~8 tiles apart)
	//   Zone starts at turn 10 with radius = maxDist + 5, then shrinks 2 tiles/turn.
	//   At 20% spawn radius (dist 4), zone starts at radius 9, shrinks to 5 by turn 12.
	//   Strategy bots (gatherer, rusher) move toward center, reaching attack range (5 tiles)
	//   within 2-5 turns, ensuring combat before zone kills them. Random bots may die
	//   from zone if they don't engage. This achieves >65% combat density target.
	// - 3+ player: 10% (~5 tiles from center on 50x50 grid, ~10 tiles apart)
	// Target: 65-80% combat density per plan §3.7.1.
	halfRows := float64(centerRow)
	halfCols := float64(centerCol)

	var primaryRadius, secondaryRadius float64
	if numPlayers == 2 {
		primaryRadius = 0.15   // ~6.8 tiles from center on 40x40 grid (~13.6 tiles apart)
		secondaryRadius = 0.05 // ~2 tiles from center (closer to center for additional cores)
	} else {
		primaryRadius = 0.10 // ~5 tiles from center on 50x50 grid
		secondaryRadius = 0.08
	}

	for i := 0; i < numPlayers; i++ {
		baseAngle := float64(i) * 2.0 * math.Pi / float64(numPlayers)

		for c := 0; c < coresPerPlayer; c++ {
			var row, col int
			if c == 0 {
				// Primary core: far from center
				row = centerRow + int(halfRows*primaryRadius*math.Cos(baseAngle))
				col = centerCol + int(halfCols*primaryRadius*math.Sin(baseAngle))
			} else {
				// Additional cores: closer to center, offset angularly
				angleOffset := (float64(c) * 0.3) / float64(numPlayers)
				angle := baseAngle + angleOffset
				row = centerRow + int(halfRows*secondaryRadius*math.Cos(angle))
				col = centerCol + int(halfCols*secondaryRadius*math.Sin(angle))
			}

			// Wrap to grid bounds
			row = ((row % gs.Config.Rows) + gs.Config.Rows) % gs.Config.Rows
			col = ((col % gs.Config.Cols) + gs.Config.Cols) % gs.Config.Cols

			pos := Position{Row: row, Col: col}
			gs.AddCore(i, pos)

			gs.SpawnBot(i, pos)
		}
	}

	// Place energy nodes symmetrically
	mr.placeEnergyNodes(gs, numPlayers)

	// Place walls symmetrically
	mr.placeWalls(gs, numPlayers)
}

// placeEnergyNodes places energy nodes symmetrically.
func (mr *MatchRunner) placeEnergyNodes(gs *GameState, numPlayers int) {
	centerRow := gs.Config.Rows / 2
	centerCol := gs.Config.Cols / 2

	// Scale energy nodes with map area: ~1 node per 150 tiles, minimum 4 per player
	totalArea := gs.Config.Rows * gs.Config.Cols
	numNodes := totalArea / 150
	minNodes := numPlayers * 4
	if numNodes < minNodes {
		numNodes = minNodes
	}
	nodesPerSector := numNodes / numPlayers

	// Tiered radius distribution biases toward center to force contested energy:
	// - 30% central (0.05-0.20): contested central zone
	// - 40% mid (0.20-0.40): mid-zone
	// - 30% outer (0.40-0.60): outer zone
	for i := 0; i < nodesPerSector; i++ {
		// Generate one position in the first sector
		angle := mr.rng.Float64() * 2.0 * math.Pi / float64(numPlayers)
		// Tiered radius: bias toward center to force contested energy collection.
		// 30% central (forces both players to midfield), 40% mid, 30% outer.
		var radius float64
		switch {
		case i < nodesPerSector*3/10:
			radius = 0.05 + mr.rng.Float64()*0.15 // 0.05–0.20: contested central zone
		case i < nodesPerSector*7/10:
			radius = 0.20 + mr.rng.Float64()*0.20 // 0.20–0.40: mid-zone
		default:
			radius = 0.40 + mr.rng.Float64()*0.20 // 0.40–0.60: outer zone
		}

		// Mirror for all players
		for p := 0; p < numPlayers; p++ {
			rotAngle := angle + float64(p)*2.0*math.Pi/float64(numPlayers)
			r := centerRow + int(float64(centerRow)*radius*math.Cos(rotAngle))
			c := centerCol + int(float64(centerCol)*radius*math.Sin(rotAngle))
			gs.AddEnergyNode(Position{Row: r, Col: c})
		}
	}

	// Initialize ~40% of energy nodes with energy already spawned.
	// This prevents immediate mutual destruction scenarios by giving bots
	// energy collection opportunities from turn 0, enabling respawns.
	initialEnergyRatio := 0.4
	for _, en := range gs.Energy {
		if mr.rng.Float64() < initialEnergyRatio {
			en.HasEnergy = true
			en.Tick = 0
		}
	}
}

// placeWalls places walls symmetrically.
func (mr *MatchRunner) placeWalls(gs *GameState, numPlayers int) {
	centerRow := gs.Config.Rows / 2
	centerCol := gs.Config.Cols / 2

	// Calculate target number of walls: 5% density (20 passable : 1 wall)
	// NOTE: Plan §3.1 specifies 15% default, but higher density in match.go
	// without connectivity validation can isolate bots. Maps generated via
	// acb-mapgen use 15% with connectivity validation.
	totalTiles := gs.Config.Rows * gs.Config.Cols
	targetWalls := totalTiles / 20
	wallsPerSector := targetWalls / numPlayers

	for i := 0; i < wallsPerSector; i++ {
		// Generate one position in the first sector
		angle := mr.rng.Float64() * 2.0 * math.Pi / float64(numPlayers)
		radius := 0.1 + mr.rng.Float64()*0.8 // 10-90% of half-size
		row := centerRow + int(float64(centerRow)*radius*math.Cos(angle))
		col := centerCol + int(float64(centerCol)*radius*math.Sin(angle))

		// Check it's not on a core or energy node
		pos := Position{Row: row, Col: col}
		if mr.isValidWallPosition(gs, pos) {
			// Mirror for all players
			for p := 0; p < numPlayers; p++ {
				rotAngle := angle + float64(p)*2.0*math.Pi/float64(numPlayers)
				r := centerRow + int(float64(centerRow)*radius*math.Cos(rotAngle))
				c := centerCol + int(float64(centerCol)*radius*math.Sin(rotAngle))
				mirrorPos := Position{Row: r, Col: c}
				if mr.isValidWallPosition(gs, mirrorPos) {
					gs.Grid.SetPos(mirrorPos, TileWall)
				}
			}
		}
	}
}

// isValidWallPosition checks if a position can have a wall.
func (mr *MatchRunner) isValidWallPosition(gs *GameState, pos Position) bool {
	// Check for core
	for _, c := range gs.Cores {
		if c.Position == pos {
			return false
		}
	}
	// Check for energy node
	for _, en := range gs.Energy {
		if en.Position == pos {
			return false
		}
	}
	return true
}
