// acb-worker: Match execution worker for AI Code Battle
//
// This worker polls PostgreSQL for pending match jobs,
// executes matches using the game engine, uploads replays to B2,
// writes results directly to PostgreSQL, and performs Glicko-2 rating updates.
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
	DatabaseURL  string        // PostgreSQL connection URL
	B2Endpoint   string        // B2 endpoint URL
	B2Bucket     string        // B2 bucket name
	B2AccessKey  string        // B2 access key ID
	B2SecretKey  string        // B2 secret access key
	B2Region     string        // B2 region (e.g., "us-west-004")
	WorkerID     string        // Unique worker identifier
	PollPeriod   time.Duration // How often to poll for jobs
	Heartbeat    time.Duration // How often to send heartbeat during match
	TurnTimeout  time.Duration // Per-turn timeout for bots
	MaxRetries   int           // Max retries for transient errors
	Verbose      bool          // Enable verbose logging
}

func main() {
	// Parse command-line flags
	databaseURL := flag.String("db", getEnv("ACB_DATABASE_URL", ""), "PostgreSQL connection URL")
	b2Endpoint := flag.String("b2-endpoint", getEnv("ACB_B2_ENDPOINT", ""), "B2 endpoint URL")
	b2Bucket := flag.String("b2-bucket", getEnv("ACB_B2_BUCKET", "acb-data"), "B2 bucket name")
	b2AccessKey := flag.String("b2-access-key", getEnv("ACB_B2_ACCESS_KEY", ""), "B2 access key ID")
	b2SecretKey := flag.String("b2-secret-key", getEnv("ACB_B2_SECRET_KEY", ""), "B2 secret access key")
	b2Region := flag.String("b2-region", getEnv("ACB_B2_REGION", "us-west-004"), "B2 region")
	workerID := flag.String("worker-id", getEnv("ACB_WORKER_ID", generateWorkerID()), "Unique worker identifier")
	pollPeriod := flag.Duration("poll", 5*time.Second, "Job polling period")
	heartbeat := flag.Duration("heartbeat", 30*time.Second, "Heartbeat interval during matches")
	turnTimeout := flag.Duration("timeout", 3*time.Second, "Per-turn bot timeout")
	maxRetries := flag.Int("retries", 3, "Max retries for transient errors")
	verbose := flag.Bool("verbose", getEnv("ACB_VERBOSE", "false") == "true", "Enable verbose logging")
	flag.Parse()

	// Validate required config
	if *databaseURL == "" {
		log.Fatal("Database URL is required (set ACB_DATABASE_URL or use -db flag)")
	}

	cfg := &Config{
		DatabaseURL:  *databaseURL,
		B2Endpoint:   *b2Endpoint,
		B2Bucket:     *b2Bucket,
		B2AccessKey:  *b2AccessKey,
		B2SecretKey:  *b2SecretKey,
		B2Region:     *b2Region,
		WorkerID:     *workerID,
		PollPeriod:   *pollPeriod,
		Heartbeat:    *heartbeat,
		TurnTimeout:  *turnTimeout,
		MaxRetries:   *maxRetries,
		Verbose:      *verbose,
	}

	// Create database client
	dbClient, err := NewDBClient(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	// Create B2 client (optional - if not configured, replays won't be uploaded)
	var b2Client *B2Client
	if cfg.B2Endpoint != "" && cfg.B2AccessKey != "" && cfg.B2SecretKey != "" {
		b2Client = NewB2Client(cfg)
	}

	// Create metrics
	metrics := NewMetrics(cfg.WorkerID)

	// Create worker
	worker := &Worker{
		cfg:       cfg,
		db:        dbClient,
		b2:        b2Client,
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
	db        *DBClient
	b2        *B2Client
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
	job, err := w.db.GetNextJob(ctx)
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

	// Claim the job and get match data
	claimData, err := w.db.ClaimJob(ctx, job.ID, w.cfg.WorkerID)
	if err != nil {
		return fmt.Errorf("failed to claim job %s: %w", job.ID, err)
	}

	w.metrics.RecordJobClaimed()
	w.logger.Printf("Claimed job %s, executing match...", job.ID)

	// Execute the match
	matchStart := time.Now()
	result, replay, err := w.executeMatch(ctx, claimData)
	if err != nil {
		w.metrics.RecordMatchError()
		w.logger.Printf("Match execution failed: %v", err)
		// Mark job as failed
		if failErr := w.db.FailJob(ctx, job.ID, w.cfg.WorkerID, err.Error()); failErr != nil {
			w.metrics.RecordJobFailed()
			w.logger.Printf("Failed to mark job as failed: %v", failErr)
		}
		return err
	}
	w.metrics.RecordMatch(time.Since(matchStart))

	// Upload replay to B2
	replayURL := ""
	if w.b2 != nil {
		uploadStart := time.Now()
		replayData, _ := json.Marshal(replay)
		replayURL, err = w.uploadReplay(ctx, claimData.Match.ID, replay)
		if err != nil {
			w.metrics.RecordReplayUploadError()
			w.logger.Printf("Failed to upload replay: %v", err)
			// Continue without replay URL - match result is more important
		} else {
			w.metrics.RecordReplayUpload(time.Since(uploadStart), len(replayData))
			w.logger.Printf("Uploaded replay to %s", replayURL)
		}
	}

	// Compute Glicko-2 rating updates
	ratingUpdates := w.computeRatingUpdates(claimData, result)
	w.logger.Printf("Computed %d rating updates", len(ratingUpdates))

	// Submit result directly to PostgreSQL
	err = w.db.SubmitMatchResult(ctx, job.ID, result, replayURL, ratingUpdates)
	if err != nil {
		return fmt.Errorf("failed to submit result for job %s: %w", job.ID, err)
	}

	w.logger.Printf("Completed job %s, winner: %s", job.ID, result.WinnerID)
	return nil
}

// executeMatch runs a match and returns the result and replay.
func (w *Worker) executeMatch(ctx context.Context, claimData *JobClaimData) (*MatchResult, *engine.Replay, error) {
	// Build game config from map data
	config := engine.Config{
		Rows:           claimData.Map.Width,
		Cols:           claimData.Map.Height,
		MaxTurns:       500, // Default max turns
		VisionRadius2:  49,  // Default vision
		AttackRadius2:  5,   // Default attack
		SpawnCost:      3,   // Default spawn cost
		EnergyInterval: 10,  // Default energy interval
	}

	// Create match runner
	runner := engine.NewMatchRunner(config,
		engine.WithRNG(w.rng),
		engine.WithVerbose(w.cfg.Verbose),
		engine.WithTimeout(w.cfg.TurnTimeout),
	)

	// Build bot ID to info lookup
	botInfoMap := make(map[string]DBBotInfo)
	for _, bot := range claimData.Bots {
		botInfoMap[bot.ID] = bot
	}

	// Add bots from claim data (in player slot order)
	participantMap := make(map[int]DBParticipant)
	for _, p := range claimData.Participants {
		participantMap[p.PlayerSlot] = p
	}

	for slot := 0; slot < len(claimData.Participants); slot++ {
		p := participantMap[slot]
		botInfo := botInfoMap[p.BotID]

		// Create auth config for HTTP bot
		auth := engine.AuthConfig{
			BotID:   p.BotID,
			Secret:  botInfo.Secret,
			MatchID: claimData.Match.ID,
		}

		// Create HTTP bot client
		httpBot := engine.NewHTTPBot(
			botInfo.EndpointURL,
			auth,
			engine.WithHTTPTimeout(w.cfg.TurnTimeout),
		)

		runner.AddBot(httpBot, p.BotID)
		w.logger.Printf("Added bot %s at %s (player %d)", p.BotID, botInfo.EndpointURL, p.PlayerSlot)
	}

	// Start heartbeat goroutine
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()

	go w.sendHeartbeats(heartbeatCtx, claimData.Job.ID)

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
	if engineResult.Winner >= 0 && engineResult.Winner < len(claimData.Participants) {
		for _, p := range claimData.Participants {
			if p.PlayerSlot == engineResult.Winner {
				result.WinnerID = p.BotID
				break
			}
		}
	}

	// Calculate scores from replay
	for _, p := range claimData.Participants {
		if p.PlayerSlot < len(engineResult.Scores) {
			result.Scores[p.BotID] = engineResult.Scores[p.PlayerSlot]
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
			if err := w.db.Heartbeat(ctx, jobID, w.cfg.WorkerID); err != nil {
				w.metrics.RecordHeartbeatError()
				w.logger.Printf("Heartbeat failed: %v", err)
			} else {
				w.metrics.RecordHeartbeat()
			}
		}
	}
}

// uploadReplay uploads the replay to B2 and returns the URL.
func (w *Worker) uploadReplay(ctx context.Context, matchID string, replay *engine.Replay) (string, error) {
	if w.b2 == nil {
		return "", fmt.Errorf("B2 client not configured")
	}

	// Serialize replay to JSON
	data, err := json.Marshal(replay)
	if err != nil {
		return "", fmt.Errorf("failed to serialize replay: %w", err)
	}

	// Upload to B2
	key := fmt.Sprintf("replays/%s.json", matchID)
	if err := w.b2.Upload(ctx, key, data, "application/json"); err != nil {
		return "", fmt.Errorf("failed to upload replay to B2: %w", err)
	}

	return fmt.Sprintf("%s/%s", w.b2.Endpoint(), key), nil
}

// computeRatingUpdates computes Glicko-2 rating updates for match participants.
func (w *Worker) computeRatingUpdates(claimData *JobClaimData, result *MatchResult) []RatingUpdate {
	if len(claimData.Participants) < 2 {
		return nil
	}

	// Extract bot IDs and current ratings
	botIDs := make([]string, len(claimData.Participants))
	ratings := make([]Glicko2Rating, len(claimData.Participants))
	scores := make([]float64, len(claimData.Participants))

	for i, p := range claimData.Participants {
		botIDs[i] = p.BotID
		ratings[i] = Glicko2Rating{
			Mu:    p.RatingMuBefore,
			Phi:   p.RatingPhiBefore,
			Sigma: p.RatingSigmaBefore,
		}
		// Normalize scores for Glicko-2 (higher is better)
		scores[i] = float64(result.Scores[p.BotID])
	}

	// Compute rating updates
	return ComputeRatingUpdates(botIDs, ratings, scores)
}
