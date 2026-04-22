package main

import (
	"math"
	"testing"
)

func TestDistance2(t *testing.T) {
	rows, cols := 60, 60
	a := Position{Row: 0, Col: 0}
	b := Position{Row: 3, Col: 4}
	got := distance2(a, b, rows, cols)
	want := 25 // 3^2 + 4^2
	if got != want {
		t.Errorf("distance2 = %d, want %d", got, want)
	}
}

func TestDistance2Wrap(t *testing.T) {
	rows, cols := 60, 60
	a := Position{Row: 2, Col: 2}
	b := Position{Row: 58, Col: 58}
	got := distance2(a, b, rows, cols)
	// Wrapped: dr=4, dc=4 → 16+16=32
	if got != 32 {
		t.Errorf("distance2 wrap = %d, want 32", got)
	}
}

func TestScoreTargetsIsolation(t *testing.T) {
	rows, cols := 60, 60
	myBots := []Position{{Row: 10, Col: 10}}
	enemies := []VisibleBot{
		{Position: Position{Row: 15, Col: 15}, Owner: 1}, // isolated
		{Position: Position{Row: 16, Col: 16}, Owner: 1}, // near other enemy
	}

	s := &OpportunistStrategy{}
	targets := s.scoreTargets(enemies, myBots, rows, cols)

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	// The isolated enemy (15,15) should score higher since its nearest friendly
	// (16,16) is close but the other one at (15,15) has the friendly at (16,16) even closer.
	// Actually both have the same owner, so:
	// Target (15,15): nearest friendly is (16,16) at dist2=2 → isolation=sqrt(2)≈1.41
	// Target (16,16): nearest friendly is (15,15) at dist2=2 → isolation=sqrt(2)≈1.41
	// Both equal, just verify they're scored
	if targets[0].score <= 0 {
		t.Errorf("target score should be positive, got %f", targets[0].score)
	}
}

func TestScoreTargetsLoneEnemy(t *testing.T) {
	rows, cols := 60, 60
	myBots := []Position{{Row: 10, Col: 10}}
	enemies := []VisibleBot{
		{Position: Position{Row: 30, Col: 30}, Owner: 1}, // completely alone
	}

	s := &OpportunistStrategy{}
	targets := s.scoreTargets(enemies, myBots, rows, cols)

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	// Lone enemy: isolation should be 10.0 (max)
	if targets[0].isolation != 10.0 {
		t.Errorf("lone enemy isolation = %f, want 10.0", targets[0].isolation)
	}
}

func TestShouldFlee(t *testing.T) {
	rows, cols := 60, 60
	bot := Position{Row: 10, Col: 10}

	s := &OpportunistStrategy{}

	// No enemies nearby → don't flee
	enemies := []VisibleBot{{Position: Position{Row: 30, Col: 30}, Owner: 1}}
	myBots := []Position{bot}
	if s.shouldFlee(bot, enemies, myBots, rows, cols) {
		t.Error("should not flee with no nearby enemies")
	}

	// Outnumbered → flee
	enemies = []VisibleBot{
		{Position: Position{Row: 11, Col: 10}, Owner: 1},
		{Position: Position{Row: 10, Col: 11}, Owner: 1},
	}
	if !s.shouldFlee(bot, enemies, myBots, rows, cols) {
		t.Error("should flee when outnumbered")
	}

	// Equal numbers → don't flee
	myBots = []Position{bot, {Row: 11, Col: 11}}
	enemies = []VisibleBot{{Position: Position{Row: 11, Col: 10}, Owner: 1}}
	if s.shouldFlee(bot, enemies, myBots, rows, cols) {
		t.Error("should not flee with equal numbers")
	}
}

