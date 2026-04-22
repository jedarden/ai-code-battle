// Package metrics defines Prometheus metrics for AI Code Battle services per plan §9.9.
//
// All services import this package to expose a /metrics endpoint on an
// internal port (default :9090). The metrics match the 9 monitoring signals
// listed in the plan.
package metrics

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// §9.9 metric definitions — registered once at init time.
var (
	// MatchThroughput counts completed matches (worker increments per result).
	MatchThroughput = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "acb_match_throughput_total",
		Help: "Total number of matches completed.",
	})

	// JobQueueDepth tracks the Valkey job queue length (matchmaker updates each tick).
	JobQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "acb_job_queue_depth",
		Help: "Current number of pending jobs in the Valkey queue.",
	})

	// BotCrashed counts bots marked as crashed by the health checker.
	BotCrashed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "acb_bot_crashed_total",
		Help: "Total number of bot crash events detected by the health checker.",
	})

	// StaleJobCount is the number of stale jobs found in the last reaper cycle.
	StaleJobCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "acb_job_stale_count",
		Help: "Number of stale jobs found in the most recent reaper cycle.",
	})

	// R2BytesUsed tracks the R2 warm cache size in bytes (index-builder updates).
	R2BytesUsed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "acb_r2_bytes_used",
		Help: "Total bytes used in the R2 warm cache.",
	})

	// ReplayUploadLatency tracks B2 replay upload duration.
	ReplayUploadLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "acb_replay_upload_latency_seconds",
		Help:    "Latency of replay uploads to B2 in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	// EvolverGenerations counts evolution cycles completed.
	EvolverGenerations = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "acb_evolver_generations_total",
		Help: "Total number of evolution generations completed.",
	})

	// IndexBuildDuration tracks how long each index build cycle takes.
	IndexBuildDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "acb_index_build_duration_seconds",
		Help:    "Duration of index build cycles in seconds.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
	})

	// HTTPRequestsTotal counts HTTP requests served by the API.
	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "acb_http_requests_total",
		Help: "Total number of HTTP requests served.",
	}, []string{"method", "path", "status"})

	// BotsActive tracks the number of currently active bots (matchmaker health checker).
	BotsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "acb_bots_active",
		Help: "Number of bots currently in active status.",
	})

	// BotsFailing tracks the number of bots failing health checks.
	BotsFailing = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "acb_bots_failing",
		Help: "Number of bots currently failing health checks.",
	})

	// WorkerMatchesTotal counts matches executed by the worker.
	WorkerMatchesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "acb_worker_matches_total",
		Help: "Total matches executed by this worker.",
	})

	// WorkerMatchErrorsTotal counts match execution errors.
	WorkerMatchErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "acb_worker_match_errors_total",
		Help: "Total match execution errors.",
	})

	// WorkerJobsClaimedTotal counts jobs claimed by the worker.
	WorkerJobsClaimedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "acb_worker_jobs_claimed_total",
		Help: "Total jobs claimed by this worker.",
	})

	// WorkerMatchDuration tracks match execution time.
	WorkerMatchDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "acb_worker_match_duration_seconds",
		Help:    "Match execution duration in seconds.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
	})

	// RateLimitHits counts requests rejected by rate limiting.
	RateLimitHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "acb_rate_limit_hits_total",
		Help: "Total number of requests rejected by rate limiting.",
	}, []string{"endpoint"})
)

func init() {
	prometheus.MustRegister(
		MatchThroughput,
		JobQueueDepth,
		BotCrashed,
		StaleJobCount,
		R2BytesUsed,
		ReplayUploadLatency,
		EvolverGenerations,
		IndexBuildDuration,
		HTTPRequestsTotal,
		BotsActive,
		BotsFailing,
		WorkerMatchesTotal,
		WorkerMatchErrorsTotal,
		WorkerJobsClaimedTotal,
		WorkerMatchDuration,
		RateLimitHits,
	)
}

// Handler returns an http.Handler that serves /metrics.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

// StartServer starts a Prometheus metrics HTTP server. Returns the server
// so the caller can shut it down gracefully. The address defaults to
// ACB_METRICS_ADDR env var, falling back to ":9090".
func StartServer() *http.Server {
	addr := os.Getenv("ACB_METRICS_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	srv := &http.Server{Addr: addr, Handler: Handler()}
	go srv.ListenAndServe()
	return srv
}

// HTTPMiddleware wraps an http.Handler to count requests via HTTPRequestsTotal.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(sw.status)).Inc()
		_ = start
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
