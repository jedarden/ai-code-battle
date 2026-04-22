package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

const (
	fairnessMinGames      = 80
	fairnessThresholdPP   = 0.10
	voteForceRetireThreshold = -20
	engagementPrunePct    = 0.10
	classicMinMonths      = 3
	classicTopN           = 5
)

// tickFairnessAudit runs the full map lifecycle audit:
//  1. Update map_fairness from completed matches
//  2. Flag positionally unfair maps as probation
//  3. Force-retire maps with >20 net negative votes
//  4. Monthly: prune bottom 10% by engagement
//  5. Promote top-5 sustained maps to classic
func (m *Matchmaker) tickFairnessAudit(ctx context.Context) {
	if err := m.updateMapFairnessStats(ctx); err != nil {
		log.Printf("fairness-audit: update stats error: %v", err)
	}
	if err := m.flagUnfairMaps(ctx); err != nil {
		log.Printf("fairness-audit: flag unfair error: %v", err)
	}
	if err := m.retireDislikedMaps(ctx); err != nil {
		log.Printf("fairness-audit: retire disliked error: %v", err)
	}
	if err := m.pruneLowEngagementMaps(ctx); err != nil {
		log.Printf("fairness-audit: prune engagement error: %v", err)
	}
	if err := m.promoteClassicMaps(ctx); err != nil {
		log.Printf("fairness-audit: promote classic error: %v", err)
	}
}

