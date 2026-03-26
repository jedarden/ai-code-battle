package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func (s *Server) StartTickers(ctx context.Context) {
	go s.runTicker(ctx, "matchmaker", time.Duration(s.cfg.MatchmakerSecs)*time.Second, s.tickMatchmaker)
	go s.runTicker(ctx, "health-checker", time.Duration(s.cfg.HealthCheckSecs)*time.Second, s.tickHealthChecker)
	go s.runTicker(ctx, "stale-reaper", time.Duration(s.cfg.ReaperSecs)*time.Second, s.tickStaleReaper)
}

func (s *Server) runTicker(ctx context.Context, name string, interval time.Duration, fn func(context.Context)) {
	log.Printf("starting ticker: %s (every %s)", name, interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopping ticker: %s", name)
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}

// tickMatchmaker creates matches between active bots and enqueues jobs.
func (s *Server) tickMatchmaker(ctx context.Context) {
	// Get all active bots
	rows, err := s.db.QueryContext(ctx,
		`SELECT bot_id, endpoint_url, shared_secret, rating_mu, rating_phi
		 FROM bots WHERE status = 'active' ORDER BY rating_mu DESC`)
	if err != nil {
		log.Printf("matchmaker: query error: %v", err)
		return
	}

	type botInfo struct {
		ID        string
		Endpoint  string
		Secret    string
		Mu, Phi   float64
	}
	var bots []botInfo
	for rows.Next() {
		var b botInfo
		if err := rows.Scan(&b.ID, &b.Endpoint, &b.Secret, &b.Mu, &b.Phi); err != nil {
			rows.Close()
			log.Printf("matchmaker: scan error: %v", err)
			return
		}
		bots = append(bots, b)
	}
	rows.Close()

	if len(bots) < 2 {
		return
	}

	// Create one match per tick: pick two bots at random (with rating-aware weighting later)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	i := rng.Intn(len(bots))
	j := rng.Intn(len(bots) - 1)
	if j >= i {
		j++
	}

	botA := bots[i]
	botB := bots[j]

	matchID, err := generateID("m_", 8)
	if err != nil {
		log.Printf("matchmaker: generate match ID error: %v", err)
		return
	}

	jobID, err := generateID("j_", 8)
	if err != nil {
		log.Printf("matchmaker: generate job ID error: %v", err)
		return
	}

	mapSeed := rng.Int63()
	mapID := fmt.Sprintf("map_%d", mapSeed%100000)

	// Build job config
	type botConfig struct {
		BotID    string `json:"bot_id"`
		Endpoint string `json:"endpoint"`
		Secret   string `json:"secret"`
		Slot     int    `json:"slot"`
	}
	type jobConfig struct {
		MatchID  string      `json:"match_id"`
		MapSeed  int64       `json:"map_seed"`
		MaxTurns int         `json:"max_turns"`
		Rows     int         `json:"rows"`
		Cols     int         `json:"cols"`
		Bots     []botConfig `json:"bots"`
	}

	// Decrypt secrets for the worker
	secretA := botA.Secret
	secretB := botB.Secret
	if s.cfg.EncryptionKey != "" {
		if dec, err := decryptSecret(botA.Secret, s.cfg.EncryptionKey); err == nil {
			secretA = dec
		}
		if dec, err := decryptSecret(botB.Secret, s.cfg.EncryptionKey); err == nil {
			secretB = dec
		}
	}

	config := jobConfig{
		MatchID:  matchID,
		MapSeed:  mapSeed,
		MaxTurns: 500,
		Rows:     60,
		Cols:     60,
		Bots: []botConfig{
			{BotID: botA.ID, Endpoint: botA.Endpoint, Secret: secretA, Slot: 0},
			{BotID: botB.ID, Endpoint: botB.Endpoint, Secret: secretB, Slot: 1},
		},
	}
	configJSON, _ := json.Marshal(config)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("matchmaker: tx error: %v", err)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO matches (match_id, map_id, map_seed, status) VALUES ($1, $2, $3, 'pending')`,
		matchID, mapID, mapSeed)
	if err != nil {
		log.Printf("matchmaker: insert match error: %v", err)
		return
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO match_participants (match_id, bot_id, player_slot) VALUES ($1, $2, 0), ($1, $3, 1)`,
		matchID, botA.ID, botB.ID)
	if err != nil {
		log.Printf("matchmaker: insert participants error: %v", err)
		return
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO jobs (job_id, match_id, status, config_json) VALUES ($1, $2, 'pending', $3)`,
		jobID, matchID, configJSON)
	if err != nil {
		log.Printf("matchmaker: insert job error: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("matchmaker: commit error: %v", err)
		return
	}

	// Enqueue in Valkey
	if err := s.rdb.LPush(ctx, valkeyJobQueue, jobID).Err(); err != nil {
		log.Printf("matchmaker: valkey push error: %v", err)
		return
	}

	log.Printf("matchmaker: created match %s (%s vs %s), job %s", matchID, botA.ID, botB.ID, jobID)
}

// tickHealthChecker pings each active bot's /health endpoint.
func (s *Server) tickHealthChecker(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT bot_id, endpoint_url, status, consec_fails FROM bots WHERE status IN ('active', 'inactive')`)
	if err != nil {
		log.Printf("health-checker: query error: %v", err)
		return
	}

	type botRow struct {
		ID          string
		Endpoint    string
		Status      string
		ConsecFails int
	}
	var bots []botRow
	for rows.Next() {
		var b botRow
		if err := rows.Scan(&b.ID, &b.Endpoint, &b.Status, &b.ConsecFails); err != nil {
			rows.Close()
			log.Printf("health-checker: scan error: %v", err)
			return
		}
		bots = append(bots, b)
	}
	rows.Close()

	client := &http.Client{Timeout: time.Duration(s.cfg.BotTimeoutSecs) * time.Second}

	for _, bot := range bots {
		healthy := false
		resp, err := client.Get(bot.Endpoint + "/health")
		if err == nil {
			healthy = resp.StatusCode == http.StatusOK
			resp.Body.Close()
		}

		if healthy {
			if bot.Status == "inactive" || bot.ConsecFails > 0 {
				s.db.ExecContext(ctx,
					`UPDATE bots SET status = 'active', consec_fails = 0, last_active = NOW()
					 WHERE bot_id = $1`, bot.ID)
				log.Printf("health-checker: %s recovered → active", bot.ID)
				if bot.Status == "inactive" {
					s.alerter.BotRecovered(ctx, bot.ID)
				}
			}
		} else {
			newFails := bot.ConsecFails + 1
			newStatus := bot.Status
			if newFails >= s.cfg.MaxConsecFails {
				newStatus = "inactive"
			}
			s.db.ExecContext(ctx,
				`UPDATE bots SET status = $1, consec_fails = $2 WHERE bot_id = $3`,
				newStatus, newFails, bot.ID)
			if newStatus != bot.Status {
				log.Printf("health-checker: %s marked inactive after %d failures", bot.ID, newFails)
				s.alerter.BotMarkedInactive(ctx, bot.ID, newFails)
			}
		}
	}
}

