// acb-worker: Match execution worker for AI Code Battle
//
// This worker polls PostgreSQL for pending match jobs,
// executes matches using the game engine, uploads replays to B2,
// writes results directly to PostgreSQL, and performs Glicko-2 rating updates.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aicodebattle/acb/engine"
	"github.com/aicodebattle/acb/metrics"
	"image/png"
)

// Config holds worker configuration.
type Config struct {
	DatabaseURL   string        // PostgreSQL connection URL
	EncryptionKey string        // AES-256-GCM key for decrypting bot shared secrets
	B2Endpoint    string        // B2 endpoint URL (ARMOR proxy)
	B2Bucket      string        // B2 bucket name
	B2AccessKey   string        // B2 access key ID
	B2SecretKey   string        // B2 secret access key
	B2Region      string        // B2 region (e.g., "us-west-004")
	R2Endpoint    string        // R2 endpoint URL (Cloudflare R2 S3 API)
	R2Bucket      string        // R2 bucket name
	R2AccessKey   string        // R2 access key ID
	R2SecretKey   string        // R2 secret access key
	WorkerID      string        // Unique worker identifier
	PollPeriod    time.Duration // How often to poll for jobs
	Heartbeat     time.Duration // How often to send heartbeat during match
	TurnTimeout   time.Duration // Per-turn timeout for bots
	MaxRetries    int           // Max retries for transient errors
	Verbose       bool          // Enable verbose logging
}

