package main

// Tests for the leaderboard publication step of the rating pipeline: the
// chain the worker runs after every match (main.go executeMatch) is
//
//	claim participants -> computeRatingUpdates -> SubmitMatchResult
//	                   -> updateLiveDelta -> mergeLiveDeltas
//
// These tests drive the pure publication core (mergeLiveDeltas) with updates
// produced by the real pipeline entry point computeRatingUpdates, so the
// compute -> publish contract is exercised end to end without R2 or
// PostgreSQL. Rating vectors reuse the independent reference values pinned in
// glicko2_test.go (win from defaults: 1662.310894/290.318964 vs
// 1337.689106/290.318964, then a draw: 1576.688664 vs 1423.311336 at RD
// 260.488764).

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const defaultDisplayRating = 800.0 // 1500 - 2*350: the display rating of a fresh bot

// publishRunsOneMatch mirrors the worker's per-match path: build a claim the
// way ClaimJob would hand it over, compute the rating updates, and fold them
// into the feed.
func publishRunsOneMatch(t *testing.T, feed LiveDeltaFeed, muA, phiA, sigmaA, muB, phiB, sigmaB float64, winner string, now time.Time) (LiveDeltaFeed, []RatingUpdate) {
	t.Helper()
	claim := &JobClaimData{
		Participants: []DBParticipant{
			{BotID: "a", RatingMuBefore: muA, RatingPhiBefore: phiA, RatingSigmaBefore: sigmaA},
			{BotID: "b", RatingMuBefore: muB, RatingPhiBefore: phiB, RatingSigmaBefore: sigmaB},
		},
	}
	result := &MatchResult{WinnerID: winner}
	updates := (&Worker{}).computeRatingUpdates(claim, result)
	if len(updates) != 2 {
		t.Fatalf("rating updates = %d entries, want 2", len(updates))
	}

	// botMatchStats reads the persisted counters and advances them by this
	// match. The bots-row counters are never written back by the worker, so
	// every match publishes a played=1 record for its participants; that is
	// the pinned contract here (true totals come from the index rebuild,
	// which counts match_participants directly).
	stats := map[string]BotMatchStats{}
	for _, update := range updates {
		won := 0
		if winner == update.BotID {
			won = 1
		}
		stats[update.BotID] = BotMatchStats{MatchesPlayed: 1, MatchesWon: won}
	}
	return mergeLiveDeltas(feed, updates, stats, now), updates
}

// TestMergeLiveDeltasPublishesFirstMatch covers the first publication for two
// fresh bots: both start at display rating 800 (1500 - 2*350), the winner's
// delta is the display gain, the loser's is the display loss, and every
// absolute field describes the new state.
func TestMergeLiveDeltasPublishesFirstMatch(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.FixedZone("EST", -5*3600))

	feed, _ := publishRunsOneMatch(t, LiveDeltaFeed{}, 1500, 350, 0.06, 1500, 350, 0.06, "a", now)

	// Reference displays after a win from the defaults (see glicko2_test.go).
	wantDisplayA := 1662.310894 - 2*290.318964 // 1081.672966
	wantDisplayB := 1337.689106 - 2*290.318964 // 757.051178

	a, ok := feed.Deltas["a"]
	if !ok {
		t.Fatal("winner a missing from published deltas")
	}
	assertClose(t, "a rating_delta", a.RatingDelta, wantDisplayA-defaultDisplayRating, refTol)
	if a.RatingDelta <= 0 {
		t.Errorf("winner delta = %v, want positive", a.RatingDelta)
	}
	assertClose(t, "a new_rating", a.NewRating, wantDisplayA, refTol)
	assertClose(t, "a new_rating_deviation", a.NewRatingDeviation, 290.318964, refTol)
	// NewWinRate is a percentage on the leaderboard.json scale this feed
	// patches (the page renders it with a % suffix): a 1/1 record publishes
	// 100, not the 0-1 fraction.
	if a.NewMatchesPlayed != 1 || a.NewMatchesWon != 1 || a.NewWinRate != 100 {
		t.Errorf("a match record = played %d won %d rate %v, want 1/1/100", a.NewMatchesPlayed, a.NewMatchesWon, a.NewWinRate)
	}

	b, ok := feed.Deltas["b"]
	if !ok {
		t.Fatal("loser b missing from published deltas")
	}
	assertClose(t, "b rating_delta", b.RatingDelta, wantDisplayB-defaultDisplayRating, refTol)
	if b.RatingDelta >= 0 {
		t.Errorf("loser delta = %v, want negative", b.RatingDelta)
	}
	assertClose(t, "b new_rating", b.NewRating, wantDisplayB, refTol)
	if b.NewMatchesPlayed != 1 || b.NewMatchesWon != 0 || b.NewWinRate != 0 {
		t.Errorf("b match record = played %d won %d rate %v, want 1/0/0", b.NewMatchesPlayed, b.NewMatchesWon, b.NewWinRate)
	}

	// A fresh loss is a legitimate zero, so the loser's zero fields must
	// serialize (no omitempty): an omitted new_matches_won/new_win_rate would
	// leave the stale batch values on display while the rest of the record
	// moved — the page merges with a ?? fallback. b is the only participant
	// with zeros here, so the blob check is specifically the loser's.
	blob, err := json.Marshal(feed)
	if err != nil {
		t.Fatalf("marshal published feed: %v", err)
	}
	if !strings.Contains(string(blob), `"new_matches_won":0`) || !strings.Contains(string(blob), `"new_win_rate":0`) {
		t.Errorf("loser zero fields omitted from published JSON: %s", blob)
	}

	// The zero-value feed (missing live-delta.json) publishes a usable map.
	if feed.Deltas == nil {
		t.Fatal("published feed has a nil deltas map")
	}

	// The stamp is RFC3339 in UTC regardless of the local zone passed in.
	stamped, err := time.Parse(time.RFC3339, feed.Updated)
	if err != nil {
		t.Fatalf("updated_at %q is not RFC3339: %v", feed.Updated, err)
	}
	if stamped.UTC() != now.UTC() {
		t.Errorf("updated_at = %v, want %v", stamped.UTC(), now.UTC())
	}
}

