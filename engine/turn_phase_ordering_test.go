package engine

import "testing"

// These tests pin the documented turn-phase ordering (README "Turn Phases"):
// Move -> Combat -> Zone -> Capture -> Collect -> Spawn -> Energy Tick ->
// Endgame Check. Each test drives ExecuteTurn end-to-end and asserts an
// outcome that only holds if the non-combat phases run in that order.

// TestTurnOrder_MoveBeforeCapture pins that a movement order onto an
// undefended enemy core resolves in the same turn's Capture phase: the bot
// arrives first, then razes the core (README: "Enemy units on undefended
// Core tiles raze them").
func TestTurnOrder_MoveBeforeCapture(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	core := gs.AddCore(p0.ID, Position{10, 10}) // p0 score: 1
	attacker := gs.SpawnBot(p1.ID, Position{9, 10})
	gs.SubmitMove(attacker.Position, DirS) // -> {10,10}, the core tile
	// A distant p0 bot keeps the match alive so the test isolates capture.
	gs.SpawnBot(p0.ID, Position{15, 15})

	result := gs.ExecuteTurn()

	if core.Active {
		t.Error("core should be razed the same turn an enemy bot moves onto it")
	}
	if !attacker.Alive {
		t.Error("capturing bot should survive razing an undefended core")
	}
	if got := gs.Players[p1.ID].Score; got != 2 { // 0 + 2 capture award
		t.Errorf("capturer score = %d, want 2", got)
	}
	if got := gs.Players[p0.ID].Score; got != 0 { // 1 (core) - 1 capture penalty
		t.Errorf("core owner score = %d, want 0", got)
	}

	var captureEvent *Event
	for i := range gs.Events {
		if gs.Events[i].Type == EventCoreCaptured {
			captureEvent = &gs.Events[i]
		}
	}
	if captureEvent == nil {
		t.Fatal("no core_captured event emitted")
	}
	if captureEvent.Turn != gs.Turn {
		t.Errorf("capture event turn = %d, want %d", captureEvent.Turn, gs.Turn)
	}
	details, ok := captureEvent.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("capture event details = %T, want map[string]interface{}", captureEvent.Details)
	}
	if details["core_pos"] != (Position{10, 10}) {
		t.Errorf("capture event core_pos = %v, want {10,10}", details["core_pos"])
	}
	if details["old_owner"] != p0.ID || details["new_owner"] != p1.ID {
		t.Errorf("capture event owners = %v -> %v, want %d -> %d",
			details["old_owner"], details["new_owner"], p0.ID, p1.ID)
	}
	if result != nil {
		t.Errorf("expected no match result in a two-player game, got %+v", result)
	}
}

// TestTurnOrder_CollectFundsSameTurnSpawn pins Collect before Spawn: energy
// gathered this turn is immediately available to the automatic spawn phase
// later in the same turn (README Spawn: "while your energy covers the
// spawn_cost").
func TestTurnOrder_CollectFundsSameTurnSpawn(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	gs.AddPlayer() // p1 keeps this a two-player game

	core := gs.AddCore(p0.ID, Position{10, 10})
	en := gs.AddEnergyNode(Position{10, 12})
	en.HasEnergy = true
	collector := gs.SpawnBot(p0.ID, Position{10, 11}) // adjacent to node, off the core
	gs.SpawnBot(1, Position{5, 5})                    // far away, no interference

	// One short of spawn cost: only the same-turn collection completes it.
	gs.Players[p0.ID].Energy = gs.Config.SpawnCost - 1

	result := gs.ExecuteTurn()

	if got := gs.Players[p0.ID].Energy; got != 0 {
		t.Errorf("player energy = %d, want 0 (collected 1, spent %d)", got, gs.Config.SpawnCost)
	}
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 2 {
		t.Fatalf("player bots = %d, want 2 (collector + spawned)", len(bots))
	}
	spawnedAtCore := false
	for _, b := range bots {
		if b.ID != collector.ID && b.Position == core.Position {
			spawnedAtCore = true
		}
	}
	if !spawnedAtCore {
		t.Error("no bot spawned at the core despite collect-then-spawn funding")
	}
	if result != nil {
		t.Errorf("expected no match result, got %+v", result)
	}
}

