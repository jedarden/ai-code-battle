// Package db provides database access for the enrichment service.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Store provides database operations for enrichment.
type Store struct {
	db *sql.DB
}

// NewStore creates a new database store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Match represents a match from the database.
type Match struct {
	ID            string
	MapID         string
	Status        string
	Winner        sql.NullInt32
	Condition     sql.NullString
	TurnCount     sql.NullInt32
	ScoresJSON    sql.NullString
	CreatedAt     time.Time
	CompletedAt   sql.NullTime
	CommentaryJSON sql.NullString // NULL if not yet enriched
}

// MatchParticipant represents a participant in a match.
type MatchParticipant struct {
	MatchID    string
	BotID      string
	PlayerSlot int
	Score      sql.NullInt32
	Status     string
}

// BotInfo represents bot information for enrichment.
type BotInfo struct {
	ID          string
	Name        string
	RatingMu    float64
	RatingPhi   float64
	RatingSigma float64
}

// CandidateMatch represents a match that may be enriched.
type CandidateMatch struct {
	MatchID         string
	TurnCount       int
	Winner          int
	Condition       string
	FinalScores     []int
	Players         []PlayerData
	WinProbCrossings int
	IsUpset         bool
	IsCloseFinish   bool
}

// PlayerData holds player info for enrichment.
type PlayerData struct {
	ID     int
	BotID  string
	Name   string
	Rating int
}

// FindCandidates finds matches that qualify for enrichment.
// Returns matches that:
// - Are completed
// - Have not been enriched yet
// - Meet the enrichment criteria (turn count, win prob crossings, upset threshold)
func (s *Store) FindCandidates(ctx context.Context, minTurns, minCrossings int, upsetThreshold float64) ([]CandidateMatch, error) {
	// Query for completed matches without commentary
	query := `
		SELECT m.match_id, m.turn_count, m.winner, m.condition, m.scores_json,
		       jsonb_agg(jsonb_build_object(
				   'bot_id', mp.bot_id,
				   'player_slot', mp.player_slot,
				   'name', b.name,
				   'rating_mu', b.rating_mu,
				   'rating_phi', b.rating_phi
		       ) ORDER BY mp.player_slot) as participants
		FROM matches m
		JOIN match_participants mp ON m.match_id = mp.match_id
		JOIN bots b ON mp.bot_id = b.bot_id
		WHERE m.status = 'completed'
		  AND m.commentary_json IS NULL
		  AND m.turn_count >= $1
		GROUP BY m.match_id, m.turn_count, m.winner, m.condition, m.scores_json
		ORDER BY m.completed_at DESC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, query, minTurns)
	if err != nil {
		return nil, fmt.Errorf("query candidates: %w", err)
	}
	defer rows.Close()

	var candidates []CandidateMatch
	for rows.Next() {
		var cm CandidateMatch
		var scoresJSON sql.NullString
		var participantsJSON string

		err := rows.Scan(
			&cm.MatchID,
			&cm.TurnCount,
			&cm.Winner,
			&cm.Condition,
			&scoresJSON,
			&participantsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}

		// Parse scores
		if scoresJSON.Valid {
			var scores []int
			if err := json.Unmarshal([]byte(scoresJSON.String), &scores); err == nil {
				cm.FinalScores = scores
			}
		}

		// Parse participants
		var participants []struct {
			BotID      string `json:"bot_id"`
			PlayerSlot int    `json:"player_slot"`
			Name       string `json:"name"`
			RatingMu   float64 `json:"rating_mu"`
			RatingPhi  float64 `json:"rating_phi"`
		}
		if err := json.Unmarshal([]byte(participantsJSON), &participants); err != nil {
			continue
		}

		for _, p := range participants {
			displayRating := int(p.RatingMu - 2*p.RatingPhi)
			cm.Players = append(cm.Players, PlayerData{
				ID:     p.PlayerSlot,
				BotID:  p.BotID,
				Name:   p.Name,
				Rating: displayRating,
			})
		}

		// Calculate win prob crossings (simplified - in real implementation
		// this would come from pre-computed data or replay analysis)
		// For now, use a heuristic based on turn count and score difference
		cm.WinProbCrossings = s.calculateCrossings(cm.FinalScores, cm.TurnCount)

		// Check for upset (lower-rated player won)
		if len(cm.Players) >= 2 && cm.Winner >= 0 && cm.Winner < len(cm.Players) {
			winnerIdx := cm.Winner
			// Find opponent
			for i, p := range cm.Players {
				if i != winnerIdx {
					if float64(cm.Players[winnerIdx].Rating) < float64(p.Rating)-upsetThreshold {
						cm.IsUpset = true
					}
					break
				}
			}
		}

		// Check for close finish
		if len(cm.FinalScores) >= 2 {
			maxScore := cm.FinalScores[0]
			minScore := cm.FinalScores[0]
			for _, s := range cm.FinalScores {
				if s > maxScore {
					maxScore = s
				}
				if s < minScore {
					minScore = s
				}
			}
			cm.IsCloseFinish = (maxScore - minScore) <= 2
		}

		// Apply filters
		if cm.WinProbCrossings < minCrossings && !cm.IsUpset && !cm.IsCloseFinish {
			continue
		}

		candidates = append(candidates, cm)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return candidates, nil
}

// calculateCrossings estimates win probability crossings from scores.
// This is a simplified heuristic - the real implementation would analyze
// the win probability curve from the replay.
func (s *Store) calculateCrossings(scores []int, turnCount int) int {
	if len(scores) < 2 {
		return 0
	}

	// Estimate crossings based on score volatility and turn count
	// More turns + closer scores = more likely to have crossings
	scoreDiff := math.Abs(float64(scores[0] - scores[1]))
	if scoreDiff < 5 {
		// Close scores suggest multiple lead changes
		return min(5, turnCount/100)
	}
	if scoreDiff < 20 {
		return min(3, turnCount/150)
	}
	return 0
}

// MarkEnriched marks a match as enriched by storing the commentary JSON.
func (s *Store) MarkEnriched(ctx context.Context, matchID string, commentaryJSON string) error {
	query := `
		UPDATE matches
		SET commentary_json = $1
		WHERE match_id = $2
	`
	_, err := s.db.ExecContext(ctx, query, commentaryJSON, matchID)
	if err != nil {
		return fmt.Errorf("mark enriched: %w", err)
	}
	return nil
}

// GetEnrichmentCount returns the number of enrichments in the last hour.
// Used for rate limiting.
func (s *Store) GetEnrichmentCount(ctx context.Context, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM matches
		WHERE commentary_json IS NOT NULL
		  AND completed_at >= $1
	`
	var count int
	err := s.db.QueryRowContext(ctx, query, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count enrichments: %w", err)
	}
	return count, nil
}

// GetBotRatings gets current ratings for a list of bot IDs.
func (s *Store) GetBotRatings(ctx context.Context, botIDs []string) (map[string]BotInfo, error) {
	if len(botIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT bot_id, name, rating_mu, rating_phi, rating_sigma
		FROM bots
		WHERE bot_id = ANY($1)
	`
	rows, err := s.db.QueryContext(ctx, query, botIDs)
	if err != nil {
		return nil, fmt.Errorf("query bot ratings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]BotInfo)
	for rows.Next() {
		var b BotInfo
		err := rows.Scan(&b.ID, &b.Name, &b.RatingMu, &b.RatingPhi, &b.RatingSigma)
		if err != nil {
			return nil, fmt.Errorf("scan bot: %w", err)
		}
		result[b.ID] = b
	}

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
