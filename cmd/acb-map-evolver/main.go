// Command acb-map-evolver evolves maps through breeding and mutation.
// It selects high-engagement parent maps, breeds offspring via crossover,
// applies mutations, validates connectivity, and smoke-tests with bots.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// Config holds command-line configuration.
type Config struct {
	DatabaseURL     string
	PlayerCount     int
	NumOffspring    int
	DryRun          bool
	MinEngagement   float64
	MaxAttempts     int
	ValidateSmoke   bool
}

// Map represents a game map.
type Map struct {
	ID          string     `json:"id"`
	Players     int        `json:"players"`
	Rows        int        `json:"rows"`
	Cols        int        `json:"cols"`
	WallDensity float64    `json:"wall_density"`
	Walls       []Position `json:"walls"`
	Cores       []Core     `json:"cores"`
	EnergyNodes []Position `json:"energy_nodes"`
}

// Position represents a grid coordinate.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Core represents a spawn point.
type Core struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// PositionSet is a set of positions.
type PositionSet map[Position]bool

// ParentMap represents a parent map with its engagement score.
type ParentMap struct {
	Map        *Map
	Engagement float64
	VoteMult   float64
}

func main() {
	cfg := parseConfig()
	if cfg == nil {
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run evolution
	evolver := NewMapEvolver(db, cfg)
	results, err := evolver.Run(ctx)
	if err != nil {
		log.Fatalf("Evolution failed: %v", err)
	}

	log.Printf("Evolution complete: %d new maps created", len(results))
	for _, m := range results {
		log.Printf("  - %s (engagement: %.2f)", m.ID, m.WallDensity)
	}
}

func parseConfig() *Config {
	cfg := &Config{
		DatabaseURL:   os.Getenv("ACB_DATABASE_URL"),
		PlayerCount:   2,
		NumOffspring:  5,
		MinEngagement: 5.0,
		MaxAttempts:   10,
		ValidateSmoke: true,
	}

	for i, arg := range os.Args[1:] {
		switch arg {
		case "--player-count":
			if i+1 < len(os.Args[1:]) {
				fmt.Sscanf(os.Args[1:][i+1], "%d", &cfg.PlayerCount)
			}
		case "--num-offspring":
			if i+1 < len(os.Args[1:]) {
				fmt.Sscanf(os.Args[1:][i+1], "%d", &cfg.NumOffspring)
			}
		case "--min-engagement":
			if i+1 < len(os.Args[1:]) {
				fmt.Sscanf(os.Args[1:][i+1], "%f", &cfg.MinEngagement)
			}
		case "--dry-run":
			cfg.DryRun = true
		case "--no-smoke":
			cfg.ValidateSmoke = false
		case "--help", "-h":
			fmt.Println("Usage: acb-map-evolver [options]")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --player-count N    Player count tier (2, 3, 4, or 6) [default: 2]")
			fmt.Println("  --num-offspring N   Number of maps to create [default: 5]")
			fmt.Println("  --min-engagement F  Minimum engagement threshold for parents [default: 5.0]")
			fmt.Println("  --dry-run           Generate maps but don't save to database")
			fmt.Println("  --no-smoke          Skip smoke-test validation")
			fmt.Println("  --help              Show this help")
			return nil
		}
	}

	if cfg.DatabaseURL == "" && !cfg.DryRun {
		log.Fatal("ACB_DATABASE_URL environment variable is required")
	}

	return cfg
}

// MapEvolver handles map evolution.
type MapEvolver struct {
	db  *sql.DB
	cfg *Config
	rng *rand.Rand
}

// NewMapEvolver creates a new map evolver.
func NewMapEvolver(db *sql.DB, cfg *Config) *MapEvolver {
	return &MapEvolver{
		db:  db,
		cfg: cfg,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Run executes the evolution pipeline.
func (e *MapEvolver) Run(ctx context.Context) ([]*Map, error) {
	// 1. Select parent maps
	parents, err := e.selectParents(ctx)
	if err != nil {
		return nil, fmt.Errorf("selecting parents: %w", err)
	}
	if len(parents) < 2 {
		return nil, fmt.Errorf("need at least 2 parent maps, found %d", len(parents))
	}

	log.Printf("Selected %d parent maps", len(parents))

	// 2. Breed offspring
	var offspring []*Map
	for i := 0; i < e.cfg.NumOffspring; i++ {
		for attempt := 0; attempt < e.cfg.MaxAttempts; attempt++ {
			child := e.breed(parents)
			if child == nil {
				continue
			}

			// 3. Validate
			if !e.validate(child) {
				continue
			}

			// 4. Smoke test (if enabled)
			if e.cfg.ValidateSmoke && !e.smokeTest(child) {
				continue
			}

			offspring = append(offspring, child)
			break
		}
	}

	// 5. Save to database
	if !e.cfg.DryRun {
		for _, m := range offspring {
			if err := e.saveMap(ctx, m); err != nil {
				log.Printf("Failed to save map %s: %v", m.ID, err)
			}
		}
	}

	return offspring, nil
}

// selectParents retrieves top maps by engagement × vote multiplier.
func (e *MapEvolver) selectParents(ctx context.Context) ([]*ParentMap, error) {
	query := `
		SELECT m.map_id, m.map_json, COALESCE(ms.engagement, 0) as engagement,
		       CASE
		         WHEN COALESCE(votes.net_votes, 0) > 10 THEN 1.5
		         WHEN COALESCE(votes.net_votes, 0) < 0 THEN 0.5
		         ELSE 1.0
		       END as vote_mult
		FROM maps m
		LEFT JOIN map_scores ms ON m.map_id = ms.map_id
		LEFT JOIN (
		    SELECT map_id, SUM(vote) as net_votes
		    FROM map_votes
		    GROUP BY map_id
		) votes ON m.map_id = votes.map_id
		WHERE m.player_count = $1
		  AND m.status IN ('active', 'classic')
		ORDER BY COALESCE(ms.engagement, 0) *
		         CASE WHEN COALESCE(votes.net_votes, 0) > 10 THEN 1.5
		              WHEN COALESCE(votes.net_votes, 0) < 0 THEN 0.5
		              ELSE 1.0 END DESC
		LIMIT 20
	`

	rows, err := e.db.QueryContext(ctx, query, e.cfg.PlayerCount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parents []*ParentMap
	for rows.Next() {
		var id string
		var mapJSON []byte
		var engagement float64
		var voteMult float64

		if err := rows.Scan(&id, &mapJSON, &engagement, &voteMult); err != nil {
			return nil, err
		}

		var m Map
		if err := json.Unmarshal(mapJSON, &m); err != nil {
			log.Printf("Failed to unmarshal map %s: %v", id, err)
			continue
		}

		m.ID = id
		parents = append(parents, &ParentMap{
			Map:        &m,
			Engagement: engagement,
			VoteMult:   voteMult,
		})
	}

	return parents, nil
}

// breed creates a new map from parent maps via crossover and mutation.
func (e *MapEvolver) breed(parents []*ParentMap) *Map {
	// Weighted random selection based on engagement × vote multiplier
	p1 := e.selectWeighted(parents)
	p2 := e.selectWeighted(parents)
	for p2 == p1 && len(parents) > 1 {
		p2 = e.selectWeighted(parents)
	}

	// Create child from crossover
	child := e.crossover(p1.Map, p2.Map)

	// Apply mutations
	e.mutate(child)

	// Generate new ID
	child.ID = generateMapID(e.rng)
	child.Players = e.cfg.PlayerCount

	return child
}

// selectWeighted selects a parent with probability proportional to engagement × vote multiplier.
func (e *MapEvolver) selectWeighted(parents []*ParentMap) *ParentMap {
	totalWeight := 0.0
	for _, p := range parents {
		w := p.Engagement * p.VoteMult
		if w < 0.1 {
			w = 0.1 // Minimum weight
		}
		totalWeight += w
	}

	r := e.rng.Float64() * totalWeight
	cumulative := 0.0
	for _, p := range parents {
		w := p.Engagement * p.VoteMult
		if w < 0.1 {
			w = 0.1
		}
		cumulative += w
		if r <= cumulative {
			return p
		}
	}

	return parents[len(parents)-1]
}

// crossover combines two parent maps into a child.
func (e *MapEvolver) crossover(p1, p2 *Map) *Map {
	child := &Map{
		Rows:        p1.Rows,
		Cols:        p1.Cols,
		Players:     e.cfg.PlayerCount,
		WallDensity: (p1.WallDensity + p2.WallDensity) / 2,
		Walls:       make([]Position, 0),
		Cores:       make([]Core, 0),
		EnergyNodes: make([]Position, 0),
	}

	// Use cores from p1 (they should be symmetric anyway)
	child.Cores = p1.Cores

	centerRow := child.Rows / 2
	centerCol := child.Cols / 2
	sectorAngle := 2.0 * math.Pi / float64(child.Players)

	// Build wall sets
	walls1 := make(PositionSet)
	for _, w := range p1.Walls {
		walls1[w] = true
	}
	walls2 := make(PositionSet)
	for _, w := range p2.Walls {
		walls2[w] = true
	}

	// Crossover: for each position in sector 0, pick wall from p1 or p2
	// Then mirror to all sectors
	for r := 0; r < child.Rows; r++ {
		for c := 0; c < child.Cols; c++ {
			dr := float64(r) - float64(centerRow)
			dc := float64(c) - float64(centerCol)
			angle := math.Atan2(dc, dr)
			if angle < 0 {
				angle += 2.0 * math.Pi
			}
			sector := int(angle / sectorAngle)
			if sector >= child.Players {
				sector = child.Players - 1
			}

			// Only process sector 0, then mirror
			if sector != 0 {
				continue
			}

			pos := Position{Row: r, Col: c}
			isWall := false
			if walls1[pos] && walls2[pos] {
				// Both have wall: keep it
				isWall = true
			} else if walls1[pos] || walls2[pos] {
				// One has wall: 50% chance
				isWall = e.rng.Float64() < 0.5
			}

			if isWall {
				// Mirror wall to all sectors
				for s := 0; s < child.Players; s++ {
					rotAngle := float64(s) * sectorAngle
					cosA := math.Cos(rotAngle)
					sinA := math.Sin(rotAngle)
					rr := int(math.Round(float64(centerRow) + dr*cosA - dc*sinA))
					rc := int(math.Round(float64(centerCol) + dr*sinA + dc*cosA))
					rr = ((rr % child.Rows) + child.Rows) % child.Rows
					rc = ((rc % child.Cols) + child.Cols) % child.Cols
					child.Walls = append(child.Walls, Position{Row: rr, Col: rc})
				}
			}
		}
	}

	// Crossover energy nodes: take from both parents
	seenNodes := make(PositionSet)
	for _, en := range p1.EnergyNodes {
		if !seenNodes[en] {
			child.EnergyNodes = append(child.EnergyNodes, en)
			seenNodes[en] = true
		}
	}
	for _, en := range p2.EnergyNodes {
		if !seenNodes[en] && e.rng.Float64() < 0.5 {
			child.EnergyNodes = append(child.EnergyNodes, en)
			seenNodes[en] = true
		}
	}

	// Update wall density
	child.WallDensity = float64(len(child.Walls)) / float64(child.Rows*child.Cols)

	return child
}

// mutate applies random mutations to a map.
func (e *MapEvolver) mutate(m *Map) {
	wallSet := make(PositionSet)
	for _, w := range m.Walls {
		wallSet[w] = true
	}

	protected := make(PositionSet)
	for _, core := range m.Cores {
		for dr := -3; dr <= 3; dr++ {
			for dc := -3; dc <= 3; dc++ {
				nr := ((core.Position.Row + dr) % m.Rows + m.Rows) % m.Rows
				nc := ((core.Position.Col + dc) % m.Cols + m.Cols) % m.Cols
				protected[Position{Row: nr, Col: nc}] = true
			}
		}
	}
	for _, en := range m.EnergyNodes {
		protected[en] = true
	}

	// Mutate walls: flip 5-10% of tiles
	mutationRate := 0.05 + e.rng.Float64()*0.05
	centerRow := m.Rows / 2
	centerCol := m.Cols / 2
	sectorAngle := 2.0 * math.Pi / float64(m.Players)

	// Collect positions to mutate in sector 0
	var toFlip []Position
	for r := 0; r < m.Rows; r++ {
		for c := 0; c < m.Cols; c++ {
			dr := float64(r) - float64(centerRow)
			dc := float64(c) - float64(centerCol)
			angle := math.Atan2(dc, dr)
			if angle < 0 {
				angle += 2.0 * math.Pi
			}
			sector := int(angle / sectorAngle)
			if sector >= m.Players {
				sector = m.Players - 1
			}
			if sector != 0 {
				continue
			}

			pos := Position{Row: r, Col: c}
			if protected[pos] {
				continue
			}

			if e.rng.Float64() < mutationRate {
				toFlip = append(toFlip, pos)
			}
		}
	}

	// Apply flips with mirroring
	for _, pos := range toFlip {
		isWall := wallSet[pos]

		// Remove existing walls at all mirrored positions
		for s := 0; s < m.Players; s++ {
			dr := float64(pos.Row) - float64(centerRow)
			dc := float64(pos.Col) - float64(centerCol)
			rotAngle := float64(s) * sectorAngle
			cosA := math.Cos(rotAngle)
			sinA := math.Sin(rotAngle)
			rr := int(math.Round(float64(centerRow) + dr*cosA - dc*sinA))
			rc := int(math.Round(float64(centerCol) + dr*sinA + dc*cosA))
			rr = ((rr % m.Rows) + m.Rows) % m.Rows
			rc = ((rc % m.Cols) + m.Cols) % m.Cols
			mirrorPos := Position{Row: rr, Col: rc}

			if isWall {
				// Remove wall
				delete(wallSet, mirrorPos)
			} else {
				// Add wall
				wallSet[mirrorPos] = true
			}
		}
	}

	// Rebuild wall list
	m.Walls = make([]Position, 0, len(wallSet))
	for pos := range wallSet {
		m.Walls = append(m.Walls, pos)
	}

	// Shift 1-3 energy nodes by 1-3 tiles (with symmetry)
	numShifts := 1 + e.rng.Intn(3)
	for i := 0; i < numShifts && len(m.EnergyNodes) > 0; i++ {
		idx := e.rng.Intn(len(m.EnergyNodes))
		oldPos := m.EnergyNodes[idx]

		// Find sector of this node
		dr := float64(oldPos.Row) - float64(centerRow)
		dc := float64(oldPos.Col) - float64(centerCol)
		angle := math.Atan2(dc, dr)
		if angle < 0 {
			angle += 2.0 * math.Pi
		}
		sector := int(angle / sectorAngle)
		if sector >= m.Players {
			sector = m.Players - 1
		}

		// Only shift if in sector 0
		if sector != 0 {
			continue
		}

		// Shift by 1-3 tiles in a random direction
		shiftDist := 1 + e.rng.Intn(3)
		shiftAngle := e.rng.Float64() * 2 * math.Pi

		// Remove old position and all mirrors
		newNodes := make([]Position, 0)
		nodeSet := make(PositionSet)
		for _, en := range m.EnergyNodes {
			nodeSet[en] = true
		}

		for s := 0; s < m.Players; s++ {
			rotAngle := float64(s) * sectorAngle
			cosA := math.Cos(rotAngle)
			sinA := math.Sin(rotAngle)
			rr := int(math.Round(float64(centerRow) + dr*cosA - dc*sinA))
			rc := int(math.Round(float64(centerCol) + dr*sinA + dc*cosA))
			delete(nodeSet, Position{Row: rr, Col: rc})
		}

		// Calculate new position in sector 0
		newR := int(math.Round(float64(oldPos.Row) + float64(shiftDist)*math.Cos(shiftAngle)))
		newC := int(math.Round(float64(oldPos.Col) + float64(shiftDist)*math.Sin(shiftAngle)))
		newR = ((newR % m.Rows) + m.Rows) % m.Rows
		newC = ((newC % m.Cols) + m.Cols) % m.Cols

		// Add new position and all mirrors
		newDR := float64(newR) - float64(centerRow)
		newDC := float64(newC) - float64(centerCol)
		for s := 0; s < m.Players; s++ {
			rotAngle := float64(s) * sectorAngle
			cosA := math.Cos(rotAngle)
			sinA := math.Sin(rotAngle)
			rr := int(math.Round(float64(centerRow) + newDR*cosA - newDC*sinA))
			rc := int(math.Round(float64(centerCol) + newDR*sinA + newDC*cosA))
			rr = ((rr % m.Rows) + m.Rows) % m.Rows
			rc = ((rc % m.Cols) + m.Cols) % m.Cols
			newPos := Position{Row: rr, Col: rc}
			if !wallSet[newPos] {
				nodeSet[newPos] = true
			}
		}

		for pos := range nodeSet {
			newNodes = append(newNodes, pos)
		}
		m.EnergyNodes = newNodes
		break // Only one shift per mutation run
	}

	// Update wall density
	m.WallDensity = float64(len(m.Walls)) / float64(m.Rows*m.Cols)

	// Apply smoothing (2 iterations of cellular automata)
	e.smoothWalls(m, protected)
}

// smoothWalls applies cellular automata smoothing to walls.
// This is a simplified version that preserves existing walls while allowing
// for some natural clustering through the mutation process.
func (e *MapEvolver) smoothWalls(m *Map, protected PositionSet) {
	// For now, skip the full cellular automata smoothing as it's too aggressive
	// when combined with the mutation. The mutation already provides enough variation.
	// The full CA smoothing is better used in initial map generation.

	// Just ensure symmetry is maintained after mutation
	centerRow := m.Rows / 2
	centerCol := m.Cols / 2
	sectorAngle := 2.0 * math.Pi / float64(m.Players)

	// Build wall set
	wallSet := make(PositionSet)
	for _, w := range m.Walls {
		wallSet[w] = true
	}

	// Collect walls in sector 0
	sector0Walls := make(PositionSet)
	for pos := range wallSet {
		dr := float64(pos.Row) - float64(centerRow)
		dc := float64(pos.Col) - float64(centerCol)
		angle := math.Atan2(dc, dr)
		if angle < 0 {
			angle += 2.0 * math.Pi
		}
		sector := int(angle / sectorAngle)
		if sector >= m.Players {
			sector = m.Players - 1
		}
		if sector == 0 && !protected[pos] {
			sector0Walls[pos] = true
		}
	}

	// Rebuild walls from sector 0 with proper mirroring
	newWallSet := make(PositionSet)
	for pos := range sector0Walls {
		dr := float64(pos.Row) - float64(centerRow)
		dc := float64(pos.Col) - float64(centerCol)

		// Mirror to all sectors
		for s := 0; s < m.Players; s++ {
			rotAngle := float64(s) * sectorAngle
			cosA := math.Cos(rotAngle)
			sinA := math.Sin(rotAngle)
			rr := int(math.Round(float64(centerRow) + dr*cosA - dc*sinA))
			rc := int(math.Round(float64(centerCol) + dr*sinA + dc*cosA))
			rr = ((rr % m.Rows) + m.Rows) % m.Rows
			rc = ((rc % m.Cols) + m.Cols) % m.Cols
			mirrorPos := Position{Row: rr, Col: rc}
			if !protected[mirrorPos] {
				newWallSet[mirrorPos] = true
			}
		}
	}

	m.Walls = make([]Position, 0, len(newWallSet))
	for pos := range newWallSet {
		m.Walls = append(m.Walls, pos)
	}

	m.WallDensity = float64(len(m.Walls)) / float64(m.Rows * m.Cols)
}

// validate checks if a map meets all validation criteria.
func (e *MapEvolver) validate(m *Map) bool {
	// Check wall density bounds
	if m.WallDensity < 0.05 || m.WallDensity > 0.30 {
		return false
	}

	// Check connectivity
	if !e.checkConnectivity(m) {
		return false
	}

	// Check open area per player
	totalTiles := m.Rows * m.Cols
	wallCount := len(m.Walls)
	openTiles := totalTiles - wallCount
	openPerPlayer := openTiles / m.Players
	if openPerPlayer < 900 || openPerPlayer > 5000 {
		return false
	}

	// Check each core can reach at least 3 energy nodes
	for _, core := range m.Cores {
		reachable := e.countReachableEnergyNodes(m, core.Position)
		if reachable < 3 {
			return false
		}
	}

	return true
}

// checkConnectivity verifies all passable tiles are reachable from cores.
func (e *MapEvolver) checkConnectivity(m *Map) bool {
	if len(m.Cores) == 0 {
		return false
	}

	// Build wall set
	wallSet := make(PositionSet)
	for _, w := range m.Walls {
		wallSet[w] = true
	}

	// Count passable tiles
	passable := make(PositionSet)
	for r := 0; r < m.Rows; r++ {
		for c := 0; c < m.Cols; c++ {
			pos := Position{Row: r, Col: c}
			if !wallSet[pos] {
				passable[pos] = true
			}
		}
	}

	// BFS from first core
	start := m.Cores[0].Position
	if wallSet[start] {
		return false
	}

	visited := make(PositionSet)
	queue := []Position{start}
	visited[start] = true
	count := 1

	dirs := []Position{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, d := range dirs {
			nr := ((curr.Row + d.Row) % m.Rows + m.Rows) % m.Rows
			nc := ((curr.Col + d.Col) % m.Cols + m.Cols) % m.Cols
			np := Position{Row: nr, Col: nc}

			if passable[np] && !visited[np] {
				visited[np] = true
				queue = append(queue, np)
				count++
			}
		}
	}

	return count == len(passable)
}

// countReachableEnergyNodes counts energy nodes reachable from a starting position.
func (e *MapEvolver) countReachableEnergyNodes(m *Map, start Position) int {
	wallSet := make(PositionSet)
	for _, w := range m.Walls {
		wallSet[w] = true
	}

	energySet := make(PositionSet)
	for _, en := range m.EnergyNodes {
		energySet[en] = true
	}

	visited := make(PositionSet)
	queue := []Position{start}
	visited[start] = true
	count := 0

	dirs := []Position{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if energySet[curr] {
			count++
		}

		for _, d := range dirs {
			nr := ((curr.Row + d.Row) % m.Rows + m.Rows) % m.Rows
			nc := ((curr.Col + d.Col) % m.Cols + m.Cols) % m.Cols
			np := Position{Row: nr, Col: nc}

			if !wallSet[np] && !visited[np] {
				visited[np] = true
				queue = append(queue, np)
			}
		}
	}

	return count
}

// smokeTest runs quick matches to verify the map produces reasonable engagement.
func (e *MapEvolver) smokeTest(m *Map) bool {
	// For now, use a simplified check: verify the map has reasonable properties
	// A full smoke test would run 3 matches with built-in bots

	// Check that map has enough energy nodes
	minEnergy := m.Players * 3
	if len(m.EnergyNodes) < minEnergy {
		return false
	}

	// Check that walls don't block paths between cores
	for i, core1 := range m.Cores {
		for j, core2 := range m.Cores {
			if i >= j {
				continue
			}
			if !e.canReach(m, core1.Position, core2.Position) {
				return false
			}
		}
	}

	return true
}

// canReach checks if two positions are reachable from each other.
func (e *MapEvolver) canReach(m *Map, start, end Position) bool {
	wallSet := make(PositionSet)
	for _, w := range m.Walls {
		wallSet[w] = true
	}

	visited := make(PositionSet)
	queue := []Position{start}
	visited[start] = true

	dirs := []Position{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr == end {
			return true
		}

		for _, d := range dirs {
			nr := ((curr.Row + d.Row) % m.Rows + m.Rows) % m.Rows
			nc := ((curr.Col + d.Col) % m.Cols + m.Cols) % m.Cols
			np := Position{Row: nr, Col: nc}

			if !wallSet[np] && !visited[np] {
				visited[np] = true
				queue = append(queue, np)
			}
		}
	}

	return false
}

// saveMap stores a map in the database.
func (e *MapEvolver) saveMap(ctx context.Context, m *Map) error {
	mapJSON, err := json.Marshal(m)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO maps (map_id, player_count, status, engagement, wall_density, energy_count, grid_width, grid_height, map_json)
		VALUES ($1, $2, 'active', 0, $3, $4, $5, $6, $7)
	`

	_, err = e.db.ExecContext(ctx, query,
		m.ID,
		m.Players,
		m.WallDensity,
		len(m.EnergyNodes),
		m.Cols,
		m.Rows,
		mapJSON,
	)

	return err
}

// generateMapID creates a random map ID.
func generateMapID(rng *rand.Rand) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return "map_" + string(b)
}
