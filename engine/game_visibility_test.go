package engine

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
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

type visibilityCapturingBot struct {
	state *VisibleState
}

func (b *visibilityCapturingBot) GetMoves(state *VisibleState) ([]Move, error) {
	b.state = state
	return []Move{}, nil
}

func TestMatchRunnerGivesEachBotItsOwnToroidalVisibility(t *testing.T) {
	config := DefaultConfig()
	config.Rows = 12
	config.Cols = 12
	config.VisionRadius2 = 1
	config.ZoneEnabled = false

	gs := NewGameState(config, rand.New(rand.NewSource(73)))
	player0 := gs.AddPlayer()
	player1 := gs.AddPlayer()

	gs.SpawnBot(player0.ID, Position{Row: 0, Col: 0})
	gs.SpawnBot(player0.ID, Position{Row: 5, Col: 5})
	gs.SpawnBot(player0.ID, Position{Row: 8, Col: 8})
	gs.SpawnBot(player1.ID, Position{Row: 0, Col: 1})
	gs.SpawnBot(player1.ID, Position{Row: 0, Col: 2})
	gs.SpawnBot(player1.ID, Position{Row: 5, Col: 6})
	gs.SpawnBot(player1.ID, Position{Row: 8, Col: 4})

	for _, position := range []Position{
		{Row: 0, Col: 11},
		{Row: 4, Col: 5},
		{Row: 8, Col: 7},
		{Row: 11, Col: 2},
		{Row: 7, Col: 4},
		{Row: 0, Col: 3},
	} {
		gs.AddEnergyNode(position).HasEnergy = true
	}
	gs.AddCore(player0.ID, Position{Row: 1, Col: 0})
	gs.AddCore(player0.ID, Position{Row: 6, Col: 5})
	gs.AddCore(player0.ID, Position{Row: 7, Col: 8})
	gs.AddCore(player1.ID, Position{Row: 0, Col: 3})
	gs.AddCore(player1.ID, Position{Row: 8, Col: 3})

	for _, position := range []Position{
		{Row: 11, Col: 0},
		{Row: 5, Col: 4},
		{Row: 8, Col: 9},
		{Row: 1, Col: 2},
		{Row: 7, Col: 4},
		{Row: 9, Col: 4},
	} {
		gs.Grid.SetPos(position, TileWall)
	}

	player0.Energy = 7
	player0.Score = 11
	player1.Energy = 700
	player1.Score = 701

	bots := []*visibilityCapturingBot{{}, {}}
	runner := NewMatchRunner(config, WithTimeout(time.Second))
	runner.AddBot(bots[0], "player-0")
	runner.AddBot(bots[1], "player-1")

	moves := runner.getMovesFromBots(gs)
	if len(moves) != len(bots) {
		t.Fatalf("getMovesFromBots() returned moves for %d players, want %d", len(moves), len(bots))
	}
	for playerID := range gs.Players {
		if _, ok := moves[playerID]; !ok {
			t.Errorf("getMovesFromBots() omitted player %d", playerID)
		}
	}

	state0 := bots[0].state
	if state0 == nil || state0.You.ID != player0.ID || state0.You.Energy != 7 || state0.You.Score != 11 {
		t.Fatalf("player 0 received state %#v", state0)
	}
	assertVisibleBots(t, "player 0 bots", state0.Bots, map[Position]VisibleBot{
		{Row: 0, Col: 0}: {Position: Position{Row: 0, Col: 0}, Owner: player0.ID},
		{Row: 5, Col: 5}: {Position: Position{Row: 5, Col: 5}, Owner: player0.ID},
		{Row: 8, Col: 8}: {Position: Position{Row: 8, Col: 8}, Owner: player0.ID},
		{Row: 0, Col: 1}: {Position: Position{Row: 0, Col: 1}, Owner: player1.ID},
		{Row: 5, Col: 6}: {Position: Position{Row: 5, Col: 6}, Owner: player1.ID},
	})
	assertPositions(t, "player 0 energy", state0.Energy, []Position{
		{Row: 0, Col: 11},
		{Row: 4, Col: 5},
		{Row: 8, Col: 7},
	})
	assertVisibleCores(t, state0.Cores, map[Position]VisibleCore{
		{Row: 1, Col: 0}: {Position: Position{Row: 1, Col: 0}, Owner: player0.ID, Active: true},
		{Row: 6, Col: 5}: {Position: Position{Row: 6, Col: 5}, Owner: player0.ID, Active: true},
		{Row: 7, Col: 8}: {Position: Position{Row: 7, Col: 8}, Owner: player0.ID, Active: true},
	})
	assertPositions(t, "player 0 walls", state0.Walls, []Position{
		{Row: 11, Col: 0},
		{Row: 5, Col: 4},
		{Row: 8, Col: 9},
	})
	assertVisibleBots(t, "player 0 dead", state0.Dead, map[Position]VisibleBot{})

	state1 := bots[1].state
	if state1 == nil || state1.You.ID != player1.ID || state1.You.Energy != 700 || state1.You.Score != 701 {
		t.Fatalf("player 1 received state %#v", state1)
	}
	assertVisibleBots(t, "player 1 bots", state1.Bots, map[Position]VisibleBot{
		{Row: 0, Col: 1}: {Position: Position{Row: 0, Col: 1}, Owner: player1.ID},
		{Row: 0, Col: 2}: {Position: Position{Row: 0, Col: 2}, Owner: player1.ID},
		{Row: 5, Col: 6}: {Position: Position{Row: 5, Col: 6}, Owner: player1.ID},
		{Row: 8, Col: 4}: {Position: Position{Row: 8, Col: 4}, Owner: player1.ID},
		{Row: 0, Col: 0}: {Position: Position{Row: 0, Col: 0}, Owner: player0.ID},
		{Row: 5, Col: 5}: {Position: Position{Row: 5, Col: 5}, Owner: player0.ID},
	})
	assertPositions(t, "player 1 energy", state1.Energy, []Position{
		{Row: 11, Col: 2},
		{Row: 7, Col: 4},
		{Row: 0, Col: 3},
	})
	assertVisibleCores(t, state1.Cores, map[Position]VisibleCore{
		{Row: 0, Col: 3}: {Position: Position{Row: 0, Col: 3}, Owner: player1.ID, Active: true},
		{Row: 8, Col: 3}: {Position: Position{Row: 8, Col: 3}, Owner: player1.ID, Active: true},
	})
	assertPositions(t, "player 1 walls", state1.Walls, []Position{
		{Row: 1, Col: 2},
		{Row: 7, Col: 4},
		{Row: 9, Col: 4},
	})
	assertVisibleBots(t, "player 1 dead", state1.Dead, map[Position]VisibleBot{})
}

