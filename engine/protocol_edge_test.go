package engine

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"math/rand"
)

// The tests in this file pin the three bot-protocol guarantees documented in
// README.md ("Writing a Bot") and docs/notes/requirements.md:
//
//  1. Any bot response failing schema validation is treated as a no-op — the
//     bot's units hold position (and the bot stays in the match).
//  2. A bot that does not respond within 3 seconds has that response ignored
//     and its units hold — the bot is NOT killed or disconnected, and keeps
//     receiving future turns. (10 consecutive failures mark it crashed.)
//  3. Requests carry X-ACB-Signature: HMAC-SHA256 over
//     "{match_id}.{turn}.{sha256_hex(body)}"; responses carry the same family
//     of signature over the response body, verified strictly by the engine.

// protocolEdgeAction scripts the fake bot's reply for one /turn request.
type protocolEdgeAction struct {
	delay   time.Duration // stall before responding (0 = immediate)
	status  int           // HTTP status to return (0 = 200)
	body    []byte        // raw response body bytes
	sig     *string       // exact X-ACB-Signature to send (nil = sign body correctly)
	omitSig bool          // send no X-ACB-Signature header at all
}

// protocolEdgeRequest records one incoming /turn request.
type protocolEdgeRequest struct {
	State   VisibleState
	RawBody []byte
	Headers http.Header
}

// protocolEdgeServer is a scripted fake bot HTTP server.
type protocolEdgeServer struct {
	t      *testing.T
	secret string
	mu     sync.Mutex
	reqs   []protocolEdgeRequest
	script func(state *VisibleState, call int) protocolEdgeAction
	server *httptest.Server
}

func newProtocolEdgeServer(t *testing.T, secret string, script func(state *VisibleState, call int) protocolEdgeAction) *protocolEdgeServer {
	s := &protocolEdgeServer{t: t, secret: secret, script: script}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := readAllBody(r)
		if err != nil {
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		var state VisibleState
		jsonErr := json.Unmarshal(raw, &state)

		s.mu.Lock()
		call := len(s.reqs)
		s.reqs = append(s.reqs, protocolEdgeRequest{State: state, RawBody: raw, Headers: r.Header.Clone()})
		s.mu.Unlock()

		if jsonErr != nil {
			// The engine always sends well-formed state; a decode failure here
			// is a test bug, not a protocol case.
			http.Error(w, "unparseable state: "+jsonErr.Error(), http.StatusBadRequest)
			return
		}

		action := s.script(&state, call)

		if action.sig == nil && !action.omitSig {
			// Sign per the README recipe: the same match_id and turn the
			// request carried (headers), not values re-read from the body.
			sig := SignResponse(s.secret,
				r.Header.Get("X-ACB-Match-Id"), headerTurn(r.Header.Get("X-ACB-Turn")), action.body)
			action.sig = &sig
		}
		if action.delay > 0 {
			time.Sleep(action.delay)
		}
		status := action.status
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		if !action.omitSig {
			w.Header().Set("X-ACB-Signature", *action.sig)
		}
		w.WriteHeader(status)
		w.Write(action.body)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func readAllBody(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}

// headerTurn parses an X-ACB-Turn header value, defaulting to 0.
func headerTurn(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func (s *protocolEdgeServer) URL() string { return s.server.URL }
func (s *protocolEdgeServer) requests() []protocolEdgeRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]protocolEdgeRequest, len(s.reqs))
	copy(out, s.reqs)
	return out
}

// edgeTestState builds a minimal two-bot visible state for edge tests.
func edgeTestState(matchID string, turn int) *VisibleState {
	return &VisibleState{
		MatchID: matchID,
		Turn:    turn,
		Config:  DefaultConfig(),
		You: struct {
			ID     int `json:"id"`
			Energy int `json:"energy"`
			Score  int `json:"score"`
		}{ID: 0, Energy: 3, Score: 1},
		Bots: []VisibleBot{
			{Position: Position{Row: 5, Col: 5}, Owner: 0},
			{Position: Position{Row: 10, Col: 10}, Owner: 1},
		},
		Energy: []Position{},
		Cores:  []VisibleCore{},
		Walls:  []Position{},
		Dead:   []VisibleBot{},
	}
}

