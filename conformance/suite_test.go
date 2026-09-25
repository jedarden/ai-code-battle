package conformance

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aicodebattle/acb/engine"
)

// independentSign computes the doc's request signature without touching any
// of this package's or the engine's helpers, so a shared bug cannot hide.
func independentSign(t *testing.T, secret, matchID, turn, timestamp string, body []byte) string {
	t.Helper()
	bodyHash := sha256.Sum256(body)
	payload := matchID + "." + turn + "." + timestamp + "." + hex.EncodeToString(bodyHash[:])
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// TestGoldenVectors pins the worked examples from docs/bot-protocol.md and
// cross-checks the engine's own signer against them.
func TestGoldenVectors(t *testing.T) {
	const (
		secret          = DefaultConformanceSecret
		matchID         = "m_conformance"
		body            = `{"match_id":"m_conformance","turn":7}`
		wantPayload     = "m_conformance.7.1711200000.866d18ae5b374c10ae5cca15e0c2f0a21b58bff2ccd1a1d2722879080e92cf56"
		wantRequestSig  = "e034fde18f2f4416b66b1d2357b280ed82f3deca0fd880e035ed709cd21622fc"
		responseBody    = `{"moves":[]}`
		wantRespPayload = "m_conformance.7.4cba52032dfb0839b8138eb84ebcb5bd281253019dfc6efb447d211ac4333f8e"
		wantResponseSig = "82fe0af54ea110caded8f3147065992368d04cedadb6bb2441daf3d25cb6a1ee"
	)

	if got := CanonicalRequestPayloadString(matchID, "7", "1711200000", []byte(body)); got != wantPayload {
		t.Fatalf("CanonicalRequestPayloadString() = %q, want doc payload %q", got, wantPayload)
	}
	if got := independentSign(t, secret, matchID, "7", "1711200000", []byte(body)); got != wantRequestSig {
		t.Fatalf("independent signer = %q, want doc signature %q", got, wantRequestSig)
	}
	if got := SignRequestString(secret, matchID, "7", "1711200000", []byte(body)); got != wantRequestSig {
		t.Fatalf("SignRequestString() = %q, want doc signature %q", got, wantRequestSig)
	}

	if got := CanonicalResponsePayload(matchID, 7, []byte(responseBody)); got != wantRespPayload {
		t.Fatalf("CanonicalResponsePayload() = %q, want doc payload %q", got, wantRespPayload)
	}
	if got := engine.SignResponse(secret, matchID, 7, []byte(responseBody)); got != wantResponseSig {
		t.Fatalf("engine.SignResponse() = %q, want doc signature %q", got, wantResponseSig)
	}

	// The engine's canonical signer must agree byte-for-byte with the
	// suite's raw-string signer on canonical input.
	if engine.CanonicalRequestPayload(matchID, 7, 1711200000, []byte(body)) != wantPayload {
		t.Fatal("engine.CanonicalRequestPayload disagrees with the doc payload")
	}
	if got := engine.SignRequest(secret, matchID, 7, 1711200000, []byte(body)); got != wantRequestSig {
		t.Fatalf("engine.SignRequest() = %q, want doc signature %q", got, wantRequestSig)
	}
}

// TestBuildCasesInvariants checks the case table is internally consistent:
// names unique, expectations stay inside the documented outcome classes, and
// signatures verify over the exact bytes each case sends — except where the
// signature itself is the thing under test.
func TestBuildCasesInvariants(t *testing.T) {
	cases := BuildCases(DefaultConformanceSecret, time.Now())
	seen := map[string]bool{}
	for _, c := range cases {
		if seen[c.Name] {
			t.Fatalf("duplicate case name %q", c.Name)
		}
		seen[c.Name] = true

		if c.Path == "/health" {
			if c.Signed || c.Want4xx || c.AcceptOr401 || c.WantStatus != http.StatusOK {
				t.Fatalf("%s: health case must want a plain 200", c.Name)
			}
			continue
		}

		switch {
		case c.Signed:
			if c.WantStatus != http.StatusOK || c.AcceptOr401 || c.Want4xx {
				t.Fatalf("%s: positive case wants status %d", c.Name, c.WantStatus)
			}
		case c.AcceptOr401:
			if c.WantStatus != 0 || c.Want4xx {
				t.Fatalf("%s: dual-outcome case must not pin a status", c.Name)
			}
		case c.Want4xx:
			// fine: doc leaves the code to the implementation
		case c.WantStatus == http.StatusUnauthorized || c.WantStatus == http.StatusBadRequest:
			// the two documented rejection classes
		default:
			t.Fatalf("%s: rejection wants status %d, outside the documented 400/401 classes", c.Name, c.WantStatus)
		}

		headers := c.Headers
		if headers["X-ACB-Match-Id"] == "" || headers["X-ACB-Turn"] == "" || headers["X-ACB-Timestamp"] == "" {
			continue // missing-header cases have nothing to re-verify
		}
		want := independentSign(t, DefaultConformanceSecret,
			headers["X-ACB-Match-Id"], headers["X-ACB-Turn"], headers["X-ACB-Timestamp"], c.Body)
		got := headers["X-ACB-Signature"]
		switch c.Name {
		case "reject_tampered_signature", "reject_signature_uppercase",
			"reject_signature_wrong_length", "reject_signature_not_hex",
			"reject_missing_header_signature":
			if got == want {
				t.Fatalf("%s: signature was supposed to be broken", c.Name)
			}
		default:
			if got != want {
				t.Fatalf("%s: case signature %q does not verify over its own bytes (want %q) — fixture bug", c.Name, got, want)
			}
		}
	}
}

// TestSuiteAgainstReference proves the suite accepts a fully conformant bot.
func TestSuiteAgainstReference(t *testing.T) {
	server := httptest.NewServer(ReferenceBotHandler(DefaultConformanceSecret))
	t.Cleanup(server.Close)
	result := RunSuite(t.Context(), SuiteClient(5*time.Second), server.URL, DefaultConformanceSecret)
	if len(result.Failed) != 0 {
		for _, f := range result.Failed {
			t.Errorf("%s: %s", f.Case, f.Detail)
		}
		t.Fatalf("reference bot failed %d of %d cases", len(result.Failed), result.Passed+len(result.Failed))
	}
}

// laxResponse writes the 200/moves/signed reply a non-strict bot would,
// signing the response with the parsed integer turn the way a real acceptor
// canonicalizes before signing.
func laxResponse(w http.ResponseWriter, secret, matchID, turn string) {
	parsedTurn, err := strconv.Atoi(turn)
	if err != nil {
		parsedTurn = 0
	}
	response := []byte(`{"moves":[]}`)
	hash := sha256.Sum256(response)
	payload := matchID + "." + strconv.Itoa(parsedTurn) + "." + hex.EncodeToString(hash[:])
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-ACB-Signature", hex.EncodeToString(mac.Sum(nil)))
	_, _ = w.Write(response)
}

// mutantHandler serves the reference bot except for requests where shouldLax
// returns true; those get a valid signed 200 the way a bot with the matching
// conformance hole would answer.
func mutantHandler(secret string, shouldLax func(r *http.Request, body []byte) bool) http.Handler {
	reference := ReferenceBotHandler(secret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/turn" && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err == nil {
				r.Body = io.NopCloser(bytes.NewReader(body))
				if shouldLax(r, body) {
					laxResponse(w, secret,
						r.Header.Get("X-ACB-Match-Id"),
						r.Header.Get("X-ACB-Turn"))
					return
				}
			}
		}
		reference.ServeHTTP(w, r)
	})
}

