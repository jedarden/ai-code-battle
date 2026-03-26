// Metrics collection and HTTP server for monitoring.
//
// Exposes Prometheus text format metrics at /metrics, plus
// /health and /ready endpoints for K8s probes.
package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects operational metrics for the match worker.
type Metrics struct {
	// Counters
	matchesTotal     atomic.Int64
	matchErrorsTotal atomic.Int64
	jobsClaimedTotal atomic.Int64
	jobsFailedTotal  atomic.Int64
	replaysUploaded  atomic.Int64
	replayUploadErrs atomic.Int64
	pollCycles       atomic.Int64
	heartbeatsSent   atomic.Int64
	heartbeatErrors  atomic.Int64

	// Histograms (stored as individual observations)
	mu                   sync.Mutex
	matchDurations       []float64 // seconds
	replayUploadDurations []float64 // seconds
	replaySizes          []float64 // bytes

	// State
	startTime time.Time
	workerID  string
	ready     atomic.Bool
}

// NewMetrics creates a new Metrics instance.
func NewMetrics(workerID string) *Metrics {
	m := &Metrics{
		startTime:             time.Now(),
		workerID:              workerID,
		matchDurations:        make([]float64, 0, 1024),
		replayUploadDurations: make([]float64, 0, 1024),
		replaySizes:           make([]float64, 0, 1024),
	}
	m.ready.Store(true)
	return m
}

// RecordMatch records a completed match.
func (m *Metrics) RecordMatch(duration time.Duration) {
	m.matchesTotal.Add(1)
	m.mu.Lock()
	m.matchDurations = append(m.matchDurations, duration.Seconds())
	m.mu.Unlock()
}

// RecordMatchError records a match execution error.
func (m *Metrics) RecordMatchError() {
	m.matchErrorsTotal.Add(1)
}

// RecordJobClaimed records a job claim.
func (m *Metrics) RecordJobClaimed() {
	m.jobsClaimedTotal.Add(1)
}

// RecordJobFailed records a job failure report.
func (m *Metrics) RecordJobFailed() {
	m.jobsFailedTotal.Add(1)
}

// RecordReplayUpload records a successful replay upload.
func (m *Metrics) RecordReplayUpload(duration time.Duration, sizeBytes int) {
	m.replaysUploaded.Add(1)
	m.mu.Lock()
	m.replayUploadDurations = append(m.replayUploadDurations, duration.Seconds())
	m.replaySizes = append(m.replaySizes, float64(sizeBytes))
	m.mu.Unlock()
}

// RecordReplayUploadError records a replay upload error.
func (m *Metrics) RecordReplayUploadError() {
	m.replayUploadErrs.Add(1)
}

// RecordPollCycle records a poll cycle.
func (m *Metrics) RecordPollCycle() {
	m.pollCycles.Add(1)
}

// RecordHeartbeat records a heartbeat sent.
func (m *Metrics) RecordHeartbeat() {
	m.heartbeatsSent.Add(1)
}

// RecordHeartbeatError records a heartbeat error.
func (m *Metrics) RecordHeartbeatError() {
	m.heartbeatErrors.Add(1)
}

// SetReady sets the worker readiness state.
func (m *Metrics) SetReady(ready bool) {
	m.ready.Store(ready)
}

// Handler returns an http.Handler serving metrics and health endpoints.
func (m *Metrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", m.handleMetrics)
	mux.HandleFunc("/health", m.handleHealth)
	mux.HandleFunc("/ready", m.handleReady)
	return mux
}

func (m *Metrics) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","worker_id":%q,"uptime_seconds":%.0f}`,
		m.workerID, time.Since(m.startTime).Seconds())
}