// movesForOwnedBody builds a signed-able response body moving every owned bot north.
func movesForOwnedBody(t *testing.T, state *VisibleState) []byte {
	t.Helper()
	moves := make([]Move, 0)
	for _, b := range state.Bots {
		if b.Owner == state.You.ID {
			moves = append(moves, Move{Position: b.Position, Direction: DirN})
		}
	}
	body, err := json.Marshal(MoveResponse{Moves: moves})
	if err != nil {
		t.Fatalf("marshal moves: %v", err)
	}
	return body
}

// TestProtocolEdge_DefaultTimeoutsAreThreeSeconds pins the documented 3-second
// budget at both layers: the HTTP client used by HTTPBot and the per-turn
// select in MatchRunner.
func TestProtocolEdge_DefaultTimeoutsAreThreeSeconds(t *testing.T) {
	bot := NewHTTPBot("http://127.0.0.1:1", AuthConfig{BotID: "b_edge", Secret: "s", MatchID: "m_edge"})
	if bot.client.Timeout != 3*time.Second {
		t.Errorf("NewHTTPBot default client timeout = %v, want 3s", bot.client.Timeout)
	}

	runner := NewMatchRunner(DefaultConfig())
	if runner.timeout != 3*time.Second {
		t.Errorf("NewMatchRunner default per-turn timeout = %v, want 3s", runner.timeout)
	}
}

// TestProtocolEdge_SlowBotCrossesThreeSecondBoundary drives a real bot past
// the default 3-second budget: the slow response is ignored (units would hold),
// the bot is not crashed, and it moves again on the next turn.
func TestProtocolEdge_SlowBotCrossesThreeSecondBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real 3s-boundary test in short mode")
	}

	secret := "edge-timeout-secret"
	good := func(state *VisibleState) []byte {
		return movesForOwnedBody(t, state)
	}
	s := newProtocolEdgeServer(t, secret, func(state *VisibleState, call int) protocolEdgeAction {
		if call == 0 {
			// Cross the 3s boundary with the bot's DEFAULT (unmodified) client.
			return protocolEdgeAction{delay: 3500 * time.Millisecond, body: good(state)}
		}
		return protocolEdgeAction{body: good(state)}
	})

	bot := NewHTTPBot(s.URL(), AuthConfig{BotID: "b_edge", Secret: secret, MatchID: "m_edge"})

	// Turn 1: bot exceeds 3 seconds — response must be ignored.
	moves, err := bot.GetMoves(edgeTestState("m_edge", 1))
	if err == nil {
		t.Fatal("expected timeout error after crossing the 3s boundary, got nil")
	}
	if len(moves) != 0 {
		t.Errorf("ignored response must yield no moves, got %d", len(moves))
	}
	if bot.failCount != 1 {
		t.Errorf("failCount = %d, want 1", bot.failCount)
	}
	if bot.IsCrashed() {
		t.Error("a single 3s timeout must NOT mark the bot crashed")
	}

	// Turn 2: same bot, prompt response — back in the game.
	moves, err = bot.GetMoves(edgeTestState("m_edge", 2))
	if err != nil {
		t.Fatalf("bot should keep receiving turns after a timeout, got error: %v", err)
	}
	if len(moves) != 1 {
		t.Errorf("recovered turn should produce the bot's move, got %d moves", len(moves))
	}
	if bot.failCount != 0 {
		t.Errorf("failCount should reset on success, got %d", bot.failCount)
	}

	// The slow first request must still have reached the bot exactly once.
	if got := len(s.requests()); got != 2 {
		t.Errorf("server saw %d requests, want 2 (timeout turn included)", got)
	}
}

