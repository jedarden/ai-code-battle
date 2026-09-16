package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/aicodebattle/acb/engine"
)

// storageParityReplay builds a representative replay through the real
// ReplayWriter: several players, several turns with bots/cores/energy, delta
// encoding of scores/energy_held, and a result. Deterministic — no RNG.
func storageParityReplay(t *testing.T) *engine.Replay {
	t.Helper()

	cfg := engine.DefaultConfig()
	cfg.Rows, cfg.Cols = 20, 20
	cfg.MaxTurns = 10
	rw := engine.NewReplayWriter("m_storage_parity", cfg)
	rw.SetPlayers([]engine.ReplayPlayer{
		{ID: 0, Name: "Parity A"},
		{ID: 1, Name: "Parity B"},
	})

	rng := rand.New(rand.NewSource(42))
	gs := engine.NewGameState(cfg, rng)
	gs.AddPlayer()
	gs.AddPlayer()
	rw.SetMap(gs)

	for turn := 0; turn < 5; turn++ {
		gs.Turn = turn
		gs.SpawnBot(0, engine.Position{Row: 3 + turn, Col: 4})
		gs.SpawnBot(1, engine.Position{Row: 16, Col: 15 - turn})
		// Score changes only every other turn and energy never changes after
		// turn 0, so the fixture exercises both the delta-encoding omission
		// path and the changed-value path for both fields.
		gs.Players[0].Score = turn / 2
		gs.Players[1].Energy = 1
		rw.RecordTurn(gs, nil)
	}

	rw.Finalize(&engine.MatchResult{
		Winner: 0,
		Reason: "elimination",
		Turns:  5,
		Scores: []int{4, 0},
	})
	return rw.GetReplay()
}

// TestCompressReplayRoundTripParity pins the at-rest format contract: what
// compressReplay produces must decode, through the same gzip/JSON steps the
// serving boundary performs, back to the recorded replay. The client loader
// (web/src/lib/replay-data.ts fetchReplayFromUrl) gunzips and JSON.parses
// exactly this byte stream.
//
// Parity is asserted on the wire bytes and the JSON value tree, not with
// reflect.DeepEqual on engine.Replay structs: Event.Details is interface{}
// (maps holding Go ints and Position structs in memory), which JSON decodes
// as float64s and nested maps — semantically identical, never struct-equal.
func TestCompressReplayRoundTripParity(t *testing.T) {
	replay := storageParityReplay(t)

	compressed, err := compressReplay(replay)
	if err != nil {
		t.Fatalf("compressReplay: %v", err)
	}

	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}

	// 1. Byte parity: the gunzipped stream is exactly the replay's JSON
	//    serialization (what the browser's JSON.parse consumes).
	want, err := json.Marshal(replay)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !bytes.Equal(decoded, want) {
		t.Fatalf("decompressed bytes differ from the replay's JSON serialization")
	}

	// 2. Schema parity: the bytes decode into the replay schema without error.
	var out engine.Replay
	if err := json.Unmarshal(decoded, &out); err != nil {
		t.Fatalf("json.Unmarshal of decompressed bytes: %v", err)
	}

	// 3. Value-tree parity: re-serializing the decoded replay yields the same
	//    JSON value tree (the equality JSON.parse gives the browser).
	renorm, err := json.Marshal(&out)
	if err != nil {
		t.Fatalf("json.Marshal of decoded replay: %v", err)
	}
	var treeA, treeB interface{}
	if err := json.Unmarshal(decoded, &treeA); err != nil {
		t.Fatalf("json.Unmarshal tree: %v", err)
	}
	if err := json.Unmarshal(renorm, &treeB); err != nil {
		t.Fatalf("json.Unmarshal renorm tree: %v", err)
	}
	if !reflect.DeepEqual(treeA, treeB) {
		t.Fatalf("decoded replay's JSON value tree differs from the recorded one")
	}

	// 4. Delta-encoding decode parity: fill the omitted scores/energy_held
	//    forward exactly as the client's reconstructReplay does, and require
	//    the result to reproduce the engine's per-turn values (score was
	//    [turn/2, 0], energy was [0, 1] throughout this fixture).
	lastScores := out.Turns[0].Scores
	lastEnergy := out.Turns[0].EnergyHeld
	if lastScores == nil || lastEnergy == nil {
		t.Fatalf("first turn must carry explicit scores and energy_held")
	}
	omitted := 0
	for i, turn := range out.Turns {
		if turn.Scores != nil {
			lastScores = turn.Scores
		} else {
			omitted++
		}
		if turn.EnergyHeld != nil {
			lastEnergy = turn.EnergyHeld
		} else {
			omitted++
		}
		if !reflect.DeepEqual(lastScores, []int{i / 2, 0}) {
			t.Fatalf("turn %d: fill-forward scores = %v, want [%d 0]", i, lastScores, i/2)
		}
		if !reflect.DeepEqual(lastEnergy, []int{0, 1}) {
			t.Fatalf("turn %d: fill-forward energy_held = %v, want [0 1]", i, lastEnergy)
		}
	}
	if omitted == 0 {
		t.Fatalf("fixture never exercised the delta-omission path")
	}
}

