package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Case is one golden request the suite sends to a bot.
//
// Rejection cases pin WantStatus exactly: 401 for authentication failures
// and 400 for authenticated malformed input, matching the doc's status
// pairing and the engine's reference gate. Want4xx accepts any 4xx where
// the doc leaves the code to the implementation (wrong HTTP method).
// AcceptOr401 marks the non-canonical-spelling cases: the doc lets a bot
// either reject a non-canonical but well-formed value or accept it by
// verifying the signature over the exact received spelling, so both a 401
// and a fully valid signed 200 response conform. Positive cases (Signed)
// must return 200 with a validly signed, schema-correct turn response.
type Case struct {
	Name        string
	Method      string // default POST
	Path        string // default /turn
	ContentType string // "" omits the header entirely
	Headers     map[string]string
	Body        []byte
	WantStatus  int  // exact expected status; mutually exclusive with Want4xx and AcceptOr401
	Want4xx     bool // any 4xx is acceptable
	AcceptOr401 bool // 401 (rejected) or a valid signed 200 (accepted) both conform
	Signed      bool // 200 + X-ACB-Signature over exact response bytes + valid moves
}

// baseStateJSON is the canonical request body every case starts from: all
// documented required fields present, zone attached because zone_enabled is
// true, and the optional config metadata fields included. Mutations for
// individual cases are applied on top of a decoded copy so every case ships
// exact, self-consistent raw bytes.
var baseStateJSON = []byte(`{
"match_id": "m_conformance",
"turn": 7,
"config": {
"rows": 40,
"cols": 40,
"max_turns": 500,
"vision_radius2": 49,
"attack_radius2": 25,
"spawn_cost": 3,
"energy_interval": 10,
"cores_per_player": 2,
"map_id": "map_conformance",
"season_id": "s_conformance",
"rules_version": "1",
"turn_timeout": 3000000000,
"zone_enabled": true,
"zone_start_turn": 10,
"zone_shrink_interval": 1,
"zone_shrink_step": 1,
"zone_min_radius": 2,
"kill_score": 1
},
"you": { "id": 0, "energy": 7, "score": 3 },
"bots": [
{ "position": { "row": 10, "col": 15 }, "owner": 0 }
],
"energy": [
{ "row": 20, "col": 25 }
],
"cores": [
{ "position": { "row": 5, "col": 5 }, "owner": 0, "active": true }
],
"walls": [
{ "row": 10, "col": 10 }
],
"dead": [],
"zone": {
"center": { "row": 20, "col": 20 },
"radius": 10,
"active": true
}
}`)

// stateMutation is an edit applied to a decoded copy of baseStateJSON before
// the case body is re-encoded.
type stateMutation func(state map[string]interface{})

func deleteTop(key string) stateMutation {
	return func(state map[string]interface{}) {
		delete(state, key)
	}
}

func topLevel(state map[string]interface{}) map[string]interface{} { return state }
func config(state map[string]interface{}) map[string]interface{} {
	return state["config"].(map[string]interface{})
}
func you(state map[string]interface{}) map[string]interface{} {
	return state["you"].(map[string]interface{})
}
func zone(state map[string]interface{}) map[string]interface{} {
	return state["zone"].(map[string]interface{})
}

func buildBody(mutation stateMutation) []byte {
	var state map[string]interface{}
	if err := json.Unmarshal(baseStateJSON, &state); err != nil {
		panic(fmt.Sprintf("conformance: base state does not decode: %v", err))
	}
	mutation(state)
	encoded, err := json.Marshal(state)
	if err != nil {
		panic(fmt.Sprintf("conformance: mutated state does not encode: %v", err))
	}
	return encoded
}

// authHeaders returns a complete, canonical, correctly signed header set for
// the given body and timestamp, with per-case overrides applied afterwards.
// Content-Type is deliberately not part of the set: cases declare it via
// ContentType so an omitted content type is genuinely omitted.
func authHeaders(secret, matchID, turn, timestamp string, body []byte, overrides map[string]string) map[string]string {
	headers := map[string]string{
		"X-ACB-Match-Id":  matchID,
		"X-ACB-Turn":      turn,
		"X-ACB-Timestamp": timestamp,
		"X-ACB-Bot-Id":    "b_conformance",
		"X-ACB-Signature": SignRequestString(secret, matchID, turn, timestamp, body),
	}
	for k, v := range overrides {
		if v == "" {
			delete(headers, k)
			continue
		}
		headers[k] = v
	}
	return headers
}

