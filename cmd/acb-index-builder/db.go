package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// BotData represents a bot for the index
type BotData struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	OwnerID          string    `json:"owner_id"`
	Description      string    `json:"description,omitempty"`
	Rating           float64   `json:"rating"`
	RatingDeviation  float64   `json:"rating_deviation"`
	RatingVolatility float64   `json:"rating_volatility"`
	MatchesPlayed    int       `json:"matches_played"`
	MatchesWon       int       `json:"matches_won"`
	HealthStatus     string    `json:"health_status"`
	Evolved          bool      `json:"evolved"`
	Island           string    `json:"island,omitempty"`
	Generation       int       `json:"generation,omitempty"`
	Archetype        string    `json:"archetype,omitempty"`
	ParentIDs        []string  `json:"parent_ids,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// MatchData represents a match for the index
type MatchData struct {
	ID           string             `json:"id"`
	MapID        string             `json:"map_id"`
	MapName      string             `json:"map_name,omitempty"`
	WinnerID     string             `json:"winner_id,omitempty"`
	TurnCount    int                `json:"turn_count"`
	EndCondition string             `json:"end_condition"`
	Participants []ParticipantData  `json:"participants"`
	CreatedAt    time.Time          `json:"created_at"`
	CompletedAt  time.Time          `json:"completed_at"`
	PlayedAt     time.Time          `json:"played_at"`
}

// ParticipantData represents a bot in a match with pre-match rating
type ParticipantData struct {
	BotID          string  `json:"bot_id"`
	PlayerSlot     int     `json:"player_slot"`
	Score          int     `json:"score"`
	Won            bool    `json:"won"`
	PreMatchRating float64 `json:"pre_match_rating,omitempty"`
	Evolved        bool    `json:"evolved,omitempty"`
}

// RatingHistoryEntry represents a rating history point
type RatingHistoryEntry struct {
	BotID      string    `json:"bot_id"`
	MatchID    string    `json:"match_id"`
	Rating     float64   `json:"rating"`
	RecordedAt time.Time `json:"recorded_at"`
}

// SeriesGameData represents one game within a series
type SeriesGameData struct {
	MatchID     string     `json:"match_id"`
	GameNum     int        `json:"game_number"`
	WinnerID    string     `json:"winner_id,omitempty"`
	WinnerSlot  *int       `json:"winner_slot"`
	Turns       int        `json:"turns,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// SeriesData represents a series for the index, enriched with bot names and games.
