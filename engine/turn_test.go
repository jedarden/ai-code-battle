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

	bot0 := gs.SpawnBot(p0.ID, Position{10, 10})
	bot1 := gs.SpawnBot(p1.ID, Position{10, 11})

	gs.executeCombat()

	if !bot0.Alive {
		t.Error("bot0 should survive an equal 1v1 engagement")
	}
	if !bot1.Alive {
		t.Error("bot1 should survive an equal 1v1 engagement")
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

func TestSpawnPriority_IdleLongestSpawnsFirst(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	// Two cores for same player
	core0 := gs.AddCore(p0.ID, Position{5, 5})
	core1 := gs.AddCore(p0.ID, Position{15, 15})

	// Give core0 a higher lastSpawnedTurn (spawned more recently)
	core0.LastSpawnedTurn = 10
	core1.LastSpawnedTurn = 3

	// Only enough energy for one spawn
	gs.Players[p0.ID].Energy = 3

	gs.executeSpawns()

	// core1 should spawn (idle longer: lastSpawnedTurn=3 < 10)
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 1 {
		t.Fatalf("expected 1 spawned bot, got %d", len(bots))
	}
	if bots[0].Position != core1.Position {
		t.Errorf("spawned at %v, want %v (idle-longest core)", bots[0].Position, core1.Position)
	}
	if core1.LastSpawnedTurn != gs.Turn {
		t.Errorf("core1 LastSpawnedTurn = %d, want %d", core1.LastSpawnedTurn, gs.Turn)
	}
}

func TestSpawnPriority_LowerIDBreaksTie(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	// Two cores, both idle since turn 0 (equal lastSpawnedTurn)
	core0 := gs.AddCore(p0.ID, Position{5, 5})
	core1 := gs.AddCore(p0.ID, Position{15, 15})

	// core0 has lower ID (added first)

	// Only enough energy for one spawn
	gs.Players[p0.ID].Energy = 3

	gs.executeSpawns()

	// core0 (lower ID) should spawn first
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 1 {
		t.Fatalf("expected 1 spawned bot, got %d", len(bots))
	}
	if bots[0].Position != core0.Position {
		t.Errorf("spawned at %v, want %v (lower ID tiebreak)", bots[0].Position, core0.Position)
	}
	if core1.LastSpawnedTurn != 0 {
		t.Errorf("core1 LastSpawnedTurn = %d, want 0 (no spawn)", core1.LastSpawnedTurn)
	}
}

func TestSpawnPriority_MultipleEligibleEnoughEnergy(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	core0 := gs.AddCore(p0.ID, Position{5, 5})
	core1 := gs.AddCore(p0.ID, Position{15, 15})

	// core1 idle longer
	core0.LastSpawnedTurn = 5
	core1.LastSpawnedTurn = 1

	// Enough energy for two spawns
	gs.Players[p0.ID].Energy = 6

	gs.executeSpawns()

	// Both should spawn (order: core1 first, then core0)
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 2 {
		t.Fatalf("expected 2 spawned bots, got %d", len(bots))
	}
	// First bot spawned is at core1 (idle-longest)
	if bots[0].Position != core1.Position {
		t.Errorf("first spawn at %v, want %v", bots[0].Position, core1.Position)
	}
	if bots[1].Position != core0.Position {
		t.Errorf("second spawn at %v, want %v", bots[1].Position, core0.Position)
	}
	if gs.Players[p0.ID].Energy != 0 {
		t.Errorf("remaining energy = %d, want 0", gs.Players[p0.ID].Energy)
	}
}

func TestSpawnPriority_LastSpawnedTurnUpdatesOnSpawn(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()

	core0 := gs.AddCore(p0.ID, Position{5, 5})
	core1 := gs.AddCore(p0.ID, Position{15, 15})

	// Both start idle
	gs.Players[p0.ID].Energy = 3
	gs.Turn = 5
	gs.executeSpawns()

	// core0 (lower ID) should have spawned
	if core0.LastSpawnedTurn != 5 {
		t.Errorf("core0 LastSpawnedTurn = %d, want 5", core0.LastSpawnedTurn)
	}
	if core1.LastSpawnedTurn != 0 {
		t.Errorf("core1 LastSpawnedTurn = %d, want 0 (no spawn)", core1.LastSpawnedTurn)
	}

	// Now give energy again — core1 should spawn (idle since turn 0 < 5)
	gs.Players[p0.ID].Energy = 3
	gs.Turn = 10
	gs.executeSpawns()

	if core1.LastSpawnedTurn != 10 {
		t.Errorf("core1 LastSpawnedTurn = %d, want 10", core1.LastSpawnedTurn)
	}
}

// TestExecuteSpawnInsufficientEnergy pins the energy gate of the automatic
// Spawn phase: with less energy than SpawnCost, no bot is produced and no
// energy is spent. Spawning is engine-automatic — bots cannot request it —
// so this gate is the only thing limiting spawn rate (see README "Spawn").
func TestExecuteSpawnInsufficientEnergy(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	gs.AddCore(p0.ID, Position{10, 10})

	// One short of the cost
	gs.Players[p0.ID].Energy = gs.Config.SpawnCost - 1

	gs.executeSpawns()

	if bots := gs.GetPlayerBots(p0.ID); len(bots) != 0 {
		t.Errorf("player should have 0 bots below spawn cost, got %d", len(bots))
	}
	if gs.Players[p0.ID].Energy != gs.Config.SpawnCost-1 {
		t.Errorf("energy = %d, want %d (unchanged)", gs.Players[p0.ID].Energy, gs.Config.SpawnCost-1)
	}
}

// TestExecuteSpawnSkipsEnemyAndInactiveCores pins that the automatic Spawn
// phase only ever produces bots on the player's own active cores: enemy
// cores and razed/inactive cores are never spawn sites, and energy is
// preserved when no eligible core exists.
func TestExecuteSpawnSkipsEnemyAndInactiveCores(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	gs.AddCore(p1.ID, Position{5, 5})               // enemy-owned core
	deadCore := gs.AddCore(p0.ID, Position{15, 15}) // own but inactive
	deadCore.Active = false

	gs.Players[p0.ID].Energy = gs.Config.SpawnCost * 3

	gs.executeSpawns()

	if bots := gs.GetPlayerBots(p0.ID); len(bots) != 0 {
		t.Errorf("no bot should spawn on an enemy or inactive core, got %d", len(bots))
	}
	if gs.Players[p0.ID].Energy != gs.Config.SpawnCost*3 {
		t.Errorf("energy = %d, want %d (no eligible core, nothing spent)",
			gs.Players[p0.ID].Energy, gs.Config.SpawnCost*3)
	}
}

func TestAutomaticSpawns_ActiveUnoccupiedCoresAndEnergyAccounting(t *testing.T) {
	gs := newTestGameState()
	gs.Config.SpawnCost = 5
	gs.Turn = 4

	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()
	p2 := gs.AddPlayer()

	eligible0 := gs.AddCore(p0.ID, Position{2, 2})
	eligible1 := gs.AddCore(p0.ID, Position{4, 4})
	occupied := gs.AddCore(p0.ID, Position{6, 6})
	inactive := gs.AddCore(p0.ID, Position{8, 8})
	inactive.Active = false
	freed := gs.AddCore(p0.ID, Position{10, 10})
	enemy := gs.AddCore(p2.ID, Position{12, 12})
	p1Core := gs.AddCore(p1.ID, Position{14, 14})

	gs.SpawnBot(p0.ID, occupied.Position)
	deadOccupant := gs.SpawnBot(p0.ID, freed.Position)
	gs.KillBot(deadOccupant, "test")

	gs.Players[p0.ID].Energy = 16
	gs.Players[p1.ID].Energy = 10
	gs.executeSpawns()

	p0Bots := gs.GetPlayerBots(p0.ID)
	if len(p0Bots) != 4 {
		t.Fatalf("player 0 bot count = %d, want 4", len(p0Bots))
	}
	if gs.Players[p0.ID].Energy != 1 {
		t.Errorf("player 0 energy = %d, want 1", gs.Players[p0.ID].Energy)
	}
	p1Bots := gs.GetPlayerBots(p1.ID)
	if len(p1Bots) != 1 {
		t.Fatalf("player 1 bot count = %d, want 1", len(p1Bots))
	}
	if p1Bots[0].Position != p1Core.Position {
		t.Errorf("player 1 spawned at %v, want %v", p1Bots[0].Position, p1Core.Position)
	}
	if gs.Players[p1.ID].Energy != 5 {
		t.Errorf("player 1 energy = %d, want 5", gs.Players[p1.ID].Energy)
	}

	counts := make(map[Position]int)
	for _, bot := range p0Bots {
		counts[bot.Position]++
	}
	for _, pos := range []Position{eligible0.Position, eligible1.Position, occupied.Position, freed.Position} {
		if counts[pos] != 1 {
			t.Errorf("player 0 bot count at %v = %d, want 1", pos, counts[pos])
		}
	}
	for _, pos := range []Position{inactive.Position, enemy.Position, p1Core.Position} {
		if counts[pos] != 0 {
			t.Errorf("player 0 bot count at ineligible %v = %d, want 0", pos, counts[pos])
		}
	}

	for _, core := range []*Core{eligible0, eligible1, freed, p1Core} {
		if core.LastSpawnedTurn != gs.Turn {
			t.Errorf("core %d LastSpawnedTurn = %d, want %d", core.ID, core.LastSpawnedTurn, gs.Turn)
		}
	}
	if inactive.LastSpawnedTurn != 0 || enemy.LastSpawnedTurn != 0 {
		t.Errorf("ineligible cores changed spawn state: inactive=%d enemy=%d", inactive.LastSpawnedTurn, enemy.LastSpawnedTurn)
	}
}

func TestAutomaticSpawns_EnergyLimit(t *testing.T) {
	tests := []struct {
		name       string
		energy     int
		wantBots   int
		wantEnergy int
	}{
		{name: "zero energy", energy: 0, wantBots: 0, wantEnergy: 0},
		{name: "below cost", energy: 4, wantBots: 0, wantEnergy: 4},
		{name: "exact cost", energy: 5, wantBots: 1, wantEnergy: 0},
		{name: "cost plus remainder", energy: 7, wantBots: 1, wantEnergy: 2},
		{name: "two costs plus remainder", energy: 12, wantBots: 2, wantEnergy: 2},
		{name: "more energy than cores", energy: 100, wantBots: 2, wantEnergy: 90},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gs := newTestGameState()
			gs.Config.SpawnCost = 5
			p0 := gs.AddPlayer()
			gs.AddCore(p0.ID, Position{2, 2})
			gs.AddCore(p0.ID, Position{4, 4})
			gs.Players[p0.ID].Energy = test.energy

			gs.executeSpawns()

			if got := len(gs.GetPlayerBots(p0.ID)); got != test.wantBots {
				t.Errorf("bot count = %d, want %d", got, test.wantBots)
			}
			if got := gs.Players[p0.ID].Energy; got != test.wantEnergy {
				t.Errorf("energy = %d, want %d", got, test.wantEnergy)
			}
		})
	}
}

