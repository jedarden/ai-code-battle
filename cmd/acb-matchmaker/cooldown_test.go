package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// openTestDBMatchmaker opens a test database for matchmaker tests.
func openTestDBMatchmaker(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("ACB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ACB_TEST_DATABASE_URL not set, skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.SetMaxOpenConns(2)
	return db
}

// setupMatchmakerTestSchema creates the tables needed for matchmaker tests.
func setupMatchmakerTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS bots (
			bot_id        VARCHAR(16) PRIMARY KEY,
			name          VARCHAR(32) UNIQUE NOT NULL,
			owner         VARCHAR(128) NOT NULL DEFAULT 'test',
			endpoint_url  TEXT NOT NULL DEFAULT 'http://localhost:8080',
			shared_secret TEXT NOT NULL DEFAULT 'secret',
			status        VARCHAR(16) NOT NULL DEFAULT 'active',
			rating_mu     DOUBLE PRECISION NOT NULL DEFAULT 1500.0,
			rating_phi    DOUBLE PRECISION NOT NULL DEFAULT 350.0,
			rating_sigma  DOUBLE PRECISION NOT NULL DEFAULT 0.06,
			evolved       BOOLEAN NOT NULL DEFAULT FALSE,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			crash_strikes INTEGER NOT NULL DEFAULT 0,
			cooldown_until TIMESTAMPTZ
		);

		CREATE TABLE IF NOT EXISTS maps (
			map_id    VARCHAR(32) PRIMARY KEY,
			grid_width  INTEGER NOT NULL DEFAULT 60,
			grid_height INTEGER NOT NULL DEFAULT 60,
			map_json  JSONB NOT NULL DEFAULT '{}'
		);
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
	// Seed a map for match creation
	_, _ = db.Exec(`INSERT INTO maps (map_id) VALUES ('map_test') ON CONFLICT DO NOTHING`)
}

// insertMMTestBot inserts a bot with specified status and cooldown state.
func insertMMTestBot(t *testing.T, db *sql.DB, botID string, status string, strikes int, cooldownUntil *time.Time) {
	t.Helper()
	var cooldownVal interface{}
	if cooldownUntil != nil {
		cooldownVal = *cooldownUntil
	}
	_, err := db.Exec(`
		INSERT INTO bots (bot_id, name, status, crash_strikes, cooldown_until)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (bot_id) DO UPDATE SET
			status = $3, crash_strikes = $4, cooldown_until = $5
	`, botID, botID, status, strikes, cooldownVal)
	if err != nil {
		t.Fatalf("insert bot %s: %v", botID, err)
	}
}

// TestMatchmakerQuery_ExcludesCooldown tests that the matchmaker eligibility query
// excludes bots whose cooldown_until is in the future.
func TestMatchmakerQuery_ExcludesCooldown(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)

	ctx := context.Background()

	// Insert 3 bots: one active, one active but on cooldown, one inactive
	activeBot := fmt.Sprintf("b_%s_active", t.Name())
	cooldownBot := fmt.Sprintf("b_%s_cool", t.Name())
	inactiveBot := fmt.Sprintf("b_%s_inact", t.Name())

	future := time.Now().Add(30 * time.Minute) // cooldown in the future

	insertMMTestBot(t, db, activeBot, "active", 0, nil)
	insertMMTestBot(t, db, cooldownBot, "active", 3, &future)
	insertMMTestBot(t, db, inactiveBot, "inactive", 0, nil)

	// This is the same query used in tickMatchmaker
	rows, err := db.QueryContext(ctx, `
		SELECT bot_id FROM bots WHERE status = 'active'
		AND (cooldown_until IS NULL OR cooldown_until < NOW())
		ORDER BY rating_mu DESC
	`)
	if err != nil {
		t.Fatal(err)
	}

	var eligible []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		eligible = append(eligible, id)
	}
	rows.Close()

	// Only activeBot should be eligible
	for _, id := range eligible {
		if id == cooldownBot {
			t.Error("bot on cooldown should NOT be eligible for pairing")
		}
		if id == inactiveBot {
			t.Error("inactive bot should NOT be eligible for pairing")
		}
	}

	found := false
	for _, id := range eligible {
		if id == activeBot {
			found = true
		}
	}
	if !found {
		t.Error("active bot without cooldown should be eligible for pairing")
	}
}

// TestMatchmakerQuery_CooldownExpired tests that a bot whose cooldown has expired
// is eligible for pairing again.
func TestMatchmakerQuery_CooldownExpired(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)

	ctx := context.Background()

	botID := fmt.Sprintf("b_%s", t.Name())
	past := time.Now().Add(-1 * time.Second) // cooldown expired

	insertMMTestBot(t, db, botID, "active", 3, &past)

	rows, err := db.QueryContext(ctx, `
		SELECT bot_id FROM bots WHERE status = 'active'
		AND (cooldown_until IS NULL OR cooldown_until < NOW())
	`)
	if err != nil {
		t.Fatal(err)
	}

	var eligible []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		eligible = append(eligible, id)
	}
	rows.Close()

	if len(eligible) != 1 || eligible[0] != botID {
		t.Errorf("bot with expired cooldown should be eligible, got: %v", eligible)
	}
}