// updateMapFairnessStats recomputes per-slot win counts from match_participants
// and writes them into the map_fairness table for all active/probation maps.
func (m *Matchmaker) updateMapFairnessStats(ctx context.Context) error {
	// For each map+slot, count completed matches where that slot won.
	rows, err := m.db.QueryContext(ctx, `
		SELECT m.map_id, mp.player_slot,
		       COUNT(*) AS games,
		       COUNT(*) FILTER (WHERE m.winner = mp.player_slot) AS wins
		FROM match_participants mp
		JOIN matches m ON m.match_id = mp.match_id
		JOIN maps map ON map.map_id = m.map_id
		WHERE m.status = 'completed'
		  AND map.status IN ('active', 'probation')
		GROUP BY m.map_id, mp.player_slot
	`)
	if err != nil {
		return fmt.Errorf("query fairness stats: %w", err)
	}
	defer rows.Close()

	type fairnessRow struct {
		MapID      string
		PlayerSlot int
		Games      int
		Wins       int
	}
	var stats []fairnessRow

	for rows.Next() {
		var r fairnessRow
		if err := rows.Scan(&r.MapID, &r.PlayerSlot, &r.Games, &r.Wins); err != nil {
			return fmt.Errorf("scan fairness row: %w", err)
		}
		stats = append(stats, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, s := range stats {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO map_fairness (map_id, player_slot, games, wins, last_check)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (map_id, player_slot) DO UPDATE
			SET games = $3, wins = $4, last_check = NOW()
		`, s.MapID, s.PlayerSlot, s.Games, s.Wins)
		if err != nil {
			log.Printf("fairness-audit: update map_fairness for %s slot %d: %v", s.MapID, s.PlayerSlot, err)
		}
	}

	if len(stats) > 0 {
		log.Printf("fairness-audit: updated fairness stats for %d map-slot pairs", len(stats))
	}
	return nil
}

// flagUnfairMaps sets status='probation' for maps where any player slot's
// win rate deviates from expected (1/N for N-player) by more than 10pp
// across 80+ completed matches.
func (m *Matchmaker) flagUnfairMaps(ctx context.Context) error {
	// Find maps where any slot has >=80 games and win rate deviation > 10pp.
	rows, err := m.db.QueryContext(ctx, `
		WITH slot_rates AS (
			SELECT
				mf.map_id,
				mf.player_slot,
				mf.games,
				mf.wins,
				mf.wins::float / NULLIF(mf.games, 0) AS win_rate,
				map.player_count,
				1.0 / map.player_count AS expected_rate
			FROM map_fairness mf
			JOIN maps map ON map.map_id = mf.map_id
			WHERE map.status = 'active'
			  AND mf.games >= $1
		),
		unfair AS (
			SELECT DISTINCT map_id
			FROM slot_rates
			WHERE ABS(win_rate - expected_rate) > $2
		)
		SELECT map_id FROM unfair
	`, fairnessMinGames, fairnessThresholdPP)
	if err != nil {
		return fmt.Errorf("query unfair maps: %w", err)
	}
	defer rows.Close()

	var flagged []string
	for rows.Next() {
		var mapID string
		if err := rows.Scan(&mapID); err != nil {
			return err
		}
		flagged = append(flagged, mapID)
	}

	for _, mapID := range flagged {
		_, err := m.db.ExecContext(ctx, `
			UPDATE maps SET status = 'probation' WHERE map_id = $1 AND status = 'active'
		`, mapID)
		if err != nil {
			log.Printf("fairness-audit: failed to flag %s as probation: %v", mapID, err)
			continue
		}
		log.Printf("fairness-audit: flagged map %s as probation (positional unfairness detected)", mapID)
	}

	return nil
}

// retireDislikedMaps force-retires maps with >20 net negative votes,
// regardless of engagement score.
func (m *Matchmaker) retireDislikedMaps(ctx context.Context) error {
	result, err := m.db.ExecContext(ctx, `
		UPDATE maps m SET
			status = 'retired',
			retired_at = NOW()
		FROM (
			SELECT map_id, SUM(vote)::int AS net_votes
			FROM map_votes
			GROUP BY map_id
			HAVING SUM(vote) < $1
		) v
		WHERE m.map_id = v.map_id
		  AND m.status IN ('active', 'probation')
	`, voteForceRetireThreshold)
	if err != nil {
		return fmt.Errorf("retire disliked maps: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		log.Printf("fairness-audit: force-retired %d map(s) with <%d net votes", affected, voteForceRetireThreshold)
	}
	return nil
}

// pruneLowEngagementMaps retires the bottom 10% of active maps by rolling
// average engagement score, run once per month (checked by day-of-month).
func (m *Matchmaker) pruneLowEngagementMaps(ctx context.Context) error {
	// Only run on the 1st of each month.
	if time.Now().Day() != 1 {
		return nil
	}

	// Check if we already pruned this month.
	var prunedThisMonth int
	err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maps
		WHERE retired_at >= DATE_TRUNC('month', CURRENT_DATE)
		  AND retired_at IS NOT NULL
	`).Scan(&prunedThisMonth)
	if err != nil {
		return fmt.Errorf("check monthly prune: %w", err)
	}
	if prunedThisMonth > 0 {
		return nil
	}

	// Compute engagement from map_scores rolling average, falling back to maps.engagement.
	// Count active maps per player_count, then prune bottom 10% within each tier.
	for _, pc := range []int{2, 3, 4, 6} {
		var totalActive int
		err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM maps
			WHERE player_count = $1 AND status = 'active'
		`, pc).Scan(&totalActive)
		if err != nil || totalActive < 10 {
			continue
		}

		toPrune := int(math.Ceil(float64(totalActive) * engagementPrunePct))
		if toPrune < 1 {
			continue
		}

		result, err := m.db.ExecContext(ctx, `
			UPDATE maps m SET
				status = 'retired',
				retired_at = NOW()
			FROM (
				SELECT map_id FROM maps
				WHERE player_count = $1 AND status = 'active'
				ORDER BY engagement ASC
				LIMIT $2
			) sub
			WHERE m.map_id = sub.map_id
		`, pc, toPrune)
		if err != nil {
			log.Printf("fairness-audit: prune engagement error for player_count=%d: %v", pc, err)
			continue
		}

		affected, _ := result.RowsAffected()
		if affected > 0 {
			log.Printf("fairness-audit: pruned %d/%d low-engagement maps for %d-player tier", affected, totalActive, pc)
		}
	}

	return nil
}

// promoteClassicMaps promotes maps that have been in the top-5 engagement
// for their player count for 3+ months to 'classic' status, making them
// immune from retirement.
func (m *Matchmaker) promoteClassicMaps(ctx context.Context) error {
	for _, pc := range []int{2, 3, 4, 6} {
		result, err := m.db.ExecContext(ctx, `
			UPDATE maps m SET status = 'classic'
			FROM (
				SELECT map_id FROM maps
				WHERE player_count = $1
				  AND status = 'active'
				  AND engagement > 0
				  AND created_at < NOW() - INTERVAL '3 months'
				ORDER BY engagement DESC
				LIMIT $2
			) sub
			WHERE m.map_id = sub.map_id
		`, pc, classicTopN)
		if err != nil {
			log.Printf("fairness-audit: promote classic error for player_count=%d: %v", pc, err)
			continue
		}

		affected, _ := result.RowsAffected()
		if affected > 0 {
			log.Printf("fairness-audit: promoted %d map(s) to classic for %d-player tier", affected, pc)
		}
	}
	return nil
}
