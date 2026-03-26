package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsHealth(t *testing.T) {
	m := NewMetrics("test-worker")
	handler := m.Handler()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("expected ok status, got: %s", body)
	}
	if !strings.Contains(string(body), `"worker_id":"test-worker"`) {
		t.Fatalf("expected worker_id, got: %s", body)
	}
}

func TestMetricsReady(t *testing.T) {
	m := NewMetrics("test-worker")
	handler := m.Handler()

	// Ready by default
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when ready, got %d", w.Result().StatusCode)
	}

	// Set not ready
	m.SetReady(false)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/ready", nil))

	if w.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when not ready, got %d", w.Result().StatusCode)
	}
}

func TestMetricsCounters(t *testing.T) {
	m := NewMetrics("test-worker")

	m.RecordMatch(5 * time.Second)
	m.RecordMatch(10 * time.Second)
	m.RecordMatchError()
	m.RecordJobClaimed()
	m.RecordJobClaimed()
	m.RecordJobClaimed()
	m.RecordJobFailed()
	m.RecordPollCycle()
	m.RecordPollCycle()
	m.RecordHeartbeat()
	m.RecordHeartbeatError()
	m.RecordReplayUpload(500*time.Millisecond, 50000)
	m.RecordReplayUploadError()

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	assertMetric(t, body, "acb_matches_total", "2")
	assertMetric(t, body, "acb_match_errors_total", "1")
	assertMetric(t, body, "acb_jobs_claimed_total", "3")
	assertMetric(t, body, "acb_jobs_failed_total", "1")
	assertMetric(t, body, "acb_replays_uploaded_total", "1")
	assertMetric(t, body, "acb_replay_upload_errors_total", "1")
	assertMetric(t, body, "acb_poll_cycles_total", "2")
	assertMetric(t, body, "acb_heartbeats_sent_total", "1")
	assertMetric(t, body, "acb_heartbeat_errors_total", "1")
}

func TestMetricsHistogram(t *testing.T) {
	m := NewMetrics("test-worker")

	// Record match durations: 2s, 8s, 15s
	m.RecordMatch(2 * time.Second)
	m.RecordMatch(8 * time.Second)
	m.RecordMatch(15 * time.Second)

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Check histogram buckets: 2 <= 5, 8 <= 10, 15 <= 30
	assertContains(t, body, `acb_match_duration_seconds_bucket{le="5"} 1`)
	assertContains(t, body, `acb_match_duration_seconds_bucket{le="10"} 2`)
	assertContains(t, body, `acb_match_duration_seconds_bucket{le="30"} 3`)
	assertContains(t, body, `acb_match_duration_seconds_bucket{le="+Inf"} 3`)
	assertContains(t, body, `acb_match_duration_seconds_sum 25`)
	assertContains(t, body, `acb_match_duration_seconds_count 3`)
}

func TestMetricsReplayHistogram(t *testing.T) {
	m := NewMetrics("test-worker")

	m.RecordReplayUpload(100*time.Millisecond, 5000)
	m.RecordReplayUpload(2*time.Second, 200000)

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Upload durations: 0.1s <= 0.1, 2s <= 2
	assertContains(t, body, `acb_replay_upload_duration_seconds_bucket{le="0.1"} 1`)
	assertContains(t, body, `acb_replay_upload_duration_seconds_bucket{le="2"} 2`)
	assertContains(t, body, `acb_replay_upload_duration_seconds_count 2`)

	// Replay sizes: 5000 <= 10240, 200000 <= 1.0486e+06
	assertContains(t, body, `acb_replay_size_bytes_bucket{le="10240"} 1`)
	assertContains(t, body, `acb_replay_size_bytes_count 2`)
}

func TestMetricsContentType(t *testing.T) {
	m := NewMetrics("test-worker")
	handler := m.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	ct := w.Result().Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain content type, got: %s", ct)
	}
}

func TestMetricsWorkerInfo(t *testing.T) {
	m := NewMetrics("my-worker-42")
	handler := m.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	assertContains(t, body, `acb_worker_info{worker_id="my-worker-42"} 1`)
}

func TestCountLE(t *testing.T) {
	sorted := []float64{1, 2, 3, 5, 10, 20}

	tests := []struct {
		boundary float64
		want     int
	}{
		{0.5, 0},
		{1, 1},
		{3, 3},
		{4, 3},
		{10, 5},
		{100, 6},
	}

	for _, tt := range tests {
		got := countLE(sorted, tt.boundary)
		if got != tt.want {
			t.Errorf("countLE(%v, %g) = %d, want %d", sorted, tt.boundary, got, tt.want)
		}
	}
}

func TestFormatLabels(t *testing.T) {
	got := formatLabels([]string{"a", "1", "b", "2"})
	want := `a="1",b="2"`
	if got != want {
		t.Errorf("formatLabels = %q, want %q", got, want)
	}
}

func TestMetricsConcurrency(t *testing.T) {
	m := NewMetrics("test-worker")

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.RecordMatch(time.Duration(j) * time.Millisecond)
				m.RecordPollCycle()
				m.RecordHeartbeat()
				m.RecordReplayUpload(time.Millisecond, 1000)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if m.matchesTotal.Load() != 1000 {
		t.Fatalf("expected 1000 matches, got %d", m.matchesTotal.Load())
	}
	if m.pollCycles.Load() != 1000 {
		t.Fatalf("expected 1000 poll cycles, got %d", m.pollCycles.Load())
	}
}

// assertMetric checks a simple counter line like "metric_name 42"
func assertMetric(t *testing.T, body, metric, value string) {
	t.Helper()
	expected := metric + " " + value
	if !strings.Contains(body, expected) {
		t.Errorf("expected %q in metrics output, got:\n%s", expected, body)
	}
}

// assertContains checks that body contains substr.
func assertContains(t *testing.T, body, substr string) {
	t.Helper()
	if !strings.Contains(body, substr) {
		t.Errorf("expected %q in output, got:\n%s", substr, body)
	}
}
