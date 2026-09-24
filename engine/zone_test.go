package engine

import (
	"math/rand"
	"testing"
)

// This file pins the zone (storm) contract end to end:
//   - activation happens on the turn matching ZoneStartTurn, before moves are
//     collected, and the radius snaps to setInitialZoneRadius at that moment
//   - the first shrink is at ZoneStartTurn+ZoneShrinkInterval (never on the
//     activation turn itself), then one step every ZoneShrinkInterval turns
//   - shrink steps clamp at ZoneMinRadius and never go below it
//   - damage: when the zone is enabled and active, a living bot dies iff its
//     toroidal distance² from ZoneCenter is strictly greater than
//     ZoneRadius²; zone kills emit one zone_death each, decrement the owner's
//     bot count, and award no score to anyone
//   - toroidal boundary behavior: the kill check judges the wrapped position,
//     so a bot that moves across the map seam is judged at where it wrapped to

// newZoneTestStateSized builds a two-player state on a rows×cols grid with the
// zone fields set explicitly, ready for state-level ExecuteTurn driving.
// Callers position bots themselves (SpawnBot) and set ZoneActive/ZoneRadius to
// simulate match.go's pre-move activation.
func newZoneTestStateSized(rows, cols, startTurn, interval, step, minRadius int) *GameState {
	cfg := DefaultConfig()
	cfg.Rows = rows
	cfg.Cols = cols
	cfg.ZoneEnabled = true
	cfg.ZoneStartTurn = startTurn
	cfg.ZoneShrinkInterval = interval
	cfg.ZoneShrinkStep = step
	cfg.ZoneMinRadius = minRadius
	gs := NewGameState(cfg, rand.New(rand.NewSource(1)))
	gs.AddPlayer()
	gs.AddPlayer()
	return gs
}

// newZoneTestState builds the standard 20×20 zone test state.
func newZoneTestState(startTurn, interval, step, minRadius int) *GameState {
	return newZoneTestStateSized(20, 20, startTurn, interval, step, minRadius)
}

// findZoneDeaths returns the zone_death events recorded on the state.
func findZoneDeaths(gs *GameState) []Event {
	var deaths []Event
	for _, e := range gs.Events {
		if e.Type == EventZoneDeath {
			deaths = append(deaths, e)
		}
	}
	return deaths
}

// zoneDeathDetails extracts the details map from a zone_death event.
func zoneDeathDetails(t *testing.T, e Event) map[string]interface{} {
	t.Helper()
	m, ok := e.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("zone_death details = %#v, want map[string]interface{}", e.Details)
	}
	return m
}

// TestZoneActivationAndShrinkTimingInMatch drives a full MatchRunner match
// with idle bots and asserts the per-turn zone bounds recorded in the replay:
// inactive before ZoneStartTurn at the pre-activation radius, active from
// ZoneStartTurn with the snapped initial radius, first shrink at
// ZoneStartTurn+ZoneShrinkInterval, then one step every interval.
func TestZoneActivationAndShrinkTimingInMatch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Rows = 40
	cfg.Cols = 40
	cfg.MaxTurns = 14
	cfg.ZoneEnabled = true
	cfg.ZoneStartTurn = 5
	cfg.ZoneShrinkInterval = 2
	cfg.ZoneShrinkStep = 1
	cfg.ZoneMinRadius = 1

	mr := NewMatchRunner(cfg, WithRNG(rand.New(rand.NewSource(7))))
	mr.AddBot(NewIdleBot(), "idle-0")
	mr.AddBot(NewIdleBot(), "idle-1")

	result, replay, err := mr.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatalf("Run() returned nil result for a turn-limited match")
	}
	if len(replay.Turns) != cfg.MaxTurns+1 {
		t.Fatalf("replay has %d turns, want %d (turn 0 + %d executed)", len(replay.Turns), cfg.MaxTurns+1, cfg.MaxTurns)
	}

	// NewGameState radius = min(Rows,Cols)/2 = 20; on activation (turn 5)
	// setInitialZoneRadius snaps it to 20*90/100 = 18; shrink step 1 every 2
	// turns from turn 7: 7->17, 9->16, 11->15, 13->14.
	wantRadius := map[int]int{
		0: 20, 1: 20, 2: 20, 3: 20, 4: 20,
		5: 18, 6: 18,
		7: 17, 8: 17,
		9: 16, 10: 16,
		11: 15, 12: 15,
		13: 14, 14: 14,
	}
	center := Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2}
	for _, turn := range replay.Turns {
		if turn.ZoneBounds == nil {
			t.Fatalf("turn %d: ZoneBounds = nil, want recorded bounds when zone is enabled", turn.Turn)
		}
		zb := turn.ZoneBounds
		wantActive := turn.Turn >= cfg.ZoneStartTurn
		if zb.Active != wantActive {
			t.Errorf("turn %d: ZoneBounds.Active = %v, want %v", turn.Turn, zb.Active, wantActive)
		}
		if want, ok := wantRadius[turn.Turn]; ok && zb.Radius != want {
			t.Errorf("turn %d: ZoneBounds.Radius = %d, want %d", turn.Turn, zb.Radius, want)
		}
		if zb.Center != center {
			t.Errorf("turn %d: ZoneBounds.Center = %v, want fixed map center %v", turn.Turn, zb.Center, center)
		}
	}

	// The activation turn itself must not shrink: turn 5 carries the snapped
	// initial radius, turn 6 is unchanged, turn 7 is the first shrink.
	if got := replay.Turns[5].ZoneBounds.Radius; got != 18 {
		t.Errorf("activation turn 5: radius = %d, want 18 (initial radius, no shrink on activation turn)", got)
	}
	if got := replay.Turns[7].ZoneBounds.Radius; got != 17 {
		t.Errorf("turn 7: radius = %d, want 17 (first shrink at ZoneStartTurn+ZoneShrinkInterval)", got)
	}
}

