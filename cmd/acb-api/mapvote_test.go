package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMapVoteRouteRegistered(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// POST /api/vote/map should be routed (400 for empty body, not 404)
	req := httptest.NewRequest("POST", "/api/vote/map", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("POST /api/vote/map returned 404 — route not registered")
	}
}

func TestMapVoteRejectsBadMethod(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// PUT should be rejected on the map vote endpoint
	req := httptest.NewRequest("PUT", "/api/vote/map", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /api/vote/map returned %d, want 405", w.Code)
	}
}

func TestMapVoteRejectsInvalidBody(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"empty body", "", "invalid request body"},
		{"missing map_id", `{"voter_id":"v1","vote":1}`, "map_id and voter_id are required"},
		{"missing voter_id", `{"map_id":"m1","vote":1}`, "map_id and voter_id are required"},
		{"invalid vote", `{"map_id":"m1","voter_id":"v1","vote":2}`, "vote must be +1 or -1"},
		{"zero vote", `{"map_id":"m1","voter_id":"v1","vote":0}`, "vote must be +1 or -1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/vote/map", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			var body map[string]string
			json.NewDecoder(w.Body).Decode(&body)
			if body["error"] != tc.wantErr {
				t.Errorf("error = %q, want %q", body["error"], tc.wantErr)
			}
		})
	}
}

func TestMapVoteAcceptsValidBody(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Without a database, the handler will fail with a 500 or 404 (map not found),
	// but the important thing is it gets past validation.
	body := `{"map_id":"map-test123","voter_id":"voter-abc","vote":1}`
	req := httptest.NewRequest("POST", "/api/vote/map", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Without a real DB, we expect either 500 (db error) or 404 (map not found)
	// — anything except 400 (validation error)
	if w.Code == http.StatusBadRequest {
		var resp map[string]string
		json.NewDecoder(w.Body).Decode(&resp)
		t.Errorf("valid body rejected with 400: %s", resp["error"])
	}
}

func TestMapVoteDownvoteBody(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"map_id":"map-test123","voter_id":"voter-abc","vote":-1}`
	req := httptest.NewRequest("POST", "/api/vote/map", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		var resp map[string]string
		json.NewDecoder(w.Body).Decode(&resp)
		t.Errorf("valid downvote body rejected with 400: %s", resp["error"])
	}
}

func TestGetMapVotesRouteRegistered(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/vote/map/test-map-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should not be 404 (route registered), will be 500 without DB
	if w.Code == http.StatusNotFound {
		t.Fatal("GET /api/vote/map/{id} returned 404 — route not registered")
	}
}

func TestGetMapVotesRejectsBadMethod(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/api/vote/map/test-map-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/vote/map/{id} returned %d, want 405", w.Code)
	}
}
