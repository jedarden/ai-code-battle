package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer creates a Server with no database or redis (for unit tests
// that don't need them). For handler tests that need DB, use the integration
// tests pattern with a test database.
func newTestServer() *Server {
	return &Server{
		cfg: Config{
			WorkerAPIKey:   "test-key",
			BotTimeoutSecs: 5,
			MaxConsecFails: 3,
		},
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health status = %d, want 200", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("health body = %v, want status=ok", body)
	}
}

func TestAuthenticateWorker(t *testing.T) {
	srv := newTestServer()

	tests := []struct {
		name   string
		header string
		value  string
		want   bool
	}{
		{"bearer", "Authorization", "Bearer test-key", true},
		{"x-api-key", "X-API-Key", "test-key", true},
		{"wrong key", "Authorization", "Bearer wrong", false},
		{"no header", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}
			got := srv.authenticateWorker(req)
			if got != tt.want {
				t.Errorf("authenticateWorker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthenticateWorker_NoKeyConfigured(t *testing.T) {
	srv := &Server{cfg: Config{WorkerAPIKey: ""}}
	req := httptest.NewRequest("GET", "/", nil)
	if !srv.authenticateWorker(req) {
		t.Error("with no key configured, all requests should be authenticated")
	}
}

func TestRegisterValidation(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	tests := []struct {
		name     string
		body     RegisterRequest
		wantCode int
	}{
		{
			name:     "missing name",
			body:     RegisterRequest{Name: "", EndpointURL: "http://example.com", Owner: "alice"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "name too short",
			body:     RegisterRequest{Name: "ab", EndpointURL: "http://example.com", Owner: "alice"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "name with spaces",
			body:     RegisterRequest{Name: "my bot", EndpointURL: "http://example.com", Owner: "alice"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "missing endpoint",
			body:     RegisterRequest{Name: "valid-bot", EndpointURL: "", Owner: "alice"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "missing owner",
			body:     RegisterRequest{Name: "valid-bot", EndpointURL: "http://example.com", Owner: ""},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/register", bytes.NewReader(body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d; body: %s", w.Code, tt.wantCode, w.Body.String())
			}
		})
	}
}

func TestValidBotName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"simple", "mybot", true},
		{"with-hyphen", "my-bot", true},
		{"with-numbers", "bot123", true},
		{"mixed", "My-Bot-42", true},
		{"three-chars", "abc", true},
		{"too-short", "ab", false},
		{"starts-with-hyphen", "-bot", false},
		{"ends-with-hyphen", "bot-", false},
		{"spaces", "my bot", false},
		{"special", "bot@123", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validBotName.MatchString(tt.input)
			if got != tt.valid {
				t.Errorf("validBotName(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["key"] != "value" {
		t.Errorf("body = %v, want key=value", body)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "test error" {
		t.Errorf("body = %v, want error=test error", body)
	}
}

func TestJobClaimRequiresAuth(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(JobClaimRequest{WorkerID: "w1"})
	req := httptest.NewRequest("POST", "/api/jobs/claim", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("job claim without auth: status = %d, want 401", w.Code)
	}
}

func TestJobResultRequiresAuth(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(JobResultRequest{WorkerID: "w1", Condition: "score", TurnCount: 100})
	req := httptest.NewRequest("POST", "/api/jobs/j_12345678/result", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("job result without auth: status = %d, want 401", w.Code)
	}
}
