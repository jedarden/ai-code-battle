package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// openTestDB opens a test database for rotate-key integration tests.
func openTestDBAPI(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("ACB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ACB_TEST_DATABASE_URL not set, skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.SetMaxOpenConns(2)
	return db
}

// setupRotateKeySchema creates the bots table needed for rotate-key tests.
func setupRotateKeySchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS bots (
			bot_id        VARCHAR(16) PRIMARY KEY,
			name          VARCHAR(32) UNIQUE NOT NULL,
			owner         VARCHAR(128) NOT NULL DEFAULT 'test',
			endpoint_url  TEXT NOT NULL DEFAULT 'http://localhost:8080',
			shared_secret TEXT NOT NULL DEFAULT 'secret',
			status        VARCHAR(16) NOT NULL DEFAULT 'active',
			rating_mu     DOUBLE PRECISION NOT NULL DEFAULT 1500.0,
			rating_phi    DOUBLE PRECISION NOT NULL DEFAULT 350.0,
			rating_sigma  DOUBLE PRECISION NOT NULL DEFAULT 0.06,
			evolved       BOOLEAN NOT NULL DEFAULT FALSE,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_active   TIMESTAMPTZ,
			debug_public  BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	if err != nil {
		t.Fatalf("create bots table: %v", err)
	}
}

// insertTestBot inserts a bot with a known encrypted secret and returns the plaintext secret.
func insertTestBot(t *testing.T, db *sql.DB, encKey, botID, name, status string) string {
	t.Helper()
	secret, err := generateSecret()
	if err != nil {
		t.Fatal(err)
	}

	var encrypted string
	if encKey != "" {
		encrypted, err = encryptSecret(secret, encKey)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		encrypted = secret
	}

	_, err = db.Exec(`INSERT INTO bots (bot_id, name, shared_secret, status) VALUES ($1, $2, $3, $4)
		ON CONFLICT (bot_id) DO UPDATE SET shared_secret = $3, status = $4`,
		botID, name, encrypted, status)
	if err != nil {
		t.Fatalf("insert test bot: %v", err)
	}
	return secret
}

// TestRotateKeyRoute tests that POST /api/rotate-key is registered (no DB needed).
func TestRotateKeyRoute(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/api/rotate-key", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should not be 404 — proves the route is registered
	if w.Code == http.StatusNotFound {
		t.Fatal("POST /api/rotate-key returned 404 — route not registered")
	}
}

// TestRotateKey_Success rotates a bot's secret and verifies the new one works.
func TestRotateKey_Success(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	botID := "b_test001"
	secret := insertTestBot(t, db, encKey, botID, "TestBot1", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Rotate the key
	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("rotate-key status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	newSecret, ok := resp["shared_secret"].(string)
	if !ok || newSecret == "" {
		t.Fatal("response missing shared_secret")
	}
	if newSecret == secret {
		t.Error("new secret should differ from old secret")
	}
	if newSecret == "" || len(newSecret) != 64 {
		t.Errorf("new secret length = %d, want 64", len(newSecret))
	}
}

// TestRotateKey_OldSecretRejected verifies the old secret is rejected after rotation.
func TestRotateKey_OldSecretRejected(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	botID := "b_test002"
	secret := insertTestBot(t, db, encKey, botID, "TestBot2", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Rotate the key
	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first rotation: status = %d, want 200", w.Code)
	}

	// Try to rotate again with the OLD secret — should fail
	body2, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	req2 := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("old secret should be rejected: status = %d, want 401; body = %s", w2.Code, w2.Body.String())
	}
}

// TestRotateKey_Retire sets the bot to retired and verifies status.
func TestRotateKey_Retire(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	botID := "b_test003"
	secret := insertTestBot(t, db, encKey, botID, "TestBot3", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Rotate and retire
	body, _ := json.Marshal(map[string]interface{}{
		"bot_id":        botID,
		"shared_secret": secret,
		"retire":        true,
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("retire status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "retired" {
		t.Errorf("response status = %v, want 'retired'", resp["status"])
	}

	// Verify DB status
	var dbStatus string
	err := db.QueryRow(`SELECT status FROM bots WHERE bot_id = $1`, botID).Scan(&dbStatus)
	if err != nil {
		t.Fatal(err)
	}
	if dbStatus != "retired" {
		t.Errorf("db status = %q, want 'retired'", dbStatus)
	}
}

// TestRotateKey_RetiredBotCannotRotate verifies a retired bot is rejected.
func TestRotateKey_RetiredBotCannotRotate(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	botID := "b_test004"
	secret := insertTestBot(t, db, encKey, botID, "TestBot4", "retired")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("retired bot rotate should return 409: status = %d, body = %s", w.Code, w.Body.String())
	}
}

// TestRotateKey_InvalidSecret verifies wrong secret is rejected.
func TestRotateKey_InvalidSecret(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	botID := "b_test005"
	insertTestBot(t, db, encKey, botID, "TestBot5", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": "completely-wrong-secret",
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong secret should return 401: status = %d, body = %s", w.Code, w.Body.String())
	}
}

// TestRotateKey_MissingFields verifies required fields are enforced.
func TestRotateKey_MissingFields(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	cases := []struct {
		name string
		body map[string]string
	}{
		{"no bot_id", map[string]string{"shared_secret": "abc"}},
		{"no secret", map[string]string{"bot_id": "b_test"}},
		{"empty body", map[string]string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}

// TestRotateKey_NewSecretWorksForSecondRotation verifies that the new secret
// returned by a rotation can be used for a subsequent rotation.
func TestRotateKey_NewSecretWorksForSecondRotation(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	botID := "b_test006"
	secret := insertTestBot(t, db, encKey, botID, "TestBot6", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// First rotation
	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first rotation: status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	newSecret := resp["shared_secret"].(string)

	// Second rotation using the new secret
	body2, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": newSecret,
	})
	req2 := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("second rotation with new secret: status = %d, want 200; body = %s", w2.Code, w2.Body.String())
	}
}

// TestRotateKey_RetiredBotExcludedFromMatchmaking verifies that a retired bot
// does not appear in the matchmaker's active bot query.
func TestRotateKey_RetiredBotExcludedFromMatchmaking(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)

	// Insert one active and one retired bot
	insertTestBot(t, db, encKey, "b_active1", "ActiveBot", "active")
	insertTestBot(t, db, encKey, "b_retired1", "RetiredBot", "retired")

	// Run the matchmaker eligibility query (from cmd/acb-matchmaker/tickers.go)
	rows, err := db.Query(`SELECT bot_id FROM bots WHERE status = 'active'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var activeIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		activeIDs = append(activeIDs, id)
	}

	for _, id := range activeIDs {
		if id == "b_retired1" {
			t.Error("retired bot should not appear in matchmaker eligibility query")
		}
	}

	found := false
	for _, id := range activeIDs {
		if id == "b_active1" {
			found = true
		}
	}
	if !found {
		t.Error("active bot should appear in matchmaker eligibility query")
	}
}

// TestRotateKey_PlaintextFallback verifies rotation works without encryption key.
func TestRotateKey_PlaintextFallback(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	// No encryption key — secrets stored as plaintext
	botID := "b_test007"
	secret := insertTestBot(t, db, "", botID, "TestBot7", "active")

	srv := &Server{cfg: Config{}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("plaintext rotation: status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

// TestRotateKey_ BotNotFound verifies 404 for nonexistent bot.
func TestRotateKey_BotNotFound(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	srv := &Server{cfg: Config{}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{
		"bot_id":        "b_nonexistent",
		"shared_secret": "whatever",
	})
	req := httptest.NewRequest("POST", "/api/rotate-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("nonexistent bot should return 404: status = %d", w.Code)
	}
}

// TestRotateKey_MethodNotAllowed verifies only POST is accepted.
func TestRotateKey_MethodNotAllowed(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	for _, method := range []string{"GET", "PUT", "PATCH", "DELETE"} {
		req := httptest.NewRequest(method, "/api/rotate-key", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/rotate-key: status = %d, want 405", method, w.Code)
		}
	}
}
