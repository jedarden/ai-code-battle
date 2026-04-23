package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aicodebattle/acb/metrics"
	"github.com/aicodebattle/acb/ratelimit"
	"github.com/redis/go-redis/v9"
)

// Server is the v1 API server for AI Code Battle.
// Provides bot registration, job coordination, replay serving,
// bot profiles, leaderboards, and UI feedback ingestion.
type Server struct {
	cfg          Config
	db           *sql.DB
	rdb          *redis.Client
	regLimiter   *ratelimit.Limiter // 5/hour per IP
	feedbackLtr  *ratelimit.Limiter // 20/hour per IP
	predictLtr   *ratelimit.Limiter // 60/hour per IP
	submitLtr    *ratelimit.Limiter // 5/day per bot_id
	voteLtr      *ratelimit.Limiter // 10/hour per IP
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Health endpoints (no rate limit)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)

	// Bot registration — 5/hour per IP
	regMW := s.regLimiter.Middleware(ipKey, func() {
		metrics.RateLimitHits.WithLabelValues("register").Inc()
	})
	mux.HandleFunc("POST /api/register", regMW(http.HandlerFunc(s.handleRegister)).ServeHTTP)

	// Bot profile edit — toggle debug_public (authenticated by shared_secret)
	mux.HandleFunc("PATCH /api/bot/", s.handleBotPatch)

	// Bot key rotation + optional retirement — §8.5
	mux.HandleFunc("POST /api/rotate-key", s.handleRotateKey)

	// Job coordination (for workers — authenticated, no public rate limit)
	mux.HandleFunc("GET /api/job", s.handleGetJob)

	// Job result submission — per-worker 5/day limit
	submitMW := s.submitLtr.Middleware(botIDKey(), func() {
		metrics.RateLimitHits.WithLabelValues("submit").Inc()
	})
	mux.HandleFunc("POST /api/job/", submitMW(http.HandlerFunc(s.handleJobResult)).ServeHTTP)

	// Replay serving (read-only, no rate limit)
	mux.HandleFunc("GET /api/replay/", s.handleGetReplay)

	// Bot profiles and leaderboard (read-only, no rate limit)
	mux.HandleFunc("GET /api/bot/", s.handleGetBot)
	mux.HandleFunc("GET /api/bots", s.handleListBots)

	// Community replay feedback — 20/hour per IP
	fbMW := s.feedbackLtr.Middleware(ipKey, func() {
		metrics.RateLimitHits.WithLabelValues("feedback").Inc()
	})
	voteMW := s.voteLtr.Middleware(ipKey, func() {
		metrics.RateLimitHits.WithLabelValues("vote").Inc()
	})
	mux.HandleFunc("POST /api/feedback", fbMW(http.HandlerFunc(s.handleCreateFeedback)).ServeHTTP)
	mux.HandleFunc("GET /api/feedback/", s.handleGetFeedback)
	mux.HandleFunc("POST /api/feedback/", voteMW(http.HandlerFunc(s.handleFeedbackUpvote)).ServeHTTP)

	// Predictions — 60/hour per IP
	predMW := s.predictLtr.Middleware(ipKey, func() {
		metrics.RateLimitHits.WithLabelValues("predict").Inc()
	})
	mux.HandleFunc("POST /api/predict", predMW(http.HandlerFunc(s.handlePredict)).ServeHTTP)
	mux.HandleFunc("GET /api/predictions/open", s.handleOpenPredictions)
	mux.HandleFunc("GET /api/predictions/history", s.handlePredictionHistory)

	// Map voting — 10/hour per IP (§14.6)
	mapVoteMW := s.voteLtr.Middleware(ipKey, func() {
		metrics.RateLimitHits.WithLabelValues("map_vote").Inc()
	})
	mux.HandleFunc("POST /api/vote/map", mapVoteMW(http.HandlerFunc(s.handleMapVote)).ServeHTTP)
	mux.HandleFunc("GET /api/vote/map/", s.handleGetMapVotes)
}

// ipKey extracts the client IP from the request for rate limiting.
// Respects X-Forwarded-For when present (behind reverse proxy).
func ipKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// botIDKey extracts a rate-limit key for the job submission endpoint.
// Workers submit results authenticated by API key, so we key by worker IP
// to enforce the per-worker submission rate limit (max 5/day).
func botIDKey() func(*http.Request) string {
	return func(r *http.Request) string {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		return "worker:" + host
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleRegister handles POST /api/register
// Request body: {"name": "...", "owner": "...", "endpoint_url": "..."}
// Response: {"bot_id": "...", "shared_secret": "..."}
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Owner       string `json:"owner"`
		EndpointURL string `json:"endpoint_url"`
		DebugPublic bool   `json:"debug_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" || req.Owner == "" || req.EndpointURL == "" {
		writeError(w, http.StatusBadRequest, "name, owner, and endpoint_url are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check if name is already taken
	var existingID string
	err := s.db.QueryRowContext(ctx, "SELECT bot_id FROM bots WHERE name = $1", req.Name).Scan(&existingID)
	if err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("bot name '%s' is already taken", req.Name))
		return
	} else if err != sql.ErrNoRows {
		log.Printf("database error checking bot name: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Generate bot ID and shared secret
	botID, err := generateID("b_", 6)
	if err != nil {
		log.Printf("failed to generate bot ID: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate bot ID")
		return
	}

	sharedSecret, err := generateSecret()
	if err != nil {
		log.Printf("failed to generate secret: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate secret")
		return
	}

	// Encrypt the shared secret
	var encryptedSecret string
	if s.cfg.EncryptionKey != "" {
		encryptedSecret, err = encryptSecret(sharedSecret, s.cfg.EncryptionKey)
		if err != nil {
			log.Printf("failed to encrypt secret: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to encrypt secret")
			return
		}
	} else {
		// If no encryption key configured, store plaintext (not recommended for production)
		encryptedSecret = sharedSecret
	}

	// Validate bot is reachable by sending a health check
	if err := s.validateBotEndpoint(ctx, req.EndpointURL); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("bot endpoint validation failed: %v", err))
		return
	}

	// Insert bot into database
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO bots (bot_id, name, owner, endpoint_url, shared_secret, status, debug_public)
		VALUES ($1, $2, $3, $4, $5, 'active', $6)
	`, botID, req.Name, req.Owner, req.EndpointURL, encryptedSecret, req.DebugPublic)
	if err != nil {
		log.Printf("failed to insert bot: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to register bot")
		return
	}

	log.Printf("registered bot %s (name=%s, owner=%s)", botID, req.Name, req.Owner)

	writeJSON(w, http.StatusCreated, map[string]string{
		"bot_id":        botID,
		"shared_secret": sharedSecret,
	})
}

