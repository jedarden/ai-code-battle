package main

// Prediction route tests (aicodeba-7f5b3b0e).
//
// POST /api/predict, GET /api/predictions/open, and
// GET /api/predictions/history are the prediction half of the match-tier
// contract (docs/bot-protocol.md §14; the Pages function answers 503
// match_tier_offline for them until acb-api revives). These tests run the
// handlers against the production schema (ensureSchema plus the
// predictable-flag migration) so the responses they pin are the ones a
// revived deployment would give.
//
// DB-backed cases skip without ACB_TEST_DATABASE_URL, matching
// rotatekey_test.go.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// predictionSchemaOnce guards the schema application: the 0002 migration is
// not idempotent (no IF NOT EXISTS on its ALTER/CREATE INDEX), so it must
// run exactly once against the shared test database.
var predictionSchemaOnce sync.Once

// setupPredictionSchema applies the production schema and the idempotent
// predictable migration (migrations/0002) used by deployment tooling. The
// API bootstrap also creates this column so a fresh service cannot expose a
// prediction route that fails against its own schema.
func setupPredictionSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	var schemaErr error
	predictionSchemaOnce.Do(func() {
		schemaErr = ensureSchema(context.Background(), db)
		if schemaErr != nil {
			return
		}
		var mig []byte
		mig, schemaErr = os.ReadFile(filepath.Join("..", "..", "migrations", "0002_add_predictable_flag.sql"))
		if schemaErr != nil {
			return
		}
		_, schemaErr = db.Exec(string(mig))
	})
	if schemaErr != nil {
		t.Fatalf("apply production schema + predictable migration: %v", schemaErr)
	}
}

// insertPredictionBot adds a bot row satisfying the production schema's
// NOT NULL columns (owner and endpoint_url have no defaults there).
func insertPredictionBot(t *testing.T, db *sql.DB, botID, name string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO bots (bot_id, name, owner, endpoint_url, shared_secret, status)
		VALUES ($1, $2, 'test', 'http://127.0.0.1:1', 'secret', 'active')
		ON CONFLICT (bot_id) DO UPDATE SET name = EXCLUDED.name`,
		botID, name)
	if err != nil {
		t.Fatalf("insert bot %s: %v", botID, err)
	}
}

// insertMatch adds a match with the given participants (slot = index) and
// returns it ready for prediction flows.
func insertMatch(t *testing.T, db *sql.DB, matchID, status string, predictable bool, botIDs ...string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO matches (match_id, map_id, status, predictable) VALUES ($1, 'map_test', $2, $3)
		ON CONFLICT (match_id) DO UPDATE SET status = EXCLUDED.status, predictable = EXCLUDED.predictable`,
		matchID, status, predictable)
	if err != nil {
		t.Fatalf("insert match %s: %v", matchID, err)
	}
	for slot, botID := range botIDs {
		_, err := db.Exec(`INSERT INTO match_participants (match_id, bot_id, player_slot) VALUES ($1, $2, $3)
			ON CONFLICT (match_id, bot_id) DO UPDATE SET player_slot = EXCLUDED.player_slot`,
			matchID, botID, slot)
		if err != nil {
			t.Fatalf("insert participant %s/%s: %v", matchID, botID, err)
		}
	}
}

// getPrediction drives a GET against the prediction read routes.
func getPrediction(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// TestPredictRoute tests that POST /api/predict is registered (no DB needed).
func TestPredictRoute(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(""))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("POST /api/predict returned 404 — route not registered")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/predict with empty body: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestPredict_MissingFields(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	for _, body := range []string{
		`{"bot_id":"b_x","predictor_id":"p"}`,
		`{"match_id":"m1","predictor_id":"p"}`,
		`{"match_id":"m1","bot_id":"b_x"}`,
	} {
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)
		req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("predict %s: status = %d, want 400; body = %s", body, w.Code, w.Body.String())
		}
	}
}

func TestPredict_ConfidenceBounds(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	for _, conf := range []string{"0", "101", "-5"} {
		body := `{"match_id":"m1","bot_id":"b_x","predictor_id":"p","confidence":` + conf + `}`
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)
		req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("confidence %s: status = %d, want 400; body = %s", conf, w.Code, w.Body.String())
		}
	}
}