// TestCompressReplayIsCompact pins compact (non-indented) JSON serialization:
// pretty-printed replays are ~2.7x larger, and the stored bytes are what the
// Pages bundle serves on the wire.
func TestCompressReplayIsCompact(t *testing.T) {
	replay := storageParityReplay(t)

	compressed, err := compressReplay(replay)
	if err != nil {
		t.Fatalf("compressReplay: %v", err)
	}

	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	plaintext, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	if bytes.Contains(plaintext, []byte("\n  ")) {
		t.Fatalf("replay JSON is indented; compact serialization required")
	}

	compact, err := json.Marshal(replay)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if len(compressed) >= len(compact) {
		t.Fatalf("stored payload (%d bytes) is not smaller than the plaintext JSON (%d bytes)", len(compressed), len(compact))
	}
}

// TestReplayStorageKey pins the object-key / URL contract. The site bundles
// replays under data/replays/<id>.json.gz and every client link constructs
// /data/replays/<id>.json.gz — changing the suffix orphans every existing URL.
func TestReplayStorageKey(t *testing.T) {
	if got := replayStorageKey("m_7f3a9b2c"); got != "replays/m_7f3a9b2c.json.gz" {
		t.Fatalf("replayStorageKey = %q, want replays/m_7f3a9b2c.json.gz", got)
	}
	if !strings.HasSuffix(replayStorageKey("x"), ".json.gz") {
		t.Fatalf("replay key must keep the .json.gz suffix")
	}
}

// TestFullMatchReplayWireBudget pins the compactness requirement
// (requirements.md: "Replay data format should be compact"). It builds a
// full-length 4-player match through the real ReplayWriter — 500 turns, a
// bot population that grows to 12 per player, score churn to exercise the
// v2.1 delta encoding — and asserts the gzipped payload every viewer
// downloads stays within a wire budget.
//
// Reference measurements behind the budget: a real 713-turn 4-player replay
// in the pre-delta v1.0 format was 17.4MB plain / 581KB gzipped at default
// gzip level; the v2.1 delta format with maximum compression comes in far
// below that for the same shape of game.
func TestFullMatchReplayWireBudget(t *testing.T) {
	const (
		players  = 4
		maxTurns = 500
		botsCap  = 12 // bots per player at plateau
		budget   = 192 * 1024
	)

	cfg := engine.DefaultConfig()
	cfg.Rows, cfg.Cols = 40, 40
	cfg.MaxTurns = maxTurns
	rw := engine.NewReplayWriter("m_wire_budget", cfg)

	ps := make([]engine.ReplayPlayer, players)
	for i := range ps {
		ps[i] = engine.ReplayPlayer{ID: i, Name: fmt.Sprintf("Budget %d", i)}
	}
	rw.SetPlayers(ps)

	rng := rand.New(rand.NewSource(7))
	gs := engine.NewGameState(cfg, rng)
	for i := 0; i < players; i++ {
		gs.AddPlayer()
	}
	rw.SetMap(gs)

	for turn := 0; turn < maxTurns; turn++ {
		gs.Turn = turn
		// Population grows to botsCap per player, then holds.
		if turn%(maxTurns/botsCap) == 0 {
			for p := 0; p < players; p++ {
				row := 4 + ((p*9 + turn) % (cfg.Rows - 8))
				col := 4 + ((p*7 + turn*3) % (cfg.Cols - 8))
				gs.SpawnBot(p, engine.Position{Row: row, Col: col})
			}
		}
		// Scores churn every third turn (delta encoding drops the other two).
		if turn%3 == 0 {
			gs.Players[turn%players].Score++
		}
		rw.RecordTurn(gs, nil)
	}

	rw.Finalize(&engine.MatchResult{
		Winner: 0,
		Reason: "turn_limit",
		Turns:  maxTurns,
		Scores: []int{167, 166, 166, 165},
	})
	replay := rw.GetReplay()

	plain, err := json.Marshal(replay)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	compressed, err := compressReplay(replay)
	if err != nil {
		t.Fatalf("compressReplay: %v", err)
	}

	t.Logf("full match (%d players, %d turns, %d bots): plain=%d bytes gz=%d bytes (%.1fx)",
		players, maxTurns, len(gs.Bots), len(plain), len(compressed),
		float64(len(plain))/float64(len(compressed)))

	if got := len(compressed); got >= budget {
		t.Fatalf("gzipped replay payload = %d bytes, wire budget is %d bytes", got, budget)
	}
}
