package engine

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	// TimestampTolerance is the allowed clock skew for request validation (30 seconds)
	TimestampTolerance = 30 * time.Second
)

// AuthConfig holds authentication configuration for a bot.
type AuthConfig struct {
	BotID   string // Unique bot identifier (e.g., "b_4e8c1d2f")
	Secret  string // Shared secret; its UTF-8 bytes are used directly as the HMAC key
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
	if signature != strings.ToLower(signature) {
		return nil, fmt.Errorf("signature must use lowercase hex")
	}
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
	decoder := json.NewDecoder(bytes.NewReader(requestBody))
	decoder.DisallowUnknownFields()
	var state VisibleState
	if err := decoder.Decode(&state); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid request body: multiple JSON values")
		}
		return fmt.Errorf("invalid request body: %w", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &fields); err != nil || fields == nil {
		return fmt.Errorf("invalid request body: expected one JSON object")
	}
	if err := requireRequestFields(requestBody, "request", "match_id", "turn", "config", "you", "bots", "energy", "cores", "walls", "dead"); err != nil {
		return err
	}
	if err := rejectRequestUnknownFields(requestBody, "request", "match_id", "turn", "config", "you", "bots", "energy", "cores", "walls", "dead", "zone"); err != nil {
		return err
	}
	if state.MatchID != auth.MatchID {
		return fmt.Errorf("request match_id does not match X-ACB-Match-Id")
	}
	if state.Turn != auth.Turn {
		return fmt.Errorf("request turn does not match X-ACB-Turn")
	}

	if err := requireRequestFields(fields["config"], "request.config", "rows", "cols", "max_turns", "vision_radius2", "attack_radius2", "spawn_cost", "energy_interval", "cores_per_player", "zone_enabled", "zone_start_turn", "zone_shrink_interval", "zone_shrink_step", "zone_min_radius", "kill_score"); err != nil {
		return err
	}
	if err := rejectRequestUnknownFields(fields["config"], "request.config", "rows", "cols", "max_turns", "vision_radius2", "attack_radius2", "spawn_cost", "energy_interval", "cores_per_player", "map_id", "season_id", "rules_version", "turn_timeout", "zone_enabled", "zone_start_turn", "zone_shrink_interval", "zone_shrink_step", "zone_min_radius", "kill_score"); err != nil {
		return err
	}
	if err := requireRequestFields(fields["you"], "request.you", "id", "energy", "score"); err != nil {
		return err
	}
	if err := rejectRequestUnknownFields(fields["you"], "request.you", "id", "energy", "score"); err != nil {
		return err
	}
	if err := requireRequestElements(fields["bots"], "request.bots", "position", "owner"); err != nil {
		return err
	}
	if err := requireRequestPositionElements(fields["energy"], "request.energy"); err != nil {
		return err
	}
	if err := requireRequestElements(fields["cores"], "request.cores", "position", "owner", "active"); err != nil {
		return err
	}
	if err := requireRequestPositionElements(fields["walls"], "request.walls"); err != nil {
		return err
	}
	if err := requireRequestElements(fields["dead"], "request.dead", "position", "owner"); err != nil {
		return err
	}

	rawZone, hasZone := fields["zone"]
	if state.Config.ZoneEnabled && !hasZone {
		return fmt.Errorf("request must include zone when zone_enabled is true")
	}
	if !state.Config.ZoneEnabled && hasZone {
		return fmt.Errorf("request must omit zone when zone_enabled is false")
	}
	if hasZone {
		if err := requireRequestFields(rawZone, "request.zone", "center", "radius", "active"); err != nil {
			return err
		}
		if err := rejectRequestUnknownFields(rawZone, "request.zone", "center", "radius", "active"); err != nil {
			return err
		}
		var zone map[string]json.RawMessage
		if err := json.Unmarshal(rawZone, &zone); err != nil {
			return fmt.Errorf("invalid request.zone: %w", err)
		}
		if err := requireRequestFields(zone["center"], "request.zone.center", "row", "col"); err != nil {
			return err
		}
		if err := rejectRequestUnknownFields(zone["center"], "request.zone.center", "row", "col"); err != nil {
			return err
		}
	}
	return nil
}

func requireRequestFields(raw json.RawMessage, context string, names ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return fmt.Errorf("%s must be a JSON object", context)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s must include %s", context, name)
		}
	}
	for name, value := range object {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s.%s must not be null", context, name)
		}
	}
	return nil
}

func rejectRequestUnknownFields(raw json.RawMessage, context string, names ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return fmt.Errorf("%s must be a JSON object", context)
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	for name := range object {
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("%s contains unknown field %s", context, name)
		}
	}
	return nil
}

func requireRequestElements(raw json.RawMessage, context string, names ...string) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%s must be an array", context)
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil || elements == nil {
		return fmt.Errorf("%s must be an array", context)
	}
	for i, element := range elements {
		elementContext := fmt.Sprintf("%s[%d]", context, i)
		if err := requireRequestFields(element, elementContext, names...); err != nil {
			return err
		}
		if err := rejectRequestUnknownFields(element, elementContext, names...); err != nil {
			return err
		}
		if err := requireRequestFields(elementValue(element, "position"), elementContext+".position", "row", "col"); err != nil {
			return err
		}
		if err := rejectRequestUnknownFields(elementValue(element, "position"), elementContext+".position", "row", "col"); err != nil {
			return err
		}
	}
	return nil
}

func requireRequestPositionElements(raw json.RawMessage, context string) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%s must be an array", context)
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil || elements == nil {
		return fmt.Errorf("%s must be an array", context)
	}
	for i, element := range elements {
		elementContext := fmt.Sprintf("%s[%d]", context, i)
		if err := requireRequestFields(element, elementContext, "row", "col"); err != nil {
			return err
		}
		if err := rejectRequestUnknownFields(element, elementContext, "row", "col"); err != nil {
			return err
		}
	}
	return nil
}

func elementValue(raw json.RawMessage, name string) json.RawMessage {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil
	}
	return object[name]
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
