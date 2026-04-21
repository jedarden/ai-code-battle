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
	// 0. Propagate match results to series tables (winner_id, a_wins/b_wins)
	if err := m.updateSeriesGameResults(ctx); err != nil {
		log.Printf("series-scheduler: update results error: %v", err)
	}

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

	// 4. Advance championship bracket (semifinals/finals)
	if err := m.advanceChampionshipBracket(ctx); err != nil {
		log.Printf("series-scheduler: bracket advance error: %v", err)
	}
}

// updateSeriesGameResults finds completed series matches that haven't had their
// winner recorded yet. It updates series_games.winner_id and increments
// a_wins or b_wins on the series table.
func (m *Matchmaker) updateSeriesGameResults(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx, `
		SELECT sg.series_id, sg.game_num, sg.match_id, m.winner
		FROM series_games sg
		JOIN matches m ON sg.match_id = m.match_id
		WHERE sg.winner_id IS NULL
		  AND m.status = 'completed'
		  AND m.winner IS NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("query completed series games: %w", err)
	}
	defer rows.Close()

	type pendingUpdate struct {
		SeriesID int64
		GameNum  int
		MatchID  string
		Winner   int
	}
	var updates []pendingUpdate

	for rows.Next() {
		var u pendingUpdate
		if err := rows.Scan(&u.SeriesID, &u.GameNum, &u.MatchID, &u.Winner); err != nil {
			return fmt.Errorf("scan series game: %w", err)
		}
		updates = append(updates, u)
	}

	for _, u := range updates {
		var winnerBotID string
		err := m.db.QueryRowContext(ctx, `
			SELECT bot_id FROM match_participants
			WHERE match_id = $1 AND player_slot = $2
		`, u.MatchID, u.Winner).Scan(&winnerBotID)
		if err != nil {
			log.Printf("series-scheduler: could not find winner bot for match %s slot %d: %v", u.MatchID, u.Winner, err)
			continue
		}

		var botAID string
		err = m.db.QueryRowContext(ctx, `SELECT bot_a_id FROM series WHERE id = $1`, u.SeriesID).Scan(&botAID)
		if err != nil {
			continue
		}

		_, err = m.db.ExecContext(ctx, `
			UPDATE series_games SET winner_id = $1
			WHERE series_id = $2 AND game_num = $3
		`, winnerBotID, u.SeriesID, u.GameNum)
		if err != nil {
			log.Printf("series-scheduler: failed to update series_game winner: %v", err)
			continue
		}

		if winnerBotID == botAID {
			_, err = m.db.ExecContext(ctx, `
				UPDATE series SET a_wins = a_wins + 1, updated_at = NOW() WHERE id = $1
			`, u.SeriesID)
		} else {
			_, err = m.db.ExecContext(ctx, `
				UPDATE series SET b_wins = b_wins + 1, updated_at = NOW() WHERE id = $1
			`, u.SeriesID)
		}
		if err != nil {
			log.Printf("series-scheduler: failed to increment wins for series %d: %v", u.SeriesID, err)
			continue
		}

		log.Printf("series-scheduler: series %d game %d result recorded — winner=%s", u.SeriesID, u.GameNum, winnerBotID)
	}

	return nil
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
// It selects maps with varied characteristics per game number (§14.7) and
// alternates player slots for fairness.
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

	// Select a map with varied characteristics per game number (§14.7)
	mapID, rows, cols, mapSeed := m.selectSeriesMap(ctx, gameNum, rng)

	// Alternate player slots per game for round-robin fairness
	slotA, slotB := 0, 1
	if gameNum%2 == 0 {
		slotA, slotB = 1, 0
	}

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
		Rows:     rows,
		Cols:     cols,
		Bots: []botConfig{
			{BotID: botAID, Endpoint: endpointA, Secret: secretA, Slot: slotA},
			{BotID: botBID, Endpoint: endpointB, Secret: secretB, Slot: slotB},
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
		`INSERT INTO match_participants (match_id, bot_id, player_slot) VALUES ($1, $2, $3), ($1, $4, $5)`,
		matchID, botAID, slotA, botBID, slotB)
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

// selectSeriesMap picks a map with varied characteristics per game number.
// Per §14.7: Game 1 = highest engagement, Game 2 = highest wall density,
// Game 3 = lowest wall density, Game 4+ = random from pool.
// Returns (mapID, rows, cols, seed). Falls back to random seed if maps table is empty.
func (m *Matchmaker) selectSeriesMap(ctx context.Context, gameNum int, rng *rand.Rand) (string, int, int, int64) {
	var orderBy string
	switch {
	case gameNum == 1:
		orderBy = "engagement DESC NULLS LAST"
	case gameNum == 2:
		orderBy = "wall_density DESC NULLS LAST"
	case gameNum == 3:
		orderBy = "wall_density ASC NULLS LAST"
	default:
		orderBy = "RANDOM()"
	}

	query := fmt.Sprintf(`
		SELECT map_id, grid_width, grid_height FROM maps
		WHERE player_count = 2 AND status = 'active'
		ORDER BY %s LIMIT 1
	`, orderBy)

	var mapID string
	var gridW, gridH int
	err := m.db.QueryRowContext(ctx, query).Scan(&mapID, &gridW, &gridH)
	if err != nil {
		// No maps in table — generate from seed
		seed := rng.Int63()
		return fmt.Sprintf("map_%d", seed%100000), 60, 60, seed
	}

	return mapID, gridH, gridW, rng.Int63()
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
		} else if ratingGap >= 200 {
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
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, updated_at)
			VALUES ($1, $2, $3, 'active', 0, 0, $4, NOW())
		`, botAID, botBID, format, seasonID)
		if err != nil {
			log.Printf("series-scheduler: failed to create series (%s vs %s): %v", botAID, botBID, err)
			continue
		}
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

	// 5. Create championship bracket for top 8 (§14.9)
	if err := m.createChampionshipBracket(ctx, seasonID); err != nil {
		log.Printf("season-reset: championship bracket creation failed for season %d: %v", seasonID, err)
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

// advanceChampionshipBracket checks if any quarterfinal or semifinal series
// have completed and creates the next round of series.
func (m *Matchmaker) advanceChampionshipBracket(ctx context.Context) error {
	// Find completed quarterfinal series whose winners haven't been placed into semifinals yet
	rows, err := m.db.QueryContext(ctx, `
		SELECT s.id, s.season_id, s.bot_a_id, s.bot_b_id, s.winner_id, s.bracket_position
		FROM series s
		WHERE s.bracket_round = 'quarterfinal'
		  AND s.status = 'completed'
		  AND s.winner_id IS NOT NULL
		  AND s.season_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM series sf
		    WHERE sf.season_id = s.season_id
		      AND sf.bracket_round = 'semifinal'
		      AND sf.bracket_position = FLOOR(s.bracket_position / 2)
		  )
		ORDER BY s.bracket_position
	`)
	if err != nil {
		return fmt.Errorf("query completed quarterfinals: %w", err)
	}
	defer rows.Close()

	type completedQF struct {
		SeriesID int64
		SeasonID int64
		WinnerID string
		Position int
	}
	var completed []completedQF

	for rows.Next() {
		var qf completedQF
		var botAID, botBID, winnerID string
		var position int
		if err := rows.Scan(&qf.SeriesID, &qf.SeasonID, &botAID, &botBID, &winnerID, &position); err != nil {
			return fmt.Errorf("scan quarterfinal: %w", err)
		}
		qf.WinnerID = winnerID
		qf.Position = position
		completed = append(completed, qf)
	}

	// Group by season and create semifinal matchups
	type semifinalPair struct {
		seasonID  int64
		position  int
		winners   []string
	}
	pairs := make(map[string]*semifinalPair)
	for _, qf := range completed {
		sfPos := qf.Position / 2 // QF 0,1 → SF 0; QF 2,3 → SF 1
		key := fmt.Sprintf("%d-%d", qf.SeasonID, sfPos)
		if pairs[key] == nil {
			pairs[key] = &semifinalPair{seasonID: qf.SeasonID, position: sfPos}
		}
		pairs[key].winners = append(pairs[key].winners, qf.WinnerID)
	}

	for _, pair := range pairs {
		if len(pair.winners) < 2 {
			continue
		}
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, bracket_round, bracket_position, updated_at)
			VALUES ($1, $2, 7, 'active', 0, 0, $3, 'semifinal', $4, NOW())
		`, pair.winners[0], pair.winners[1], pair.seasonID, pair.position)
		if err != nil {
			log.Printf("series-scheduler: failed to create semifinal (%s vs %s): %v", pair.winners[0], pair.winners[1], err)
			continue
		}
		log.Printf("series-scheduler: created championship semifinal: %s vs %s", pair.winners[0], pair.winners[1])
	}

	// Check for completed semifinals → create final
	sfRows, err := m.db.QueryContext(ctx, `
		SELECT s.id, s.season_id, s.winner_id, s.bracket_position
		FROM series s
		WHERE s.bracket_round = 'semifinal'
		  AND s.status = 'completed'
		  AND s.winner_id IS NOT NULL
		  AND s.season_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM series f
		    WHERE f.season_id = s.season_id AND f.bracket_round = 'final'
		  )
		ORDER BY s.bracket_position
	`)
	if err != nil {
		return fmt.Errorf("query completed semifinals: %w", err)
	}
	defer sfRows.Close()

	type completedSF struct {
		SeasonID int64
		WinnerID string
	}
	var sfWinners []completedSF
	for sfRows.Next() {
		var sf completedSF
		var id int64
		var pos int
		if err := sfRows.Scan(&id, &sf.SeasonID, &sf.WinnerID, &pos); err != nil {
			return fmt.Errorf("scan semifinal: %w", err)
		}
		sfWinners = append(sfWinners, sf)
	}

	if len(sfWinners) >= 2 && sfWinners[0].SeasonID == sfWinners[1].SeasonID {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, bracket_round, bracket_position, updated_at)
			VALUES ($1, $2, 7, 'active', 0, 0, $3, 'final', 0, NOW())
		`, sfWinners[0].WinnerID, sfWinners[1].WinnerID, sfWinners[0].SeasonID)
		if err != nil {
			log.Printf("series-scheduler: failed to create championship final: %v", err)
		} else {
			log.Printf("series-scheduler: created championship final: %s vs %s", sfWinners[0].WinnerID, sfWinners[1].WinnerID)
		}
	}

	return nil
}