// TestProtocolEdge_SchemaViolationIsNoOpAndRecovers covers the documented
// "any response that fails schema validation is treated as a no-op" guarantee
// across malformed, empty, wrong-typed, and semantically invalid bodies — each
// correctly signed, so only the schema layer is under test. Every case must
// hold units (error or zero accepted moves), never crash the bot, and be
// followed by a fully recovered turn.
func TestProtocolEdge_SchemaViolationIsNoOpAndRecovers(t *testing.T) {
	secret := "edge-schema-secret"

	cases := []struct {
		name      string
		body      func(state *VisibleState) []byte
		wantErr   bool // true: GetMoves must return an error (failure counted)
		wantMoves int  // accepted moves when err == nil (0 = units hold)
	}{
		{
			name:    "non-json garbage",
			body:    func(*VisibleState) []byte { return []byte("this is not json at all") },
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    func(*VisibleState) []byte { return []byte{} },
			wantErr: true,
		},
		{
			name:      "json null body",
			body:      func(*VisibleState) []byte { return []byte("null") },
			wantMoves: 0,
		},
		{
			name:    "moves field wrong type",
			body:    func(*VisibleState) []byte { return []byte(`{"moves":"north"}`) },
			wantErr: true,
		},
		{
			name:      "moves null",
			body:      func(*VisibleState) []byte { return []byte(`{"moves":null}`) },
			wantMoves: 0,
		},
		{
			name: "unknown direction string",
			body: func(state *VisibleState) []byte {
				return []byte(fmt.Sprintf(`{"moves":[{"position":{"row":%d,"col":%d},"direction":"spin"}]}`,
					state.Bots[0].Position.Row, state.Bots[0].Position.Col))
			},
			wantMoves: 0,
		},
		{
			name: "documented stay direction holds",
			body: func(state *VisibleState) []byte {
				return []byte(fmt.Sprintf(`{"moves":[{"position":{"row":%d,"col":%d},"direction":"stay"}]}`,
					state.Bots[0].Position.Row, state.Bots[0].Position.Col))
			},
			wantMoves: 0,
		},
		{
			name: "out-of-range integer direction",
			body: func(state *VisibleState) []byte {
				return []byte(fmt.Sprintf(`{"moves":[{"position":{"row":%d,"col":%d},"direction":99}]}`,
					state.Bots[0].Position.Row, state.Bots[0].Position.Col))
			},
			wantMoves: 0,
		},
		{
			name: "move for non-owned position only",
			body: func(state *VisibleState) []byte {
				return []byte(fmt.Sprintf(`{"moves":[{"position":{"row":%d,"col":%d},"direction":"N"}]}`,
					state.Bots[1].Position.Row, state.Bots[1].Position.Col))
			},
			wantMoves: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := 0
			s := newProtocolEdgeServer(t, secret, func(state *VisibleState, _ int) protocolEdgeAction {
				call++
				if call == 1 {
					return protocolEdgeAction{body: tc.body(state)}
				}
				return protocolEdgeAction{body: movesForOwnedBody(t, state)}
			})
			bot := NewHTTPBot(s.URL(), AuthConfig{BotID: "b_edge", Secret: secret, MatchID: "m_edge"})

			moves, err := bot.GetMoves(edgeTestState("m_edge", 1))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected schema error, got nil (moves=%d)", len(moves))
				}
				if len(moves) != 0 {
					t.Errorf("failed response must yield no moves, got %d", len(moves))
				}
			} else {
				if err != nil {
					t.Fatalf("expected schema-tolerant no-op (nil error), got: %v", err)
				}
				if len(moves) != tc.wantMoves {
					t.Errorf("got %d moves, want %d (units hold)", len(moves), tc.wantMoves)
				}
			}
			if tc.wantErr {
				// Hard schema failures count toward the 10-failure crash threshold.
				if bot.failCount != 1 {
					t.Errorf("failCount = %d, want 1 after the schema error", bot.failCount)
				}
			} else if bot.failCount != 0 {
				// Filtered no-ops are accepted responses, not failures.
				t.Errorf("failCount = %d, want 0: a tolerated no-op must not count toward the crash threshold", bot.failCount)
			}
			if bot.IsCrashed() {
				t.Error("a single schema violation must NOT crash the bot")
			}

			// Next turn: well-formed signed response — full recovery.
			moves, err = bot.GetMoves(edgeTestState("m_edge", 2))
			if err != nil {
				t.Fatalf("bot should recover on the next turn, got: %v", err)
			}
			if len(moves) != 1 {
				t.Errorf("recovered turn should accept 1 move, got %d", len(moves))
			}
			if bot.failCount != 0 {
				t.Errorf("failCount should reset on recovery, got %d", bot.failCount)
			}
		})
	}
}

