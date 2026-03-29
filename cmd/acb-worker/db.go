// PostgreSQL database client for match results and job coordination
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// DBClient handles PostgreSQL operations.
type DBClient struct {
	db *sql.DB
}

// NewDBClient creates a new database client.
func NewDBClient(databaseURL string) (*DBClient, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DBClient{db: db}, nil
}

// Close closes the database connection.
func (c *DBClient) Close() error {
	return c.db.Close()
}

// DBJob represents a pending job from the database.
type DBJob struct {
	ID          string     `json:"id"`
	MatchID     string     `json:"match_id"`
	Status      string     `json:"status"`
	WorkerID    *string    `json:"worker_id"`
	ClaimedAt   *time.Time `json:"claimed_at"`
	HeartbeatAt *time.Time `json:"heartbeat_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// DBMatch represents match metadata from the database.
type DBMatch struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Winner      *int       `json:"winner"` // player index
	MapID       string     `json:"map_id"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// DBMatch represents match metadata.
type DBMatch struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Winner      *int       `json:"winner"` // player index
	MapID       string     `json:"map_id"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// DBParticipant represents a match participant.
type DBParticipant struct {
	MatchID              string `json:"match_id"`
	BotID                string `json:"bot_id"`
	PlayerSlot           int    `json:"player_slot"`
	Score                int    `json:"score"`
	RatingMuBefore       float64
	RatingPhiBefore      float64
	RatingSigmaBefore    float64
	RatingMuAfter        *float64
	RatingPhiAfter       *float64
	RatingSigmaAfter     *float64
}

// DBBotInfo contains bot endpoint and secret information.
type DBBotInfo struct {
	ID          string
	EndpointURL string
	Secret      string
}

// DBMapData represents map configuration.
type DBMapData struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Walls  string `json:"walls"`
	Spawns string `json:"spawns"`
	Cores  string `json:"cores"`
}

// JobClaimData contains all data needed to execute a match.
type JobClaimData struct {
	Job          DBJob
	Match        DBMatch
	Participants []DBParticipant
	Map          DBMapData
	Bots         []DBBotInfo
}

// GetNextJob fetches the next pending job from the database.
func (c *DBClient) GetNextJob(ctx context.Context) (*DBJob, error) {
	query := `
		SELECT job_id, match_id, status, worker_id, claimed_at, heartbeat_at, created_at
		FROM jobs
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	var job DBJob
	err := c.db.QueryRowContext(ctx, query).Scan(
		&job.ID, &job.MatchID, &job.Status, &job.WorkerID,
		&job.ClaimedAt, &job.HeartbeatAt, &job.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No pending jobs
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get next job: %w", err)
	}

	return &job, nil
}