// createChampionshipBracket creates best-of-7 series for the top 8 bots
// in a single-elimination bracket at season end (§14.9).
// Bracket seeding: #1 vs #8, #4 vs #5, #3 vs #6, #2 vs #7
func (m *Matchmaker) createChampionshipBracket(ctx context.Context, seasonID int64) error {
	// Check if championship series already exist for this season
	var existing int
	err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM series WHERE season_id = $1 AND format = 7
	`, seasonID).Scan(&existing)
	if err != nil || existing > 0 {
		return nil // already created
	}

	// Get top 8 active bots by rating
	rows, err := m.db.QueryContext(ctx, `
		SELECT bot_id FROM bots
		WHERE status = 'active'
		ORDER BY rating_mu DESC
		LIMIT 8
	`)
	if err != nil {
		return fmt.Errorf("query top 8: %w", err)
	}
	defer rows.Close()

	var botIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		botIDs = append(botIDs, id)
	}

	if len(botIDs) < 8 {
		log.Printf("season-reset: not enough active bots (%d) for championship bracket, need 8", len(botIDs))
		return nil
	}

	// Standard bracket seeding: #1v8, #4v5, #3v6, #2v7
	// This ensures top seeds face weakest opponents and #1/#2 can only meet in finals
	bracket := []struct {
		a, b     string
		position int
	}{
		{botIDs[0], botIDs[7], 0}, // #1 vs #8
		{botIDs[3], botIDs[4], 1}, // #4 vs #5
		{botIDs[2], botIDs[5], 2}, // #3 vs #6
		{botIDs[1], botIDs[6], 3}, // #2 vs #7
	}

	for _, matchup := range bracket {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, bracket_round, bracket_position, updated_at)
			VALUES ($1, $2, 7, 'active', 0, 0, $3, 'quarterfinal', $4, NOW())
		`, matchup.a, matchup.b, seasonID, matchup.position)
		if err != nil {
			log.Printf("season-reset: failed to create championship quarterfinal series (%s vs %s): %v",
				matchup.a, matchup.b, err)
			continue
		}
		log.Printf("season-reset: created championship quarterfinal series: %s vs %s (bo7)",
			matchup.a, matchup.b)
	}

	return nil
}
