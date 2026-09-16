// Package game provides authentication utilities for AI Code Battle bots.
package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
)

// VerifyRequest verifies the HMAC signature of an incoming request.
func VerifyRequest(secret string, headers AuthHeaders, body []byte) bool {
	// Verify timestamp is within allowed window
	if headers.Timestamp != "" && !VerifyTimestamp(headers.Timestamp) {
		return false
	}

	// Compute expected signature. The signing string excludes the timestamp
	// (X-ACB-Timestamp is validated above for clock skew, never signed).
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s",
		headers.MatchID,
		headers.Turn,
		hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(headers.Signature), []byte(expectedSig))
}

// SignResponse generates the HMAC signature for a response.
// Same signing string as the request: {match_id}.{turn}.{sha256_hex(body)}
func SignResponse(secret, matchID, turnStr string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	turn, _ := strconv.Atoi(turnStr)

	signingString := fmt.Sprintf("%s.%d.%s",
		matchID,
		turn,
		hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}
