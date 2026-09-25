package main

import "testing"

const strictGoodBody = `{"match_id":"m_7f3a9b2c","turn":42,` +
	`"config":{"rows":40,"cols":40,"max_turns":500,"vision_radius2":49,"attack_radius2":25,` +
	`"spawn_cost":3,"energy_interval":10,"cores_per_player":2,"zone_enabled":true,` +
	`"zone_start_turn":10,"zone_shrink_interval":1,"zone_shrink_step":1,"zone_min_radius":2,"kill_score":1},` +
	`"you":{"id":0,"energy":7,"score":3},` +
	`"bots":[{"position":{"row":10,"col":15},"owner":0}],` +
	`"energy":[{"row":20,"col":25}],` +
	`"cores":[{"position":{"row":5,"col":5},"owner":0,"active":true}],` +
	`"walls":[{"row":10,"col":10}],` +
	`"dead":[],` +
	`"zone":{"center":{"row":20,"col":20},"radius":10,"active":true}}`

func strictBodyWithBotElementField(field string) string {
	return `{"match_id":"m_7f3a9b2c","turn":42,` +
		`"config":{"rows":40,"cols":40,"max_turns":500,"vision_radius2":49,"attack_radius2":25,` +
		`"spawn_cost":3,"energy_interval":10,"cores_per_player":2,"zone_enabled":false},` +
		`"you":{"id":0,"energy":7,"score":3},` +
		`"bots":[{"position":{"row":10,"col":15},"owner":0,"` + field + `":1}],` +
		`"energy":[],"cores":[],"walls":[],"dead":[]}`
}

// TestDecodeStrictStateAcceptsDocumentedBody pins that a body exactly in the
// docs/bot-protocol.md shape passes strict decoding, including nested
// zone.center.
func TestDecodeStrictStateAcceptsDocumentedBody(t *testing.T) {
	if err := decodeStrictState([]byte(strictGoodBody)); err != nil {
		t.Fatalf("documented body rejected: %v", err)
	}
}

// TestDecodeStrictStateRejectsUnknownFieldsInElements pins that unknown
// fields are rejected at every nesting level, including inside the
// bots/energy/cores/walls/dead array elements and the nested zone.center
// object — the doc declares a request malformed when it "contains an
// unknown field", not merely at the top level.
func TestDecodeStrictStateRejectsUnknownFieldsInElements(t *testing.T) {
	cases := map[string]string{
		"bots element":       strictBodyWithBotElementField("sneaky"),
		"cores element":      `{"match_id":"m","turn":1,"config":{"rows":1,"cols":1,"max_turns":1,"vision_radius2":1,"attack_radius2":1,"spawn_cost":1,"energy_interval":1,"cores_per_player":1,"zone_enabled":false},"you":{"id":0,"energy":1,"score":0},"bots":[],"energy":[],"cores":[{"position":{"row":0,"col":0},"owner":0,"active":true,"extra":1}],"walls":[],"dead":[]}`,
		"energy element":     `{"match_id":"m","turn":1,"config":{"rows":1,"cols":1,"max_turns":1,"vision_radius2":1,"attack_radius2":1,"spawn_cost":1,"energy_interval":1,"cores_per_player":1,"zone_enabled":false},"you":{"id":0,"energy":1,"score":0},"bots":[],"energy":[{"row":0,"col":0,"zzz":0}],"cores":[],"walls":[],"dead":[]}`,
		"zone.center nested": `{"match_id":"m","turn":1,"config":{"rows":1,"cols":1,"max_turns":1,"vision_radius2":1,"attack_radius2":1,"spawn_cost":1,"energy_interval":1,"cores_per_player":1,"zone_enabled":true,"zone_start_turn":1,"zone_shrink_interval":1,"zone_shrink_step":1,"zone_min_radius":1,"kill_score":1},"you":{"id":0,"energy":1,"score":0},"bots":[],"energy":[],"cores":[],"walls":[],"dead":[],"zone":{"center":{"row":1,"col":1,"zzz":0},"radius":1,"active":true}}`,
	}
	for name, body := range cases {
		if err := decodeStrictState([]byte(body)); err == nil {
			t.Errorf("unknown field inside %s was accepted", name)
		}
	}
}
