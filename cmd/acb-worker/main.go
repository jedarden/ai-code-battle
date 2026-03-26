// acb-worker: Match execution worker for AI Code Battle
//
// This worker polls the Cloudflare Worker API for pending match jobs,
// executes matches using the game engine, uploads replays to R2,
// and submits results back to the API.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aicodebattle/acb/engine"
)

// Config holds worker configuration.
type Config struct {
	APIEndpoint string        // Worker API endpoint (e.g., https://api.aicodebattle.com)
	APIKey      string        // Worker API key for authentication
	R2Endpoint  string        // R2 endpoint for replay uploads
	R2Bucket    string        // R2 bucket name
	R2AccessKey string        // R2 access key ID
	R2SecretKey string        // R2 secret access key
	WorkerID    string        // Unique worker identifier
	PollPeriod  time.Duration // How often to poll for jobs
	Heartbeat   time.Duration // How often to send heartbeat during match
	TurnTimeout time.Duration // Per-turn timeout for bots
	MaxRetries  int           // Max retries for transient errors
	Verbose     bool          // Enable verbose logging
}

func main() {
	// Parse command-line flags
	apiEndpoint := flag.String("api", getEnv("ACB_API_ENDPOINT", "http://localhost:8787"), "Worker API endpoint")
	apiKey := flag.String("api-key", getEnv("ACB_API_KEY", ""), "Worker API key")
	r2Endpoint := flag.String("r2-endpoint", getEnv("ACB_R2_ENDPOINT", ""), "R2 endpoint URL")
	r2Bucket := flag.String("r2-bucket", getEnv("ACB_R2_BUCKET", "acb-data"), "R2 bucket name")
	r2AccessKey := flag.String("r2-access-key", getEnv("ACB_R2_ACCESS_KEY", ""), "R2 access key ID")
	r2SecretKey := flag.String("r2-secret-key", getEnv("ACB_R2_SECRET_KEY", ""), "R2 secret access key")
	workerID := flag.String("worker-id", getEnv("ACB_WORKER_ID", generateWorkerID()), "Unique worker identifier")
	pollPeriod := flag.Duration("poll", 5*time.Second, "Job polling period")
	heartbeat := flag.Duration("heartbeat", 30*time.Second, "Heartbeat interval during matches")
	turnTimeout := flag.Duration("timeout", 3*time.Second, "Per-turn bot timeout")
	maxRetries := flag.Int("retries", 3, "Max retries for transient errors")
	verbose := flag.Bool("verbose", getEnv("ACB_VERBOSE", "false") == "true", "Enable verbose logging")
	flag.Parse()

	// Validate required config
	if *apiKey == "" {
		log.Fatal("API key is required (set ACB_API_KEY or use -api-key flag)")
	}

	cfg := &Config{
		APIEndpoint: *apiEndpoint,
		APIKey:      *apiKey,
		R2Endpoint:  *r2Endpoint,
		R2Bucket:    *r2Bucket,
		R2AccessKey: *r2AccessKey,
		R2SecretKey: *r2SecretKey,
		WorkerID:    *workerID,
		PollPeriod:  *pollPeriod,
		Heartbeat:   *heartbeat,
		TurnTimeout: *turnTimeout,
		MaxRetries:  *maxRetries,
		Verbose:     *verbose,
	}

	// Create API client
	apiClient := NewAPIClient(cfg)

	// Create R2 client (optional - if not configured, replays won't be uploaded)
	var r2Client *R2Client
	if cfg.R2Endpoint != "" && cfg.R2AccessKey != "" && cfg.R2SecretKey != "" {
		r2Client = NewR2Client(cfg)
	}

	// Create metrics
	metrics := NewMetrics(cfg.WorkerID)

	// Create worker
	worker := &Worker{
		cfg:       cfg,
		api:       apiClient,
		r2:        r2Client,
		metrics:   metrics,
		logger:    log.New(os.Stdout, fmt.Sprintf("[worker-%s] ", cfg.WorkerID), log.LstdFlags),
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		heartbeat: *heartbeat,
	}

	// Start metrics HTTP server
	metricsAddr := getEnv("ACB_METRICS_ADDR", ":9090")
	metricsServer := &http.Server{
		Addr:    metricsAddr,
		Handler: metrics.Handler(),
	}
	go func() {
		worker.logger.Printf("Metrics server listening on %s", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			worker.logger.Printf("Metrics server error: %v", err)
		}
	}()

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		worker.logger.Println("Received shutdown signal, finishing current job...")
		cancel()
	}()

	// Run worker loop
	worker.Run(ctx)

	// Shut down metrics server gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	metricsServer.Shutdown(shutdownCtx)
}

