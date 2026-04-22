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

// openTestDB opens a connection to the test database if ACB_TEST_DATABASE_URL is set.
// Returns nil if no test database is configured.
func openTestDB(t *testing.T) *sql.DB {
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

// setupTestSchema creates the bots table used by crash strike tests.
func setupTestSchema(t *testing.T, db *sql.DB) {
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
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
}

// insertTestBot inserts a bot with the given ID and crash_strikes/cooldown state.
func insertTestBot(t *testing.T, db *sql.DB, botID string, strikes int, cooldownUntil *time.Time) {
	t.Helper()
	var cooldownVal interface{}
	if cooldownUntil != nil {
		cooldownVal = *cooldownUntil
	}
	_, err := db.Exec(`
		INSERT INTO bots (bot_id, name, crash_strikes, cooldown_until)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (bot_id) DO UPDATE SET
			crash_strikes = $3, cooldown_until = $4
	`, botID, botID, strikes, cooldownVal)
	if err != nil {
		t.Fatalf("insert bot %s: %v", botID, err)
	}
}

// getBotStrikes reads crash_strikes and cooldown_until for a bot.
func getBotStrikes(t *testing.T, db *sql.DB, botID string) (int, *time.Time) {
	t.Helper()
	var strikes int
	var cooldown *time.Time
	err := db.QueryRow(`SELECT crash_strikes, cooldown_until FROM bots WHERE bot_id = $1`, botID).Scan(&strikes, &cooldown)
	if err != nil {
		t.Fatalf("get bot %s: %v", botID, err)
	}
	return strikes, cooldown
}

// TestCrashStrikesConstants verifies the constants match the spec (§4.5, §6.1).
func TestCrashStrikesConstants(t *testing.T) {
	if MaxCrashStrikes != 3 {
		t.Errorf("MaxCrashStrikes = %d, want 3", MaxCrashStrikes)
	}
	if CrashCooldownDuration != 30*time.Minute {
		t.Errorf("CrashCooldownDuration = %v, want 30m", CrashCooldownDuration)
	}
}

// TestCrashStrikes_SingleCrashNoCooldown tests that a single crash does NOT trigger cooldown.
func TestCrashStrikes_SingleCrashNoCooldown(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	setupTestSchema(t, db)

	botID := fmt.Sprintf("b_%s", t.Name())
	insertTestBot(t, db, botID, 0, nil)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	strikes, cooldown := getBotStrikes(t, db, botID)
	if strikes != 1 {
		t.Errorf("strikes after 1 crash = %d, want 1", strikes)
	}
	if cooldown != nil {
		t.Errorf("cooldown after 1 crash = %v, want nil", cooldown)
	}
}

// TestCrashStrikes_TwoCrashesNoCooldown tests that 2 crashes do NOT trigger cooldown.
func TestCrashStrikes_TwoCrashesNoCooldown(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	setupTestSchema(t, db)

	botID := fmt.Sprintf("b_%s", t.Name())
	insertTestBot(t, db, botID, 0, nil)

	ctx := context.Background()

	// First crash
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// Second crash
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	strikes, cooldown := getBotStrikes(t, db, botID)
	if strikes != 2 {
		t.Errorf("strikes after 2 crashes = %d, want 2", strikes)
	}
	if cooldown != nil {
		t.Errorf("cooldown after 2 crashes = %v, want nil", cooldown)
	}
}

// TestCrashStrikes_ThreeCrashesTriggerCooldown tests that 3 consecutive crashes trigger the 30-min cooldown.
func TestCrashStrikes_ThreeCrashesTriggerCooldown(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	setupTestSchema(t, db)

	botID := fmt.Sprintf("b_%s", t.Name())
	insertTestBot(t, db, botID, 0, nil)

	ctx := context.Background()

	for crashNum := range 3 {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
		if err != nil {
			t.Fatalf("crash %d: %v", crashNum+1, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	strikes, cooldown := getBotStrikes(t, db, botID)
	if strikes != 3 {
		t.Errorf("strikes after 3 crashes = %d, want 3", strikes)
	}
	if cooldown == nil {
		t.Fatal("cooldown should be set after 3 consecutive crashes, got nil")
	}

	// Cooldown should be approximately NOW() + 30min (allow 2 second tolerance)
	expectedMin := time.Now().Add(CrashCooldownDuration).Add(-2 * time.Second)
	expectedMax := time.Now().Add(CrashCooldownDuration).Add(2 * time.Second)
	if cooldown.Before(expectedMin) || cooldown.After(expectedMax) {
		t.Errorf("cooldown = %v, want approximately %v", cooldown, time.Now().Add(CrashCooldownDuration))
	}
}

// TestCrashStrikes_SuccessResetsStrikes tests that a successful match resets the strike counter.
func TestCrashStrikes_SuccessResetsStrikes(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	setupTestSchema(t, db)

	botID := fmt.Sprintf("b_%s", t.Name())
	insertTestBot(t, db, botID, 2, nil) // 2 strikes already

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Successful match resets strikes
	err = updateCrashStrikes(ctx, tx, map[string]bool{botID: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	strikes, cooldown := getBotStrikes(t, db, botID)
	if strikes != 0 {
		t.Errorf("strikes after successful match = %d, want 0", strikes)
	}
	if cooldown != nil {
		t.Errorf("cooldown after successful match = %v, want nil", cooldown)
	}
}

// TestCrashStrikes_InterleavedResets tests that a success between crashes resets the counter.
func TestCrashStrikes_InterleavedResets(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	setupTestSchema(t, db)

	botID := fmt.Sprintf("b_%s", t.Name())
	insertTestBot(t, db, botID, 0, nil)

	ctx := context.Background()

	// Crash twice
	for range 2 {
		tx, _ := db.BeginTx(ctx, nil)
		_ = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
		_ = tx.Commit()
	}

	// Succeed (resets strikes to 0)
	tx, _ := db.BeginTx(ctx, nil)
	_ = updateCrashStrikes(ctx, tx, map[string]bool{botID: false})
	_ = tx.Commit()

	// Crash once more — should be strike 1, not 3
	tx, _ = db.BeginTx(ctx, nil)
	_ = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
	_ = tx.Commit()

	strikes, cooldown := getBotStrikes(t, db, botID)
	if strikes != 1 {
		t.Errorf("strikes after crash-success-crash = %d, want 1", strikes)
	}
	if cooldown != nil {
		t.Errorf("cooldown after crash-success-crash = %v, want nil", cooldown)
	}
}

// TestCrashStrikes_CooldownExtendsOnRepeatedCrash tests that crashing again while on cooldown
// extends the cooldown (re-triggers the 30-min timer).
func TestCrashStrikes_CooldownExtendsOnRepeatedCrash(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	setupTestSchema(t, db)

	botID := fmt.Sprintf("b_%s", t.Name())
	insertTestBot(t, db, botID, 0, nil)

	ctx := context.Background()

	// Accumulate 3 strikes to trigger initial cooldown
	for range 3 {
		tx, _ := db.BeginTx(ctx, nil)
		_ = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
		_ = tx.Commit()
	}

	_, firstCooldown := getBotStrikes(t, db, botID)
	if firstCooldown == nil {
		t.Fatal("expected initial cooldown to be set")
	}

	// Wait a tiny bit, then crash again — cooldown should extend
	time.Sleep(100 * time.Millisecond)
	tx, _ := db.BeginTx(ctx, nil)
	_ = updateCrashStrikes(ctx, tx, map[string]bool{botID: true})
	_ = tx.Commit()

	strikes, secondCooldown := getBotStrikes(t, db, botID)
	if strikes != 4 {
		t.Errorf("strikes after 4th crash = %d, want 4", strikes)
	}
	if secondCooldown == nil {
		t.Fatal("expected cooldown to still be set after 4th crash")
	}
	// Cooldown should have been extended (later than the first one)
	if !secondCooldown.After(*firstCooldown) {
		t.Errorf("cooldown should have been extended: first=%v, second=%v", firstCooldown, secondCooldown)
	}
}

// TestCrashStrikes_MultipleBots tests updating crash strikes for multiple bots in one call.
func TestCrashStrikes_MultipleBots(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	setupTestSchema(t, db)

	crashBot := fmt.Sprintf("b_%s_crash", t.Name())
	okBot := fmt.Sprintf("b_%s_ok", t.Name())
	insertTestBot(t, db, crashBot, 0, nil)
	insertTestBot(t, db, okBot, 1, nil)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = updateCrashStrikes(ctx, tx, map[string]bool{
		crashBot: true,
		okBot:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	crashStrikes, crashCooldown := getBotStrikes(t, db, crashBot)
	okStrikes, okCooldown := getBotStrikes(t, db, okBot)

	if crashStrikes != 1 {
		t.Errorf("crashed bot strikes = %d, want 1", crashStrikes)
	}
	if crashCooldown != nil {
		t.Errorf("crashed bot cooldown after 1 strike = %v, want nil", crashCooldown)
	}
	if okStrikes != 0 {
		t.Errorf("ok bot strikes = %d, want 0 (reset)", okStrikes)
	}
	if okCooldown != nil {
		t.Errorf("ok bot cooldown = %v, want nil", okCooldown)
	}
}

// TestCrashStrikes_EmptyMap tests that calling updateCrashStrikes with an empty map is a no-op.
func TestCrashStrikes_EmptyMap(t *testing.T) {
	ctx := context.Background()
	err := updateCrashStrikes(ctx, nil, map[string]bool{})
	if err != nil {
		t.Errorf("expected nil error for empty map, got: %v", err)
	}
}