type SeriesData struct {
	ID              int64            `json:"id"`
	BotAID          string           `json:"bot1_id"`
	BotBID          string           `json:"bot2_id"`
	BotAName        string           `json:"bot1_name"`
	BotBName        string           `json:"bot2_name"`
	Format          int              `json:"best_of"`
	AWins           int              `json:"bot1_wins"`
	BWins           int              `json:"bot2_wins"`
	Status          string           `json:"status"`
	WinnerID        string           `json:"winner_id,omitempty"`
	BracketRound    string           `json:"bracket_round,omitempty"`
	BracketPosition int              `json:"bracket_position,omitempty"`
	ScheduledAt     *time.Time       `json:"scheduled_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Games           []SeriesGameData `json:"games"`
}

// SeasonSnapshotData represents a bot's end-of-season snapshot
type SeasonSnapshotData struct {
	BotID   string  `json:"bot_id"`
	BotName string  `json:"bot_name"`
	Rating  float64 `json:"rating"`
	Rank    int     `json:"rank"`
	Wins    int     `json:"wins"`
	Losses  int     `json:"losses"`
}

// ChampionshipSeries is a lightweight series summary for bracket display on the season page.
type ChampionshipSeries struct {
	ID              int64            `json:"id"`
	BotAID          string           `json:"bot1_id"`
	BotBID          string           `json:"bot2_id"`
	BotAName        string           `json:"bot1_name"`
	BotBName        string           `json:"bot2_name"`
	Format          int              `json:"best_of"`
	AWins           int              `json:"bot1_wins"`
	BWins           int              `json:"bot2_wins"`
	Status          string           `json:"status"`
	WinnerID        string           `json:"winner_id,omitempty"`
	Round           string           `json:"round"`
	BracketPosition int              `json:"bracket_position"`
	Games           []SeriesGameData `json:"games"`
}

// SeasonData represents a season for the index, enriched with champion name, match count, and snapshots.
type SeasonData struct {
	ID                  int64                `json:"id"`
	Name                string               `json:"name"`
	Theme               string               `json:"theme,omitempty"`
	RulesVer            string               `json:"rules_version"`
	Status              string               `json:"status"`
	ChampionID          string               `json:"champion_id,omitempty"`
	ChampionName        string               `json:"champion_name,omitempty"`
	StartsAt            time.Time            `json:"starts_at"`
	EndsAt              time.Time            `json:"ends_at,omitempty"`
	TotalMatches        int                  `json:"total_matches"`
	CreatedAt           time.Time            `json:"created_at"`
	Snapshots           []SeasonSnapshotData `json:"final_snapshot"`
	ChampionshipBracket []ChampionshipSeries `json:"championship_bracket,omitempty"`
}

// PredictionData represents a prediction for the index
type PredictionData struct {
	ID          int64      `json:"id"`
	MatchID     string     `json:"match_id"`
	PredictorID string     `json:"predictor_id"`
	PredictedBot string    `json:"predicted_bot"`
	Correct     *bool      `json:"correct,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// PredictorStats represents predictor statistics
type PredictorStats struct {
	PredictorID string `json:"predictor_id"`
	Correct     int    `json:"correct"`
	Incorrect   int    `json:"incorrect"`
	Streak      int    `json:"streak"`
	BestStreak  int    `json:"best_streak"`
}

// MapData represents a map for the index
type MapData struct {
	MapID       string    `json:"map_id"`
	PlayerCount int       `json:"player_count"`
	Status      string    `json:"status"`
	Engagement  float64   `json:"engagement"`
	WallDensity float64   `json:"wall_density"`
	EnergyCount int       `json:"energy_count"`
	GridWidth   int       `json:"grid_width"`
	GridHeight  int       `json:"grid_height"`
	CreatedAt   time.Time `json:"created_at"`
}

// IndexData contains all data needed for index generation
type IndexData struct {
	GeneratedAt    time.Time
	Bots           []BotData
	Matches        []MatchData
	RatingHistory  []RatingHistoryEntry
	Series         []SeriesData
	Seasons        []SeasonData
	Predictions    []PredictionData
	PredictorStats []PredictorStats
	Maps           []MapData
	TopPredictors  []PredictorStats
}

// fetchAllData retrieves all data from PostgreSQL for index generation
func fetchAllData(ctx context.Context, db *sql.DB) (*IndexData, error) {
	data := &IndexData{
		GeneratedAt: time.Now().UTC(),
	}

	var err error
	if data.Bots, err = fetchBots(ctx, db); err != nil {
		return nil, err
	}
	if data.Matches, err = fetchMatches(ctx, db); err != nil {
		return nil, err
	}
	if data.RatingHistory, err = fetchRatingHistory(ctx, db); err != nil {
		return nil, err
	}
	if data.Series, err = fetchSeries(ctx, db); err != nil {
		return nil, err
	}
	if data.Seasons, err = fetchSeasons(ctx, db); err != nil {
		return nil, err
	}
	if data.Predictions, err = fetchPredictions(ctx, db); err != nil {
		return nil, err
	}
	if data.PredictorStats, err = fetchPredictorStats(ctx, db); err != nil {
		return nil, err
	}
	if data.Maps, err = fetchMaps(ctx, db); err != nil {
		return nil, err
	}

	data.TopPredictors = computeTopPredictors(data.PredictorStats)

	return data, nil
}

func fetchBots(ctx context.Context, db *sql.DB) ([]BotData, error) {
	query := `
		SELECT bot_id, name, owner, description,
		       rating_mu, rating_phi, rating_sigma,
		       0, 0, status,
		       evolved, island, generation,
		       COALESCE(archetype, ''), COALESCE(parent_ids, '[]'::jsonb),
		       created_at, COALESCE(last_active, created_at)
		FROM bots
		WHERE status != 'retired'
		ORDER BY rating_mu DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []BotData
	for rows.Next() {
		var b BotData
		var desc, island sql.NullString
		var gen sql.NullInt64
		var parentIDsJSON []byte

		err := rows.Scan(
			&b.ID, &b.Name, &b.OwnerID, &desc,
			&b.Rating, &b.RatingDeviation, &b.RatingVolatility,
			&b.MatchesPlayed, &b.MatchesWon, &b.HealthStatus,
			&b.Evolved, &island, &gen,
			&b.Archetype, &parentIDsJSON,
			&b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if desc.Valid {
			b.Description = desc.String
		}
		if island.Valid {
			b.Island = island.String
		}
		if gen.Valid {
			b.Generation = int(gen.Int64)
		}
		if len(parentIDsJSON) > 0 {
			json.Unmarshal(parentIDsJSON, &b.ParentIDs)
		}

		bots = append(bots, b)
	}

	for i := range bots {
		mp, mw, err := getBotMatchStats(ctx, db, bots[i].ID)
		if err != nil {
			return nil, err
		}
		bots[i].MatchesPlayed = mp
		bots[i].MatchesWon = mw
	}

	return bots, nil
}

func getBotMatchStats(ctx context.Context, db *sql.DB, botID string) (played, won int, err error) {
	query := `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE mp.player_slot = m.winner)
		FROM match_participants mp
		JOIN matches m ON mp.match_id = m.match_id
		WHERE mp.bot_id = $1 AND m.status = 'completed'
	`
	err = db.QueryRowContext(ctx, query, botID).Scan(&played, &won)
	return
}

func fetchMatches(ctx context.Context, db *sql.DB) ([]MatchData, error) {
	query := `
		SELECT m.match_id, m.map_id, m.winner, m.turn_count, m.condition,
		       m.created_at, m.completed_at,
		       COALESCE(
		           json_agg(
		               json_build_object(
		                   'bot_id', mp.bot_id,
		                   'player_slot', mp.player_slot,
		                   'score', mp.score,
		                   'won', mp.player_slot = m.winner,
		                   'pre_match_rating', COALESCE(
		                       (SELECT rh.rating FROM rating_history rh
		                        WHERE rh.bot_id = mp.bot_id AND rh.match_id = m.match_id
		                        LIMIT 1), 0),
		                   'evolved', COALESCE(
		                       (SELECT b.evolved FROM bots b WHERE b.bot_id = mp.bot_id), false)
		           )
		           ORDER BY mp.player_slot
		           ) FILTER (WHERE mp.bot_id IS NOT NULL),
		           '[]'::json
		       ) as participants
		FROM matches m
		LEFT JOIN match_participants mp ON m.match_id = mp.match_id
		WHERE m.status = 'completed'
		GROUP BY m.match_id, m.map_id, m.winner, m.turn_count, m.condition,
		         m.created_at, m.completed_at
		ORDER BY m.completed_at DESC
		LIMIT 1000
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []MatchData
	for rows.Next() {
		var m MatchData
		var winnerID sql.NullString
		var participantsJSON []byte

		err := rows.Scan(
			&m.ID, &m.MapID, &winnerID, &m.TurnCount, &m.EndCondition,
			&m.CreatedAt, &m.CompletedAt, &participantsJSON,
		)
		if err != nil {
			return nil, err
		}

		if winnerID.Valid {
			m.WinnerID = winnerID.String
		}
		if err := json.Unmarshal(participantsJSON, &m.Participants); err != nil {
			return nil, err
		}

		// PlayedAt is used for weekly filtering in blog/stats generation.
		// CompletedAt is the authoritative timestamp; fall back to CreatedAt.
		if !m.CompletedAt.IsZero() {
			m.PlayedAt = m.CompletedAt
		} else {
			m.PlayedAt = m.CreatedAt
		}

		matches = append(matches, m)
	}

	return matches, nil
}

