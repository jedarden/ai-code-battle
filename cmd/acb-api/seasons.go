package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// handleListSeasons handles GET /api/seasons
func (s *Server) handleListSeasons(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, theme, rules_version, status, champion_id, starts_at, ends_at, created_at
		FROM seasons
		ORDER BY created_at DESC
		LIMIT 20
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type seasonEntry struct {
		ID           int64      `json:"id"`
		Name         string     `json:"name"`
		Theme        *string    `json:"theme"`
		RulesVersion string     `json:"rules_version"`
		Status       string     `json:"status"`
		ChampionID   *string    `json:"champion_id"`
		StartsAt     time.Time  `json:"starts_at"`
		EndsAt       *time.Time `json:"ends_at"`
		CreatedAt    time.Time  `json:"created_at"`
	}
	seasons := make([]seasonEntry, 0)
	for rows.Next() {
		var se seasonEntry
		var theme, championID sql.NullString
		var endsAt sql.NullTime
		if err := rows.Scan(&se.ID, &se.Name, &theme, &se.RulesVersion, &se.Status,
			&championID, &se.StartsAt, &endsAt, &se.CreatedAt); err != nil {
			continue
		}
		if theme.Valid {
			se.Theme = &theme.String
		}
		if championID.Valid {
			se.ChampionID = &championID.String
		}
		if endsAt.Valid {
			se.EndsAt = &endsAt.Time
		}
		seasons = append(seasons, se)
	}

	writeJSON(w, http.StatusOK, map[string]any{"seasons": seasons})
}

// handleCreateSeason handles POST /api/seasons
func (s *Server) handleCreateSeason(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Theme        string `json:"theme"`
		RulesVersion string `json:"rules_version"`
		EndsAt       string `json:"ends_at"` // RFC3339
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.RulesVersion == "" {
		req.RulesVersion = "1.0"
	}

	ctx := r.Context()

	var endsAt sql.NullTime
	if req.EndsAt != "" {
		t, err := time.Parse(time.RFC3339, req.EndsAt)
		if err == nil {
			endsAt = sql.NullTime{Time: t, Valid: true}
		}
	}

	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO seasons (name, theme, rules_version, ends_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, req.Name, req.Theme, req.RulesVersion, endsAt).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create season")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"season_id": id, "ok": true})
}

// handleGetSeason handles GET /api/seasons/{id}
func (s *Server) handleGetSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	var se struct {
		ID           int64      `json:"id"`
		Name         string     `json:"name"`
		Theme        *string    `json:"theme"`
		RulesVersion string     `json:"rules_version"`
		Status       string     `json:"status"`
		ChampionID   *string    `json:"champion_id"`
		StartsAt     time.Time  `json:"starts_at"`
		EndsAt       *time.Time `json:"ends_at"`
	}
	var theme, championID sql.NullString
	var endsAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, theme, rules_version, status, champion_id, starts_at, ends_at
		FROM seasons WHERE id = $1
	`, id).Scan(&se.ID, &se.Name, &theme, &se.RulesVersion, &se.Status,
		&championID, &se.StartsAt, &endsAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "season not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if theme.Valid {
		se.Theme = &theme.String
	}
	if championID.Valid {
		se.ChampionID = &championID.String
	}
	if endsAt.Valid {
		se.EndsAt = &endsAt.Time
	}

	// Get leaderboard snapshot for this season
	rows, err := s.db.QueryContext(ctx, `
		SELECT ss.bot_id, b.name, ss.rank, ss.rating, ss.wins, ss.losses, ss.recorded_at
		FROM season_snapshots ss
		JOIN bots b ON ss.bot_id = b.bot_id
		WHERE ss.season_id = $1
		ORDER BY ss.rank
		LIMIT 50
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type snap struct {
		BotID      string    `json:"bot_id"`
		BotName    string    `json:"bot_name"`
		Rank       int       `json:"rank"`
		Rating     float64   `json:"rating"`
		Wins       int       `json:"wins"`
		Losses     int       `json:"losses"`
		RecordedAt time.Time `json:"recorded_at"`
	}
	snapshots := make([]snap, 0)
	for rows.Next() {
		var sn snap
		if err := rows.Scan(&sn.BotID, &sn.BotName, &sn.Rank, &sn.Rating, &sn.Wins, &sn.Losses, &sn.RecordedAt); err != nil {
			continue
		}
		snapshots = append(snapshots, sn)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"season":    se,
		"standings": snapshots,
	})
}

// handleSnapshotSeason handles POST /api/seasons/{id}/snapshot
// Takes a snapshot of the current leaderboard for the season archive.
func (s *Server) handleSnapshotSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	// Check season exists
	var seasonName string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM seasons WHERE id = $1`, id).Scan(&seasonName)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "season not found")
		return
	}

	// Take snapshot of current leaderboard
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO season_snapshots (season_id, bot_id, rank, rating, wins, losses)
		SELECT $1, bot_id,
		       ROW_NUMBER() OVER (ORDER BY rating_mu DESC),
		       rating_mu,
		       (SELECT COUNT(*) FROM match_participants mp2
		        JOIN matches m2 ON mp2.match_id = m2.match_id
		        WHERE mp2.bot_id = b.bot_id AND m2.status = 'completed'
		          AND m2.winner = mp2.player_slot),
		       (SELECT COUNT(*) FROM match_participants mp3
		        JOIN matches m3 ON mp3.match_id = m3.match_id
		        WHERE mp3.bot_id = b.bot_id AND m3.status = 'completed'
		          AND m3.winner != mp3.player_slot AND m3.winner >= 0)
		FROM bots b
		WHERE status = 'active'
		ORDER BY rating_mu DESC
		LIMIT 100
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to snapshot season")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleCloseSeason handles POST /api/seasons/{id}/close
func (s *Server) handleCloseSeason(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	// Find current leader
	var championID sql.NullString
	_ = s.db.QueryRowContext(ctx, `
		SELECT bot_id FROM season_snapshots
		WHERE season_id = $1
		ORDER BY rank ASC LIMIT 1
	`, id).Scan(&championID)

	_, err := s.db.ExecContext(ctx, `
		UPDATE seasons SET status = 'archived', champion_id = $1, ends_at = NOW()
		WHERE id = $2
	`, championID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close season")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