func main() {
	// Parse command-line flags
	databaseURL := flag.String("db", getEnv("ACB_DATABASE_URL", ""), "PostgreSQL connection URL")
	encryptionKey := flag.String("encryption-key", getEnv("ACB_ENCRYPTION_KEY", ""), "AES-256-GCM key for decrypting bot secrets")
	b2Endpoint := flag.String("b2-endpoint", getEnv("ACB_B2_ENDPOINT", ""), "B2 endpoint URL")
	b2Bucket := flag.String("b2-bucket", getEnv("ACB_B2_BUCKET", "acb-data"), "B2 bucket name")
	b2AccessKey := flag.String("b2-access-key", getEnv("ACB_B2_ACCESS_KEY", ""), "B2 access key ID")
	b2SecretKey := flag.String("b2-secret-key", getEnv("ACB_B2_SECRET_KEY", ""), "B2 secret access key")
	b2Region := flag.String("b2-region", getEnv("ACB_B2_REGION", "us-west-004"), "B2 region")
	r2Endpoint := flag.String("r2-endpoint", getEnv("ACB_R2_ENDPOINT", ""), "R2 endpoint URL")
	r2Bucket := flag.String("r2-bucket", getEnv("ACB_R2_BUCKET", ""), "R2 bucket name")
	r2AccessKey := flag.String("r2-access-key", getEnv("ACB_R2_ACCESS_KEY", ""), "R2 access key ID")
	r2SecretKey := flag.String("r2-secret-key", getEnv("ACB_R2_SECRET_KEY", ""), "R2 secret access key")
	workerID := flag.String("worker-id", getEnv("ACB_WORKER_ID", generateWorkerID()), "Unique worker identifier")
	pollPeriod := flag.Duration("poll", 5*time.Second, "Job polling period")
	heartbeat := flag.Duration("heartbeat", 30*time.Second, "Heartbeat interval during matches")
	turnTimeout := flag.Duration("timeout", 3*time.Second, "Per-turn bot timeout")
	maxRetries := flag.Int("retries", 3, "Max retries for transient errors")
	verbose := flag.Bool("verbose", getEnv("ACB_VERBOSE", "false") == "true", "Enable verbose logging")
	mode := flag.String("mode", "worker", "Operation mode: 'worker' (normal polling) or 'recalc-ratings' (disaster recovery)")
	flag.Parse()

	// Validate required config
	if *databaseURL == "" {
		log.Fatal("Database URL is required (set ACB_DATABASE_URL or use -db flag)")
	}

	cfg := &Config{
		DatabaseURL:   *databaseURL,
		EncryptionKey: *encryptionKey,
		B2Endpoint:    *b2Endpoint,
		B2Bucket:      *b2Bucket,
		B2AccessKey:   *b2AccessKey,
		B2SecretKey:   *b2SecretKey,
		B2Region:      *b2Region,
		R2Endpoint:    *r2Endpoint,
		R2Bucket:      *r2Bucket,
		R2AccessKey:   *r2AccessKey,
		R2SecretKey:   *r2SecretKey,
		WorkerID:      *workerID,
		PollPeriod:    *pollPeriod,
		Heartbeat:     *heartbeat,
		TurnTimeout:   *turnTimeout,
		MaxRetries:    *maxRetries,
		Verbose:       *verbose,
	}

	// Create database client
	dbClient, err := NewDBClient(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	// Handle different operation modes
	switch *mode {
	case "recalc-ratings":
		// Disaster recovery: recompute all ratings from match history
		logger := log.New(os.Stdout, "[recalc-ratings] ", log.LstdFlags)
		if err := recalcRatings(context.Background(), dbClient, logger, *verbose); err != nil {
			log.Fatalf("Rating recalculation failed: %v", err)
		}
		logger.Println("Rating recalculation completed successfully")
		return
	}

	// Normal worker mode (default)

	// Create B2 client (optional - if not configured, replays won't be uploaded to cold archive)
	var b2Client *B2Client
	if cfg.B2Endpoint != "" && cfg.B2AccessKey != "" && cfg.B2SecretKey != "" {
		b2Client = NewB2Client(cfg)
	}

	// Create R2 client (optional - if configured, replays are written to R2 immediately,
	// making them available without waiting for the B2→R2 promotion cycle)
	var r2Client *B2Client
	if cfg.R2Endpoint != "" && cfg.R2AccessKey != "" && cfg.R2SecretKey != "" {
		r2Client = NewR2Client(cfg)
	}

	// Create metrics
	wMetrics := NewMetrics(cfg.WorkerID)

	// Create worker
	worker := &Worker{
		cfg:       cfg,
		db:        dbClient,
		b2:        b2Client,
		r2:        r2Client,
		metrics:   wMetrics,
		logger:    log.New(os.Stdout, fmt.Sprintf("[worker-%s] ", cfg.WorkerID), log.LstdFlags),
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		heartbeat: *heartbeat,
	}

	// Start Prometheus metrics server (shared package provides /metrics + /health)
	metricsSrv := metrics.StartServer()
	defer metricsSrv.Close()

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
	r2        *B2Client
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
	metrics.WorkerJobsClaimedTotal.Inc()
	w.logger.Printf("Claimed job %s, executing match...", job.ID)

	// Execute the match
	matchStart := time.Now()
	result, replay, err := w.executeMatch(ctx, claimData)
	if err != nil {
		w.metrics.RecordMatchError()
		metrics.WorkerMatchErrorsTotal.Inc()
		w.logger.Printf("Match execution failed: %v", err)
		// Mark job as failed
		if failErr := w.db.FailJob(ctx, job.ID, w.cfg.WorkerID, err.Error()); failErr != nil {
			w.metrics.RecordJobFailed()
			w.logger.Printf("Failed to mark job as failed: %v", failErr)
		}
		return err
	}
	w.metrics.RecordMatch(time.Since(matchStart))
	metrics.MatchThroughput.Inc()
	metrics.WorkerMatchesTotal.Inc()
	metrics.WorkerMatchDuration.Observe(time.Since(matchStart).Seconds())

	// Upload replay to B2
	replayURL := ""
	if w.b2 != nil {
		uploadStart := time.Now()
		replayURL, err = w.uploadReplay(ctx, claimData.Match.ID, replay)
		uploadSec := time.Since(uploadStart).Seconds()
		if err != nil {
			w.metrics.RecordReplayUploadError()
			w.logger.Printf("Failed to upload replay: %v", err)
		} else {
			replaySize, _ := json.Marshal(replay)
			w.metrics.RecordReplayUpload(time.Since(uploadStart), len(replaySize))
			metrics.ReplayUploadLatency.Observe(uploadSec)
			w.logger.Printf("Uploaded replay to %s", replayURL)
		}

		// Generate and upload thumbnail
		thumbStart := time.Now()
		if thumbErr := w.uploadThumbnail(ctx, claimData.Match.ID, replay); thumbErr != nil {
			w.logger.Printf("Failed to upload thumbnail: %v", thumbErr)
		} else {
			thumbSec := time.Since(thumbStart).Seconds()
			w.logger.Printf("Uploaded thumbnail in %.2fs", thumbSec)
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

	// Update live-tail.json and live-delta.json in R2 for near-real-time freshness
	if w.r2 != nil {
		if tailErr := w.updateLiveTail(ctx, claimData, result); tailErr != nil {
			w.logger.Printf("Warning: failed to update live-tail.json: %v", tailErr)
		}
		if deltaErr := w.updateLiveDelta(ctx, claimData, result, ratingUpdates); deltaErr != nil {
			w.logger.Printf("Warning: failed to update live-delta.json: %v", deltaErr)
		}
	}

	w.logger.Printf("Completed job %s, winner: %s", job.ID, result.WinnerID)
	return nil
}

// executeMatch runs a match and returns the result and replay.
func (w *Worker) executeMatch(ctx context.Context, claimData *JobClaimData) (*MatchResult, *engine.Replay, error) {
	// Build game config using ConfigForPlayers to get proper attack radius and zone parameters
	numPlayers := len(claimData.Participants)
	config := engine.ConfigForPlayers(numPlayers, 2) // 2 cores per player default

	// Override grid dimensions from the pre-generated map
	config.Rows = claimData.Map.Width
	config.Cols = claimData.Map.Height

	// Set match metadata
	config.SeasonID = claimData.Match.SeasonID
	config.RulesVersion = claimData.Match.RulesVersion

	// Prepare pre-generated map data for the match runner
	preGenMap := engine.PreGeneratedMap{
		WallsJSON: claimData.Map.Walls,
		CoresJSON: claimData.Map.Cores,
	}

	// Create match runner with pre-generated map
	runner := engine.NewMatchRunner(config,
		engine.WithRNG(w.rng),
		engine.WithVerbose(w.cfg.Verbose),
		engine.WithTimeout(w.cfg.TurnTimeout),
		engine.WithMap(preGenMap),
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

		// Decrypt the bot's shared secret if an encryption key is configured.
		// The API stores secrets AES-GCM encrypted; bots use the plaintext key.
		secret := botInfo.Secret
		if w.cfg.EncryptionKey != "" {
			plaintext, err := decryptSecret(botInfo.Secret, w.cfg.EncryptionKey)
			if err != nil {
				w.logger.Printf("Warning: failed to decrypt secret for bot %s: %v — using raw value", p.BotID, err)
			} else {
				secret = plaintext
			}
		}

		// Create auth config for HTTP bot
		auth := engine.AuthConfig{
			BotID:   p.BotID,
			Secret:  secret,
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
		WinnerID:    "",
		Turns:       engineResult.Turns,
		EndReason:   engineResult.Reason,
		Scores:      make(map[string]int),
		CrashedBots: make(map[string]bool),
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

	// Propagate crash status from engine
	for _, p := range claimData.Participants {
		if p.PlayerSlot < len(engineResult.Crashed) {
			result.CrashedBots[p.BotID] = engineResult.Crashed[p.PlayerSlot]
		}
	}

	// Compute combat_turns: count distinct turns where ≥1 bot died from "combat" (enemy kill)
	result.CombatTurns = computeCombatTurns(replay)

	// Calculate map engagement score from replay
	engagement := engine.CalculateMapEngagement(replay)
	w.logger.Printf("Map engagement: crossings=%.0f, combat_deaths=%d, critical_moments=%d, resource_contest_turns=%d, survival_turns=%d, score=%.2f",
		engagement.WinProbCrossings, engagement.CombatDeaths, engagement.CriticalMoments, engagement.ResourceContestTurns, engagement.SurvivalTurns, engagement.Engagement)

	// Update map engagement in database
	if err := w.db.UpdateMapEngagement(ctx, claimData.Match.MapID, engagement.Engagement, result.Turns); err != nil {
		// Log but don't fail the match — map engagement is non-critical
		w.logger.Printf("Warning: failed to update map engagement: %v", err)
	}

	return result, replay, nil
}

// computeCombatTurns counts the number of distinct turns in a replay where at
// least one bot died from focus-fire combat (EventCombatDeath). Deaths from
// self-collision, zone, or other causes are excluded.
func computeCombatTurns(replay *engine.Replay) int {
	if replay == nil {
		return 0
	}
	combatTurnSet := make(map[int]struct{})
	for _, turn := range replay.Turns {
		for _, event := range turn.Events {
			if event.Type == engine.EventCombatDeath {
				combatTurnSet[turn.Turn] = struct{}{}
				break // one combat death is enough to count this turn
			}
		}
	}
	return len(combatTurnSet)
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

// uploadReplay uploads the gzipped replay to B2 (cold archive) and R2 (hot cache).
// Returns error only if both uploads fail; a B2-only failure is logged but not fatal.
func (w *Worker) uploadReplay(ctx context.Context, matchID string, replay *engine.Replay) (string, error) {
	if w.b2 == nil && w.r2 == nil {
		return "", fmt.Errorf("no storage client configured")
	}

	// Serialize replay to JSON
	data, err := json.Marshal(replay)
	if err != nil {
		return "", fmt.Errorf("failed to serialize replay: %w", err)
	}

	// Gzip compress
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return "", fmt.Errorf("failed to gzip replay: %w", err)
	}
	if err := gw.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %w", err)
	}
	compressed := buf.Bytes()

	key := fmt.Sprintf("replays/%s.json.gz", matchID)
	var uploadURL string

	// Upload to B2 via ARMOR (cold archive, encrypted)
	if w.b2 != nil {
		if err := w.b2.Upload(ctx, key, compressed, "application/json", "gzip"); err != nil {
			w.logger.Printf("Warning: failed to upload replay to B2 (non-fatal): %v", err)
		} else {
			uploadURL = fmt.Sprintf("%s/%s", w.b2.Endpoint(), key)
		}
	}

	// Upload to R2 directly (hot cache, bypasses B2→R2 promotion cycle)
	if w.r2 != nil {
		if err := w.r2.Upload(ctx, key, compressed, "application/json", "gzip"); err != nil {
			w.logger.Printf("Warning: failed to upload replay to R2: %v", err)
		} else {
			w.logger.Printf("Uploaded replay to R2: %s/%s", w.r2.Endpoint(), key)
		}
	}

	if uploadURL == "" && w.b2 != nil {
		return "", fmt.Errorf("failed to upload replay to B2")
	}
	if uploadURL == "" {
		uploadURL = fmt.Sprintf("replays/%s.json.gz", matchID)
	}
	return uploadURL, nil
}

// uploadThumbnail generates and uploads a PNG thumbnail to B2 (archive) and R2 (hot cache).
func (w *Worker) uploadThumbnail(ctx context.Context, matchID string, replay *engine.Replay) error {
	if w.b2 == nil && w.r2 == nil {
		return fmt.Errorf("no storage client configured")
	}

	// Generate thumbnail image
	img, err := engine.GenerateMatchThumbnail(replay)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("failed to encode thumbnail as PNG: %w", err)
	}
	thumbData := buf.Bytes()

	key := fmt.Sprintf("thumbnails/%s.png", matchID)

	// Upload to B2 via ARMOR (cold archive, encrypted)
	if w.b2 != nil {
		if err := w.b2.Upload(ctx, key, thumbData, "image/png", ""); err != nil {
			w.logger.Printf("Warning: failed to upload thumbnail to B2 (non-fatal): %v", err)
		}
	}

	// Upload to R2 directly (hot cache)
	if w.r2 != nil {
		if err := w.r2.Upload(ctx, key, thumbData, "image/png", ""); err != nil {
			w.logger.Printf("Warning: failed to upload thumbnail to R2: %v", err)
		}
	}

	return nil
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
		// Use winner identity for pairwise Glicko-2 scoring.
		// Raw game scores (captures) are often tied, so we use the declared
		// winner as the discriminator: winner=1.0, others=0.0, draw=0.5.
		if result.WinnerID == "" {
			scores[i] = 0.5
		} else if result.WinnerID == p.BotID {
			scores[i] = 1.0
		} else {
			scores[i] = 0.0
		}
	}

	// Compute rating updates
	return ComputeRatingUpdates(botIDs, ratings, scores)
}

// recalcRatings recalculates all Glicko-2 ratings from scratch by replaying
// all completed matches in chronological order. Used for disaster recovery
// when ratings are corrupted or lost.
func recalcRatings(ctx context.Context, db *DBClient, logger *log.Logger, verbose bool) error {
	logger.Println("Starting rating recalculation...")
	logger.Println("Step 1: Resetting all bot ratings to defaults")

	// Step 1: Reset all bot ratings to defaults
	if err := db.ResetAllRatings(ctx); err != nil {
		return fmt.Errorf("failed to reset ratings: %w", err)
	}
	logger.Println("  All ratings reset to defaults (mu=1500, phi=350, sigma=0.06)")

	// Step 2: Fetch all completed matches in chronological order
	logger.Println("Step 2: Fetching completed matches in chronological order")
	matches, err := db.GetAllCompletedMatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch matches: %w", err)
	}
	logger.Printf("  Found %d completed matches to process", len(matches))

	if len(matches) == 0 {
		logger.Println("No matches to process, ratings remain at defaults")
		return nil
	}

	// Step 3: Track current ratings in memory
	currentRatings := make(map[string]Glicko2Rating)

	// Step 4: Process each match in order
	logger.Println("Step 3: Replaying matches to recompute ratings")
	processed := 0
	for _, match := range matches {
		// Ensure all participants have default ratings initialized
		for _, p := range match.Participants {
			if _, exists := currentRatings[p.BotID]; !exists {
				currentRatings[p.BotID] = Glicko2Rating{
					Mu:    glicko2DefaultMu,
					Phi:   glicko2DefaultRD,
					Sigma: glicko2DefaultSigma, // default sigma
				}
			}
		}

		// Build arrays for rating computation
		n := len(match.Participants)
		botIDs := make([]string, n)
		ratings := make([]Glicko2Rating, n)
		scores := make([]float64, n)

		for i, p := range match.Participants {
			botIDs[i] = p.BotID
			ratings[i] = currentRatings[p.BotID]

			// Determine score based on match result
			// If winner is a player slot, convert to bot_id and score accordingly
			if match.Winner == nil {
				// Draw or no winner
				scores[i] = 0.5
			} else if match.WinnerBotID != nil && *match.WinnerBotID == p.BotID {
				scores[i] = 1.0
			} else {
				scores[i] = 0.0
			}
		}

		// Compute new ratings using Glicko-2
		newRatings := UpdateRatings(ratings, scores)

		// Update stored ratings
		for i, botID := range botIDs {
			currentRatings[botID] = newRatings[i]
		}

		processed++
		if processed%1000 == 0 || verbose {
			logger.Printf("  Processed %d/%d matches (match_id=%s)", processed, len(matches), match.ID)
		}
	}

	logger.Printf("  Processed all %d matches", processed)

	// Step 5: Write final ratings back to database
	logger.Println("Step 4: Writing recalculated ratings to database")
	if err := db.UpdateAllRatings(ctx, currentRatings); err != nil {
		return fmt.Errorf("failed to write ratings: %w", err)
	}

	logger.Printf("  Updated ratings for %d bots", len(currentRatings))
	logger.Println("Rating recalculation complete")
	return nil
}

// LiveTailMatch represents a single match in the live-tail feed.
type LiveTailMatch struct {
	ID           string                    `json:"id"`
	CompletedAt  string                    `json:"completed_at"`
	Participants []MatchSummaryParticipant `json:"participants"`
	WinnerID     *string                   `json:"winner_id,omitempty"`
	MapID        *string                   `json:"map_id,omitempty"`
	Turns        int                       `json:"turns"`
	EndReason    string                    `json:"end_reason"`
	Enriched     bool                      `json:"enriched,omitempty"`
}

// MatchSummaryParticipant represents a participant in a match summary.
type MatchSummaryParticipant struct {
	BotID string `json:"bot_id"`
	Name  string `json:"name"`
	Score int    `json:"score"`
	Won   bool   `json:"won"`
}

// LiveTailFeed represents the live-tail.json structure.
type LiveTailFeed struct {
	Matches []LiveTailMatch `json:"matches"`
	Updated string          `json:"updated_at"`
}

// updateLiveTail updates matches/live-tail.json in R2 with the latest completed match.
// Maintains a rolling list of the last 50 completed matches.
func (w *Worker) updateLiveTail(ctx context.Context, claimData *JobClaimData, result *MatchResult) error {
	// Download existing live-tail.json
	var feed LiveTailFeed
	existingData, err := w.r2.Download(ctx, "matches/live-tail.json")
	if err == nil && len(existingData) > 0 {
		if err := json.Unmarshal(existingData, &feed); err != nil {
			w.logger.Printf("Warning: failed to parse existing live-tail.json, starting fresh: %v", err)
			feed = LiveTailFeed{Matches: []LiveTailMatch{}}
		}
	} else {
		// File doesn't exist yet, start fresh
		feed = LiveTailFeed{Matches: []LiveTailMatch{}}
	}

	// Build bot ID to name lookup
	botNameMap := make(map[string]string)
	for _, bot := range claimData.Bots {
		botNameMap[bot.ID] = bot.Name
	}

	// Build participants array with names and scores
	participants := make([]MatchSummaryParticipant, len(claimData.Participants))
	for i, p := range claimData.Participants {
		participants[i] = MatchSummaryParticipant{
			BotID: p.BotID,
			Name:  botNameMap[p.BotID],
			Score: result.Scores[p.BotID],
			Won:   result.WinnerID == p.BotID,
		}
	}

	// Determine winner_id (null for draw)
	var winnerID *string
	if result.WinnerID != "" {
		winnerID = &result.WinnerID
	}

	// Create new entry
	entry := LiveTailMatch{
		ID:           claimData.Match.ID,
		CompletedAt:  time.Now().UTC().Format(time.RFC3339),
		Participants: participants,
		WinnerID:     winnerID,
		MapID:        &claimData.Map.ID,
		Turns:        result.Turns,
		EndReason:    result.EndReason,
	}

	// Append new match and trim to last 50
	feed.Matches = append([]LiveTailMatch{entry}, feed.Matches...)
	if len(feed.Matches) > 50 {
		feed.Matches = feed.Matches[:50]
	}
	feed.Updated = time.Now().UTC().Format(time.RFC3339)

	// Serialize and upload
	data, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal live-tail feed: %w", err)
	}

	if err := w.r2.UploadLive(ctx, "matches/live-tail.json", data); err != nil {
		return fmt.Errorf("failed to upload live-tail.json: %w", err)
	}

	w.logger.Printf("Updated live-tail.json with match %s (%d matches in tail)", claimData.Match.ID, len(feed.Matches))
	return nil
}

