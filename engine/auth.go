package engine

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

const (
	// TimestampTolerance is the allowed clock skew for request validation (30 seconds)
	TimestampTolerance = 30 * time.Second
)

// AuthConfig holds authentication configuration for a bot.
type AuthConfig struct {
	BotID   string // Unique bot identifier (e.g., "b_4e8c1d2f")
	Secret  string // Shared secret (hex-encoded, 64 characters)
	MatchID string // Current match ID
}

// RequestAuth contains the authentication headers for an engine-to-bot request.
type RequestAuth struct {
	MatchID   string
	Turn      int
	Timestamp int64
	BotID     string
	Signature string
}

// SignRequest generates the HMAC signature for an outgoing request.
// signing_string = "{match_id}.{turn}.{timestamp}.{sha256(request_body)}"
// signature = HMAC-SHA256(shared_secret, signing_string)
func SignRequest(secret, matchID string, turn int, timestamp int64, requestBody []byte) string {
	bodyHash := sha256.Sum256(requestBody)
	signingString := fmt.Sprintf("%s.%d.%d.%s", matchID, turn, timestamp, hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignResponse generates the HMAC signature for a bot response.
// signing_string = "{match_id}.{turn}.{sha256(response_body)}"
// signature = HMAC-SHA256(shared_secret, signing_string)
func SignResponse(secret, matchID string, turn int, responseBody []byte) string {
	bodyHash := sha256.Sum256(responseBody)
	signingString := fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyRequest verifies an incoming request's signature.
// Returns an error if verification fails.
func VerifyRequest(secret string, auth RequestAuth, requestBody []byte) error {
	// Check timestamp is within tolerance
	now := time.Now().Unix()
	requestTime := auth.Timestamp
	diff := now - requestTime
	if diff < 0 {
		diff = -diff
	}
	if time.Duration(diff)*time.Second > TimestampTolerance {
		return fmt.Errorf("timestamp expired: request was %v ago (tolerance: %v)",
			time.Duration(diff)*time.Second, TimestampTolerance)
	}

	// Compute expected signature
	expectedSig := SignRequest(secret, auth.MatchID, auth.Turn, auth.Timestamp, requestBody)

	// Constant-time comparison
	if !hmac.Equal([]byte(auth.Signature), []byte(expectedSig)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// VerifyResponse verifies a bot response's signature.
func VerifyResponse(secret, matchID string, turn int, signature string, responseBody []byte) error {
	expectedSig := SignResponse(secret, matchID, turn, responseBody)

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("invalid response signature")
	}

	return nil
}

// ParseAuthHeaders extracts authentication info from HTTP headers.
// Headers: X-ACB-Match-Id, X-ACB-Turn, X-ACB-Timestamp, X-ACB-Bot-Id, X-ACB-Signature
func ParseAuthHeaders(headers map[string]string) (RequestAuth, error) {
	var auth RequestAuth
	var err error

	auth.MatchID = headers["X-ACB-Match-Id"]
	if auth.MatchID == "" {
		return auth, fmt.Errorf("missing X-ACB-Match-Id header")
	}

	turnStr := headers["X-ACB-Turn"]
	if turnStr == "" {
		return auth, fmt.Errorf("missing X-ACB-Turn header")
	}
	auth.Turn, err = strconv.Atoi(turnStr)
	if err != nil {
		return auth, fmt.Errorf("invalid X-ACB-Turn header: %w", err)
	}

	timestampStr := headers["X-ACB-Timestamp"]
	if timestampStr == "" {
		return auth, fmt.Errorf("missing X-ACB-Timestamp header")
	}
	auth.Timestamp, err = strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return auth, fmt.Errorf("invalid X-ACB-Timestamp header: %w", err)
	}

	auth.BotID = headers["X-ACB-Bot-Id"]
	if auth.BotID == "" {
		return auth, fmt.Errorf("missing X-ACB-Bot-Id header")
	}

	auth.Signature = headers["X-ACB-Signature"]
	if auth.Signature == "" {
		return auth, fmt.Errorf("missing X-ACB-Signature header")
	}

	return auth, nil
}

// GenerateSecret generates a new random 256-bit secret (hex-encoded).
// This should be called at bot registration time.
func GenerateSecret(rng interface{ Read([]byte) (int, error) }) (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rng.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
