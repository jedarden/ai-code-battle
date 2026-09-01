package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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
	DebugPublic      bool      `json:"debug_public"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// MatchData represents a match for the index
type MatchData struct {
	ID           string            `json:"id"`
	MapID        string            `json:"map_id"`
	MapName      string            `json:"map_name,omitempty"`
	WinnerID     string            `json:"winner_id,omitempty"`
	TurnCount    int               `json:"turn_count"`
	EndCondition string            `json:"end_condition"`
	CombatTurns  int               `json:"combat_turns"` // turns with ≥1 enemy-kill combat death
	Participants []ParticipantData `json:"participants"`
	CreatedAt    time.Time         `json:"created_at"`
	CompletedAt  time.Time         `json:"completed_at"`
	PlayedAt     time.Time         `json:"played_at"`
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
	MapID       string          `json:"map_id"`
	PlayerCount int             `json:"player_count"`
	Status      string          `json:"status"`
	Engagement  float64         `json:"engagement"`
	WallDensity float64         `json:"wall_density"`
	EnergyCount int             `json:"energy_count"`
	GridWidth   int             `json:"grid_width"`
	GridHeight  int             `json:"grid_height"`
	NetVotes    int             `json:"net_votes"` // Sum of votes from map_votes table
	CreatedAt   time.Time       `json:"created_at"`
	RawJSON     json.RawMessage `json:"-"`
}

// OpenPredictionMatch represents a pending match open for predictions
type OpenPredictionMatch struct {
	MatchID          string    `json:"match_id"`
	BotAID           string    `json:"bot_a"`
	BotBID           string    `json:"bot_b"`
	BotAName         string    `json:"bot_a_name"`
	BotBName         string    `json:"bot_b_name"`
	ARating          float64   `json:"a_rating"`
	BRating          float64   `json:"b_rating"`
	AEvolved         bool      `json:"a_evolved"`
	BEvolved         bool      `json:"b_evolved"`
	CreatedAt        time.Time `json:"created_at"`
	IsSeriesMatch    bool      `json:"is_series_match"`
	HeadToHeadRecord *string   `json:"head_to_head_record,omitempty"`
}

// FeedbackEntry represents a community replay annotation from §13.6.
type FeedbackEntry struct {
	FeedbackID string    `json:"feedback_id"`
	MatchID    string    `json:"match_id"`
	Turn       int       `json:"turn"`
	Type       string    `json:"type"`
	Body       string    `json:"body"`
	Author     string    `json:"author"`
	Upvotes    int       `json:"upvotes"`
	CreatedAt  time.Time `json:"created_at"`
}

// IndexData contains all data needed for index generation
type IndexData struct {
	GeneratedAt           time.Time
	Bots                  []BotData
	Matches               []MatchData
	RatingHistory         []RatingHistoryEntry
	Series                []SeriesData
	Seasons               []SeasonData
	Predictions           []PredictionData
	PredictorStats        []PredictorStats
	Maps                  []MapData
	TopPredictors         []PredictorStats
	OpenPredictionMatches []OpenPredictionMatch
	Feedback              []FeedbackEntry
	EvolutionMeta         *EvolutionMeta
	Lineage               []LineageNode
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
	if data.OpenPredictionMatches, err = fetchOpenPredictions(ctx, db); err != nil {
		return nil, err
	}

	if data.Feedback, err = fetchFeedback(ctx, db); err != nil {
		return nil, err
	}

	// Evolution data (may be missing if evolver is not running)
	data.EvolutionMeta, _ = fetchEvolutionMeta(ctx, db)
	data.Lineage, _ = fetchLineage(ctx, db)
	if data.EvolutionMeta != nil && data.EvolutionMeta.Generation > 0 {
		slog.Info("Evolution system running",
			"generation", data.EvolutionMeta.Generation,
			"promoted_today", data.EvolutionMeta.PromotedToday,
			"total_promoted", data.EvolutionMeta.TotalPromoted,
			"islands", len(data.EvolutionMeta.IslandPopulations))
	} else {
		slog.Info("Evolution system not initialized or not running")
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
		       debug_public,
		       created_at, COALESCE(last_active, created_at)
		FROM bots
		WHERE status != 'retired'
		ORDER BY rating_mu DESC
		LIMIT 2000
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
			&b.DebugPublic,
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

	// Fetch match stats for all bots in a single query to avoid O(n) query loop
	// that would cause 10,000+ separate database calls (N+1 query problem)
	// This fixes the CrashLoopBackOff issue caused by OOMKill during fetchBots
	botMatchStats := make(map[string][2]int) // bot_id -> [matches_played, matches_won]
	statsRows, err := db.QueryContext(ctx, `
		SELECT mp.bot_id,
		       COUNT(*) as matches_played,
		       COUNT(*) FILTER (WHERE mp.player_slot = m.winner) as matches_won
		FROM match_participants mp
		JOIN matches m ON mp.match_id = m.match_id
		WHERE m.status = 'completed'
		GROUP BY mp.bot_id
		LIMIT 20000
	`)
	if err != nil {
		return nil, fmt.Errorf("query bot match stats: %w", err)
	}
	defer statsRows.Close()

	for statsRows.Next() {
		var botID string
		var played, won int
		if err := statsRows.Scan(&botID, &played, &won); err != nil {
			return nil, fmt.Errorf("scan bot match stats: %w", err)
		}
		botMatchStats[botID] = [2]int{played, won}
	}

	// Apply the stats to each bot
	for i := range bots {
		stats, ok := botMatchStats[bots[i].ID]
		if ok {
			bots[i].MatchesPlayed = stats[0]
			bots[i].MatchesWon = stats[1]
		}
		// If bot has no completed matches, stats remain at 0 (already initialized)
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
		       COALESCE(m.combat_turns, 0),
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
		         m.combat_turns, m.created_at, m.completed_at
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
			&m.CombatTurns,
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
		LIMIT 5000
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
		LIMIT 1000
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []SeriesData
	var seriesIDs []int64
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
		seriesIDs = append(seriesIDs, s.ID)
	}

	if len(series) == 0 {
		return series, nil
	}

	// Fetch all games for all series in a single batch query to avoid N+1 problem
	// that causes OOMKill (1000 separate queries → 1 batch query)
	gamesMap := make(map[int64][]SeriesGameData)
	if len(seriesIDs) > 0 {
		placeholders := make([]string, len(seriesIDs))
		args := make([]interface{}, len(seriesIDs))
		for i, id := range seriesIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}
		query := fmt.Sprintf(`
			SELECT sg.series_id, sg.match_id, sg.game_num, sg.winner_id,
			       COALESCE(m.turn_count, 0), m.completed_at,
			       CASE WHEN sg.winner_id IS NOT NULL THEN
			           (SELECT mp.player_slot FROM match_participants mp
			            WHERE mp.match_id = sg.match_id AND mp.bot_id = sg.winner_id)
			       END
			FROM series_games sg
			LEFT JOIN matches m ON sg.match_id = m.match_id
			WHERE sg.series_id IN (%s)
			ORDER BY sg.series_id, sg.game_num
			LIMIT 10000
		`, strings.Join(placeholders, ", "))

		gamesRows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("query series games: %w", err)
		}
		defer gamesRows.Close()

		for gamesRows.Next() {
			var g SeriesGameData
			var seriesID int64
			var winnerID sql.NullString
			var winnerSlot sql.NullInt64
			var turns sql.NullInt64
			var completedAt sql.NullTime

			err := gamesRows.Scan(&seriesID, &g.MatchID, &g.GameNum, &winnerID, &turns, &completedAt, &winnerSlot)
			if err != nil {
				return nil, fmt.Errorf("scan series game: %w", err)
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
			gamesMap[seriesID] = append(gamesMap[seriesID], g)
		}
	}

	// Assign games to each series
	for i := range series {
		series[i].Games = gamesMap[series[i].ID]
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
		LIMIT 100
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
		LIMIT 100
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
		LIMIT 500
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
		LIMIT 64
	`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChampionshipSeries
	var seriesIDs []int64
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
		seriesIDs = append(seriesIDs, cs.ID)
	}

	if len(result) == 0 {
		return result, nil
	}

	// Fetch all games for all series in a single query to avoid N+1 query problem
	// that causes OOMKill (500 separate queries → 1 batch query)
	gamesMap := make(map[int64][]SeriesGameData)
	if len(seriesIDs) > 0 {
		// Build WHERE IN clause with up to 64 series IDs
		placeholders := make([]string, len(seriesIDs))
		args := make([]interface{}, len(seriesIDs))
		for i, id := range seriesIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+2)
			args[i] = id
		}
		query := fmt.Sprintf(`
			SELECT sg.series_id, sg.match_id, sg.game_num, sg.winner_id,
			       COALESCE(m.turn_count, 0), m.completed_at,
			       CASE WHEN sg.winner_id IS NOT NULL THEN
			           (SELECT mp.player_slot FROM match_participants mp
			            WHERE mp.match_id = sg.match_id AND mp.bot_id = sg.winner_id)
			       END
			FROM series_games sg
			LEFT JOIN matches m ON sg.match_id = m.match_id
			WHERE sg.series_id = $1
			   OR sg.series_id IN (%s)
			ORDER BY sg.series_id, sg.game_num
			LIMIT 500
		`, strings.Join(placeholders, ", "))

		fullArgs := append([]interface{}{seasonID}, args...)
		gamesRows, err := db.QueryContext(ctx, query, fullArgs...)
		if err != nil {
			return nil, fmt.Errorf("query championship games: %w", err)
		}
		defer gamesRows.Close()

		for gamesRows.Next() {
			var g SeriesGameData
			var seriesID int64
			var winnerID sql.NullString
			var winnerSlot sql.NullInt64
			var turns sql.NullInt64
			var completedAt sql.NullTime

			err := gamesRows.Scan(&seriesID, &g.MatchID, &g.GameNum, &winnerID, &turns, &completedAt, &winnerSlot)
			if err != nil {
				return nil, fmt.Errorf("scan championship game: %w", err)
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
			gamesMap[seriesID] = append(gamesMap[seriesID], g)
		}
	}

	// Assign games to each series
	for i := range result {
		result[i].Games = gamesMap[result[i].ID]
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
			LIMIT 1000
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
			SELECT m.map_id, m.player_count, m.status, m.engagement, m.wall_density,
			       m.energy_count, m.grid_width, m.grid_height, m.created_at, m.map_json,
			       COALESCE(v.vote_sum, 0) as net_votes
			FROM maps m
			LEFT JOIN (
				SELECT map_id, SUM(vote)::int as vote_sum
				FROM map_votes
				GROUP BY map_id
			) v ON m.map_id = v.map_id
			WHERE m.status IN ('active', 'probation', 'classic')
			ORDER BY m.engagement DESC
			LIMIT 1000
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
			&m.EnergyCount, &m.GridWidth, &m.GridHeight, &m.CreatedAt, &m.RawJSON,
			&m.NetVotes,
		); err != nil {
			return nil, err
		}
		maps = append(maps, m)
	}

	return maps, nil
}

