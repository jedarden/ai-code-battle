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
//   - phase ordering: combat resolves before zone damage, and zone damage
//     resolves before capture
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

// TestZonePhaseOrdering pins the two interactions that distinguish zone
// damage from the neighboring phases. Combat deaths must not be reclassified
// as zone deaths, and a bot killed by the zone must not capture a core later
// in the same turn.
func TestZonePhaseOrdering(t *testing.T) {
	t.Run("combat resolves before zone damage", func(t *testing.T) {
		gs := newZoneTestState(1, 1000, 1, 1)
		gs.Config.AttackRadius2 = 1
		gs.ZoneActive = true
		gs.ZoneRadius = 4

		p0 := gs.Players[0]
		p1 := gs.Players[1]
		// p0Target is outside the zone but is outnumbered by the two p1
		// attackers. p1AttackerOutside is also outside the zone; if zone
		// damage ran first, no combat_death could be emitted for p0Target.
		p0Target := gs.SpawnBot(p0.ID, Position{Row: 10, Col: 5})
		p1AttackerOutside := gs.SpawnBot(p1.ID, Position{Row: 10, Col: 4})
		p1AttackerBoundary := gs.SpawnBot(p1.ID, Position{Row: 10, Col: 6})
		gs.SpawnBot(p0.ID, Position{Row: 10, Col: 10})
		gs.SpawnBot(p1.ID, Position{Row: 10, Col: 14})

		if result := gs.ExecuteTurn(); result != nil {
			t.Fatalf("ExecuteTurn() result = %+v, want match to continue", result)
		}
		if p0Target.Alive {
			t.Fatal("outnumbered target survived; combat should run before zone damage")
		}
		if !p1AttackerBoundary.Alive {
			t.Fatal("boundary attacker died; distance equal to the radius must be safe")
		}

		var combatTarget, zoneOutside bool
		for _, event := range gs.Events {
			switch event.Type {
			case EventCombatDeath:
				if id, ok := event.Details.(map[string]interface{})["bot_id"].(int); ok && id == p0Target.ID {
					combatTarget = true
				}
			case EventZoneDeath:
				if id, ok := zoneDeathDetails(t, event)["bot_id"].(int); ok && id == p1AttackerOutside.ID {
					zoneOutside = true
				}
			}
		}
		if !combatTarget {
			t.Errorf("events = %+v, want combat_death for outnumbered target %d", gs.Events, p0Target.ID)
		}
		if !zoneOutside {
			t.Errorf("events = %+v, want zone_death for outside attacker %d", gs.Events, p1AttackerOutside.ID)
		}
	})

	t.Run("zone damage resolves before capture", func(t *testing.T) {
		gs := newZoneTestState(1, 1000, 1, 1)
		gs.Config.AttackRadius2 = 1
		gs.ZoneActive = true
		gs.ZoneRadius = 4

		p0 := gs.Players[0]
		p1 := gs.Players[1]
		corePosition := Position{Row: 10, Col: 5} // outside radius 4
		core := gs.AddCore(p0.ID, corePosition)
		attacker := gs.SpawnBot(p1.ID, corePosition)
		gs.SpawnBot(p0.ID, Position{Row: 10, Col: 10})
		gs.SpawnBot(p1.ID, Position{Row: 10, Col: 14})

		if result := gs.ExecuteTurn(); result != nil {
			t.Fatalf("ExecuteTurn() result = %+v, want match to continue", result)
		}
		if attacker.Alive {
			t.Fatal("outside-zone attacker survived, want zone damage")
		}
		if !core.Active {
			t.Fatal("core was captured after its attacker died to zone damage")
		}

		for _, event := range gs.Events {
			if event.Type == EventCoreCaptured {
				t.Fatalf("unexpected core_captured event after zone death: %+v", event)
			}
		}
		deaths := findZoneDeaths(gs)
		if len(deaths) != 1 {
			t.Fatalf("got %d zone_death events, want exactly one", len(deaths))
		}
		if id, ok := zoneDeathDetails(t, deaths[0])["bot_id"].(int); !ok || id != attacker.ID {
			t.Errorf("zone_death bot_id = %v, want %d", zoneDeathDetails(t, deaths[0])["bot_id"], attacker.ID)
		}
	})
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

// TestZoneDamageCatchesBotsAcrossTurns pins damage as a per-turn consequence
// of the shrinking radius: a bot that starts inside the zone stays alive until
// the radius first crosses its distance², dies exactly on that turn (never
// before), and a bot sitting exactly on the final boundary soaks every turn.
// gs.Events only holds the current turn's events (ClearTurnState), so each
// zone_death is captured on the turn it fires and the death turns are
// reconciled at the end.
func TestZoneDamageCatchesBotsAcrossTurns(t *testing.T) {
	const (
		startTurn = 1
		interval  = 1
		step      = 1
		minRadius = 2
	)
	gs := newZoneTestState(startTurn, interval, step, minRadius)
	gs.ZoneActive = true
	gs.ZoneRadius = 10

	p0 := gs.Players[0]
	p1 := gs.Players[1]
	// d² from the center (10,10): far 36, mid 16, edge 4. The radius path is
	// 10,9,8,... clamped at 2, so the shrink crosses far's d² on turn 6
	// (radius 5) and mid's on turn 8 (radius 3), while edge sits exactly on
	// the final boundary (4 == 2²) and must survive the whole soak. The bots
	// are pairwise beyond attack range (min mutual d² 20) and never move, so
	// combat cannot interfere with the zone verdicts.
	far := gs.SpawnBot(p0.ID, Position{Row: 4, Col: 10})
	mid := gs.SpawnBot(p0.ID, Position{Row: 10, Col: 6})
	edge := gs.SpawnBot(p1.ID, Position{Row: 12, Col: 10})

	// Radius expected after executing turn N; clamped at minRadius from 9 on.
	wantRadius := map[int]int{
		1: 10, 2: 9, 3: 8, 4: 7, 5: 6, 6: 5, 7: 4, 8: 3,
		9: 2, 10: 2, 11: 2, 12: 2,
	}
	// Death turn = first turn with dist² > radius²: alive on every earlier
	// turn, dead from that one on.
	wantDeathTurn := map[*Bot]int{far: 6, mid: 8}

	deathTurns := map[int]int{} // bot ID -> turn its zone_death fired
	for turn := 1; turn <= 12; turn++ {
		gs.ExecuteTurn()
		if gs.ZoneRadius != wantRadius[turn] {
			t.Fatalf("after turn %d: ZoneRadius = %d, want %d", turn, gs.ZoneRadius, wantRadius[turn])
		}

		for _, b := range []*Bot{far, mid, edge} {
			wantAlive := true
			if dt, dying := wantDeathTurn[b]; dying && turn >= dt {
				wantAlive = false
			}
			if b.Alive != wantAlive {
				t.Fatalf("turn %d: bot at %v (d² %d) alive = %v, want %v",
					turn, b.Position, gs.Grid.Distance2(b.Position, gs.ZoneCenter), b.Alive, wantAlive)
			}
		}

		for _, e := range findZoneDeaths(gs) {
			id, _ := zoneDeathDetails(t, e)["bot_id"].(int)
			if prev, seen := deathTurns[id]; seen {
				if prev != e.Turn {
					t.Fatalf("bot %d has zone_death stamped turn %d and turn %d", id, prev, e.Turn)
				}
				continue // same event still on the accumulated list
			}
			if e.Turn != turn {
				t.Fatalf("new zone_death for bot %d stamped turn %d, first seen during turn %d", id, e.Turn, turn)
			}
			deathTurns[id] = turn
		}
		for _, b := range []*Bot{far, mid} {
			wantAlive := true
			if dt := wantDeathTurn[b]; turn >= dt {
				wantAlive = false
			}
			if _, ok := deathTurns[b.ID]; ok != !wantAlive {
				t.Fatalf("turn %d: bot at %v zone_death event presence = %v, want %v",
					turn, b.Position, ok, !wantAlive)
			}
		}
		for _, e := range gs.Events {
			if e.Type == EventCombatDeath {
				t.Fatalf("turn %d: unexpected combat_death event: %+v", turn, e)
			}
		}
	}

	if len(deathTurns) != 2 {
		t.Fatalf("got %d zone_death events over the soak (%v), want exactly 2", len(deathTurns), deathTurns)
	}
	for b, want := range wantDeathTurn {
		if got := deathTurns[b.ID]; got != want {
			t.Errorf("bot at %v (d² %d) died on turn %d, want turn %d",
				b.Position, gs.Grid.Distance2(b.Position, gs.ZoneCenter), got, want)
		}
	}
	if !edge.Alive {
		t.Fatalf("edge bot died: d² %d must survive at the final radius %d (boundary == radius²)",
			gs.Grid.Distance2(edge.Position, gs.ZoneCenter), gs.ZoneRadius)
	}
	if gs.Players[p0.ID].BotCount != 0 || gs.Players[p1.ID].BotCount != 1 {
		t.Errorf("bot counts = %d/%d, want 0/1",
			gs.Players[p0.ID].BotCount, gs.Players[p1.ID].BotCount)
	}
	for _, p := range gs.Players {
		if p.Score != 0 {
			t.Errorf("player %d score = %d, want 0 (zone kills award no score)", p.ID, p.Score)
		}
	}
}

// TestZoneDisabledMatchHasNoZoneActivity drives a full MatchRunner match with
// the zone disabled and pins the absence contract end to end: no turn records
// ZoneBounds (the replay only records bounds when the zone is enabled), no
// zone_death or combat_death event ever fires, both idle bots survive to the
// turn limit, and the match still completes normally on turns.
func TestZoneDisabledMatchHasNoZoneActivity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Rows = 20
	cfg.Cols = 20
	cfg.MaxTurns = 6
	cfg.ZoneEnabled = false

	mr := NewMatchRunner(cfg, WithRNG(rand.New(rand.NewSource(11))))
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

	for _, turn := range replay.Turns {
		if turn.ZoneBounds != nil {
			t.Errorf("turn %d: ZoneBounds = %+v, want nil when the zone is disabled", turn.Turn, turn.ZoneBounds)
		}
		for _, e := range turn.Events {
			if e.Type == EventZoneDeath {
				t.Errorf("turn %d: unexpected zone_death event: %+v", turn.Turn, e)
			}
			if e.Type == EventCombatDeath {
				t.Errorf("turn %d: unexpected combat_death event: %+v", turn.Turn, e)
			}
		}
	}

	final := replay.Turns[len(replay.Turns)-1]
	for _, b := range final.Bots {
		if !b.Alive {
			t.Errorf("final turn: bot %d alive = false, want true (nothing may die in a disabled-zone idle match)", b.ID)
		}
	}

	if result.Turns != cfg.MaxTurns {
		t.Errorf("result.Turns = %d, want %d", result.Turns, cfg.MaxTurns)
	}
	if result.Reason != "turns" {
		t.Errorf("result.Reason = %q, want %q (idle bots must reach the turn limit with the zone disabled)", result.Reason, "turns")
	}
	if len(result.BotsAlive) != 2 {
		t.Fatalf("result.BotsAlive = %v, want one entry per player", result.BotsAlive)
	}
	for i, alive := range result.BotsAlive {
		if alive != cfg.CoresPerPlayer {
			t.Errorf("result.BotsAlive[%d] = %d, want %d (every bot must survive a disabled-zone idle match)",
				i, alive, cfg.CoresPerPlayer)
		}
	}
}

