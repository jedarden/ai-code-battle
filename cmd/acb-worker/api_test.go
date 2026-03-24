// API client tests
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetNextJob(t *testing.T) {
	tests := []struct {
		name       string
		response   APIResponse
		wantNil    bool
		wantErr    bool
	}{
		{
			name: "no pending jobs",
			response: APIResponse{
				Success: true,
				Data:    nil,
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "pending job found",
			response: APIResponse{
				Success: true,
				Data: json.RawMessage(`{
					"id": "job-123",
					"match_id": "match-456",
					"status": "pending",
					"created_at": "2024-01-01T00:00:00Z"
				}`),
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "api error",
			response: APIResponse{
				Success: false,
				Error:   "internal server error",
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/jobs/next" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != "GET" {
					t.Errorf("unexpected method: %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			cfg := &Config{
				APIEndpoint: server.URL,
				APIKey:      "test-key",
				MaxRetries:  0,
			}
			client := NewAPIClient(cfg)

			job, err := client.GetNextJob(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNextJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (job == nil) != tt.wantNil {
				t.Errorf("GetNextJob() job = %v, wantNil %v", job, tt.wantNil)
			}
		})
	}
}

func TestClaimJob(t *testing.T) {
	tests := []struct {
		name     string
		response APIResponse
		wantErr  bool
	}{
		{
			name: "successful claim",
			response: APIResponse{
				Success: true,
				Data: json.RawMessage(`{
					"job": {"id": "job-123", "match_id": "match-456", "status": "claimed", "created_at": "2024-01-01T00:00:00Z"},
					"match": {"id": "match-456", "status": "running", "map_id": "map-789", "created_at": "2024-01-01T00:00:00Z"},
					"participants": [],
					"map": {"id": "map-789", "width": 60, "height": 60, "walls": "", "spawns": "", "cores": ""},
					"bots": [],
					"bot_secrets": []
				}`),
			},
			wantErr: false,
		},
		{
			name: "job already claimed",
			response: APIResponse{
				Success: false,
				Error:   "job not found or already claimed",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("unexpected method: %s", r.Method)
				}

				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("failed to decode body: %v", err)
				}
				if body["worker_id"] != "worker-1" {
					t.Errorf("unexpected worker_id: %s", body["worker_id"])
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			cfg := &Config{
				APIEndpoint: server.URL,
				APIKey:      "test-key",
				MaxRetries:  0,
			}
			client := NewAPIClient(cfg)

			claim, err := client.ClaimJob(context.Background(), "job-123", "worker-1")
			if (err != nil) != tt.wantErr {
				t.Errorf("ClaimJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && claim == nil {
				t.Error("ClaimJob() returned nil claim without error")
			}
		})
	}
}

func TestHeartbeat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["worker_id"] != "worker-1" {
			t.Errorf("unexpected worker_id: %s", body["worker_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	cfg := &Config{
		APIEndpoint: server.URL,
		APIKey:      "test-key",
		MaxRetries:  0,
	}
	client := NewAPIClient(cfg)

	if err := client.Heartbeat(context.Background(), "job-123", "worker-1"); err != nil {
		t.Errorf("Heartbeat() error = %v", err)
	}
}

func TestSubmitResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["winner_id"] != "bot-1" {
			t.Errorf("unexpected winner_id: %v", body["winner_id"])
		}
		if body["turns"].(float64) != 100 {
			t.Errorf("unexpected turns: %v", body["turns"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	cfg := &Config{
		APIEndpoint: server.URL,
		APIKey:      "test-key",
		MaxRetries:  0,
	}
	client := NewAPIClient(cfg)

	result := &MatchResult{
		WinnerID:   "bot-1",
		Turns:      100,
		EndReason:  "elimination",
		Scores:     map[string]int{"bot-1": 5, "bot-2": 2},
	}

	if err := client.SubmitResult(context.Background(), "job-123", result, "https://r2.example.com/replay.json"); err != nil {
		t.Errorf("SubmitResult() error = %v", err)
	}
}

func TestHTTPError(t *testing.T) {
	err := &HTTPError{StatusCode: 404, Body: "not found"}
	expected := "HTTP 404: not found"
	if err.Error() != expected {
		t.Errorf("HTTPError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestAPIAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("unexpected authorization header: %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	cfg := &Config{
		APIEndpoint: server.URL,
		APIKey:      "test-api-key",
		MaxRetries:  0,
	}
	client := NewAPIClient(cfg)

	_, err := client.GetNextJob(context.Background())
	if err != nil {
		t.Errorf("GetNextJob() error = %v", err)
	}
}

func TestRetryLogic(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Simulate server error (retryable)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Success on third attempt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Success: true})
	}))
	defer server.Close()

	cfg := &Config{
		APIEndpoint: server.URL,
		APIKey:      "test-key",
		MaxRetries:  3,
	}
	client := NewAPIClient(cfg)

	_, err := client.GetNextJob(context.Background())
	if err != nil {
		t.Errorf("GetNextJob() error = %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClientErrorNoRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Client error (should not retry)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	cfg := &Config{
		APIEndpoint: server.URL,
		APIKey:      "test-key",
		MaxRetries:  3,
	}
	client := NewAPIClient(cfg)

	_, err := client.GetNextJob(context.Background())
	if err == nil {
		t.Error("expected error for bad request")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry for client errors), got %d", attempts)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Long delay
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		APIEndpoint: server.URL,
		APIKey:      "test-key",
		MaxRetries:  0,
	}
	client := NewAPIClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.GetNextJob(ctx)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}
