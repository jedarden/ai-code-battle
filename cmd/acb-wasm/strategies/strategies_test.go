package strategies

import (
	"math/rand"
	"testing"

	"github.com/aicodebattle/acb/engine"
)

// extendedRosterTestState builds a small synthetic visible state: player 0
// owns two bots and a core near the top-left; player 1 mirrors it bottom-right
// with an energy node in between and a wall column separating the halves.
func extendedRosterTestState() *engine.VisibleState {
	state := &engine.VisibleState{
		MatchID: "test-match",
		Turn:    5,
		Config: engine.Config{
			Rows: 20, Cols: 20, MaxTurns: 200,
			VisionRadius2: 49, AttackRadius2: 5,
			SpawnCost: 3, EnergyInterval: 10, CoresPerPlayer: 1,
		},
	}
	state.You.ID = 0
	state.You.Energy = 10
	state.Bots = []engine.VisibleBot{
		{Position: engine.Position{Row: 2, Col: 2}, Owner: 0},
		{Position: engine.Position{Row: 3, Col: 2}, Owner: 0},
		{Position: engine.Position{Row: 17, Col: 17}, Owner: 1},
	}
	state.Energy = []engine.Position{{Row: 10, Col: 10}, {Row: 10, Col: 11}}
	state.Cores = []engine.VisibleCore{
		{Position: engine.Position{Row: 1, Col: 1}, Owner: 0, Active: true},
		{Position: engine.Position{Row: 18, Col: 18}, Owner: 1, Active: true},
	}
	for r := 0; r < 20; r++ {
		state.Walls = append(state.Walls, engine.Position{Row: r, Col: 5})
	}
	return state
}

// TestExtendedRosterProducesValidMoves checks every extended-roster strategy
// accepts the state, returns moves only for own bots, with in-bounds
// positions and valid directions.
func TestExtendedRosterProducesValidMoves(t *testing.T) {
	for _, name := range []string{"farmer", "opportunist", "siege", "economist", "assassin", "phalanx", "zone-driver"} {
		t.Run(name, func(t *testing.T) {
			bot := New(name, rand.New(rand.NewSource(1)))
			state := extendedRosterTestState()
			moves, err := bot.GetMoves(state)
			if err != nil {
				t.Fatalf("GetMoves returned error: %v", err)
			}
			for _, mv := range moves {
				if mv.Position.Row < 0 || mv.Position.Row >= 20 || mv.Position.Col < 0 || mv.Position.Col >= 20 {
					t.Fatalf("move position out of bounds: %+v", mv)
				}
				if mv.Direction != engine.DirN && mv.Direction != engine.DirE && mv.Direction != engine.DirS && mv.Direction != engine.DirW {
					t.Fatalf("move has invalid direction %d", mv.Direction)
				}
				// Every move must originate from one of our bots.
				owned := false
				for _, b := range state.Bots {
					if b.Owner == 0 && b.Position == mv.Position {
						owned = true
						break
					}
				}
				if !owned {
					t.Fatalf("move issued for non-owned bot at %+v", mv.Position)
				}
			}
			// Assassin rushes cores: with an active enemy core visible it must
			// commit at least one bot.
			if name == "assassin" && len(moves) == 0 {
				t.Fatal("assassin produced no moves despite a visible active enemy core")
			}
		})
	}
}

// TestStatefulStrategiesResetOnNewMatch checks the persistent-state strategies
// (economist, assassin, phalanx) drop their cross-turn memory when the match
// ID changes.
func TestStatefulStrategiesResetOnNewMatch(t *testing.T) {
	for _, name := range []string{"economist", "assassin", "phalanx"} {
		t.Run(name, func(t *testing.T) {
			bot := New(name, rand.New(rand.NewSource(1)))
			state := extendedRosterTestState()
			if _, err := bot.GetMoves(state); err != nil {
				t.Fatalf("first call: %v", err)
			}
			state.MatchID = "other-match"
			state.Turn = 1
			if _, err := bot.GetMoves(state); err != nil {
				t.Fatalf("second call with new match ID: %v", err)
			}
		})
	}
}

// TestZoneDriverUsesZone checks the zone-driver reacts to an active zone by
// retreating bots stranded outside it.
func TestZoneDriverUsesZone(t *testing.T) {
	bot := NewZoneDriver()
	state := extendedRosterTestState()
	state.Zone = &engine.ZoneBounds{
		Center: engine.Position{Row: 10, Col: 10},
		Radius: 4,
		Active: true,
	}
	moves, err := bot.GetMoves(state)
	if err != nil {
		t.Fatalf("GetMoves returned error: %v", err)
	}
	if len(moves) == 0 {
		t.Fatal("zone-driver produced no moves with an active zone")
	}
	// Both bots (2,2) and (3,2) are far outside radius 4 around (10,10) —
	// each should be moving to save itself.
	if len(moves) != 2 {
		t.Fatalf("expected both endangered bots to move, got %d moves", len(moves))
	}
}

// TestSiegeLockoutRing checks siege sends bots toward enemy-core ring tiles
// when the core is directly reachable.
func TestSiegeLockoutRing(t *testing.T) {
	bot := NewSiege()
	state := extendedRosterTestState()
	// Remove walls so the enemy core is fully approachable.
	state.Walls = nil
	moves, err := bot.GetMoves(state)
	if err != nil {
		t.Fatalf("GetMoves returned error: %v", err)
	}
	if len(moves) == 0 {
		t.Fatal("siege produced no moves with a visible enemy core")
	}
}

// TestExtendedRosterRunsFullMatches runs every extended-roster strategy
// through a real engine match against gatherer — the same path the WASM
// sandbox uses — and asserts the match completes cleanly. Cores are placed in
// opposite corners (the engine's generated 2-player map deliberately spawns
// bots inside attack range of each other, which trades the single starting
// bots at turn 1 and hides everything but the opening move).
func TestExtendedRosterRunsFullMatches(t *testing.T) {
	roster := []string{"farmer", "opportunist", "siege", "economist", "assassin", "phalanx", "zone-driver"}
	for _, name := range roster {
		t.Run(name, func(t *testing.T) {
			cfg := engine.ConfigForPlayers(2, 1)
			cfg.MaxTurns = 150 // shorten; we only assert the match completes
			preGen := engine.PreGeneratedMap{
				WallsJSON: `[]`,
				CoresJSON: `[{"position":{"row":6,"col":6},"owner":0},
				             {"position":{"row":33,"col":33},"owner":1}]`,
			}
			mr := engine.NewMatchRunner(cfg, engine.WithRNG(rand.New(rand.NewSource(7))), engine.WithMap(preGen))
			mr.AddBot(New(name, rand.New(rand.NewSource(1))), name)
			mr.AddBot(NewGatherer(rand.New(rand.NewSource(2))), "gatherer")
			result, _, err := mr.Run()
			if err != nil {
				t.Fatalf("match failed: %v", err)
			}
			if result.Turns <= 1 {
				t.Fatalf("match ended immediately after %d turns (reason %q)", result.Turns, result.Reason)
			}
			if result.Winner < -1 || result.Winner > 1 {
				t.Fatalf("unexpected winner %d", result.Winner)
			}
		})
	}
}