// getEnv gets an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// generateWorkerID generates a random worker ID.
func generateWorkerID() string {
	return fmt.Sprintf("worker-%d", rand.Intn(100000))
}

// Worker executes match jobs.
type Worker struct {
	cfg       *Config
	api       *APIClient
	r2        *R2Client
	metrics   *Metrics
	logger    *log.Logger
	rng       *rand.Rand
	heartbeat time.Duration
}

// Run starts the worker loop.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Println("Worker started, polling for jobs...")

	ticker := time.NewTicker(w.cfg.PollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Println("Worker shutting down")
			return
		case <-ticker.C:
			if err := w.pollAndExecute(ctx); err != nil {
				w.logger.Printf("Error in poll cycle: %v", err)
			}
		}
	}
}

// pollAndExecute polls for a job and executes it if available.
func (w *Worker) pollAndExecute(ctx context.Context) error {
	w.metrics.RecordPollCycle()

	// Get next pending job
	job, err := w.api.GetNextJob(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next job: %w", err)
	}

	if job == nil {
		if w.cfg.Verbose {
			w.logger.Println("No pending jobs")
		}
		return nil
	}

	w.logger.Printf("Found job %s for match %s", job.ID, job.MatchID)

	// Claim the job
	claimResp, err := w.api.ClaimJob(ctx, job.ID, w.cfg.WorkerID)
	if err != nil {
		return fmt.Errorf("failed to claim job %s: %w", job.ID, err)
	}

	w.metrics.RecordJobClaimed()
	w.logger.Printf("Claimed job %s, executing match...", job.ID)

	// Execute the match
	matchStart := time.Now()
	result, replay, err := w.executeMatch(ctx, claimResp)
	if err != nil {
		w.metrics.RecordMatchError()
		w.logger.Printf("Match execution failed: %v", err)
		// Mark job as failed
		if failErr := w.api.FailJob(ctx, job.ID, w.cfg.WorkerID, err.Error()); failErr != nil {
			w.metrics.RecordJobFailed()
			w.logger.Printf("Failed to mark job as failed: %v", failErr)
		}
		return err
	}
	w.metrics.RecordMatch(time.Since(matchStart))

	// Upload replay to R2
	replayURL := ""
	if w.r2 != nil {
		uploadStart := time.Now()
		replayData, _ := json.Marshal(replay)
		replayURL, err = w.uploadReplay(ctx, claimResp.Match.ID, replay)
		if err != nil {
			w.metrics.RecordReplayUploadError()
			w.logger.Printf("Failed to upload replay: %v", err)
			// Continue without replay URL - match result is more important
		} else {
			w.metrics.RecordReplayUpload(time.Since(uploadStart), len(replayData))
			w.logger.Printf("Uploaded replay to %s", replayURL)
		}
	}

	// Submit result
	err = w.api.SubmitResult(ctx, job.ID, result, replayURL)
	if err != nil {
		return fmt.Errorf("failed to submit result for job %s: %w", job.ID, err)
	}

	w.logger.Printf("Completed job %s, winner: %s", job.ID, result.WinnerID)
	return nil
}