// fetchOpenPredictions retrieves pending matches that are "predictable":
// - Both bots are in the top 20
// - It's a rivalry match (at least 3 previous h2h matches)
// - It's a series match
// - An evolved bot faces a top-10 human-written bot
func fetchOpenPredictions(ctx context.Context, db *sql.DB) ([]OpenPredictionMatch, error) {
	// Get predictable matches that are still pending (matchmaker sets predictable flag)
	query := `
		SELECT m.match_id, m.created_at,
		       mp1.bot_id as bot_a_id, b1.name as bot_a_name,
		       (b1.rating_mu - 2*b1.rating_phi) as bot_a_rating,
		       b1.evolved as bot_a_evolved,
		       mp2.bot_id as bot_b_id, b2.name as bot_b_name,
		       (b2.rating_mu - 2*b2.rating_phi) as bot_b_rating,
		       b2.evolved as bot_b_evolved,
		       COALESCE(EXISTS(
		           SELECT 1 FROM series_games sg
		           WHERE sg.match_id = m.match_id
		       ), false) as is_series_match
		FROM matches m
		JOIN match_participants mp1 ON m.match_id = mp1.match_id AND mp1.player_slot = 0
		JOIN match_participants mp2 ON m.match_id = mp2.match_id AND mp2.player_slot = 1
		JOIN bots b1 ON mp1.bot_id = b1.bot_id
		JOIN bots b2 ON mp2.bot_id = b2.bot_id
		WHERE m.status = 'pending' AND m.predictable = TRUE
		ORDER BY m.created_at ASC
		LIMIT 50
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query predictable matches: %w", err)
	}
	defer rows.Close()

	var matches []OpenPredictionMatch
	for rows.Next() {
		var m OpenPredictionMatch
		var isSeries bool
		err := rows.Scan(
			&m.MatchID, &m.CreatedAt,
			&m.BotAID, &m.BotAName, &m.ARating, &m.AEvolved,
			&m.BotBID, &m.BotBName, &m.BRating, &m.BEvolved,
			&isSeries,
		)
		if err != nil {
			return nil, fmt.Errorf("scan predictable match: %w", err)
		}
		m.IsSeriesMatch = isSeries
		matches = append(matches, m)
	}

	// Calculate head-to-head records for all predictable matches (limit to 10)
	var predictableMatches []OpenPredictionMatch
	for i, m := range matches {
		if i >= 10 {
			break
		}
		// Calculate head-to-head record
		h2hRecord := computeHeadToHeadRecord(ctx, db, m.BotAID, m.BotBID)
		m.HeadToHeadRecord = &h2hRecord
		predictableMatches = append(predictableMatches, m)
	}

	return predictableMatches, nil
}

// computeHeadToHeadRecord returns the head-to-head record between two bots
func computeHeadToHeadRecord(ctx context.Context, db *sql.DB, botAID, botBID string) string {
	var aWins, bWins int
	err := db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE m.winner = 0) as a_wins,
			COUNT(*) FILTER (WHERE m.winner = 1) as b_wins
		FROM matches m
		JOIN match_participants mp1 ON m.match_id = mp1.match_id AND mp1.player_slot = 0
		JOIN match_participants mp2 ON m.match_id = mp2.match_id AND mp2.player_slot = 1
		WHERE mp1.bot_id = $1 AND mp2.bot_id = $2 AND m.status = 'completed'
	`, botAID, botBID).Scan(&aWins, &bWins)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", aWins, bWins)
}

