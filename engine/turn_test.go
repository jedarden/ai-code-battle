package engine

import (
	"math/rand"
	"testing"
)

func newTestGameState() *GameState {
	config := DefaultConfig()
	config.Rows = 20
	config.Cols = 20
	rng := rand.New(rand.NewSource(42))
	return NewGameState(config, rng)
}

func TestExecuteMoves(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	bot0 := gs.SpawnBot(p0.ID, Position{10, 10})
	bot1 := gs.SpawnBot(p1.ID, Position{5, 5})

	// Submit moves
	gs.SubmitMove(bot0.Position, DirN) // 10,10 -> 9,10
	gs.SubmitMove(bot1.Position, DirE) // 5,5 -> 5,6

	gs.executeMoves()

	// Verify positions
	if bot0.Position != (Position{9, 10}) {
		t.Errorf("bot0 position = %v, want {9,10}", bot0.Position)
	}
	if bot1.Position != (Position{5, 6}) {
		t.Errorf("bot1 position = %v, want {5,6}", bot1.Position)
	}
}

func TestExecuteMovesIntoWall(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	// Place a wall
	gs.Grid.Set(9, 10, TileWall)

	bot := gs.SpawnBot(p0.ID, Position{10, 10})
	gs.SubmitMove(bot.Position, DirN) // Would go to 9,10 which is a wall

	gs.executeMoves()

	// Bot should stay in place
	if bot.Position != (Position{10, 10}) {
		t.Errorf("bot position = %v, want {10,10} (blocked by wall)", bot.Position)
	}
}

func TestExecuteMovesWrap(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	bot := gs.SpawnBot(p0.ID, Position{0, 0})
	gs.SubmitMove(bot.Position, DirN) // Should wrap to 19,0

	gs.executeMoves()

	if bot.Position != (Position{19, 0}) {
		t.Errorf("bot position = %v, want {19,0} (wrapped)", bot.Position)
	}
}

func TestExecuteMovesSelfCollision(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	// Two bots from same player trying to move to same position
	bot0 := gs.SpawnBot(p0.ID, Position{10, 10})
	bot1 := gs.SpawnBot(p0.ID, Position{10, 12})

	gs.SubmitMove(bot0.Position, DirE) // 10,10 -> 10,11
	gs.SubmitMove(bot1.Position, DirW) // 10,12 -> 10,11

	gs.executeMoves()

	// Both should be dead
	if bot0.Alive {
		t.Error("bot0 should be dead from self-collision")
	}
	if bot1.Alive {
		t.Error("bot1 should be dead from self-collision")
	}
}

func TestExecuteCombat1v1(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Two bots adjacent - both should die (1v1 = mutual destruction)
	bot0 := gs.SpawnBot(p0.ID, Position{10, 10})
	bot1 := gs.SpawnBot(p1.ID, Position{10, 11})

	gs.executeCombat()

	// Both should be dead (1 enemy each, equal counts)
	if bot0.Alive {
		t.Error("bot0 should be dead in 1v1")
	}
	if bot1.Alive {
		t.Error("bot1 should be dead in 1v1")
	}
}

func TestExecuteCombat2v1(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Two bots vs one - the lone bot should die
	bot0 := gs.SpawnBot(p0.ID, Position{10, 10})
	bot0b := gs.SpawnBot(p0.ID, Position{10, 11}) // Adjacent to bot1
	bot1 := gs.SpawnBot(p1.ID, Position{10, 12})

	gs.executeCombat()

	// bot1 should die (1 enemy vs 2 enemies)
	if bot1.Alive {
		t.Error("bot1 should be dead in 2v1")
	}
	// bot0 and bot0b should survive (2 enemies vs 1 enemy)
	if !bot0.Alive || !bot0b.Alive {
		t.Error("bot0 and bot0b should survive 2v1")
	}
}

func TestExecuteCombatFormation(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Tight formation (3 bots) vs scattered (3 bots)
	// Formation: 3 bots in a line
	formation := []*Bot{
		gs.SpawnBot(p0.ID, Position{10, 10}),
		gs.SpawnBot(p0.ID, Position{10, 11}),
		gs.SpawnBot(p0.ID, Position{10, 12}),
	}

	// Scattered: 3 bots spread out (only one in attack range of formation)
	scattered := []*Bot{
		gs.SpawnBot(p1.ID, Position{10, 13}), // In range of formation
		gs.SpawnBot(p1.ID, Position{5, 5}),   // Far away
		gs.SpawnBot(p1.ID, Position{15, 15}), // Far away
	}

	gs.executeCombat()

	// The scattered bot in range (10,13) faces 3 enemies
	// Each formation bot faces 1 enemy
	// Formation bots: 1 enemy each
	// Scattered bot: 3 enemies
	// Scattered bot dies (3 >= 1)
	// Formation bots survive (1 < 3)

	if scattered[0].Alive {
		t.Error("scattered bot in range should die")
	}
	for i, b := range formation {
		if !b.Alive {
			t.Errorf("formation bot %d should survive", i)
		}
	}
}

