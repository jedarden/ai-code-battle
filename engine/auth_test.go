package engine

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"
)

func TestSignRequest(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	matchID := "m_7f3a9b2c"
	turn := 42
	timestamp := int64(1711200000)
	body := []byte(`{"match_id":"m_7f3a9b2c"}`)

	sig := SignRequest(secret, matchID, turn, timestamp, body)

	// Signature should be 64 hex characters (256 bits)
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}

	// Same input should produce same signature
	sig2 := SignRequest(secret, matchID, turn, timestamp, body)
	if sig != sig2 {
		t.Error("signature not deterministic")
	}

	// Different secret should produce different signature
	sig3 := SignRequest("different"+secret[10:], matchID, turn, timestamp, body)
	if sig == sig3 {
		t.Error("different secrets produced same signature")
	}

	// Different body should produce different signature
	sig4 := SignRequest(secret, matchID, turn, timestamp, []byte(`{}`))
	if sig == sig4 {
		t.Error("different bodies produced same signature")
	}
}

func TestSignResponse(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	matchID := "m_7f3a9b2c"
	turn := 42
	body := []byte(`{"moves":[]}`)

	sig := SignResponse(secret, matchID, turn, body)

	// Signature should be 64 hex characters
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}

	// Same input should produce same signature
	sig2 := SignResponse(secret, matchID, turn, body)
	if sig != sig2 {
		t.Error("signature not deterministic")
	}
}

func TestVerifyRequest(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	matchID := "m_7f3a9b2c"
	turn := 42
	timestamp := time.Now().Unix()
	body := []byte(`{"match_id":"m_7f3a9b2c"}`)

	sig := SignRequest(secret, matchID, turn, timestamp, body)

	auth := RequestAuth{
		MatchID:   matchID,
		Turn:      turn,
		Timestamp: timestamp,
		BotID:     "b_test",
		Signature: sig,
	}

	// Valid signature should pass
	if err := VerifyRequest(secret, auth, body); err != nil {
		t.Errorf("valid signature failed: %v", err)
	}

	// Wrong secret should fail
	if err := VerifyRequest("wrong"+secret[5:], auth, body); err == nil {
		t.Error("wrong secret should fail verification")
	}

	// Wrong signature should fail
	auth2 := auth
	auth2.Signature = "0" + auth.Signature[1:]
	if err := VerifyRequest(secret, auth2, body); err == nil {
		t.Error("wrong signature should fail verification")
	}

	// Expired timestamp should fail
	auth3 := auth
	auth3.Timestamp = time.Now().Unix() - 60 // 60 seconds ago
	if err := VerifyRequest(secret, auth3, body); err == nil {
		t.Error("expired timestamp should fail verification")
	}

	// Future timestamp should fail
	auth4 := auth
	auth4.Timestamp = time.Now().Unix() + 60 // 60 seconds in future
	if err := VerifyRequest(secret, auth4, body); err == nil {
		t.Error("future timestamp should fail verification")
	}
}

func TestVerifyResponse(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	matchID := "m_7f3a9b2c"
	turn := 42
	body := []byte(`{"moves":[]}`)

	sig := SignResponse(secret, matchID, turn, body)

	// Valid signature should pass
	if err := VerifyResponse(secret, matchID, turn, sig, body); err != nil {
		t.Errorf("valid signature failed: %v", err)
	}

	// Wrong secret should fail
	if err := VerifyResponse("wrong", matchID, turn, sig, body); err == nil {
		t.Error("wrong secret should fail verification")
	}

	// Wrong turn should fail
	if err := VerifyResponse(secret, matchID, turn+1, sig, body); err == nil {
		t.Error("wrong turn should fail verification")
	}

	// Wrong body should fail
	if err := VerifyResponse(secret, matchID, turn, sig, []byte(`{}`)); err == nil {
		t.Error("wrong body should fail verification")
	}
}

func TestParseAuthHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		wantErr bool
	}{
		{
			name: "valid headers",
			headers: map[string]string{
				"X-ACB-Match-Id":  "m_7f3a9b2c",
				"X-ACB-Turn":      "42",
				"X-ACB-Timestamp": "1711200000",
				"X-ACB-Bot-Id":    "b_4e8c1d2f",
				"X-ACB-Signature": "abc123",
			},
			wantErr: false,
		},
		{
			name:    "missing all headers",
			headers: map[string]string{},
			wantErr: true,
		},
		{
			name: "missing signature",
			headers: map[string]string{
				"X-ACB-Match-Id":  "m_7f3a9b2c",
				"X-ACB-Turn":      "42",
				"X-ACB-Timestamp": "1711200000",
				"X-ACB-Bot-Id":    "b_4e8c1d2f",
			},
			wantErr: true,
		},
		{
			name: "invalid turn",
			headers: map[string]string{
				"X-ACB-Match-Id":  "m_7f3a9b2c",
				"X-ACB-Turn":      "notanumber",
				"X-ACB-Timestamp": "1711200000",
				"X-ACB-Bot-Id":    "b_4e8c1d2f",
				"X-ACB-Signature": "abc123",
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp",
			headers: map[string]string{
				"X-ACB-Match-Id":  "m_7f3a9b2c",
				"X-ACB-Turn":      "42",
				"X-ACB-Timestamp": "notanumber",
				"X-ACB-Bot-Id":    "b_4e8c1d2f",
				"X-ACB-Signature": "abc123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := ParseAuthHeaders(tt.headers)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthHeaders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if auth.MatchID != "m_7f3a9b2c" {
					t.Errorf("MatchID = %q, want %q", auth.MatchID, "m_7f3a9b2c")
				}
				if auth.Turn != 42 {
					t.Errorf("Turn = %d, want 42", auth.Turn)
				}
			}
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	// Should be 64 hex characters (256 bits)
	if len(secret) != 64 {
		t.Errorf("secret length = %d, want 64", len(secret))
	}

	// Should be valid hex
	_, err = hex.DecodeString(secret)
	if err != nil {
		t.Errorf("secret is not valid hex: %v", err)
	}

	// Should produce different values
	secret2, err := GenerateSecret(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateSecret(2) failed: %v", err)
	}
	if secret == secret2 {
		t.Error("GenerateSecret produced same value twice")
	}
}
