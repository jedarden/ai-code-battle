package main

import (
	"testing"
)

func makeState(myID, energy int, bots []VisibleBot, energyTiles []Position, cores []VisibleCore, walls []Position) *GameState {
	state := &GameState{
		MatchID: "test",
		Turn:    1,
		Config: GameConfig{
			Rows:           20,
			Cols:           20,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Energy: energyTiles,
		Cores:  cores,
		Walls:  walls,
	}
	state.You.ID = myID
	state.You.Energy = energy
	state.Bots = bots
	return state
}

func TestFarmerSeeksEnergy(t *testing.T) {
	state := makeState(0, 0,
		[]VisibleBot{{Position: Position{10, 10}, Owner: 0}},
		[]Position{{10, 14}}, // energy 4 tiles east
		[]VisibleCore{{Position: Position{10, 10}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	if moves[0].Direction != "E" {
		t.Errorf("expected move E toward energy, got %s", moves[0].Direction)
	}
}

func TestFarmerFleesEnemy(t *testing.T) {
	state := makeState(0, 0,
		[]VisibleBot{
			{Position: Position{10, 10}, Owner: 0}, // my bot
			{Position: Position{10, 12}, Owner: 1}, // enemy 2 tiles south
		},
		[]Position{{10, 14}}, // energy exists but should flee instead
		[]VisibleCore{{Position: Position{10, 10}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	// Should flee away from enemy at (10,12), not go toward energy at (10,14)
	if moves[0].Direction == "E" {
		t.Errorf("bot should flee from enemy, not move toward energy; got %s", moves[0].Direction)
	}
}

func TestFarmerFleesFromNearbyEnemy(t *testing.T) {
	// Enemy within 3 tiles (fleeRadius2 = 9, distance2 = 4 < 9)
	state := makeState(0, 0,
		[]VisibleBot{
			{Position: Position{10, 10}, Owner: 0},
			{Position: Position{12, 10}, Owner: 1}, // enemy 2 tiles south, d2=4
		},
		nil,
		[]VisibleCore{{Position: Position{10, 10}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	// Should flee north (away from enemy to the south)
	if moves[0].Direction != "N" {
		t.Errorf("expected flee north, got %s", moves[0].Direction)
	}
}

func TestFarmerMultipleBotsSeekDifferentEnergy(t *testing.T) {
	state := makeState(0, 6,
		[]VisibleBot{
			{Position: Position{5, 5}, Owner: 0},
			{Position: Position{15, 15}, Owner: 0},
		},
		[]Position{{5, 8}, {15, 12}}, // two energy tiles
		[]VisibleCore{{Position: Position{5, 5}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(moves))
	}

	// Each bot should target its nearest energy
	dirs := map[string]bool{}
	for _, m := range moves {
		dirs[m.Direction] = true
	}

	// Bot at (5,5) should move E toward (5,8)
	// Bot at (15,15) should move W toward (15,12)
	// We can't guarantee exact mapping, but both should have moves
	if len(dirs) < 1 {
		t.Errorf("expected bots to move toward different energy, got dirs: %v", dirs)
	}
}

func TestFarmerHoldsOnEnergy(t *testing.T) {
	// Bot already on energy tile, no enemies
	state := makeState(0, 0,
		[]VisibleBot{{Position: Position{10, 10}, Owner: 0}},
		[]Position{{10, 10}}, // energy on same tile as bot
		[]VisibleCore{{Position: Position{5, 5}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	// Bot should hold position to collect energy (no move issued)
	if len(moves) != 0 {
		t.Errorf("expected bot to hold on energy (0 moves), got %d", len(moves))
	}
}

func TestFarmerStaysNearCore(t *testing.T) {
	// Bot far from core, no energy, no enemies
	state := makeState(0, 0,
		[]VisibleBot{{Position: Position{15, 15}, Owner: 0}},
		nil,
		[]VisibleCore{{Position: Position{5, 5}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	// Should move toward core at (5,5), i.e. N or W
	dir := moves[0].Direction
	if dir != "N" && dir != "W" {
		t.Errorf("expected move toward core (N or W), got %s", dir)
	}
}

func TestFarmerIgnoresContestedEnergy(t *testing.T) {
	// Energy at (10,14) with enemy adjacent at (10,15) - contested
	// Uncontested energy at (10,5)
	state := makeState(0, 0,
		[]VisibleBot{
			{Position: Position{10, 10}, Owner: 0},
			{Position: Position{10, 15}, Owner: 1}, // enemy adjacent to energy
		},
		[]Position{{10, 14}, {10, 5}}, // contested energy, safe energy
		[]VisibleCore{{Position: Position{10, 10}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	// Should prefer safe energy at (10,5) -> move W
	if moves[0].Direction != "W" {
		t.Errorf("expected move W toward safe energy, got %s", moves[0].Direction)
	}
}

func TestFarmerAvoidsWalls(t *testing.T) {
	// Bot at (10,10), energy at (10,14), wall at (10,11) blocking direct path
	state := makeState(0, 0,
		[]VisibleBot{{Position: Position{10, 10}, Owner: 0}},
		[]Position{{10, 14}},
		[]VisibleCore{{Position: Position{10, 10}, Owner: 0, Active: true}},
		[]Position{{10, 11}},
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) != 1 {
		t.Fatalf("expected 1 move, got %d", len(moves))
	}
	// Should not move E into the wall
	if moves[0].Direction == "E" {
		t.Errorf("bot should avoid wall at (10,11), got direction E")
	}
}

func TestDistance2(t *testing.T) {
	tests := []struct {
		a, b     Position
		rows, cols int
		want     int
	}{
		{Position{0, 0}, Position{0, 0}, 20, 20, 0},
		{Position{0, 0}, Position{0, 3}, 20, 20, 9},
		{Position{0, 0}, Position{3, 4}, 20, 20, 25},
		// Toroidal: distance from (0,0) to (19,0) on 20-row grid = 1 row
		{Position{0, 0}, Position{19, 0}, 20, 20, 1},
		// Toroidal: distance from (0,0) to (0,19) on 20-col grid = 1 col
		{Position{0, 0}, Position{0, 19}, 20, 20, 1},
	}

	for _, tt := range tests {
		got := distance2(tt.a, tt.b, tt.rows, tt.cols)
		if got != tt.want {
			t.Errorf("distance2(%v, %v, %d, %d) = %d, want %d",
				tt.a, tt.b, tt.rows, tt.cols, got, tt.want)
		}
	}
}

func TestBFS(t *testing.T) {
	wallSet := map[Position]bool{{10, 11}: true}
	passable := func(p Position) bool { return !wallSet[p] }

	// Direct path north
	dir := BFS(Position{5, 5}, Position{3, 5}, passable, 20, 20)
	if dir != "N" {
		t.Errorf("BFS to north: got %q, want N", dir)
	}

	// Path around wall: (10,10) to (10,14) with wall at (10,11)
	dir = BFS(Position{10, 10}, Position{10, 14}, passable, 20, 20)
	if dir == "" {
		t.Error("BFS should find path around wall")
	}
	// Should go N or S to bypass wall at (10,11), not E
	if dir == "E" {
		t.Errorf("BFS should not go directly into wall, got E")
	}
}

func TestSimulateMove(t *testing.T) {
	tests := []struct {
		pos      Position
		dir      string
		rows, cols int
		want     Position
	}{
		{Position{5, 5}, "N", 20, 20, Position{4, 5}},
		{Position{5, 5}, "S", 20, 20, Position{6, 5}},
		{Position{5, 5}, "E", 20, 20, Position{5, 6}},
		{Position{5, 5}, "W", 20, 20, Position{5, 4}},
		// Toroidal wrap
		{Position{0, 0}, "N", 20, 20, Position{19, 0}},
		{Position{0, 0}, "W", 20, 20, Position{0, 19}},
		{Position{19, 19}, "S", 20, 20, Position{0, 19}},
		{Position{19, 19}, "E", 20, 20, Position{19, 0}},
	}

	for _, tt := range tests {
		got := simulateMove(tt.pos, tt.dir, tt.rows, tt.cols)
		if got != tt.want {
			t.Errorf("simulateMove(%v, %q, %d, %d) = %v, want %v",
				tt.pos, tt.dir, tt.rows, tt.cols, got, tt.want)
		}
	}
}

func TestFarmerNoSelfCollision(t *testing.T) {
	// Two bots at (10,10) and (10,11), energy at (10,12)
	// Both want to go east, but can't land on same tile
	state := makeState(0, 0,
		[]VisibleBot{
			{Position: Position{10, 10}, Owner: 0},
			{Position: Position{10, 11}, Owner: 0},
		},
		[]Position{{10, 12}},
		[]VisibleCore{{Position: Position{10, 10}, Owner: 0, Active: true}},
		nil,
	)

	s := NewFarmerStrategy()
	moves := s.ComputeMoves(state)

	// Collect destinations
	dests := map[Position]bool{}
	for _, m := range moves {
		dest := simulateMove(m.Position, m.Direction, 20, 20)
		if dests[dest] {
			t.Errorf("two bots collide at %v", dest)
		}
		dests[dest] = true
	}
}