// TestTurnOrder_SpawnedBotCollectsNextTurn pins the other side of Collect
// before Spawn: a bot produced this turn does not also collect this turn —
// its first collection opportunity is the next turn's Collect phase.
func TestTurnOrder_SpawnedBotCollectsNextTurn(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	gs.AddPlayer()

	core := gs.AddCore(p0.ID, Position{10, 10})
	en := gs.AddEnergyNode(Position{10, 11}) // adjacent to the core tile
	en.HasEnergy = true
	gs.SpawnBot(1, Position{5, 5}) // far away, no interference

	gs.Players[p0.ID].Energy = gs.Config.SpawnCost

	gs.ExecuteTurn() // spawns a bot at the core, next to the charged node

	bots := gs.GetPlayerBots(p0.ID)
	if got := len(bots); got != 1 {
		t.Fatalf("player bots = %d, want 1 (spawned at core)", got)
	}
	if bots[0].Position != core.Position {
		t.Errorf("spawned bot at %v, want core position %v", bots[0].Position, core.Position)
	}
	if !en.HasEnergy {
		t.Error("bot spawned this turn must not collect until the next turn's Collect phase")
	}
	if got := gs.Players[p0.ID].Energy; got != 0 {
		t.Errorf("player energy = %d, want 0 (spent on spawn, nothing collected)", got)
	}

	gs.ExecuteTurn() // bot stays put (no move) and collects

	if en.HasEnergy {
		t.Error("energy should be collected on the turn after the spawn")
	}
	if got := gs.Players[p0.ID].Energy; got != 1 {
		t.Errorf("player energy = %d, want 1 (collected next turn)", got)
	}
}

// TestTurnOrder_CollectBeforeEnergyTick pins Collect before Energy Tick: a
// node that (re)generates during this turn's Energy Tick phase is not
// collectible until the next turn.
func TestTurnOrder_CollectBeforeEnergyTick(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	gs.Config.EnergyInterval = 3
	p0 := gs.AddPlayer()
	gs.AddPlayer()

	en := gs.AddEnergyNode(Position{10, 12})
	en.Tick = gs.Config.EnergyInterval - 1 // regenerates during this turn's tick
	gs.SpawnBot(p0.ID, Position{10, 11})   // adjacent, ready to collect
	gs.SpawnBot(1, Position{5, 5})

	gs.ExecuteTurn()

	if !en.HasEnergy {
		t.Fatal("node should regenerate in the Energy Tick phase")
	}
	if got := gs.Players[p0.ID].Energy; got != 0 {
		t.Errorf("player energy = %d, want 0 (energy regenerating this turn is not collectible yet)", got)
	}

	gs.ExecuteTurn()

	if en.HasEnergy {
		t.Error("regenerated energy should be collectible the following turn")
	}
	if got := gs.Players[p0.ID].Energy; got != 1 {
		t.Errorf("player energy = %d, want 1 (collected the turn after regen)", got)
	}
}