// tickStaleReaper re-enqueues jobs that have been running too long.
func (s *Server) tickStaleReaper(ctx context.Context) {
	threshold := time.Duration(s.cfg.StaleJobMinutes) * time.Minute

	rows, err := s.db.QueryContext(ctx,
		`SELECT job_id FROM jobs
		 WHERE status = 'running' AND claimed_at < $1`,
		time.Now().Add(-threshold))
	if err != nil {
		log.Printf("stale-reaper: query error: %v", err)
		return
	}

	var staleJobs []string
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			rows.Close()
			return
		}
		staleJobs = append(staleJobs, jobID)
	}
	rows.Close()

	for _, jobID := range staleJobs {
		result, err := s.db.ExecContext(ctx,
			`UPDATE jobs SET status = 'pending', worker_id = NULL, claimed_at = NULL
			 WHERE job_id = $1 AND status = 'running'`, jobID)
		if err != nil {
			log.Printf("stale-reaper: update error for %s: %v", jobID, err)
			continue
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			continue // already completed or re-enqueued by another reaper
		}

		if err := s.rdb.LPush(ctx, valkeyJobQueue, jobID).Err(); err != nil {
			log.Printf("stale-reaper: re-enqueue error for %s: %v", jobID, err)
			continue
		}

		log.Printf("stale-reaper: re-enqueued stale job %s", jobID)
	}

	if len(staleJobs) > 0 {
		log.Printf("stale-reaper: processed %d stale jobs", len(staleJobs))
		s.alerter.StaleJobsReaped(ctx, staleJobs)
	}
}

// queryActiveBotCount returns the number of active bots (used by tests).
func (s *Server) queryActiveBotCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bots WHERE status = 'active'`).Scan(&count)
	return count, err
}

// Unused but required to avoid "imported and not used" for sql package
var _ = sql.ErrNoRows