var knownTopLevel = map[string]bool{
	"match_id": true, "turn": true, "config": true, "you": true, "bots": true,
	"energy": true, "cores": true, "walls": true, "dead": true, "zone": true,
}

var knownConfig = map[string]bool{
	"rows": true, "cols": true, "max_turns": true, "vision_radius2": true,
	"attack_radius2": true, "spawn_cost": true, "energy_interval": true,
	"cores_per_player": true, "map_id": true, "season_id": true,
	"rules_version": true, "turn_timeout": true, "zone_enabled": true,
	"zone_start_turn": true, "zone_shrink_interval": true, "zone_shrink_step": true,
	"zone_min_radius": true, "kill_score": true,
}

var requiredTopLevel = []string{"match_id", "turn", "config", "you", "bots", "energy", "cores", "walls", "dead"}

func bodyHasUnknownField(body []byte) bool {
	var state map[string]interface{}
	if err := json.Unmarshal(body, &state); err != nil {
		return false
	}
	for key := range state {
		if !knownTopLevel[key] {
			return true
		}
	}
	knownNested := map[string]map[string]bool{
		"config": knownConfig,
		"you":    {"id": true, "energy": true, "score": true},
		"zone":   {"center": true, "radius": true, "active": true},
	}
	for parent, known := range knownNested {
		obj, ok := state[parent].(map[string]interface{})
		if !ok {
			continue
		}
		for key := range obj {
			if !known[key] {
				return true
			}
		}
	}
	return false
}

