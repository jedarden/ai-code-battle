package engine

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type botProtocolConformanceRequest struct {
	MatchID *string                              `json:"match_id"`
	Turn    *int                                 `json:"turn"`
	Config  *botProtocolConformanceConfig        `json:"config"`
	You     *botProtocolConformancePlayer        `json:"you"`
	Bots    *[]botProtocolConformanceVisibleBot  `json:"bots"`
	Energy  *[]botProtocolConformancePosition    `json:"energy"`
	Cores   *[]botProtocolConformanceVisibleCore `json:"cores"`
	Walls   *[]botProtocolConformancePosition    `json:"walls"`
	Dead    *[]botProtocolConformanceVisibleBot  `json:"dead"`
	Zone    *botProtocolConformanceZone          `json:"zone,omitempty"`
}

type botProtocolConformanceConfig struct {
	Rows               *int    `json:"rows"`
	Cols               *int    `json:"cols"`
	MaxTurns           *int    `json:"max_turns"`
	VisionRadius2      *int    `json:"vision_radius2"`
	AttackRadius2      *int    `json:"attack_radius2"`
	SpawnCost          *int    `json:"spawn_cost"`
	EnergyInterval     *int    `json:"energy_interval"`
	CoresPerPlayer     *int    `json:"cores_per_player"`
	MapID              *string `json:"map_id,omitempty"`
	SeasonID           *string `json:"season_id,omitempty"`
	RulesVersion       *string `json:"rules_version,omitempty"`
	TurnTimeout        *int64  `json:"turn_timeout,omitempty"`
	ZoneEnabled        *bool   `json:"zone_enabled"`
	ZoneStartTurn      *int    `json:"zone_start_turn"`
	ZoneShrinkInterval *int    `json:"zone_shrink_interval"`
	ZoneShrinkStep     *int    `json:"zone_shrink_step"`
	ZoneMinRadius      *int    `json:"zone_min_radius"`
	KillScore          *int    `json:"kill_score"`
}

type botProtocolConformancePlayer struct {
	ID     *int `json:"id"`
	Energy *int `json:"energy"`
	Score  *int `json:"score"`
}

type botProtocolConformanceVisibleBot struct {
	Position *botProtocolConformancePosition `json:"position"`
	Owner    *int                            `json:"owner"`
}

type botProtocolConformanceVisibleCore struct {
	Position *botProtocolConformancePosition `json:"position"`
	Owner    *int                            `json:"owner"`
	Active   *bool                           `json:"active"`
}

type botProtocolConformancePosition struct {
	Row *int `json:"row"`
	Col *int `json:"col"`
}

type botProtocolConformanceZone struct {
	Center *botProtocolConformancePosition `json:"center"`
	Radius *int                            `json:"radius"`
	Active *bool                           `json:"active"`
}

type botProtocolConformanceObservation struct {
	method  string
	path    string
	headers http.Header
	body    []byte
	err     error
}

func TestBotProtocolConformance_ExactSignatureVectors(t *testing.T) {
	const (
		secret          = "sëcret-🔑"
		matchID         = "m_conformance"
		wantRequestSig  = "e034fde18f2f4416b66b1d2357b280ed82f3deca0fd880e035ed709cd21622fc"
		wantResponseSig = "82fe0af54ea110caded8f3147065992368d04cedadb6bb2441daf3d25cb6a1ee"
	)
	requestBody := []byte(`{"match_id":"m_conformance","turn":7}`)
	wantRequestPayload := "m_conformance.7.1711200000.866d18ae5b374c10ae5cca15e0c2f0a21b58bff2ccd1a1d2722879080e92cf56"
	if got := CanonicalRequestPayload(matchID, 7, 1711200000, requestBody); got != wantRequestPayload {
		t.Fatalf("CanonicalRequestPayload() = %q, want %q", got, wantRequestPayload)
	}
	if got := SignRequest(secret, matchID, 7, 1711200000, requestBody); got != wantRequestSig {
		t.Fatalf("SignRequest() = %q, want %q", got, wantRequestSig)
	}
	if !verifySignature(secret, wantRequestPayload, wantRequestSig) {
		t.Fatal("fixed request signature did not verify")
	}

	responseBody := []byte(`{"moves":[]}`)
	wantResponsePayload := "m_conformance.7.4cba52032dfb0839b8138eb84ebcb5bd281253019dfc6efb447d211ac4333f8e"
	if got := CanonicalResponsePayload(matchID, 7, responseBody); got != wantResponsePayload {
		t.Fatalf("CanonicalResponsePayload() = %q, want %q", got, wantResponsePayload)
	}
	if got := SignResponse(secret, matchID, 7, responseBody); got != wantResponseSig {
		t.Fatalf("SignResponse() = %q, want %q", got, wantResponseSig)
	}
	if err := VerifyResponse(secret, matchID, 7, wantResponseSig, responseBody); err != nil {
		t.Fatalf("fixed response signature did not verify: %v", err)
	}
	if err := VerifyResponse(secret, matchID, 7, strings.ToUpper(wantResponseSig), responseBody); err == nil {
		t.Fatal("uppercase response signature was accepted")
	}

	reformatted := []byte(` { "turn": 7, "match_id": "m_conformance" } `)
	if SignRequest(secret, matchID, 7, 1711200000, reformatted) == wantRequestSig {
		t.Fatal("semantically equivalent JSON with different raw bytes reused the signature")
	}
}

