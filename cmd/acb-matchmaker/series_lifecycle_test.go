package main

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/lib/pq"
)

// setupSeriesTestSchema creates the slice of the production schema the series
// lifecycle passes touch. Column names mirror migrations/0001_initial.sql.
func setupSeriesTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS bots (
			bot_id        VARCHAR(16) PRIMARY KEY,
			endpoint_url  TEXT NOT NULL DEFAULT 'http://localhost:8080',
			shared_secret TEXT NOT NULL DEFAULT 'secret',
			status        VARCHAR(16) NOT NULL DEFAULT 'active',
			rating_mu     DOUBLE PRECISION NOT NULL DEFAULT 1500.0
		);

		CREATE TABLE IF NOT EXISTS matches (
			match_id     VARCHAR(32) PRIMARY KEY,
			map_id       VARCHAR(64),
			map_seed     BIGINT,
			status       VARCHAR(16) NOT NULL DEFAULT 'pending',
			winner       INTEGER,
			completed_at TIMESTAMPTZ
		);

		CREATE TABLE IF NOT EXISTS match_participants (
			match_id    VARCHAR(32) NOT NULL REFERENCES matches(match_id),
			bot_id      VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
			player_slot INTEGER NOT NULL,
			PRIMARY KEY (match_id, bot_id)
		);

		CREATE TABLE IF NOT EXISTS jobs (
			job_id       VARCHAR(32) PRIMARY KEY,
			match_id     VARCHAR(32) REFERENCES matches(match_id),
			status       VARCHAR(16) NOT NULL DEFAULT 'pending',
			worker_id    VARCHAR(64),
			claimed_at   TIMESTAMPTZ,
			heartbeat_at TIMESTAMPTZ,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ
		);

		CREATE TABLE IF NOT EXISTS series (
			id               BIGSERIAL PRIMARY KEY,
			bot_a_id         VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
			bot_b_id         VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
			format           INTEGER NOT NULL DEFAULT 5,
			a_wins           INTEGER NOT NULL DEFAULT 0,
			b_wins           INTEGER NOT NULL DEFAULT 0,
			status           VARCHAR(16) NOT NULL DEFAULT 'active',
			winner_id        VARCHAR(16),
			season_id        BIGINT,
			bracket_round    VARCHAR(32),
			bracket_position INTEGER,
			featured         BOOLEAN NOT NULL DEFAULT FALSE,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS series_games (
			id         BIGSERIAL PRIMARY KEY,
			series_id  BIGINT NOT NULL REFERENCES series(id),
			match_id   VARCHAR(32) REFERENCES matches(match_id),
			game_num   INTEGER NOT NULL,
			map_id     VARCHAR(64),
			winner_id  VARCHAR(16),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		t.Fatalf("create series schema: %v", err)
	}
}

// newSeriesLifecycleMatchmaker builds a Matchmaker wired to the test database.
// The lifecycle passes under test only touch m.db, so the redis client and
// alerter can stay nil.
func newSeriesLifecycleMatchmaker(db *sql.DB) *Matchmaker {
	return &Matchmaker{db: db}
}

// seedSeriesGame inserts one game of a series with its match, participants and
// job, returning the ids it generated so a test can mutate them.
func seedSeriesGame(t *testing.T, db *sql.DB, seriesID int64, gameNum int, botA, botB string, slotA, slotB int) (matchID, jobID string) {
	t.Helper()
	matchID = fmt.Sprintf("m_%d_%d", seriesID, gameNum)
	jobID = fmt.Sprintf("j_%d_%d", seriesID, gameNum)

	if _, err := db.Exec(`
		INSERT INTO matches (match_id, map_id, status) VALUES ($1, 'map_test', 'pending')
	`, matchID); err != nil {
		t.Fatalf("seed match: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO match_participants (match_id, bot_id, player_slot) VALUES ($1, $2, $3), ($1, $4, $5)
	`, matchID, botA, slotA, botB, slotB); err != nil {
		t.Fatalf("seed participants: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO jobs (job_id, match_id, status) VALUES ($1, $2, 'pending')
	`, jobID, matchID); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO series_games (series_id, match_id, game_num, map_id) VALUES ($1, $2, $3, 'map_test')
	`, seriesID, matchID, gameNum); err != nil {
		t.Fatalf("seed series_game: %v", err)
	}
	return matchID, jobID
}

// seedSeries inserts an active best-of-N series between two fresh bots.
func seedSeries(t *testing.T, db *sql.DB, tag string, format int) int64 {
	t.Helper()
	botA, botB := "bot_"+tag+"_a", "bot_"+tag+"_b"
	for _, id := range []string{botA, botB} {
		if _, err := db.Exec(`
			INSERT INTO bots (bot_id) VALUES ($1) ON CONFLICT (bot_id) DO NOTHING
		`, id); err != nil {
			t.Fatalf("seed bot: %v", err)
		}
	}
	var id int64
	if err := db.QueryRow(`
		INSERT INTO series (bot_a_id, bot_b_id, format, status) VALUES ($1, $2, $3, 'active') RETURNING id
	`, botA, botB, format).Scan(&id); err != nil {
		t.Fatalf("seed series: %v", err)
	}
	return id
}

// seriesState reads the series' status, winner and win tally.
func seriesState(t *testing.T, db *sql.DB, seriesID int64) (status, winnerID string, aWins, bWins int) {
	t.Helper()
	var winner sql.NullString
	if err := db.QueryRow(`
		SELECT status, winner_id, a_wins, b_wins FROM series WHERE id = $1
	`, seriesID).Scan(&status, &winner, &aWins, &bWins); err != nil {
		t.Fatalf("read series: %v", err)
	}
	return status, winner.String, aWins, bWins
}

// jobStatus reads one job's status.
func jobStatus(t *testing.T, db *sql.DB, jobID string) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM jobs WHERE job_id = $1`, jobID).Scan(&status); err != nil {
		t.Fatalf("read job %s: %v", jobID, err)
	}
	return status
}