func TestPredict_MatchNotFound(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	body := `{"match_id":"m_missing","bot_id":"b_x","predictor_id":"p"}`
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown match: status = %d, want 404; body = %s", w.Code, w.Body.String())
	}
}

func TestPredict_MatchStarted(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)
	insertPredictionBot(t, db, "b_running", "Runner")
	insertMatch(t, db, "m_started", "complete", false, "b_running")

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	body := `{"match_id":"m_started","bot_id":"b_running","predictor_id":"p"}`
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("started match: status = %d, want 409; body = %s", w.Code, w.Body.String())
	}
}

func TestPredict_NotPredictable(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)
	insertPredictionBot(t, db, "b_plain", "Plain")
	insertMatch(t, db, "m_plain", "pending", false, "b_plain")

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	body := `{"match_id":"m_plain","bot_id":"b_plain","predictor_id":"p"}`
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unflagged match: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not open for predictions") {
		t.Errorf("unflagged match body = %s, want the not-open message", w.Body.String())
	}
}

func TestPredict_NotParticipant(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)
	insertPredictionBot(t, db, "b_in", "InMatch")
	insertPredictionBot(t, db, "b_out", "Outsider")
	insertMatch(t, db, "m_part", "pending", true, "b_in")

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	body := `{"match_id":"m_part","bot_id":"b_out","predictor_id":"p"}`
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-participant: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not a participant") {
		t.Errorf("non-participant body = %s, want the participant message", w.Body.String())
	}
}

func TestPredict_SuccessAndRepick(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)
	insertPredictionBot(t, db, "b_alpha", "Alpha")
	insertPredictionBot(t, db, "b_beta", "Beta")
	insertMatch(t, db, "m_repick", "pending", true, "b_alpha", "b_beta")

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/predict", strings.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}

	w := post(`{"match_id":"m_repick","bot_id":"b_alpha","predictor_id":"fan_repick","confidence":72}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("predict: status = %d, want 201; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode predict response: %v", err)
	}
	if resp["match_id"] != "m_repick" || resp["predicted"] != "b_alpha" || resp["predictor"] != "fan_repick" {
		t.Errorf("predict response echo = %v, want the submitted pick echoed", resp)
	}
	if resp["confidence"] != float64(72) {
		t.Errorf("predict response confidence = %v, want 72", resp["confidence"])
	}
	if _, ok := resp["id"].(float64); !ok {
		t.Errorf("predict response id missing or not numeric: %v", resp["id"])
	}

	// A second pick by the same predictor replaces the first (UNIQUE
	// match_id+predictor_id, upsert), including dropping the confidence when
	// it is not resubmitted.
	w2 := post(`{"match_id":"m_repick","bot_id":"b_beta","predictor_id":"fan_repick"}`)
	if w2.Code != http.StatusCreated {
		t.Fatalf("repick: status = %d, want 201; body = %s", w2.Code, w2.Body.String())
	}
	var count int
	var predicted string
	if err := db.QueryRow(`SELECT COUNT(*), MIN(predicted_bot) FROM predictions WHERE match_id = 'm_repick' AND predictor_id = 'fan_repick'`).Scan(&count, &predicted); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("prediction rows for fan_repick = %d, want 1 (repick replaces, not duplicates)", count)
	}
	if predicted != "b_beta" {
		t.Errorf("repick stored predicted_bot = %q, want b_beta", predicted)
	}
}

func TestOpenPredictions(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)
	insertPredictionBot(t, db, "b_alpha", "Alpha")
	insertPredictionBot(t, db, "b_beta", "Beta")
	insertMatch(t, db, "m_view", "pending", true, "b_alpha", "b_beta")
	insertMatch(t, db, "m_not_predictable", "pending", false, "b_alpha", "b_beta")
	insertMatch(t, db, "m_done", "complete", true, "b_alpha", "b_beta")

	// fan1 picked alpha in the open match.
	_, err := db.Exec(`INSERT INTO predictions (match_id, predictor_id, predicted_bot, confidence)
		VALUES ('m_view', 'fan_view', 'b_alpha', 60)`)
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}
	w := getPrediction(t, srv, "/api/predictions/open?predictor_id=fan_view")
	if w.Code != http.StatusOK {
		t.Fatalf("open predictions: status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Matches []struct {
			MatchID      string                   `json:"match_id"`
			Participants []map[string]interface{} `json:"participants"`
			YourPick     *string                  `json:"your_pick"`
		} `json:"matches"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode open predictions: %v", err)
	}

	// The database is shared with the other tests' pending matches, so the
	// list can be longer than one — what matters is that m_view is listed
	// with its participants and the caller's pick, and the completed match
	// is not listed at all.
	var view *struct {
		MatchID      string                   `json:"match_id"`
		Participants []map[string]interface{} `json:"participants"`
		YourPick     *string                  `json:"your_pick"`
	}
	for i := range resp.Matches {
		switch resp.Matches[i].MatchID {
		case "m_view":
			view = &resp.Matches[i]
		case "m_done":
			t.Error("completed match m_done listed as open for predictions")
		case "m_not_predictable":
			t.Error("unflagged match m_not_predictable listed as open for predictions")
		}
	}
	if view == nil {
		t.Fatalf("m_view missing from open matches: %+v", resp.Matches)
	}
	if len(view.Participants) != 2 {
		t.Errorf("participants = %d, want 2", len(view.Participants))
	}
	if view.YourPick == nil || *view.YourPick != "b_alpha" {
		t.Errorf("your_pick = %v, want b_alpha", view.YourPick)
	}
}