// TestProtocolEdge_OversizedDebugHolds pins the §14.1 limit: debug telemetry
// over 10 KB fails the turn (units hold); at-or-under the limit is accepted.
func TestProtocolEdge_OversizedDebugHolds(t *testing.T) {
	secret := "edge-debug-secret"

	debugBody := func(reasoningLen int) func(*VisibleState) []byte {
		return func(*VisibleState) []byte {
			resp := MoveResponse{
				Moves: []Move{{Position: Position{Row: 5, Col: 5}, Direction: DirN}},
				Debug: &DebugInfo{Reasoning: string(bytes.Repeat([]byte("a"), reasoningLen))},
			}
			body, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			return body
		}
	}

	cases := []struct {
		name         string
		reasoningLen int
		wantErr      bool
	}{
		// {"reasoning":"…"} wraps the payload in 16 bytes of JSON punctuation.
		{name: "over 10 KB limit rejected", reasoningLen: 10*1024 + 1 - 16, wantErr: true},
		{name: "exactly 10 KB accepted", reasoningLen: 10*1024 - 16},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := 0
			s := newProtocolEdgeServer(t, secret, func(state *VisibleState, _ int) protocolEdgeAction {
				call++
				if call == 1 {
					return protocolEdgeAction{body: debugBody(tc.reasoningLen)(state)}
				}
				return protocolEdgeAction{body: movesForOwnedBody(t, state)}
			})
			bot := NewHTTPBot(s.URL(), AuthConfig{BotID: "b_edge", Secret: secret, MatchID: "m_edge"})

			moves, err := bot.GetMoves(edgeTestState("m_edge", 1))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected oversized-debug rejection, got nil error")
				}
				if len(moves) != 0 {
					t.Errorf("rejected response must yield no moves, got %d", len(moves))
				}
			} else {
				if err != nil {
					t.Fatalf("debug at the limit should be accepted, got: %v", err)
				}
				if len(moves) != 1 {
					t.Errorf("accepted turn should carry 1 move, got %d", len(moves))
				}
			}
		})
	}
}

// TestProtocolEdge_ResponseSignatureEnforced covers guarantee (3) response
// half: the engine strictly verifies X-ACB-Signature over the response body —
// tampered, foreign-body, wrong-turn, and missing signatures all fail the turn
// (units hold), and the bot recovers once it signs correctly again.
func TestProtocolEdge_ResponseSignatureEnforced(t *testing.T) {
	secret := "edge-sig-secret"

	cases := []struct {
		name   string
		sigFor func(state *VisibleState, body []byte) *string
		omit   bool
	}{
		{
			name: "flipped hex character",
			sigFor: func(state *VisibleState, body []byte) *string {
				sig := SignResponse(secret, state.MatchID, state.Turn, body)
				last := sig[len(sig)-1]
				replacement := byte('0')
				if last == '0' {
					replacement = '1'
				}
				tampered := sig[:len(sig)-1] + string(replacement)
				return &tampered
			},
		},
		{
			name: "signature over different body",
			sigFor: func(state *VisibleState, body []byte) *string {
				sig := SignResponse(secret, state.MatchID, state.Turn, append(append([]byte{}, body...), '!'))
				return &sig
			},
		},
		{
			name: "signature computed for wrong turn",
			sigFor: func(state *VisibleState, body []byte) *string {
				sig := SignResponse(secret, state.MatchID, state.Turn+1, body)
				return &sig
			},
		},
		{
			name: "signature computed for wrong match",
			sigFor: func(state *VisibleState, body []byte) *string {
				sig := SignResponse(secret, "m_other_match", state.Turn, body)
				return &sig
			},
		},
		{
			name: "missing signature header",
			omit: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := 0
			s := newProtocolEdgeServer(t, secret, func(state *VisibleState, _ int) protocolEdgeAction {
				call++
				body := movesForOwnedBody(t, state)
				if call == 1 {
					action := protocolEdgeAction{body: body, omitSig: tc.omit}
					if !tc.omit {
						action.sig = tc.sigFor(state, body)
					}
					return action
				}
				return protocolEdgeAction{body: body}
			})
			bot := NewHTTPBot(s.URL(), AuthConfig{BotID: "b_edge", Secret: secret, MatchID: "m_edge"})

			moves, err := bot.GetMoves(edgeTestState("m_edge", 1))
			if err == nil {
				t.Fatal("expected signature enforcement error, got nil")
			}
			if len(moves) != 0 {
				t.Errorf("rejected response must yield no moves, got %d", len(moves))
			}
			if bot.failCount != 1 {
				t.Errorf("failCount = %d, want 1", bot.failCount)
			}
			if bot.IsCrashed() {
				t.Error("a single signature failure must NOT crash the bot")
			}

			moves, err = bot.GetMoves(edgeTestState("m_edge", 2))
			if err != nil {
				t.Fatalf("bot should recover once it signs correctly, got: %v", err)
			}
			if len(moves) != 1 {
				t.Errorf("recovered turn should accept 1 move, got %d", len(moves))
			}
		})
	}
}

