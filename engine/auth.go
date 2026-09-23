package engine

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// CanonicalRequestPayload returns the exact bytes covered by an engine request signature.
func CanonicalRequestPayload(matchID string, turn int, timestamp int64, requestBody []byte) string {
	bodyHash := sha256.Sum256(requestBody)
	return fmt.Sprintf("%s.%d.%d.%s", matchID, turn, timestamp, hex.EncodeToString(bodyHash[:]))
}

// CanonicalResponsePayload returns the exact bytes covered by a bot response signature.
func CanonicalResponsePayload(matchID string, turn int, responseBody []byte) string {
	bodyHash := sha256.Sum256(responseBody)
	return fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))
}

func signPayload(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func decodeSignature(signature string) ([]byte, error) {
	decoded, err := hex.DecodeString(signature)
	if err != nil {
		return nil, fmt.Errorf("signature must be hex encoded: %w", err)
	}
	if len(decoded) != sha256.Size {
		return nil, fmt.Errorf("signature must be %d bytes, got %d", sha256.Size, len(decoded))
	}
	return decoded, nil
}

func verifySignature(secret, payload, signature string) bool {
	provided, err := decodeSignature(signature)
	if err != nil {
		return false
	}
	expected, err := decodeSignature(signPayload(secret, payload))
	if err != nil {
		return false
	}
	return hmac.Equal(provided, expected)
}

func validateRequestPayload(auth RequestAuth, requestBody []byte) error {
	var payload struct {
		MatchID *string `json:"match_id"`
		Turn    *int    `json:"turn"`
	}
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	if payload.MatchID == nil || payload.Turn == nil {
		return fmt.Errorf("request body must include match_id and turn")
	}
	if *payload.MatchID != auth.MatchID {
		return fmt.Errorf("request match_id does not match X-ACB-Match-Id")
	}
	if *payload.Turn != auth.Turn {
		return fmt.Errorf("request turn does not match X-ACB-Turn")
	}
	return nil
}

// SignRequest generates the HMAC signature for an outgoing request.
func SignRequest(secret, matchID string, turn int, timestamp int64, requestBody []byte) string {
	return signPayload(secret, CanonicalRequestPayload(matchID, turn, timestamp, requestBody))
}

// SignResponse generates the HMAC signature for a bot response.
func SignResponse(secret, matchID string, turn int, responseBody []byte) string {
	return signPayload(secret, CanonicalResponsePayload(matchID, turn, responseBody))
}

// VerifyRequest verifies an incoming request's identity, freshness, and signature.
func VerifyRequest(secret string, auth RequestAuth, requestBody []byte) error {
	if secret == "" {
		return fmt.Errorf("shared secret is empty")
	}
	if auth.MatchID == "" {
		return fmt.Errorf("request match ID is empty")
	}
	if auth.BotID == "" {
		return fmt.Errorf("request bot ID is empty")
	}
	if auth.Turn < 0 {
		return fmt.Errorf("request turn must be non-negative")
	}

	now := time.Now()
	requestTime := time.Unix(auth.Timestamp, 0)
	if requestTime.Before(now.Add(-TimestampTolerance)) || requestTime.After(now.Add(TimestampTolerance)) {
		return fmt.Errorf("timestamp expired: request timestamp %d is outside tolerance %v", auth.Timestamp, TimestampTolerance)
	}

	payload := CanonicalRequestPayload(auth.MatchID, auth.Turn, auth.Timestamp, requestBody)
	if !verifySignature(secret, payload, auth.Signature) {
		return fmt.Errorf("invalid request signature")
	}
	if err := validateRequestPayload(auth, requestBody); err != nil {
		return err
	}
	return nil
}

// VerifyResponse verifies a bot response's signature.
func VerifyResponse(secret, matchID string, turn int, signature string, responseBody []byte) error {
	if secret == "" {
		return fmt.Errorf("shared secret is empty")
	}
	if matchID == "" {
		return fmt.Errorf("response match ID is empty")
	}
	if turn < 0 {
		return fmt.Errorf("response turn must be non-negative")
	}
	if !verifySignature(secret, CanonicalResponsePayload(matchID, turn, responseBody), signature) {
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
	if _, err := decodeSignature(auth.Signature); err != nil {
		return auth, fmt.Errorf("invalid X-ACB-Signature header: %w", err)
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