// TestZoneShrinkCadence pins the shrink schedule at the state level with
// interval 3 and step 2: no shrink before or on the start turn, then exactly
// one step on every turn where (Turn-ZoneStartTurn) % interval == 0.
func TestZoneShrinkCadence(t *testing.T) {
	const (
		startTurn = 3
		interval  = 3
		step      = 2
		minRadius = 1
	)
	gs := newZoneTestState(startTurn, interval, step, minRadius)
	gs.ZoneActive = true
	gs.ZoneRadius = 20

	// Both bots stay well inside the smallest radius used (14) and far
	// enough apart that combat never interferes.
	p0 := gs.Players[0]
	p1 := gs.Players[1]
	gs.SpawnBot(p0.ID, Position{Row: 10, Col: 6})  // d2 from (10,10) = 16
	gs.SpawnBot(p1.ID, Position{Row: 10, Col: 14}) // d2 = 16, mutual d2 = 64

	// Radius expected after executing turn N.
	wantRadius := map[int]int{
		1: 20, 2: 20, 3: 20, // activation turn (3) must not shrink
		4: 20, 5: 20,
		6: 18, 7: 18, 8: 18,
		9: 16, 10: 16, 11: 16,
		12: 14,
	}

	for turn := 1; turn <= 12; turn++ {
		gs.ExecuteTurn()
		if gs.ZoneRadius != wantRadius[turn] {
			t.Fatalf("after turn %d: ZoneRadius = %d, want %d", turn, gs.ZoneRadius, wantRadius[turn])
		}
	}
}

// TestZoneMinimumRadiusEnforcement pins the clamp: a shrink step that would
// overshoot the minimum lands exactly on it, and once at the minimum the
// radius stays there no matter how many more shrink turns pass.
func TestZoneMinimumRadiusEnforcement(t *testing.T) {
	t.Run("overshoot clamps to minimum", func(t *testing.T) {
		const (
			startTurn = 1
			interval  = 1
			step      = 3
			minRadius = 4
		)
		gs := newZoneTestState(startTurn, interval, step, minRadius)
		gs.ZoneActive = true
		gs.ZoneRadius = 5

		// Both bots survive at radius 4: one at the center, one exactly on
		// the boundary (d2 = 16 = radius²), out of each other's attack range.
		p0 := gs.Players[0]
		p1 := gs.Players[1]
		boundaryBot := gs.SpawnBot(p0.ID, Position{Row: 10, Col: 14})
		gs.SpawnBot(p1.ID, Position{Row: 10, Col: 10})

		for turn := 1; turn <= 30; turn++ {
			gs.ExecuteTurn()
			want := 5
			if turn >= 2 {
				want = 4 // turn 2: 5-3 = 2 would overshoot -> clamped to 4
			}
			if gs.ZoneRadius != want {
				t.Fatalf("after turn %d: ZoneRadius = %d, want %d", turn, gs.ZoneRadius, want)
			}
			if !boundaryBot.Alive {
				t.Fatalf("turn %d: boundary bot died, zone kill must require dist2 > radius²", turn)
			}
		}
	})

	t.Run("never shrinks below minimum", func(t *testing.T) {
		const (
			startTurn = 1
			interval  = 1
			step      = 5
			minRadius = 4
		)
		gs := newZoneTestState(startTurn, interval, step, minRadius)
		gs.ZoneActive = true
		gs.ZoneRadius = 4 // already at the minimum

		p0 := gs.Players[0]
		p1 := gs.Players[1]
		gs.SpawnBot(p0.ID, Position{Row: 10, Col: 10})
		gs.SpawnBot(p1.ID, Position{Row: 10, Col: 14})

		for turn := 1; turn <= 10; turn++ {
			gs.ExecuteTurn()
			if gs.ZoneRadius != minRadius {
				t.Fatalf("after turn %d: ZoneRadius = %d, want %d (must never go below minimum)", turn, gs.ZoneRadius, minRadius)
			}
		}
	})
}