func TestSerializedTurnPayloadCannotRevealHiddenState(t *testing.T) {
	first := newHiddenVisibilityState(false)
	second := newHiddenVisibilityState(true)

	if reflect.DeepEqual(first, second) {
		t.Fatal("test fixture did not vary hidden state")
	}

	type observation struct {
		body []byte
		err  error
	}
	observed := make(chan observation, 2)
	responseBody := []byte(`{"moves":[]}`)
	const secret = "visibility-payload-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		observed <- observation{body: body, err: err}
		turn, parseErr := strconv.Atoi(r.Header.Get("X-ACB-Turn"))
		if parseErr != nil {
			t.Errorf("X-ACB-Turn is not an integer: %v", parseErr)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-ACB-Signature", SignResponse(secret, r.Header.Get("X-ACB-Match-Id"), turn, responseBody))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(server.Close)

	bot := NewHTTPBot(server.URL, AuthConfig{
		BotID:   "b_visibility",
		Secret:  secret,
		MatchID: first.MatchID,
	})
	for _, state := range []*VisibleState{first.GetVisibleState(0), second.GetVisibleState(0)} {
		if _, err := bot.GetMoves(state); err != nil {
			t.Fatalf("GetMoves() error = %v", err)
		}
	}

	payloads := make([][]byte, 0, 2)
	for range 2 {
		request := <-observed
		if request.err != nil {
			t.Fatalf("read turn request body: %v", request.err)
		}
		payloads = append(payloads, request.body)
	}
	if !bytes.Equal(payloads[0], payloads[1]) {
		t.Fatalf("turn payloads revealed hidden state:\nfirst:  %s\nsecond: %s", payloads[0], payloads[1])
	}
}

func newHiddenVisibilityState(extraHiddenEntities bool) *GameState {
	config := DefaultConfig()
	config.Rows = 12
	config.Cols = 12
	config.VisionRadius2 = 1
	config.ZoneEnabled = false

	gs := NewGameState(config, rand.New(rand.NewSource(91)))
	gs.MatchID = "m_hidden_payload"
	gs.Turn = 17
	player0 := gs.AddPlayer()
	player1 := gs.AddPlayer()

	gs.SpawnBot(player0.ID, Position{Row: 0, Col: 0})
	gs.SpawnBot(player1.ID, Position{Row: 5, Col: 5})
	if extraHiddenEntities {
		gs.SpawnBot(player1.ID, Position{Row: 7, Col: 7})
	}

	gs.AddEnergyNode(Position{Row: 0, Col: 1}).HasEnergy = true
	gs.AddEnergyNode(Position{Row: 5, Col: 4}).HasEnergy = true
	if extraHiddenEntities {
		gs.AddEnergyNode(Position{Row: 7, Col: 6}).HasEnergy = true
	}

	gs.AddCore(player0.ID, Position{Row: 1, Col: 0})
	gs.AddCore(player1.ID, Position{Row: 5, Col: 6})
	if extraHiddenEntities {
		gs.AddCore(player1.ID, Position{Row: 7, Col: 8})
	}

	gs.Grid.SetPos(Position{Row: 11, Col: 0}, TileWall)
	gs.Grid.SetPos(Position{Row: 4, Col: 4}, TileWall)
	if extraHiddenEntities {
		gs.Grid.SetPos(Position{Row: 4, Col: 5}, TileWall)
	}

	gs.KillBot(gs.SpawnBot(player1.ID, Position{Row: 3, Col: 3}), "hidden")
	if extraHiddenEntities {
		gs.KillBot(gs.SpawnBot(player1.ID, Position{Row: 3, Col: 4}), "hidden")
	}

	player0.Energy = 12
	player0.Score = 34
	player1.Energy = 101
	player1.Score = 202
	if extraHiddenEntities {
		player1.Energy = 909
		player1.Score = 808
	}
	return gs
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
