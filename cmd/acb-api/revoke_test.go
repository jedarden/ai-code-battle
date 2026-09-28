package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRevokeKeyRoute(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/revoke-key", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("POST /api/revoke-key returned 404 — route not registered")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/revoke-key with empty body: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestRevokeKey_SuccessInvalidatesCredential(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	botID := "b_revoke001"
	secret := insertTestBot(t, db, encKey, botID, "RevokeBot1", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/revoke-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("revoke-key status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode revoke-key response: %v", err)
	}
	if response["bot_id"] != botID || response["status"] != "retired" {
		t.Errorf("revoke-key response = %v, want bot_id=%q and status=retired", response, botID)
	}
	if _, ok := response["shared_secret"]; ok {
		t.Error("revoke-key response must not deliver a replacement shared_secret")
	}

	var dbStatus, stored string
	if err := db.QueryRow(`SELECT status, shared_secret FROM bots WHERE bot_id = $1`, botID).Scan(&dbStatus, &stored); err != nil {
		t.Fatal(err)
	}
	if dbStatus != "retired" {
		t.Errorf("database status = %q, want retired", dbStatus)
	}
	if stored == secret {
		t.Error("revocation left the old credential in the database")
	}
	if decrypted, err := decryptSecret(stored, encKey); err != nil {
		t.Fatalf("decrypt revoked credential: %v", err)
	} else if decrypted == secret {
		t.Error("revocation encrypted the old credential again instead of replacing it")
	}

	// The old credential must fail immediately on the other owner-authenticated
	// API routes too, not only on a second revocation attempt.
	rotateBody, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": secret,
	})
	rotateReq := httptest.NewRequest(http.MethodPost, "/api/rotate-key", bytes.NewReader(rotateBody))
	rotateW := httptest.NewRecorder()
	mux.ServeHTTP(rotateW, rotateReq)
	if rotateW.Code != http.StatusUnauthorized {
		t.Errorf("revoked credential on rotate-key: status = %d, want 401; body = %s", rotateW.Code, rotateW.Body.String())
	}

	patchBody, _ := json.Marshal(map[string]interface{}{
		"debug_public": true,
		"api_secret":   secret,
	})
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/bot/"+botID, bytes.NewReader(patchBody))
	patchW := httptest.NewRecorder()
	mux.ServeHTTP(patchW, patchReq)
	if patchW.Code != http.StatusUnauthorized {
		t.Errorf("revoked credential on bot patch: status = %d, want 401; body = %s", patchW.Code, patchW.Body.String())
	}
}

func TestRevokeKey_InvalidSecretLeavesBotActive(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("cd", 32)
	botID := "b_revoke002"
	insertTestBot(t, db, encKey, botID, "RevokeBot2", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body, _ := json.Marshal(map[string]string{
		"bot_id":        botID,
		"shared_secret": "not-the-current-credential",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/revoke-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid credential status = %d, want 401; body = %s", w.Code, w.Body.String())
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM bots WHERE bot_id = $1`, botID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Errorf("invalid credential changed bot status to %q, want active", status)
	}
}

func TestRevokeKey_AlreadyRetired(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ef", 32)
	botID := "b_revoke003"
	secret := insertTestBot(t, db, encKey, botID, "RevokeBot3", "retired")

	srv := &Server{cfg: Config{EncryptionKey: encKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body, _ := json.Marshal(map[string]string{"bot_id": botID, "shared_secret": secret})
	req := httptest.NewRequest(http.MethodPost, "/api/revoke-key", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("already retired bot status = %d, want 409; body = %s", w.Code, w.Body.String())
	}
}

func TestRevokeKey_MissingFields(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	cases := []struct {
		name string
		body map[string]string
	}{
		{name: "missing bot_id", body: map[string]string{"shared_secret": "secret"}},
		{name: "missing secret", body: map[string]string{"bot_id": "b_revoke004"}},
		{name: "empty body", body: map[string]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/revoke-key", bytes.NewReader(body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestRevokeKey_MethodNotAllowed(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/revoke-key", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/revoke-key: status = %d, want 405", method, w.Code)
		}
	}
}