// TestProtocolEdge_EngineRequestSignaturesMatchReadme verifies the request
// half of guarantee (3) end-to-end: a bot that recomputes X-ACB-Signature from
// the raw request exactly as README documents ("To verify an incoming
// request") must accept every request the engine sends, and the identity
// headers must agree with the signed state payload.
func TestProtocolEdge_EngineRequestSignaturesMatchReadme(t *testing.T) {
	secret := "edge-request-secret"

	s := newProtocolEdgeServer(t, secret, func(state *VisibleState, _ int) protocolEdgeAction {
		return protocolEdgeAction{body: movesForOwnedBody(t, state)}
	})
	bot := NewHTTPBot(s.URL(), AuthConfig{BotID: "b_edge_request", Secret: secret, MatchID: "m_edge_req"})

	for turn := 1; turn <= 3; turn++ {
		if _, err := bot.GetMoves(edgeTestState("m_edge_req", turn)); err != nil {
			t.Fatalf("turn %d: GetMoves failed: %v", turn, err)
		}
	}

	reqs := s.requests()
	if len(reqs) != 3 {
		t.Fatalf("server saw %d requests, want 3", len(reqs))
	}

	for i, req := range reqs {
		h := req.Headers
		for _, name := range []string{
			"Content-Type",
			"X-ACB-Match-Id", "X-ACB-Turn", "X-ACB-Timestamp", "X-ACB-Bot-Id", "X-ACB-Signature",
		} {
			if h.Get(name) == "" {
				t.Errorf("request %d: missing %s header", i, name)
			}
		}

		// README recipe: sha256(body) → "{match_id}.{turn}.{body_hash}" → HMAC-SHA256 → hex.
		bodyHash := sha256.Sum256(req.RawBody)
		signingString := fmt.Sprintf("%s.%s.%s",
			h.Get("X-ACB-Match-Id"), h.Get("X-ACB-Turn"), hex.EncodeToString(bodyHash[:]))
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signingString))
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(h.Get("X-ACB-Signature")), []byte(expected)) {
			t.Errorf("request %d: X-ACB-Signature does not match the documented recipe", i)
		}

		// Identity headers must agree with the signed payload.
		if got, want := h.Get("X-ACB-Match-Id"), req.State.MatchID; got != want {
			t.Errorf("request %d: X-ACB-Match-Id = %q, body match_id = %q", i, got, want)
		}
		turnHdr, err := strconv.Atoi(h.Get("X-ACB-Turn"))
		if err != nil {
			t.Errorf("request %d: X-ACB-Turn not an integer: %v", i, err)
		} else if turnHdr != req.State.Turn {
			t.Errorf("request %d: X-ACB-Turn = %d, body turn = %d", i, turnHdr, req.State.Turn)
		}
		ts, err := strconv.ParseInt(h.Get("X-ACB-Timestamp"), 10, 64)
		if err != nil {
			t.Errorf("request %d: X-ACB-Timestamp not an integer: %v", i, err)
		} else {
			skew := time.Since(time.Unix(ts, 0))
			if skew < 0 {
				skew = -skew
			}
			if skew > TimestampTolerance {
				t.Errorf("request %d: timestamp skew %v exceeds tolerance %v", i, skew, TimestampTolerance)
			}
		}
	}
}