// matchStatus reads one match's status.
func matchStatus(t *testing.T, db *sql.DB, matchID string) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM matches WHERE match_id = $1`, matchID).Scan(&status); err != nil {
		t.Fatalf("read match %s: %v", matchID, err)
	}
	return status
}

// TestCancelDecidedSeries_AbandonsSeriesWedgedByFailedGame covers the ordering
// gate's failure mode: every game of a series is created up front, so a game
// that fails permanently leaves the games behind it pending and unclaimable
// forever. The maintenance pass must abandon the series and retire those
// leftovers in the same pass.
func TestCancelDecidedSeries_AbandonsSeriesWedgedByFailedGame(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)
	setupSeriesTestSchema(t, db)
	ctx := context.Background()

	seriesID := seedSeries(t, db, "wedged", 3)
	_, job1 := seedSeriesGame(t, db, seriesID, 1, "bot_wedged_a", "bot_wedged_b", 0, 1)
	_, job2 := seedSeriesGame(t, db, seriesID, 2, "bot_wedged_a", "bot_wedged_b", 1, 0)
	_, job3 := seedSeriesGame(t, db, seriesID, 3, "bot_wedged_a", "bot_wedged_b", 0, 1)

	// Game 1 failed for good: no winner was ever recorded.
	if _, err := db.Exec(`
		UPDATE jobs SET status = 'failed', completed_at = NOW() WHERE job_id = $1
	`, job1); err != nil {
		t.Fatalf("fail game 1 job: %v", err)
	}

	m := newSeriesLifecycleMatchmaker(db)
	if err := m.cancelDecidedSeriesGames(ctx); err != nil {
		t.Fatalf("cancelDecidedSeriesGames: %v", err)
	}

	status, winner, aWins, bWins := seriesState(t, db, seriesID)
	if status != "completed" {
		t.Errorf("wedged series status = %q, want completed", status)
	}
	if winner != "" {
		t.Errorf("wedged 0-0 series winner = %q, want empty", winner)
	}
	if aWins != 0 || bWins != 0 {
		t.Errorf("wedged series tally = %d-%d, want 0-0", aWins, bWins)
	}
	if got := jobStatus(t, db, job2); got != "cancelled" {
		t.Errorf("game 2 job status = %q, want cancelled", got)
	}
	if got := jobStatus(t, db, job3); got != "cancelled" {
		t.Errorf("game 3 job status = %q, want cancelled", got)
	}
}

// TestCancelDecidedSeries_AbandonedSeriesGoesToLeader checks that a series
// abandoned mid-way is awarded to whoever was ahead when it wedged.
func TestCancelDecidedSeries_AbandonedSeriesGoesToLeader(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)
	setupSeriesTestSchema(t, db)
	ctx := context.Background()

	seriesID := seedSeries(t, db, "leader", 3)
	if _, err := db.Exec(`UPDATE series SET a_wins = 1 WHERE id = $1`, seriesID); err != nil {
		t.Fatalf("set tally: %v", err)
	}
	match1, _ := seedSeriesGame(t, db, seriesID, 1, "bot_leader_a", "bot_leader_b", 0, 1)
	seedSeriesGame(t, db, seriesID, 2, "bot_leader_a", "bot_leader_b", 1, 0)

	// Game 1 was won by bot A, game 2 failed for good.
	if _, err := db.Exec(`
		UPDATE matches SET status = 'completed', winner = 0, completed_at = NOW() WHERE match_id = $1
	`, match1); err != nil {
		t.Fatalf("complete game 1: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE series_games SET winner_id = 'bot_leader_a' WHERE series_id = $1 AND game_num = 1
	`, seriesID); err != nil {
		t.Fatalf("record game 1 winner: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE jobs SET status = 'failed', completed_at = NOW()
		WHERE match_id IN (SELECT sg.match_id FROM series_games sg WHERE sg.series_id = $1 AND sg.game_num = 2)
	`, seriesID); err != nil {
		t.Fatalf("fail game 2 job: %v", err)
	}

	m := newSeriesLifecycleMatchmaker(db)
	if err := m.cancelDecidedSeriesGames(ctx); err != nil {
		t.Fatalf("cancelDecidedSeriesGames: %v", err)
	}

	status, winner, _, _ := seriesState(t, db, seriesID)
	if status != "completed" {
		t.Errorf("abandoned series status = %q, want completed", status)
	}
	if winner != "bot_leader_a" {
		t.Errorf("abandoned series winner = %q, want bot_leader_a (the 1-0 leader)", winner)
	}
}

// TestCancelDecidedSeries_CancelsLeftoversOfFinishedSeries pins the cleanup the
// ordering gate depends on: a pending game on a series that is no longer active
// is not held back by anything, so it must be retired before it can run.
func TestCancelDecidedSeries_CancelsLeftoversOfFinishedSeries(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)
	setupSeriesTestSchema(t, db)
	ctx := context.Background()

	seriesID := seedSeries(t, db, "finished", 3)
	seedSeriesGame(t, db, seriesID, 1, "bot_finished_a", "bot_finished_b", 0, 1)
	_, job2 := seedSeriesGame(t, db, seriesID, 2, "bot_finished_a", "bot_finished_b", 1, 0)
	_, match3 := seedSeriesGame(t, db, seriesID, 3, "bot_finished_a", "bot_finished_b", 0, 1)

	// The series finished while games 2 and 3 were still pending — exactly the
	// state the worker's completion transaction normally avoids, and the state
	// finalizeCompletedSeries leaves behind when it decides a series itself.
	if _, err := db.Exec(`
		UPDATE series SET status = 'completed', winner_id = 'bot_finished_a', a_wins = 2 WHERE id = $1
	`, seriesID); err != nil {
		t.Fatalf("finish series: %v", err)
	}

	m := newSeriesLifecycleMatchmaker(db)
	if err := m.cancelDecidedSeriesGames(ctx); err != nil {
		t.Fatalf("cancelDecidedSeriesGames: %v", err)
	}

	if got := jobStatus(t, db, job2); got != "cancelled" {
		t.Errorf("leftover game 2 job status = %q, want cancelled", got)
	}
	if got := matchStatus(t, db, match3); got != "cancelled" {
		t.Errorf("leftover game 3 match status = %q, want cancelled", got)
	}
}

// TestUpdateSeriesGameResults_RecordsWinnerForActiveSeries guards against the
// active-series restriction overreaching: a normal result on a live series must
// still be recorded.
func TestUpdateSeriesGameResults_RecordsWinnerForActiveSeries(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)
	setupSeriesTestSchema(t, db)
	ctx := context.Background()

	seriesID := seedSeries(t, db, "live", 3)
	match1, _ := seedSeriesGame(t, db, seriesID, 1, "bot_live_a", "bot_live_b", 0, 1)

	if _, err := db.Exec(`
		UPDATE matches SET status = 'completed', winner = 1, completed_at = NOW() WHERE match_id = $1
	`, match1); err != nil {
		t.Fatalf("complete game 1: %v", err)
	}

	m := newSeriesLifecycleMatchmaker(db)
	if err := m.updateSeriesGameResults(ctx); err != nil {
		t.Fatalf("updateSeriesGameResults: %v", err)
	}

	status, _, aWins, bWins := seriesState(t, db, seriesID)
	if status != "active" {
		t.Errorf("series status = %q, want active after one game", status)
	}
	if aWins != 1 || bWins != 0 {
		t.Errorf("series tally = %d-%d, want 1-0 (winner was player slot 1 = bot B)", aWins, bWins)
	}
	var gameWinner sql.NullString
	if err := db.QueryRow(`
		SELECT winner_id FROM series_games WHERE series_id = $1 AND game_num = 1
	`, seriesID).Scan(&gameWinner); err != nil {
		t.Fatalf("read game 1 winner: %v", err)
	}
	if !gameWinner.Valid || gameWinner.String != "bot_live_b" {
		t.Errorf("game 1 winner = %v, want bot_live_b", gameWinner)
	}
}

// TestUpdateSeriesGameResults_IgnoresFinishedSeries covers the zombie-game
// hazard: once a series has left 'active', the ordering gate stops holding its
// remaining games back, so a result can arrive for a series that is already
// decided. It must not be folded into the final tally.
func TestUpdateSeriesGameResults_IgnoresFinishedSeries(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)
	setupSeriesTestSchema(t, db)
	ctx := context.Background()

	seriesID := seedSeries(t, db, "zombie", 3)
	match1, _ := seedSeriesGame(t, db, seriesID, 1, "bot_zombie_a", "bot_zombie_b", 0, 1)
	match2, _ := seedSeriesGame(t, db, seriesID, 2, "bot_zombie_a", "bot_zombie_b", 1, 0)

	// Series decided 2-0 after two games; a third result lands afterwards.
	if _, err := db.Exec(`
		UPDATE series SET status = 'completed', winner_id = 'bot_zombie_a', a_wins = 2 WHERE id = $1
	`, seriesID); err != nil {
		t.Fatalf("finish series: %v", err)
	}
	for _, id := range []string{match1, match2} {
		if _, err := db.Exec(`
			UPDATE matches SET status = 'completed', winner = 0, completed_at = NOW() WHERE match_id = $1
		`, id); err != nil {
			t.Fatalf("complete match %s: %v", id, err)
		}
	}

	m := newSeriesLifecycleMatchmaker(db)
	if err := m.updateSeriesGameResults(ctx); err != nil {
		t.Fatalf("updateSeriesGameResults: %v", err)
	}

	_, winner, aWins, bWins := seriesState(t, db, seriesID)
	if winner != "bot_zombie_a" {
		t.Errorf("series winner = %q, want bot_zombie_a", winner)
	}
	if aWins != 2 || bWins != 0 {
		t.Errorf("series tally = %d-%d, want the finalised 2-0", aWins, bWins)
	}
}

// TestSeriesGateKeepsLaterGamesPending pins the ordering gate end to end: with
// game 1 unresolved, a GetNextJob-shaped query must not hand out game 2, and it
// must hand it out once game 1 has a winner.
func TestSeriesGateKeepsLaterGamesPending(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)
	setupSeriesTestSchema(t, db)
	ctx := context.Background()

	seriesID := seedSeries(t, db, "gate", 3)
	match1, job1 := seedSeriesGame(t, db, seriesID, 1, "bot_gate_a", "bot_gate_b", 0, 1)
	_, job2 := seedSeriesGame(t, db, seriesID, 2, "bot_gate_a", "bot_gate_b", 1, 0)

	claimable := func() []string {
		t.Helper()
		rows, err := db.QueryContext(ctx, `
			SELECT job_id FROM jobs
			WHERE status = 'pending'
			  AND NOT EXISTS (`+seriesGateTestBlocking+`)
			ORDER BY created_at ASC
		`)
		if err != nil {
			t.Fatalf("gate query: %v", err)
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatalf("scan: %v", err)
			}
			ids = append(ids, id)
		}
		return ids
	}

	// Both games are pending, but only game 1 may run.
	got := claimable()
	if len(got) != 1 || got[0] != job1 {
		t.Errorf("claimable before game 1 resolves = %v, want [%s] only", got, job1)
	}

	// Game 1 completes with a winner; game 2 becomes claimable.
	if _, err := db.Exec(`
		UPDATE matches SET status = 'completed', winner = 0, completed_at = NOW() WHERE match_id = $1
	`, match1); err != nil {
		t.Fatalf("complete game 1: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE series_games SET winner_id = 'bot_gate_a' WHERE series_id = $1 AND game_num = 1
	`, seriesID); err != nil {
		t.Fatalf("record game 1 winner: %v", err)
	}

	got = claimable()
	if len(got) != 1 || got[0] != job2 {
		t.Errorf("claimable after game 1 resolves = %v, want [%s] only", got, job2)
	}
}

// seriesGateTestBlocking mirrors the worker's GetNextJob predicate so the gate
// can be exercised here without importing cmd/acb-worker. It is the same SQL
// seriesgate.Blocking emits.
const seriesGateTestBlocking = `SELECT 1
			FROM series_games mine
			JOIN series waited ON waited.id = mine.series_id AND waited.status = 'active'
			JOIN series_games earlier
			  ON earlier.series_id = mine.series_id
			 AND earlier.game_num < mine.game_num
			WHERE mine.match_id = jobs.match_id
			  AND earlier.winner_id IS NULL`