// TestZoneDamageOutsideSafeZone pins who the zone hurts: with the zone
// enabled and active, a living bot dies iff its toroidal distance² from the
// zone center is strictly greater than radius². Boundary bots survive, zone
// kills emit exactly one zone_death each, decrement the owner's bot count,
// and award no score to anyone.
func TestZoneDamageOutsideSafeZone(t *testing.T) {
	const zoneRadius = 4 // center (10,10) -> boundary at dist² = 16

	cases := []struct {
		name    string
		pos     Position
		enabled bool
		active  bool
		wantDry bool // true = nobody dies
	}{
		{name: "bot inside survives", pos: Position{Row: 10, Col: 8}, enabled: true, active: true, wantDry: true},              // d2 = 4
		{name: "bot exactly on boundary survives", pos: Position{Row: 10, Col: 6}, enabled: true, active: true, wantDry: true}, // d2 = 16 == r²
		{name: "bot outside dies", pos: Position{Row: 10, Col: 3}, enabled: true, active: true, wantDry: false},                // d2 = 49 > 16
		{name: "inactive zone does not damage", pos: Position{Row: 1, Col: 1}, enabled: true, active: false, wantDry: true},
		{name: "disabled zone does not damage", pos: Position{Row: 1, Col: 1}, enabled: false, active: false, wantDry: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			gs := newZoneTestState(1, 1000, 1, 1) // interval too large to shrink during the test
			gs.Config.ZoneEnabled = tt.enabled
			gs.ZoneActive = tt.active
			gs.ZoneRadius = zoneRadius

			p0 := gs.Players[0]
			p1 := gs.Players[1]
			subject := gs.SpawnBot(p0.ID, tt.pos)
			// Control bot for the second player, always safe on the boundary
			// and out of the subject's attack range.
			control := gs.SpawnBot(p1.ID, Position{Row: 10, Col: 14})

			gs.ExecuteTurn()

			if tt.wantDry {
				if !subject.Alive || !control.Alive {
					t.Fatalf("subject alive = %v, control alive = %v, want both alive", subject.Alive, control.Alive)
				}
				if deaths := findZoneDeaths(gs); len(deaths) != 0 {
					t.Fatalf("got %d zone_death events, want 0", len(deaths))
				}
				return
			}

			if subject.Alive {
				t.Fatalf("bot at %v (dist² %d) survived radius %d, want death",
					subject.Position, gs.Grid.Distance2(subject.Position, gs.ZoneCenter), zoneRadius)
			}
			if !control.Alive {
				t.Fatalf("control bot died, want alive")
			}
			if gs.Players[p0.ID].BotCount != 0 {
				t.Errorf("owner bot count = %d, want 0 after zone death", gs.Players[p0.ID].BotCount)
			}
			if len(gs.DeadBots) != 1 || gs.DeadBots[0] != subject {
				t.Errorf("DeadBots holds %d entries, want exactly the subject", len(gs.DeadBots))
			}
			// Zone kills are not combat kills: no score for anyone.
			for _, p := range gs.Players {
				if p.Score != 0 {
					t.Errorf("player %d score = %d, want 0 (zone kills award no score)", p.ID, p.Score)
				}
			}

			deaths := findZoneDeaths(gs)
			if len(deaths) != 1 {
				t.Fatalf("got %d zone_death events, want exactly 1", len(deaths))
			}
			death := deaths[0]
			if death.Turn != 1 {
				t.Errorf("zone_death turn = %d, want 1", death.Turn)
			}
			details := zoneDeathDetails(t, death)
			if id, ok := details["bot_id"].(int); !ok || id != subject.ID {
				t.Errorf("zone_death bot_id = %v, want %d", details["bot_id"], subject.ID)
			}
			if owner, ok := details["owner"].(int); !ok || owner != p0.ID {
				t.Errorf("zone_death owner = %v, want %d", details["owner"], p0.ID)
			}
			if pos, ok := details["position"].(Position); !ok || pos != subject.Position {
				t.Errorf("zone_death position = %v, want %v", details["position"], subject.Position)
			}
			for _, e := range gs.Events {
				if e.Type == EventCombatDeath {
					t.Errorf("unexpected combat_death event: %+v", e)
				}
			}
		})
	}
}

