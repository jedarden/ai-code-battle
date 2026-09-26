package main

// Producer half of the ADR-001 live-feed contract, exercised end to end: the
// worker's real R2 write path — updateLiveTail PUTting matches/live-tail.json
// and updateLiveDelta PUTting leaderboard/live-delta.json through B2Client —
// against an in-process S3-compatible server, the exact surface Cloudflare R2
// presents. The uploaded bytes are asserted on the wire, because that JSON is
// byte for byte what the SPA downloads at /r2/... and merges into the home,
// leaderboard and matches pages (the consumer half is pinned in
// web/src/live-feed.integration.test.ts; the pure publication core is already
// covered by live_delta_test.go).
//
// updateLiveDelta reads each participant's persisted counters through
// botMatchStats; the harness points that at an unreachable PostgreSQL so the
// documented fallback applies ("a bot whose row cannot be read falls back to
// zeros"), which publishes the same per-match played=1 record
// live_delta_test.go pins. Feed degradation on the read side — a missing or
// malformed stored blob — must never fail the match publication: the worker
// starts a fresh feed and the next write heals the file.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const liveFeedBucket = "acb-live-feed-test"

// uploadRecord captures the caching headers of one live-feed upload. The
// feeds are only "live" if intermediaries revalidate them within seconds —
// that contract lives in UploadLive's headers, not in the JSON.
type uploadRecord struct {
	cacheControl string
	contentType  string
}

// fakeR2Store is an in-process S3-compatible server covering the subset of
// the API B2Client uses for the live feeds: path-style PutObject (UploadLive)
// and GetObject (Download), with an S3-shaped NoSuchKey for missing objects.
type fakeR2Store struct {
	server  *httptest.Server
	mu      sync.Mutex
	store   map[string][]byte
	uploads map[string]uploadRecord
}

