package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer creates a Server with no database or redis (for unit tests
// that don't need them).
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

// TestFeedbackEndpointPath asserts that the community feedback endpoint is
// served at /api/feedback per plan §13.6 — not /api/ui-feedback.
func TestFeedbackEndpointPath(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// POST /api/feedback should be routed (200 from handler, not 404)
	req := httptest.NewRequest("POST", "/api/feedback", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Handler returns 400 for empty body, not 404 — proves the route is registered
	if w.Code == http.StatusNotFound {
		t.Fatal("POST /api/feedback returned 404 — route not registered (expected per plan §13.6)")
	}

	// POST /api/ui-feedback (old name) must NOT be routed
	reqOld := httptest.NewRequest("POST", "/api/ui-feedback", nil)
	wOld := httptest.NewRecorder()
	mux.ServeHTTP(wOld, reqOld)

	if wOld.Code != http.StatusNotFound {
		t.Errorf("POST /api/ui-feedback returned %d, want 404 — old route name should not be registered", wOld.Code)
	}
}