// LeaderboardDeltaEntry represents rating deltas for a single bot.
type LeaderboardDeltaEntry struct {
	RatingDelta        float64 `json:"rating_delta"`
	NewRating          float64 `json:"new_rating"`
	NewRatingDeviation float64 `json:"new_rating_deviation,omitempty"`
	NewMatchesPlayed   int     `json:"new_matches_played,omitempty"`
	NewMatchesWon      int     `json:"new_matches_won,omitempty"`
	NewWinRate         float64 `json:"new_win_rate,omitempty"`
}

// LiveDeltaFeed represents the live-delta.json structure.
type LiveDeltaFeed struct {
	Deltas  map[string]LeaderboardDeltaEntry `json:"deltas"`
	Updated string                           `json:"updated_at"`
}

// updateLiveDelta updates leaderboard/live-delta.json in R2 with rating changes from the latest match.
// Accumulates deltas since the last full index build.
func (w *Worker) updateLiveDelta(ctx context.Context, claimData *JobClaimData, result *MatchResult, ratingUpdates []RatingUpdate) error {
	// Download existing live-delta.json
	var feed LiveDeltaFeed
	existingData, err := w.r2.Download(ctx, "leaderboard/live-delta.json")
	if err == nil && len(existingData) > 0 {
		if err := json.Unmarshal(existingData, &feed); err != nil {
			w.logger.Printf("Warning: failed to parse existing live-delta.json, starting fresh: %v", err)
			feed = LiveDeltaFeed{Deltas: make(map[string]LeaderboardDeltaEntry)}
		}
	} else {
		// File doesn't exist yet, start fresh
		feed = LiveDeltaFeed{Deltas: make(map[string]LeaderboardDeltaEntry)}
	}

	// Query current bot stats (matches_played, matches_won) from database
	botIDs := make([]string, len(ratingUpdates))
	for i, update := range ratingUpdates {
		botIDs[i] = update.BotID
	}

	// Build bot info map with current stats from database
	botStats := make(map[string]struct {
		matchesPlayed int
		matchesWon    int
	})

	// Query each bot's current stats from the database
	for _, botID := range botIDs {
		var matchesPlayed, matchesWon int
		err := w.db.db.QueryRowContext(ctx, `
			SELECT COALESCE(matches_played, 0), COALESCE(matches_won, 0)
			FROM bots WHERE bot_id = $1
		`, botID).Scan(&matchesPlayed, &matchesWon)
		if err != nil {
			w.logger.Printf("Warning: failed to get stats for bot %s: %v", botID, err)
			// Use zeros as fallback
			matchesPlayed, matchesWon = 0, 0
		}

		// Increment matches played for this match (the update represents the new state),
		// counting the win here too — a map value's fields are not addressable in Go.
		won := 0
		if result.WinnerID == botID {
			won = 1
		}
		botStats[botID] = struct {
			matchesPlayed int
			matchesWon    int
		}{
			matchesPlayed: matchesPlayed + 1,
			matchesWon:    matchesWon + won,
		}
	}

	// Update deltas for each participant
	for _, update := range ratingUpdates {
		stats, ok := botStats[update.BotID]
		if !ok {
			continue
		}

		// Calculate new rating state
		newRating := update.DisplayRating
		newRatingDeviation := update.Phi
		newMatchesPlayed := stats.matchesPlayed
		newMatchesWon := stats.matchesWon
		newWinRate := 0.0
		if newMatchesPlayed > 0 {
			newWinRate = float64(newMatchesWon) / float64(newMatchesPlayed)
		}

		// Calculate rating delta
		ratingDelta := newRating - (update.RatingMuBefore - 2*update.RatingPhiBefore)

		// Store or accumulate delta
		existing, exists := feed.Deltas[update.BotID]
		if !exists {
			existing = LeaderboardDeltaEntry{
				RatingDelta:        ratingDelta,
				NewRating:          newRating,
				NewRatingDeviation: newRatingDeviation,
				NewMatchesPlayed:   newMatchesPlayed,
				NewMatchesWon:      newMatchesWon,
				NewWinRate:         newWinRate,
			}
		} else {
			// Accumulate: add delta, replace absolute values
			existing.RatingDelta += ratingDelta
			existing.NewRating = newRating
			existing.NewRatingDeviation = newRatingDeviation
			existing.NewMatchesPlayed = newMatchesPlayed
			existing.NewMatchesWon = newMatchesWon
			existing.NewWinRate = newWinRate
		}

		feed.Deltas[update.BotID] = existing
	}
	feed.Updated = time.Now().UTC().Format(time.RFC3339)

	// Serialize and upload
	data, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal live-delta feed: %w", err)
	}

	if err := w.r2.UploadLive(ctx, "leaderboard/live-delta.json", data); err != nil {
		return fmt.Errorf("failed to upload live-delta.json: %w", err)
	}

	w.logger.Printf("Updated live-delta.json with %d bot deltas", len(ratingUpdates))
	return nil
}
