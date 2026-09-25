// Package conformance implements the bot-side HTTP protocol conformance
// suite for docs/bot-protocol.md.
//
// The suite drives a running bot over real HTTP with golden raw-byte
// requests: tampered signatures, stale and future timestamps, non-canonical
// header spellings, unknown JSON fields, body/header identity mismatches,
// malformed bodies, and health-check probes. Positive cases additionally
// verify that the bot signs its response over the exact bytes on the wire.
//
// The expected statuses mirror the reference gate pinned by
// TestBotProtocolConformance_RejectedRequestNeverExecutes in the engine
// package, which is the executable reading of docs/bot-protocol.md.
package conformance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// DefaultConformanceSecret is the UTF-8 HMAC key used by the suite. It is
// the same non-ASCII key worked through the examples in docs/bot-protocol.md,
// so every implementation is exercised on multi-byte key handling.
const DefaultConformanceSecret = "sëcret-🔑"

// ConformanceMatchID and ConformanceTurn identify the suite's traffic.
const (
	ConformanceMatchID = "m_conformance"
	ConformanceTurn    = 7
)

// TimestampSkew is applied to the canonical timestamp to build the stale and
// future cases. It deliberately exceeds the 30 second tolerance by enough to
// stay wrong even if case construction and the request land seconds apart.
const TimestampSkew = 35 * time.Second

// CanonicalRequestPayloadString builds the signed payload from the exact
// header strings as they will be sent on the wire. The engine helper takes
// parsed integers; the suite must be able to sign non-canonical spellings,
// so it carries its own raw-string builder. For canonical inputs the two
// agree byte for byte, which TestGoldenVectors pins.
func CanonicalRequestPayloadString(matchID, turn, timestamp string, requestBody []byte) string {
	bodyHash := sha256.Sum256(requestBody)
	return fmt.Sprintf("%s.%s.%s.%s", matchID, turn, timestamp, hex.EncodeToString(bodyHash[:]))
}

// SignRequestString HMACs a raw-string canonical payload the way the engine
// signs canonical ones: the key is the UTF-8 bytes of the secret.
func SignRequestString(secret, matchID, turn, timestamp string, requestBody []byte) string {
	return signPayload(secret, CanonicalRequestPayloadString(matchID, turn, timestamp, requestBody))
}

// CanonicalResponsePayload builds the payload a conformant bot signs.
func CanonicalResponsePayload(matchID string, turn int, responseBody []byte) string {
	bodyHash := sha256.Sum256(responseBody)
	return fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))
}

func signPayload(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// canonicalTimestamp renders a Unix-seconds timestamp in canonical base-10.
func canonicalTimestamp(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}