// validateBotEndpoint checks if the bot endpoint is reachable
func (s *Server) validateBotEndpoint(ctx context.Context, endpointURL string) error {
	// Remove trailing slash for consistency
	endpointURL = strings.TrimRight(endpointURL, "/")

	// Try to GET /health endpoint with a timeout
	healthURL := endpointURL + "/health"
	client := &http.Client{Timeout: time.Duration(s.cfg.BotTimeoutSecs) * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// handleGetJob handles GET /api/job
// Workers poll this endpoint to get the next pending match job.
// Requires Bearer token authentication (worker API key).
// Response: job JSON or empty if no jobs available.
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Authenticate worker
	if !s.authenticateWorker(r) {
		writeError(w, http.StatusUnauthorized, "invalid or missing worker API key")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Query for the next pending job
	var job struct {
		JobID      string          `json:"job_id"`
		MatchID    string          `json:"match_id"`
		ConfigJSON json.RawMessage `json:"config_json"`
	}

	err := s.db.QueryRowContext(ctx, `
		SELECT job_id, match_id, config_json
		FROM jobs
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&job.JobID, &job.MatchID, &job.ConfigJSON)

	if err == sql.ErrNoRows {
		// No pending jobs
		writeJSON(w, http.StatusOK, map[string]string{"status": "no_jobs"})
		return
	} else if err != nil {
		log.Printf("database error getting job: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Parse config_json to get match details
	var config struct {
		MapID     string `json:"map_id"`
		MapSeed   int64  `json:"map_seed"`
		BotIDs    []string `json:"bot_ids"`
		PlayerSlots []int `json:"player_slots"`
	}
	if err := json.Unmarshal(job.ConfigJSON, &config); err != nil {
		log.Printf("failed to parse job config: %v", err)
		writeError(w, http.StatusInternalServerError, "invalid job config")
		return
	}

	// Get map data
	var mapData struct {
		MapID      string          `json:"map_id"`
		GridWidth  int             `json:"grid_width"`
		GridHeight int             `json:"grid_height"`
		MapJSON    json.RawMessage `json:"map_json"`
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT map_id, grid_width, grid_height, map_json
		FROM maps WHERE map_id = $1
	`, config.MapID).Scan(&mapData.MapID, &mapData.GridWidth, &mapData.GridHeight, &mapData.MapJSON)
	if err != nil {
		log.Printf("failed to get map: %v", err)
		writeError(w, http.StatusInternalServerError, "map not found")
		return
	}

	// Get bot endpoints and secrets
	bots := make([]map[string]interface{}, 0, len(config.BotIDs))
	for _, botID := range config.BotIDs {
		var endpointURL, encryptedSecret string
		err := s.db.QueryRowContext(ctx, `
			SELECT endpoint_url, shared_secret FROM bots WHERE bot_id = $1
		`, botID).Scan(&endpointURL, &encryptedSecret)
		if err != nil {
			log.Printf("failed to get bot %s: %v", botID, err)
			writeError(w, http.StatusInternalServerError, "bot not found")
			return
		}

		// Decrypt secret if encryption key is configured
		var sharedSecret string
		if s.cfg.EncryptionKey != "" {
			sharedSecret, err = decryptSecret(encryptedSecret, s.cfg.EncryptionKey)
			if err != nil {
				log.Printf("failed to decrypt secret for bot %s: %v", botID, err)
				// Fall back to treating it as plaintext
				sharedSecret = encryptedSecret
			}
		} else {
			sharedSecret = encryptedSecret
		}

		bots = append(bots, map[string]interface{}{
			"bot_id":        botID,
			"endpoint_url":  endpointURL,
			"shared_secret": sharedSecret,
		})
	}

	// Build response
	response := map[string]interface{}{
		"job_id":        job.JobID,
		"match_id":      job.MatchID,
		"map_id":        config.MapID,
		"map_seed":      config.MapSeed,
		"map_width":     mapData.GridWidth,
		"map_height":    mapData.GridHeight,
		"map_json":      mapData.MapJSON,
		"bots":          bots,
		"player_slots":  config.PlayerSlots,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleJobResult handles POST /api/job/{id}/result
// Workers submit match results here.
// Requires Bearer token authentication.
// Request body: {"winner": "...", "turns": 123, "end_reason": "...", "scores": {...}, "replay": {...}}
func (s *Server) handleJobResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Authenticate worker
	if !s.authenticateWorker(r) {
		writeError(w, http.StatusUnauthorized, "invalid or missing worker API key")
		return
	}

	// Extract job ID from path: /api/job/{id}/result
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 || pathParts[3] != "result" {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	jobID := pathParts[2]

	var req struct {
		WinnerID  string          `json:"winner"`
		Turns     int             `json:"turns"`
		EndReason string          `json:"end_reason"`
		Scores    map[string]int  `json:"scores"`
		Replay    json.RawMessage `json:"replay"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("failed to begin transaction: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback()

	// Get match ID for this job
	var matchID string
	err = tx.QueryRowContext(ctx, "SELECT match_id FROM jobs WHERE job_id = $1", jobID).Scan(&matchID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	} else if err != nil {
		log.Printf("failed to get job: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Update job status
	_, err = tx.ExecContext(ctx, `
		UPDATE jobs SET status = 'completed', completed_at = NOW() WHERE job_id = $1
	`, jobID)
	if err != nil {
		log.Printf("failed to update job: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Determine winner player index
	var winnerIndex *int
	if req.WinnerID != "" {
		var idx int
		err := tx.QueryRowContext(ctx, `
			SELECT player_slot FROM match_participants WHERE match_id = $1 AND bot_id = $2
		`, matchID, req.WinnerID).Scan(&idx)
		if err == nil {
			winnerIndex = &idx
		}
	}

	// Update match status
	scoresJSON, _ := json.Marshal(req.Scores)
	_, err = tx.ExecContext(ctx, `
		UPDATE matches
		SET status = 'completed', winner = $1, condition = $2, turn_count = $3, scores_json = $4, completed_at = NOW()
		WHERE match_id = $5
	`, winnerIndex, req.EndReason, req.Turns, scoresJSON, matchID)
	if err != nil {
		log.Printf("failed to update match: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Update participant scores
	for botID, score := range req.Scores {
		_, err = tx.ExecContext(ctx, `
			UPDATE match_participants SET score = $1 WHERE match_id = $2 AND bot_id = $3
		`, score, matchID, botID)
		if err != nil {
			log.Printf("failed to update participant score: %v", err)
		}
	}

	// Note: Rating updates are handled by the worker separately via the rating endpoint
	// or can be computed here if the ratings are provided in the request

	// Resolve predictions for this match
	if err := s.resolvePredictions(ctx, tx, matchID, req.WinnerID); err != nil {
		log.Printf("failed to resolve predictions for match %s: %v", matchID, err)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("failed to commit transaction: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	log.Printf("completed job %s, match %s, winner %s", jobID, matchID, req.WinnerID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleGetReplay handles GET /api/replay/{id}
// Serves replay JSON from R2 warm cache with B2 fallback.
// Debug telemetry is stripped for bots with debug_public=false unless the
// requester authenticates as that bot's owner via Authorization: Bearer <api_key>.
func (s *Server) handleGetReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract match ID from path: /api/replay/{id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/replay/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid match ID")
		return
	}
	matchID := pathParts[0]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var replayData []byte
	var source string

	// First, try to get from R2 warm cache
	var err error
	replayData, err = s.fetchReplayFromR2(ctx, matchID)
	if err != nil {
		log.Printf("R2 fetch failed for %s: %v", matchID, err)
		// Fall back to B2 cold archive
		replayData, err = s.fetchReplayFromB2(ctx, matchID)
		if err != nil {
			log.Printf("B2 fetch also failed for %s: %v", matchID, err)
			writeError(w, http.StatusNotFound, "replay not found")
			return
		}
		source = "b2"
	}

	// Filter debug telemetry based on bot debug_public settings.
	authHeader := r.Header.Get("Authorization")
	filtered, filterErr := s.filterReplayDebug(ctx, matchID, replayData, authHeader)
	if filterErr != nil {
		log.Printf("warning: failed to filter replay debug for %s: %v", matchID, filterErr)
		// Serve original decompressed bytes on filter failure
		filtered, _ = decompressIfGzip(replayData)
		if filtered == nil {
			filtered = replayData
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if source != "" {
		w.Header().Set("X-ACB-Source", source)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(filtered)
}

// decompressIfGzip returns decompressed data if the input is gzip-encoded,
// otherwise returns the input unchanged.
func decompressIfGzip(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// filterReplayDebug parses the replay JSON and removes debug telemetry for bots
// whose debug_public flag is false. If authHeader is a valid "Bearer <api_key>"
// for one of the bots in the match, that bot's debug data is preserved regardless.
func (s *Server) filterReplayDebug(ctx context.Context, matchID string, data []byte, authHeader string) ([]byte, error) {
	decompressed, err := decompressIfGzip(data)
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}

	// Look up each participant's debug_public flag and encrypted secret.
	type participant struct {
		BotID           string
		PlayerSlot      int
		DebugPublic     bool
		EncryptedSecret string
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT mp.bot_id, mp.player_slot, b.debug_public, b.shared_secret
		FROM match_participants mp
		JOIN bots b ON mp.bot_id = b.bot_id
		WHERE mp.match_id = $1
	`, matchID)
	if err != nil {
		return nil, fmt.Errorf("query participants: %w", err)
	}
	defer rows.Close()

	var participants []participant
	for rows.Next() {
		var p participant
		if err := rows.Scan(&p.BotID, &p.PlayerSlot, &p.DebugPublic, &p.EncryptedSecret); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Determine which slots belong to the authenticated bot owner (if any).
	ownerSlots := make(map[int]bool)
	if apiKey := strings.TrimPrefix(authHeader, "Bearer "); apiKey != "" && authHeader != apiKey {
		for _, p := range participants {
			secret := p.EncryptedSecret
			if s.cfg.EncryptionKey != "" {
				if decrypted, err := decryptSecret(p.EncryptedSecret, s.cfg.EncryptionKey); err == nil {
					secret = decrypted
				}
			}
			if secret == apiKey {
				ownerSlots[p.PlayerSlot] = true
			}
		}
	}

	// Build slot → debug_public map; skip filtering for owner slots.
	slotPublic := make(map[string]bool)
	allPublic := true
	for _, p := range participants {
		pub := p.DebugPublic || ownerSlots[p.PlayerSlot]
		slotPublic[strconv.Itoa(p.PlayerSlot)] = pub
		if !pub {
			allPublic = false
		}
	}

	if allPublic || len(participants) == 0 {
		return decompressed, nil
	}

	// Parse and filter the replay JSON.
	var replay map[string]json.RawMessage
	if err := json.Unmarshal(decompressed, &replay); err != nil {
		return nil, fmt.Errorf("parse replay: %w", err)
	}

	turnsRaw, ok := replay["turns"]
	if !ok {
		return decompressed, nil
	}

	var turns []json.RawMessage
	if err := json.Unmarshal(turnsRaw, &turns); err != nil {
		return nil, fmt.Errorf("parse turns: %w", err)
	}

	for i, turnRaw := range turns {
		var turn map[string]json.RawMessage
		if err := json.Unmarshal(turnRaw, &turn); err != nil {
			continue
		}
		debugRaw, hasDebug := turn["debug"]
		if !hasDebug {
			continue
		}
		var debugMap map[string]json.RawMessage
		if err := json.Unmarshal(debugRaw, &debugMap); err != nil {
			continue
		}
		changed := false
		for slot, pub := range slotPublic {
			if !pub {
				if _, exists := debugMap[slot]; exists {
					delete(debugMap, slot)
					changed = true
				}
			}
		}
		if !changed {
			continue
		}
		if len(debugMap) == 0 {
			delete(turn, "debug")
		} else {
			if b, err := json.Marshal(debugMap); err == nil {
				turn["debug"] = b
			}
		}
		if b, err := json.Marshal(turn); err == nil {
			turns[i] = b
		}
	}

	if b, err := json.Marshal(turns); err == nil {
		replay["turns"] = b
	}
	return json.Marshal(replay)
}

// fetchReplayFromR2 attempts to fetch a replay from R2 warm cache
func (s *Server) fetchReplayFromR2(ctx context.Context, matchID string) ([]byte, error) {
	// R2 endpoint and credentials would be configured via environment variables
	r2Endpoint := "https://r2.aicodebattle.com" // Default R2 endpoint
	if env := getEnv("ACB_R2_ENDPOINT", ""); env != "" {
		r2Endpoint = env
	}

	url := fmt.Sprintf("%s/replays/%s.json.gz", r2Endpoint, matchID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("R2 returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// fetchReplayFromB2 attempts to fetch a replay from B2 cold archive
func (s *Server) fetchReplayFromB2(ctx context.Context, matchID string) ([]byte, error) {
	// B2 endpoint and credentials would be configured via environment variables
	b2Endpoint := "https://b2.aicodebattle.com" // Default B2 endpoint
	if env := getEnv("ACB_B2_ENDPOINT", ""); env != "" {
		b2Endpoint = env
	}

	url := fmt.Sprintf("%s/replays/%s.json.gz", b2Endpoint, matchID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("B2 returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// handleGetBot handles GET /api/bot/{id}
// Returns bot profile JSON including rating, record, and metadata.
func (s *Server) handleGetBot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract bot ID from path: /api/bot/{id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/bot/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid bot ID")
		return
	}
	botID := pathParts[0]

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get bot details
	var bot struct {
		BotID      string  `json:"bot_id"`
		Name       string  `json:"name"`
		Owner      string  `json:"owner"`
		Status     string  `json:"status"`
		RatingMu   float64 `json:"rating_mu"`
		RatingPhi  float64 `json:"rating_phi"`
		Evolved    bool    `json:"evolved"`
		Island     *string `json:"island,omitempty"`
		Generation *int    `json:"generation,omitempty"`
		ParentIDs   *string `json:"parent_ids,omitempty"`
		DebugPublic bool    `json:"debug_public"`
		CreatedAt   string  `json:"created_at"`
		LastActive *string `json:"last_active,omitempty"`
	}

	err := s.db.QueryRowContext(ctx, `
		SELECT bot_id, name, owner, status, rating_mu, rating_phi,
		       evolved, island, generation, parent_ids, debug_public,
		       to_char(created_at, 'YYYY-MM-DD\"T\"HH24:MI:SSZ') as created_at,
		       to_char(last_active, 'YYYY-MM-DD\"T\"HH24:MI:SSZ') as last_active
		FROM bots WHERE bot_id = $1
	`, botID).Scan(
		&bot.BotID, &bot.Name, &bot.Owner, &bot.Status,
		&bot.RatingMu, &bot.RatingPhi, &bot.Evolved,
		&bot.Island, &bot.Generation, &bot.ParentIDs, &bot.DebugPublic,
		&bot.CreatedAt, &bot.LastActive,
	)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "bot not found")
		return
	} else if err != nil {
		log.Printf("database error getting bot: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Calculate win/loss record
	var wins, losses int
	err = s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE mp.bot_id = $1 AND m.winner = (
				SELECT player_slot FROM match_participants WHERE match_id = m.match_id AND bot_id = $1
			)) as wins,
			COUNT(*) FILTER (WHERE mp.bot_id = $1 AND m.winner IS NOT NULL AND m.winner != (
				SELECT player_slot FROM match_participants WHERE match_id = m.match_id AND bot_id = $1
			)) as losses
		FROM match_participants mp
		JOIN matches m ON mp.match_id = m.match_id
		WHERE mp.bot_id = $1 AND m.status = 'completed'
	`, botID).Scan(&wins, &losses)
	if err != nil {
		log.Printf("error getting bot record: %v", err)
		// Continue without record data
	}

	// Build response
	response := map[string]interface{}{
		"bot_id":       bot.BotID,
		"name":         bot.Name,
		"owner":        bot.Owner,
		"status":       bot.Status,
		"rating":       bot.RatingMu - 2*bot.RatingPhi, // Conservative rating estimate
		"rating_mu":    bot.RatingMu,
		"rating_phi":   bot.RatingPhi,
		"evolved":      bot.Evolved,
		"island":       bot.Island,
		"generation":   bot.Generation,
		"parent_ids":   bot.ParentIDs,
		"debug_public": bot.DebugPublic,
		"created_at":   bot.CreatedAt,
		"last_active":  bot.LastActive,
		"record": map[string]int{
			"wins":   wins,
			"losses": losses,
		},
	}

	writeJSON(w, http.StatusOK, response)
}

// handleBotPatch handles PATCH /api/bot/{id}
// Allows owners to toggle debug_public using their shared_secret for auth.
func (s *Server) handleBotPatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract bot ID from path: /api/bot/{id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/bot/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid bot ID")
		return
	}
	botID := pathParts[0]

	var req struct {
		DebugPublic *bool   `json:"debug_public"`
		APISecret   string  `json:"api_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DebugPublic == nil {
		writeError(w, http.StatusBadRequest, "debug_public field is required")
		return
	}
	if req.APISecret == "" {
		writeError(w, http.StatusUnauthorized, "api_secret is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Verify the secret matches
	var encryptedSecret string
	err := s.db.QueryRowContext(ctx, `SELECT shared_secret FROM bots WHERE bot_id = $1`, botID).Scan(&encryptedSecret)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "bot not found")
		return
	} else if err != nil {
		log.Printf("database error getting bot secret: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Decrypt stored secret for comparison
	var storedSecret string
	if s.cfg.EncryptionKey != "" {
		storedSecret, err = decryptSecret(encryptedSecret, s.cfg.EncryptionKey)
		if err != nil {
			storedSecret = encryptedSecret
		}
	} else {
		storedSecret = encryptedSecret
	}

	if storedSecret != req.APISecret {
		writeError(w, http.StatusUnauthorized, "invalid api_secret")
		return
	}

	// Update debug_public
	_, err = s.db.ExecContext(ctx, `UPDATE bots SET debug_public = $1 WHERE bot_id = $2`, *req.DebugPublic, botID)
	if err != nil {
		log.Printf("failed to update debug_public for bot %s: %v", botID, err)
		writeError(w, http.StatusInternalServerError, "failed to update")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bot_id":       botID,
		"debug_public": *req.DebugPublic,
	})
}

// handleRotateKey handles POST /api/rotate-key (§8.5)
// Authenticates with the current shared_secret, generates a new 256-bit secret,
// encrypts it with AES-256-GCM, updates the bots table, and returns the new secret.
// Optionally retires the bot when called with retire: true.
func (s *Server) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BotID  string `json:"bot_id"`
		Secret string `json:"shared_secret"`
		Retire bool   `json:"retire"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.BotID == "" || req.Secret == "" {
		writeError(w, http.StatusBadRequest, "bot_id and shared_secret are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Look up the bot and verify the current secret
	var encryptedSecret string
	var currentStatus string
	err := s.db.QueryRowContext(ctx,
		`SELECT shared_secret, status FROM bots WHERE bot_id = $1`, req.BotID,
	).Scan(&encryptedSecret, &currentStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "bot not found")
		return
	} else if err != nil {
		log.Printf("database error getting bot %s: %v", req.BotID, err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Decrypt stored secret for comparison
	var storedSecret string
	if s.cfg.EncryptionKey != "" {
		storedSecret, err = decryptSecret(encryptedSecret, s.cfg.EncryptionKey)
		if err != nil {
			storedSecret = encryptedSecret
		}
	} else {
		storedSecret = encryptedSecret
	}

	if storedSecret != req.Secret {
		writeError(w, http.StatusUnauthorized, "invalid shared_secret")
		return
	}

	// A retired bot cannot rotate its key again
	if currentStatus == "retired" {
		writeError(w, http.StatusConflict, "bot is retired")
		return
	}

	// Generate new 256-bit secret
	newSecret, err := generateSecret()
	if err != nil {
		log.Printf("failed to generate new secret for bot %s: %v", req.BotID, err)
		writeError(w, http.StatusInternalServerError, "failed to generate secret")
		return
	}

	// Encrypt the new secret
	var encryptedNew string
	if s.cfg.EncryptionKey != "" {
		encryptedNew, err = encryptSecret(newSecret, s.cfg.EncryptionKey)
		if err != nil {
			log.Printf("failed to encrypt new secret for bot %s: %v", req.BotID, err)
			writeError(w, http.StatusInternalServerError, "failed to encrypt secret")
			return
		}
	} else {
		encryptedNew = newSecret
	}

	// Update the bot: always rotate the secret; optionally set status to retired
	if req.Retire {
		_, err = s.db.ExecContext(ctx,
			`UPDATE bots SET shared_secret = $1, status = 'retired', last_active = NOW() WHERE bot_id = $2`,
			encryptedNew, req.BotID)
	} else {
		_, err = s.db.ExecContext(ctx,
			`UPDATE bots SET shared_secret = $1, last_active = NOW() WHERE bot_id = $2`,
			encryptedNew, req.BotID)
	}
	if err != nil {
		log.Printf("failed to update bot %s: %v", req.BotID, err)
		writeError(w, http.StatusInternalServerError, "failed to update bot")
		return
	}

	log.Printf("rotated key for bot %s (retire=%v)", req.BotID, req.Retire)

	resp := map[string]interface{}{
		"bot_id":        req.BotID,
		"shared_secret": newSecret,
	}
	if req.Retire {
		resp["status"] = "retired"
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleListBots handles GET /api/bots
// Returns leaderboard snapshot of all active bots.
func (s *Server) handleListBots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters for pagination
	limit := 100
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	// Query active bots ordered by rating
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.bot_id, b.name, b.owner, b.rating_mu, b.rating_phi,
		       b.evolved, b.island, b.generation,
		       to_char(b.created_at, 'YYYY-MM-DD\"T\"HH24:MI:SSZ') as created_at,
		       COALESCE(wins.wins, 0) as wins, COALESCE(losses.losses, 0) as losses
		FROM bots b
		LEFT JOIN (
			SELECT mp.bot_id, COUNT(*) FILTER (WHERE m.winner = mp.player_slot) as wins
			FROM match_participants mp
			JOIN matches m ON mp.match_id = m.match_id
			WHERE m.status = 'completed'
			GROUP BY mp.bot_id
		) wins ON b.bot_id = wins.bot_id
		LEFT JOIN (
			SELECT mp.bot_id, COUNT(*) FILTER (WHERE m.winner IS NOT NULL AND m.winner != mp.player_slot) as losses
			FROM match_participants mp
			JOIN matches m ON mp.match_id = m.match_id
			WHERE m.status = 'completed'
			GROUP BY mp.bot_id
		) losses ON b.bot_id = losses.bot_id
		WHERE b.status = 'active'
		ORDER BY (b.rating_mu - 2*b.rating_phi) DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		log.Printf("database error listing bots: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	bots := make([]map[string]interface{}, 0)
	for rows.Next() {
		var bot struct {
			BotID      string  `json:"bot_id"`
			Name       string  `json:"name"`
			Owner      string  `json:"owner"`
			RatingMu   float64 `json:"rating_mu"`
			RatingPhi  float64 `json:"rating_phi"`
			Evolved    bool    `json:"evolved"`
			Island     *string `json:"island,omitempty"`
			Generation *int    `json:"generation,omitempty"`
			CreatedAt  string  `json:"created_at"`
			Wins       int     `json:"wins"`
			Losses     int     `json:"losses"`
		}
		err := rows.Scan(
			&bot.BotID, &bot.Name, &bot.Owner, &bot.RatingMu, &bot.RatingPhi,
			&bot.Evolved, &bot.Island, &bot.Generation, &bot.CreatedAt,
			&bot.Wins, &bot.Losses,
		)
		if err != nil {
			log.Printf("error scanning bot: %v", err)
			continue
		}

		bots = append(bots, map[string]interface{}{
			"bot_id":     bot.BotID,
			"name":       bot.Name,
			"owner":      bot.Owner,
			"rating":     bot.RatingMu - 2*bot.RatingPhi,
			"rating_mu":  bot.RatingMu,
			"rating_phi": bot.RatingPhi,
			"evolved":    bot.Evolved,
			"island":     bot.Island,
			"generation": bot.Generation,
			"created_at": bot.CreatedAt,
			"record": map[string]int{
				"wins":   bot.Wins,
				"losses": bot.Losses,
			},
		})
	}

	if rows.Err() != nil {
		log.Printf("error iterating bots: %v", rows.Err())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bots":   bots,
		"limit":  limit,
		"offset": offset,
		"count":  len(bots),
	})
}

// handlePredict handles POST /api/predict
// Accepts {match_id, bot_id, confidence, predictor_id} and writes to predictions table.
// Rejects if the match has already started (status != 'pending').
func (s *Server) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		MatchID    string `json:"match_id"`
		BotID      string `json:"bot_id"`
		Predictor  string `json:"predictor_id"`
		Confidence *int   `json:"confidence"` // optional 1-100
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MatchID == "" || req.BotID == "" || req.Predictor == "" {
		writeError(w, http.StatusBadRequest, "match_id, bot_id, and predictor_id are required")
		return
	}

	if req.Confidence != nil && (*req.Confidence < 1 || *req.Confidence > 100) {
		writeError(w, http.StatusBadRequest, "confidence must be between 1 and 100")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Verify match exists and hasn't started
	var matchStatus string
	err := s.db.QueryRowContext(ctx, `
		SELECT status FROM matches WHERE match_id = $1
	`, req.MatchID).Scan(&matchStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "match not found")
		return
	} else if err != nil {
		log.Printf("database error checking match: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if matchStatus != "pending" {
		writeError(w, http.StatusConflict, "match has already started")
		return
	}

	// Verify bot is a participant in this match
	var participantExists bool
	err = s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM match_participants
			WHERE match_id = $1 AND bot_id = $2
		)
	`, req.MatchID, req.BotID).Scan(&participantExists)
	if err != nil {
		log.Printf("database error checking participant: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !participantExists {
		writeError(w, http.StatusBadRequest, "bot is not a participant in this match")
		return
	}

	// Insert prediction (UNIQUE constraint handles duplicates)
	var predictionID int64
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO predictions (match_id, predictor_id, predicted_bot, confidence)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (match_id, predictor_id) DO UPDATE SET predicted_bot = $3, confidence = $4
		RETURNING id
	`, req.MatchID, req.Predictor, req.BotID, req.Confidence).Scan(&predictionID)
	if err != nil {
		log.Printf("failed to insert prediction: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to submit prediction")
		return
	}

	resp := map[string]interface{}{
		"id":         predictionID,
		"match_id":   req.MatchID,
		"predicted":  req.BotID,
		"predictor":  req.Predictor,
	}
	if req.Confidence != nil {
		resp["confidence"] = *req.Confidence
	}

	writeJSON(w, http.StatusCreated, resp)
}

// handleOpenPredictions handles GET /api/predictions/open
// Returns pending matches that are open for predictions, along with
// any predictions the requesting predictor has made.
func (s *Server) handleOpenPredictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	predictorID := r.URL.Query().Get("predictor_id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get pending matches with their participants
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.match_id, m.created_at,
		       COALESCE(json_agg(json_build_object('bot_id', mp.bot_id, 'name', b.name, 'rating', b.rating_mu - 2*b.rating_phi)) FILTER (WHERE mp.bot_id IS NOT NULL), '[]')
		FROM matches m
		JOIN match_participants mp ON m.match_id = mp.match_id
		JOIN bots b ON mp.bot_id = b.bot_id
		WHERE m.status = 'pending'
		GROUP BY m.match_id, m.created_at
		ORDER BY m.created_at ASC
		LIMIT 20
	`)
	if err != nil {
		log.Printf("database error fetching open matches: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type MatchPrediction struct {
		MatchID     string          `json:"match_id"`
		CreatedAt   string          `json:"created_at"`
		Participants []map[string]interface{} `json:"participants"`
		YourPick    *string         `json:"your_pick,omitempty"`
	}

	var matches []MatchPrediction
	for rows.Next() {
		var m MatchPrediction
		var participantsJSON string
		if err := rows.Scan(&m.MatchID, &m.CreatedAt, &participantsJSON); err != nil {
			log.Printf("error scanning match: %v", err)
			continue
		}
		json.Unmarshal([]byte(participantsJSON), &m.Participants)

		// If predictor_id given, look up their existing prediction
		if predictorID != "" {
			var predictedBot string
			err := s.db.QueryRowContext(ctx, `
				SELECT predicted_bot FROM predictions
				WHERE match_id = $1 AND predictor_id = $2
			`, m.MatchID, predictorID).Scan(&predictedBot)
			if err == nil {
				m.YourPick = &predictedBot
			}
		}

		matches = append(matches, m)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matches": matches,
	})
}

// resolvePredictions marks open predictions as correct/incorrect and updates predictor_stats.
func (s *Server) resolvePredictions(ctx context.Context, tx *sql.Tx, matchID string, winnerBotID string) error {
	var rows *sql.Rows
	var err error

	if winnerBotID == "" {
		rows, err = tx.QueryContext(ctx, `
			UPDATE predictions
			SET correct = false, resolved_at = NOW()
			WHERE match_id = $1 AND correct IS NULL
			RETURNING predictor_id, correct
		`, matchID)
	} else {
		rows, err = tx.QueryContext(ctx, `
			UPDATE predictions
			SET correct = (predicted_bot = $1), resolved_at = NOW()
			WHERE match_id = $2 AND correct IS NULL
			RETURNING predictor_id, correct
		`, winnerBotID, matchID)
	}
	if err != nil {
		return fmt.Errorf("failed to resolve predictions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var predictorID string
		var correct bool
		if err := rows.Scan(&predictorID, &correct); err != nil {
			return fmt.Errorf("failed to scan resolved prediction: %w", err)
		}
		if err := s.upsertPredictorStats(ctx, tx, predictorID, correct); err != nil {
			return fmt.Errorf("failed to update predictor_stats for %s: %w", predictorID, err)
		}
	}
	return nil
}

// upsertPredictorStats updates the predictor_stats row for a single resolution.
func (s *Server) upsertPredictorStats(ctx context.Context, tx *sql.Tx, predictorID string, correct bool) error {
	if correct {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO predictor_stats (predictor_id, correct, streak, best_streak, updated_at)
			VALUES ($1, 1, 1, 1, NOW())
			ON CONFLICT (predictor_id) DO UPDATE SET
				correct = predictor_stats.correct + 1,
				streak = predictor_stats.streak + 1,
				best_streak = GREATEST(predictor_stats.best_streak, predictor_stats.streak + 1),
				updated_at = NOW()
		`, predictorID)
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO predictor_stats (predictor_id, incorrect, streak, best_streak, updated_at)
		VALUES ($1, 1, 0, 0, NOW())
		ON CONFLICT (predictor_id) DO UPDATE SET
			incorrect = predictor_stats.incorrect + 1,
			streak = 0,
			updated_at = NOW()
	`, predictorID)
	return err
}

// handlePredictionHistory handles GET /api/predictions/history
// Returns resolved predictions for a predictor, used for polling resolution status.
func (s *Server) handlePredictionHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	predictorID := r.URL.Query().Get("predictor_id")
	if predictorID == "" {
		writeError(w, http.StatusBadRequest, "predictor_id is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.match_id, p.predicted_bot,
		       COALESCE(wb.name, p.predicted_bot) AS predicted_name,
		       p.correct, p.confidence, p.created_at, p.resolved_at,
		       m.status AS match_status, m.winner,
		       COALESCE(CASE WHEN m.winner IS NOT NULL THEN
		           (SELECT b.name FROM match_participants mp2 JOIN bots b ON mp2.bot_id = b.bot_id
		            WHERE mp2.match_id = m.match_id AND mp2.player_slot = m.winner)
		       END, '') AS winner_name
		FROM predictions p
		JOIN matches m ON p.match_id = m.match_id
		LEFT JOIN bots wb ON p.predicted_bot = wb.bot_id
		WHERE p.predictor_id = $1
		ORDER BY COALESCE(p.resolved_at, p.created_at) DESC
		LIMIT $2
	`, predictorID, limit)
	if err != nil {
		log.Printf("database error fetching prediction history: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type PredictionEntry struct {
		ID            int64   `json:"id"`
		MatchID       string  `json:"match_id"`
		PredictedBot  string  `json:"predicted_bot"`
		PredictedName string  `json:"predicted_name"`
		Correct       *bool   `json:"correct"`
		Confidence    *int    `json:"confidence,omitempty"`
		CreatedAt     string  `json:"created_at"`
		ResolvedAt    *string `json:"resolved_at,omitempty"`
		MatchStatus   string  `json:"match_status"`
		WinnerName    string  `json:"winner_name,omitempty"`
	}

	var predictions []PredictionEntry
	for rows.Next() {
		var p PredictionEntry
		var createdAt time.Time
		var resolvedAt sql.NullTime
		var winnerName sql.NullString
		var winnerSlot sql.NullInt64

		if err := rows.Scan(&p.ID, &p.MatchID, &p.PredictedBot, &p.PredictedName,
			&p.Correct, &p.Confidence, &createdAt, &resolvedAt,
			&p.MatchStatus, &winnerSlot, &winnerName); err != nil {
			log.Printf("error scanning prediction: %v", err)
			continue
		}

		p.CreatedAt = createdAt.Format(time.RFC3339)
		if resolvedAt.Valid {
			s := resolvedAt.Time.Format(time.RFC3339)
			p.ResolvedAt = &s
		}
		if winnerName.Valid {
			p.WinnerName = winnerName.String
		}
		predictions = append(predictions, p)
	}

	if predictions == nil {
		predictions = []PredictionEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"predictions": predictions,
	})
}

// handleCreateFeedback handles POST /api/feedback
// Accepts community replay feedback per plan §13.6.
// Stores in replay_feedback table with type enum: insight, mistake, idea, highlight.
func (s *Server) handleCreateFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		MatchID string `json:"match_id"`
		Turn    int    `json:"turn"`
		Type    string `json:"type"` // insight, mistake, idea, highlight
		Body    string `json:"body"`
		Author  string `json:"author"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MatchID == "" || req.Type == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "match_id, type, and body are required")
		return
	}

	validTypes := map[string]bool{"insight": true, "mistake": true, "idea": true, "highlight": true}
	if !validTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "type must be one of: insight, mistake, idea, highlight")
		return
	}

	if req.Author == "" {
		req.Author = "Anonymous"
	}

	feedbackID, err := generateID("fb_", 6)
	if err != nil {
		log.Printf("failed to generate feedback ID: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO replay_feedback (feedback_id, match_id, turn, type, body, author, upvotes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0, NOW())
	`, feedbackID, req.MatchID, req.Turn, req.Type, req.Body, req.Author)

	if err != nil {
		log.Printf("[FEEDBACK] db insert failed: match=%s turn=%d type=%s: %v", req.MatchID, req.Turn, req.Type, err)
		writeError(w, http.StatusInternalServerError, "failed to store feedback")
		return
	}

	log.Printf("[FEEDBACK] stored: id=%s match=%s turn=%d type=%s", feedbackID, req.MatchID, req.Turn, req.Type)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "recorded", "feedback_id": feedbackID})
}

// handleFeedbackUpvote handles POST /api/feedback/{feedback_id}/upvote
// One upvote per visitor (deduped by voter_id from localStorage UUID).
func (s *Server) handleFeedbackUpvote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract feedback_id from path: /api/feedback/{feedback_id}/upvote
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/feedback/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] != "upvote" {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	feedbackID := pathParts[0]

	var req struct {
		VoterID string `json:"voter_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.VoterID == "" {
		writeError(w, http.StatusBadRequest, "voter_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Insert upvote (UNIQUE constraint on feedback_id+voter_id prevents duplicates)
	var inserted bool
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO feedback_upvotes (feedback_id, voter_id)
		VALUES ($1, $2)
		ON CONFLICT (feedback_id, voter_id) DO NOTHING
		RETURNING true
	`, feedbackID, req.VoterID).Scan(&inserted)

	if err == sql.ErrNoRows {
		// Already upvoted
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_upvoted"})
		return
	} else if err != nil {
		log.Printf("[FEEDBACK] upvote failed: feedback=%s voter=%s: %v", feedbackID, req.VoterID, err)
		writeError(w, http.StatusInternalServerError, "failed to upvote")
		return
	}

	// Increment upvote counter on the feedback row
	_, err = s.db.ExecContext(ctx, `
		UPDATE replay_feedback SET upvotes = upvotes + 1 WHERE feedback_id = $1
	`, feedbackID)
	if err != nil {
		log.Printf("[FEEDBACK] upvote counter increment failed: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "upvoted"})
}

// handleGetFeedback handles GET /api/feedback/{match_id}
// Returns all feedback for a match sorted by turn then upvotes descending.
func (s *Server) handleGetFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract match_id from path: /api/feedback/{match_id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/feedback/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid match ID")
		return
	}
	matchID := pathParts[0]

	// Don't treat upvote or other sub-paths as match_id lookups
	if matchID == "upvote" {
		writeError(w, http.StatusBadRequest, "invalid match ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT feedback_id, match_id, turn, type, body, author, upvotes,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ') as created_at
		FROM replay_feedback
		WHERE match_id = $1
		ORDER BY turn ASC, upvotes DESC, created_at ASC
	`, matchID)
	if err != nil {
		log.Printf("[FEEDBACK] query failed for match %s: %v", matchID, err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type FeedbackEntry struct {
		FeedbackID string `json:"feedback_id"`
		MatchID    string `json:"match_id"`
		Turn       int    `json:"turn"`
		Type       string `json:"type"`
		Body       string `json:"body"`
		Author     string `json:"author"`
		Upvotes    int    `json:"upvotes"`
		CreatedAt  string `json:"created_at"`
	}

	feedback := make([]FeedbackEntry, 0)
	for rows.Next() {
		var f FeedbackEntry
		if err := rows.Scan(&f.FeedbackID, &f.MatchID, &f.Turn, &f.Type, &f.Body, &f.Author, &f.Upvotes, &f.CreatedAt); err != nil {
			log.Printf("[FEEDBACK] scan error: %v", err)
			continue
		}
		feedback = append(feedback, f)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"match_id": matchID,
		"feedback": feedback,
	})
}

// authenticateWorker checks if the request has a valid worker API key
func (s *Server) authenticateWorker(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// Expect "Bearer {api_key}"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false
	}

	return parts[1] == s.cfg.WorkerAPIKey
}

// handleMapVote handles POST /api/vote/map (§14.6)
// Accepts {map_id, voter_id, vote: +1|-1}, enforces UNIQUE(map_id, voter_id),
// and returns the current net vote count for the map.
func (s *Server) handleMapVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		MapID   string `json:"map_id"`
		VoterID string `json:"voter_id"`
		Vote    int    `json:"vote"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MapID == "" || req.VoterID == "" {
		writeError(w, http.StatusBadRequest, "map_id and voter_id are required")
		return
	}
	if req.Vote != 1 && req.Vote != -1 {
		writeError(w, http.StatusBadRequest, "vote must be +1 or -1")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if s.db == nil {
		writeError(w, http.StatusInternalServerError, "database not configured")
		return
	}

	// Verify the map exists
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM maps WHERE map_id = $1)`, req.MapID).Scan(&exists)
	if err != nil {
		log.Printf("[MAPVOTE] db error checking map %s: %v", req.MapID, err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "map not found")
		return
	}

	// Insert or update the vote (UNIQUE constraint on map_id + voter_id)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO map_votes (map_id, voter_id, vote)
		VALUES ($1, $2, $3)
		ON CONFLICT (map_id, voter_id) DO UPDATE SET vote = $3
	`, req.MapID, req.VoterID, req.Vote)
	if err != nil {
		log.Printf("[MAPVOTE] insert failed: map=%s voter=%s: %v", req.MapID, req.VoterID, err)
		writeError(w, http.StatusInternalServerError, "failed to record vote")
		return
	}

	// Get net vote count
	var netVotes int
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(vote), 0) FROM map_votes WHERE map_id = $1
	`, req.MapID).Scan(&netVotes)
	if err != nil {
		log.Printf("[MAPVOTE] sum failed for map %s: %v", req.MapID, err)
		netVotes = 0
	}

	log.Printf("[MAPVOTE] recorded: map=%s voter=%s vote=%d net=%d", req.MapID, req.VoterID, req.Vote, netVotes)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"map_id":    req.MapID,
		"vote":      req.Vote,
		"net_votes": netVotes,
	})
}

// handleGetMapVotes handles GET /api/vote/map/{map_id}
// Returns the net vote count and, if voter_id is provided, the caller's existing vote.
func (s *Server) handleGetMapVotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract map_id from path: /api/vote/map/{map_id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/vote/map/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid map ID")
		return
	}
	mapID := pathParts[0]

	voterID := r.URL.Query().Get("voter_id")

	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"map_id":    mapID,
			"net_votes": 0,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var netVotes int
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(vote), 0) FROM map_votes WHERE map_id = $1
	`, mapID).Scan(&netVotes)
	if err != nil {
		log.Printf("[MAPVOTE] sum query failed for map %s: %v", mapID, err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	resp := map[string]interface{}{
		"map_id":    mapID,
		"net_votes": netVotes,
	}

	if voterID != "" {
		var myVote int
		err := s.db.QueryRowContext(ctx, `
			SELECT vote FROM map_votes WHERE map_id = $1 AND voter_id = $2
		`, mapID, voterID).Scan(&myVote)
		if err == nil {
			resp["my_vote"] = myVote
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