func TestBotProtocolConformance_Health(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			observed := make(chan botProtocolConformanceObservation, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				observed <- botProtocolConformanceObservation{
					method:  r.Method,
					path:    r.URL.Path,
					headers: r.Header.Clone(),
					body:    body,
					err:     err,
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte("health"))
			}))
			t.Cleanup(server.Close)

			bot := NewHTTPBot(server.URL, AuthConfig{BotID: "b_health", Secret: "secret", MatchID: "m_health"})
			err := bot.Health()
			if status == http.StatusOK && err != nil {
				t.Fatalf("Health() returned %v for HTTP 200", err)
			}
			if status != http.StatusOK && err == nil {
				t.Fatalf("Health() accepted HTTP %d", status)
			}

			var request botProtocolConformanceObservation
			select {
			case request = <-observed:
			case <-time.After(time.Second):
				t.Fatal("health request was not observed")
			}
			if request.err != nil {
				t.Fatalf("read health request body: %v", request.err)
			}
			if request.method != http.MethodGet || request.path != "/health" {
				t.Errorf("health request = %s %s, want GET /health", request.method, request.path)
			}
			if len(request.body) != 0 {
				t.Errorf("health request body = %q, want empty", request.body)
			}
			for _, name := range []string{"Content-Type", "X-ACB-Match-Id", "X-ACB-Turn", "X-ACB-Timestamp", "X-ACB-Bot-Id", "X-ACB-Signature"} {
				if value := request.headers.Get(name); value != "" {
					t.Errorf("health request unexpectedly carried %s: %q", name, value)
				}
			}
		})
	}

	t.Run("redirect", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				http.Redirect(w, r, server.URL+"/ready", http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		bot := NewHTTPBot(server.URL, AuthConfig{BotID: "b_health", Secret: "secret", MatchID: "m_health"})
		if err := bot.Health(); err == nil {
			t.Fatal("Health() followed a redirect instead of requiring a direct 200 response")
		}
	})
}

