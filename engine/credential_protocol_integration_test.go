package engine

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestCredentialLifecycle_TimestampWindow(t *testing.T) {
	const matchID = "m_credential_timestamp"
	const turn = 4
	secret, err := GenerateSecret(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(botProtocolConformanceState(matchID, turn))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		offset time.Duration
		valid  bool
	}{
		{name: "current", offset: 0, valid: true},
		{name: "just inside past", offset: -TimestampTolerance + time.Second, valid: true},
		{name: "just inside future", offset: TimestampTolerance - time.Second, valid: true},
		{name: "stale", offset: -TimestampTolerance - time.Second, valid: false},
		{name: "future", offset: TimestampTolerance + time.Second, valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			timestamp := time.Now().Add(test.offset).Unix()
			auth := RequestAuth{
				MatchID:   matchID,
				Turn:      turn,
				Timestamp: timestamp,
				BotID:     "b_timestamp",
				Signature: SignRequest(secret, matchID, turn, timestamp, body),
			}
			err := VerifyRequest(secret, auth, body)
			if (err == nil) != test.valid {
				t.Fatalf("VerifyRequest() error = %v, valid = %t", err, test.valid)
			}
		})
	}
}

func TestCredentialLifecycle_SignedTurnRoundTrip(t *testing.T) {
	const (
		botID   = "b_credential"
		matchID = "m_credential_http"
	)
	secret, err := GenerateSecret(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read turn body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		headers := map[string]string{
			"X-ACB-Match-Id":  r.Header.Get("X-ACB-Match-Id"),
			"X-ACB-Turn":      r.Header.Get("X-ACB-Turn"),
			"X-ACB-Timestamp": r.Header.Get("X-ACB-Timestamp"),
			"X-ACB-Bot-Id":    r.Header.Get("X-ACB-Bot-Id"),
			"X-ACB-Signature": r.Header.Get("X-ACB-Signature"),
		}
		auth, err := ParseAuthHeaders(headers)
		if err != nil {
			t.Errorf("ParseAuthHeaders() = %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := VerifyRequest(secret, auth, body); err != nil {
			t.Errorf("VerifyRequest() = %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		turn, err := strconv.Atoi(r.Header.Get("X-ACB-Turn"))
		if err != nil {
			t.Errorf("parse turn: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		responseBody := []byte(`{"moves":[]}`)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-ACB-Signature", SignResponse(secret, matchID, turn, responseBody))
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(server.Close)

	bot := NewHTTPBot(server.URL, AuthConfig{BotID: botID, Secret: secret, MatchID: matchID})
	moves, err := bot.GetMoves(botProtocolConformanceState(matchID, 9))
	if err != nil {
		t.Fatalf("signed turn round trip failed: %v", err)
	}
	if len(moves) != 0 {
		t.Fatalf("signed empty move response produced %d moves", len(moves))
	}
}