// TestMatchmakerQuery_NoCooldown tests that a bot with NULL cooldown_until is eligible.
func TestMatchmakerQuery_NoCooldown(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)

	ctx := context.Background()

	botID := fmt.Sprintf("b_%s", t.Name())
	insertMMTestBot(t, db, botID, "active", 1, nil) // 1 strike, no cooldown

	rows, err := db.QueryContext(ctx, `
		SELECT bot_id FROM bots WHERE status = 'active'
		AND (cooldown_until IS NULL OR cooldown_until < NOW())
	`)
	if err != nil {
		t.Fatal(err)
	}

	var eligible []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		eligible = append(eligible, id)
	}
	rows.Close()

	if len(eligible) != 1 || eligible[0] != botID {
		t.Errorf("bot with no cooldown should be eligible, got: %v", eligible)
	}
}

// TestMatchmakerQuery_SeriesEligibility tests the series scheduling cooldown filter.
func TestMatchmakerQuery_SeriesEligibility(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)

	ctx := context.Background()

	cooldownBot := fmt.Sprintf("b_%s_cool", t.Name())
	okBot := fmt.Sprintf("b_%s_ok", t.Name())

	future := time.Now().Add(30 * time.Minute)
	insertMMTestBot(t, db, cooldownBot, "active", 3, &future)
	insertMMTestBot(t, db, okBot, "active", 0, nil)

	// This is the same query used in scheduleNextSeriesGames
	var eligible bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM bots WHERE bot_id = $1 AND status = 'active'
		 AND (cooldown_until IS NULL OR cooldown_until < NOW()))
	`, cooldownBot).Scan(&eligible)
	if err != nil {
		t.Fatal(err)
	}
	if eligible {
		t.Error("bot on cooldown should NOT be eligible for series games")
	}

	err = db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM bots WHERE bot_id = $1 AND status = 'active'
		 AND (cooldown_until IS NULL OR cooldown_until < NOW()))
	`, okBot).Scan(&eligible)
	if err != nil {
		t.Fatal(err)
	}
	if !eligible {
		t.Error("active bot without cooldown should be eligible for series games")
	}
}

// TestCrashCooldownIntegration tests the full flow: crash 3 matches → cooldown → exclude from pairing → cooldown expires → re-pair.
// This uses direct SQL to simulate what updateCrashStrikes does in the worker, verifying the matchmaker
// query responds correctly to each state transition.
func TestCrashCooldownIntegration(t *testing.T) {
	db := openTestDBMatchmaker(t)
	defer db.Close()
	setupMatchmakerTestSchema(t, db)

	ctx := context.Background()
	botID := fmt.Sprintf("b_%s", t.Name())
	insertMMTestBot(t, db, botID, "active", 0, nil)

	// Simulate 3 consecutive crashes using the same SQL logic as updateCrashStrikes
	maxStrikes := 3
	cooldownDur := 30 * time.Minute
	for i := range 3 {
		_, err := db.ExecContext(ctx, `
			UPDATE bots
			SET crash_strikes = crash_strikes + 1,
			    cooldown_until = CASE
			        WHEN crash_strikes + 1 >= $1 THEN NOW() + $2
			        ELSE cooldown_until
			    END
			WHERE bot_id = $3
		`, maxStrikes, cooldownDur, botID)
		if err != nil {
			t.Fatalf("crash %d: %v", i+1, err)
		}
	}

	// Verify bot is on cooldown
	var strikes int
	var cooldown *time.Time
	err := db.QueryRowContext(ctx, `SELECT crash_strikes, cooldown_until FROM bots WHERE bot_id = $1`, botID).Scan(&strikes, &cooldown)
	if err != nil {
		t.Fatal(err)
	}
	if strikes != 3 {
		t.Fatalf("strikes = %d, want 3", strikes)
	}
	if cooldown == nil {
		t.Fatal("cooldown should be set after 3 crashes")
	}

	// Verify bot is excluded from matchmaker eligibility
	var eligible bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM bots WHERE bot_id = $1 AND status = 'active'
		 AND (cooldown_until IS NULL OR cooldown_until < NOW()))
	`, botID).Scan(&eligible)
	if err != nil {
		t.Fatal(err)
	}
	if eligible {
		t.Error("bot should NOT be eligible for pairing while on cooldown")
	}

	// Simulate cooldown expiry by setting cooldown_until to the past
	_, err = db.ExecContext(ctx, `UPDATE bots SET cooldown_until = NOW() - INTERVAL '1 second' WHERE bot_id = $1`, botID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify bot is eligible again after cooldown expires
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM bots WHERE bot_id = $1 AND status = 'active'
		 AND (cooldown_until IS NULL OR cooldown_until < NOW()))
	`, botID).Scan(&eligible)
	if err != nil {
		t.Fatal(err)
	}
	if !eligible {
		t.Error("bot should be eligible for pairing after cooldown expires")
	}

	// Simulate a successful match — strikes should reset
	_, err = db.ExecContext(ctx, `UPDATE bots SET crash_strikes = 0 WHERE bot_id = $1`, botID)
	if err != nil {
		t.Fatal(err)
	}

	err = db.QueryRowContext(ctx, `SELECT crash_strikes FROM bots WHERE bot_id = $1`, botID).Scan(&strikes)
	if err != nil {
		t.Fatal(err)
	}
	if strikes != 0 {
		t.Errorf("strikes after successful match = %d, want 0", strikes)
	}
}
