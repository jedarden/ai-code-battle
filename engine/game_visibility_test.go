package engine

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

func TestGetVisibleStateFiltersCollectiveToroidalVisibility(t *testing.T) {
	config := DefaultConfig()
	config.Rows = 12
	config.Cols = 20
	config.VisionRadius2 = 9
	config.ZoneEnabled = false

	gs := NewGameState(config, rand.New(rand.NewSource(42)))
	player0 := gs.AddPlayer()
	player1 := gs.AddPlayer()

	gs.SpawnBot(player0.ID, Position{Row: 11, Col: 0})
	gs.SpawnBot(player0.ID, Position{Row: 0, Col: 19})
	gs.SpawnBot(player1.ID, Position{Row: 11, Col: 3})
	gs.SpawnBot(player1.ID, Position{Row: 3, Col: 19})
	gs.SpawnBot(player1.ID, Position{Row: 6, Col: 10})

	gs.KillBot(gs.SpawnBot(player1.ID, Position{Row: 10, Col: 2}), "visible")
	gs.KillBot(gs.SpawnBot(player1.ID, Position{Row: 2, Col: 18}), "visible")
	gs.KillBot(gs.SpawnBot(player1.ID, Position{Row: 6, Col: 7}), "hidden")

	addEnergy := func(row, col int, hasEnergy bool) {
		gs.AddEnergyNode(Position{Row: row, Col: col}).HasEnergy = hasEnergy
	}
	addEnergy(10, 1, true)
	addEnergy(2, 19, true)
	addEnergy(0, 0, true)
	addEnergy(6, 11, true)
	addEnergy(11, 2, false)

	addCore := func(owner, row, col int, active bool) {
		gs.AddCore(owner, Position{Row: row, Col: col}).Active = active
	}
	addCore(player0.ID, 11, 1, true)
	addCore(player1.ID, 0, 17, true)
	addCore(player0.ID, 0, 1, true)
	addCore(player1.ID, 9, 19, false)
	addCore(player1.ID, 6, 9, true)

	gs.Grid.Set(9, 0, TileWall)
	gs.Grid.Set(0, 16, TileWall)
	gs.Grid.Set(11, 19, TileWall)
	gs.Grid.Set(6, 8, TileWall)

	player0.Energy = 7
	player0.Score = 3
	player1.Energy = 997
	player1.Score = 991
	gs.Turn = 42
	gs.MatchID = "m_visibility"

	state := gs.GetVisibleState(player0.ID)

	if state.MatchID != "m_visibility" {
		t.Errorf("MatchID = %q, want %q", state.MatchID, "m_visibility")
	}
	if state.Turn != 42 {
		t.Errorf("Turn = %d, want 42", state.Turn)
	}
	if state.Config != config {
		t.Errorf("Config = %#v, want %#v", state.Config, config)
	}
	if state.You.ID != player0.ID || state.You.Energy != 7 || state.You.Score != 3 {
		t.Errorf("You = %#v, want player 0 with energy 7 and score 3", state.You)
	}

	assertVisibleBots(t, "Bots", state.Bots, map[Position]VisibleBot{
		{Row: 11, Col: 0}: {Position: Position{Row: 11, Col: 0}, Owner: player0.ID},
		{Row: 0, Col: 19}: {Position: Position{Row: 0, Col: 19}, Owner: player0.ID},
		{Row: 11, Col: 3}: {Position: Position{Row: 11, Col: 3}, Owner: player1.ID},
		{Row: 3, Col: 19}: {Position: Position{Row: 3, Col: 19}, Owner: player1.ID},
	})
	assertVisibleBots(t, "Dead", state.Dead, map[Position]VisibleBot{
		{Row: 10, Col: 2}: {Position: Position{Row: 10, Col: 2}, Owner: player1.ID},
		{Row: 2, Col: 18}: {Position: Position{Row: 2, Col: 18}, Owner: player1.ID},
	})
	assertPositions(t, "Energy", state.Energy, []Position{
		{Row: 10, Col: 1},
		{Row: 2, Col: 19},
		{Row: 0, Col: 0},
	})
	assertVisibleCores(t, state.Cores, map[Position]VisibleCore{
		{Row: 11, Col: 1}: {Position: Position{Row: 11, Col: 1}, Owner: player0.ID, Active: true},
		{Row: 0, Col: 17}: {Position: Position{Row: 0, Col: 17}, Owner: player1.ID, Active: true},
		{Row: 0, Col: 1}:  {Position: Position{Row: 0, Col: 1}, Owner: player0.ID, Active: true},
		{Row: 9, Col: 19}: {Position: Position{Row: 9, Col: 19}, Owner: player1.ID, Active: false},
	})
	assertPositions(t, "Walls", state.Walls, []Position{
		{Row: 9, Col: 0},
		{Row: 0, Col: 16},
		{Row: 11, Col: 19},
	})
	if state.Zone != nil {
		t.Errorf("Zone = %#v, want nil when zone is disabled", state.Zone)
	}

	payload, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(payload), `"row":6`) {
		t.Errorf("visible state leaked an entity on hidden row 6: %s", payload)
	}
}

func assertVisibleBots(t *testing.T, name string, got []VisibleBot, want map[Position]VisibleBot) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("len(%s) = %d, want %d: %#v", name, len(got), len(want), got)
	}

	gotByPosition := make(map[Position]VisibleBot, len(got))
	for _, bot := range got {
		if _, exists := gotByPosition[bot.Position]; exists {
			t.Errorf("%s contains duplicate position %v", name, bot.Position)
		}
		gotByPosition[bot.Position] = bot
	}
	for position, wantBot := range want {
		gotBot, exists := gotByPosition[position]
		if !exists {
			t.Errorf("%s is missing visible bot at %v", name, position)
			continue
		}
		if gotBot != wantBot {
			t.Errorf("%s at %v = %#v, want %#v", name, position, gotBot, wantBot)
		}
	}
}

func assertVisibleCores(t *testing.T, got []VisibleCore, want map[Position]VisibleCore) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("len(Cores) = %d, want %d: %#v", len(got), len(want), got)
	}

	gotByPosition := make(map[Position]VisibleCore, len(got))
	for _, core := range got {
		if _, exists := gotByPosition[core.Position]; exists {
			t.Errorf("Cores contains duplicate position %v", core.Position)
		}
		gotByPosition[core.Position] = core
	}
	for position, wantCore := range want {
		gotCore, exists := gotByPosition[position]
		if !exists {
			t.Errorf("Cores is missing visible core at %v", position)
			continue
		}
		if gotCore != wantCore {
			t.Errorf("Cores at %v = %#v, want %#v", position, gotCore, wantCore)
		}
	}
}

func assertPositions(t *testing.T, name string, got []Position, want []Position) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("len(%s) = %d, want %d: %#v", name, len(got), len(want), got)
	}

	gotSet := make(map[Position]bool, len(got))
	for _, position := range got {
		if gotSet[position] {
			t.Errorf("%s contains duplicate position %v", name, position)
		}
		gotSet[position] = true
	}
	for _, position := range want {
		if !gotSet[position] {
			t.Errorf("%s is missing visible position %v", name, position)
		}
	}
}
