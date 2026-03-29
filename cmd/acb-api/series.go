package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// handleCreateSeries handles POST /api/series
func (s *Server) handleCreateSeries(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BotAID string `json:"bot_a_id"`
		BotBID string `json:"bot_b_id"`
		Format int    `json:"format"` // best of N
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.BotAID == "" || req.BotBID == "" {
		writeError(w, http.StatusBadRequest, "bot_a_id and bot_b_id are required")
		return
	}
	if req.Format < 1 {
		req.Format = 5
	}

	ctx := r.Context()

	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO series (bot_a_id, bot_b_id, format)
		VALUES ($1, $2, $3)
		RETURNING id
	`, req.BotAID, req.BotBID, req.Format).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create series")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"series_id": id, "ok": true})
}

// handleListSeries handles GET /api/series
func (s *Server) handleListSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.bot_a_id, ba.name, s.bot_b_id, bb.name,
		       s.format, s.a_wins, s.b_wins, s.status, s.winner_id,
		       s.created_at, s.updated_at
		FROM series s
		JOIN bots ba ON s.bot_a_id = ba.bot_id
		JOIN bots bb ON s.bot_b_id = bb.bot_id
		ORDER BY s.updated_at DESC
		LIMIT 50
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type seriesEntry struct {
		ID        int64     `json:"id"`
		BotAID    string    `json:"bot_a_id"`
		BotAName  string    `json:"bot_a_name"`
		BotBID    string    `json:"bot_b_id"`
		BotBName  string    `json:"bot_b_name"`
		Format    int       `json:"format"`
		AWins     int       `json:"a_wins"`
		BWins     int       `json:"b_wins"`
		Status    string    `json:"status"`
		WinnerID  *string   `json:"winner_id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	entries := make([]seriesEntry, 0)
	for rows.Next() {
		var e seriesEntry
		var winnerID sql.NullString
		if err := rows.Scan(&e.ID, &e.BotAID, &e.BotAName, &e.BotBID, &e.BotBName,
			&e.Format, &e.AWins, &e.BWins, &e.Status, &winnerID,
			&e.CreatedAt, &e.UpdatedAt); err != nil {
			continue
		}
		if winnerID.Valid {
			e.WinnerID = &winnerID.String
		}
		entries = append(entries, e)
	}

	writeJSON(w, http.StatusOK, map[string]any{"series": entries})
}

// handleGetSeries handles GET /api/series/{id}
func (s *Server) handleGetSeries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	type game struct {
		MatchID   string    `json:"match_id"`
		GameNum   int       `json:"game_num"`
		WinnerID  *string   `json:"winner_id"`
		CreatedAt time.Time `json:"created_at"`
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT match_id, game_num, winner_id, created_at
		FROM series_games
		WHERE series_id = $1
		ORDER BY game_num
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	games := make([]game, 0)
	for rows.Next() {
		var g game
		var winnerID sql.NullString
		if err := rows.Scan(&g.MatchID, &g.GameNum, &winnerID, &g.CreatedAt); err != nil {
			continue
		}
		if winnerID.Valid {
			g.WinnerID = &winnerID.String
		}
		games = append(games, g)
	}

	// Get series header
	var se struct {
		ID        int64     `json:"id"`
		BotAID    string    `json:"bot_a_id"`
		BotAName  string    `json:"bot_a_name"`
		BotBID    string    `json:"bot_b_id"`
		BotBName  string    `json:"bot_b_name"`
		Format    int       `json:"format"`
		AWins     int       `json:"a_wins"`
		BWins     int       `json:"b_wins"`
		Status    string    `json:"status"`
		WinnerID  *string   `json:"winner_id"`
		CreatedAt time.Time `json:"created_at"`
	}
	var winnerID sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT s.id, s.bot_a_id, ba.name, s.bot_b_id, bb.name,
		       s.format, s.a_wins, s.b_wins, s.status, s.winner_id, s.created_at
		FROM series s
		JOIN bots ba ON s.bot_a_id = ba.bot_id
		JOIN bots bb ON s.bot_b_id = bb.bot_id
		WHERE s.id = $1
	`, id).Scan(&se.ID, &se.BotAID, &se.BotAName, &se.BotBID, &se.BotBName,
		&se.Format, &se.AWins, &se.BWins, &se.Status, &winnerID, &se.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "series not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if winnerID.Valid {
		se.WinnerID = &winnerID.String
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"series": se,
		"games":  games,
	})
}

// handleAddSeriesGame handles POST /api/series/{id}/games
func (s *Server) handleAddSeriesGame(w http.ResponseWriter, r *http.Request) {
	seriesID := r.PathValue("id")
	var req struct {
		MatchID string `json:"match_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()

	// Get series info
	var botAID, botBID string
	var format, aWins, bWins int
	var status string
	err := s.db.QueryRowContext(ctx, `
		SELECT bot_a_id, bot_b_id, format, a_wins, b_wins, status
		FROM series WHERE id = $1
	`, seriesID).Scan(&botAID, &botBID, &format, &aWins, &bWins, &status)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "series not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if status != "active" {
		writeError(w, http.StatusConflict, "series is not active")
		return
	}

	// Get match winner
	var matchWinnerSlot sql.NullInt64
	err = s.db.QueryRowContext(ctx, `SELECT winner FROM matches WHERE match_id = $1`, req.MatchID).Scan(&matchWinnerSlot)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "match not found")
		return
	}

	// Determine which bot won
	var winnerBotID sql.NullString
	if matchWinnerSlot.Valid {
		slot := int(matchWinnerSlot.Int64)
		if slot == 0 {
			winnerBotID.String = botAID
			winnerBotID.Valid = true
		} else if slot == 1 {
			winnerBotID.String = botBID
			winnerBotID.Valid = true
		}
	}

	// Get next game number
	var gameNum int
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(game_num), 0) + 1 FROM series_games WHERE series_id = $1
	`, seriesID).Scan(&gameNum)

	// Insert game
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO series_games (series_id, match_id, game_num, winner_id)
		VALUES ($1, $2, $3, $4)
	`, seriesID, req.MatchID, gameNum, winnerBotID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add game")
		return
	}

	// Update win counts and check if series is decided
	if winnerBotID.Valid {
		if winnerBotID.String == botAID {
			aWins++
		} else {
			bWins++
		}
	}

	toWin := (format / 2) + 1
	newStatus := "active"
	var seriesWinner sql.NullString
	if aWins >= toWin {
		newStatus = "completed"
		seriesWinner.String = botAID
		seriesWinner.Valid = true
	} else if bWins >= toWin {
		newStatus = "completed"
		seriesWinner.String = botBID
		seriesWinner.Valid = true
	}

	_, _ = s.db.ExecContext(ctx, `
		UPDATE series SET a_wins=$1, b_wins=$2, status=$3, winner_id=$4, updated_at=NOW()
		WHERE id = $5
	`, aWins, bWins, newStatus, seriesWinner, seriesID)

	writeJSON(w, http.StatusOK, map[string]any{
		"game_num": gameNum,
		"a_wins":   aWins,
		"b_wins":   bWins,
		"status":   newStatus,
	})
}
