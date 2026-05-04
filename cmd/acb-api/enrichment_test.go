package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestEnrichmentRouteRegistered(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// POST /api/request-enrichment should be routed (400 for empty body, not 404)
	req := httptest.NewRequest("POST", "/api/request-enrichment", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("POST /api/request-enrichment returned 404 — route not registered")
	}
}

func TestRequestEnrichmentRejectsBadMethod(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// GET should be rejected on the enrichment endpoint
	req := httptest.NewRequest("GET", "/api/request-enrichment", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/request-enrichment returned %d, want 405", w.Code)
	}
}

func TestRequestEnrichmentRejectsInvalidBody(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "empty body",
			body:    "",
			wantErr: "invalid request body",
		},
		{
			name:    "missing match_id",
			body:    `{"shared_secret": "test"}`,
			wantErr: "match_id and shared_secret are required",
		},
		{
			name:    "missing shared_secret",
			body:    `{"match_id": "m_test123"}`,
			wantErr: "match_id and shared_secret are required",
		},
		{
			name:    "empty match_id",
			body:    `{"match_id": "", "shared_secret": "test"}`,
			wantErr: "match_id and shared_secret are required",
		},
		{
			name:    "empty shared_secret",
			body:    `{"match_id": "m_test123", "shared_secret": ""}`,
			wantErr: "match_id and shared_secret are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/request-enrichment", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("POST /api/request-enrichment with %s returned %d, want 400", tt.name, w.Code)
			}

			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp["error"] != tt.wantErr {
				t.Errorf("error message = %q, want %q", resp["error"], tt.wantErr)
			}
		})
	}
}

// Note: Full enrichment endpoint testing requires a database connection.
// The tests above cover routing and basic request validation.
// Integration tests with real database are skipped unless ACB_TEST_DATABASE_URL is set.
