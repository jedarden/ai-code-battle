package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// Helper function to generate a request signature for testing
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
	body := []byte(`{"test": "data"}`)
	now := time.Now()

	tests := []struct {
		name    string
		headers AuthHeaders
		want    bool
	}{
		{
			name: "valid signature",
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: now.Format(time.RFC3339),
				Signature: signRequest(secret, "m_test123", "42", now.Format(time.RFC3339), body),
			},
			want: true,
		},
		{
			name: "invalid signature",
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: now.Format(time.RFC3339),
				Signature: "invalid",
			},
			want: false,
		},
		{
			name: "old timestamp",
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: now.Add(-60 * time.Second).Format(time.RFC3339),
				Signature: signRequest(secret, "m_test123", "42", now.Add(-60*time.Second).Format(time.RFC3339), body),
			},
			want: false,
		},
		{
			name: "wrong body",
			headers: AuthHeaders{
				MatchID:   "m_test123",
				Turn:      "42",
				Timestamp: now.Format(time.RFC3339),
				Signature: signRequest(secret, "m_test123", "42", now.Format(time.RFC3339), []byte("wrong")),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifyRequest(secret, tt.headers, body); got != tt.want {
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