// resign recomputes the signature header over the final header values and
// body, honouring non-canonical spellings by signing exactly what is sent.
func resign(headers map[string]string, secret string, body []byte) {
	headers["X-ACB-Signature"] = SignRequestString(
		secret,
		headers["X-ACB-Match-Id"],
		headers["X-ACB-Turn"],
		headers["X-ACB-Timestamp"],
		body,
	)
}

// BuildCases renders the golden case table. Timestamps are anchored to now so
// freshness cases are stable regardless of when the suite runs. The match id
// and turn are the doc's worked example values.
func BuildCases(secret string, now time.Time) []Case {
	const matchID = ConformanceMatchID
	const turn = ConformanceTurn
	turnStr := fmt.Sprintf("%d", turn)
	fresh := canonicalTimestamp(now)
	stale := canonicalTimestamp(now.Add(-TimestampSkew))
	future := canonicalTimestamp(now.Add(TimestampSkew))

	fullBody := buildBody(func(state map[string]interface{}) {})
	minimalBody := buildBody(func(state map[string]interface{}) {
		cfg := config(state)
		delete(cfg, "map_id")
		delete(cfg, "season_id")
		delete(cfg, "rules_version")
		delete(cfg, "turn_timeout")
	})
	zoneOffBody := buildBody(func(state map[string]interface{}) {
		config(state)["zone_enabled"] = false
		delete(state, "zone")
	})
	trailingNewlineBody := append(append([]byte{}, fullBody...), '\n')

	cases := []Case{
		{
			Name:        "positive_canonical",
			ContentType: "application/json",
			Headers:     authHeaders(secret, matchID, turnStr, fresh, fullBody, nil),
			Body:        fullBody,
			WantStatus:  http.StatusOK,
			Signed:      true,
		},
		{
			Name:        "positive_optional_config_absent",
			ContentType: "application/json",
			Headers:     authHeaders(secret, matchID, turnStr, fresh, minimalBody, nil),
			Body:        minimalBody,
			WantStatus:  http.StatusOK,
			Signed:      true,
		},
		{
			Name:        "positive_zone_disabled_no_zone_key",
			ContentType: "application/json",
			Headers:     authHeaders(secret, matchID, turnStr, fresh, zoneOffBody, nil),
			Body:        zoneOffBody,
			WantStatus:  http.StatusOK,
			Signed:      true,
		},
		{
			Name: "positive_timestamp_near_boundary",
			// 25s old: inside the ±30s window but far enough from the edge
			// that construction and transport delay cannot push it out.
			ContentType: "application/json",
			Headers:     authHeaders(secret, matchID, turnStr, canonicalTimestamp(now.Add(-25*time.Second)), fullBody, nil),
			Body:        fullBody,
			WantStatus:  http.StatusOK,
			Signed:      true,
		},
		{
			Name: "positive_trailing_newline_bytes_signed",
			// Same parsed JSON as positive_canonical with one trailing byte
			// added: the signature covers the exact wire bytes, so a bot that
			// re-serializes JSON before hashing fails here.
			ContentType: "application/json",
			Headers:     authHeaders(secret, matchID, turnStr, fresh, trailingNewlineBody, nil),
			Body:        trailingNewlineBody,
			WantStatus:  http.StatusOK,
			Signed:      true,
		},
		{
			Name: "positive_pretty_bytes_signed",
			// The document-order, whitespace-padded spelling of the same
			// state: different bytes than the compact re-encoding every other
			// case ships, same parsed JSON. A bot that hashes its own
			// canonical re-serialization instead of the received bytes
			// rejects a correctly signed request here.
			ContentType: "application/json",
			Headers:     authHeaders(secret, matchID, turnStr, fresh, baseStateJSON, nil),
			Body:        baseStateJSON,
			WantStatus:  http.StatusOK,
			Signed:      true,
		},
	}

	// Signature tampering.
	tampered := authHeaders(secret, matchID, turnStr, fresh, fullBody, nil)
	sig := tampered["X-ACB-Signature"]
	tampered["X-ACB-Signature"] = sig[:len(sig)-1] + flipHexChar(sig[len(sig)-1])
	uppercase := authHeaders(secret, matchID, turnStr, fresh, fullBody, nil)
	uppercase["X-ACB-Signature"] = strings.ToUpper(uppercase["X-ACB-Signature"])
	short := authHeaders(secret, matchID, turnStr, fresh, fullBody, nil)
	short["X-ACB-Signature"] = short["X-ACB-Signature"][:63]
	nonHex := authHeaders(secret, matchID, turnStr, fresh, fullBody, nil)
	nonHex["X-ACB-Signature"] = "g" + nonHex["X-ACB-Signature"][1:]
	cases = append(cases,
		Case{Name: "reject_tampered_signature", ContentType: "application/json", Headers: tampered, Body: fullBody, WantStatus: http.StatusUnauthorized},
		Case{Name: "reject_signature_uppercase", ContentType: "application/json", Headers: uppercase, Body: fullBody, WantStatus: http.StatusUnauthorized},
		Case{Name: "reject_signature_wrong_length", ContentType: "application/json", Headers: short, Body: fullBody, WantStatus: http.StatusUnauthorized},
		Case{Name: "reject_signature_not_hex", ContentType: "application/json", Headers: nonHex, Body: fullBody, WantStatus: http.StatusUnauthorized},
	)

	// Missing each required header in turn. The signature is recomputed over
	// the remaining header values first, so only the absence itself can fail
	// the case.
	for _, missing := range []struct{ header, label string }{
		{"X-ACB-Match-Id", "match_id"},
		{"X-ACB-Turn", "turn"},
		{"X-ACB-Timestamp", "timestamp"},
		{"X-ACB-Bot-Id", "bot_id"},
		{"X-ACB-Signature", "signature"},
	} {
		headers := authHeaders(secret, matchID, turnStr, fresh, fullBody, nil)
		resign(headers, secret, fullBody)
		headers[missing.header] = ""
		cases = append(cases, Case{
			Name:        "reject_missing_header_" + missing.label,
			ContentType: "application/json",
			Headers:     headers,
			Body:        fullBody,
			WantStatus:  http.StatusUnauthorized,
		})
	}

	// Timestamp freshness: outside the ±30s window is an authentication
	// failure regardless of spelling.
	staleHeaders := authHeaders(secret, matchID, turnStr, stale, fullBody, nil)
	futureHeaders := authHeaders(secret, matchID, turnStr, future, fullBody, nil)
	cases = append(cases,
		Case{Name: "reject_timestamp_stale", ContentType: "application/json", Headers: staleHeaders, Body: fullBody, WantStatus: http.StatusUnauthorized},
		Case{Name: "reject_timestamp_future", ContentType: "application/json", Headers: futureHeaders, Body: fullBody, WantStatus: http.StatusUnauthorized},
	)

	// Non-canonical but well-formed spellings that parse to the intended
	// value. The doc lets a bot reject these or accept them by verifying the
	// signature over the exact received spelling, so both outcomes conform.
	tsLeadingZero := authHeaders(secret, matchID, turnStr, "0"+fresh, fullBody, nil)
	resign(tsLeadingZero, secret, fullBody)
	tsPlus := authHeaders(secret, matchID, turnStr, "+"+fresh, fullBody, nil)
	resign(tsPlus, secret, fullBody)
	turnLeadingZero := authHeaders(secret, matchID, "0"+turnStr, fresh, fullBody, nil)
	resign(turnLeadingZero, secret, fullBody)
	turnPlus := authHeaders(secret, matchID, "+"+turnStr, fresh, fullBody, nil)
	resign(turnPlus, secret, fullBody)
	turnSpace := authHeaders(secret, matchID, " "+turnStr, fresh, fullBody, nil)
	resign(turnSpace, secret, fullBody)
	cases = append(cases,
		Case{Name: "spelling_timestamp_leading_zero", ContentType: "application/json", Headers: tsLeadingZero, Body: fullBody, AcceptOr401: true},
		Case{Name: "spelling_timestamp_plus_prefix", ContentType: "application/json", Headers: tsPlus, Body: fullBody, AcceptOr401: true},
		Case{Name: "spelling_turn_leading_zero", ContentType: "application/json", Headers: turnLeadingZero, Body: fullBody, AcceptOr401: true},
		Case{Name: "spelling_turn_plus_prefix", ContentType: "application/json", Headers: turnPlus, Body: fullBody, AcceptOr401: true},
		Case{Name: "spelling_turn_whitespace", ContentType: "application/json", Headers: turnSpace, Body: fullBody, AcceptOr401: true},
	)

	// A non-canonical spelling that does not parse to the body's turn must
	// never execute under either documented handling: "08" spells 8, the
	// body says 7, and no reading of the contract accepts that pair.
	turnValueMismatch := authHeaders(secret, matchID, "08", fresh, fullBody, nil)
	resign(turnValueMismatch, secret, fullBody)
	turnNegative := authHeaders(secret, matchID, "-1", fresh, fullBody, nil)
	resign(turnNegative, secret, fullBody)
	cases = append(cases,
		Case{Name: "reject_turn_spelling_value_mismatch", ContentType: "application/json", Headers: turnValueMismatch, Body: fullBody, WantStatus: http.StatusUnauthorized},
		Case{Name: "reject_turn_negative", ContentType: "application/json", Headers: turnNegative, Body: fullBody, WantStatus: http.StatusUnauthorized},
	)

	// Body/header identity mismatches: both values present but contradictory.
	// The signature covers the header values plus the exact body bytes, so
	// only the identity check can catch these.
	mismatchTurnBody := buildBody(func(state map[string]interface{}) {})
	mismatchTurnHeaders := authHeaders(secret, matchID, "8", fresh, mismatchTurnBody, nil)
	mismatchMatchBody := buildBody(func(state map[string]interface{}) {
		topLevel(state)["match_id"] = "m_other"
	})
	mismatchMatchHeaders := authHeaders(secret, matchID, turnStr, fresh, mismatchMatchBody, nil)
	cases = append(cases,
		Case{Name: "reject_body_turn_header_mismatch", ContentType: "application/json", Headers: mismatchTurnHeaders, Body: mismatchTurnBody, WantStatus: http.StatusUnauthorized},
		Case{Name: "reject_body_match_id_header_mismatch", ContentType: "application/json", Headers: mismatchMatchHeaders, Body: mismatchMatchBody, WantStatus: http.StatusUnauthorized},
	)

	// Transport-level contract.
	getHeaders := authHeaders(secret, matchID, turnStr, fresh, fullBody, nil)
	cases = append(cases,
		Case{Name: "reject_get_turn", Method: http.MethodGet, ContentType: "application/json", Headers: getHeaders, Body: fullBody, Want4xx: true},
		Case{Name: "reject_missing_content_type", ContentType: "", Headers: authHeaders(secret, matchID, turnStr, fresh, fullBody, nil), Body: fullBody, WantStatus: http.StatusUnauthorized},
		Case{Name: "reject_wrong_content_type", ContentType: "text/plain", Headers: authHeaders(secret, matchID, turnStr, fresh, fullBody, nil), Body: fullBody, WantStatus: http.StatusUnauthorized},
	)

	// Schema violations on an otherwise authenticated body.
	unknownTop := buildBody(func(state map[string]interface{}) {
		topLevel(state)["hax"] = "surely_not_forward_compatible"
	})
	unknownConfig := buildBody(func(state map[string]interface{}) {
		config(state)["gainz"] = 1
	})
	unknownYou := buildBody(func(state map[string]interface{}) {
		you(state)["mood"] = "fine"
	})
	unknownZone := buildBody(func(state map[string]interface{}) {
		zone(state)["humidity"] = 0.4
	})
	missingWalls := buildBody(deleteTop("walls"))
	missingBodyMatchID := buildBody(deleteTop("match_id"))
	wrongTypeTurn := buildBody(func(state map[string]interface{}) {
		topLevel(state)["turn"] = "7"
	})
	malformedJSON := []byte(`{"match_id": "m_conformance", "turn": 7,`)
	notAnObject := []byte(`[{"match_id":"m_conformance","turn":7}]`)
	twoValues := append(append([]byte{}, fullBody...), []byte(` {"match_id":"m_conformance","turn":7}`)...)
	cases = append(cases,
		Case{Name: "reject_unknown_top_level_field", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, unknownTop, nil), Body: unknownTop, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_unknown_config_field", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, unknownConfig, nil), Body: unknownConfig, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_unknown_you_field", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, unknownYou, nil), Body: unknownYou, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_unknown_zone_field", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, unknownZone, nil), Body: unknownZone, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_missing_required_field_walls", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, missingWalls, nil), Body: missingWalls, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_missing_body_match_id", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, missingBodyMatchID, nil), Body: missingBodyMatchID, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_wrong_type_turn", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, wrongTypeTurn, nil), Body: wrongTypeTurn, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_malformed_json", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, malformedJSON, nil), Body: malformedJSON, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_body_not_an_object", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, notAnObject, nil), Body: notAnObject, WantStatus: http.StatusBadRequest},
		Case{Name: "reject_two_json_values", ContentType: "application/json", Headers: authHeaders(secret, matchID, turnStr, fresh, twoValues, nil), Body: twoValues, WantStatus: http.StatusBadRequest},
	)

	// Health: direct 200, no redirect, no auth headers.
	cases = append(cases, Case{
		Name:       "health_direct_200",
		Method:     http.MethodGet,
		Path:       "/health",
		WantStatus: http.StatusOK,
	})

	return cases
}

// flipHexChar flips the trailing hex digit so the tampered signature stays a
// well-formed 64-character hex string that differs only in its last byte.
func flipHexChar(c byte) string {
	if c == '0' {
		return "1"
	}
	return "0"
}