// TestZoneConfigDocumentedPlayerCountDifferences pins the §3.7.1 zone table
// as ConfigForPlayers materializes it: both tiers share ZoneStartTurn=10,
// ZoneShrinkInterval=1 and ZoneShrinkStep=1 (the zone starts on the same turn
// everywhere and shrinks exactly one tile per turn — never faster than a bot
// can move), and they differ only in ZoneMinRadius (2 for 2-player, 1 for
// 3+ player) and AttackRadius2 (25 vs 12).
func TestZoneConfigDocumentedPlayerCountDifferences(t *testing.T) {
	for _, tt := range []struct {
		name          string
		numPlayers    int
		wantMinRadius int
		wantAttack2   int
	}{
		{"2-player", 2, 2, 25},
		{"3-player", 3, 1, 12},
		{"4-player", 4, 1, 12},
		{"6-player", 6, 1, 12},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConfigForPlayers(tt.numPlayers, 1)

			if !cfg.ZoneEnabled {
				t.Error("ZoneEnabled = false, want true for every player count")
			}
			if cfg.ZoneStartTurn != 10 {
				t.Errorf("ZoneStartTurn = %d, want 10 (same start turn for both tiers)", cfg.ZoneStartTurn)
			}
			if cfg.ZoneShrinkInterval != 1 {
				t.Errorf("ZoneShrinkInterval = %d, want 1 (shrinks every turn)", cfg.ZoneShrinkInterval)
			}
			if cfg.ZoneShrinkStep != 1 {
				t.Errorf("ZoneShrinkStep = %d, want 1 (zone must not outrun bot movement)", cfg.ZoneShrinkStep)
			}
			if cfg.ZoneMinRadius != tt.wantMinRadius {
				t.Errorf("ZoneMinRadius = %d, want %d", cfg.ZoneMinRadius, tt.wantMinRadius)
			}
			if cfg.AttackRadius2 != tt.wantAttack2 {
				t.Errorf("AttackRadius2 = %d, want %d", cfg.AttackRadius2, tt.wantAttack2)
			}
		})
	}
}

