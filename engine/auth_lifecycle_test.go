package engine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// mintCredential mints a credential pair the way registration does: a
// b_-prefixed identifier from 6 random bytes and a 256-bit secret rendered
// as 64 lowercase hex characters. Secrets are generated at test runtime and
// must never be embedded in source, logged, or included in an error string.
func mintCredential(t *testing.T) (string, string) {
	t.Helper()
	secret, err := GenerateSecret(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return "b_" + hex.EncodeToString(raw), secret
}

// assertNoSecretIn fails the test if err's text embeds any of the given
// secret values. Verification errors speak about signatures, timestamps,
// and schemas — never about key material.
func assertNoSecretIn(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	for _, secret := range secrets {
		if strings.Contains(msg, secret) {
			t.Fatalf("verification error embeds secret material: %q", msg)
		}
	}
}

// TestCredentialLifecycleValidCredential walks the delivered-credential
// path: a registered bot and the engine share the secret the platform
// returned at registration, and both directions of one turn verify.
func TestCredentialLifecycleValidCredential(t *testing.T) {
	botID, secret := mintCredential(t)
	matchID := "m_lifecycle_valid"
	turn := 7

	if len(secret) != 64 {
		t.Fatalf("secret length = %d, want 64 hex characters", len(secret))
	}
	if _, err := hex.DecodeString(secret); err != nil {
		t.Fatalf("minted secret is not hex: %v", err)
	}

	body, err := json.Marshal(botProtocolConformanceState(matchID, turn))
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	// The engine signs the request with the platform-held secret.
	timestamp := time.Now().Unix()
	auth := RequestAuth{
		MatchID:   matchID,
		Turn:      turn,
		Timestamp: timestamp,
		BotID:     botID,
		Signature: SignRequest(secret, matchID, turn, timestamp, body),
	}

	// The bot holding the delivered secret verifies the engine's request.
	if err := VerifyRequest(secret, auth, body); err != nil {
		t.Fatalf("delivered credential failed to verify request: %v", err)
	}

	// The bot signs its response; the engine verifies it.
	responseBody := []byte(`{"moves":[]}`)
	responseSig := SignResponse(secret, matchID, turn, responseBody)
	if err := VerifyResponse(secret, matchID, turn, responseSig, responseBody); err != nil {
		t.Fatalf("delivered credential failed to verify response: %v", err)
	}

	// The bot ID is not covered by any signature: possession of the shared
	// secret is the whole attestation, so a different BotID header on an
	// otherwise identical request still verifies. Identifier hygiene is the
	// verifier's prerogative, not a property of the MAC.
	auth.BotID = "b_ffffffffffff"
	if err := VerifyRequest(secret, auth, body); err != nil {
		t.Fatalf("verification must attest the secret alone, got: %v", err)
	}
}

// TestCredentialLifecycleInvalidCredential covers credentials that never
// matched: a bot provisioned with the wrong value, a bot with no secret at
// all, and malformed signatures. Every rejection must fail closed without
// echoing key material.
func TestCredentialLifecycleInvalidCredential(t *testing.T) {
	botID, platformSecret := mintCredential(t)
	_, wrongSecret := mintCredential(t) // what a mis-provisioned bot holds
	matchID := "m_lifecycle_invalid"
	turn := 3

	body, err := json.Marshal(botProtocolConformanceState(matchID, turn))
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	timestamp := time.Now().Unix()
	auth := RequestAuth{
		MatchID:   matchID,
		Turn:      turn,
		Timestamp: timestamp,
		BotID:     botID,
		Signature: SignRequest(platformSecret, matchID, turn, timestamp, body),
	}

	// A bot provisioned with the wrong secret rejects the engine's request.
	err = VerifyRequest(wrongSecret, auth, body)
	if err == nil {
		t.Fatal("mismatched credential must not verify the engine's request")
	}
	assertNoSecretIn(t, err, platformSecret, wrongSecret)

	// A bot with no secret at all refuses everything, before any
	// comparison is attempted.
	err = VerifyRequest("", auth, body)
	if err == nil {
		t.Fatal("unprovisioned (empty) secret must refuse verification")
	}
	assertNoSecretIn(t, err, platformSecret)

	// The engine likewise rejects a response signed with the wrong secret.
	responseBody := []byte(`{"moves":[]}`)
	err = VerifyResponse(platformSecret, matchID, turn,
		SignResponse(wrongSecret, matchID, turn, responseBody), responseBody)
	if err == nil {
		t.Fatal("response signed with a foreign secret must not verify")
	}
	assertNoSecretIn(t, err, platformSecret, wrongSecret)

	err = VerifyResponse("", matchID, turn,
		SignResponse(wrongSecret, matchID, turn, responseBody), responseBody)
	if err == nil {
		t.Fatal("unprovisioned (empty) secret must refuse response verification")
	}

	// Malformed signatures fail closed regardless of credential state.
	validSig := SignRequest(platformSecret, matchID, turn, timestamp, body)
	malformed := []string{
		"",
		"not-hex",
		"zzzz",
		strings.ToUpper(validSig), // uppercase hex is non-conformant
		validSig + "0",            // wrong length
		validSig[:len(validSig)-1],
	}
	for _, sig := range malformed {
		auth.Signature = sig
		if err := VerifyRequest(platformSecret, auth, body); err == nil {
			t.Errorf("malformed signature (length %d) must not verify", len(sig))
		}
	}
}

// TestCredentialLifecycleRevokedCredential covers rotation and revocation:
// once the platform replaces a secret, the replaced value authenticates
// nothing in either direction, credentials are per-bot, and the replacement
// works end-to-end once the bot redeploys with it.
func TestCredentialLifecycleRevokedCredential(t *testing.T) {
	botID, original := mintCredential(t)
	_, rotated := mintCredential(t) // what the platform holds after rotation/revocation
	matchID := "m_lifecycle_revoked"
	turn := 11

	body, err := json.Marshal(botProtocolConformanceState(matchID, turn))
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	// A bot that missed the rotation rejects requests signed with the
	// platform's rotated secret — an authentication failure, scored as a
	// failed turn.
	timestamp := time.Now().Unix()
	rotatedAuth := RequestAuth{
		MatchID:   matchID,
		Turn:      turn,
		Timestamp: timestamp,
		BotID:     botID,
		Signature: SignRequest(rotated, matchID, turn, timestamp, body),
	}
	err = VerifyRequest(original, rotatedAuth, body)
	if err == nil {
		t.Fatal("stale credential must not verify requests signed by the rotated secret")
	}
	assertNoSecretIn(t, err, original, rotated)

	// The engine rejects the stale bot's responses by the same token.
	responseBody := []byte(`{"moves":[]}`)
	err = VerifyResponse(rotated, matchID, turn,
		SignResponse(original, matchID, turn, responseBody), responseBody)
	if err == nil {
		t.Fatal("response signed with the revoked secret must not verify")
	}
	assertNoSecretIn(t, err, original, rotated)

	// After the bot redeploys with the rotated credential, both directions
	// pass again.
	if err := VerifyRequest(rotated, rotatedAuth, body); err != nil {
		t.Fatalf("rotated credential failed to verify request: %v", err)
	}
	if err := VerifyResponse(rotated, matchID, turn,
		SignResponse(rotated, matchID, turn, responseBody), responseBody); err != nil {
		t.Fatalf("rotated credential failed to verify response: %v", err)
	}

	// The original secret stays invalid — a revoked credential is not
	// resurrected by later traffic.
	err = VerifyRequest(original, rotatedAuth, body)
	if err == nil {
		t.Fatal("revoked credential must remain invalid after the rotation")
	}

	// Credentials are per-bot: another bot's secret never authenticates
	// this bot's traffic in either direction.
	_, foreign := mintCredential(t)
	foreignAuth := rotatedAuth
	foreignAuth.Signature = SignRequest(foreign, matchID, turn, timestamp, body)
	if err := VerifyRequest(original, foreignAuth, body); err == nil {
		t.Fatal("a foreign bot's secret must not verify this bot's traffic")
	}
	if err := VerifyResponse(foreign, matchID, turn,
		SignResponse(original, matchID, turn, responseBody), responseBody); err == nil {
		t.Fatal("a foreign bot's secret must not verify this bot's response")
	}
}
