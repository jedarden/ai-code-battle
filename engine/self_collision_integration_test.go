package engine

import (
	"math/rand"
	"testing"
)

func newSelfCollisionTestState() *GameState {
	config := DefaultConfig()
	config.Rows = 8
	config.Cols = 8
	config.MaxTurns = 100
	config.ZoneEnabled = false
	config.SpawnCost = 100
	config.EnergyInterval = 100

	gs := NewGameState(config, rand.New(rand.NewSource(7)))
	gs.AddPlayer()
	gs.AddPlayer()
	return gs
}

func countEventType(gs *GameState, eventType string) int {
	count := 0
	for _, event := range gs.Events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func countSelfCollisionDeaths(gs *GameState) int {
	count := 0
	for _, event := range gs.Events {
		if event.Type != EventBotDied {
			continue
		}
		details, ok := event.Details.(map[string]interface{})
		if ok && details["reason"] == "self_collision" {
			count++
		}
	}
	return count
}

func TestSelfCollisionKillsAllUnitsAtSharedDestination(t *testing.T) {
	gs := newSelfCollisionTestState()
	p0 := gs.Players[0]
	p1 := gs.Players[1]

	units := []*Bot{
		gs.SpawnBot(p0.ID, Position{2, 1}),
		gs.SpawnBot(p0.ID, Position{2, 3}),
		gs.SpawnBot(p0.ID, Position{1, 2}),
	}
	gs.SpawnBot(p1.ID, Position{7, 7})
	gs.ClearTurnState()

	gs.SubmitMove(units[0].Position, DirE)
	gs.SubmitMove(units[1].Position, DirW)
	gs.SubmitMove(units[2].Position, DirS)
	gs.ExecuteTurn()

	for i, unit := range units {
		if unit.Alive {
			t.Errorf("unit %d survived a same-owner shared destination", i)
		}
	}
	if got := countSelfCollisionDeaths(gs); got != len(units) {
		t.Errorf("self-collision death events = %d, want %d", got, len(units))
	}
	if got := countEventType(gs, EventCombatDeath); got != 0 {
		t.Errorf("combat death events = %d, want 0", got)
	}
	if got := gs.Players[p0.ID].BotCount; got != 0 {
		t.Errorf("player %d living bot count = %d, want 0", p0.ID, got)
	}
}

func TestSelfCollisionLeavesDistinctDestinationsAlive(t *testing.T) {
	gs := newSelfCollisionTestState()
	p0 := gs.Players[0]
	p1 := gs.Players[1]

	first := gs.SpawnBot(p0.ID, Position{2, 1})
	second := gs.SpawnBot(p0.ID, Position{4, 4})
	gs.SpawnBot(p1.ID, Position{7, 7})
	gs.ClearTurnState()

	gs.SubmitMove(first.Position, DirE)
	gs.SubmitMove(second.Position, DirE)
	gs.ExecuteTurn()

	if !first.Alive || !second.Alive {
		t.Fatalf("distinct destinations killed friendly units: first=%v, second=%v", first.Alive, second.Alive)
	}
	if first.Position != (Position{2, 2}) {
		t.Errorf("first position = %v, want {2,2}", first.Position)
	}
	if second.Position != (Position{4, 5}) {
		t.Errorf("second position = %v, want {4,5}", second.Position)
	}
	if got := countSelfCollisionDeaths(gs); got != 0 {
		t.Errorf("self-collision death events = %d, want 0", got)
	}
}

func TestSelfCollisionUsesWrappedDestination(t *testing.T) {
	gs := newSelfCollisionTestState()
	p0 := gs.Players[0]
	p1 := gs.Players[1]

	wrappingUnit := gs.SpawnBot(p0.ID, Position{7, 0})
	otherUnit := gs.SpawnBot(p0.ID, Position{0, 1})
	nonCollidingUnit := gs.SpawnBot(p0.ID, Position{7, 1})
	gs.SpawnBot(p1.ID, Position{7, 7})
	gs.ClearTurnState()

	gs.SubmitMove(wrappingUnit.Position, DirS)
	gs.SubmitMove(otherUnit.Position, DirW)
	gs.SubmitMove(nonCollidingUnit.Position, DirS)
	gs.ExecuteTurn()

	if wrappingUnit.Alive || otherUnit.Alive {
		t.Fatalf("wrapped shared destination did not kill both units: wrapping=%v, other=%v", wrappingUnit.Alive, otherUnit.Alive)
	}
	if !nonCollidingUnit.Alive {
		t.Error("unit with a distinct wrapped destination died")
	}
	if nonCollidingUnit.Position != (Position{0, 1}) {
		t.Errorf("non-colliding wrapped position = %v, want {0,1}", nonCollidingUnit.Position)
	}
	if got := countSelfCollisionDeaths(gs); got != 2 {
		t.Errorf("self-collision death events = %d, want 2", got)
	}
}

func TestMixedOwnerSharedDestinationUsesCombatPhase(t *testing.T) {
	gs := newSelfCollisionTestState()
	p0 := gs.Players[0]
	p1 := gs.Players[1]

	first := gs.SpawnBot(p0.ID, Position{2, 1})
	second := gs.SpawnBot(p0.ID, Position{2, 3})
	enemy := gs.SpawnBot(p1.ID, Position{1, 2})
	gs.SpawnBot(p1.ID, Position{0, 7})
	gs.ClearTurnState()

	gs.SubmitMove(first.Position, DirE)
	gs.SubmitMove(second.Position, DirW)
	gs.SubmitMove(enemy.Position, DirS)
	gs.ExecuteTurn()

	if !first.Alive || !second.Alive {
		t.Fatalf("friendly units in a mixed-owner collision died before combat: first=%v, second=%v", first.Alive, second.Alive)
	}
	if enemy.Alive {
		t.Error("outnumbered enemy survived the mixed-owner collision")
	}
	if first.Position != (Position{2, 2}) || second.Position != (Position{2, 2}) {
		t.Errorf("friendly positions = %v, %v, want both {2,2}", first.Position, second.Position)
	}
	if got := countSelfCollisionDeaths(gs); got != 0 {
		t.Errorf("self-collision death events = %d, want 0", got)
	}
	if got := countEventType(gs, EventCombatDeath); got != 1 {
		t.Errorf("combat death events = %d, want 1", got)
	}
}

func TestSelfCollisionResolvesBeforeCombat(t *testing.T) {
	gs := newSelfCollisionTestState()
	gs.Config.AttackRadius2 = 1
	p0 := gs.Players[0]
	p1 := gs.Players[1]

	collidingFirst := gs.SpawnBot(p0.ID, Position{2, 1})
	collidingSecond := gs.SpawnBot(p0.ID, Position{2, 3})
	combatFirst := gs.SpawnBot(p0.ID, Position{6, 2})
	combatSecond := gs.SpawnBot(p0.ID, Position{6, 4})
	enemy := gs.SpawnBot(p1.ID, Position{6, 3})
	gs.SpawnBot(p1.ID, Position{0, 7})
	gs.ClearTurnState()

	gs.SubmitMove(collidingFirst.Position, DirE)
	gs.SubmitMove(collidingSecond.Position, DirW)
	gs.ExecuteTurn()

	if collidingFirst.Alive || collidingSecond.Alive {
		t.Fatalf("same-owner collision was not resolved before combat: first=%v, second=%v", collidingFirst.Alive, collidingSecond.Alive)
	}
	if !combatFirst.Alive || !combatSecond.Alive {
		t.Fatalf("surviving friendly units died during combat: first=%v, second=%v", combatFirst.Alive, combatSecond.Alive)
	}
	if enemy.Alive {
		t.Error("outnumbered enemy survived the combat phase")
	}
	if got := countSelfCollisionDeaths(gs); got != 2 {
		t.Errorf("self-collision death events = %d, want 2", got)
	}
	if got := countEventType(gs, EventCombatDeath); got != 1 {
		t.Errorf("combat death events = %d, want 1", got)
	}
}

func TestSelfCollisionRemovesDefenderBeforeCapture(t *testing.T) {
	gs := newSelfCollisionTestState()
	p0 := gs.Players[0]
	p1 := gs.Players[1]

	core := gs.AddCore(p0.ID, Position{4, 4})
	firstDefender := gs.SpawnBot(p0.ID, Position{3, 4})
	secondDefender := gs.SpawnBot(p0.ID, Position{5, 4})
	attacker := gs.SpawnBot(p1.ID, Position{4, 5})
	gs.SpawnBot(p0.ID, Position{0, 0})
	gs.ClearTurnState()

	gs.SubmitMove(firstDefender.Position, DirS)
	gs.SubmitMove(secondDefender.Position, DirN)
	gs.ExecuteTurn()

	if firstDefender.Alive || secondDefender.Alive {
		t.Fatalf("self-colliding defenders survived: first=%v, second=%v", firstDefender.Alive, secondDefender.Alive)
	}
	if !core.Active {
		t.Fatal("core was captured without an enemy occupying it")
	}
	if got := countEventType(gs, EventCoreCaptured); got != 0 {
		t.Errorf("core capture events = %d, want 0", got)
	}

	gs.ClearTurnState()
	gs.SubmitMove(attacker.Position, DirW)
	gs.ExecuteTurn()

	if !attacker.Alive {
		t.Fatal("attacker died before the capture phase")
	}
	if core.Active {
		t.Fatal("core remained active after the undefended capture")
	}
	if got := countEventType(gs, EventCoreCaptured); got != 1 {
		t.Errorf("core capture events = %d, want 1", got)
	}
}