// TestZoneToroidalBoundary pins that the zone kill check judges bots at their
// wrapped (toroidal) position, not their raw Euclidean offset.
func TestZoneToroidalBoundary(t *testing.T) {
	t.Run("wrapped short path is inside the zone", func(t *testing.T) {
		// Off-center zone: only against a non-center point does the toroidal
		// short path actually differ from the raw offset.
		gs := newZoneTestStateSized(12, 12, 1, 1000, 1, 1)
		gs.ZoneCenter = Position{Row: 2, Col: 2}
		gs.ZoneActive = true
		gs.ZoneRadius = 5 // radius² = 25

		p0 := gs.Players[0]
		p1 := gs.Players[1]
		// Raw offset from (2,2) is (9,9) -> 162, but the toroidal short path
		// is (-3,-3) -> 18 <= 25: inside. A non-wrapping check would kill it.
		wrappedBot := gs.SpawnBot(p0.ID, Position{Row: 11, Col: 11})
		// Same wrap direction but beyond the wrapped radius: (6,6) -> 72 > 25.
		outsideBot := gs.SpawnBot(p1.ID, Position{Row: 8, Col: 8})
		// Control at the zone center itself.
		gs.SpawnBot(p1.ID, Position{Row: 2, Col: 2})

		gs.ExecuteTurn()

		if !wrappedBot.Alive {
			t.Fatalf("bot at (11,11) died: toroidal d2 = %d, want <= radius² %d (wrap must be honored)",
				gs.Grid.Distance2(wrappedBot.Position, gs.ZoneCenter), gs.ZoneRadius*gs.ZoneRadius)
		}
		if outsideBot.Alive {
			t.Fatalf("bot at (8,8) survived: toroidal d2 = %d, want > radius² %d",
				gs.Grid.Distance2(outsideBot.Position, gs.ZoneCenter), gs.ZoneRadius*gs.ZoneRadius)
		}
		deaths := findZoneDeaths(gs)
		if len(deaths) != 1 {
			t.Fatalf("got %d zone_death events, want exactly 1", len(deaths))
		}
		if id, _ := zoneDeathDetails(t, deaths[0])["bot_id"].(int); id != outsideBot.ID {
			t.Errorf("zone_death bot_id = %v, want %d", zoneDeathDetails(t, deaths[0])["bot_id"], outsideBot.ID)
		}
	})

	t.Run("bot crossing the seam is judged at its wrapped position", func(t *testing.T) {
		gs := newZoneTestState(1, 1000, 1, 1)
		gs.ZoneActive = true
		gs.ZoneRadius = 5 // radius² = 25, center (10,10)

		p0 := gs.Players[0]
		p1 := gs.Players[1]
		// (0,8) is outside (d2 = 104 > 25) and so is its wrapped destination
		// (19,8) (d2 = 85 > 25) — the bot dies either way, but only if the
		// zone phase runs after move execution and judges the wrapped tile
		// does the event record (19,8) rather than (0,8).
		seamBot := gs.SpawnBot(p0.ID, Position{Row: 0, Col: 8})
		gs.SpawnBot(p1.ID, Position{Row: 10, Col: 10}) // control at the center

		gs.SubmitMove(seamBot.Position, DirN) // (0,8) -> wraps to (19,8)
		gs.ExecuteTurn()

		if seamBot.Alive {
			t.Fatalf("seam bot survived at %v, want dead (d2 %d > radius² %d)",
				seamBot.Position, gs.Grid.Distance2(seamBot.Position, gs.ZoneCenter), gs.ZoneRadius*gs.ZoneRadius)
		}
		if seamBot.Position != (Position{Row: 19, Col: 8}) {
			t.Fatalf("seam bot position = %v, want wrapped (19,8)", seamBot.Position)
		}
		deaths := findZoneDeaths(gs)
		if len(deaths) != 1 {
			t.Fatalf("got %d zone_death events, want exactly 1", len(deaths))
		}
		if pos, ok := zoneDeathDetails(t, deaths[0])["position"].(Position); !ok || pos != (Position{Row: 19, Col: 8}) {
			t.Errorf("zone_death position = %v, want wrapped (19,8)", zoneDeathDetails(t, deaths[0])["position"])
		}
	})

	t.Run("corner is the farthest tile and covering it covers the torus", func(t *testing.T) {
		gs := newZoneTestState(1, 1000, 1, 1)
		gs.ZoneActive = true
		gs.ZoneRadius = 15 // radius² = 225

		p0 := gs.Players[0]
		p1 := gs.Players[1]
		// On an even grid with the center at (Rows/2, Cols/2) no offset ever
		// exceeds half the map, so the corner is the farthest tile from the
		// zone center: d2 = 10²+10² = 200. A radius whose square covers the
		// corner therefore keeps every wrapped position on the map safe too.
		cornerBot := gs.SpawnBot(p0.ID, Position{Row: 0, Col: 0})
		gs.SpawnBot(p1.ID, Position{Row: 10, Col: 10})

		gs.ExecuteTurn()

		if !cornerBot.Alive {
			t.Fatalf("corner bot died at radius %d, want alive (d2 = %d <= radius² = %d)",
				gs.ZoneRadius, gs.Grid.Distance2(cornerBot.Position, gs.ZoneCenter), gs.ZoneRadius*gs.ZoneRadius)
		}
		if deaths := findZoneDeaths(gs); len(deaths) != 0 {
			t.Fatalf("got %d zone_death events, want 0", len(deaths))
		}
	})
}

