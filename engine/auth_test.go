package engine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func TestSignRequest(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	matchID := "m_7f3a9b2c"
	turn := 42
	timestamp := int64(1711200000)
	body := []byte(`{"match_id":"m_7f3a9b2c","turn":42}`)

	sig := SignRequest(secret, matchID, turn, timestamp, body)

	wantPayload := "m_7f3a9b2c.42.1711200000.467d8abda5a8473a8a1a4e41bf1dd4e0d8df2297767dff9184777678c1c1bfbb"
	if got := CanonicalRequestPayload(matchID, turn, timestamp, body); got != wantPayload {
		t.Errorf("CanonicalRequestPayload() = %q, want %q", got, wantPayload)
	}
	if sig != "41875634fc1c5adf152da8997026ba8f1948029c68f54279bc976e04548db8c4" {
		t.Errorf("SignRequest() = %q, want fixed HMAC test vector", sig)
	}

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

	wantPayload := "m_7f3a9b2c.42.4cba52032dfb0839b8138eb84ebcb5bd281253019dfc6efb447d211ac4333f8e"
	if got := CanonicalResponsePayload(matchID, turn, body); got != wantPayload {
		t.Errorf("CanonicalResponsePayload() = %q, want %q", got, wantPayload)
	}
	if sig != "142b87dffcad9eae1ec2cc2a26ee430ac9f67e89a1f87f4ae9ce0167f9ba420c" {
		t.Errorf("SignResponse() = %q, want fixed HMAC test vector", sig)
	}

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
	body, err := json.Marshal(botProtocolConformanceState(matchID, turn))
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

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

	// Wrong signature should fail — use a fixed garbage value that is never a valid HMAC
	auth2 := auth
	auth2.Signature = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	if err := VerifyRequest(secret, auth2, body); err == nil {
		t.Error("wrong signature should fail verification")
	}

	// Expired timestamp should fail
	auth3 := auth
	auth3.Timestamp = time.Now().Unix() - 60
	auth3.Signature = SignRequest(secret, matchID, turn, auth3.Timestamp, body)
	if err := VerifyRequest(secret, auth3, body); err == nil {
		t.Error("expired timestamp should fail verification")
	}

	// Future timestamp should fail
	auth4 := auth
	auth4.Timestamp = time.Now().Unix() + 60
	auth4.Signature = SignRequest(secret, matchID, turn, auth4.Timestamp, body)
	if err := VerifyRequest(secret, auth4, body); err == nil {
		t.Error("future timestamp should fail verification")
	}

	authExtreme := auth
	authExtreme.Timestamp = int64(^uint64(0) >> 1)
	authExtreme.Signature = SignRequest(secret, matchID, turn, authExtreme.Timestamp, body)
	if err := VerifyRequest(secret, authExtreme, body); err == nil {
		t.Error("extreme future timestamp should fail verification")
	}

	auth5 := auth
	auth5.Timestamp++
	if err := VerifyRequest(secret, auth5, body); err == nil {
		t.Error("tampered timestamp should fail verification")
	}

	auth6 := auth
	auth6.Signature = "not-hex"
	if err := VerifyRequest(secret, auth6, body); err == nil {
		t.Error("malformed signature should fail verification")
	}

	malformedBody := []byte(`{"match_id":`)
	malformedAuth := auth
	malformedAuth.Signature = SignRequest(secret, matchID, turn, timestamp, malformedBody)
	if err := VerifyRequest(secret, malformedAuth, malformedBody); err == nil {
		t.Error("authenticated malformed request should fail schema verification")
	}

	mismatchedBody, err := json.Marshal(botProtocolConformanceState("m_other", turn))
	if err != nil {
		t.Fatalf("marshal mismatched request body: %v", err)
	}
	mismatchedAuth := auth
	mismatchedAuth.Signature = SignRequest(secret, matchID, turn, timestamp, mismatchedBody)
	if err := VerifyRequest(secret, mismatchedAuth, mismatchedBody); err == nil {
		t.Error("request body identity should match signed headers")
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

	if err := VerifyResponse(secret, matchID, turn, "not-hex", body); err == nil {
		t.Error("malformed response signature should fail verification")
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
				"X-ACB-Signature": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
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
			name: "malformed signature",
			headers: map[string]string{
				"X-ACB-Match-Id":  "m_7f3a9b2c",
				"X-ACB-Turn":      "42",
				"X-ACB-Timestamp": "1711200000",
				"X-ACB-Bot-Id":    "b_4e8c1d2f",
				"X-ACB-Signature": "not-hex",
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
