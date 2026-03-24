package engine

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"
)

// TestDeterminism_SameSeedSameReplay verifies that running the same match
// with the same seed produces identical replay output (excluding timestamps).
func TestDeterminism_SameSeedSameReplay(t *testing.T) {
	seed := int64(12345)
	config := DefaultConfig()
	config.Rows = 20
	config.Cols = 20
	config.MaxTurns = 50 // Shorter for testing

	runMatch := func() *Replay {
		rng := rand.New(rand.NewSource(seed))
		mr := NewMatchRunner(config,
			WithRNG(rng),
			WithTimeout(1*time.Second),
		)

		// Use deterministic bots (IdleBot always returns same moves)
		mr.AddBot(NewIdleBot(), "Idle1")
		mr.AddBot(NewIdleBot(), "Idle2")

		_, replay, err := mr.Run()
		if err != nil {
			t.Fatalf("Match failed: %v", err)
		}
		return replay
	}

	// Run match twice
	replay1 := runMatch()
	replay2 := runMatch()

	// Compare match ID (deterministic from seed)
	if replay1.MatchID != replay2.MatchID {
		t.Errorf("MatchID differs: %s vs %s", replay1.MatchID, replay2.MatchID)
	}

	// Compare map (should be identical)
	if len(replay1.Map.Walls) != len(replay2.Map.Walls) {
		t.Errorf("Wall count differs: %d vs %d", len(replay1.Map.Walls), len(replay2.Map.Walls))
	}
	if len(replay1.Map.Cores) != len(replay2.Map.Cores) {
		t.Errorf("Core count differs: %d vs %d", len(replay1.Map.Cores), len(replay2.Map.Cores))
	}

	// Compare turns
	if len(replay1.Turns) != len(replay2.Turns) {
		t.Fatalf("Turn count differs: %d vs %d", len(replay1.Turns), len(replay2.Turns))
	}

	for i := range replay1.Turns {
		t1 := replay1.Turns[i]
		t2 := replay2.Turns[i]

		if t1.Turn != t2.Turn {
			t.Errorf("Turn %d number differs: %d vs %d", i, t1.Turn, t2.Turn)
		}

		if len(t1.Bots) != len(t2.Bots) {
			t.Errorf("Turn %d bot count differs: %d vs %d", i, len(t1.Bots), len(t2.Bots))
			continue
		}

		for j := range t1.Bots {
			b1 := t1.Bots[j]
			b2 := t2.Bots[j]

			if b1.Position != b2.Position {
				t.Errorf("Turn %d bot %d position differs: %v vs %v", i, j, b1.Position, b2.Position)
			}
			if b1.Alive != b2.Alive {
				t.Errorf("Turn %d bot %d alive differs: %v vs %v", i, j, b1.Alive, b2.Alive)
			}
			if b1.Owner != b2.Owner {
				t.Errorf("Turn %d bot %d owner differs: %d vs %d", i, j, b1.Owner, b2.Owner)
			}
		}

		// Compare scores
		for j := range t1.Scores {
			if j >= len(t2.Scores) {
				break
			}
			if t1.Scores[j] != t2.Scores[j] {
				t.Errorf("Turn %d player %d score differs: %d vs %d", i, j, t1.Scores[j], t2.Scores[j])
			}
		}
	}

	// Compare result
	if replay1.Result != nil && replay2.Result != nil {
		if replay1.Result.Winner != replay2.Result.Winner {
			t.Errorf("Winner differs: %d vs %d", replay1.Result.Winner, replay2.Result.Winner)
		}
		if replay1.Result.Reason != replay2.Result.Reason {
			t.Errorf("Reason differs: %s vs %s", replay1.Result.Reason, replay2.Result.Reason)
		}
		if replay1.Result.Turns != replay2.Result.Turns {
			t.Errorf("Result turns differs: %d vs %d", replay1.Result.Turns, replay2.Result.Turns)
		}
	}
}