func fetchRatingHistory(ctx context.Context, db *sql.DB) ([]RatingHistoryEntry, error) {
	query := `
		SELECT bot_id, match_id, rating, recorded_at
		FROM rating_history
		ORDER BY recorded_at DESC
		LIMIT 10000
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []RatingHistoryEntry
	for rows.Next() {
		var e RatingHistoryEntry
		if err := rows.Scan(&e.BotID, &e.MatchID, &e.Rating, &e.RecordedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, nil
}

func fetchSeries(ctx context.Context, db *sql.DB) ([]SeriesData, error) {
	query := `
		SELECT s.id, s.bot_a_id, s.bot_b_id,
		       ba.name, bb.name,
		       s.format, s.a_wins, s.b_wins, s.status, s.winner_id,
		       COALESCE(s.bracket_round, ''), COALESCE(s.bracket_position, 0),
		       s.created_at, s.updated_at
		FROM series s
		JOIN bots ba ON s.bot_a_id = ba.bot_id
		JOIN bots bb ON s.bot_b_id = bb.bot_id
		ORDER BY s.created_at DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	for rows.Next() {
		var s SeriesData
		var winnerID sql.NullString

		err := rows.Scan(
			&s.ID, &s.BotAID, &s.BotBID,
			&s.BotAName, &s.BotBName,
			&s.Format, &s.AWins, &s.BWins, &s.Status, &winnerID,
			&s.BracketRound, &s.BracketPosition,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if winnerID.Valid {
			s.WinnerID = winnerID.String
		}
		series = append(series, s)
	}

	for i := range series {
		games, err := fetchSeriesGames(ctx, db, series[i].ID)
		if err != nil {
			return nil, err
		}
		series[i].Games = games
	}

	return series, nil
}

func fetchSeriesGames(ctx context.Context, db *sql.DB, seriesID int64) ([]SeriesGameData, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT sg.match_id, sg.game_num, sg.winner_id,
		       COALESCE(m.turn_count, 0), m.completed_at,
		       CASE WHEN sg.winner_id IS NOT NULL THEN
		           (SELECT mp.player_slot FROM match_participants mp
		            WHERE mp.match_id = sg.match_id AND mp.bot_id = sg.winner_id)
		       END
		FROM series_games sg
		LEFT JOIN matches m ON sg.match_id = m.match_id
		WHERE sg.series_id = $1
		ORDER BY sg.game_num
	`, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []SeriesGameData
	for rows.Next() {
		var g SeriesGameData
		var winnerID sql.NullString
		var winnerSlot sql.NullInt64
		var turns sql.NullInt64
		var completedAt sql.NullTime

		err := rows.Scan(&g.MatchID, &g.GameNum, &winnerID, &turns, &completedAt, &winnerSlot)
		if err != nil {
			return nil, err
		}

		if winnerID.Valid {
			g.WinnerID = winnerID.String
		}
		if winnerSlot.Valid {
			slot := int(winnerSlot.Int64)
			g.WinnerSlot = &slot
		}
		if turns.Valid && turns.Int64 > 0 {
			g.Turns = int(turns.Int64)
		}
		if completedAt.Valid {
			g.CompletedAt = &completedAt.Time
		}
		games = append(games, g)
	}

	return games, nil
}

func fetchSeasons(ctx context.Context, db *sql.DB) ([]SeasonData, error) {
	query := `
		SELECT s.id, s.name, s.theme, s.rules_version, s.status,
		       s.champion_id, b.name,
		       s.starts_at, s.ends_at, s.created_at
		FROM seasons s
		LEFT JOIN bots b ON s.champion_id = b.bot_id
		ORDER BY s.starts_at DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []SeasonData
	for rows.Next() {
		var s SeasonData
		var theme, championID, championName sql.NullString
		var endsAt sql.NullTime

		err := rows.Scan(
			&s.ID, &s.Name, &theme, &s.RulesVer, &s.Status,
			&championID, &championName,
			&s.StartsAt, &endsAt, &s.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if theme.Valid {
			s.Theme = theme.String
		}
		if championID.Valid {
			s.ChampionID = championID.String
		}
		if championName.Valid {
			s.ChampionName = championName.String
		}
		if endsAt.Valid {
			s.EndsAt = endsAt.Time
		}
		seasons = append(seasons, s)
	}

	// Enrich each season with match count, snapshots, and championship bracket
	for i := range seasons {
		seasons[i].TotalMatches, _ = getSeasonMatchCount(ctx, db, seasons[i].ID)
		snapshots, err := fetchSeasonSnapshots(ctx, db, seasons[i].ID)
		if err == nil && len(snapshots) > 0 {
			seasons[i].Snapshots = snapshots
		}
		bracket, err := fetchChampionshipBracket(ctx, db, seasons[i].ID)
		if err == nil && len(bracket) > 0 {
			seasons[i].ChampionshipBracket = bracket
		}
	}

	return seasons, nil
}

func getSeasonMatchCount(ctx context.Context, db *sql.DB, seasonID int64) (int, error) {
	// Count matches from series in this season
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT sg.match_id)
		FROM series_games sg
		JOIN series s ON sg.series_id = s.id
		WHERE s.season_id = $1 AND sg.match_id IS NOT NULL
	`, seasonID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func fetchSeasonSnapshots(ctx context.Context, db *sql.DB, seasonID int64) ([]SeasonSnapshotData, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ss.bot_id, b.name, ss.rating, ss.rank, ss.wins, ss.losses
		FROM season_snapshots ss
		JOIN bots b ON ss.bot_id = b.bot_id
		WHERE ss.season_id = $1
		ORDER BY ss.rank
	`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []SeasonSnapshotData
	for rows.Next() {
		var snap SeasonSnapshotData
		if err := rows.Scan(&snap.BotID, &snap.BotName, &snap.Rating, &snap.Rank, &snap.Wins, &snap.Losses); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

func fetchChampionshipBracket(ctx context.Context, db *sql.DB, seasonID int64) ([]ChampionshipSeries, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.id, s.bot_a_id, ba.name, s.bot_b_id, bb.name,
		       s.format, s.a_wins, s.b_wins, s.status, s.winner_id,
		       COALESCE(s.bracket_round, 'quarterfinal'), COALESCE(s.bracket_position, 0)
		FROM series s
		JOIN bots ba ON s.bot_a_id = ba.bot_id
		JOIN bots bb ON s.bot_b_id = bb.bot_id
		WHERE s.season_id = $1 AND s.bracket_round IS NOT NULL
		ORDER BY
		    CASE s.bracket_round
		        WHEN 'quarterfinal' THEN 0
		        WHEN 'semifinal' THEN 1
		        WHEN 'final' THEN 2
		    END,
		    s.bracket_position
	`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChampionshipSeries
	for rows.Next() {
		var cs ChampionshipSeries
		var winnerID sql.NullString
		if err := rows.Scan(&cs.ID, &cs.BotAID, &cs.BotAName, &cs.BotBID, &cs.BotBName,
			&cs.Format, &cs.AWins, &cs.BWins, &cs.Status, &winnerID,
			&cs.Round, &cs.BracketPosition); err != nil {
			return nil, err
		}
		if winnerID.Valid {
			cs.WinnerID = winnerID.String
		}
		result = append(result, cs)
	}

	// Fetch games for each series
	for i := range result {
		games, err := fetchSeriesGames(ctx, db, result[i].ID)
		if err == nil {
			result[i].Games = games
		}
	}

	return result, nil
}

func fetchPredictions(ctx context.Context, db *sql.DB) ([]PredictionData, error) {
	query := `
		SELECT id, match_id, predictor_id, predicted_bot, correct, created_at, resolved_at
		FROM predictions
		ORDER BY created_at DESC
		LIMIT 1000
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var predictions []PredictionData
	for rows.Next() {
		var p PredictionData
		var correct sql.NullBool
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&p.ID, &p.MatchID, &p.PredictorID, &p.PredictedBot,
			&correct, &p.CreatedAt, &resolvedAt,
		)
		if err != nil {
			return nil, err
		}

		if correct.Valid {
			p.Correct = &correct.Bool
		}
		if resolvedAt.Valid {
			p.ResolvedAt = &resolvedAt.Time
		}
		predictions = append(predictions, p)
	}

	return predictions, nil
}

func fetchPredictorStats(ctx context.Context, db *sql.DB) ([]PredictorStats, error) {
	query := `
		SELECT predictor_id, correct, incorrect, streak, best_streak
		FROM predictor_stats
		ORDER BY (correct::float / NULLIF(correct + incorrect, 0)) DESC NULLS LAST
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []PredictorStats
	for rows.Next() {
		var s PredictorStats
		if err := rows.Scan(&s.PredictorID, &s.Correct, &s.Incorrect, &s.Streak, &s.BestStreak); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

func fetchMaps(ctx context.Context, db *sql.DB) ([]MapData, error) {
	query := `
		SELECT map_id, player_count, status, engagement, wall_density,
		       energy_count, grid_width, grid_height, created_at
		FROM maps
		WHERE status IN ('active', 'probation', 'classic')
		ORDER BY engagement DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var maps []MapData
	for rows.Next() {
		var m MapData
		if err := rows.Scan(
			&m.MapID, &m.PlayerCount, &m.Status, &m.Engagement, &m.WallDensity,
			&m.EnergyCount, &m.GridWidth, &m.GridHeight, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		maps = append(maps, m)
	}

	return maps, nil
}

func computeTopPredictors(stats []PredictorStats) []PredictorStats {
	if len(stats) > 50 {
		return stats[:50]
	}
	return stats
}

// persistPlaylists writes generated playlist definitions and their match associations
// to the playlists and playlist_matches tables. It uses upsert semantics so playlists
// are updated in place without creating duplicates.
func persistPlaylists(ctx context.Context, db *sql.DB, playlists []persistedPlaylist) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, pl := range playlists {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO playlists (slug, title, description, category, is_auto, updated_at)
			VALUES ($1, $2, $3, $4, TRUE, NOW())
			ON CONFLICT (slug) DO UPDATE SET
				title = EXCLUDED.title,
				description = EXCLUDED.description,
				category = EXCLUDED.category,
				updated_at = NOW()
		`, pl.Slug, pl.Title, pl.Description, pl.Category)
		if err != nil {
			return fmt.Errorf("persist playlist %s: %w", pl.Slug, err)
		}

		// Delete old match associations and re-insert
		_, err = tx.ExecContext(ctx, `DELETE FROM playlist_matches WHERE playlist_slug = $1`, pl.Slug)
		if err != nil {
			return fmt.Errorf("clear playlist_matches for %s: %w", pl.Slug, err)
		}

		for _, pm := range pl.Matches {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO playlist_matches (playlist_slug, match_id, sort_order, curation_tag)
				VALUES ($1, $2, $3, $4)
			`, pl.Slug, pm.MatchID, pm.SortOrder, pm.CurationTag)
			if err != nil {
				return fmt.Errorf("persist playlist_match %s/%s: %w", pl.Slug, pm.MatchID, err)
			}
		}
	}

	return tx.Commit()
}

type persistedPlaylist struct {
	Slug        string
	Title       string
	Description string
	Category    string
	Matches     []persistedPlaylistMatch
}

type persistedPlaylistMatch struct {
	MatchID     string
	SortOrder   int
	CurationTag string
}
