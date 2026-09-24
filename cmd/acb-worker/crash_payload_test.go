package main

import "testing"

// These tests pin the worker-payload link of crash propagation:
// engineResult.Crashed (per player slot) -> MatchResult.CrashedBots (per bot).

// TestMapCrashedBots_MapsByPlayerSlotNotSlicePosition pins the slot indexing:
// the engine reports crash flags by player slot (bots join the runner in slot
// order), so the payload must key each bot's flag on its participant's
// PlayerSlot even when the participant slice itself arrives unordered.
func TestMapCrashedBots_MapsByPlayerSlotNotSlicePosition(t *testing.T) {
	// Deliberately unordered: slice position 0 holds slot 1.
	participants := []DBParticipant{
		{BotID: "bot-late", PlayerSlot: 1},
		{BotID: "bot-early", PlayerSlot: 0},
	}
	engineCrashed := []bool{true, false} // slot 0 crashed, slot 1 did not

	got := mapCrashedBots(participants, engineCrashed)

	if !got["bot-early"] {
		t.Errorf("bot-early (slot 0) crashed = false, want true")
	}
	if got["bot-late"] {
		t.Errorf("bot-late (slot 1) crashed = true, want false")
	}
	if len(got) != 2 {
		t.Errorf("payload populated %d entries, want 2 (every participant, crashed or not)", len(got))
	}
}

// TestMapCrashedBots_NonCrashedBotsPresent verifies clean bots still appear in
// the payload with false: updateCrashStrikes reads those entries to reset a
// bot's strike count after a match it did not crash (db.go).
func TestMapCrashedBots_NonCrashedBotsPresent(t *testing.T) {
	participants := []DBParticipant{
		{BotID: "bot-a", PlayerSlot: 0},
		{BotID: "bot-b", PlayerSlot: 1},
	}

	got := mapCrashedBots(participants, []bool{false, false})

	for _, p := range participants {
		crashed, ok := got[p.BotID]
		if !ok {
			t.Errorf("bot %q missing from payload, want a false entry", p.BotID)
		}
		if crashed {
			t.Errorf("bot %q crashed = true, want false", p.BotID)
		}
	}
}

// TestMapCrashedBots_OutOfRangeSlotIsSkipped pins the guard: a participant
// whose slot has no engine flag must not panic and must not fabricate a value.
func TestMapCrashedBots_OutOfRangeSlotIsSkipped(t *testing.T) {
	participants := []DBParticipant{
		{BotID: "bot-in", PlayerSlot: 0},
		{BotID: "bot-out", PlayerSlot: 5},
	}

	got := mapCrashedBots(participants, []bool{true})

	if !got["bot-in"] {
		t.Errorf("bot-in crashed = false, want true")
	}
	if _, ok := got["bot-out"]; ok {
		t.Errorf("bot-out has a payload entry for a slot the engine never reported")
	}
}

// TestMapCrashedBots_EmptyEngineFlagsYieldsEmptyPayload: without engine crash
// data the payload is empty, which updateCrashStrikes treats as a no-op.
func TestMapCrashedBots_EmptyEngineFlagsYieldsEmptyPayload(t *testing.T) {
	got := mapCrashedBots([]DBParticipant{{BotID: "bot-a", PlayerSlot: 0}}, nil)
	if len(got) != 0 {
		t.Errorf("payload = %v, want empty for nil engine crash flags", got)
	}
}