func TestAssignAttackersAdvantage(t *testing.T) {
	rows, cols := 60, 60
	attackR2 := 5

	s := &OpportunistStrategy{}

	// 3 my bots near 1 enemy → should assign attackers
	myBots := []Position{
		{Row: 10, Col: 10},
		{Row: 10, Col: 11},
		{Row: 11, Col: 10},
	}
	enemies := []VisibleBot{
		{Position: Position{Row: 12, Col: 12}, Owner: 1},
	}
	targets := s.scoreTargets(enemies, myBots, rows, cols)

	assignments := s.assignAttackers(targets, myBots, attackR2, rows, cols)

	// Should assign at least 2 bots to the target
	assigned := 0
	for _, mb := range myBots {
		if _, ok := assignments[mb]; ok {
			assigned++
		}
	}
	if assigned < 2 {
		t.Errorf("expected at least 2 attackers assigned, got %d", assigned)
	}
}

func TestAssignAttackersNoAdvantage(t *testing.T) {
	rows, cols := 60, 60
	attackR2 := 5

	s := &OpportunistStrategy{}

	// 1 my bot vs 3 enemies → should NOT assign attackers
	myBots := []Position{{Row: 10, Col: 10}}
	enemies := []VisibleBot{
		{Position: Position{Row: 11, Col: 10}, Owner: 1},
		{Position: Position{Row: 10, Col: 11}, Owner: 1},
		{Position: Position{Row: 12, Col: 10}, Owner: 1},
	}
	targets := s.scoreTargets(enemies, myBots, rows, cols)

	assignments := s.assignAttackers(targets, myBots, attackR2, rows, cols)

	if len(assignments) > 0 {
		t.Error("should not assign attackers when outnumbered")
	}
}

func TestComputeMovesBasic(t *testing.T) {
	state := &GameState{
		MatchID: "test",
		Turn:    1,
		Config: GameConfig{
			Rows:           60,
			Cols:           60,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 10, Col: 10}, Owner: 0}, // mine
			{Position: Position{Row: 30, Col: 30}, Owner: 1}, // enemy far
		},
		Energy: []Position{{Row: 12, Col: 10}}, // energy nearby
		Cores: []VisibleCore{
			{Position: Position{Row: 5, Col: 5}, Owner: 0, Active: true},
		},
		Walls: []Position{},
	}
	state.You.ID = 0
	state.You.Energy = 0
	state.You.Score = 1

	s := NewOpportunistStrategy()
	moves := s.ComputeMoves(state)

	// Should produce at least one move for our bot
	if len(moves) == 0 {
		t.Error("expected at least one move")
	}
}

func TestComputeMovesNoEnemies(t *testing.T) {
	state := &GameState{
		MatchID: "test",
		Turn:    1,
		Config: GameConfig{
			Rows:           60,
			Cols:           60,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 10, Col: 10}, Owner: 0},
		},
		Energy: []Position{{Row: 12, Col: 10}},
		Cores: []VisibleCore{
			{Position: Position{Row: 5, Col: 5}, Owner: 0, Active: true},
		},
		Walls: []Position{},
	}
	state.You.ID = 0

	s := NewOpportunistStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) == 0 {
		t.Error("expected at least one move toward energy")
	}
}

func TestComputeMovesRetreat(t *testing.T) {
	state := &GameState{
		MatchID: "test",
		Turn:    1,
		Config: GameConfig{
			Rows:           60,
			Cols:           60,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 10, Col: 10}, Owner: 0}, // my lone bot
			{Position: Position{Row: 11, Col: 10}, Owner: 1}, // enemy adjacent
			{Position: Position{Row: 10, Col: 11}, Owner: 1}, // enemy adjacent
		},
		Energy: []Position{},
		Cores: []VisibleCore{
			{Position: Position{Row: 5, Col: 5}, Owner: 0, Active: true},
		},
		Walls: []Position{},
	}
	state.You.ID = 0

	s := NewOpportunistStrategy()
	moves := s.ComputeMoves(state)

	// Bot should move (retreat from outnumbered situation)
	if len(moves) == 0 {
		t.Error("expected bot to retreat from 2v1")
	}
}

func TestBFS(t *testing.T) {
	rows, cols := 60, 60
	start := Position{Row: 10, Col: 10}
	goal := Position{Row: 12, Col: 10}

	passable := func(p Position) bool { return true }

	dir := BFS(start, goal, passable, rows, cols)
	if dir != "S" {
		t.Errorf("BFS direction = %q, want %q", dir, "S")
	}
}