// TestTurnOrder_CollectionResetsRegenTimer pins the passive regeneration
// cadence (README: energy "regenerated passively"): collecting resets the
// node's timer before the same turn's Energy Tick increments it, and the
// node produces new energy exactly one EnergyInterval later.
func TestTurnOrder_CollectionResetsRegenTimer(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	gs.Config.EnergyInterval = 3
	p0 := gs.AddPlayer()
	gs.AddPlayer()

	en := gs.AddEnergyNode(Position{10, 12})
	en.HasEnergy = true
	en.Tick = 7 // stale pre-collection ticks must not count toward regen
	gs.SpawnBot(p0.ID, Position{10, 11})
	gs.SpawnBot(1, Position{5, 5})

	gs.ExecuteTurn() // collect (timer reset to 0), then tick -> 1

	if got := gs.Players[p0.ID].Energy; got != 1 {
		t.Fatalf("player energy = %d, want 1 (collected turn 1)", got)
	}
	if en.HasEnergy {
		t.Fatal("node should be drained after collection")
	}
	if en.Tick != 1 {
		t.Errorf("node tick = %d after collecting turn, want 1 (reset by Collect, incremented by Energy Tick)", en.Tick)
	}

	gs.ExecuteTurn() // tick -> 2
	if en.Tick != 2 || en.HasEnergy {
		t.Errorf("node tick = %d hasEnergy = %v after 2 ticks, want 2/false", en.Tick, en.HasEnergy)
	}

	gs.ExecuteTurn() // tick -> 3 >= interval: regenerates
	if !en.HasEnergy || en.Tick != 0 {
		t.Errorf("node hasEnergy = %v tick = %d after interval, want true/0", en.HasEnergy, en.Tick)
	}
	if got := gs.Players[p0.ID].Energy; got != 1 {
		t.Errorf("player energy = %d before node is collectible again, want 1", got)
	}

	gs.ExecuteTurn() // bot collects the regenerated energy
	if got := gs.Players[p0.ID].Energy; got != 2 {
		t.Errorf("player energy = %d after regen cycle, want 2", got)
	}
}

// TestEndgame_EliminationBonusPerSurvivingEnemyCore pins the core-based
// elimination bonus: the sole survivor gains +2 per surviving enemy core,
// and its own cores are excluded (README win condition: cores decide the
// outcome).
func TestEndgame_EliminationBonusPerSurvivingEnemyCore(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	gs.AddCore(p0.ID, Position{2, 2})    // own core: excluded from bonus (p0 score 1)
	gs.AddCore(p1.ID, Position{5, 5})    // enemy core 1 (p1 score 2)
	gs.AddCore(p1.ID, Position{15, 15})  // enemy core 2
	gs.SpawnBot(p0.ID, Position{10, 10}) // sole survivor, not on any core

	result := gs.ExecuteTurn()

	if result == nil {
		t.Fatal("expected elimination result")
	}
	if result.Winner != p0.ID || result.Reason != "elimination" {
		t.Errorf("winner/reason = %d/%s, want %d/elimination", result.Winner, result.Reason, p0.ID)
	}
	// 1 (own core) + 2 bonus per surviving enemy core x 2
	if got := gs.Players[p0.ID].Score; got != 5 {
		t.Errorf("winner score = %d, want 5 (1 own core + 4 enemy-core bonus)", got)
	}
	if result.Scores[p0.ID] != 5 || result.Scores[p1.ID] != 2 {
		t.Errorf("result scores = %v, want winner 5 (own cores excluded from bonus) and loser 2", result.Scores)
	}
}

// TestTurnOrder_CaptureBeforeEndgameBonus pins Capture before Endgame Check:
// enemy cores razed earlier in the same turn no longer count as "surviving"
// when the elimination bonus is computed.
func TestTurnOrder_CaptureBeforeEndgameBonus(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	razed := gs.AddCore(p1.ID, Position{5, 5})
	survivor1 := gs.AddCore(p1.ID, Position{15, 15})
	survivor2 := gs.AddCore(p1.ID, Position{18, 18}) // p1 score: 3
	attacker := gs.SpawnBot(p0.ID, Position{4, 5})
	gs.SubmitMove(attacker.Position, DirS) // -> {5,5}, undefended enemy core

	result := gs.ExecuteTurn()

	if result == nil {
		t.Fatal("expected elimination result (p1 has no bots)")
	}
	if result.Winner != p0.ID || result.Reason != "elimination" {
		t.Errorf("winner/reason = %d/%s, want %d/elimination", result.Winner, result.Reason, p0.ID)
	}
	if razed.Active || !survivor1.Active || !survivor2.Active {
		t.Error("exactly the moved-onto core should be razed this turn")
	}
	// Capturer: 0 + 2 (capture) + 2x2 (two surviving enemy cores).
	if got := result.Scores[p0.ID]; got != 6 {
		t.Errorf("winner score = %d, want 6 (capture award + bonus for cores still standing)", got)
	}
	// Owner: 3 (cores) - 1 (capture penalty). If the endgame check ran
	// before captures, this would still be 3 and no bonus exclusion occurs.
	if got := result.Scores[p1.ID]; got != 2 {
		t.Errorf("loser score = %d, want 2 (core penalty applied before result)", got)
	}
}