// TestZoneDocumentedContactGuaranteeAtMinimumRadius pins the §3.7.1 rationale
// end to end on the real per-player-count configs: two bots standing at
// opposite edges of the final (minimum-radius) zone sit exactly on the zone
// boundary — so the zone itself cannot kill them — and are within attack
// range of each other, which is what forces the final confrontation.
func TestZoneDocumentedContactGuaranteeAtMinimumRadius(t *testing.T) {
	for _, tt := range []struct {
		name       string
		numPlayers int
	}{
		{"2-player", 2},
		{"3-player", 3},
		{"4-player", 4},
		{"6-player", 6},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConfigForPlayers(tt.numPlayers, 1)
			gs := NewGameState(cfg, rand.New(rand.NewSource(3)))
			for range tt.numPlayers {
				gs.AddPlayer()
			}

			// The zone is anchored at the map center; pin that default so the
			// bot placement below and the engine's boundary check agree by
			// construction even if the center initialization ever moves.
			center := Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2}
			if gs.ZoneCenter != center {
				t.Fatalf("NewGameState ZoneCenter = %v, want the map center %v (the final zone must stay anchored)",
					gs.ZoneCenter, center)
			}

			// Clamp the zone to its documented minimum, as the shrink loop
			// would leave it after enough shrink turns.
			gs.ZoneActive = true
			gs.ZoneRadius = cfg.ZoneMinRadius

			// Two bots of the same owner at opposite edges of the final zone:
			// exactly minRadius from the center along the column axis, so each
			// sits ON the boundary (d² == radius²). Same owner, so combat can
			// never interfere with the zone verdict.
			west := gs.SpawnBot(gs.Players[0].ID, Position{Row: center.Row, Col: center.Col - cfg.ZoneMinRadius})
			east := gs.SpawnBot(gs.Players[0].ID, Position{Row: center.Row, Col: center.Col + cfg.ZoneMinRadius})

			gs.ExecuteTurn()

			if gs.ZoneRadius != cfg.ZoneMinRadius {
				t.Errorf("ZoneRadius = %d, want %d (minimum radius must hold)", gs.ZoneRadius, cfg.ZoneMinRadius)
			}
			for _, b := range []*Bot{west, east} {
				if !b.Alive {
					t.Fatalf("bot at %v (d² %d) died at minimum radius %d, want alive (boundary is safe)",
						b.Position, gs.Grid.Distance2(b.Position, gs.ZoneCenter), cfg.ZoneMinRadius)
				}
			}
			if deaths := findZoneDeaths(gs); len(deaths) != 0 {
				t.Fatalf("got %d zone_death events, want 0 (opposite boundary edges are inside the final zone)", len(deaths))
			}

			// The documented contact guarantee: bots at opposite edges of the
			// final zone are within attack range, so the match must end in a
			// fight rather than a standoff.
			if mutual := gs.Grid.Distance2(west.Position, east.Position); mutual > cfg.AttackRadius2 {
				t.Errorf("opposite zone edges are d² %d apart, want <= AttackRadius2 %d (final zone must force contact)",
					mutual, cfg.AttackRadius2)
			}
		})
	}
}

