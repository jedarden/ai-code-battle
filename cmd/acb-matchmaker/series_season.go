package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"
)

// tickSeriesScheduler schedules remaining games for active series.
// For each active series with unplayed games, it schedules the next game
// in round-robin order, feeding the match into the job queue.
// It also marks series as completed when a bot reaches the winning threshold.
func (m *Matchmaker) tickSeriesScheduler(ctx context.Context) {
	// 1. Finalize any completed series (check if winner reached threshold)
	if err := m.finalizeCompletedSeries(ctx); err != nil {
		log.Printf("series-scheduler: finalize error: %v", err)
	}

	// 2. Schedule next game for active series that need one
	if err := m.scheduleNextSeriesGames(ctx); err != nil {
		log.Printf("series-scheduler: schedule error: %v", err)
	}

	// 3. Auto-create series for top bots (one per bot per day, best-of-5)
	if err := m.autoCreateSeries(ctx); err != nil {
		log.Printf("series-scheduler: auto-create error: %v", err)
	}
}

// finalizeCompletedSeries checks active series where one bot has already won enough games.
func (m *Matchmaker) finalizeCompletedSeries(ctx context.Context) error {
	// Find active series where a_wins or b_wins >= ceil(format/2)
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, bot_a_id, bot_b_id, format, a_wins, b_wins
		FROM series
		WHERE status = 'active'
		  AND (a_wins >= ((format + 1) / 2) OR b_wins >= ((format + 1) / 2))
	`)
	if err != nil {
		return fmt.Errorf("query completed series: %w", err)
	}
	defer rows.Close()

	type completedSeries struct {
		ID     int64
		BotAID string
		BotBID string
		Format int
		AWins  int
		BWins  int
	}
	var completed []completedSeries

	for rows.Next() {
		var s completedSeries
		if err := rows.Scan(&s.ID, &s.BotAID, &s.BotBID, &s.Format, &s.AWins, &s.BWins); err != nil {
			return fmt.Errorf("scan series: %w", err)
		}
		completed = append(completed, s)
	}

	for _, s := range completed {
		winsNeeded := (s.Format + 1) / 2
		var winnerID string
		if s.AWins >= winsNeeded {
			winnerID = s.BotAID
		} else {
			winnerID = s.BotBID
		}

		_, err := m.db.ExecContext(ctx, `
			UPDATE series
			SET status = 'completed', winner_id = $1, updated_at = NOW()
			WHERE id = $2 AND status = 'active'
		`, winnerID, s.ID)
		if err != nil {
			log.Printf("series-scheduler: failed to finalize series %d: %v", s.ID, err)
			continue
		}
		log.Printf("series-scheduler: finalized series %d, winner=%s (%d-%d)", s.ID, winnerID, s.AWins, s.BWins)
	}

	return nil
}

// scheduleNextSeriesGames finds active series with unplayed games and schedules the next one.
func (m *Matchmaker) scheduleNextSeriesGames(ctx context.Context) error {
	// Find active series where the next sequential game has no match_id yet
	rows, err := m.db.QueryContext(ctx, `
		SELECT s.id, s.bot_a_id, s.bot_b_id, s.format, s.a_wins, s.b_wins,
		       COALESCE(MAX(sg.game_num), 0) AS last_game_num
		FROM series s
		LEFT JOIN series_games sg ON s.id = sg.series_id AND sg.match_id IS NOT NULL
		WHERE s.status = 'active'
		GROUP BY s.id, s.bot_a_id, s.bot_b_id, s.format, s.a_wins, s.b_wins
		HAVING COUNT(sg.id) < s.format
	`)
	if err != nil {
		return fmt.Errorf("query pending series: %w", err)
	}
	defer rows.Close()

	type pendingSeries struct {
		ID         int64
		BotAID     string
		BotBID     string
		Format     int
		AWins      int
		BWins      int
		LastGameNum int
	}
	var pending []pendingSeries

	for rows.Next() {
		var s pendingSeries
		if err := rows.Scan(&s.ID, &s.BotAID, &s.BotBID, &s.Format, &s.AWins, &s.BWins, &s.LastGameNum); err != nil {
			return fmt.Errorf("scan pending series: %w", err)
		}

		// Skip if series is already decided
		winsNeeded := (s.Format + 1) / 2
		if s.AWins >= winsNeeded || s.BWins >= winsNeeded {
			continue
		}

		// Check that both bots are active
		var aActive, bActive bool
		err := m.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM bots WHERE bot_id = $1 AND status = 'active')`, s.BotAID).Scan(&aActive)
		if err != nil {
			continue
		}
		err = m.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM bots WHERE bot_id = $1 AND status = 'active')`, s.BotBID).Scan(&bActive)
		if err != nil {
			continue
		}
		if !aActive || !bActive {
			continue
		}

		pending = append(pending, s)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, s := range pending {
		nextGameNum := s.LastGameNum + 1

		// Check if this game already has a pending match (not yet completed)
		var existingMatch int
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM series_games
			WHERE series_id = $1 AND game_num = $2 AND match_id IS NOT NULL
		`, s.ID, nextGameNum).Scan(&existingMatch)
		if err != nil || existingMatch > 0 {
			continue // already scheduled or played
		}

		// Check if there's already a pending/running job for this series
		var pendingJobs int
		err = m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM series_games sg
			JOIN matches m ON sg.match_id = m.match_id
			JOIN jobs j ON j.match_id = m.match_id
			WHERE sg.series_id = $1 AND j.status IN ('pending', 'running')
		`, s.ID).Scan(&pendingJobs)
		if err != nil || pendingJobs > 0 {
			continue // a game is already in progress for this series
		}

		if err := m.scheduleSeriesGame(ctx, s.ID, s.BotAID, s.BotBID, nextGameNum, rng); err != nil {
			log.Printf("series-scheduler: failed to schedule game %d for series %d: %v", nextGameNum, s.ID, err)
			continue
		}
		log.Printf("series-scheduler: scheduled game %d for series %d (%s vs %s)", nextGameNum, s.ID, s.BotAID, s.BotBID)
	}

	return nil
}

// scheduleSeriesGame creates a match and job for one game in a series.
func (m *Matchmaker) scheduleSeriesGame(ctx context.Context, seriesID int64, botAID, botBID string, gameNum int, rng *rand.Rand) error {
	// Fetch bot endpoints and secrets
	var endpointA, secretA, endpointB, secretB string
	err := m.db.QueryRowContext(ctx,
		`SELECT endpoint_url, shared_secret FROM bots WHERE bot_id = $1`, botAID).Scan(&endpointA, &secretA)
	if err != nil {
		return fmt.Errorf("fetch bot %s: %w", botAID, err)
	}
	err = m.db.QueryRowContext(ctx,
		`SELECT endpoint_url, shared_secret FROM bots WHERE bot_id = $1`, botBID).Scan(&endpointB, &secretB)
	if err != nil {
		return fmt.Errorf("fetch bot %s: %w", botBID, err)
	}

	// Decrypt secrets
	if m.cfg.EncryptionKey != "" {
		if dec, err := decryptSecret(secretA, m.cfg.EncryptionKey); err == nil {
			secretA = dec
		}
		if dec, err := decryptSecret(secretB, m.cfg.EncryptionKey); err == nil {
			secretB = dec
		}
	}

	matchID, err := generateID("m_", 8)
	if err != nil {
		return err
	}
	jobID, err := generateID("j_", 8)
	if err != nil {
		return err
	}

	mapSeed := rng.Int63()
	mapID := fmt.Sprintf("map_%d", mapSeed%100000)

	type botConfig struct {
		BotID    string `json:"bot_id"`
		Endpoint string `json:"endpoint"`
		Secret   string `json:"secret"`
		Slot     int    `json:"slot"`
	}
	type jobConfig struct {
		MatchID  string      `json:"match_id"`
		SeriesID int64       `json:"series_id,omitempty"`
		GameNum  int         `json:"game_num,omitempty"`
		MapSeed  int64       `json:"map_seed"`
		MaxTurns int         `json:"max_turns"`
		Rows     int         `json:"rows"`
		Cols     int         `json:"cols"`
		Bots     []botConfig `json:"bots"`
	}

	config := jobConfig{
		MatchID:  matchID,
		SeriesID: seriesID,
		GameNum:  gameNum,
		MapSeed:  mapSeed,
		MaxTurns: 500,
		Rows:     60,
		Cols:     60,
		Bots: []botConfig{
			{BotID: botAID, Endpoint: endpointA, Secret: secretA, Slot: 0},
			{BotID: botBID, Endpoint: endpointB, Secret: secretB, Slot: 1},
		},
	}
	configJSON, _ := json.Marshal(config)

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO matches (match_id, map_id, map_seed, status) VALUES ($1, $2, $3, 'pending')`,
		matchID, mapID, mapSeed)
	if err != nil {
		return fmt.Errorf("insert match: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO match_participants (match_id, bot_id, player_slot) VALUES ($1, $2, 0), ($1, $3, 1)`,
		matchID, botAID, botBID)
	if err != nil {
		return fmt.Errorf("insert participants: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO jobs (job_id, match_id, status, config_json) VALUES ($1, $2, 'pending', $3)`,
		jobID, matchID, configJSON)
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	// Create the series_games row
	_, err = tx.ExecContext(ctx, `
		INSERT INTO series_games (series_id, match_id, game_num, winner_id)
		VALUES ($1, $2, $3, NULL)
	`, seriesID, matchID, gameNum)
	if err != nil {
		return fmt.Errorf("insert series_game: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Enqueue in Valkey
	if err := m.rdb.LPush(ctx, valkeyJobQueue, jobID).Err(); err != nil {
		return fmt.Errorf("valkey push: %w", err)
	}

	return nil
}

// autoCreateSeries creates best-of-5 series between top-20 active bots,
// one per bot per day.
func (m *Matchmaker) autoCreateSeries(ctx context.Context) error {
	// Find top-20 active bots by rating
	rows, err := m.db.QueryContext(ctx, `
		SELECT bot_id FROM bots
		WHERE status = 'active' AND evolved = false
		ORDER BY rating_mu DESC
		LIMIT 20
	`)
	if err != nil {
		return fmt.Errorf("query top bots: %w", err)
	}
	defer rows.Close()

	var topBots []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		topBots = append(topBots, id)
	}

	if len(topBots) < 2 {
		return nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, botID := range topBots {
		// Check if this bot already has an active or pending series created today
		var todaySeries int
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM series
			WHERE (bot_a_id = $1 OR bot_b_id = $1)
			  AND created_at >= NOW() - INTERVAL '24 hours'
			  AND status IN ('active', 'pending')
		`, botID).Scan(&todaySeries)
		if err != nil || todaySeries > 0 {
			continue
		}

		// Pick an opponent — closest rating that isn't this bot and doesn't have an active series
		var opponentID string
		err = m.db.QueryRowContext(ctx, `
			SELECT b.bot_id FROM bots b
			WHERE b.bot_id != $1
			  AND b.status = 'active'
			  AND NOT EXISTS (
			    SELECT 1 FROM series s
			    WHERE ((s.bot_a_id = $1 AND s.bot_b_id = b.bot_id)
			           OR (s.bot_a_id = b.bot_id AND s.bot_b_id = $1))
			      AND s.status IN ('active', 'pending')
			  )
			ORDER BY ABS(b.rating_mu - (SELECT rating_mu FROM bots WHERE bot_id = $1)) ASC
			LIMIT 1
		`, botID).Scan(&opponentID)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return fmt.Errorf("find opponent for %s: %w", botID, err)
		}

		// Determine format based on ratings — closer ratings get longer series
		var botRating, oppRating float64
		err = m.db.QueryRowContext(ctx,
			`SELECT rating_mu FROM bots WHERE bot_id = $1`, botID).Scan(&botRating)
		if err != nil {
			continue
		}
		err = m.db.QueryRowContext(ctx,
			`SELECT rating_mu FROM bots WHERE bot_id = $1`, opponentID).Scan(&oppRating)
		if err != nil {
			continue
		}

		format := 5 // default best-of-5
		ratingGap := botRating - oppRating
		if ratingGap < 0 {
			ratingGap = -ratingGap
		}
		if ratingGap < 50 {
			format = 7 // close ratings → best-of-7
		} else if ratingGap > 200 {
			format = 3 // large gap → best-of-3
		}

		// Randomize who is bot_a vs bot_b
		botAID, botBID := botID, opponentID
		if rng.Intn(2) == 0 {
			botAID, botBID = botBID, botAID
		}

		// Get the active season ID (if any)
		var seasonID sql.NullInt64
		m.db.QueryRowContext(ctx,
			`SELECT id FROM seasons WHERE status = 'active' ORDER BY starts_at DESC LIMIT 1`).Scan(&seasonID)

		_, err = m.db.ExecContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, updated_at)
			VALUES ($1, $2, $3, 'active', 0, 0, NOW())
		`, botAID, botBID, format)
		if err != nil {
			log.Printf("series-scheduler: failed to create series (%s vs %s): %v", botAID, botBID, err)
			continue
		}
		_ = seasonID // use in future season-series linking
		log.Printf("series-scheduler: created best-of-%d series: %s vs %s", format, botAID, botBID)
	}

	return nil
}

// tickSeasonReset checks for seasons that have ended and performs:
// 1. Snapshot current ELO ratings into season_snapshots
// 2. Apply decay formula to all bot ratings
// 3. Close the old season and start a new one
func (m *Matchmaker) tickSeasonReset(ctx context.Context) {
	// Find active seasons that have passed their ends_at
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, theme, rules_version FROM seasons
		WHERE status = 'active' AND ends_at IS NOT NULL AND ends_at <= NOW()
	`)
	if err != nil {
		log.Printf("season-reset: query error: %v", err)
		return
	}
	defer rows.Close()

	type endingSeason struct {
		ID           int64
		Name         string
		Theme        string
		RulesVersion string
	}
	var ending []endingSeason

	for rows.Next() {
		var s endingSeason
		var theme sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &theme, &s.RulesVersion); err != nil {
			log.Printf("season-reset: scan error: %v", err)
			return
		}
		if theme.Valid {
			s.Theme = theme.String
		}
		ending = append(ending, s)
	}

	for _, s := range ending {
		if err := m.processSeasonEnd(ctx, s.ID, s.Name); err != nil {
			log.Printf("season-reset: failed to process season %d (%s): %v", s.ID, s.Name, err)
			continue
		}
		log.Printf("season-reset: processed season %d (%s) — snapshot + decay complete", s.ID, s.Name)
	}

	// Check if there's no active season and auto-start one
	m.autoStartSeason(ctx)
}

// processSeasonEnd handles the end-of-season workflow for one season.
func (m *Matchmaker) processSeasonEnd(ctx context.Context, seasonID int64, seasonName string) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Snapshot current ratings into season_snapshots
	_, err = tx.ExecContext(ctx, `
		INSERT INTO season_snapshots (season_id, bot_id, rank, rating, wins, losses)
		SELECT $1, b.bot_id,
		       ROW_NUMBER() OVER (ORDER BY b.rating_mu DESC)::int,
		       b.rating_mu,
		       COALESCE(mp.wins, 0),
		       COALESCE(mp.losses, 0)
		FROM bots b
		LEFT JOIN (
		    SELECT bot_id,
		           COUNT(*) FILTER (WHERE player_slot = m.winner) AS wins,
		           COUNT(*) FILTER (WHERE player_slot != m.winner) AS losses
		    FROM match_participants mp
		    JOIN matches m ON mp.match_id = m.match_id
		    WHERE m.status = 'completed'
		    GROUP BY bot_id
		) mp ON mp.bot_id = b.bot_id
		WHERE b.status != 'retired'
		ORDER BY b.rating_mu DESC
	`, seasonID)
	if err != nil {
		return fmt.Errorf("snapshot ratings: %w", err)
	}

	// 2. Determine champion (rank 1)
	var championID string
	err = tx.QueryRowContext(ctx, `
		SELECT bot_id FROM season_snapshots
		WHERE season_id = $1 AND rank = 1
	`, seasonID).Scan(&championID)
	if err != nil {
		log.Printf("season-reset: could not determine champion for season %d: %v", seasonID, err)
	}

	// 3. Mark season as completed
	_, err = tx.ExecContext(ctx, `
		UPDATE seasons SET status = 'completed', champion_id = $1 WHERE id = $2
	`, championID, seasonID)
	if err != nil {
		return fmt.Errorf("complete season: %w", err)
	}

	// 4. Apply decay to all non-retired bots
	//    Formula: new_mu = default + (current_mu - default) * decay_factor
	//    This pulls ratings toward 1500 but preserves relative ordering
	decayFactor := m.cfg.SeasonDecayFactor
	defaultMu := 1500.0
	defaultPhi := 350.0
	defaultSigma := 0.06

	_, err = tx.ExecContext(ctx, `
		UPDATE bots SET
			rating_mu = $1 + (rating_mu - $1) * $2,
			rating_phi = $3,
			rating_sigma = $4
		WHERE status != 'retired'
	`, defaultMu, decayFactor, defaultPhi, defaultSigma)
	if err != nil {
		return fmt.Errorf("apply decay: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("season-reset: season %d (%s) complete — champion=%s, decay=%.0f%%",
		seasonID, seasonName, championID, decayFactor*100)

	return nil
}

// autoStartSeason creates a new season if no active season exists.
func (m *Matchmaker) autoStartSeason(ctx context.Context) {
	var activeCount int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM seasons WHERE status = 'active'`).Scan(&activeCount)
	if err != nil || activeCount > 0 {
		return
	}

	// Determine next season number
	var maxNum int
	err = m.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), 0) FROM seasons`).Scan(&maxNum)
	if err != nil {
		return
	}

	nextNum := maxNum + 1
	seasonName := fmt.Sprintf("Season %d", nextNum)
	themes := []string{"The Labyrinth", "Energy Rush", "Fog of War", "The Colosseum", "Shifting Sands"}
	theme := themes[(nextNum-1)%len(themes)]
	rulesVersion := fmt.Sprintf("%d.0", nextNum)

	_, err = m.db.ExecContext(ctx, `
		INSERT INTO seasons (name, theme, rules_version, status, starts_at, ends_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW() + INTERVAL '28 days')
	`, seasonName, theme, rulesVersion)
	if err != nil {
		log.Printf("season-reset: failed to create new season: %v", err)
		return
	}

	log.Printf("season-reset: auto-started %s (%s) — ends in 28 days", seasonName, theme)
}
