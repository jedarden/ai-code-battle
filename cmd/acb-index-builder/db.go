package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
}

// RatingHistoryEntry represents a rating history point
type RatingHistoryEntry struct {
	BotID      string    `json:"bot_id"`
	MatchID    string    `json:"match_id"`
	Rating     float64   `json:"rating"`
	RecordedAt time.Time `json:"recorded_at"`
}

// SeriesData represents a series for the index
type SeriesData struct {
	ID        int64     `json:"id"`
	BotAID    string    `json:"bot_a_id"`
	BotBID    string    `json:"bot_b_id"`
	Format    int       `json:"format"`
	AWins     int       `json:"a_wins"`
	BWins     int       `json:"b_wins"`
	Status    string    `json:"status"`
	WinnerID  string    `json:"winner_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SeasonData represents a season for the index
type SeasonData struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Theme       string    `json:"theme,omitempty"`
	RulesVer    string    `json:"rules_version"`
	ChampionID  string    `json:"champion_id,omitempty"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// PredictionData represents a prediction for the index
type PredictionData struct {
	ID           int64      `json:"id"`
	MatchID      string     `json:"match_id"`
	PredictorID  string     `json:"predictor_id"`
	PredictedBot string     `json:"predicted_bot"`
	Correct      *bool      `json:"correct,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
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
	MapID        string    `json:"map_id"`
	PlayerCount  int       `json:"player_count"`
	Status       string    `json:"status"`
	Engagement   float64   `json:"engagement"`
	WallDensity  float64   `json:"wall_density"`
	EnergyCount  int       `json:"energy_count"`
	GridWidth    int       `json:"grid_width"`
	GridHeight   int       `json:"grid_height"`
	CreatedAt    time.Time `json:"created_at"`
}

// IndexData contains all data needed for index generation
type IndexData struct {
	GeneratedAt     time.Time
	Bots            []BotData
	Matches         []MatchData
	RatingHistory   []RatingHistoryEntry
	Series          []SeriesData
	Seasons         []SeasonData
	Predictions     []PredictionData
	PredictorStats  []PredictorStats
	Maps            []MapData
	TopPredictors   []PredictorStats
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

	// Get top predictors (sorted by accuracy)
	data.TopPredictors = computeTopPredictors(data.PredictorStats)

	return data, nil
}

func fetchBots(ctx context.Context, db *sql.DB) ([]BotData, error) {
	query := `
		SELECT bot_id, name, owner, description,
		       rating_mu, rating_phi, rating_sigma,
		       0, 0, status,
		       evolved, island, generation,
		       COALESCE(archetype, ''), COALESCE(parent_ids, '[]'::json),
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

	// Calculate matches played and won from match_participants
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
		SELECT COUNT(*), COUNT(*) FILTER (WHERE mp.bot_id = m.winner)
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
		                   'won', mp.bot_id = m.winner
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
		SELECT id, bot_a_id, bot_b_id, format, a_wins, b_wins, status, winner_id, created_at, updated_at
		FROM series
		ORDER BY created_at DESC
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
			&s.ID, &s.BotAID, &s.BotBID, &s.Format, &s.AWins, &s.BWins,
			&s.Status, &winnerID, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if winnerID.Valid {
			s.WinnerID = winnerID.String
		}
		series = append(series, s)
	}

	return series, nil
}

func fetchSeasons(ctx context.Context, db *sql.DB) ([]SeasonData, error) {
	query := `
		SELECT id, name, theme, rules_version, champion_id, starts_at, ends_at, created_at
		FROM seasons
		ORDER BY starts_at DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []SeasonData
	for rows.Next() {
		var s SeasonData
		var theme, championID sql.NullString
		var endsAt sql.NullTime

		err := rows.Scan(
			&s.ID, &s.Name, &theme, &s.RulesVer, &championID,
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
		if endsAt.Valid {
			s.EndsAt = endsAt.Time
		}
		seasons = append(seasons, s)
	}

	return seasons, nil
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
	// Sort by accuracy (correct / total)
	// Already sorted in query, just return top 50
	if len(stats) > 50 {
		return stats[:50]
	}
	return stats
}