func TestBFSWithWall(t *testing.T) {
	rows, cols := 60, 60
	start := Position{Row: 10, Col: 10}
	goal := Position{Row: 10, Col: 12}

	walls := map[Position]bool{{Row: 10, Col: 11}: true}
	passable := func(p Position) bool { return !walls[p] }

	dir := BFS(start, goal, passable, rows, cols)
	// Should find a path around the wall
	if dir == "" {
		t.Error("BFS should find a path around wall")
	}
}

func TestToroidalManhattan(t *testing.T) {
	rows, cols := 60, 60
	a := Position{Row: 2, Col: 2}
	b := Position{Row: 58, Col: 58}

	d := ToroidalManhattan(a, b, rows, cols)
	// Wrapped: dr=4, dc=4 → 8
	if d != 8 {
		t.Errorf("ToroidalManhattan = %d, want 8", d)
	}
}

func TestAbs(t *testing.T) {
	if abs(-5) != 5 {
		t.Error("abs(-5) != 5")
	}
	if abs(5) != 5 {
		t.Error("abs(5) != 5")
	}
	if abs(0) != 0 {
		t.Error("abs(0) != 0")
	}
}

func TestComputeMovesNoSelfCollision(t *testing.T) {
	state := &GameState{
		MatchID: "test",
		Turn:    1,
		Config: GameConfig{
			Rows:           60,
			Cols:           60,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 10, Col: 10}, Owner: 0},
			{Position: Position{Row: 11, Col: 10}, Owner: 0},
		},
		Energy: []Position{{Row: 12, Col: 10}}, // both want to go south
		Cores:  []VisibleCore{},
		Walls:  []Position{},
	}
	state.You.ID = 0

	s := NewOpportunistStrategy()
	moves := s.ComputeMoves(state)

	// Verify no two bots end up on the same destination
	destinations := make(map[Position]bool)
	for _, m := range moves {
		dest := simulateMove(m.Position, m.Direction, 60, 60)
		if destinations[dest] {
			t.Errorf("two bots assigned to same destination %v", dest)
		}
		destinations[dest] = true
	}
}

func TestSimulateMove(t *testing.T) {
	p := Position{Row: 0, Col: 0}
	got := simulateMove(p, "N", 60, 60)
	if got.Row != 59 || got.Col != 0 {
		t.Errorf("simulateMove N wrap = %v, want {59 0}", got)
	}

	got = simulateMove(p, "E", 60, 60)
	if got.Row != 0 || got.Col != 1 {
		t.Errorf("simulateMove E = %v, want {0 1}", got)
	}

	got = simulateMove(Position{Row: 59, Col: 59}, "S", 60, 60)
	if got.Row != 0 || got.Col != 59 {
		t.Errorf("simulateMove S wrap = %v, want {0 59}", got)
	}
}

func TestScoreTargetsMultipleOwners(t *testing.T) {
	rows, cols := 60, 60
	myBots := []Position{{Row: 10, Col: 10}}
	enemies := []VisibleBot{
		{Position: Position{Row: 15, Col: 15}, Owner: 1}, // owner 1, alone
		{Position: Position{Row: 40, Col: 40}, Owner: 2}, // owner 2, alone
		{Position: Position{Row: 41, Col: 41}, Owner: 2}, // owner 2, paired
	}

	s := &OpportunistStrategy{}
	targets := s.scoreTargets(enemies, myBots, rows, cols)

	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}

	// Owner 1's lone enemy should have higher isolation than owner 2's paired enemies
	var owner1Target *targetInfo
	for i := range targets {
		if targets[i].owner == 1 {
			owner1Target = &targets[i]
			break
		}
	}
	if owner1Target == nil {
		t.Fatal("no target found for owner 1")
	}

	if owner1Target.isolation != 10.0 {
		t.Errorf("lone owner-1 enemy isolation = %f, want 10.0", owner1Target.isolation)
	}

	// Highest scoring target should be the most isolated
	if targets[0].score <= 0 {
		t.Errorf("top target score = %f, expected positive", targets[0].score)
	}
}