// TestTurnOrder_SpawnBeforeEndgame pins Spawn before Endgame Check: a bot
// produced this turn counts as alive, so a player whose only bot arrives via
// the automatic spawn is not eliminated this turn.
func TestTurnOrder_SpawnBeforeEndgame(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	core := gs.AddCore(p0.ID, Position{10, 10})
	gs.Players[p0.ID].Energy = gs.Config.SpawnCost // funds exactly one spawn
	gs.SpawnBot(p1.ID, Position{5, 5})             // far away, no combat contact

	result := gs.ExecuteTurn()

	if result != nil {
		t.Fatalf("same-turn spawn must keep both players alive, got winner=%d reason=%s", result.Winner, result.Reason)
	}
	bots := gs.GetPlayerBots(p0.ID)
	if len(bots) != 1 || bots[0].Position != core.Position {
		t.Errorf("expected 1 bot spawned at the core, got %v", bots)
	}
	if got := gs.Players[p0.ID].Energy; got != 0 {
		t.Errorf("player energy = %d, want 0 (spent on the spawn)", got)
	}
}

// TestEndgame_EliminationPrecedesTurnLimit pins the endgame check order:
// sole-survivor elimination is evaluated before the turn-limit score
// comparison, so a match that hits MaxTurns with one player left ends by
// elimination, not "turns".
func TestEndgame_EliminationPrecedesTurnLimit(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	gs.AddPlayer() // p1: no bots

	gs.SpawnBot(p0.ID, Position{10, 10})
	gs.Turn = gs.Config.MaxTurns - 1 // this ExecuteTurn reaches the limit

	result := gs.ExecuteTurn()

	if result == nil {
		t.Fatal("expected a match result at the turn limit")
	}
	if result.Winner != p0.ID || result.Reason != "elimination" {
		t.Errorf("winner/reason = %d/%s, want %d/elimination (elimination beats turn limit)", result.Winner, result.Reason, p0.ID)
	}
}

// TestEndgame_DrawPrecedesTurnLimit pins that simultaneous annihilation
// (here via self-collision, a non-combat death) is evaluated before the
// turn-limit check: the match ends as a draw, not a score decision.
func TestEndgame_DrawPrecedesTurnLimit(t *testing.T) {
	gs := newTestGameState()
	gs.Config.ZoneEnabled = false
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	// Both players' entire forces self-collide during Move.
	a := gs.SpawnBot(p0.ID, Position{10, 10})
	b := gs.SpawnBot(p0.ID, Position{10, 12})
	gs.SubmitMove(a.Position, DirE) // both -> {10,11}
	gs.SubmitMove(b.Position, DirW)
	c := gs.SpawnBot(p1.ID, Position{5, 5})
	d := gs.SpawnBot(p1.ID, Position{5, 7})
	gs.SubmitMove(c.Position, DirE) // both -> {5,6}
	gs.SubmitMove(d.Position, DirW)

	gs.Turn = gs.Config.MaxTurns - 1

	result := gs.ExecuteTurn()

	if result == nil {
		t.Fatal("expected a match result")
	}
	if result.Winner != -1 || result.Reason != "draw" {
		t.Errorf("winner/reason = %d/%s, want -1/draw (annihilation beats turn limit)", result.Winner, result.Reason)
	}
	if gs.GetLivingBotCount() != 0 {
		t.Errorf("living bots = %d, want 0 (all self-collided)", gs.GetLivingBotCount())
	}
}