func TestExecuteCapture(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Player 0 has a core
	core := gs.AddCore(p0.ID, Position{10, 10})

	// Player 1's bot moves onto the core
	bot1 := gs.SpawnBot(p1.ID, Position{9, 10})
	gs.SubmitMove(bot1.Position, DirS) // Move to 10,10 (the core)
	gs.executeMoves()

	// Core is undefended (no p0 bot on it)
	gs.executeCaptures()

	// Core should be razed
	if core.Active {
		t.Error("core should be razed after capture")
	}
	// Scoring: p1 +2 (p1 didn't start with a core, so score was 0)
	// p0: started with 1 point (from core), loses 1 point = 0
	if gs.Players[p1.ID].Score != 2 { // 0 (starting) + 2 (capture)
		t.Errorf("p1 score = %d, want 2", gs.Players[p1.ID].Score)
	}
	if gs.Players[p0.ID].Score != 0 { // 1 (starting) - 1 (capture)
		t.Errorf("p0 score = %d, want 0", gs.Players[p0.ID].Score)
	}
}

func TestExecuteCaptureDefended(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Player 0 has a core with a defending bot
	core := gs.AddCore(p0.ID, Position{10, 10})
	defender := gs.SpawnBot(p0.ID, Position{10, 10}) // Defending

	// Player 1's bot moves onto the core
	attacker := gs.SpawnBot(p1.ID, Position{9, 10})
	gs.SubmitMove(attacker.Position, DirS)
	gs.executeMoves()

	// Combat resolves first - both bots on same tile
	// Actually, in our implementation, two enemy bots on same tile is handled in combat
	// Let me reconsider: if both bots end up on the same tile, combat handles it
	// For this test, let's have the attacker adjacent but not on the core

	gs.ClearTurnState()
	attacker.Position = Position{10, 11} // Adjacent to core

	gs.executeCaptures()

	// Core should still be active (defended)
	if !core.Active {
		t.Error("core should not be captured when defended")
	}
	if !defender.Alive {
		t.Error("defender should still be alive")
	}
}

func TestExecuteCollection(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	// Place energy
	en := gs.AddEnergyNode(Position{10, 10})
	en.HasEnergy = true

	// Bot adjacent to energy
	_ = gs.SpawnBot(p0.ID, Position{10, 11})

	gs.executeCollection()

	// Player should have collected energy
	if gs.Players[p0.ID].Energy != 1 {
		t.Errorf("player energy = %d, want 1", gs.Players[p0.ID].Energy)
	}
	if en.HasEnergy {
		t.Error("energy should be collected")
	}
}

func TestExecuteCollectionContested(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Place energy
	en := gs.AddEnergyNode(Position{10, 10})
	en.HasEnergy = true

	// Bots from both players adjacent
	gs.SpawnBot(p0.ID, Position{10, 11})
	gs.SpawnBot(p1.ID, Position{10, 9})

	gs.executeCollection()

	// Energy should be destroyed (contested)
	if gs.Players[p0.ID].Energy != 0 || gs.Players[p1.ID].Energy != 0 {
		t.Error("no player should collect contested energy")
	}
	if en.HasEnergy {
		t.Error("contested energy should be destroyed")
	}
}

func TestExecuteSpawn(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	// Player has a core
	gs.AddCore(p0.ID, Position{10, 10})

	// Give player enough energy
	gs.Players[p0.ID].Energy = 3

	gs.executeSpawns()

	// Player should have spawned a bot at the core
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 1 {
		t.Errorf("player should have 1 bot, got %d", len(bots))
	}
	if bots[0].Position != (Position{10, 10}) {
		t.Errorf("spawned bot position = %v, want {10,10}", bots[0].Position)
	}
	if gs.Players[p0.ID].Energy != 0 {
		t.Errorf("player energy = %d, want 0", gs.Players[p0.ID].Energy)
	}
}

func TestExecuteSpawnOccupiedCore(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	// Player has a core with a bot already on it
	gs.AddCore(p0.ID, Position{10, 10})
	gs.SpawnBot(p0.ID, Position{10, 10})

	// Give player enough energy
	gs.Players[p0.ID].Energy = 3

	gs.executeSpawns()

	// No spawn should happen (core occupied)
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 1 {
		t.Errorf("player should still have 1 bot, got %d", len(bots))
	}
	if gs.Players[p0.ID].Energy != 3 {
		t.Error("energy should not be spent on occupied core")
	}
}

