package main

// These tests exercise the credential contract across the registration and
// management handlers instead of testing encryption and handlers in
// isolation. They use the same opt-in PostgreSQL fixture as the existing API
// integration tests (ACB_TEST_DATABASE_URL).

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func credentialLifecycleName(prefix string) string {
	return prefix + strconv.FormatInt(time.Now().UnixNano()%1000000000, 10)
}

func credentialLifecycleEncryptionKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate test encryption key: %v", err)
	}
	return hex.EncodeToString(key)
}

func credentialLifecycleEndpoint(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func decodeCredentialResponse(t *testing.T, response *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var body struct {
		BotID        string `json:"bot_id"`
		SharedSecret string `json:"shared_secret"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode credential response: %v", err)
	}
	return body.BotID, body.SharedSecret
}

func TestCredentialLifecycle_RegisterDeliveryIsOneTimeAndNeverLogged(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encryptionKey := credentialLifecycleEncryptionKey(t)
	backend := credentialLifecycleEndpoint(t)
	defer backend.Close()
	name := credentialLifecycleName("OnceBot")

	srv := &Server{
		cfg:        Config{EncryptionKey: encryptionKey, BotTimeoutSecs: 5},
		regLimiter: newTestRegLimiter(),
		db:         db,
	}

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	body, err := json.Marshal(map[string]string{
		"name":         name,
		"owner":        "lifecycle-test",
		"endpoint_url": backend.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := postRegister(t, srv, string(body))
	if response.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, want 201", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("registration Cache-Control = %q, want no-store", got)
	}
	botID, secret := decodeCredentialResponse(t, response)
	if botID == "" || len(secret) != 64 {
		t.Fatalf("registration returned malformed credential metadata: bot_id present=%t, secret length=%d", botID != "", len(secret))
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM bots WHERE bot_id = $1`, botID) })

	var stored string
	if err := db.QueryRow(`SELECT shared_secret FROM bots WHERE bot_id = $1`, botID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == secret {
		t.Fatal("registration persisted the delivered secret in plaintext")
	}
	if decrypted, err := decryptSecret(stored, encryptionKey); err != nil {
		t.Fatalf("decrypt stored registration secret: %v", err)
	} else if decrypted != secret {
		t.Fatal("encrypted registration secret does not round-trip to the delivered value")
	}

	// Public profile/status reads are the available post-registration reads;
	// neither is allowed to become a secret read-back endpoint.
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	for _, path := range []string{"/api/status/" + botID, "/api/bot/" + botID} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		readback := httptest.NewRecorder()
		mux.ServeHTTP(readback, request)
		if strings.Contains(readback.Body.String(), secret) || strings.Contains(readback.Body.String(), stored) {
			t.Errorf("%s returned credential material", path)
		}
	}

	if count := strings.Count(response.Body.String(), secret); count != 1 {
		t.Errorf("registration response contains the secret %d times, want exactly once", count)
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatal("registration log output contains the delivered secret")
	}
}

func TestCredentialLifecycle_RegisterWithoutEncryptionKeyUsesDevelopmentFallback(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	backend := credentialLifecycleEndpoint(t)
	defer backend.Close()
	name := credentialLifecycleName("FallbackBot")
	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, regLimiter: newTestRegLimiter(), db: db}
	body := fmt.Sprintf(`{"name":%q,"owner":"lifecycle-test","endpoint_url":%q}`, name, backend.URL)
	response := postRegister(t, srv, body)
	if response.Code != http.StatusCreated {
		t.Fatalf("registration without encryption key status = %d, want 201", response.Code)
	}
	botID, secret := decodeCredentialResponse(t, response)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM bots WHERE bot_id = $1`, botID) })

	var stored string
	if err := db.QueryRow(`SELECT shared_secret FROM bots WHERE bot_id = $1`, botID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != secret {
		t.Fatalf("missing-key development fallback stored a different value (length %d)", len(stored))
	}
}

func TestCredentialLifecycle_InvalidEncryptionKeyDoesNotPersistCredential(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	backend := credentialLifecycleEndpoint(t)
	defer backend.Close()
	name := credentialLifecycleName("BadKeyBot")
	srv := &Server{
		cfg:        Config{EncryptionKey: "not-a-32-byte-key", BotTimeoutSecs: 5},
		regLimiter: newTestRegLimiter(),
		db:         db,
	}
	body := fmt.Sprintf(`{"name":%q,"owner":"lifecycle-test","endpoint_url":%q}`, name, backend.URL)
	response := postRegister(t, srv, body)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("invalid encryption key status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "shared_secret") {
		t.Fatal("failed registration returned a shared_secret")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bots WHERE name = $1`, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid encryption key persisted %d bot row(s), want 0", count)
	}
}

func TestCredentialLifecycle_RevocationResponseAndLogsContainNoSecret(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encryptionKey := credentialLifecycleEncryptionKey(t)
	botID := "b_lifecycle_log"
	secret := insertTestBot(t, db, encryptionKey, botID, "LifecycleLogBot", "active")
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM bots WHERE bot_id = $1`, botID) })

	srv := &Server{cfg: Config{EncryptionKey: encryptionKey}, db: db}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body, err := json.Marshal(map[string]string{"bot_id": botID, "shared_secret": secret})
	if err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	request := httptest.NewRequest(http.MethodPost, "/api/revoke-key", bytes.NewReader(body))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("revocation status = %d, want 200", response.Code)
	}
	if strings.Contains(response.Body.String(), "shared_secret") || strings.Contains(response.Body.String(), secret) {
		t.Fatal("revocation response disclosed credential material")
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatal("revocation log output contains the revoked secret")
	}

	var stored string
	if err := db.QueryRow(`SELECT shared_secret FROM bots WHERE bot_id = $1`, botID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == secret {
		t.Fatal("revocation left the old credential stored")
	}
	if replacement, err := decryptSecret(stored, encryptionKey); err != nil {
		t.Fatalf("decrypt replacement credential: %v", err)
	} else if strings.Contains(logs.String(), replacement) {
		t.Fatal("revocation log output contains the undisclosed replacement credential")
	}
}