func newFakeR2Store(t *testing.T) *fakeR2Store {
	t.Helper()
	f := &fakeR2Store{store: map[string][]byte{}, uploads: map[string]uploadRecord{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/"+liveFeedBucket+"/")
		switch r.Method {
		case http.MethodGet:
			f.mu.Lock()
			data, ok := f.store[key]
			f.mu.Unlock()
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>%s</Message></Error>`, key)
				return
			}
			_, _ = w.Write(data)
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.mu.Lock()
			f.store[key] = body
			f.uploads[key] = uploadRecord{
				cacheControl: r.Header.Get("Cache-Control"),
				contentType:  r.Header.Get("Content-Type"),
			}
			f.mu.Unlock()
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeR2Store) put(key string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[key] = data
}

func (f *fakeR2Store) get(t *testing.T, key string) []byte {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.store[key]
	if !ok {
		t.Fatalf("%s was never uploaded", key)
	}
	return data
}

func (f *fakeR2Store) uploadOf(t *testing.T, key string) uploadRecord {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.uploads[key]
	if !ok {
		t.Fatalf("%s was never uploaded", key)
	}
	return rec
}

// newLiveFeedWorker builds a Worker whose R2 client talks to the fake store
// and whose database handle cannot connect, so botMatchStats exercises the
// zeros fallback on every read.
func newLiveFeedWorker(t *testing.T, store *fakeR2Store) *Worker {
	t.Helper()
	cfg := &Config{
		R2Endpoint:  store.server.URL,
		R2Bucket:    liveFeedBucket,
		R2AccessKey: "test-access-key",
		R2SecretKey: "test-secret-key",
	}
	// Port 1 on loopback refuses immediately: nothing listens there, so every
	// botMatchStats query errors into the documented zeros fallback.
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable connect_timeout=1")
	if err != nil {
		t.Fatalf("open unreachable stats db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Worker{
		cfg:    cfg,
		db:     &DBClient{db: db},
		r2:     NewR2Client(cfg),
		logger: log.New(io.Discard, "", 0),
	}
}

// liveFeedClaim builds the claim the matchmaker hands the worker for one
// two-bot match, and liveFeedUpdates the rating updates computeRatingUpdates
// would produce for it — hand-built here so the published numbers are simple
// arithmetic, not Glicko reference vectors (those are pinned elsewhere).
func liveFeedClaim(matchID string) *JobClaimData {
	return &JobClaimData{
		Match: DBMatch{ID: matchID},
		Map:   DBMapData{ID: "map-duel-01"},
		Participants: []DBParticipant{
			{BotID: "alpha", PlayerSlot: 0},
			{BotID: "beta", PlayerSlot: 1},
		},
		Bots: []DBBotInfo{
			{ID: "alpha", Name: "Alpha"},
			{ID: "beta", Name: "Beta"},
		},
	}
}

// TestLiveFeedWritePathEndToEnd publishes three matches — alpha wins, beta
// wins, then a draw — through the real write path and asserts both files on
// the wire after each: field names and shapes the SPA's types mirror, newest-
// first tail ordering, accumulated deltas with replaced absolutes, the draw's
// omitted winner_id, and the live-cache upload headers.
func TestLiveFeedWritePathEndToEnd(t *testing.T) {
	store := newFakeR2Store(t)
	w := newLiveFeedWorker(t, store)
	ctx := context.Background()

	// Both bots start at display rating 800 (mu 1500 - 2*350). Each publish
	// chains from the previous post-state: a match's rating_delta is new
	// display minus (mu_before - 2*phi_before), so the before values must be
	// the prior match's published state, as the live pipeline sees them.
	//
	//	match 1: alpha 800 -> 1060 (+260)   beta 800 -> 760 (-40)
	//	match 2: alpha 1060 -> 1000 (-60)   beta 760 -> 820 (+60)
	//	match 3: alpha 1000 ->  990 (-10)   beta 820 -> 830 (+10)
	publish := func(matchID, winner string, before [2][2]float64, display [2]float64) {
		t.Helper()
		result := &MatchResult{
			WinnerID:  winner,
			Turns:     142,
			EndReason: "annihilation",
			Scores:    map[string]int{"alpha": 12, "beta": 4},
		}
		if err := w.updateLiveTail(ctx, liveFeedClaim(matchID), result); err != nil {
			t.Fatalf("updateLiveTail (%s): %v", matchID, err)
		}
		updates := []RatingUpdate{
			{BotID: "alpha", DisplayRating: display[0], Phi: 300, RatingMuBefore: before[0][0], RatingPhiBefore: before[0][1]},
			{BotID: "beta", DisplayRating: display[1], Phi: 300, RatingMuBefore: before[1][0], RatingPhiBefore: before[1][1]},
		}
		if err := w.updateLiveDelta(ctx, liveFeedClaim(matchID), result, updates); err != nil {
			t.Fatalf("updateLiveDelta (%s): %v", matchID, err)
		}
	}

	assertRFC3339 := func(t *testing.T, stamp string, where string) {
		t.Helper()
		if _, err := time.Parse(time.RFC3339, stamp); err != nil {
			t.Fatalf("%s updated_at %q is not RFC3339: %v", where, stamp, err)
		}
	}

	tailObject := func(t *testing.T) map[string]any {
		t.Helper()
		var tail map[string]any
		if err := json.Unmarshal(store.get(t, "matches/live-tail.json"), &tail); err != nil {
			t.Fatalf("stored live-tail.json is not valid JSON: %v", err)
		}
		return tail
	}

	deltaObject := func(t *testing.T) map[string]any {
		t.Helper()
		var delta map[string]any
		if err := json.Unmarshal(store.get(t, "leaderboard/live-delta.json"), &delta); err != nil {
			t.Fatalf("stored live-delta.json is not valid JSON: %v", err)
		}
		return delta
	}

	// ── Match 1: alpha wins, 800 -> 1060 display (+260); beta loses to 760 (-40).
	publish("m-0001", "alpha", [2][2]float64{{1500, 350}, {1500, 350}}, [2]float64{1060, 760})

	// The upload headers are the freshness contract: a CDN serving these under
	// the batch build's immutable year-long policy would freeze the tail.
	for _, key := range []string{"matches/live-tail.json", "leaderboard/live-delta.json"} {
		rec := store.uploadOf(t, key)
		if rec.cacheControl != "public, max-age=10" {
			t.Errorf("%s uploaded with Cache-Control %q, want \"public, max-age=10\"", key, rec.cacheControl)
		}
		if rec.contentType != "application/json" {
			t.Errorf("%s uploaded with Content-Type %q, want application/json", key, rec.contentType)
		}
	}

	tail := tailObject(t)
	if got := tail["updated_at"]; got == nil {
		t.Error("live-tail.json missing updated_at")
	} else {
		assertRFC3339(t, got.(string), "live-tail.json")
	}
	matches, _ := tail["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("live-tail.json has %d matches after the first publish, want 1", len(matches))
	}
	m1, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("live-tail.json matches[0] is %T, want object", matches[0])
	}
	if m1["id"] != "m-0001" {
		t.Errorf("tail match id = %v, want m-0001", m1["id"])
	}
	if m1["map_id"] != "map-duel-01" {
		t.Errorf("tail match map_id = %v, want map-duel-01", m1["map_id"])
	}
	assertRFC3339(t, m1["completed_at"].(string), "tail match")
	// Wire names the SPA's LiveTailMatch interface mirrors.
	for _, key := range []string{"id", "completed_at", "participants", "winner_id", "map_id", "turns", "end_reason"} {
		if _, ok := m1[key]; !ok {
			t.Errorf("tail match missing %q on the wire", key)
		}
	}
	if m1["turns"] != float64(142) || m1["end_reason"] != "annihilation" {
		t.Errorf("tail match turns/end_reason = %v/%v, want 142/annihilation", m1["turns"], m1["end_reason"])
	}
	if m1["winner_id"] != "alpha" {
		t.Errorf("tail match winner_id = %v, want alpha", m1["winner_id"])
	}
	participants, _ := m1["participants"].([]any)
	if len(participants) != 2 {
		t.Fatalf("tail match has %d participants, want 2", len(participants))
	}
	p0 := participants[0].(map[string]any)
	// Player-slot order, names resolved from the claim's bot list, scores and
	// the won flag straight off the result — this is what the match cards render.
	if p0["bot_id"] != "alpha" || p0["name"] != "Alpha" || p0["score"] != float64(12) || p0["won"] != true {
		t.Errorf("tail participants[0] = %v, want alpha/Alpha/12/won", p0)
	}
	p1 := participants[1].(map[string]any)
	if p1["bot_id"] != "beta" || p1["won"] != false {
		t.Errorf("tail participants[1] = %v, want beta/not won", p1)
	}

	delta := deltaObject(t)
	assertRFC3339(t, delta["updated_at"].(string), "live-delta.json")
	deltas, _ := delta["deltas"].(map[string]any)
	if len(deltas) != 2 {
		t.Fatalf("live-delta.json has %d deltas, want 2", len(deltas))
	}
	alpha := deltas["alpha"].(map[string]any)
	// rating_delta = new display (1060) minus before display (1500 - 2*350).
	assertClose(t, "alpha rating_delta", alpha["rating_delta"].(float64), 260, 1e-9)
	assertClose(t, "alpha new_rating", alpha["new_rating"].(float64), 1060, 1e-9)
	assertClose(t, "alpha new_rating_deviation", alpha["new_rating_deviation"].(float64), 300, 1e-9)
	// botMatchStats fell back to zeros and advanced by this match: played=1,
	// and the win rate is a percentage on the leaderboard.json scale.
	if alpha["new_matches_played"] != float64(1) || alpha["new_matches_won"] != float64(1) {
		t.Errorf("alpha match record = %v/%v, want 1/1", alpha["new_matches_played"], alpha["new_matches_won"])
	}
	assertClose(t, "alpha new_win_rate", alpha["new_win_rate"].(float64), 100, 1e-9)

	// The loser's zeros must be on the wire: the page merges omitted fields
	// with a ?? fallback, so a dropped new_matches_won would leave the stale
	// batch record on display (see live_delta_test.go for the marshal check).
	beta := deltas["beta"].(map[string]any)
	assertClose(t, "beta rating_delta", beta["rating_delta"].(float64), -40, 1e-9)
	for _, key := range []string{"rating_delta", "new_rating", "new_rating_deviation", "new_matches_played", "new_matches_won", "new_win_rate"} {
		if _, ok := beta[key]; !ok {
			t.Errorf("loser delta missing %q on the wire — the page would keep the stale batch value", key)
		}
	}
	if beta["new_matches_won"] != float64(0) || beta["new_win_rate"] != float64(0) {
		t.Errorf("beta match record = %v/%v, want 0/0", beta["new_matches_won"], beta["new_win_rate"])
	}

	// ── Match 2: beta wins. Both feeds are read-modify-write through the
	// store, exactly as the next match's worker process sees them.
	publish("m-0002", "beta", [2][2]float64{{1660, 300}, {1360, 300}}, [2]float64{1000, 820})

	matches, _ = tailObject(t)["matches"].([]any)
	if len(matches) != 2 {
		t.Fatalf("live-tail.json has %d matches after the second publish, want 2", len(matches))
	}
	if matches[0].(map[string]any)["id"] != "m-0002" || matches[1].(map[string]any)["id"] != "m-0001" {
		t.Errorf("tail ordering = %v, %v; newest must lead", matches[0].(map[string]any)["id"], matches[1].(map[string]any)["id"])
	}

	deltas, _ = deltaObject(t)["deltas"].(map[string]any)
	alpha = deltas["alpha"].(map[string]any)
	assertClose(t, "alpha rating_delta after two matches", alpha["rating_delta"].(float64), 200, 1e-9)
	assertClose(t, "alpha new_rating", alpha["new_rating"].(float64), 1000, 1e-9)
	// The lost match replaced the record: played stays 1 (per-match fallback),
	// won and rate dropped to 0.
	if alpha["new_matches_won"] != float64(0) || alpha["new_win_rate"] != float64(0) {
		t.Errorf("alpha record after loss = %v/%v, want 0/0", alpha["new_matches_won"], alpha["new_win_rate"])
	}
	beta = deltas["beta"].(map[string]any)
	assertClose(t, "beta rating_delta after two matches", beta["rating_delta"].(float64), 20, 1e-9)
	assertClose(t, "beta new_rating", beta["new_rating"].(float64), 820, 1e-9)

	// ── Match 3: a draw. No winner_id on the wire (omitempty), nobody won.
	publish("m-0003", "", [2][2]float64{{1600, 300}, {1420, 300}}, [2]float64{990, 830})

	matches, _ = tailObject(t)["matches"].([]any)
	draw := matches[0].(map[string]any)
	if draw["id"] != "m-0003" {
		t.Fatalf("draw match not prepended: %v", draw["id"])
	}
	if _, ok := draw["winner_id"]; ok {
		t.Errorf("draw match carries winner_id %v on the wire, want the key omitted", draw["winner_id"])
	}
	drawParticipants, _ := draw["participants"].([]any)
	for i, p := range drawParticipants {
		if p.(map[string]any)["won"] != false {
			t.Errorf("draw participants[%d] won = %v, want false", i, p.(map[string]any)["won"])
		}
	}

	deltas, _ = deltaObject(t)["deltas"].(map[string]any)
	assertClose(t, "alpha rating_delta after draw", deltas["alpha"].(map[string]any)["rating_delta"].(float64), 190, 1e-9)
	assertClose(t, "beta rating_delta after draw", deltas["beta"].(map[string]any)["rating_delta"].(float64), 30, 1e-9)
}

// TestLiveFeedHealsMissingAndMalformedBlobs covers the read-side degradation:
// a missing feed (nothing published yet) and a malformed one (a truncated
// previous write) both start a fresh feed, and the publish that follows heals
// the object with a single-match feed.
func TestLiveFeedHealsMissingAndMalformedBlobs(t *testing.T) {
	store := newFakeR2Store(t)
	w := newLiveFeedWorker(t, store)
	ctx := context.Background()
	result := &MatchResult{
		WinnerID:  "alpha",
		Turns:     60,
		EndReason: "zone-hold",
		Scores:    map[string]int{"alpha": 7, "beta": 3},
	}

	// Missing live-tail.json (NoSuchKey): the first publish after the bucket
	// is emptied by the weekly index rebuild must succeed.
	if err := w.updateLiveTail(ctx, liveFeedClaim("m-first"), result); err != nil {
		t.Fatalf("updateLiveTail with no stored feed: %v", err)
	}
	matches := tailMatches(t, store)
	if len(matches) != 1 || matches[0]["id"] != "m-first" {
		t.Fatalf("fresh tail = %v, want exactly m-first", matches)
	}

	// Missing live-delta.json: same contract.
	updates := []RatingUpdate{
		{BotID: "alpha", DisplayRating: 900, Phi: 300, RatingMuBefore: 1500, RatingPhiBefore: 350},
		{BotID: "beta", DisplayRating: 760, Phi: 300, RatingMuBefore: 1500, RatingPhiBefore: 350},
	}
	if err := w.updateLiveDelta(ctx, liveFeedClaim("m-first"), result, updates); err != nil {
		t.Fatalf("updateLiveDelta with no stored feed: %v", err)
	}
	deltas := storedDeltas(t, store)
	if _, ok := deltas["alpha"]; !ok {
		t.Fatalf("fresh delta feed missing alpha: %v", deltas)
	}

	// Malformed live-tail.json — the worker must start fresh rather than fail
	// the publication (its log carries a warning; the next write heals it).
	store.put("matches/live-tail.json", []byte(`{"updated_at": "2026-09-26T00:00:00Z", "matches": [ {"id": "trunc`))
	if err := w.updateLiveTail(ctx, liveFeedClaim("m-heal"), result); err != nil {
		t.Fatalf("updateLiveTail over a malformed feed: %v", err)
	}
	matches = tailMatches(t, store)
	if len(matches) != 1 || matches[0]["id"] != "m-heal" {
		t.Fatalf("tail after malformed blob = %v, want exactly m-heal", matches)
	}

	// Malformed live-delta.json: the legacy deltas are abandoned, not merged.
	store.put("leaderboard/live-delta.json", []byte(`{"deltas": {"legacy": {"new_rating": `))
	if err := w.updateLiveDelta(ctx, liveFeedClaim("m-heal"), result, updates); err != nil {
		t.Fatalf("updateLiveDelta over a malformed feed: %v", err)
	}
	deltas = storedDeltas(t, store)
	if len(deltas) != 2 {
		t.Fatalf("delta feed after malformed blob has %d entries (%v), want exactly alpha and beta", len(deltas), deltas)
	}
	if _, ok := deltas["legacy"]; ok {
		t.Error("malformed feed's phantom bots survived the heal")
	}
}

// TestLiveTailRollingWindow pins the 50-match retention: seeding 50 matches
// into the store and publishing one more must keep the newest 50, evicting
// the oldest rather than growing the object unbounded.
func TestLiveTailRollingWindow(t *testing.T) {
	store := newFakeR2Store(t)
	w := newLiveFeedWorker(t, store)
	ctx := context.Background()

	// Seed newest-first, the order the worker maintains: seed-049 leads.
	seed := LiveTailFeed{Updated: "2026-09-26T00:00:00Z", Matches: []LiveTailMatch{}}
	for i := 49; i >= 0; i-- {
		id := fmt.Sprintf("seed-%03d", i)
		seed.Matches = append(seed.Matches, LiveTailMatch{ID: id, CompletedAt: "2026-09-26T00:00:00Z"})
	}
	blob, err := json.Marshal(seed)
	if err != nil {
		t.Fatalf("marshal seed feed: %v", err)
	}
	store.put("matches/live-tail.json", blob)

	result := &MatchResult{WinnerID: "alpha", EndReason: "annihilation", Scores: map[string]int{"alpha": 1, "beta": 0}}
	if err := w.updateLiveTail(ctx, liveFeedClaim("m-new"), result); err != nil {
		t.Fatalf("updateLiveTail over a full tail: %v", err)
	}

	matches := tailMatches(t, store)
	if len(matches) != 50 {
		t.Fatalf("tail holds %d matches after the 51st publish, want the 50-match window", len(matches))
	}
	if matches[0]["id"] != "m-new" {
		t.Errorf("newest match = %v, want m-new leading", matches[0]["id"])
	}
	if matches[49]["id"] != "seed-001" {
		t.Errorf("oldest retained = %v, want seed-001 (seed-000 must have been evicted)", matches[49]["id"])
	}
	for _, m := range matches {
		if m["id"] == "seed-000" {
			t.Error("seed-000 survived the rolling window")
		}
	}
}

// tailMatches unmarshals the stored live-tail.json's match array into generic
// objects so the assertions see exactly what the SPA fetch sees.
func tailMatches(t *testing.T, store *fakeR2Store) []map[string]any {
	t.Helper()
	var tail struct {
		Matches []map[string]any `json:"matches"`
	}
	if err := json.Unmarshal(store.get(t, "matches/live-tail.json"), &tail); err != nil {
		t.Fatalf("stored live-tail.json is not valid JSON: %v", err)
	}
	return tail.Matches
}

// storedDeltas unmarshals the stored live-delta.json deltas the same way.
func storedDeltas(t *testing.T, store *fakeR2Store) map[string]map[string]any {
	t.Helper()
	var delta struct {
		Deltas map[string]map[string]any `json:"deltas"`
	}
	if err := json.Unmarshal(store.get(t, "leaderboard/live-delta.json"), &delta); err != nil {
		t.Fatalf("stored live-delta.json is not valid JSON: %v", err)
	}
	return delta.Deltas
}
