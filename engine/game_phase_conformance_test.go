package engine

import "testing"

// These tests pin documented non-combat phase rules that no other test
// isolates. Each assertion traces to a line in README "Game Rules" or
// docs/plan/plan.md section 3 (Game Mechanics); the focus-fire combat phase
// is deliberately absent — it is covered by the combat conformance work
// (engine/bot_strategies.go analysis, docs/focus-fire-analysis.md, and the
// combat tests in turn_test.go) and must not be re-pinned here.

// TestPhaseConformance_TurnLimitTiebreakChain pins the documented turn-limit
// resolution (plan §3.6: "Highest score wins; ties broken by energy
// collected, then bots alive"): score decides outright, energy only breaks
// score ties, living bots only break score-and-energy ties.
func TestPhaseConformance_TurnLimitTiebreakChain(t *testing.T) {
	t.Run("higher score wins outright", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		p1 := gs.AddPlayer()
		gs.SpawnBot(p0.ID, Position{10, 10})
		gs.SpawnBot(p1.ID, Position{5, 5})

		gs.Players[p0.ID].Score = 4
		gs.Players[p1.ID].Score = 3
		// p1 hoards a decisive energy lead — score must still decide.
		gs.Players[p0.ID].Energy = 0
		gs.Players[p1.ID].Energy = 99
		gs.Turn = gs.Config.MaxTurns

		result := gs.checkWinConditions()

		if result == nil {
			t.Fatal("expected a result at the turn limit")
		}
		if result.Winner != p0.ID || result.Reason != "turns" {
			t.Errorf("winner/reason = %d/%s, want %d/turns (score beats energy)",
				result.Winner, result.Reason, p0.ID)
		}
	})

	t.Run("equal score: higher energy wins even with fewer bots", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		p1 := gs.AddPlayer()
		// p0 holds the larger army; p1 only the larger bank. If bots were
		// consulted before energy, p0 would win — the doc says energy first.
		gs.SpawnBot(p0.ID, Position{10, 10})
		gs.SpawnBot(p0.ID, Position{10, 12})
		gs.SpawnBot(p1.ID, Position{5, 5})

		gs.Players[p0.ID].Score = 3
		gs.Players[p1.ID].Score = 3
		gs.Players[p0.ID].Energy = 2
		gs.Players[p1.ID].Energy = 5
		gs.Turn = gs.Config.MaxTurns

		result := gs.checkWinConditions()

		if result == nil {
			t.Fatal("expected a result at the turn limit")
		}
		if result.Winner != p1.ID || result.Reason != "turns" {
			t.Errorf("winner/reason = %d/%s, want %d/turns (energy breaks score ties)",
				result.Winner, result.Reason, p1.ID)
		}
	})

	t.Run("equal score and energy: more bots alive wins", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		p1 := gs.AddPlayer()
		gs.SpawnBot(p0.ID, Position{10, 10})
		gs.SpawnBot(p0.ID, Position{10, 12})
		gs.SpawnBot(p1.ID, Position{5, 5})

		gs.Players[p0.ID].Score = 2
		gs.Players[p1.ID].Score = 2
		gs.Players[p0.ID].Energy = 4
		gs.Players[p1.ID].Energy = 4
		gs.Turn = gs.Config.MaxTurns

		result := gs.checkWinConditions()

		if result == nil {
			t.Fatal("expected a result at the turn limit")
		}
		if result.Winner != p0.ID || result.Reason != "turns" {
			t.Errorf("winner/reason = %d/%s, want %d/turns (bots alive break remaining ties)",
				result.Winner, result.Reason, p0.ID)
		}
	})
}