// TestDeterminism_TurnExecutionIsDeterministic verifies that executing
// the same turn with the same moves produces identical results.
func TestDeterminism_TurnExecutionIsDeterministic(t *testing.T) {
	runTurn := func() *GameState {
		config := DefaultConfig()
		config.Rows = 10
		config.Cols = 10

		rng := rand.New(rand.NewSource(42))
		gs := NewGameState(config, rng)

		p0 := gs.AddPlayer()
		p1 := gs.AddPlayer()

		// Set up identical initial state
		gs.AddCore(p0.ID, Position{5, 5})
		gs.AddCore(p1.ID, Position{4, 4})

		bot0 := gs.SpawnBot(p0.ID, Position{5, 5})
		bot1 := gs.SpawnBot(p1.ID, Position{4, 4})

		// Add energy node
		en := gs.AddEnergyNode(Position{3, 3})
		en.HasEnergy = true
		en.Tick = 0

		// Submit same moves
		gs.SubmitMove(bot0.Position, DirN)
		gs.SubmitMove(bot1.Position, DirE)

		// Execute turn
		gs.ExecuteTurn()

		return gs
	}

	// Run twice (seeds shouldn't matter for deterministic turn execution)
	gs1 := runTurn()
	gs2 := runTurn()

	// Compare states
	if gs1.Turn != gs2.Turn {
		t.Errorf("Turn differs: %d vs %d", gs1.Turn, gs2.Turn)
	}

	if len(gs1.Bots) != len(gs2.Bots) {
		t.Errorf("Bot count differs: %d vs %d", len(gs1.Bots), len(gs2.Bots))
	}

	for i := range gs1.Bots {
		if i >= len(gs2.Bots) {
			break
		}
		b1 := gs1.Bots[i]
		b2 := gs2.Bots[i]

		if b1.Position != b2.Position {
			t.Errorf("Bot %d position differs: %v vs %v", i, b1.Position, b2.Position)
		}
		if b1.Alive != b2.Alive {
			t.Errorf("Bot %d alive differs: %v vs %v", i, b1.Alive, b2.Alive)
		}
	}

	// Compare scores
	for i := range gs1.Players {
		if i >= len(gs2.Players) {
			break
		}
		if gs1.Players[i].Score != gs2.Players[i].Score {
			t.Errorf("Player %d score differs: %d vs %d",
				i, gs1.Players[i].Score, gs2.Players[i].Score)
		}
		if gs1.Players[i].Energy != gs2.Players[i].Energy {
			t.Errorf("Player %d energy differs: %d vs %d",
				i, gs1.Players[i].Energy, gs2.Players[i].Energy)
		}
	}
}

// TestDeterminism_GridOperationsAreDeterministic verifies grid operations
// produce consistent results.
func TestDeterminism_GridOperationsAreDeterministic(t *testing.T) {
	g := NewGrid(60, 60)

	// Test wrapping
	for r := -100; r <= 100; r += 10 {
		for c := -100; c <= 100; c += 10 {
			p1 := g.Wrap(r, c)
			p2 := g.Wrap(r, c)
			if p1 != p2 {
				t.Errorf("Wrap(%d,%d) not deterministic: %v vs %v", r, c, p1, p2)
			}
		}
	}

	// Test distance
	positions := []Position{
		{0, 0}, {30, 30}, {59, 59}, {10, 20}, {50, 10},
	}
	for _, a := range positions {
		for _, b := range positions {
			d1 := g.Distance2(a, b)
			d2 := g.Distance2(a, b)
			if d1 != d2 {
				t.Errorf("Distance2(%v, %v) not deterministic: %d vs %d", a, b, d1, d2)
			}
			// Distance should be symmetric
			d3 := g.Distance2(b, a)
			if d1 != d3 {
				t.Errorf("Distance2 not symmetric: %v->%v = %d, %v->%v = %d",
					a, b, d1, b, a, d3)
			}
		}
	}

	// Test visibility
	vis1 := g.VisibleFrom(positions, 49)
	vis2 := g.VisibleFrom(positions, 49)
	for p := range vis1 {
		if !vis2[p] {
			t.Errorf("VisibleFrom not deterministic: %v in vis1 but not vis2", p)
		}
	}
	for p := range vis2 {
		if !vis1[p] {
			t.Errorf("VisibleFrom not deterministic: %v in vis2 but not vis1", p)
		}
	}
}