// TestZoneActivationTurnKillsWithoutShrinking pins the activation-turn
// semantics match.go produces: turns before ZoneStartTurn neither kill nor
// shrink; the activation turn itself kills bots already outside the initial
// radius but does not shrink; the first shrink is a later turn.
func TestZoneActivationTurnKillsWithoutShrinking(t *testing.T) {
	const (
		startTurn = 3
		interval  = 2
		step      = 1
		minRadius = 1
	)
	gs := newZoneTestState(startTurn, interval, step, minRadius)
	gs.ZoneRadius = 10 // radius² = 100

	p0 := gs.Players[0]
	p1 := gs.Players[1]
	outside := gs.SpawnBot(p0.ID, Position{Row: 1, Col: 1}) // d2 from (10,10) = 162
	inside := gs.SpawnBot(p1.ID, Position{Row: 10, Col: 8}) // d2 = 4

	// Turns 1-2: zone not yet active — nobody dies, radius untouched.
	for turn := 1; turn < startTurn; turn++ {
		gs.ExecuteTurn()
		if !outside.Alive {
			t.Fatalf("turn %d: bot died before zone activation", turn)
		}
		if gs.ZoneRadius != 10 {
			t.Fatalf("turn %d: radius = %d, want 10 (inactive zone must not shrink)", turn, gs.ZoneRadius)
		}
	}

	// Activation, as match.go performs it before collecting moves on turn 3.
	gs.ZoneActive = true

	gs.ExecuteTurn() // turn 3

	if outside.Alive {
		t.Fatalf("activation turn: bot outside radius %d survived, want death on the activation turn", gs.ZoneRadius)
	}
	if !inside.Alive {
		t.Fatalf("activation turn: bot inside the zone died, want alive")
	}
	if gs.ZoneRadius != 10 {
		t.Fatalf("activation turn: radius = %d, want 10 (no shrink on the activation turn)", gs.ZoneRadius)
	}

	// Turn 4: inside the interval, no shrink. Turn 5: first shrink.
	gs.ExecuteTurn()
	if gs.ZoneRadius != 10 {
		t.Fatalf("turn 4: radius = %d, want 10", gs.ZoneRadius)
	}
	gs.ExecuteTurn()
	if gs.ZoneRadius != 9 {
		t.Fatalf("turn 5: radius = %d, want 9 (first shrink at ZoneStartTurn+ZoneShrinkInterval)", gs.ZoneRadius)
	}
}