// TestPhaseConformance_CollectionAdjacencyGeometry pins the documented
// collection reach (plan §3.3: "A bot adjacent to (or on) an energy tile
// collects it"): the node's own tile and the diagonal neighbor (the dist²=2
// boundary) both collect, while a bot two tiles away (dist²=4) does not.
func TestPhaseConformance_CollectionAdjacencyGeometry(t *testing.T) {
	t.Run("standing on the node collects", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		en := gs.AddEnergyNode(Position{10, 10})
		en.HasEnergy = true
		gs.SpawnBot(p0.ID, Position{10, 10})

		gs.executeCollection()

		if got := gs.Players[p0.ID].Energy; got != 1 {
			t.Errorf("player energy = %d, want 1 (bot on the node collects)", got)
		}
		if en.HasEnergy {
			t.Error("node should be drained after on-tile collection")
		}
	})

	t.Run("diagonal neighbor at dist2 boundary collects", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		en := gs.AddEnergyNode(Position{10, 10})
		en.HasEnergy = true
		gs.SpawnBot(p0.ID, Position{11, 11}) // dist² = 1 + 1 = 2

		gs.executeCollection()

		if got := gs.Players[p0.ID].Energy; got != 1 {
			t.Errorf("player energy = %d, want 1 (dist²=2 diagonal is adjacent)", got)
		}
		if en.HasEnergy {
			t.Error("node should be drained after diagonal collection")
		}
	})

	t.Run("two tiles away does not collect", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		en := gs.AddEnergyNode(Position{10, 10})
		en.HasEnergy = true
		gs.SpawnBot(p0.ID, Position{10, 12}) // dist² = 4, beyond adjacency

		gs.executeCollection()

		if got := gs.Players[p0.ID].Energy; got != 0 {
			t.Errorf("player energy = %d, want 0 (dist²=4 is not adjacent)", got)
		}
		if !en.HasEnergy {
			t.Error("out-of-reach node must keep its energy")
		}
	})
}

// TestPhaseConformance_SameOwnerAdjacencyIsNotContested pins that contesting
// requires bots "from multiple players" (plan §3.3): two bots of the same
// owner around one node collect it once, without destroying the energy or
// double-crediting the player.
func TestPhaseConformance_SameOwnerAdjacencyIsNotContested(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer() // exists but keeps every bot far from the node
	gs.SpawnBot(p1.ID, Position{18, 18})

	en := gs.AddEnergyNode(Position{10, 10})
	en.HasEnergy = true
	gs.SpawnBot(p0.ID, Position{10, 11})
	gs.SpawnBot(p0.ID, Position{9, 10})

	gs.executeCollection()

	if got := gs.Players[p0.ID].Energy; got != 1 {
		t.Errorf("player energy = %d, want 1 (same-owner pair collects exactly once)", got)
	}
	if en.HasEnergy {
		t.Error("node should be drained after uncontested same-owner collection")
	}
	if got := gs.Players[p1.ID].Energy; got != 0 {
		t.Errorf("absent player energy = %d, want 0", got)
	}
}

// TestPhaseConformance_EnemyAdjacentToUndefendedCoreDoesNotRaze pins that
// capture requires occupation (README Capture: "Enemy units on undefended
// Core tiles raze them"; plan §3.7: "enemy bots on undefended cores raze
// them"): an enemy standing next to a core with no defender anywhere leaves
// it standing, with no capture event and no score movement.
func TestPhaseConformance_EnemyAdjacentToUndefendedCoreDoesNotRaze(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	core := gs.AddCore(p0.ID, Position{10, 10}) // p0 score 1, no defender bots
	gs.SpawnBot(p1.ID, Position{10, 11})        // adjacent, but not on the core

	gs.executeCaptures()

	if !core.Active {
		t.Error("adjacency must not raze an undefended core — occupation is required")
	}
	if got := gs.Players[p0.ID].Score; got != 1 {
		t.Errorf("owner score = %d, want 1 (no capture penalty)", got)
	}
	if got := gs.Players[p1.ID].Score; got != 0 {
		t.Errorf("adjacent enemy score = %d, want 0 (no capture award)", got)
	}
	for i := range gs.Events {
		if gs.Events[i].Type == EventCoreCaptured {
			t.Error("no core_captured event should be emitted without occupation")
		}
	}
}