// executeMatch runs a match and returns the result and replay.
func (w *Worker) executeMatch(ctx context.Context, claim *JobClaimResponse) (*MatchResult, *engine.Replay, error) {
	// Build game config from map data
	config := engine.Config{
		Rows:          claim.Map.Width,
		Cols:          claim.Map.Height,
		MaxTurns:      500, // Default max turns
		VisionRadius2: 49,  // Default vision
		AttackRadius2: 5,   // Default attack
		SpawnCost:     3,   // Default spawn cost
		EnergyInterval: 10, // Default energy interval
	}

	// Create match runner
	runner := engine.NewMatchRunner(config,
		engine.WithRNG(w.rng),
		engine.WithVerbose(w.cfg.Verbose),
		engine.WithTimeout(w.cfg.TurnTimeout),
	)

	// Add bots from claim response
	for _, participant := range claim.Participants {
		// Find bot endpoint
		var endpointURL string
		for _, bot := range claim.Bots {
			if bot.ID == participant.BotID {
				endpointURL = bot.EndpointURL
				break
			}
		}

		// Find bot secret
		var secret string
		for _, s := range claim.BotSecrets {
			if s.BotID == participant.BotID {
				secret = s.Secret
				break
			}
		}

		// Create auth config for HTTP bot
		auth := engine.AuthConfig{
			BotID:   participant.BotID,
			Secret:  secret,
			MatchID: claim.Match.ID,
		}

		// Create HTTP bot client
		httpBot := engine.NewHTTPBot(
			endpointURL,
			auth,
			engine.WithHTTPTimeout(w.cfg.TurnTimeout),
		)

		runner.AddBot(httpBot, participant.BotID)
		w.logger.Printf("Added bot %s at %s (player %d)", participant.BotID, endpointURL, participant.PlayerIndex)
	}

	// Start heartbeat goroutine
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()

	go w.sendHeartbeats(heartbeatCtx, claim.Job.ID)

	// Run the match
	engineResult, replay, err := runner.Run()
	if err != nil {
		return nil, nil, fmt.Errorf("match execution failed: %w", err)
	}

	// Convert result
	result := &MatchResult{
		WinnerID:   "",
		Turns:      engineResult.Turns,
		EndReason:  engineResult.Reason,
		Scores:     make(map[string]int),
	}

	// Set winner ID from result (Winner is int, -1 for draw)
	if engineResult.Winner >= 0 && engineResult.Winner < len(claim.Participants) {
		for _, p := range claim.Participants {
			if p.PlayerIndex == engineResult.Winner {
				result.WinnerID = p.BotID
				break
			}
		}
	}

	// Calculate scores from replay
	for i, p := range claim.Participants {
		if i < len(engineResult.Scores) {
			result.Scores[p.BotID] = engineResult.Scores[i]
		}
	}

	return result, replay, nil
}

// sendHeartbeats sends periodic heartbeats while a match is running.
func (w *Worker) sendHeartbeats(ctx context.Context, jobID string) {
	ticker := time.NewTicker(w.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.api.Heartbeat(ctx, jobID, w.cfg.WorkerID); err != nil {
				w.metrics.RecordHeartbeatError()
				w.logger.Printf("Heartbeat failed: %v", err)
			} else {
				w.metrics.RecordHeartbeat()
			}
		}
	}
}

// uploadReplay uploads the replay to R2 and returns the URL.
func (w *Worker) uploadReplay(ctx context.Context, matchID string, replay *engine.Replay) (string, error) {
	if w.r2 == nil {
		return "", fmt.Errorf("R2 client not configured")
	}

	// Serialize replay to JSON
	data, err := json.Marshal(replay)
	if err != nil {
		return "", fmt.Errorf("failed to serialize replay: %w", err)
	}

	// Upload to R2
	key := fmt.Sprintf("replays/%s.json", matchID)
	if err := w.r2.Upload(ctx, key, data, "application/json"); err != nil {
		return "", fmt.Errorf("failed to upload replay to R2: %w", err)
	}

	return fmt.Sprintf("%s/%s", w.r2.Endpoint(), key), nil
}

// MatchResult represents the result of a match for API submission.
type MatchResult struct {
	WinnerID  string         `json:"winner_id"`
	Turns     int            `json:"turns"`
	EndReason string         `json:"end_reason"`
	Scores    map[string]int `json:"scores"`
}