// TestProtocolEdge_TenConsecutiveTimeoutsMarkCrashed pins the documented
// escalation: repeated timeouts may mark the bot crashed — at exactly 10
// consecutive failures. Nine failures must leave the bot live and polled; the
// tenth stops HTTP traffic entirely (units hold without disconnecting peers).
func TestProtocolEdge_TenConsecutiveTimeoutsMarkCrashed(t *testing.T) {
	secret := "edge-crash-secret"

	s := newProtocolEdgeServer(t, secret, func(state *VisibleState, call int) protocolEdgeAction {
		return protocolEdgeAction{delay: 200 * time.Millisecond} // always over the 50ms client budget
	})
	bot := NewHTTPBot(s.URL(), AuthConfig{BotID: "b_edge", Secret: secret, MatchID: "m_edge"},
		WithHTTPTimeout(50*time.Millisecond))

	state := edgeTestState("m_edge", 1)

	for i := 1; i <= 9; i++ {
		if _, err := bot.GetMoves(state); err == nil {
			t.Fatalf("failure %d: expected timeout error", i)
		}
		if bot.IsCrashed() {
			t.Fatalf("bot crashed after %d failures, want crash only at 10", i)
		}
	}
	if got := len(s.requests()); got != 9 {
		t.Errorf("server saw %d requests after 9 failures, want 9 (bot still polled)", got)
	}

	// Tenth consecutive failure crosses the threshold.
	if _, err := bot.GetMoves(state); err == nil {
		t.Fatal("expected timeout error on 10th failure")
	}
	if !bot.IsCrashed() {
		t.Fatal("bot should be crashed after 10 consecutive failures")
	}

	// Crashed bot holds silently: empty moves, no error, and no further HTTP traffic.
	before := len(s.requests())
	moves, err := bot.GetMoves(edgeTestState("m_edge", 2))
	if err != nil {
		t.Fatalf("crashed bot must return empty moves without error, got: %v", err)
	}
	if len(moves) != 0 {
		t.Errorf("crashed bot must hold (0 moves), got %d", len(moves))
	}
	if got := len(s.requests()); got != before {
		t.Errorf("crashed bot must not send more HTTP requests: before=%d after=%d", before, got)
	}
}

// protocolEdgeFleeMoves moves each owned bot to the passable adjacent tile
// that maximizes total squared distance from visible enemies. Returns moves
// for the engine's position-addressed Move schema.
func protocolEdgeFleeMoves(state *VisibleState) []Move {
	passable := func(dest Position) bool {
		if dest.Row < 0 || dest.Row >= state.Config.Rows || dest.Col < 0 || dest.Col >= state.Config.Cols {
			return false
		}
		for _, w := range state.Walls {
			if w == dest {
				return false
			}
		}
		return true
	}

	moves := make([]Move, 0)
	for _, b := range state.Bots {
		if b.Owner != state.You.ID {
			continue
		}
		best := DirNone
		bestScore := -1
		for _, d := range []Direction{DirN, DirE, DirS, DirW} {
			dr, dc := d.Delta()
			dest := Position{Row: b.Position.Row + dr, Col: b.Position.Col + dc}
			if !passable(dest) {
				continue
			}
			score := 0
			for _, e := range state.Bots {
				if e.Owner == state.You.ID {
					continue
				}
				edr := dest.Row - e.Position.Row
				edc := dest.Col - e.Position.Col
				score += edr*edr + edc*edc
			}
			if score > bestScore {
				bestScore = score
				best = d
			}
		}
		if best != DirNone {
			moves = append(moves, Move{Position: b.Position, Direction: best})
		}
	}
	return moves
}

func protocolEdgeFleeBody(t *testing.T, state *VisibleState) []byte {
	t.Helper()
	body, err := json.Marshal(MoveResponse{Moves: protocolEdgeFleeMoves(state)})
	if err != nil {
		t.Fatalf("marshal flee moves: %v", err)
	}
	return body
}

func protocolEdgeBotPositions(t *testing.T, replay *Replay, turnIdx, owner int) map[int]Position {
	t.Helper()
	if turnIdx >= len(replay.Turns) {
		t.Fatalf("replay has %d turns, want index %d", len(replay.Turns), turnIdx)
	}
	out := make(map[int]Position)
	for _, b := range replay.Turns[turnIdx].Bots {
		if b.Owner == owner && b.Alive {
			out[b.ID] = b.Position
		}
	}
	return out
}

