package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// handleSubmitPrediction handles POST /api/predictions
func (s *Server) handleSubmitPrediction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MatchID      string `json:"match_id"`
		PredictorID  string `json:"predictor_id"`
		PredictedBot string `json:"predicted_bot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MatchID == "" || req.PredictorID == "" || req.PredictedBot == "" {
		writeError(w, http.StatusBadRequest, "match_id, predictor_id, and predicted_bot are required")
		return
	}

	ctx := r.Context()

	// Verify match exists and is pending/active
	var matchStatus string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM matches WHERE match_id = $1`, req.MatchID).Scan(&matchStatus)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "match not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if matchStatus == "completed" {
		writeError(w, http.StatusConflict, "match already completed; predictions closed")
		return
	}

	// Upsert prediction (one per predictor per match)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO predictions (match_id, predictor_id, predicted_bot)
		VALUES ($1, $2, $3)
		ON CONFLICT (match_id, predictor_id)
		DO UPDATE SET predicted_bot = EXCLUDED.predicted_bot
	`, req.MatchID, req.PredictorID, req.PredictedBot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store prediction")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleResolvePredictions handles POST /api/predictions/{match_id}/resolve
// Called internally (worker or ticker) after a match completes.
func (s *Server) handleResolvePredictions(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("match_id")
	if matchID == "" {
		writeError(w, http.StatusBadRequest, "missing match_id")
		return
	}

	ctx := r.Context()

	// Get match winner
	var winnerID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT mp.bot_id FROM match_participants mp
		JOIN matches m ON mp.match_id = m.match_id
		WHERE m.match_id = $1
		  AND mp.player_slot = m.winner
	`, matchID).Scan(&winnerID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "match not found or has no winner")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	winner := winnerID.String

	// Get all unresolved predictions for this match
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, predictor_id, predicted_bot
		FROM predictions
		WHERE match_id = $1 AND correct IS NULL
	`, matchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type predRow struct {
		id          int64
		predictorID string
		predictedBot string
	}
	var preds []predRow
	for rows.Next() {
		var p predRow
		if err := rows.Scan(&p.id, &p.predictorID, &p.predictedBot); err != nil {
			continue
		}
		preds = append(preds, p)
	}

	now := time.Now().UTC()
	resolved := 0
	for _, p := range preds {
		correct := p.predictedBot == winner
		_, err := s.db.ExecContext(ctx, `
			UPDATE predictions SET correct = $1, resolved_at = $2 WHERE id = $3
		`, correct, now, p.id)
		if err != nil {
			continue
		}

		// Update predictor stats
		if correct {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO predictor_stats (predictor_id, correct, streak, best_streak)
				VALUES ($1, 1, 1, 1)
				ON CONFLICT (predictor_id) DO UPDATE SET
					correct    = predictor_stats.correct + 1,
					streak     = predictor_stats.streak + 1,
					best_streak = GREATEST(predictor_stats.best_streak, predictor_stats.streak + 1),
					updated_at  = NOW()
			`, p.predictorID)
		} else {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO predictor_stats (predictor_id, incorrect, streak)
				VALUES ($1, 1, 0)
				ON CONFLICT (predictor_id) DO UPDATE SET
					incorrect  = predictor_stats.incorrect + 1,
					streak     = 0,
					updated_at  = NOW()
			`, p.predictorID)
		}
		resolved++
	}

	writeJSON(w, http.StatusOK, map[string]int{"resolved": resolved})
}

// handlePredictionLeaderboard handles GET /api/predictions/leaderboard
func (s *Server) handlePredictionLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := s.db.QueryContext(ctx, `
		SELECT predictor_id, correct, incorrect,
		       CASE WHEN (correct + incorrect) > 0
		            THEN ROUND(100.0 * correct / (correct + incorrect), 1)
		            ELSE 0 END AS accuracy,
		       streak, best_streak
		FROM predictor_stats
		WHERE (correct + incorrect) >= 5
		ORDER BY accuracy DESC, correct DESC
		LIMIT 100
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type entry struct {
		PredictorID string  `json:"predictor_id"`
		Correct     int     `json:"correct"`
		Incorrect   int     `json:"incorrect"`
		Accuracy    float64 `json:"accuracy"`
		Streak      int     `json:"streak"`
		BestStreak  int     `json:"best_streak"`
	}
	entries := make([]entry, 0)
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.PredictorID, &e.Correct, &e.Incorrect, &e.Accuracy, &e.Streak, &e.BestStreak); err != nil {
			continue
		}
		entries = append(entries, e)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"leaderboard": entries,
		"updated_at":  time.Now().UTC(),
	})
}

// handleGetPredictions handles GET /api/predictions/{match_id}
func (s *Server) handleGetPredictions(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("match_id")
	ctx := r.Context()

	rows, err := s.db.QueryContext(ctx, `
		SELECT predictor_id, predicted_bot, correct
		FROM predictions
		WHERE match_id = $1
		ORDER BY created_at DESC
	`, matchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type pred struct {
		PredictorID  string  `json:"predictor_id"`
		PredictedBot string  `json:"predicted_bot"`
		Correct      *bool   `json:"correct"`
	}
	preds := make([]pred, 0)
	for rows.Next() {
		var p pred
		var correct sql.NullBool
		if err := rows.Scan(&p.PredictorID, &p.PredictedBot, &correct); err != nil {
			continue
		}
		if correct.Valid {
			b := correct.Bool
			p.Correct = &b
		}
		preds = append(preds, p)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"match_id":    matchID,
		"predictions": preds,
	})
}
