package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestVerifyTimestamp(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp string
		want      bool
	}{
		{
			name:      "current time RFC3339",
			timestamp: now.Format(time.RFC3339),
			want:      true,
		},
		{
			name:      "20 seconds ago RFC3339",
			timestamp: now.Add(-20 * time.Second).Format(time.RFC3339),
			want:      true,
		},
		{
			name:      "20 seconds future RFC3339",
			timestamp: now.Add(20 * time.Second).Format(time.RFC3339),
			want:      true,
		},
		{
			name:      "31 seconds ago - too old",
			timestamp: now.Add(-31 * time.Second).Format(time.RFC3339),
			want:      false,
		},
		{
			name:      "31 seconds future - too new",
			timestamp: now.Add(31 * time.Second).Format(time.RFC3339),
			want:      false,
		},
		{
			name:      "unix timestamp current",
			timestamp: fmt.Sprintf("%d", now.Unix()),
			want:      true,
		},
		{
			name:      "unix timestamp 20 seconds ago",
			timestamp: fmt.Sprintf("%d", now.Add(-20*time.Second).Unix()),
			want:      true,
		},
		{
			name:      "unix timestamp 31 seconds ago - too old",
			timestamp: fmt.Sprintf("%d", now.Add(-31*time.Second).Unix()),
			want:      false,
		},
		{
			name:      "invalid format",
			timestamp: "invalid",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Allow some tolerance for current time tests
			if tt.name == "current time RFC3339" || tt.name == "unix timestamp current" {
				if got := VerifyTimestamp(tt.timestamp); !got {
					t.Errorf("VerifyTimestamp() = false, want true (may be timing issue)")
				}
				return
			}
			if got := VerifyTimestamp(tt.timestamp); got != tt.want {
				t.Errorf("VerifyTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to generate a request signature for testing.
func signRequest(secret, matchID, turn, timestamp string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s.%s",
		matchID,
		turn,
		timestamp,
		hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyRequest(t *testing.T) {
	secret := "test-secret"
	body := []byte("{\n  \"match_id\": \"m_test123\",\n  \"turn\": 42\n}")
	now := time.Now()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	oldTimestamp := strconv.FormatInt(now.Add(-60*time.Second).Unix(), 10)
	futureTimestamp := strconv.FormatInt(now.Add(60*time.Second).Unix(), 10)
	tamperedTimestamp := strconv.FormatInt(now.Add(-5*time.Second).Unix(), 10)

	tests := []struct {
		name    string
		secret  string
		headers AuthHeaders
		body    []byte
		want    bool
	}{
		{
			name:   "valid signature",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, body),
			},
			body: body,
			want: true,
		},
		{
			name:   "empty secret",
			secret: "",
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "missing match ID",
			secret: secret,
			headers: AuthHeaders{
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "", "42", timestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "missing bot ID",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "missing turn",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "", timestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "missing timestamp",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Signature: signRequest(secret, "m_test123", "42", "", body),
			},
			body: body,
			want: false,
		},
		{
			name:   "missing signature",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
			},
			body: body,
			want: false,
		},
		{
			name:   "invalid signature",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: "invalid",
			},
			body: body,
			want: false,
		},
		{
			name:   "old timestamp",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: oldTimestamp,
				Signature: signRequest(secret, "m_test123", "42", oldTimestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "future timestamp",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: futureTimestamp,
				Signature: signRequest(secret, "m_test123", "42", futureTimestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "malformed timestamp",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: "invalid",
				Signature: signRequest(secret, "m_test123", "42", "invalid", body),
			},
			body: body,
			want: false,
		},
		{
			name:   "malformed turn",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "invalid",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "invalid", timestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "negative turn",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "-1",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "-1", timestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "tampered timestamp",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", tamperedTimestamp, body),
			},
			body: body,
			want: false,
		},
		{
			name:   "wrong raw body",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, body),
			},
			body: []byte("{}"),
			want: false,
		},
		{
			name:   "body match ID mismatch",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, []byte(`{"match_id":"other","turn":42}`)),
			},
			body: []byte(`{"match_id":"other","turn":42}`),
			want: false,
		},
		{
			name:   "body turn mismatch",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, []byte(`{"match_id":"m_test123","turn":41}`)),
			},
			body: []byte(`{"match_id":"m_test123","turn":41}`),
			want: false,
		},
		{
			name:   "missing body identity",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, []byte(`{"turn":42}`)),
			},
			body: []byte(`{"turn":42}`),
			want: false,
		},
		{
			name:   "malformed body",
			secret: secret,
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: timestamp,
				Signature: signRequest(secret, "m_test123", "42", timestamp, []byte("{")),
			},
			body: []byte("{"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "missing bot ID" {
				tt.headers.BotID = "b_test123"
			}
			if got := VerifyRequest(tt.secret, tt.headers, tt.body); got != tt.want {
				t.Errorf("VerifyRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSignResponse(t *testing.T) {
	secret := "test-secret"
	matchID := "m_test123"
	turn := "42"
	body := []byte(`{"moves":[]}`)

	sig := SignResponse(secret, matchID, turn, body)

	if sig == "" {
		t.Error("SignResponse() returned empty string")
	}

	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if sig != expectedSig {
		t.Errorf("SignResponse() = %q, want %q", sig, expectedSig)
	}

	// Signature should be hex string (sha256 = 64 hex chars)
	if len(sig) != 64 {
		t.Errorf("SignResponse() returned signature of length %d, want 64", len(sig))
	}

	// Same inputs should produce same signature
	sig2 := SignResponse(secret, matchID, turn, body)
	if sig != sig2 {
		t.Error("SignResponse() produced different signatures for same inputs")
	}

	// Different secret should produce different signature
	sig3 := SignResponse("other-secret", matchID, turn, body)
	if sig == sig3 {
		t.Error("SignResponse() produced same signature for different secrets")
	}
}