func TestPredictionHistory(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupPredictionSchema(t, db)
	insertPredictionBot(t, db, "b_alpha", "Alpha")
	insertPredictionBot(t, db, "b_beta", "Beta")
	insertMatch(t, db, "m_done", "complete", true, "b_alpha", "b_beta")
	// Alpha won slot 0.
	if _, err := db.Exec(`UPDATE matches SET winner = 0 WHERE match_id = 'm_done'`); err != nil {
		t.Fatal(err)
	}
	// fan1 correctly picked alpha; fan2 wrongly picked beta.
	_, err := db.Exec(`INSERT INTO predictions (match_id, predictor_id, predicted_bot, confidence, correct, resolved_at)
		VALUES ('m_done', 'fan1', 'b_alpha', 80, TRUE, NOW()),
		       ('m_done', 'fan2', 'b_beta', 20, FALSE, NOW())`)
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, predictLtr: newTestRegLimiter(), db: db}

	// predictor_id is mandatory.
	wNone := getPrediction(t, srv, "/api/predictions/history")
	if wNone.Code != http.StatusBadRequest {
		t.Errorf("history without predictor_id: status = %d, want 400; body = %s", wNone.Code, wNone.Body.String())
	}

	w := getPrediction(t, srv, "/api/predictions/history?predictor_id=fan1")
	if w.Code != http.StatusOK {
		t.Fatalf("history: status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Predictions []struct {
			MatchID       string  `json:"match_id"`
			PredictedBot  string  `json:"predicted_bot"`
			PredictedName string  `json:"predicted_name"`
			Correct       *bool   `json:"correct"`
			Confidence    *int    `json:"confidence"`
			MatchStatus   string  `json:"match_status"`
			WinnerName    string  `json:"winner_name"`
			ResolvedAt    *string `json:"resolved_at"`
		} `json:"predictions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(resp.Predictions) != 1 {
		t.Fatalf("history entries = %d, want 1", len(resp.Predictions))
	}
	p := resp.Predictions[0]
	if p.PredictedBot != "b_alpha" || p.PredictedName != "Alpha" {
		t.Errorf("predicted bot/name = %q/%q, want b_alpha/Alpha", p.PredictedBot, p.PredictedName)
	}
	if p.Correct == nil || !*p.Correct {
		t.Errorf("correct = %v, want true", p.Correct)
	}
	if p.Confidence == nil || *p.Confidence != 80 {
		t.Errorf("confidence = %v, want 80", p.Confidence)
	}
	if p.MatchStatus != "complete" {
		t.Errorf("match_status = %q, want complete", p.MatchStatus)
	}
	if p.WinnerName != "Alpha" {
		t.Errorf("winner_name = %q, want Alpha", p.WinnerName)
	}
	if p.ResolvedAt == nil {
		t.Error("resolved_at missing on a resolved prediction")
	}
}
