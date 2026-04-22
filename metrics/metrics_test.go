package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerReturnsOK(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	// Increment some metrics to ensure they appear
	MatchThroughput.Inc()
	JobQueueDepth.Set(42)
	HTTPRequestsTotal.WithLabelValues("GET", "/test", "200").Inc()

	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	// Verify §9.9 metrics are present
	expectedMetrics := []string{
		"acb_match_throughput_total",
		"acb_job_queue_depth",
		"acb_bot_crashed_total",
		"acb_job_stale_count",
		"acb_r2_bytes_used",
		"acb_replay_upload_latency_seconds",
		"acb_evolver_generations_total",
		"acb_index_build_duration_seconds",
		"acb_http_requests_total",
	}
	for _, name := range expectedMetrics {
		if !strings.Contains(body, name) {
			t.Errorf("metrics output missing %q", name)
		}
	}

	// Verify the incremented counter is present
	if !strings.Contains(body, "acb_match_throughput_total ") {
		t.Error("match throughput counter not found in output")
	}

	// Verify the gauge value
	if !strings.Contains(body, "acb_job_queue_depth 42") {
		t.Error("job queue depth gauge not found with expected value")
	}
}

func TestHTTPRequestsCounter(t *testing.T) {
	HTTPRequestsTotal.WithLabelValues("GET", "/api/status", "200").Inc()
	HTTPRequestsTotal.WithLabelValues("POST", "/api/register", "201").Inc()

	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `method="GET",path="/api/status",status="200"`) {
		t.Error("labelled HTTP request counter not found")
	}
	if !strings.Contains(body, `method="POST",path="/api/register",status="201"`) {
		t.Error("labelled HTTP request counter not found")
	}
}

func TestHistogramObserved(t *testing.T) {
	ReplayUploadLatency.Observe(1.5)
	IndexBuildDuration.Observe(30.2)

	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "acb_replay_upload_latency_seconds_bucket") {
		t.Error("replay upload latency histogram not found")
	}
	if !strings.Contains(body, "acb_index_build_duration_seconds_bucket") {
		t.Error("index build duration histogram not found")
	}
}