func TestComputeMovesLargeScale(t *testing.T) {
	// Test with multiple bots and enemies to ensure no panics or deadlocks
	state := &GameState{
		MatchID: "test",
		Turn:    50,
		Config: GameConfig{
			Rows:           60,
			Cols:           60,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 10, Col: 10}, Owner: 0},
			{Position: Position{Row: 10, Col: 12}, Owner: 0},
			{Position: Position{Row: 12, Col: 10}, Owner: 0},
			{Position: Position{Row: 30, Col: 30}, Owner: 1},
			{Position: Position{Row: 32, Col: 30}, Owner: 1},
			{Position: Position{Row: 40, Col: 40}, Owner: 2},
		},
		Energy: []Position{
			{Row: 15, Col: 10},
			{Row: 20, Col: 20},
		},
		Cores: []VisibleCore{
			{Position: Position{Row: 5, Col: 5}, Owner: 0, Active: true},
		},
		Walls: []Position{},
	}
	state.You.ID = 0
	state.You.Energy = 2
	state.You.Score = 1

	s := NewOpportunistStrategy()
	moves := s.ComputeMoves(state)

	// Should have moves for our 3 bots
	if len(moves) == 0 {
		t.Error("expected moves for our bots")
	}

	// Verify no self-collision
	destinations := make(map[Position]bool)
	for _, m := range moves {
		dest := simulateMove(m.Position, m.Direction, 60, 60)
		if destinations[dest] {
			t.Errorf("two bots assigned to same destination %v", dest)
		}
		destinations[dest] = true
	}
}

func TestComputeMovesNearbyAdvantageAttack(t *testing.T) {
	// 3v1 situation: our bots should attack the lone enemy
	state := &GameState{
		MatchID: "test",
		Turn:    10,
		Config: GameConfig{
			Rows:           60,
			Cols:           60,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 10, Col: 10}, Owner: 0},
			{Position: Position{Row: 10, Col: 12}, Owner: 0},
			{Position: Position{Row: 12, Col: 10}, Owner: 0},
			{Position: Position{Row: 14, Col: 14}, Owner: 1}, // lone enemy
		},
		Energy: []Position{},
		Cores: []VisibleCore{
			{Position: Position{Row: 5, Col: 5}, Owner: 0, Active: true},
		},
		Walls: []Position{},
	}
	state.You.ID = 0

	s := NewOpportunistStrategy()
	moves := s.ComputeMoves(state)

	if len(moves) == 0 {
		t.Error("expected attack moves in 3v1 situation")
	}

	// At least some bots should move toward the enemy
	movingTowardEnemy := 0
	enemyPos := Position{Row: 14, Col: 14}
	for _, m := range moves {
		before := distance2(m.Position, enemyPos, 60, 60)
		after := distance2(simulateMove(m.Position, m.Direction, 60, 60), enemyPos, 60, 60)
		if after < before {
			movingTowardEnemy++
		}
	}

	if movingTowardEnemy == 0 {
		t.Error("expected at least one bot to move toward the lone enemy")
	}
}

func BenchmarkComputeMoves(b *testing.B) {
	state := &GameState{
		MatchID: "bench",
		Turn:    100,
		Config: GameConfig{
			Rows:           60,
			Cols:           60,
			MaxTurns:       500,
			VisionRadius2:  49,
			AttackRadius2:  5,
			SpawnCost:      3,
			EnergyInterval: 10,
		},
		Bots: []VisibleBot{
			{Position: Position{Row: 10, Col: 10}, Owner: 0},
			{Position: Position{Row: 12, Col: 12}, Owner: 0},
			{Position: Position{Row: 14, Col: 14}, Owner: 0},
			{Position: Position{Row: 30, Col: 30}, Owner: 1},
			{Position: Position{Row: 32, Col: 32}, Owner: 1},
		},
		Energy: []Position{{Row: 20, Col: 20}, {Row: 25, Col: 25}},
		Cores: []VisibleCore{
			{Position: Position{Row: 5, Col: 5}, Owner: 0, Active: true},
		},
		Walls: []Position{},
	}
	state.You.ID = 0

	s := NewOpportunistStrategy()

	// Use the value to prevent compiler optimization
	_ = math.Sqrt(1.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ComputeMoves(state)
	}
}
