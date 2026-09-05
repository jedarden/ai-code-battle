package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"time"
)

// tickSeriesScheduler maintains the series pipeline. Every game of a series is
// created up front as a pending job (§14.7) — createSeriesGames does that at
// series creation — so this tick's job is bookkeeping: propagate match results
// into the series tables, finalise series that are decided, retire the games
// left over on series that are over or wedged, and create new series.
func (m *Matchmaker) tickSeriesScheduler(ctx context.Context) {
	// 0. Propagate match results to series tables (winner_id, a_wins/b_wins)
	if err := m.updateSeriesGameResults(ctx); err != nil {
		log.Printf("series-scheduler: update results error: %v", err)
	}

	// 1. Finalize any completed series (check if winner reached threshold)
	if err := m.finalizeCompletedSeries(ctx); err != nil {
		log.Printf("series-scheduler: finalize error: %v", err)
	}

	// 2. Cancel the games left over on series that are over, and finalize any
	// series a permanently failed game has wedged.
	if err := m.cancelDecidedSeriesGames(ctx); err != nil {
		log.Printf("series-scheduler: cancel error: %v", err)
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
// a_wins or b_wins on the series table. Drawn games (m.winner IS NULL) are
// marked so the series can continue to the next game.
func (m *Matchmaker) updateSeriesGameResults(ctx context.Context) error {
	// Both passes below are scoped to series that are still active. The
	// ordering gate opens a series' remaining games the moment its status
	// leaves 'active', so a result arriving for a finished series can only
	// come from a game that should have been cancelled — recording it would
	// rewrite a tally that has already been finalised.
	//
	// First, handle draws: completed matches with no winner
	_, err := m.db.ExecContext(ctx, `
		UPDATE series_games SET winner_id = 'draw'
		FROM matches m
		JOIN series s ON s.id = series_games.series_id
		WHERE series_games.match_id = m.match_id
		  AND series_games.winner_id IS NULL
		  AND m.status = 'completed'
		  AND m.winner IS NULL
		  AND s.status = 'active'
	`)
	if err != nil {
		log.Printf("series-scheduler: failed to process drawn games: %v", err)
	}

	// Then, process games with a winner
	rows, err := m.db.QueryContext(ctx, `
		SELECT sg.series_id, sg.game_num, sg.match_id, m.winner
		FROM series_games sg
		JOIN matches m ON sg.match_id = m.match_id
		JOIN series s ON s.id = sg.series_id
		WHERE sg.winner_id IS NULL
		  AND m.status = 'completed'
		  AND m.winner IS NOT NULL
		  AND s.status = 'active'
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
				UPDATE series SET a_wins = a_wins + 1, updated_at = NOW()
				WHERE id = $1 AND status = 'active'
			`, u.SeriesID)
		} else {
			_, err = m.db.ExecContext(ctx, `
				UPDATE series SET b_wins = b_wins + 1, updated_at = NOW()
				WHERE id = $1 AND status = 'active'
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

	// Also finalize series where all games are played but neither side reached
	// the threshold (can happen with draws). Winner is whoever has more wins,
	// or NULL if equal.
	allPlayed, err := m.db.QueryContext(ctx, `
		SELECT s.id, s.bot_a_id, s.bot_b_id, s.a_wins, s.b_wins, s.format
		FROM series s
		WHERE s.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1 FROM series_games sg
		    WHERE sg.series_id = s.id AND sg.winner_id IS NULL
		  )
		  AND EXISTS (SELECT 1 FROM series_games sg WHERE sg.series_id = s.id)
	`)
	if err != nil {
		return fmt.Errorf("query all-played series: %w", err)
	}
	defer allPlayed.Close()

	for allPlayed.Next() {
		var apID int64
		var apBotA, apBotB string
		var apAWins, apBWins, apFormat int
		if err := allPlayed.Scan(&apID, &apBotA, &apBotB, &apAWins, &apBWins, &apFormat); err != nil {
			continue
		}
		winsNeeded := (apFormat + 1) / 2
		if apAWins >= winsNeeded || apBWins >= winsNeeded {
			continue // already handled above
		}
		var winnerID *string
		if apAWins > apBWins {
			winnerID = &apBotA
		} else if apBWins > apAWins {
			winnerID = &apBotB
		}
		_, err := m.db.ExecContext(ctx, `
			UPDATE series SET status = 'completed', winner_id = $1, updated_at = NOW()
			WHERE id = $2 AND status = 'active'
		`, winnerID, apID)
		if err != nil {
			log.Printf("series-scheduler: failed to finalize all-played series %d: %v", apID, err)
			continue
		}
		log.Printf("series-scheduler: finalized all-played series %d (%d-%d), winner=%v", apID, apAWins, apBWins, winnerID)
	}

	return nil
}

// createSeriesGames creates every game of a series up front, as §14.7
// describes: each game gets a pending match, a pending job and a series_games
// row carrying the map chosen for its game number. Workers then run them in
// order — a game is only claimable once every earlier game in the same series
// has a result (see GetNextJob's ordering gate), and the games left over once
// a bot reaches the winning threshold are cancelled by cancelDecidedSeriesGames.
func (m *Matchmaker) createSeriesGames(ctx context.Context, seriesID int64, botAID, botBID string, format int) error {
	// Fetch bot endpoints and secrets once; every game uses the same pair.
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

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for gameNum := 1; gameNum <= format; gameNum++ {
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

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO matches (match_id, map_id, map_seed, status) VALUES ($1, $2, $3, 'pending')`,
			matchID, mapID, mapSeed); err != nil {
			return fmt.Errorf("insert match: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO match_participants (match_id, bot_id, player_slot) VALUES ($1, $2, $3), ($1, $4, $5)`,
			matchID, botAID, slotA, botBID, slotB); err != nil {
			return fmt.Errorf("insert participants: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO jobs (job_id, match_id, status, config_json) VALUES ($1, $2, 'pending', $3)`,
			jobID, matchID, configJSON); err != nil {
			return fmt.Errorf("insert job: %w", err)
		}

		// Create the series_games row, recording the map picked for this game
		// so the series page can name it before the game is played.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO series_games (series_id, match_id, game_num, map_id, winner_id)
			VALUES ($1, $2, $3, $4, NULL)
		`, seriesID, matchID, gameNum, mapID); err != nil {
			return fmt.Errorf("insert series_game: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Wake workers up to game 1. Games 2..N are visible in the jobs table but
	// stay unclaimable until their predecessors resolve, so pushing them here
	// would only invite workers to bounce off the ordering gate.
	var firstJob string
	if err := m.db.QueryRowContext(ctx, `
		SELECT j.job_id FROM jobs j
		JOIN series_games sg ON sg.match_id = j.match_id
		WHERE sg.series_id = $1 AND sg.game_num = 1
	`, seriesID).Scan(&firstJob); err == nil {
		if err := m.rdb.LPush(ctx, valkeyJobQueue, firstJob).Err(); err != nil {
			return fmt.Errorf("valkey push: %w", err)
		}
	}

	return nil
}

// cancelDecidedSeriesGames is the maintenance pass for the up-front game
// model. Any series that is no longer active has its remaining pending games
// cancelled, and a series stuck behind a permanently failed game is abandoned
// so it cannot block the queue forever.
func (m *Matchmaker) cancelDecidedSeriesGames(ctx context.Context) error {
	// 1. Cancel the not-yet-run games of series that are already over. The
	// ordering gate only holds games back while their series is active, so a
	// pending game on a finished series is a zombie waiting to run — and its
	// result would then be recorded into a series that is already decided.
	// Both the job and its match are retired, so the game disappears from the
	// queue and from the open-match feed. This deliberately keys off the
	// series status rather than the win threshold: finalizeCompletedSeries has
	// already flipped decided series to 'completed' by the time this runs, so
	// a threshold test here would never fire.
	decided := `
		JOIN series_games sg ON sg.match_id = matches.match_id
		JOIN series s ON s.id = sg.series_id
		WHERE s.status <> 'active'
	`
	res, err := m.db.ExecContext(ctx, `
		UPDATE jobs SET status = 'cancelled', completed_at = NOW()
		FROM matches
		WHERE jobs.match_id = matches.match_id
		  AND jobs.status = 'pending'
		  AND EXISTS (`+decided+`)
	`)
	if err != nil {
		return fmt.Errorf("cancel decided series games: %w", err)
	}
	if _, err := m.db.ExecContext(ctx, `
		UPDATE matches SET status = 'cancelled'
		WHERE matches.status = 'pending'
		  AND EXISTS (`+decided+`)
	`); err != nil {
		return fmt.Errorf("cancel decided series matches: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("series-scheduler: cancelled %d leftover game(s) on decided series", n)
	}

	// 2. Abandon series a permanently dead game has wedged. A game whose job
	// failed or was cancelled never records a winner, and every later game in
	// the series waits on that winner — so the series can make no further
	// progress and the games behind the dead one would sit pending forever.
	// Only one game per active series is ever live (the gate keeps the rest
	// pending), so there is nothing still in flight to wait for.
	stuck, err := m.db.QueryContext(ctx, `
		SELECT s.id, s.bot_a_id, s.bot_b_id FROM series s
		WHERE s.status = 'active'
		  AND EXISTS (
		    SELECT 1 FROM series_games sg
		    JOIN jobs j ON j.match_id = sg.match_id
		    WHERE sg.series_id = s.id
		      AND sg.winner_id IS NULL
		      AND j.status IN ('failed', 'cancelled')
		  )
	`)
	if err != nil {
		return fmt.Errorf("query stuck series: %w", err)
	}
	defer stuck.Close()

	type stuckSeries struct {
		ID   int64
		BotA string
		BotB string
	}
	var stuckList []stuckSeries
	for stuck.Next() {
		var s stuckSeries
		if err := stuck.Scan(&s.ID, &s.BotA, &s.BotB); err != nil {
			continue
		}
		stuckList = append(stuckList, s)
	}
	stuck.Close()

	for _, s := range stuckList {
		var aWins, bWins int
		if err := m.db.QueryRowContext(ctx,
			`SELECT a_wins, b_wins FROM series WHERE id = $1`, s.ID).Scan(&aWins, &bWins); err != nil {
			continue
		}
		var winnerID *string
		switch {
		case aWins > bWins:
			winnerID = &s.BotA
		case bWins > aWins:
			winnerID = &s.BotB
		}
		if _, err := m.db.ExecContext(ctx, `
			UPDATE series SET status = 'completed', winner_id = $1, updated_at = NOW()
			WHERE id = $2 AND status = 'active'
		`, winnerID, s.ID); err != nil {
			log.Printf("series-scheduler: failed to finalize stuck series %d: %v", s.ID, err)
			continue
		}

		// Retire the games the abandoned series never played, in the same
		// breath: the moment the status flips, the ordering gate stops holding
		// them back, so leaving them pending would let them run as zombies.
		if _, err := m.db.ExecContext(ctx, `
			UPDATE jobs SET status = 'cancelled', completed_at = NOW()
			WHERE status = 'pending'
			  AND match_id IN (SELECT sg.match_id FROM series_games sg WHERE sg.series_id = $1)
		`, s.ID); err != nil {
			log.Printf("series-scheduler: failed to cancel stuck series %d games: %v", s.ID, err)
		}
		if _, err := m.db.ExecContext(ctx, `
			UPDATE matches SET status = 'cancelled'
			WHERE status = 'pending'
			  AND match_id IN (SELECT sg.match_id FROM series_games sg WHERE sg.series_id = $1)
		`, s.ID); err != nil {
			log.Printf("series-scheduler: failed to cancel stuck series %d matches: %v", s.ID, err)
		}

		log.Printf("series-scheduler: finalized stuck series %d (%d-%d)", s.ID, aWins, bWins)
	}

	return nil
}

// selectSeriesMap picks a map with varied characteristics per game number.
// Per §14.7: Game 1 = highest engagement, Game 2 = highest wall density,
// Game 3 = lowest wall density, Game 4 = most recent evolved, Game 5+ = random from pool.
// Returns (mapID, rows, cols, seed). Falls back to random seed if maps table is empty.
func (m *Matchmaker) selectSeriesMap(ctx context.Context, gameNum int, rng *rand.Rand) (string, int, int, int64) {
	var mapID string
	var gridW, gridH int
	var err error

	switch {
	case gameNum == 1:
		// Highest engagement (the "classic")
		err = m.db.QueryRowContext(ctx, `
			SELECT map_id, grid_width, grid_height FROM maps
			WHERE player_count = 2 AND status IN ('active', 'classic')
			ORDER BY engagement DESC NULLS LAST LIMIT 1
		`).Scan(&mapID, &gridW, &gridH)

	case gameNum == 2:
		// Highest wall density (corridors/chokepoints)
		err = m.db.QueryRowContext(ctx, `
			SELECT map_id, grid_width, grid_height FROM maps
			WHERE player_count = 2 AND status IN ('active', 'classic')
			ORDER BY wall_density DESC NULLS LAST LIMIT 1
		`).Scan(&mapID, &gridW, &gridH)

	case gameNum == 3:
		// Lowest wall density (open field)
		err = m.db.QueryRowContext(ctx, `
			SELECT map_id, grid_width, grid_height FROM maps
			WHERE player_count = 2 AND status IN ('active', 'classic')
			ORDER BY wall_density ASC NULLS LAST LIMIT 1
		`).Scan(&mapID, &gridW, &gridH)

	case gameNum == 4:
		// Most recent evolved map (untested terrain)
		// Join with programs table to find maps recently used in evolution
		err = m.db.QueryRowContext(ctx, `
			SELECT m.map_id, m.grid_width, m.grid_height
			FROM maps m
			WHERE m.player_count = 2 AND m.status IN ('active', 'classic')
			ORDER BY m.created_at DESC
			LIMIT 1
		`).Scan(&mapID, &gridW, &gridH)

	default:
		// Game 5+: Random from remaining pool
		err = m.db.QueryRowContext(ctx, `
			SELECT map_id, grid_width, grid_height FROM maps
			WHERE player_count = 2 AND status IN ('active', 'classic')
			ORDER BY RANDOM() LIMIT 1
		`).Scan(&mapID, &gridW, &gridH)
	}

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
	// Find top-20 active bots by rating (excluding crash-cooldown bots)
	rows, err := m.db.QueryContext(ctx, `
		SELECT bot_id FROM bots
		WHERE status = 'active' AND evolved = false
		  AND (cooldown_until IS NULL OR cooldown_until < NOW())
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
			  AND (b.cooldown_until IS NULL OR b.cooldown_until < NOW())
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

		var seriesID int64
		err = m.db.QueryRowContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, updated_at)
			VALUES ($1, $2, $3, 'active', 0, 0, $4, NOW())
			RETURNING id
		`, botAID, botBID, format, seasonID).Scan(&seriesID)
		if err != nil {
			log.Printf("series-scheduler: failed to create series (%s vs %s): %v", botAID, botBID, err)
			continue
		}

		if err := m.createSeriesGames(ctx, seriesID, botAID, botBID, format); err != nil {
			log.Printf("series-scheduler: failed to create games for series %d: %v", seriesID, err)
			continue
		}
		log.Printf("series-scheduler: created best-of-%d series %d: %s vs %s", format, seriesID, botAID, botBID)
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

	// 4. Reset prediction standings for new season (§14.9)
	//    Predictor stats reset but predictor identities persist
	_, err = tx.ExecContext(ctx, `
		UPDATE predictor_stats SET
			correct = 0,
			incorrect = 0,
			streak = 0,
			best_streak = 0,
			updated_at = NOW()
	`)
	if err != nil {
		return fmt.Errorf("reset predictor stats: %w", err)
	}

	// 5. Rotate map pool for new season (§14.9)
	//    Retire some maps to 'classic' status, promote probation maps
	//    This keeps the meta fresh while preserving beloved maps
	_, err = tx.ExecContext(ctx, `
		UPDATE maps SET status = 'classic', retired_at = NOW()
		WHERE player_count = 2
		  AND status = 'active'
		  AND created_at < NOW() - INTERVAL '90 days'
		  AND engagement < 0.3
	`)
	if err != nil {
		return fmt.Errorf("rotate map pool to classic: %w", err)
	}

	// 6. Reset auto-playlist contents for new season (§14.9)
	//    Clear auto-generated playlists but preserve curated ones
	_, err = tx.ExecContext(ctx, `
		DELETE FROM playlist_matches
		WHERE playlist_slug IN (
			SELECT slug FROM playlists WHERE is_auto = TRUE
		)
	`)
	if err != nil {
		return fmt.Errorf("reset auto-playlists: %w", err)
	}

	// 7. Apply decay to all non-retired bots (§14.9)
	//    Formula: new_mu = default + (current_mu - default) * decay_factor
	//    This pulls ratings toward 1500 but preserves relative ordering
	//    Backward compatibility: Old bots retain competitive edge via relative ordering
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

	log.Printf("season-reset: season %d (%s) complete — champion=%s, snapshot+resets+decay complete",
		seasonID, seasonName, championID)

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
		seasonID int64
		position int
		winners  []string
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
		var sfID int64
		err := m.db.QueryRowContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, bracket_round, bracket_position, updated_at)
			VALUES ($1, $2, 7, 'active', 0, 0, $3, 'semifinal', $4, NOW())
			RETURNING id
		`, pair.winners[0], pair.winners[1], pair.seasonID, pair.position).Scan(&sfID)
		if err != nil {
			log.Printf("series-scheduler: failed to create semifinal (%s vs %s): %v", pair.winners[0], pair.winners[1], err)
			continue
		}
		if err := m.createSeriesGames(ctx, sfID, pair.winners[0], pair.winners[1], 7); err != nil {
			log.Printf("series-scheduler: failed to create games for semifinal %d: %v", sfID, err)
			continue
		}
		log.Printf("series-scheduler: created championship semifinal %d: %s vs %s", sfID, pair.winners[0], pair.winners[1])
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
		var finalID int64
		err := m.db.QueryRowContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, bracket_round, bracket_position, updated_at)
			VALUES ($1, $2, 7, 'active', 0, 0, $3, 'final', 0, NOW())
			RETURNING id
		`, sfWinners[0].WinnerID, sfWinners[1].WinnerID, sfWinners[0].SeasonID).Scan(&finalID)
		if err != nil {
			log.Printf("series-scheduler: failed to create championship final: %v", err)
		} else if err := m.createSeriesGames(ctx, finalID, sfWinners[0].WinnerID, sfWinners[1].WinnerID, 7); err != nil {
			log.Printf("series-scheduler: failed to create games for final %d: %v", finalID, err)
		} else {
			log.Printf("series-scheduler: created championship final %d: %s vs %s", finalID, sfWinners[0].WinnerID, sfWinners[1].WinnerID)
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

	// Get top 8 active bots by rating (excluding crash-cooldown bots)
	rows, err := m.db.QueryContext(ctx, `
		SELECT bot_id FROM bots
		WHERE status = 'active'
		  AND (cooldown_until IS NULL OR cooldown_until < NOW())
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
		var qfID int64
		err := m.db.QueryRowContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, bracket_round, bracket_position, updated_at)
			VALUES ($1, $2, 7, 'active', 0, 0, $3, 'quarterfinal', $4, NOW())
			RETURNING id
		`, matchup.a, matchup.b, seasonID, matchup.position).Scan(&qfID)
		if err != nil {
			log.Printf("season-reset: failed to create championship quarterfinal series (%s vs %s): %v",
				matchup.a, matchup.b, err)
			continue
		}
		if err := m.createSeriesGames(ctx, qfID, matchup.a, matchup.b, 7); err != nil {
			log.Printf("season-reset: failed to create games for quarterfinal %d: %v", qfID, err)
			continue
		}
		log.Printf("season-reset: created championship quarterfinal series %d: %s vs %s (bo7)",
			qfID, matchup.a, matchup.b)
	}

	return nil
}

// tickFeaturedSeries creates best-of-5 weekly featured series on Friday at 20:00 UTC.
// It selects top 20 bots by rating and creates 4 rivalry pairs by ELO proximity.
// Plan §14.7: weekly featured matches between top-ranked bot rivalries.
func (m *Matchmaker) tickFeaturedSeries(ctx context.Context) {
	// Check if current time is Friday at 20:00 UTC (within a 1-hour window)
	now := time.Now().UTC()
	if now.Weekday() != time.Friday {
		return
	}
	hour := now.Hour()
	if hour < 20 || hour >= 21 {
		return // Only run during the 20:00-20:59 UTC window
	}

	// Check if featured series were already created this week
	var thisWeekCount int
	err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM series
		WHERE featured = TRUE
		  AND created_at >= date_trunc('week', NOW()) + INTERVAL '4 days' + INTERVAL '20 hours'
		  AND created_at < date_trunc('week', NOW()) + INTERVAL '4 days' + INTERVAL '21 hours'
	`).Scan(&thisWeekCount)
	if err != nil {
		log.Printf("featured-series: check existing error: %v", err)
		return
	}
	if thisWeekCount > 0 {
		return // Already created featured series this Friday
	}

	// Query top 20 active bots by rating (excluding crash-cooldown bots)
	rows, err := m.db.QueryContext(ctx, `
		SELECT bot_id, rating_mu FROM bots
		WHERE status = 'active'
		  AND (cooldown_until IS NULL OR cooldown_until < NOW())
		ORDER BY rating_mu DESC
		LIMIT 20
	`)
	if err != nil {
		log.Printf("featured-series: query top bots error: %v", err)
		return
	}
	defer rows.Close()

	type botRating struct {
		ID     string
		Rating float64
	}
	var topBots []botRating
	for rows.Next() {
		var br botRating
		if err := rows.Scan(&br.ID, &br.Rating); err != nil {
			log.Printf("featured-series: scan bot error: %v", err)
			continue
		}
		topBots = append(topBots, br)
	}

	if len(topBots) < 8 {
		log.Printf("featured-series: not enough active bots (%d), need at least 8", len(topBots))
		return
	}

	// Select 4 rivalry pairs by ELO proximity
	// Sort bots by rating
	sort.Slice(topBots, func(i, j int) bool {
		return topBots[i].Rating > topBots[j].Rating
	})

	// Create pairs by adjacent ratings (closest ELO proximity)
	// Pairing: #1-#2, #3-#4, #5-#6, #7-#8 (top rivalries)
	// Or use a more sophisticated pairing based on historical match count
	type botPair struct {
		A string
		B string
	}
	var pairs []botPair

	// Try to find actual rivalries first (bots that have played each other multiple times)
	for i := 0; i < len(topBots)-1; i++ {
		if len(pairs) >= 4 {
			break
		}
		for j := i + 1; j < len(topBots); j++ {
			// Check if these bots have a rivalry history (3+ matches)
			var matchCount int
			err := m.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM match_participants mp1
				JOIN match_participants mp2 ON mp1.match_id = mp2.match_id
				WHERE mp1.bot_id = $1 AND mp2.bot_id = $2
			`, topBots[i].ID, topBots[j].ID).Scan(&matchCount)
			if err == nil && matchCount >= 3 {
				pairs = append(pairs, botPair{A: topBots[i].ID, B: topBots[j].ID})
				// Mark these as used
				topBots[i].ID = ""
				topBots[j].ID = ""
				break
			}
		}
	}

	// Fill remaining slots with closest ELO pairs from remaining bots
	remaining := make([]botRating, 0)
	for _, br := range topBots {
		if br.ID != "" {
			remaining = append(remaining, br)
		}
	}

	// Pair adjacent bots by rating (closest ELO)
	for i := 0; i < len(remaining)-1 && len(pairs) < 4; i += 2 {
		if i+1 < len(remaining) {
			pairs = append(pairs, botPair{A: remaining[i].ID, B: remaining[i+1].ID})
		}
	}

	// Ensure we have exactly 4 pairs
	if len(pairs) > 4 {
		pairs = pairs[:4]
	} else if len(pairs) < 4 {
		// Fill with remaining adjacent pairs
		for i := len(pairs); i < 4 && i+1 < len(remaining); i++ {
			pairs = append(pairs, botPair{A: remaining[i].ID, B: remaining[i+1].ID})
		}
	}

	rng := rand.New(rand.NewSource(now.UnixNano()))

	// Get the active season ID (if any)
	var seasonID sql.NullInt64
	m.db.QueryRowContext(ctx,
		`SELECT id FROM seasons WHERE status = 'active' ORDER BY starts_at DESC LIMIT 1`).Scan(&seasonID)

	// Create bo5 featured series for each pair
	for i, pair := range pairs {
		if pair.A == "" || pair.B == "" {
			continue
		}

		// Randomize who is bot_a vs bot_b
		botAID, botBID := pair.A, pair.B
		if rng.Intn(2) == 0 {
			botAID, botBID = pair.B, pair.A
		}

		var featID int64
		err := m.db.QueryRowContext(ctx, `
			INSERT INTO series (bot_a_id, bot_b_id, format, status, a_wins, b_wins, season_id, featured, updated_at)
			VALUES ($1, $2, 5, 'active', 0, 0, $3, TRUE, NOW())
			RETURNING id
		`, botAID, botBID, seasonID).Scan(&featID)
		if err != nil {
			log.Printf("featured-series: failed to create series %d (%s vs %s): %v", i+1, botAID, botBID, err)
			continue
		}
		if err := m.createSeriesGames(ctx, featID, botAID, botBID, 5); err != nil {
			log.Printf("featured-series: failed to create games for series %d: %v", featID, err)
			continue
		}
		log.Printf("featured-series: created weekly featured bo5 series %d: %s vs %s", i+1, botAID, botBID)
	}

	log.Printf("featured-series: created %d weekly featured bo5 series for Friday %s", len(pairs), now.Format("2006-01-02"))
}

// isFriday20UTC checks if the given time is within the Friday 20:00 UTC window.
func isFriday20UTC(t time.Time) bool {
	// Convert to UTC
	utc := t.UTC()
	// Check if it's Friday (weekday 5)
	if utc.Weekday() != time.Friday {
		return false
	}
	// Check if hour is 20 (8 PM UTC)
	return utc.Hour() == 20
}

// ratingDistance returns the absolute difference in ratings between two bots.
func ratingDistance(r1, r2 float64) float64 {
	return math.Abs(r1 - r2)
}
