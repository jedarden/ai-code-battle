package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// decodeStrictState enforces the request schema from docs/bot-protocol.md:
// exactly one JSON object, every required field present with the right JSON
// type, optional config metadata accepted, and no unknown fields anywhere —
// including inside the bots, energy, cores, walls and dead elements, so the
// shapes here mirror the conformance reference handler.
// It runs after signature verification, so a decoding failure is
// authenticated malformed input (400), never an authentication failure.
func decodeStrictState(raw []byte) error {
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
		Bots   *[]protocolBot   `json:"bots"`
		Energy *[]protocolPoint `json:"energy"`
		Cores  *[]protocolCore  `json:"cores"`
		Walls  *[]protocolPoint `json:"walls"`
		Dead   *[]protocolBot   `json:"dead"`
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
		return fmt.Errorf("schema: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("schema: expected exactly one JSON object")
	}
	if parsed.MatchID == "" {
		return fmt.Errorf("schema: match_id is required")
	}
	if parsed.Turn == nil {
		return fmt.Errorf("schema: turn is required")
	}
	if parsed.Config == nil {
		return fmt.Errorf("schema: config is required")
	}
	cfg := parsed.Config
	if cfg.Rows == nil || cfg.Cols == nil || cfg.MaxTurns == nil || cfg.VisionRadius2 == nil ||
		cfg.AttackRadius2 == nil || cfg.SpawnCost == nil || cfg.EnergyInterval == nil ||
		cfg.CoresPerPlayer == nil || cfg.ZoneEnabled == nil || cfg.ZoneStartTurn == nil ||
		cfg.ZoneShrinkInterval == nil || cfg.ZoneShrinkStep == nil || cfg.ZoneMinRadius == nil ||
		cfg.KillScore == nil {
		return fmt.Errorf("schema: config is missing required fields")
	}
	if parsed.You == nil || parsed.You.ID == nil || parsed.You.Energy == nil || parsed.You.Score == nil {
		return fmt.Errorf("schema: you is missing required fields")
	}
	if parsed.Bots == nil || parsed.Energy == nil || parsed.Cores == nil || parsed.Walls == nil || parsed.Dead == nil {
		return fmt.Errorf("schema: bots, energy, cores, walls, dead are required")
	}
	return nil
}

// protocolBot is one element of the bots or dead arrays: a position plus an
// owning player.
type protocolBot struct {
	Position struct {
		Row int `json:"row"`
		Col int `json:"col"`
	} `json:"position"`
	Owner *int `json:"owner"`
}

// protocolPoint is one element of the energy or walls arrays.
type protocolPoint struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// protocolCore is one element of the cores array.
type protocolCore struct {
	Position struct {
		Row int `json:"row"`
		Col int `json:"col"`
	} `json:"position"`
	Owner  *int  `json:"owner"`
	Active *bool `json:"active"`
}