func TestBotProtocolConformance_SignedTurnRequest(t *testing.T) {
	const (
		secret  = "conformance-request-secret"
		botID   = "b_conformance"
		matchID = "m_conformance"
	)
	responseBody := []byte(`{"moves":[{"position":{"row":5,"col":5},"direction":"N"}]}`)
	observed := make(chan botProtocolConformanceObservation, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		observed <- botProtocolConformanceObservation{
			method:  r.Method,
			path:    r.URL.Path,
			headers: r.Header.Clone(),
			body:    body,
			err:     err,
		}
		turn, parseErr := strconv.Atoi(r.Header.Get("X-ACB-Turn"))
		if parseErr != nil {
			t.Errorf("X-ACB-Turn is not an integer: %v", parseErr)
		}
		signature := SignResponse(secret, r.Header.Get("X-ACB-Match-Id"), turn, responseBody)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-ACB-Signature", signature)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(server.Close)

	bot := NewHTTPBot(server.URL, AuthConfig{BotID: botID, Secret: secret, MatchID: matchID})
	moves, err := bot.GetMoves(botProtocolConformanceVisibleState(t, matchID, 7))
	if err != nil {
		t.Fatalf("GetMoves() error = %v", err)
	}
	if len(moves) != 1 || moves[0].Position != (Position{Row: 5, Col: 5}) || moves[0].Direction != DirN {
		t.Fatalf("GetMoves() = %#v, want the signed N move", moves)
	}

	request := <-observed
	if request.err != nil {
		t.Fatalf("read turn request body: %v", request.err)
	}
	if request.method != http.MethodPost || request.path != "/turn" {
		t.Errorf("turn request = %s %s, want POST /turn", request.method, request.path)
	}
	required := map[string]string{
		"Content-Type":   "application/json",
		"X-ACB-Match-Id": matchID,
		"X-ACB-Turn":     "7",
		"X-ACB-Bot-Id":   botID,
	}
	for name, want := range required {
		if got := request.headers.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	timestamp, err := strconv.ParseInt(request.headers.Get("X-ACB-Timestamp"), 10, 64)
	if err != nil {
		t.Fatalf("X-ACB-Timestamp is not an integer: %v", err)
	}
	skew := time.Since(time.Unix(timestamp, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > TimestampTolerance {
		t.Errorf("request timestamp skew = %v, want <= %v", skew, TimestampTolerance)
	}

	bodyHash := sha256.Sum256(request.body)
	canonical := fmt.Sprintf("%s.%s.%s.%s",
		request.headers.Get("X-ACB-Match-Id"),
		request.headers.Get("X-ACB-Turn"),
		request.headers.Get("X-ACB-Timestamp"),
		hex.EncodeToString(bodyHash[:]),
	)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	wantSignature := hex.EncodeToString(mac.Sum(nil))
	gotSignature := request.headers.Get("X-ACB-Signature")
	if gotSignature != strings.ToLower(gotSignature) {
		t.Errorf("X-ACB-Signature = %q, want lowercase hex", gotSignature)
	}
	if !hmac.Equal([]byte(gotSignature), []byte(wantSignature)) {
		t.Errorf("X-ACB-Signature = %q, want independent HMAC result %q", gotSignature, wantSignature)
	}
	auth, err := ParseAuthHeaders(map[string]string{
		"X-ACB-Match-Id":  request.headers.Get("X-ACB-Match-Id"),
		"X-ACB-Turn":      request.headers.Get("X-ACB-Turn"),
		"X-ACB-Timestamp": request.headers.Get("X-ACB-Timestamp"),
		"X-ACB-Bot-Id":    request.headers.Get("X-ACB-Bot-Id"),
		"X-ACB-Signature": gotSignature,
	})
	if err != nil {
		t.Fatalf("ParseAuthHeaders() error = %v", err)
	}
	if err := VerifyRequest(secret, auth, request.body); err != nil {
		t.Fatalf("VerifyRequest() rejected the engine request: %v", err)
	}
	if err := validateBotProtocolConformanceRequest(request.body); err != nil {
		t.Fatalf("engine emitted a non-conformant request schema: %v", err)
	}
}

func TestBotProtocolConformance_TurnRedirectRejected(t *testing.T) {
	const secret = "conformance-redirect-secret"
	requests := make(chan struct{}, 2)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		if r.URL.Path == "/turn" {
			http.Redirect(w, r, server.URL+"/redirected", http.StatusTemporaryRedirect)
			return
		}
		http.Error(w, "redirect followed", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	bot := NewHTTPBot(server.URL, AuthConfig{BotID: "b_redirect", Secret: secret, MatchID: "m_redirect"})
	moves, err := bot.GetMoves(botProtocolConformanceState("m_redirect", 3))
	if err == nil {
		t.Fatal("GetMoves() followed a /turn redirect")
	}
	if len(moves) != 0 {
		t.Errorf("redirect rejection returned %d moves, want 0", len(moves))
	}
	if got := len(requests); got != 1 {
		t.Errorf("redirected request count = %d, want 1", got)
	}
}

func TestBotProtocolConformance_RequestAuthentication(t *testing.T) {
	const secret = "conformance-auth-secret"

	rewrite := func(body []byte, change func(map[string]interface{})) []byte {
		t.Helper()
		var object map[string]interface{}
		if err := json.Unmarshal(body, &object); err != nil {
			t.Fatalf("decode request fixture: %v", err)
		}
		change(object)
		encoded, err := json.Marshal(object)
		if err != nil {
			t.Fatalf("encode request fixture: %v", err)
		}
		return encoded
	}
	resign := func(headers map[string]string, body []byte, timestamp int64) {
		t.Helper()
		turn, err := strconv.Atoi(headers["X-ACB-Turn"])
		if err != nil {
			t.Fatalf("parse fixture turn: %v", err)
		}
		headers["X-ACB-Signature"] = SignRequest(secret, headers["X-ACB-Match-Id"], turn, timestamp, body)
	}

	tests := []struct {
		name   string
		want   string
		mutate func(map[string]string, []byte, int64) []byte
	}{
		{name: "missing match ID", want: "missing X-ACB-Match-Id", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			delete(headers, "X-ACB-Match-Id")
			return body
		}},
		{name: "missing turn", want: "missing X-ACB-Turn", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			delete(headers, "X-ACB-Turn")
			return body
		}},
		{name: "missing timestamp", want: "missing X-ACB-Timestamp", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			delete(headers, "X-ACB-Timestamp")
			return body
		}},
		{name: "missing bot ID", want: "missing X-ACB-Bot-Id", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			delete(headers, "X-ACB-Bot-Id")
			return body
		}},
		{name: "missing signature", want: "missing X-ACB-Signature", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			delete(headers, "X-ACB-Signature")
			return body
		}},
		{name: "malformed turn", want: "invalid X-ACB-Turn", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			headers["X-ACB-Turn"] = "eleven"
			return body
		}},
		{name: "malformed timestamp", want: "invalid X-ACB-Timestamp", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			headers["X-ACB-Timestamp"] = "now"
			return body
		}},
		{name: "malformed signature", want: "invalid X-ACB-Signature", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			headers["X-ACB-Signature"] = "not-hex"
			return body
		}},
		{name: "uppercase signature", want: "lowercase hex", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			headers["X-ACB-Signature"] = "F" + headers["X-ACB-Signature"][1:]
			return body
		}},
		{name: "unauthorized signature", want: "invalid request signature", mutate: func(headers map[string]string, body []byte, _ int64) []byte {
			replacement := byte('0')
			if headers["X-ACB-Signature"][0] == '0' {
				replacement = '1'
			}
			headers["X-ACB-Signature"] = string(replacement) + headers["X-ACB-Signature"][1:]
			return body
		}},
		{name: "malformed JSON", want: "invalid request body", mutate: func(headers map[string]string, _ []byte, timestamp int64) []byte {
			body := []byte(`{"match_id":`)
			resign(headers, body, timestamp)
			return body
		}},
		{name: "missing config", want: "request must include config", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { delete(object, "config") })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "unknown request field", want: "unknown field", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { object["future"] = true })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "case-variant request field", want: "unknown field Turn", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { object["Turn"] = 11 })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "null scalar", want: "request.turn must not be null", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { object["turn"] = nil })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "null nested scalar", want: "request.you.energy must not be null", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) {
				object["you"].(map[string]interface{})["energy"] = nil
			})
			resign(headers, body, timestamp)
			return body
		}},
		{name: "enabled zone omitted", want: "must include zone", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { delete(object, "zone") })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "disabled zone included", want: "must omit zone", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) {
				object["config"].(map[string]interface{})["zone_enabled"] = false
			})
			resign(headers, body, timestamp)
			return body
		}},
		{name: "null dead array", want: "request.dead must not be null", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { object["dead"] = nil })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "wrong nested type", want: "invalid request body", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) {
				object["you"].(map[string]interface{})["energy"] = "high"
			})
			resign(headers, body, timestamp)
			return body
		}},
		{name: "missing body identity", want: "request must include match_id", mutate: func(headers map[string]string, _ []byte, timestamp int64) []byte {
			body := []byte(`{}`)
			resign(headers, body, timestamp)
			return body
		}},
		{name: "match ID mismatch", want: "match_id does not match", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { object["match_id"] = "m_other" })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "turn mismatch", want: "turn does not match", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { object["turn"] = 12 })
			resign(headers, body, timestamp)
			return body
		}},
		{name: "negative turn", want: "turn must be non-negative", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			body = rewrite(body, func(object map[string]interface{}) { object["turn"] = -1 })
			headers["X-ACB-Turn"] = "-1"
			resign(headers, body, timestamp)
			return body
		}},
		{name: "stale timestamp", want: "timestamp expired", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			timestamp -= int64(TimestampTolerance/time.Second) + 1
			headers["X-ACB-Timestamp"] = strconv.FormatInt(timestamp, 10)
			resign(headers, body, timestamp)
			return body
		}},
		{name: "future timestamp", want: "timestamp expired", mutate: func(headers map[string]string, body []byte, timestamp int64) []byte {
			timestamp += int64(TimestampTolerance/time.Second) + 1
			headers["X-ACB-Timestamp"] = strconv.FormatInt(timestamp, 10)
			resign(headers, body, timestamp)
			return body
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			timestamp := time.Now().Unix()
			body, err := json.Marshal(botProtocolConformanceState("m_auth", 11))
			if err != nil {
				t.Fatalf("marshal request fixture: %v", err)
			}
			headers := map[string]string{
				"X-ACB-Match-Id":  "m_auth",
				"X-ACB-Turn":      "11",
				"X-ACB-Timestamp": strconv.FormatInt(timestamp, 10),
				"X-ACB-Bot-Id":    "b_auth",
				"X-ACB-Signature": SignRequest(secret, "m_auth", 11, timestamp, body),
			}
			body = test.mutate(headers, body, timestamp)
			auth, err := ParseAuthHeaders(headers)
			if err == nil {
				err = VerifyRequest(secret, auth, body)
			}
			if err == nil {
				t.Fatal("malformed or unauthorized request was accepted")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("rejection = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestBotProtocolConformance_ResponseSchemaAndNoOp(t *testing.T) {
	const secret = "conformance-response-secret"
	tests := []struct {
		name      string
		body      string
		wantMoves int
		wantErr   bool
	}{
		{name: "valid empty moves", body: `{"moves":[]}`},
		{name: "valid stay", body: `{"moves":[{"position":{"row":5,"col":5},"direction":"stay"}]}`},
		{name: "valid move with additive field", body: `{"moves":[{"position":{"row":5,"col":5},"direction":"N"}],"future":{"enabled":true}}`, wantMoves: 1},
		{name: "malformed JSON", body: `{`, wantErr: true},
		{name: "missing moves", body: `{"debug":null}`, wantErr: true},
		{name: "null moves", body: `{"moves":null}`, wantErr: true},
		{name: "missing position", body: `{"moves":[{"direction":"N"}]}`, wantErr: true},
		{name: "missing direction", body: `{"moves":[{"position":{"row":5,"col":5}}]}`, wantErr: true},
		{name: "invalid direction", body: `{"moves":[{"position":{"row":5,"col":5},"direction":"spin"}]}`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(test.body)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				turn, err := strconv.Atoi(r.Header.Get("X-ACB-Turn"))
				if err != nil {
					t.Errorf("X-ACB-Turn is not an integer: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-ACB-Signature", SignResponse(secret, r.Header.Get("X-ACB-Match-Id"), turn, body))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(body)
			}))
			t.Cleanup(server.Close)

			bot := NewHTTPBot(server.URL, AuthConfig{BotID: "b_response", Secret: secret, MatchID: "m_response"})
			moves, err := bot.GetMoves(botProtocolConformanceState("m_response", 13))
			if (err != nil) != test.wantErr {
				t.Fatalf("GetMoves() error = %v, wantErr %v", err, test.wantErr)
			}
			if len(moves) != test.wantMoves {
				t.Errorf("GetMoves() returned %d moves, want %d", len(moves), test.wantMoves)
			}
			if test.wantErr {
				if bot.failCount != 1 {
					t.Errorf("failCount = %d, want 1", bot.failCount)
				}
				if bot.IsCrashed() {
					t.Error("one invalid response marked the bot inactive")
				}
			} else if bot.failCount != 0 {
				t.Errorf("failCount = %d, want 0", bot.failCount)
			}
		})
	}
}