// TestZoneShrinkPathForRealConfigs drives the zone from activation to its
// floor using the actual ConfigForPlayers values for every player count:
// pre-activation radius stays at min(Rows,Cols)/2, activation snaps to 90% of
// the half-side (18 on the 40×40 2-player map — anchored against the §3.7.1
// table), the radius then drops one tile per turn from ZoneStartTurn+1 and
// bottoms out exactly at the documented ZoneMinRadius, where it holds. A
// single bot parked at the zone center soaks the whole path, so the timing
// contract is observed with no zone kills and no combat in the way.
func TestZoneShrinkPathForRealConfigs(t *testing.T) {
	for _, tt := range []struct {
		name       string
		numPlayers int
	}{
		{"2-player", 2},
		{"3-player", 3},
		{"4-player", 4},
		{"6-player", 6},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConfigForPlayers(tt.numPlayers, 1)
			gs := NewGameState(cfg, rand.New(rand.NewSource(5)))
			for range tt.numPlayers {
				gs.AddPlayer()
			}

			soaker := gs.SpawnBot(gs.Players[0].ID, Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2})

			preActivation := min(cfg.Rows, cfg.Cols) / 2
			snapped := (min(cfg.Rows, cfg.Cols) / 2 * 90) / 100
			if tt.numPlayers == 2 {
				if preActivation != 20 || snapped != 18 {
					t.Fatalf("2-player anchor moved: preActivation = %d, snapped = %d, want 20/18", preActivation, snapped)
				}
			}

			// Drive well past the clamp: activation snap, full descent, then
			// three extra turns parked on the minimum.
			lastTurn := cfg.ZoneStartTurn + (snapped - cfg.ZoneMinRadius) + 3
			for turn := 1; turn <= lastTurn; turn++ {
				if turn == cfg.ZoneStartTurn {
					// Activation, as match.go performs it before the turn runs.
					gs.ZoneActive = true
					gs.setInitialZoneRadius()
				}
				gs.ExecuteTurn()

				want := preActivation
				switch {
				case turn == cfg.ZoneStartTurn:
					want = snapped // activation snaps, never shrinks
				case turn > cfg.ZoneStartTurn:
					want = max(cfg.ZoneMinRadius, snapped-(turn-cfg.ZoneStartTurn))
				}
				if gs.ZoneRadius != want {
					t.Fatalf("after turn %d: ZoneRadius = %d, want %d", turn, gs.ZoneRadius, want)
				}
			}

			if gs.ZoneRadius != cfg.ZoneMinRadius {
				t.Errorf("final ZoneRadius = %d, want exactly the documented minimum %d",
					gs.ZoneRadius, cfg.ZoneMinRadius)
			}
			if !soaker.Alive {
				t.Fatal("center soaker died, want alive for the whole descent (d² 0 is always inside)")
			}
			if deaths := findZoneDeaths(gs); len(deaths) != 0 {
				t.Errorf("got %d zone_death events, want 0 (a centered bot must outsoak the entire shrink)", len(deaths))
			}
		})
	}
}