// TestMergeLiveDeltasAccumulatesAcrossMatches pins the read-modify-write
// publication contract: the second match downloads the first feed (round
// tripped through JSON here, the same representation R2 returns), adds its
// rating delta on top, and replaces every absolute field with the latest
// match's values. After win-then-draw, a's accumulated delta must equal its
// final display rating minus the starting display rating exactly.
func TestMergeLiveDeltasAccumulatesAcrossMatches(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

	feed, updates1 := publishRunsOneMatch(t, LiveDeltaFeed{}, 1500, 350, 0.06, 1500, 350, 0.06, "a", now)

	// Simulate the R2 round trip: the next match sees the serialized feed.
	blob, err := json.Marshal(feed)
	if err != nil {
		t.Fatalf("marshal published feed: %v", err)
	}
	var downloaded LiveDeltaFeed
	if err := json.Unmarshal(blob, &downloaded); err != nil {
		t.Fatalf("unmarshal downloaded feed: %v", err)
	}

	feed, updates2 := publishRunsOneMatch(t, downloaded,
		updates1[0].Mu, updates1[0].Phi, updates1[0].Sigma,
		updates1[1].Mu, updates1[1].Phi, updates1[1].Sigma,
		"", now) // draw: everyone scores 0.5

	a := feed.Deltas["a"]
	b := feed.Deltas["b"]

	// a: win then draw. Display 1081.672966 -> 1055.711137; the accumulated
	// delta lands back on final display minus starting display. (The display
	// expectations compose two 6-decimal reference vectors, so they check at
	// tol, not refTol; the delta identity stays at refTol.)
	assertClose(t, "a rating_delta after two matches", a.RatingDelta, updates2[0].DisplayRating-defaultDisplayRating, refTol)
	assertClose(t, "a new_rating", a.NewRating, 1576.688664-2*260.488764, tol)
	assertClose(t, "a new_rating_deviation", a.NewRatingDeviation, 260.488764, refTol)

	// b: loss then draw, 757.051178 -> 902.333808; its negative and positive
	// deltas accumulate the same way.
	assertClose(t, "b rating_delta after two matches", b.RatingDelta, updates2[1].DisplayRating-defaultDisplayRating, refTol)
	assertClose(t, "b new_rating", b.NewRating, 1423.311336-2*260.488764, tol)

	// Absolute match records were replaced, not accumulated: the draw's
	// played=1 record (no winner) overwrote match 1's.
	if a.NewMatchesWon != 0 || a.NewWinRate != 0 {
		t.Errorf("a match record after draw = won %d rate %v, want match-2 values 0/0", a.NewMatchesWon, a.NewWinRate)
	}

	// The leaderboard-visible ordering survived both matches.
	if a.NewRating <= b.NewRating {
		t.Errorf("published ordering inverted: a %v b %v", a.NewRating, b.NewRating)
	}
}

// TestMergeLiveDeltasSkipsUntrackedBotsAndNilMap covers the publication
// guards: a rating update with no stats entry is not published (botMatchStats
// emits one entry per participant, so this is belt-and-braces), and a stored
// feed whose deltas array is JSON null — which a naive map write would panic
// on — starts fresh instead.
func TestMergeLiveDeltasSkipsUntrackedBotsAndNilMap(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	updates := []RatingUpdate{{BotID: "tracked", DisplayRating: 900, RatingMuBefore: 1500, RatingPhiBefore: 350}}

	feed := mergeLiveDeltas(LiveDeltaFeed{}, updates, nil, now)
	if len(feed.Deltas) != 0 {
		t.Errorf("untracked bot published: %v", feed.Deltas)
	}

	// "deltas": null must not panic and must start a fresh map.
	var downloaded LiveDeltaFeed
	if err := json.Unmarshal([]byte(`{"deltas": null, "updated_at": "old"}`), &downloaded); err != nil {
		t.Fatalf("unmarshal null-deltas feed: %v", err)
	}
	feed = mergeLiveDeltas(downloaded, updates, map[string]BotMatchStats{
		"tracked": {MatchesPlayed: 1, MatchesWon: 1},
	}, now)
	entry, ok := feed.Deltas["tracked"]
	if !ok {
		t.Fatal("tracked bot missing after null-deltas round trip")
	}
	assertClose(t, "tracked rating_delta", entry.RatingDelta, 900-defaultDisplayRating, 1e-9)
}