// TestProtocolEdge_MatchHoldsOnBadTurnsAndRecovers runs a real two-player
// match in which player 1's bot misbehaves for three consecutive turns —
// malformed body, then a stall over its HTTP budget, then a tampered
// signature — and behaves afterwards. Player 1's units must hold through the
// bad stretch and move again after recovery, the bot must keep receiving
// every turn, and neither side may be marked crashed.
func TestProtocolEdge_MatchHoldsOnBadTurnsAndRecovers(t *testing.T) {
	secret := "edge-match-secret"

	// Player 0: always well-formed, signed, fleeing moves.
	s0 := newProtocolEdgeServer(t, secret, func(state *VisibleState, _ int) protocolEdgeAction {
		return protocolEdgeAction{body: protocolEdgeFleeBody(t, state)}
	})

	// Player 1: misbehaves on state turns 0-2, well-behaved from turn 3 on.
	s1 := newProtocolEdgeServer(t, secret, func(state *VisibleState, _ int) protocolEdgeAction {
		body := protocolEdgeFleeBody(t, state)
		switch state.Turn {
		case 0: // schema violation, correctly signed → parse error → hold
			return protocolEdgeAction{body: []byte("}not json{")}
		case 1: // stall past the bot's own 120ms HTTP budget → timeout → hold
			return protocolEdgeAction{delay: 500 * time.Millisecond, body: body}
		case 2: // tampered signature → verification failure → hold
			sig := SignResponse(secret, state.MatchID, state.Turn, append(append([]byte{}, body...), ' '))
			return protocolEdgeAction{body: body, sig: &sig}
		default:
			return protocolEdgeAction{body: body}
		}
	})

	config := DefaultConfig()
	config.Rows = 40
	config.Cols = 40
	config.CoresPerPlayer = 1
	config.MaxTurns = 6

	runner := NewMatchRunner(config,
		WithRNG(rand.New(rand.NewSource(42))),
		WithTimeout(2*time.Second),
	)
	runner.AddBot(NewHTTPBot(s0.URL(), AuthConfig{BotID: "b_0", Secret: secret, MatchID: "m_edge_match"},
		WithHTTPTimeout(2*time.Second)), "well-behaved")
	runner.AddBot(NewHTTPBot(s1.URL(), AuthConfig{BotID: "b_1", Secret: secret, MatchID: "m_edge_match"},
		WithHTTPTimeout(120*time.Millisecond)), "misbehaving-then-recovering")

	result, replay, err := runner.Run()
	if err != nil {
		t.Fatalf("match failed: %v", err)
	}

	for i, crashed := range result.Crashed {
		if crashed {
			t.Errorf("player %d marked crashed — protocol misbehavior must not kill a bot before 10 consecutive failures", i)
		}
	}

	// Player 1's bot kept receiving every turn (not disconnected).
	if got := len(s1.requests()); got != config.MaxTurns {
		t.Errorf("misbehaving bot received %d turn requests, want %d", got, config.MaxTurns)
	}

	// Replay turn N is the state after executing the moves fetched while
	// state.Turn == N-1, so bad fetch turns 0/1/2 hold replay turns 1/2/3 and
	// the recovery fetch at state turn 3 moves replay turn 4.
	p1 := func(idx int) map[int]Position {
		return protocolEdgeBotPositions(t, replay, idx, 1)
	}
	for _, idx := range []int{1, 2, 3} {
		for id, pos := range p1(idx) {
			prev, ok := p1(idx - 1)[id]
			if !ok {
				t.Fatalf("turn %d: player 1 bot %d missing from previous turn", idx, id)
			}
			if pos != prev {
				t.Errorf("turn %d: player 1 bot %d moved to %v during the bad stretch (turn %d), want hold at %v",
					idx, id, pos, idx-1, prev)
			}
		}
	}
	if len(p1(3)) == 0 {
		t.Fatal("player 1 has no living bots at turn 3 — test precondition lost")
	}
	for id, pos := range p1(4) {
		if pos == p1(3)[id] {
			t.Errorf("turn 4: player 1 bot %d still held after recovering (turn 3 fetch was well-behaved)", id)
		}
	}

	// Player 0 must be unaffected: its bot advances every turn.
	for idx := 1; idx <= 4; idx++ {
		for id, pos := range protocolEdgeBotPositions(t, replay, idx, 0) {
			prev := protocolEdgeBotPositions(t, replay, idx-1, 0)[id]
			if pos == prev {
				t.Errorf("turn %d: player 0 bot %d did not move — misbehaving opponent leaked into the healthy bot", idx, id)
			}
		}
	}
}
