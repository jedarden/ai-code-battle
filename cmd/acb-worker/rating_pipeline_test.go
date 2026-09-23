package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestRatingPipelineEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL rating pipeline test in short mode")
	}

	db := openTestDB(t)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := fmt.Sprintf("rating_pipeline_%d", time.Now().UnixNano())
	quotedSchema := pq.QuoteIdentifier(schema)
	if _, err := db.Exec("CREATE SCHEMA " + quotedSchema); err != nil {
		t.Fatalf("create rating test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP SCHEMA " + quotedSchema + " CASCADE")
		_ = db.Close()
	})
	if _, err := db.Exec("SET search_path TO " + quotedSchema); err != nil {
		t.Fatalf("select rating test schema: %v", err)
	}

	for _, path := range []string{
		"../../migrations/0001_initial.sql",
		"../../migrations/0005_add_participant_ratings.sql",
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		if _, err := db.Exec(string(contents)); err != nil {
			t.Fatalf("apply migration %s: %v", path, err)
		}
	}

	ctx := context.Background()
	if _, err := db.Exec(`
		INSERT INTO bots (bot_id, name, owner, endpoint_url, shared_secret, status)
		VALUES
			('rating_winner', 'rating_winner', 'owner', 'http://winner.invalid', 'winner-secret', 'active'),
			('rating_loser', 'rating_loser', 'owner', 'http://loser.invalid', 'loser-secret', 'active');
		INSERT INTO maps (map_id, player_count, grid_width, grid_height, map_json)
		VALUES ('rating-map', 2, 10, 10, '{"walls":[],"spawns":[],"cores":[]}'::jsonb);
		INSERT INTO matches (match_id, map_id, status)
		VALUES ('rating-match', 'rating-map', 'pending');
		INSERT INTO match_participants (match_id, bot_id, player_slot)
		VALUES
			('rating-match', 'rating_winner', 0),
			('rating-match', 'rating_loser', 1);
		INSERT INTO jobs (job_id, match_id, status, config_json)
		VALUES ('rating-job', 'rating-match', 'pending', '{}'::jsonb);
	`); err != nil {
		t.Fatalf("seed rating match: %v", err)
	}

	client := &DBClient{db: db}
	claim, err := client.ClaimJob(ctx, "rating-job", "rating-test-worker")
	if err != nil {
		t.Fatalf("claim rating job: %v", err)
	}
	if len(claim.Participants) != 2 {
		t.Fatalf("claimed participants = %d, want 2", len(claim.Participants))
	}
	for _, participant := range claim.Participants {
		if participant.RatingMuBefore != glicko2DefaultMu ||
			participant.RatingPhiBefore != glicko2DefaultRD ||
			participant.RatingSigmaBefore != glicko2DefaultSigma {
			t.Errorf("initial rating for %s = (%v, %v, %v), want defaults", participant.BotID, participant.RatingMuBefore, participant.RatingPhiBefore, participant.RatingSigmaBefore)
		}
	}

	result := &MatchResult{
		WinnerID:  "rating_winner",
		Turns:     25,
		EndReason: "stalemate",
		Scores: map[string]int{
			"rating_winner": 3,
			"rating_loser":  1,
		},
	}
	updates := (&Worker{}).computeRatingUpdates(claim, result)
	if len(updates) != 2 {
		t.Fatalf("rating updates = %d, want 2", len(updates))
	}
	if updates[0].Mu <= glicko2DefaultMu || updates[1].Mu >= glicko2DefaultMu {
		t.Fatalf("winner/loser updates = (%v, %v), want gain and loss", updates[0].Mu, updates[1].Mu)
	}

	if err := client.SubmitMatchResult(ctx, "rating-job", result, "", updates); err != nil {
		t.Fatalf("submit rating match: %v", err)
	}

	ratings, err := client.GetBotRatings(ctx, []string{"rating_winner", "rating_loser"})
	if err != nil {
		t.Fatalf("load persisted ratings: %v", err)
	}
	for _, update := range updates {
		persisted := ratings[update.BotID]
		if persisted.Mu != update.Mu || persisted.Phi != update.Phi || persisted.Sigma != update.Sigma {
			t.Errorf("persisted rating for %s = (%v, %v, %v), want (%v, %v, %v)", update.BotID, persisted.Mu, persisted.Phi, persisted.Sigma, update.Mu, update.Phi, update.Sigma)
		}

		var muAfter, phiAfter, sigmaAfter, historyRating float64
		if err := db.QueryRowContext(ctx, `
			SELECT mp.rating_mu_after, mp.rating_phi_after, mp.rating_sigma_after, rh.rating
			FROM match_participants mp
			JOIN rating_history rh ON rh.match_id = mp.match_id AND rh.bot_id = mp.bot_id
			WHERE mp.match_id = $1 AND mp.bot_id = $2
		`, claim.Match.ID, update.BotID).Scan(&muAfter, &phiAfter, &sigmaAfter, &historyRating); err != nil {
			t.Fatalf("load participant rating for %s: %v", update.BotID, err)
		}
		if muAfter != update.Mu || phiAfter != update.Phi || sigmaAfter != update.Sigma {
			t.Errorf("participant rating for %s = (%v, %v, %v), want (%v, %v, %v)", update.BotID, muAfter, phiAfter, sigmaAfter, update.Mu, update.Phi, update.Sigma)
		}
		if historyRating != update.DisplayRating {
			t.Errorf("history rating for %s = %v, want %v", update.BotID, historyRating, update.DisplayRating)
		}
	}

	var matchStatus, jobStatus string
	var winnerSlot int
	if err := db.QueryRowContext(ctx, `
		SELECT m.status, m.winner, j.status
		FROM matches m
		JOIN jobs j ON j.match_id = m.match_id
		WHERE m.match_id = $1
	`, claim.Match.ID).Scan(&matchStatus, &winnerSlot, &jobStatus); err != nil {
		t.Fatalf("load completed rating match: %v", err)
	}
	if matchStatus != "completed" || jobStatus != "completed" || winnerSlot != 0 {
		t.Errorf("completed state = match %q winner %d job %q, want completed/0/completed", matchStatus, winnerSlot, jobStatus)
	}
}