func (m *Metrics) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if m.ready.Load() {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ready","worker_id":%q}`, m.workerID)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_ready","worker_id":%q}`, m.workerID)
	}
}

func (m *Metrics) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var b strings.Builder

	// Worker info
	writeGauge(&b, "acb_worker_info", "Worker metadata", 1,
		"worker_id", m.workerID)
	writeGauge(&b, "acb_worker_uptime_seconds", "Time since worker started",
		time.Since(m.startTime).Seconds())

	// Counters
	writeCounter(&b, "acb_matches_total", "Total matches executed",
		float64(m.matchesTotal.Load()))
	writeCounter(&b, "acb_match_errors_total", "Total match execution errors",
		float64(m.matchErrorsTotal.Load()))
	writeCounter(&b, "acb_jobs_claimed_total", "Total jobs claimed",
		float64(m.jobsClaimedTotal.Load()))
	writeCounter(&b, "acb_jobs_failed_total", "Total jobs reported as failed",
		float64(m.jobsFailedTotal.Load()))
	writeCounter(&b, "acb_replays_uploaded_total", "Total replays uploaded to R2",
		float64(m.replaysUploaded.Load()))
	writeCounter(&b, "acb_replay_upload_errors_total", "Total replay upload errors",
		float64(m.replayUploadErrs.Load()))
	writeCounter(&b, "acb_poll_cycles_total", "Total job poll cycles",
		float64(m.pollCycles.Load()))
	writeCounter(&b, "acb_heartbeats_sent_total", "Total heartbeats sent",
		float64(m.heartbeatsSent.Load()))
	writeCounter(&b, "acb_heartbeat_errors_total", "Total heartbeat errors",
		float64(m.heartbeatErrors.Load()))

	// Histograms (snapshot under lock)
	m.mu.Lock()
	matchDurs := make([]float64, len(m.matchDurations))
	copy(matchDurs, m.matchDurations)
	uploadDurs := make([]float64, len(m.replayUploadDurations))
	copy(uploadDurs, m.replayUploadDurations)
	replaySizes := make([]float64, len(m.replaySizes))
	copy(replaySizes, m.replaySizes)
	m.mu.Unlock()

	matchBuckets := []float64{1, 5, 10, 30, 60, 120, 300, 600}
	writeHistogram(&b, "acb_match_duration_seconds",
		"Match execution duration in seconds", matchDurs, matchBuckets)

	uploadBuckets := []float64{0.1, 0.5, 1, 2, 5, 10, 30}
	writeHistogram(&b, "acb_replay_upload_duration_seconds",
		"Replay upload duration in seconds", uploadDurs, uploadBuckets)

	sizeBuckets := []float64{1024, 10240, 102400, 1048576, 10485760}
	writeHistogram(&b, "acb_replay_size_bytes",
		"Replay file size in bytes", replaySizes, sizeBuckets)

	w.Write([]byte(b.String()))
}

// writeCounter writes a counter metric in Prometheus text format.
func writeCounter(b *strings.Builder, name, help string, value float64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", name)
	fmt.Fprintf(b, "%s %g\n\n", name, value)
}

// writeGauge writes a gauge metric in Prometheus text format.
func writeGauge(b *strings.Builder, name, help string, value float64, labels ...string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s gauge\n", name)
	if len(labels) > 0 {
		labelStr := formatLabels(labels)
		fmt.Fprintf(b, "%s{%s} %g\n\n", name, labelStr, value)
	} else {
		fmt.Fprintf(b, "%s %g\n\n", name, value)
	}
}

// writeHistogram writes a histogram metric in Prometheus text format.
func writeHistogram(b *strings.Builder, name, help string, observations []float64, buckets []float64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s histogram\n", name)

	sort.Float64s(buckets)

	var sum float64
	for _, v := range observations {
		sum += v
	}

	sorted := make([]float64, len(observations))
	copy(sorted, observations)
	sort.Float64s(sorted)

	count := len(sorted)
	for _, boundary := range buckets {
		n := countLE(sorted, boundary)
		fmt.Fprintf(b, "%s_bucket{le=\"%g\"} %d\n", name, boundary, n)
	}
	fmt.Fprintf(b, "%s_bucket{le=\"+Inf\"} %d\n", name, count)
	fmt.Fprintf(b, "%s_sum %g\n", name, sum)
	fmt.Fprintf(b, "%s_count %d\n\n", name, count)
}

// countLE counts values <= boundary in a sorted slice.
func countLE(sorted []float64, boundary float64) int {
	i := sort.Search(len(sorted), func(i int) bool {
		return sorted[i] > boundary
	})
	return i
}

// formatLabels formats label key-value pairs for Prometheus output.
func formatLabels(labels []string) string {
	var parts []string
	for i := 0; i+1 < len(labels); i += 2 {
		parts = append(parts, fmt.Sprintf(`%s=%q`, labels[i], labels[i+1]))
	}
	return strings.Join(parts, ",")
}