func fetchFeedback(ctx context.Context, db *sql.DB) ([]FeedbackEntry, error) {
	query := `
		SELECT feedback_id, match_id, turn, type, body, author, upvotes, created_at
		FROM replay_feedback
		ORDER BY upvotes DESC, created_at DESC
		LIMIT 1000
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []FeedbackEntry
	for rows.Next() {
		var e FeedbackEntry
		if err := rows.Scan(&e.FeedbackID, &e.MatchID, &e.Turn, &e.Type, &e.Body, &e.Author, &e.Upvotes, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func computeTopPredictors(stats []PredictorStats) []PredictorStats {
	if len(stats) > 50 {
		return stats[:50]
	}
	return stats
}

// ─── Evolution Data (meta.json, lineage.json) ───────────────────────────────────────

// EvolutionMeta represents data/evolution/meta.json
type EvolutionMeta struct {
	Generation        int                `json:"generation"`
	PromotedToday     int                `json:"promoted_today"`
	Top10Count        int                `json:"top_10_count"`
	IslandPopulations map[string]int     `json:"island_populations"` // island -> program count
	BestRatings       []EvolvedBotRating `json:"best_ratings"`       // top 10 evolved bots
	TotalPromoted     int                `json:"total_promoted"`     // all-time promoted count
	PromotionRate     float64            `json:"promotion_rate"`     // promoted/total
	UpdatedAt         string             `json:"updated_at"`
	MatchesToday      int                `json:"matches_today"` // plan §16.18: matches completed today
	ActiveBots        int                `json:"active_bots"`   // plan §16.18: active bot count
}

// EvolvedBotRating represents an evolved bot's rating info
type EvolvedBotRating struct {
	BotID    string  `json:"bot_id"`
	Name     string  `json:"name"`
	Rating   float64 `json:"rating"`
	Island   string  `json:"island"`
	Language string  `json:"language"`
}

// LineageNode represents a single program in the lineage tree
type LineageNode struct {
	ID         int64     `json:"id"`
	ParentIDs  []int64   `json:"parent_ids"`
	Generation int       `json:"generation"`
	Island     string    `json:"island"`
	Fitness    float64   `json:"fitness"`
	Promoted   bool      `json:"promoted"`
	Language   string    `json:"language"`
	CreatedAt  time.Time `json:"created_at"`
}

// fetchEvolutionMeta queries the evolver database for evolution statistics.
// It connects to the evolver database using the same connection parameters.
// Returns empty meta (not an error) if the evolution system is not running.
func fetchEvolutionMeta(ctx context.Context, db *sql.DB) (*EvolutionMeta, error) {
	// Query the programs table in the evolver database
	// Note: the evolver uses a separate database but same PostgreSQL instance
	query := `
		SELECT
			COALESCE(MAX(generation), 0) as generation,
			COALESCE(COUNT(*) FILTER (WHERE promoted AND created_at >= CURRENT_DATE), 0) as promoted_today,
			COALESCE(COUNT(*) FILTER (WHERE promoted), 0) as total_promoted,
				COALESCE(COUNT(*), 0) as total_programs
		FROM programs
	`

	var meta EvolutionMeta
	var updatedAt string = time.Now().UTC().Format(time.RFC3339)
	var totalPrograms int

	err := db.QueryRowContext(ctx, query).Scan(&meta.Generation, &meta.PromotedToday, &meta.TotalPromoted, &totalPrograms)
	if err != nil {
		// If evolver tables don't exist or query fails, return empty meta
		// This is expected when the evolution system has not been initialized yet
		slog.Info("Evolution system not running or programs table empty", "error", err)
		return &EvolutionMeta{
			Generation:        0,
			PromotedToday:     0,
			Top10Count:        0,
			IslandPopulations: make(map[string]int),
			BestRatings:       []EvolvedBotRating{},
			TotalPromoted:     0,
			PromotionRate:     0,
			UpdatedAt:         updatedAt,
			MatchesToday:      0,
			ActiveBots:        0,
		}, nil
	}

	// Calculate promotion rate
	if totalPrograms > 0 {
		meta.PromotionRate = float64(meta.TotalPromoted) / float64(totalPrograms)
	}

	// Fetch island populations
	meta.IslandPopulations = make(map[string]int)
	islandRows, err := db.QueryContext(ctx, `
		SELECT island, COUNT(*) FROM programs GROUP BY island LIMIT 100
	`)
	if err == nil {
		for islandRows.Next() {
			var island string
			var count int
			if islandRows.Scan(&island, &count) == nil {
				meta.IslandPopulations[island] = count
			}
		}
		islandRows.Close()
	}

	// Fetch best ratings (top 10 evolved bots by rating)
	// Join with bots table to get current ratings
	bestRows, err := db.QueryContext(ctx, `
		SELECT
			p.bot_id, b.name, b.rating_mu - 2*b.rating_phi as rating,
			p.island, p.language
		FROM programs p
		JOIN bots b ON p.bot_id = b.bot_id
		WHERE b.status = 'active'
		ORDER BY b.rating_mu DESC
		LIMIT 10
	`)
	if err == nil {
		for bestRows.Next() {
			var rating EvolvedBotRating
			if bestRows.Scan(&rating.BotID, &rating.Name, &rating.Rating, &rating.Island, &rating.Language) == nil {
				meta.BestRatings = append(meta.BestRatings, rating)
			}
		}
		bestRows.Close()
	}

	// Count evolved bots in top 10
	meta.Top10Count = len(meta.BestRatings)

	// Fetch matches today (plan §16.18: completed matches since midnight UTC)
	var matchesToday int
	matchErr := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM matches
		WHERE completed_at >= CURRENT_DATE
	`).Scan(&matchesToday)
	if matchErr != nil {
		matchesToday = 0
	}
	meta.MatchesToday = matchesToday

	// Fetch active bots count (plan §16.18: bots with status = 'active')
	var activeBots int
	botErr := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM bots
		WHERE status = 'active'
	`).Scan(&activeBots)
	if botErr != nil {
		activeBots = 0
	}
	meta.ActiveBots = activeBots

	meta.UpdatedAt = updatedAt
	return &meta, nil
}

// fetchLineage queries the evolver database for the full lineage tree.
// Returns all programs with their parent relationships.
func fetchLineage(ctx context.Context, db *sql.DB) ([]LineageNode, error) {
	query := `
		SELECT id, parent_ids, generation, island, fitness, promoted, language, created_at
		FROM programs
		ORDER BY generation ASC, id ASC
		LIMIT 1000
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		// If evolver tables don't exist, return empty lineage
		if err == sql.ErrNoRows {
			return []LineageNode{}, nil
		}
		return nil, fmt.Errorf("fetch lineage: %w", err)
	}
	defer rows.Close()

	var nodes []LineageNode
	for rows.Next() {
		var node LineageNode
		var parentJSON string

		err := rows.Scan(&node.ID, &parentJSON, &node.Generation, &node.Island,
			&node.Fitness, &node.Promoted, &node.Language, &node.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan lineage node: %w", err)
		}

		// Unmarshal parent_ids from JSONB
		if err := json.Unmarshal([]byte(parentJSON), &node.ParentIDs); err != nil {
			return nil, fmt.Errorf("unmarshal parent_ids: %w", err)
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
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
