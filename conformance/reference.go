package conformance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ReferenceBotHandler is an executable reading of docs/bot-protocol.md: the
// verification order, the documented statuses per rejection class, raw-byte
// signature verification, and strict schema enforcement. The suite must pass
// cleanly against it (TestSuiteAgainstReference) and the engine's own signer
// must agree with it (TestGoldenVectors). It is also usable as a local
// stand-in bot when exercising the harness itself.
func ReferenceBotHandler(secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return
		}
		if r.URL.Path != "/turn" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// Step 1: transport.
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}

		// Step 2: retain the unparsed bytes.
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Step 3: headers, spelling, freshness.
		matchID := r.Header.Get("X-ACB-Match-Id")
		turnStr := r.Header.Get("X-ACB-Turn")
		timestamp := r.Header.Get("X-ACB-Timestamp")
		botID := r.Header.Get("X-ACB-Bot-Id")
		signature := r.Header.Get("X-ACB-Signature")
		if matchID == "" || turnStr == "" || timestamp == "" || botID == "" || signature == "" {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		if !canonicalBase10(turnStr) {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		turn, err := strconv.Atoi(turnStr)
		if err != nil || turn < 0 {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		if !canonicalBase10(timestamp) {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		timestampUnix, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		requestTime := time.Unix(timestampUnix, 0)
		now := time.Now()
		if requestTime.Before(now.Add(-30*time.Second)) || requestTime.After(now.Add(30*time.Second)) {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}
		if !isLowercaseHex64(signature) {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}

		// Step 4: signature over the exact retained bytes.
		if SignRequestString(secret, matchID, turnStr, timestamp, raw) != signature {
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}

		// Step 5: strict schema, then identity.
		state, err := decodeStrictState(raw)
		if err != nil {
			http.Error(w, "invalid game state: "+err.Error(), http.StatusBadRequest)
			return
		}
		if state.MatchID != matchID || state.Turn != turn {
			http.Error(w, "request identity mismatch", http.StatusUnauthorized)
			return
		}

		// Step 6: execute.
		responseBody := buildReferenceResponse(state)
		bodyHash := sha256.Sum256(responseBody)
		responseSig := signPayload(secret, fmt.Sprintf("%s.%d.%s", state.MatchID, state.Turn, hex.EncodeToString(bodyHash[:])))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-ACB-Signature", responseSig)
		_, _ = w.Write(responseBody)
	})
}

// referenceState is the decoded, schema-validated request.
type referenceState struct {
	MatchID string
	Turn    int
	YouID   int
}

func canonicalBase10(s string) bool {
	if s == "" {
		return false
	}
	if s == "0" {
		return true
	}
	if s[0] < '1' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isLowercaseHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// decodeStrictState enforces the documented request schema: exactly one JSON
// object, all required fields present with the right types, no unknown
// fields anywhere.
func decodeStrictState(raw []byte) (referenceState, error) {
	var state referenceState
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var parsed struct {
		MatchID string `json:"match_id"`
		Turn    *int   `json:"turn"`
		Config  *struct {
			Rows               *int   `json:"rows"`
			Cols               *int   `json:"cols"`
			MaxTurns           *int   `json:"max_turns"`
			VisionRadius2      *int   `json:"vision_radius2"`
			AttackRadius2      *int   `json:"attack_radius2"`
			SpawnCost          *int   `json:"spawn_cost"`
			EnergyInterval     *int   `json:"energy_interval"`
			CoresPerPlayer     *int   `json:"cores_per_player"`
			MapID              string `json:"map_id,omitempty"`
			SeasonID           string `json:"season_id,omitempty"`
			RulesVersion       string `json:"rules_version,omitempty"`
			TurnTimeout        *int64 `json:"turn_timeout,omitempty"`
			ZoneEnabled        *bool  `json:"zone_enabled"`
			ZoneStartTurn      *int   `json:"zone_start_turn"`
			ZoneShrinkInterval *int   `json:"zone_shrink_interval"`
			ZoneShrinkStep     *int   `json:"zone_shrink_step"`
			ZoneMinRadius      *int   `json:"zone_min_radius"`
			KillScore          *int   `json:"kill_score"`
		} `json:"config"`
		You *struct {
			ID     *int `json:"id"`
			Energy *int `json:"energy"`
			Score  *int `json:"score"`
		} `json:"you"`
		Bots   *[]referenceBot   `json:"bots"`
		Energy *[]referencePoint `json:"energy"`
		Cores  *[]referenceCore  `json:"cores"`
		Walls  *[]referencePoint `json:"walls"`
		Dead   *[]referenceBot   `json:"dead"`
		Zone   *struct {
			Center struct {
				Row int `json:"row"`
				Col int `json:"col"`
			} `json:"center"`
			Radius *int  `json:"radius"`
			Active *bool `json:"active"`
		} `json:"zone,omitempty"`
	}
	if err := decoder.Decode(&parsed); err != nil {
		return state, fmt.Errorf("schema: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return state, fmt.Errorf("schema: expected exactly one JSON object")
	}
	if parsed.MatchID == "" {
		return state, fmt.Errorf("schema: match_id is required")
	}
	if parsed.Turn == nil {
		return state, fmt.Errorf("schema: turn is required")
	}
	if parsed.Config == nil {
		return state, fmt.Errorf("schema: config is required")
	}
	cfg := parsed.Config
	if cfg.Rows == nil || cfg.Cols == nil || cfg.MaxTurns == nil || cfg.VisionRadius2 == nil ||
		cfg.AttackRadius2 == nil || cfg.SpawnCost == nil || cfg.EnergyInterval == nil ||
		cfg.CoresPerPlayer == nil || cfg.ZoneEnabled == nil || cfg.ZoneStartTurn == nil ||
		cfg.ZoneShrinkInterval == nil || cfg.ZoneShrinkStep == nil || cfg.ZoneMinRadius == nil ||
		cfg.KillScore == nil {
		return state, fmt.Errorf("schema: config is missing required fields")
	}
	if parsed.You == nil || parsed.You.ID == nil || parsed.You.Energy == nil || parsed.You.Score == nil {
		return state, fmt.Errorf("schema: you is missing required fields")
	}
	if parsed.Bots == nil || parsed.Energy == nil || parsed.Cores == nil || parsed.Walls == nil || parsed.Dead == nil {
		return state, fmt.Errorf("schema: bots, energy, cores, walls, dead are required")
	}
	state.MatchID = parsed.MatchID
	state.Turn = *parsed.Turn
	state.YouID = *parsed.You.ID
	return state, nil
}

type referenceBot struct {
	Position struct {
		Row int `json:"row"`
		Col int `json:"col"`
	} `json:"position"`
	Owner *int `json:"owner"`
}

type referencePoint struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type referenceCore struct {
	Position struct {
		Row int `json:"row"`
		Col int `json:"col"`
	} `json:"position"`
	Owner  *int  `json:"owner"`
	Active *bool `json:"active"`
}

// buildReferenceResponse holds every living own unit in place — a valid,
// signed, schema-correct response without strategy opinions.
func buildReferenceResponse(state referenceState) []byte {
	type position struct {
		Row int `json:"row"`
		Col int `json:"col"`
	}
	type move struct {
		Position  position `json:"position"`
		Direction string   `json:"direction"`
	}
	response := struct {
		Moves []move `json:"moves"`
	}{Moves: []move{}}
	_ = state.YouID // a hold-in-place response is valid regardless of unit ownership
	encoded, err := json.Marshal(response)
	if err != nil {
		panic(fmt.Sprintf("conformance: reference response does not encode: %v", err))
	}
	return encoded
}