func botProtocolConformanceVisibleState(t *testing.T, matchID string, turn int) *VisibleState {
	t.Helper()
	game := NewGameState(DefaultConfig(), rand.New(rand.NewSource(7)))
	game.MatchID = matchID
	game.Turn = turn
	game.AddPlayer()
	game.AddPlayer()
	game.SpawnBot(0, Position{Row: 5, Col: 5})
	game.SpawnBot(1, Position{Row: 10, Col: 10})
	return game.GetVisibleState(0)
}

func botProtocolConformanceState(matchID string, turn int) *VisibleState {
	state := &VisibleState{
		MatchID: matchID,
		Turn:    turn,
		Config:  DefaultConfig(),
		Bots: []VisibleBot{
			{Position: Position{Row: 5, Col: 5}, Owner: 0},
			{Position: Position{Row: 10, Col: 10}, Owner: 1},
		},
		Energy: []Position{},
		Cores:  []VisibleCore{},
		Walls:  []Position{},
		Dead:   []VisibleBot{},
		Zone:   &ZoneBounds{Center: Position{Row: 20, Col: 20}, Radius: 20},
	}
	state.You.ID = 0
	return state
}

func validateBotProtocolConformanceRequest(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var request botProtocolConformanceRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing request data: %w", err)
	}

	if request.MatchID == nil || request.Turn == nil || request.Config == nil || request.You == nil ||
		request.Bots == nil || request.Energy == nil || request.Cores == nil || request.Walls == nil || request.Dead == nil {
		return fmt.Errorf("request omits a required top-level field")
	}
	if request.You.ID == nil || request.You.Energy == nil || request.You.Score == nil {
		return fmt.Errorf("request omits a required you field")
	}
	config := request.Config
	if config.Rows == nil || config.Cols == nil || config.MaxTurns == nil || config.VisionRadius2 == nil ||
		config.AttackRadius2 == nil || config.SpawnCost == nil || config.EnergyInterval == nil ||
		config.CoresPerPlayer == nil || config.ZoneEnabled == nil || config.ZoneStartTurn == nil ||
		config.ZoneShrinkInterval == nil || config.ZoneShrinkStep == nil || config.ZoneMinRadius == nil || config.KillScore == nil {
		return fmt.Errorf("request omits a required config field")
	}
	for i, bot := range *request.Bots {
		if err := validateBotProtocolPosition(bot.Position); err != nil {
			return fmt.Errorf("bots[%d]: %w", i, err)
		}
		if bot.Owner == nil {
			return fmt.Errorf("bots[%d]: missing owner", i)
		}
	}
	for i, position := range *request.Energy {
		if err := validateBotProtocolPosition(&position); err != nil {
			return fmt.Errorf("energy[%d]: %w", i, err)
		}
	}
	for i, core := range *request.Cores {
		if err := validateBotProtocolPosition(core.Position); err != nil {
			return fmt.Errorf("cores[%d]: %w", i, err)
		}
		if core.Owner == nil || core.Active == nil {
			return fmt.Errorf("cores[%d]: missing owner or active", i)
		}
	}
	for i, position := range *request.Walls {
		if err := validateBotProtocolPosition(&position); err != nil {
			return fmt.Errorf("walls[%d]: %w", i, err)
		}
	}
	for i, bot := range *request.Dead {
		if err := validateBotProtocolPosition(bot.Position); err != nil {
			return fmt.Errorf("dead[%d]: %w", i, err)
		}
		if bot.Owner == nil {
			return fmt.Errorf("dead[%d]: missing owner", i)
		}
	}
	if request.Zone != nil {
		if err := validateBotProtocolPosition(request.Zone.Center); err != nil {
			return fmt.Errorf("zone: %w", err)
		}
		if request.Zone.Radius == nil || request.Zone.Active == nil {
			return fmt.Errorf("zone: missing radius or active")
		}
	}
	return nil
}

func validateBotProtocolPosition(position *botProtocolConformancePosition) error {
	if position == nil || position.Row == nil || position.Col == nil {
		return fmt.Errorf("position must include integer row and col")
	}
	return nil
}