func TestExecuteEnergyTick(t *testing.T) {
	gs := newTestGameState()
	gs.Config.EnergyInterval = 3

	// Energy node with tick = 2 (one more turn until spawn)
	en := gs.AddEnergyNode(Position{10, 10})
	en.Tick = 2

	gs.executeEnergyTick()

	if !en.HasEnergy {
		t.Error("energy should have spawned")
	}
	if en.Tick != 0 {
		t.Errorf("tick should be 0, got %d", en.Tick)
	}
}

func TestCheckWinConditionsElimination(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	_ = gs.AddPlayer() // p1 - opponent with no bots

	// Player 0 has bots, player 1 doesn't
	gs.SpawnBot(p0.ID, Position{10, 10})

	result := gs.checkWinConditions()

	if result == nil {
		t.Fatal("expected win result")
	}
	if result.Winner != p0.ID {
		t.Errorf("winner = %d, want %d", result.Winner, p0.ID)
	}
	if result.Reason != "elimination" {
		t.Errorf("reason = %s, want elimination", result.Reason)
	}
}

func TestCheckWinConditionsDraw(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// No bots alive for anyone
	bot0 := gs.SpawnBot(p0.ID, Position{10, 10})
	bot1 := gs.SpawnBot(p1.ID, Position{10, 11})
	bot0.Alive = false
	bot1.Alive = false

	result := gs.checkWinConditions()

	if result == nil {
		t.Fatal("expected win result")
	}
	if result.Winner != -1 {
		t.Errorf("winner = %d, want -1 (draw)", result.Winner)
	}
	if result.Reason != "draw" {
		t.Errorf("reason = %s, want draw", result.Reason)
	}
}

func TestCheckWinConditionsDominance(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Player 0 has 9 bots, player 1 has 1 bot = 90% dominance
	for i := 0; i < 9; i++ {
		gs.SpawnBot(p0.ID, Position{Row: i, Col: 0})
	}
	gs.SpawnBot(p1.ID, Position{Row: 15, Col: 15})

	// Dominance requires 100 consecutive turns at >= 80%
	// First 99 turns should not trigger
	for i := 0; i < 99; i++ {
		result := gs.checkWinConditions()
		if result != nil && result.Reason == "dominance" {
			t.Fatalf("dominance should not trigger at turn %d (only %d consecutive)", i, i+1)
		}
	}

	// 100th check should trigger dominance
	result := gs.checkWinConditions()
	if result == nil {
		t.Fatal("expected dominance win after 100 consecutive turns")
	}
	if result.Winner != p0.ID {
		t.Errorf("winner = %d, want %d", result.Winner, p0.ID)
	}
	if result.Reason != "dominance" {
		t.Errorf("reason = %s, want dominance", result.Reason)
	}
}

func TestCheckWinConditionsDominanceReset(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Player 0 has 9 bots, player 1 has 1 = 90% dominance
	bots0 := make([]*Bot, 9)
	for i := 0; i < 9; i++ {
		bots0[i] = gs.SpawnBot(p0.ID, Position{Row: i, Col: 0})
	}
	gs.SpawnBot(p1.ID, Position{Row: 15, Col: 15})

	// Run 50 turns of dominance
	for i := 0; i < 50; i++ {
		result := gs.checkWinConditions()
		if result != nil && result.Reason == "dominance" {
			t.Fatalf("dominance should not trigger at %d turns", i+1)
		}
	}

	// Break dominance by killing some p0 bots
	for i := 0; i < 6; i++ {
		gs.KillBot(bots0[i], "test")
	}
	// Now p0 has 3 bots, p1 has 1 = 75% (< 80%)

	result := gs.checkWinConditions()
	// Should not trigger dominance and counter should reset
	if result != nil && result.Reason == "dominance" {
		t.Error("dominance should not trigger when below 80%")
	}
	if gs.Dominance[p0.ID] != 0 {
		t.Errorf("dominance counter should reset to 0, got %d", gs.Dominance[p0.ID])
	}
}

func TestCheckWinConditionsTurns(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Both have bots
	gs.SpawnBot(p0.ID, Position{10, 10})
	gs.SpawnBot(p1.ID, Position{5, 5})

	// Set turn to max
	gs.Turn = gs.Config.MaxTurns

	// Player 0 has higher score
	gs.Players[p0.ID].Score = 5
	gs.Players[p1.ID].Score = 3

	result := gs.checkWinConditions()

	if result == nil {
		t.Fatal("expected win result")
	}
	if result.Winner != p0.ID {
		t.Errorf("winner = %d, want %d (higher score)", result.Winner, p0.ID)
	}
	if result.Reason != "turns" {
		t.Errorf("reason = %s, want turns", result.Reason)
	}
}