// ClaimJob claims a job and returns all data needed to execute the match.
func (c *DBClient) ClaimJob(ctx context.Context, jobID string, workerID string) (*JobClaimData, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update job status
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'claimed', worker_id = $1, claimed_at = $2
		WHERE job_id = $3 AND status = 'pending'
	`, workerID, now, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to claim job: %w", err)
	}

	// Get job details
	var job DBJob
	err = tx.QueryRowContext(ctx, `
		SELECT job_id, match_id, status, worker_id, claimed_at, heartbeat_at, created_at
		FROM jobs WHERE job_id = $1
	`, jobID).Scan(
		&job.ID, &job.MatchID, &job.Status, &job.WorkerID,
		&job.ClaimedAt, &job.HeartbeatAt, &job.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Get match details
	var match DBMatch
	err = tx.QueryRowContext(ctx, `
		SELECT match_id, status, winner, map_id, created_at, completed_at
		FROM matches WHERE match_id = $1
	`, job.MatchID).Scan(
		&match.ID, &match.Status, &match.Winner, &match.MapID,
		&match.CreatedAt, &match.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}

	// Get map data
	var mapData DBMapData
	err = tx.QueryRowContext(ctx, `
		SELECT map_id, grid_width, grid_height, map_json->>'walls' as walls,
		       map_json->>'spawns' as spawns, map_json->>'cores' as cores
		FROM maps WHERE map_id = $1
	`, match.MapID).Scan(
		&mapData.ID, &mapData.Width, &mapData.Height,
		&mapData.Walls, &mapData.Spawns, &mapData.Cores,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get map: %w", err)
	}

	// Get participants
	participantRows, err := tx.QueryContext(ctx, `
		SELECT mp.match_id, mp.bot_id, mp.player_slot, mp.score,
		       b.rating_mu, b.rating_phi, b.rating_sigma
		FROM match_participants mp
		JOIN bots b ON mp.bot_id = b.bot_id
		WHERE mp.match_id = $1
		ORDER BY mp.player_slot
	`, job.MatchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}
	defer participantRows.Close()

	var participants []DBParticipant
	var botIDs []string
	for participantRows.Next() {
		var p DBParticipant
		err := participantRows.Scan(
			&p.MatchID, &p.BotID, &p.PlayerSlot, &p.Score,
			&p.RatingMuBefore, &p.RatingPhiBefore, &p.RatingSigmaBefore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}
		participants = append(participants, p)
		botIDs = append(botIDs, p.BotID)
	}

	// Get bot endpoints and secrets
	botRows, err := tx.QueryContext(ctx, `
		SELECT bot_id, endpoint_url, shared_secret
		FROM bots WHERE bot_id = ANY($1)
	`, botIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get bots: %w", err)
	}
	defer botRows.Close()

	var bots []DBBotInfo
	for botRows.Next() {
		var b DBBotInfo
		if err := botRows.Scan(&b.ID, &b.EndpointURL, &b.Secret); err != nil {
			return nil, fmt.Errorf("failed to scan bot: %w", err)
		}
		bots = append(bots, b)
	}

	// Update match status to running
	_, err = tx.ExecContext(ctx, `
		UPDATE matches SET status = 'running' WHERE match_id = $1
	`, job.MatchID)
	if err != nil {
		return nil, fmt.Errorf("failed to update match status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &JobClaimData{
		Job:          job,
		Match:        match,
		Participants: participants,
		Map:          mapData,
		Bots:         bots,
	}, nil
}

// Heartbeat updates the heartbeat timestamp for a job.
func (c *DBClient) Heartbeat(ctx context.Context, jobID string, workerID string) error {
	result, err := c.db.ExecContext(ctx, `
		UPDATE jobs
		SET heartbeat_at = NOW()
		WHERE job_id = $1 AND worker_id = $2 AND status = 'claimed'
	`, jobID, workerID)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("job not found or not claimed by this worker")
	}

	return nil
}

// SubmitMatchResult writes the match result to the database and updates ratings.
func (c *DBClient) SubmitMatchResult(ctx context.Context, jobID string, result *MatchResult, replayURL string, ratingUpdates []RatingUpdate) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()

	// Determine winner player index from result
	var winnerIndex *int
	if result.WinnerID != "" {
		// Look up player slot for winner
		var idx int
		err := tx.QueryRowContext(ctx, `
			SELECT player_slot FROM match_participants WHERE match_id = (
				SELECT match_id FROM jobs WHERE job_id = $1
			) AND bot_id = $2
		`, jobID, result.WinnerID).Scan(&idx)
		if err == nil {
			winnerIndex = &idx
		}
	}

	// Update job status
	_, err = tx.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'completed', completed_at = $1
		WHERE job_id = $2
	`, now, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	// Get match ID
	var matchID string
	err = tx.QueryRowContext(ctx, `
		SELECT match_id FROM jobs WHERE job_id = $1
	`, jobID).Scan(&matchID)
	if err != nil {
		return fmt.Errorf("failed to get match ID: %w", err)
	}

	// Update match status and result
	scoresJSON, _ := json.Marshal(result.Scores)
	_, err = tx.ExecContext(ctx, `
		UPDATE matches
		SET status = 'completed', winner = $1, condition = $2,
		    turn_count = $3, scores_json = $4, completed_at = $5
		WHERE match_id = $6
	`, winnerIndex, result.EndReason, result.Turns, scoresJSON, now, matchID)
	if err != nil {
		return fmt.Errorf("failed to update match: %w", err)
	}

	// Update participant scores
	for botID, score := range result.Scores {
		_, err = tx.ExecContext(ctx, `
			UPDATE match_participants
			SET score = $1
			WHERE match_id = $2 AND bot_id = $3
		`, score, matchID, botID)
		if err != nil {
			return fmt.Errorf("failed to update participant score: %w", err)
		}
	}

	// Apply rating updates (Glicko-2)
	for _, update := range ratingUpdates {
		// Update bot rating
		_, err = tx.ExecContext(ctx, `
			UPDATE bots
			SET rating_mu = $1, rating_phi = $2, rating_sigma = $3, last_active = $4
			WHERE bot_id = $5
		`, update.Mu, update.Phi, update.Sigma, now, update.BotID)
		if err != nil {
			return fmt.Errorf("failed to update bot rating: %w", err)
		}

		// Record rating history
		_, err = tx.ExecContext(ctx, `
			INSERT INTO rating_history (bot_id, match_id, rating, recorded_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (bot_id, match_id) DO UPDATE SET rating = $3, recorded_at = $4
		`, update.BotID, matchID, update.DisplayRating, now)
		if err != nil {
			return fmt.Errorf("failed to record rating history: %w", err)
		}

		// Update participant with rating after
		_, err = tx.ExecContext(ctx, `
			UPDATE match_participants
			SET rating_mu_after = $1, rating_phi_after = $2, rating_sigma_after = $3
			WHERE match_id = $4 AND bot_id = $5
		`, update.Mu, update.Phi, update.Sigma, matchID, update.BotID)
		if err != nil {
			return fmt.Errorf("failed to update participant rating after: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// FailJob marks a job as failed.
func (c *DBClient) FailJob(ctx context.Context, jobID string, workerID string, errorMessage string) error {
	result, err := c.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'failed', completed_at = NOW()
		WHERE job_id = $1 AND worker_id = $2 AND status = 'claimed'
	`, jobID, workerID)
	if err != nil {
		return fmt.Errorf("failed to fail job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("job not found or not claimed by this worker")
	}

	// Also update match status
	_, err = c.db.ExecContext(ctx, `
		UPDATE matches
		SET status = 'failed', completed_at = NOW()
		WHERE match_id = (SELECT match_id FROM jobs WHERE job_id = $1)
	`, jobID)
	if err != nil {
		return fmt.Errorf("failed to update match status: %w", err)
	}

	return nil
}

// RatingUpdate represents a Glicko-2 rating update for a bot.
type RatingUpdate struct {
	BotID                 string
	Mu                    float64
	Phi                   float64
	Sigma                 float64
	DisplayRating         float64
	RatingMuBefore        float64
	RatingPhiBefore       float64
	RatingDeviationChange float64
}

// GetBotRatings retrieves current ratings for a list of bots.
func (c *DBClient) GetBotRatings(ctx context.Context, botIDs []string) (map[string]Glicko2Rating, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT bot_id, rating_mu, rating_phi, rating_sigma
		FROM bots WHERE bot_id = ANY($1)
	`, botIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get bot ratings: %w", err)
	}
	defer rows.Close()

	ratings := make(map[string]Glicko2Rating)
	for rows.Next() {
		var botID string
		var r Glicko2Rating
		if err := rows.Scan(&botID, &r.Mu, &r.Phi, &r.Sigma); err != nil {
			return nil, fmt.Errorf("failed to scan rating: %w", err)
		}
		ratings[botID] = r
	}

	return ratings, nil
}