func bodyMissingRequired(body []byte) bool {
	var state map[string]json.RawMessage
	if err := json.Unmarshal(body, &state); err != nil {
		return false
	}
	for _, key := range requiredTopLevel {
		if _, ok := state[key]; !ok {
			return true
		}
	}
	return false
}

func bodyTurnWrongType(body []byte) bool {
	var state map[string]json.RawMessage
	if err := json.Unmarshal(body, &state); err != nil {
		return false
	}
	raw, ok := state["turn"]
	if !ok {
		return false
	}
	return json.Unmarshal(raw, new(int)) != nil
}

func headerNonCanonical(value string) bool { return !canonicalBase10(value) }

// redirectingHealth answers /health with a 302 the way a misconfigured
// reverse-proxy-fronted bot would.
func redirectingHealth(secret string) http.Handler {
	reference := ReferenceBotHandler(secret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			http.Redirect(w, r, "/healthz", http.StatusFound)
			return
		}
		reference.ServeHTTP(w, r)
	})
}

// TestSuiteDetectsNonConformance proves each class of conformance hole is
// caught by exactly the cases that name it — the suite must have teeth.
func TestSuiteDetectsNonConformance(t *testing.T) {
	secret := DefaultConformanceSecret
	tests := []struct {
		name    string
		handler http.Handler
		want    []string
	}{
		{
			name: "accepts unknown fields",
			handler: mutantHandler(secret, func(_ *http.Request, body []byte) bool {
				return bodyHasUnknownField(body) || bodyMissingRequired(body) || bodyTurnWrongType(body)
			}),
			want: []string{
				"reject_unknown_top_level_field", "reject_unknown_config_field",
				"reject_unknown_you_field", "reject_unknown_zone_field",
				"reject_missing_required_field_walls", "reject_missing_body_match_id",
				"reject_wrong_type_turn",
			},
		},
		{
			name: "accepts uppercase signature hex",
			handler: mutantHandler(secret, func(r *http.Request, body []byte) bool {
				return r.Header.Get("X-ACB-Signature") != strings.ToLower(r.Header.Get("X-ACB-Signature"))
			}),
			want: []string{"reject_signature_uppercase"},
		},
		{
			// A bot that neither requires the signature header nor checks
			// its format or value, but otherwise behaves like the reference:
			// requests whose only defect is the signature are accepted;
			// everything else falls through to the reference's own checks.
			name: "ignores the request signature entirely",
			handler: mutantHandler(secret, func(r *http.Request, body []byte) bool {
				if r.Header.Get("Content-Type") != "application/json" ||
					r.Header.Get("X-ACB-Match-Id") == "" ||
					r.Header.Get("X-ACB-Turn") == "" ||
					r.Header.Get("X-ACB-Timestamp") == "" ||
					r.Header.Get("X-ACB-Bot-Id") == "" {
					return false
				}
				if !canonicalBase10(r.Header.Get("X-ACB-Turn")) || !canonicalBase10(r.Header.Get("X-ACB-Timestamp")) {
					return false
				}
				ts, err := strconv.ParseInt(r.Header.Get("X-ACB-Timestamp"), 10, 64)
				if err != nil {
					return false
				}
				if now := time.Now(); ts < now.Add(-30*time.Second).Unix() || ts > now.Add(30*time.Second).Unix() {
					return false
				}
				want := SignRequestString(secret,
					r.Header.Get("X-ACB-Match-Id"),
					r.Header.Get("X-ACB-Turn"),
					r.Header.Get("X-ACB-Timestamp"),
					body)
				return r.Header.Get("X-ACB-Signature") != want
			}),
			want: []string{
				"reject_tampered_signature", "reject_signature_uppercase",
				"reject_signature_wrong_length", "reject_signature_not_hex",
				"reject_missing_header_signature",
			},
		},
		{
			name: "skips the content type check",
			handler: mutantHandler(secret, func(r *http.Request, _ []byte) bool {
				return r.Header.Get("Content-Type") != "application/json"
			}),
			want: []string{"reject_missing_content_type", "reject_wrong_content_type"},
		},
		{
			name:    "redirects health",
			handler: redirectingHealth(secret),
			want:    []string{"health_direct_200"},
		},
		{
			name: "re-serializes the body before hashing",
			// A reference bot whose only flaw is decoding the body and
			// hashing its own canonical re-encoding instead of the exact
			// received bytes. Byte-identical re-encodings behave correctly,
			// so only the byte-variant positive cases expose it.
			handler: func() http.Handler {
				reference := ReferenceBotHandler(secret)
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, "failed to read body", http.StatusBadRequest)
						return
					}
					var state map[string]interface{}
					if json.Unmarshal(body, &state) != nil {
						r.Body = io.NopCloser(bytes.NewReader(body))
						reference.ServeHTTP(w, r)
						return
					}
					reserialized, _ := json.Marshal(state)
					r.Body = io.NopCloser(bytes.NewReader(reserialized))
					reference.ServeHTTP(w, r)
				})
			}(),
			want: []string{
				"positive_trailing_newline_bytes_signed",
				"positive_pretty_bytes_signed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			t.Cleanup(server.Close)
			result := RunSuite(t.Context(), SuiteClient(5*time.Second), server.URL, secret)
			gotFailed := map[string]string{}
			for _, f := range result.Failed {
				gotFailed[f.Case] = f.Detail
			}
			for _, want := range tt.want {
				if _, ok := gotFailed[want]; !ok {
					t.Errorf("mutant %q was not caught by case %s", tt.name, want)
				}
			}
			for caseName := range gotFailed {
				expected := false
				for _, want := range tt.want {
					if want == caseName {
						expected = true
						break
					}
				}
				if !expected {
					t.Errorf("mutant %q unexpectedly failed case %s: %s", tt.name, caseName, gotFailed[caseName])
				}
			}
		})
	}
}

