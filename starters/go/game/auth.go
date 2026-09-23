// Package game provides authentication utilities for AI Code Battle bots.
package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// VerifyRequest verifies the HMAC signature of an incoming request.
func VerifyRequest(secret string, headers AuthHeaders, body []byte) bool {
	if secret == "" || headers.MatchID == "" || headers.Turn == "" || headers.Timestamp == "" || headers.BotID == "" || headers.Signature == "" {
		return false
	}

	turn, err := strconv.Atoi(headers.Turn)
	if err != nil || turn < 0 || strconv.Itoa(turn) != headers.Turn {
		return false
	}

	timestampUnix, err := strconv.ParseInt(headers.Timestamp, 10, 64)
	if err != nil {
		return false
	}
	age := time.Since(time.Unix(timestampUnix, 0))
	if age < -30*time.Second || age > 30*time.Second {
		return false
	}

	// Compute expected signature over the timestamp and exact request bytes.
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s.%s",
		headers.MatchID,
		headers.Turn,
		headers.Timestamp,
		hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(headers.Signature), []byte(expectedSig)) {
		return false
	}

	var identity struct {
		MatchID *string `json:"match_id"`
		Turn    *int    `json:"turn"`
	}
	if err := json.Unmarshal(body, &identity); err != nil {
		return false
	}
	return identity.MatchID != nil && identity.Turn != nil && *identity.MatchID == headers.MatchID && *identity.Turn == turn
}

// SignResponse generates the HMAC signature for a response.
// signing_string = "{match_id}.{turn}.{sha256_hex(body)}"
func SignResponse(secret, matchID, turnStr string, body []byte) string {
	if secret == "" || matchID == "" || turnStr == "" {
		return ""
	}
	turn, err := strconv.Atoi(turnStr)
	if err != nil {
		return ""
	}

	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%d.%s",
		matchID,
		turn,
		hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}
