package main

// Registration route tests (aicodeba-7f5b3b0e).
//
// handleRegister is the contract docs/bot-protocol.md "Register Your Bot"
// documents: 400 for a malformed body or invalid/missing name/owner/endpoint_url,
// 409 for an already-taken name, a live GET {endpoint_url}/health probe that
// must answer 200 before the bot is accepted, and a single 201 delivery of
// bot_id + shared_secret with the secret stored encrypted. The service runs
// undeployed (the Pages function answers 503 match_tier_offline for these
// routes), so these tests are the executable form of that contract until
// revival.
//
// DB-backed cases skip without ACB_TEST_DATABASE_URL, matching
// rotatekey_test.go. Each test builds its own Server, so the 5/hour
// registration rate limiter never carries state between tests.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aicodebattle/acb/ratelimit"
)

// newTestRegLimiter mirrors newTestServer's registration limiter (5/hour).
func newTestRegLimiter() *ratelimit.Limiter {
	return ratelimit.NewLimiter(5, 5.0/3600)
}

// postRegister drives POST /api/register against a fresh mux.
func postRegister(t *testing.T, srv *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest("POST", "/api/register", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// TestRegisterRoute tests that POST /api/register is registered (no DB needed).
func TestRegisterRoute(t *testing.T) {
	srv := newTestServer()
	w := postRegister(t, srv, "")

	// Empty body decodes to an error before any DB access — the point is
	// that the route answers, not 404.
	if w.Code == http.StatusNotFound {
		t.Fatal("POST /api/register returned 404 — route not registered")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/register with empty body: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestRegister_MissingFields(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, regLimiter: newTestRegLimiter(), db: db}

	for _, body := range []string{
		`{"owner":"o","endpoint_url":"http://x/health"}`,
		`{"name":"n","endpoint_url":"http://x/health"}`,
		`{"name":"n","owner":"o"}`,
		`{}`,
	} {
		w := postRegister(t, srv, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("register %s: status = %d, want 400; body = %s", body, w.Code, w.Body.String())
			continue
		}
		var resp map[string]string
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Errorf("register %s: response not error JSON: %v", body, err)
		}
	}
}

func TestRegister_InvalidBotName(t *testing.T) {
	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, regLimiter: newTestRegLimiter()}
	for _, name := range []string{"ab", "bot_name", "bot name", strings.Repeat("a", 33)} {
		w := postRegister(t, srv, `{"name":"`+name+`","owner":"o","endpoint_url":"http://127.0.0.1:1"}`)
		if w.Code != http.StatusBadRequest {
			t.Errorf("name %q: status = %d, want 400; body = %s", name, w.Code, w.Body.String())
		}
	}
}

func TestRegister_InvalidBody(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, regLimiter: newTestRegLimiter(), db: db}
	w := postRegister(t, srv, `{"name": not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("register malformed JSON: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestRegister_DuplicateName(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("ab", 32)
	insertTestBot(t, db, encKey, "b_dup00001", "DupBot", "active")

	srv := &Server{cfg: Config{EncryptionKey: encKey, BotTimeoutSecs: 5}, regLimiter: newTestRegLimiter(), db: db}
	body, _ := json.Marshal(map[string]string{
		"name":         "DupBot",
		"owner":        "someone-else",
		"endpoint_url": "http://127.0.0.1:1",
	})
	w := postRegister(t, srv, string(body))

	// The name conflict is rejected before the endpoint probe fires, so an
	// unreachable endpoint_url must not change the answer.
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate name: status = %d, want 409; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "already taken") {
		t.Errorf("duplicate name body = %s, want it to name the taken name", w.Body.String())
	}
}

func TestRegister_HealthProbeNon200(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	var probes int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&probes, 1)
		if r.URL.Path != "/health" {
			t.Errorf("probe path = %q, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer backend.Close()

	srv := &Server{cfg: Config{BotTimeoutSecs: 5}, regLimiter: newTestRegLimiter(), db: db}
	body, _ := json.Marshal(map[string]string{
		"name":         "ProbeBot",
		"owner":        "o",
		"endpoint_url": backend.URL,
	})
	w := postRegister(t, srv, string(body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unhealthy endpoint: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "bot endpoint validation failed") {
		t.Errorf("unhealthy endpoint body = %s, want the probe failure message", w.Body.String())
	}
	if atomic.LoadInt32(&probes) != 1 {
		t.Errorf("health probe count = %d, want 1", probes)
	}

	// A rejected bot must not be inserted.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bots WHERE name = 'ProbeBot'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("rejected registration inserted %d bot row(s), want 0", count)
	}
}

func TestRegister_HealthProbeUnreachable(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	srv := &Server{cfg: Config{BotTimeoutSecs: 1}, regLimiter: newTestRegLimiter(), db: db}
	body, _ := json.Marshal(map[string]string{
		"name":         "DarkBot",
		"owner":        "o",
		"endpoint_url": "http://127.0.0.1:1", // nothing listens on port 1
	})
	w := postRegister(t, srv, string(body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unreachable endpoint: status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestRegister_Success(t *testing.T) {
	db := openTestDBAPI(t)
	defer db.Close()
	setupRotateKeySchema(t, db)

	encKey := strings.Repeat("cd", 32)
	var probes int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&probes, 1)
		if r.URL.Path != "/health" {
			t.Errorf("probe path = %q, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	srv := &Server{cfg: Config{EncryptionKey: encKey, BotTimeoutSecs: 5}, regLimiter: newTestRegLimiter(), db: db}
	body, _ := json.Marshal(map[string]string{
		"name":         "FreshBot",
		"owner":        "o",
		"endpoint_url": backend.URL,
	})
	w := postRegister(t, srv, string(body))

	if w.Code != http.StatusCreated {
		t.Fatalf("register: status = %d, want 201; body = %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	botID := resp["bot_id"]
	secret := resp["shared_secret"]
	if !regexp.MustCompile(`^b_[0-9a-f]{12}$`).MatchString(botID) {
		t.Errorf("bot_id = %q, want b_ + 12 lowercase hex chars", botID)
	}
	if len(secret) != 64 {
		t.Errorf("shared_secret length = %d, want 64", len(secret))
	}
	if atomic.LoadInt32(&probes) != 1 {
		t.Errorf("health probe count = %d, want 1", probes)
	}

	// The stored secret is the ciphertext of what was delivered, not the
	// plaintext — the response is the only delivery.
	var stored string
	if err := db.QueryRow(`SELECT shared_secret FROM bots WHERE bot_id = $1`, botID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == secret {
		t.Error("stored secret is plaintext; want the encrypted form")
	}
	roundTrip, err := decryptSecret(stored, encKey)
	if err != nil {
		t.Fatalf("decrypt stored secret: %v", err)
	}
	if roundTrip != secret {
		t.Error("decrypted stored secret does not match the delivered secret")
	}

	// The insert is what makes the name taken — a second registration with
	// the same name conflicts.
	w2 := postRegister(t, srv, string(body))
	if w2.Code != http.StatusConflict {
		t.Fatalf("second registration with same name: status = %d, want 409; body = %s", w2.Code, w2.Body.String())
	}
}