// TestDeterminism_CombatResolutionIsDeterministic verifies that combat
// resolution produces consistent results.
func TestDeterminism_CombatResolutionIsDeterministic(t *testing.T) {
	runCombat := func() (alive0, alive1 int) {
		config := DefaultConfig()
		config.Rows = 10
		config.Cols = 10
		rng := rand.New(rand.NewSource(42))
		gs := NewGameState(config, rng)

		p0 := gs.AddPlayer()
		p1 := gs.AddPlayer()

		// 2v1 scenario
		gs.SpawnBot(p0.ID, Position{5, 5})
		gs.SpawnBot(p0.ID, Position{5, 6})
		gs.SpawnBot(p1.ID, Position{5, 7})

		gs.executeCombat()

		bots0 := gs.GetPlayerBots(p0.ID)
		bots1 := gs.GetPlayerBots(p1.ID)

		alive0 = len(bots0)
		alive1 = len(bots1)

		return alive0, alive1
	}

	// Run combat 10 times
	for i := 0; i < 10; i++ {
		a0_1, a1_1 := runCombat()
		a0_2, a1_2 := runCombat()

		if a0_1 != a0_2 || a1_1 != a1_2 {
			t.Errorf("Combat resolution not deterministic on iteration %d: (%v,%v) vs (%v,%v)",
				i, a0_1, a1_1, a0_2, a1_2)
		}
	}
}

// TestDeterminism_ReplaySerializationRoundTrip verifies that replays
// can be serialized and deserialized without data loss.
func TestDeterminism_ReplaySerializationRoundTrip(t *testing.T) {
	config := DefaultConfig()
	config.Rows = 10
	config.Cols = 10
	config.MaxTurns = 10

	rng := rand.New(rand.NewSource(42))
	mr := NewMatchRunner(config, WithRNG(rng))

	mr.AddBot(NewIdleBot(), "Player1")
	mr.AddBot(NewIdleBot(), "Player2")

	_, replay1, err := mr.Run()
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	// Serialize
	data, err := json.Marshal(replay1)
	if err != nil {
		t.Fatalf("Failed to marshal replay: %v", err)
	}

	// Deserialize
	replay2, err := LoadReplay(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal replay: %v", err)
	}

	// Compare key fields
	if replay1.MatchID != replay2.MatchID {
		t.Errorf("MatchID differs: %s vs %s", replay1.MatchID, replay2.MatchID)
	}

	if len(replay1.Turns) != len(replay2.Turns) {
		t.Errorf("Turn count differs: %d vs %d", len(replay1.Turns), len(replay2.Turns))
	}

	for i := range replay1.Turns {
		if i >= len(replay2.Turns) {
			break
		}
		if replay1.Turns[i].Turn != replay2.Turns[i].Turn {
			t.Errorf("Turn %d number differs: %d vs %d",
				i, replay1.Turns[i].Turn, replay2.Turns[i].Turn)
		}
		if len(replay1.Turns[i].Bots) != len(replay2.Turns[i].Bots) {
			t.Errorf("Turn %d bot count differs: %d vs %d",
				i, len(replay1.Turns[i].Bots), len(replay2.Turns[i].Bots))
		}
	}
}

// TestDeterminism_Full500TurnMatch verifies that a full-length match
// runs to completion with deterministic results.
func TestDeterminism_Full500TurnMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 500-turn match in short mode")
	}

	seed := int64(99999)
	config := DefaultConfig()
	config.Rows = 40
	config.Cols = 40
	config.MaxTurns = 500

	runMatch := func() *MatchResult {
		rng := rand.New(rand.NewSource(seed))
		mr := NewMatchRunner(config,
			WithRNG(rng),
			WithTimeout(1*time.Second),
		)

		mr.AddBot(NewIdleBot(), "Player1")
		mr.AddBot(NewIdleBot(), "Player2")

		result, _, err := mr.Run()
		if err != nil {
			t.Fatalf("Match failed: %v", err)
		}
		return result
	}

	// Run match twice
	result1 := runMatch()
	result2 := runMatch()

	// Compare results
	if result1.Winner != result2.Winner {
		t.Errorf("Winner differs: %d vs %d", result1.Winner, result2.Winner)
	}
	if result1.Reason != result2.Reason {
		t.Errorf("Reason differs: %s vs %s", result1.Reason, result2.Reason)
	}
	if result1.Turns != result2.Turns {
		t.Errorf("Turns differs: %d vs %d", result1.Turns, result2.Turns)
	}
}