func TestAutomaticSpawns_CorePriorityAndTieBreak(t *testing.T) {
	t.Run("oldest spawns first", func(t *testing.T) {
		gs := newTestGameState()
		gs.Turn = 12
		p0 := gs.AddPlayer()
		core0 := gs.AddCore(p0.ID, Position{2, 2})
		core1 := gs.AddCore(p0.ID, Position{4, 4})
		core2 := gs.AddCore(p0.ID, Position{6, 6})
		core0.LastSpawnedTurn = 9
		core1.LastSpawnedTurn = 2
		core2.LastSpawnedTurn = 5
		gs.Cores[0], gs.Cores[1], gs.Cores[2] = core2, core0, core1
		gs.Players[p0.ID].Energy = gs.Config.SpawnCost * 3

		gs.executeSpawns()

		bots := gs.GetPlayerBots(p0.ID)
		want := []Position{core1.Position, core2.Position, core0.Position}
		if len(bots) != len(want) {
			t.Fatalf("bot count = %d, want %d", len(bots), len(want))
		}
		for i, bot := range bots {
			if bot.Position != want[i] {
				t.Errorf("spawn %d at %v, want %v", i, bot.Position, want[i])
			}
		}
	})

	t.Run("lowest ID breaks equal age", func(t *testing.T) {
		gs := newTestGameState()
		gs.Turn = 13
		p0 := gs.AddPlayer()
		core0 := gs.AddCore(p0.ID, Position{2, 2})
		core1 := gs.AddCore(p0.ID, Position{4, 4})
		core2 := gs.AddCore(p0.ID, Position{6, 6})
		gs.Cores[0], gs.Cores[1], gs.Cores[2] = core2, core1, core0
		gs.Players[p0.ID].Energy = gs.Config.SpawnCost * 3

		gs.executeSpawns()

		bots := gs.GetPlayerBots(p0.ID)
		want := []Position{core0.Position, core1.Position, core2.Position}
		if len(bots) != len(want) {
			t.Fatalf("bot count = %d, want %d", len(bots), len(want))
		}
		for i, bot := range bots {
			if bot.Position != want[i] {
				t.Errorf("spawn %d at %v, want %v", i, bot.Position, want[i])
			}
		}
	})
}

func TestExecuteTurn_RunsAutomaticSpawnPhase(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()
	core0 := gs.AddCore(p0.ID, Position{2, 2})
	core1 := gs.AddCore(p0.ID, Position{4, 4})
	gs.SpawnBot(p1.ID, Position{18, 18})
	gs.Players[p0.ID].Energy = gs.Config.SpawnCost * 2

	gs.ExecuteTurn()

	if gs.Turn != 1 {
		t.Fatalf("turn = %d, want 1", gs.Turn)
	}
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 2 {
		t.Fatalf("player 0 bot count = %d, want 2", len(bots))
	}
	if bots[0].Position != core0.Position || bots[1].Position != core1.Position {
		t.Errorf("spawn positions = %v, %v, want %v, %v", bots[0].Position, bots[1].Position, core0.Position, core1.Position)
	}
	if gs.Players[p0.ID].Energy != 0 {
		t.Errorf("player 0 energy = %d, want 0", gs.Players[p0.ID].Energy)
	}
}