// laxSpellingAcceptor models the second documented handling of non-canonical
// spellings: it verifies the signature over the exact received header
// strings, parses both integers leniently, and executes. The doc says a bot
// may reject non-canonical values or accept them this way, so the suite must
// pass against both the reference bot (strict rejector) and this acceptor.
func laxSpellingAcceptor(secret string) http.Handler {
	reference := ReferenceBotHandler(secret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turn" || r.Method != http.MethodPost {
			reference.ServeHTTP(w, r)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			reference.ServeHTTP(w, r)
			return
		}
		matchID := r.Header.Get("X-ACB-Match-Id")
		turnStr := strings.TrimSpace(r.Header.Get("X-ACB-Turn"))
		timestamp := r.Header.Get("X-ACB-Timestamp")
		if matchID == "" || turnStr == "" || timestamp == "" ||
			r.Header.Get("X-ACB-Bot-Id") == "" || r.Header.Get("X-ACB-Signature") == "" {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		turn, err := strconv.ParseInt(turnStr, 10, 64)
		ts, err2 := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || err2 != nil || turn < 0 || ts < 0 {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		if now := time.Now(); ts < now.Add(-30*time.Second).Unix() || ts > now.Add(30*time.Second).Unix() {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		// Signature over the exact received spellings.
		if SignRequestString(secret, matchID, turnStr, timestamp, raw) != r.Header.Get("X-ACB-Signature") {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		var identity struct {
			MatchID string `json:"match_id"`
			Turn    int    `json:"turn"`
		}
		if json.Unmarshal(raw, &identity) != nil || identity.MatchID != matchID || identity.Turn != int(turn) {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		laxResponse(w, secret, matchID, turnStr)
	})
}

// TestSpellingHandlingBothDocumentedWays pins that the non-canonical-spelling
// cases accept BOTH documented bot behaviors: strict rejection (the reference
// bot) and lenient acceptance over exact received bytes. A suite that only
// passed one reading would over-specify the contract.
func TestSpellingHandlingBothDocumentedWays(t *testing.T) {
	secret := DefaultConformanceSecret
	spellings := []string{
		"spelling_timestamp_leading_zero", "spelling_timestamp_plus_prefix",
		"spelling_turn_leading_zero", "spelling_turn_plus_prefix",
		"spelling_turn_whitespace",
	}
	for _, name := range []struct {
		label   string
		handler http.Handler
	}{
		{"strict rejector", ReferenceBotHandler(secret)},
		{"lenient acceptor", laxSpellingAcceptor(secret)},
	} {
		t.Run(name.label, func(t *testing.T) {
			server := httptest.NewServer(name.handler)
			t.Cleanup(server.Close)
			result := RunSuite(t.Context(), SuiteClient(5*time.Second), server.URL, secret)
			failed := map[string]string{}
			for _, f := range result.Failed {
				failed[f.Case] = f.Detail
			}
			for _, spelling := range spellings {
				if detail, ok := failed[spelling]; ok {
					t.Errorf("%s: %s handling rejected a conformant outcome: %s", spelling, name.label, detail)
				}
			}
			// Both handlers must still REJECT the never-acceptable pair: "08"
			// spells 8 and the body says 7, a body/header identity mismatch
			// the doc requires not to execute. The case passes exactly when
			// the bot rejected it; a failure here means the handler executed.
			if detail, ok := failed["reject_turn_spelling_value_mismatch"]; ok {
				t.Errorf("%s executed a body/header identity mismatch (must reject): %s", name.label, detail)
			}
		})
	}
}