// TestPhaseConformance_WrappedWallMoveStaysInPlace pins the composite of the
// two documented movement rules (plan §3.1 toroidal grid, §3.2 "A bot
// ordered into a wall tile stays in place"): a wall lookup itself wraps, so
// moving off one edge into a wall parked at the wrapped destination is
// blocked and the bot holds its position.
func TestPhaseConformance_WrappedWallMoveStaysInPlace(t *testing.T) {
	t.Run("control: unwrapped seam move arrives", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		bot := gs.SpawnBot(p0.ID, Position{0, 10})
		gs.SubmitMove(bot.Position, DirN) // wraps to {19,10}

		gs.executeMoves()

		if bot.Position != (Position{19, 10}) {
			t.Errorf("bot position = %v, want {19,10} (fixture control: the seam move itself works)", bot.Position)
		}
		if !bot.Alive {
			t.Error("a plain seam move must not kill the bot")
		}
	})

	t.Run("wall at the wrapped destination blocks", func(t *testing.T) {
		gs := newTestGameState()
		p0 := gs.AddPlayer()
		gs.Grid.Set(19, 10, TileWall) // the tile DirN from {0,10} wraps into
		bot := gs.SpawnBot(p0.ID, Position{0, 10})
		gs.SubmitMove(bot.Position, DirN)

		gs.executeMoves()

		if bot.Position != (Position{0, 10}) {
			t.Errorf("bot position = %v, want {0,10} (order ignored: wrapped destination is a wall)", bot.Position)
		}
		if !bot.Alive {
			t.Error("a wall-blocked order must not kill the bot")
		}
	})
}

// TestPhaseConformance_DominanceBoundaryAtExactlyEightyPercent pins the
// documented dominance threshold (plan §3.6: "controls >=80% of all bots for
// 100 consecutive turns"): four of five bots is exactly 80% and must count,
// reaching the win on the 100th consecutive check and not before.
func TestPhaseConformance_DominanceBoundaryAtExactlyEightyPercent(t *testing.T) {
	gs := newTestGameState()
	p0 := gs.AddPlayer()
	p1 := gs.AddPlayer()

	for i := 0; i < 4; i++ {
		gs.SpawnBot(p0.ID, Position{Row: i, Col: 0})
	}
	gs.SpawnBot(p1.ID, Position{15, 15}) // 4/5 = exactly 80%

	// Nudge energy each check so the stalemate counter (which shares the
	// endgame phase and counts unchanged-energy turns) stays at zero and the
	// streak being counted is dominance's alone.
	for i := 0; i < 99; i++ {
		gs.Players[p1.ID].Energy = i
		if result := gs.checkWinConditions(); result != nil {
			t.Fatalf("check %d produced a %q result before the 100th consecutive dominance turn",
				i+1, result.Reason)
		}
	}

	result := gs.checkWinConditions()

	if result == nil {
		t.Fatal("expected a dominance win on the 100th consecutive check at exactly 80%")
	}
	if result.Winner != p0.ID || result.Reason != "dominance" {
		t.Errorf("winner/reason = %d/%s, want %d/dominance (>=80%% includes exactly 80%%)",
			result.Winner, result.Reason, p0.ID)
	}
}

// TestPhaseConformance_EnergyTickDocumentedCadence pins the passive energy
// tick (plan §3.3: "Energy appears on a node every energy_interval turns
// (default: 10)"; README phase "Energy Tick") at the documented default
// interval: charged nodes pass through the tick untouched, and an empty node
// counts one tick per turn, regenerating exactly when the counter reaches the
// interval.
func TestPhaseConformance_EnergyTickDocumentedCadence(t *testing.T) {
	if got := DefaultConfig().EnergyInterval; got != 10 {
		t.Fatalf("DefaultConfig().EnergyInterval = %d, want the documented default 10", got)
	}

	gs := newTestGameState() // keeps DefaultConfig's interval of 10
	charged := gs.AddEnergyNode(Position{10, 10})
	charged.HasEnergy = true
	counting := gs.AddEnergyNode(Position{10, 14})
	counting.Tick = 8

	gs.executeEnergyTick()

	if !charged.HasEnergy || charged.Tick != 0 {
		t.Errorf("charged node = hasEnergy %v tick %d, want true/0 (tick must not touch charged nodes)",
			charged.HasEnergy, charged.Tick)
	}
	if counting.HasEnergy || counting.Tick != 9 {
		t.Errorf("counting node = hasEnergy %v tick %d, want false/9 (one tick per turn, no early regen)",
			counting.HasEnergy, counting.Tick)
	}

	gs.executeEnergyTick()

	if !counting.HasEnergy || counting.Tick != 0 {
		t.Errorf("counting node = hasEnergy %v tick %d, want true/0 (regenerates exactly at the interval)",
			counting.HasEnergy, counting.Tick)
	}
}
