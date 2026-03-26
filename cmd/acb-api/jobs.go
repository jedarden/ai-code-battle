package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const valkeyJobQueue = "acb:jobs:pending"

type JobClaimRequest struct {
	WorkerID string `json:"worker_id"`
}

type JobClaimResponse struct {
	JobID      string          `json:"job_id"`
	MatchID    string          `json:"match_id"`
	ConfigJSON json.RawMessage `json:"config"`
}

func (s *Server) handleJobClaim(w http.ResponseWriter, r *http.Request) {
	// Authenticate worker
	if !s.authenticateWorker(r) {
		writeError(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	var req JobClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.WorkerID == "" {
		writeError(w, http.StatusBadRequest, "worker_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Blocking pop from Valkey queue (short timeout for HTTP context)
	result, err := s.rdb.BRPop(ctx, 4*time.Second, valkeyJobQueue).Result()
	if err != nil {
		// Timeout or empty queue
		writeJSON(w, http.StatusNoContent, nil)
		return
	}

	jobID := result[1] // BRPop returns [key, value]

	// Fetch job details from PostgreSQL and mark as running
	var resp JobClaimResponse
	var configJSON []byte
	err = s.db.QueryRowContext(r.Context(),
		`UPDATE jobs SET status = 'running', worker_id = $1, claimed_at = NOW()
		 WHERE job_id = $2 AND status = 'pending'
		 RETURNING job_id, match_id, config_json`,
		req.WorkerID, jobID,
	).Scan(&resp.JobID, &resp.MatchID, &configJSON)
	if err != nil {
		// Job was already claimed or doesn't exist; put it back if it was something else
		writeJSON(w, http.StatusNoContent, nil)
		return
	}
	resp.ConfigJSON = configJSON

	writeJSON(w, http.StatusOK, resp)
}

type JobResultRequest struct {
	WorkerID  string          `json:"worker_id"`
	Winner    *int            `json:"winner"`
	Condition string          `json:"condition"`
	TurnCount int             `json:"turn_count"`
	Scores    json.RawMessage `json:"scores"`
}

func (s *Server) handleJobResult(w http.ResponseWriter, r *http.Request) {
	if !s.authenticateWorker(r) {
		writeError(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	jobID := r.PathValue("job_id")

	var req JobResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	ctx := r.Context()

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction error")
		return
	}
	defer tx.Rollback()

	// Get match_id from job
	var matchID string
	err = tx.QueryRowContext(ctx,
		`UPDATE jobs SET status = 'completed', completed_at = NOW()
		 WHERE job_id = $1 AND status = 'running'
		 RETURNING match_id`, jobID,
	).Scan(&matchID)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found or not running")
		return
	}

	// Update match
	_, err = tx.ExecContext(ctx,
		`UPDATE matches SET status = 'completed', winner = $1, condition = $2,
		 turn_count = $3, scores_json = $4, completed_at = NOW()
		 WHERE match_id = $5`,
		req.Winner, req.Condition, req.TurnCount, req.Scores, matchID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update match")
		return
	}

	// Update participant scores
	var scores []int
	if err := json.Unmarshal(req.Scores, &scores); err == nil {
		for slot, score := range scores {
			_, _ = tx.ExecContext(ctx,
				`UPDATE match_participants SET score = $1, status = 'completed'
				 WHERE match_id = $2 AND player_slot = $3`,
				score, matchID, slot,
			)
		}
	}

	// Get participants for rating update
	rows, err := tx.QueryContext(ctx,
		`SELECT mp.bot_id, mp.player_slot, mp.score,
		        b.rating_mu, b.rating_phi, b.rating_sigma
		 FROM match_participants mp
		 JOIN bots b ON b.bot_id = mp.bot_id
		 WHERE mp.match_id = $1
		 ORDER BY mp.player_slot`, matchID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch participants")
		return
	}

	type participant struct {
		botID    string
		slot     int
		score    int
		mu, phi  float64
		sigma    float64
	}
	var participants []participant
	for rows.Next() {
		var p participant
		if err := rows.Scan(&p.botID, &p.slot, &p.score, &p.mu, &p.phi, &p.sigma); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		participants = append(participants, p)
	}
	rows.Close()

	// Update Glicko-2 ratings
	if len(participants) >= 2 {
		ratings := make([]Glicko2Rating, len(participants))
		scores := make([]float64, len(participants))
		for i, p := range participants {
			ratings[i] = Glicko2Rating{Mu: p.mu, Phi: p.phi, Sigma: p.sigma}
			scores[i] = float64(p.score)
		}

		newRatings := updateRatings(ratings, scores)

		for i, p := range participants {
			nr := newRatings[i]
			_, _ = tx.ExecContext(ctx,
				`UPDATE bots SET rating_mu = $1, rating_phi = $2, rating_sigma = $3, last_active = NOW()
				 WHERE bot_id = $4`,
				nr.Mu, nr.Phi, nr.Sigma, p.botID,
			)
			displayRating := nr.Mu - 2*nr.Phi
			_, _ = tx.ExecContext(ctx,
				`INSERT INTO rating_history (bot_id, match_id, rating)
				 VALUES ($1, $2, $3)`,
				p.botID, matchID, displayRating,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "commit error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authenticateWorker(r *http.Request) bool {
	if s.cfg.WorkerAPIKey == "" {
		return true // no auth configured (dev mode)
	}
	key := r.Header.Get("Authorization")
	if key == "" {
		key = r.Header.Get("X-API-Key")
	}
	return key == "Bearer "+s.cfg.WorkerAPIKey || key == s.cfg.WorkerAPIKey
}
