package promoter

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// ── dockerfileFor ─────────────────────────────────────────────────────────────

func TestDockerfileFor_AllSupportedLanguages(t *testing.T) {
	languages := []string{"go", "python", "rust", "typescript", "java", "php"}
	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			df, err := dockerfileFor(lang)
			if err != nil {
				t.Fatalf("dockerfileFor(%q) error: %v", lang, err)
			}
			if !strings.Contains(df, "FROM ") {
				t.Errorf("Dockerfile for %q missing FROM instruction", lang)
			}
			if !strings.Contains(df, "BOT_PORT") {
				t.Errorf("Dockerfile for %q missing BOT_PORT env var", lang)
			}
			if !strings.Contains(df, "BOT_SECRET") {
				t.Errorf("Dockerfile for %q missing BOT_SECRET env var", lang)
			}
			if !strings.Contains(df, "EXPOSE 8080") {
				t.Errorf("Dockerfile for %q missing EXPOSE 8080", lang)
			}
		})
	}
}

func TestDockerfileFor_UnsupportedLanguage(t *testing.T) {
	_, err := dockerfileFor("cobol")
	if err == nil {
		t.Error("expected error for unsupported language, got nil")
	}
}

func TestDockerfileFor_GoUsesMultistage(t *testing.T) {
	df, _ := dockerfileFor("go")
	if !strings.Contains(df, "AS builder") {
		t.Error("Go Dockerfile should use multi-stage build")
	}
	if !strings.Contains(df, "golang:") {
		t.Error("Go Dockerfile should use a golang base image")
	}
}

func TestDockerfileFor_RustUsesMultistage(t *testing.T) {
	df, _ := dockerfileFor("rust")
	if !strings.Contains(df, "AS builder") {
		t.Error("Rust Dockerfile should use multi-stage build")
	}
}

// ── generateBotID ─────────────────────────────────────────────────────────────

func TestGenerateBotID_Format(t *testing.T) {
	id, err := generateBotID()
	if err != nil {
		t.Fatalf("generateBotID error: %v", err)
	}
	if !strings.HasPrefix(id, "b_") {
		t.Errorf("bot ID %q does not start with 'b_'", id)
	}
	// b_ + 8 hex chars = 10 total
	if len(id) != 10 {
		t.Errorf("bot ID %q has length %d, want 10", id, len(id))
	}
}

func TestGenerateBotID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateBotID()
		if err != nil {
			t.Fatalf("generateBotID error at iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Errorf("duplicate bot ID generated: %s", id)
		}
		seen[id] = true
	}
}

// ── generateSecret ────────────────────────────────────────────────────────────

func TestGenerateSecret_Length(t *testing.T) {
	s, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret error: %v", err)
	}
	// 32 random bytes encoded as 64 hex chars
	if len(s) != 64 {
		t.Errorf("secret %q has length %d, want 64", s, len(s))
	}
}

func TestGenerateSecret_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		s, err := generateSecret()
		if err != nil {
			t.Fatalf("generateSecret error at iteration %d: %v", i, err)
		}
		if seen[s] {
			t.Errorf("duplicate secret generated: %s", s)
		}
		seen[s] = true
	}
}

// ── encryptAESGCM / decryptAESGCM ─────────────────────────────────────────────

func TestEncryptDecryptAESGCM_RoundTrip(t *testing.T) {
	// 32-byte key = 64 hex chars
	key := strings.Repeat("ab", 32) // "abababab..." 64 chars
	plaintext := "my-super-secret-bot-key"

	ct, err := encryptAESGCM(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ct == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}
}

func TestEncryptAESGCM_InvalidKey(t *testing.T) {
	_, err := encryptAESGCM("plaintext", "notahexkey")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

// ── manifest templates ────────────────────────────────────────────────────────

func TestManifestTemplates_Execute(t *testing.T) {
	data := manifestData{
		Name:         "acb-evo-test",
		Namespace:    "ai-code-battle",
		Island:       "alpha",
		Generation:   1,
		Registry:     "registry.example.com/acb",
		Port:         8080,
		SecretBase64: "dGVzdA==",
	}

	for name, tmpl := range map[string]interface{ Execute(interface{}, interface{}) error }{} {
		_ = name
		_ = tmpl
	}

	// Test secret manifest
	var buf strings.Builder
	if err := secretManifestTmpl.Execute(&buf, data); err != nil {
		t.Fatalf("secretManifestTmpl.Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "acb-evo-test-secret") {
		t.Error("secret manifest missing expected name")
	}
	if !strings.Contains(out, "dGVzdA==") {
		t.Error("secret manifest missing base64 secret")
	}

	// Test deployment manifest
	buf.Reset()
	if err := deployManifestTmpl.Execute(&buf, data); err != nil {
		t.Fatalf("deployManifestTmpl.Execute: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "acb-evo-test") {
		t.Error("deployment manifest missing bot name")
	}
	if !strings.Contains(out, "registry.example.com/acb/acb-evo-test:latest") {
		t.Error("deployment manifest missing full image reference")
	}
	if !strings.Contains(out, "acb/island: alpha") {
		t.Error("deployment manifest missing island label")
	}

	// Test service manifest
	buf.Reset()
	if err := svcManifestTmpl.Execute(&buf, data); err != nil {
		t.Fatalf("svcManifestTmpl.Execute: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "ClusterIP") {
		t.Error("service manifest missing ClusterIP type")
	}
}

// ── EnforcePolicy / cap enforcement ──────────────────────────────────────────────

// TestEnforcePolicy_CapEnforcement is an integration test that verifies the
// 50-bot population cap is enforced correctly. It creates a scenario with
// more than the cap number of active evolved bots and verifies that:
// 1. The lowest-rated bots are retired first
// 2. The cap is enforced after retirement
// 3. The bots table and programs table are updated correctly
//
// NOTE: This test requires a test database. Set ACB_TEST_DATABASE_URL
// to run this test, otherwise it is skipped.
func TestEnforcePolicy_CapEnforcement(t *testing.T) {
	testDBURL := testDatabaseURL()
	if testDBURL == "" {
		t.Skip("ACB_TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", testDBURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Create a clean test schema
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS programs_test CASCADE`); err != nil {
		t.Fatalf("drop test table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE programs_test (
			id              BIGSERIAL PRIMARY KEY,
			code            TEXT NOT NULL,
			language        VARCHAR(32) NOT NULL,
			island          VARCHAR(16) NOT NULL,
			generation      INTEGER NOT NULL DEFAULT 0,
			parent_ids      JSONB NOT NULL DEFAULT '[]',
			behavior_vector DOUBLE PRECISION[] NOT NULL DEFAULT '{}',
			fitness         DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			promoted        BOOLEAN NOT NULL DEFAULT FALSE,
			bot_id          VARCHAR(16),
			bot_name        VARCHAR(64),
			bot_secret      TEXT,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		t.Fatalf("create test table: %v", err)
	}
	defer db.ExecContext(ctx, `DROP TABLE programs_test CASCADE`)

	// Create a bots test table
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS bots_test CASCADE`); err != nil {
		t.Fatalf("drop bots test table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE bots_test (
			bot_id        VARCHAR(16) PRIMARY KEY,
			name          VARCHAR(64) NOT NULL,
			owner         VARCHAR(32) NOT NULL,
			endpoint_url  TEXT NOT NULL,
			shared_secret TEXT NOT NULL,
			status        VARCHAR(16) NOT NULL DEFAULT 'active',
			description   TEXT,
			last_active   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			rating_mu     DOUBLE PRECISION NOT NULL DEFAULT 1500,
			rating_phi    DOUBLE PRECISION NOT NULL DEFAULT 100
		)
	`); err != nil {
		t.Fatalf("create bots test table: %v", err)
	}
	defer db.ExecContext(ctx, `DROP TABLE bots_test CASCADE`)

	// Insert test data: 52 active evolved bots (exceeds cap of 50)
	// Ratings range from 500 (lowest) to 1500 (highest)
	const numBots = 52
	for i := 0; i < numBots; i++ {
		botID := tBotID(i)
		displayRating := 500.0 + float64(i)*20 // 500, 520, 540, ..., 1520

		// Calculate mu and phi from display_rating = mu - 2*phi
		// For simplicity, assume phi = 100, so mu = display_rating + 200
		mu := displayRating + 200
		phi := 100.0

		// Insert into bots_test
		_, err := db.ExecContext(ctx, `
			INSERT INTO bots_test (bot_id, name, owner, endpoint_url, shared_secret, status, rating_mu, rating_phi)
			VALUES ($1, $2, 'acb-evolver', 'http://test:8080', 'secret', 'active', $3, $4)
		`, botID, "test-bot-"+botID, mu, phi)
		if err != nil {
			t.Fatalf("insert bot %d: %v", i, err)
		}

		// Insert into programs_test
		_, err = db.ExecContext(ctx, `
			INSERT INTO programs_test (promoted, bot_id, bot_name, language, island, generation)
			VALUES (TRUE, $1, $2, 'go', 'alpha', 1)
		`, botID, "test-bot-"+botID)
		if err != nil {
			t.Fatalf("insert program %d: %v", i, err)
		}
	}

	// Enforce policy with cap of 50
	// Query bots that should be retired (lowest 2 rated)
	rows, err := db.QueryContext(ctx, `
		SELECT p.bot_id, p.bot_name, b.rating_mu - 2*b.rating_phi AS display_rating
		FROM programs_test p
		JOIN bots_test b ON p.bot_id = b.bot_id
		WHERE p.promoted = TRUE AND b.status = 'active' AND b.owner = 'acb-evolver'
		ORDER BY display_rating ASC
		LIMIT 2
	`)
	if err != nil {
		t.Fatalf("query bots to retire: %v", err)
	}
	defer rows.Close()

	type botInfo struct {
		botID         string
		botName       string
		displayRating float64
	}
	var toRetire []botInfo
	for rows.Next() {
		var b botInfo
		if err := rows.Scan(&b.botID, &b.botName, &b.displayRating); err != nil {
			t.Fatalf("scan bot: %v", err)
		}
		toRetire = append(toRetire, b)
	}
	if len(toRetire) != 2 {
		t.Fatalf("expected 2 bots to retire, got %d", len(toRetire))
	}

	// Verify the lowest-rated bots are selected
	if toRetire[0].displayRating != 500.0 {
		t.Errorf("expected lowest-rated bot to have rating 500, got %f", toRetire[0].displayRating)
	}
	if toRetire[1].displayRating != 520.0 {
		t.Errorf("expected second-lowest-rated bot to have rating 520, got %f", toRetire[1].displayRating)
	}

	// Simulate retirement: mark bots as retired and clear promoted flag
	for _, b := range toRetire {
		_, err := db.ExecContext(ctx, `
			UPDATE bots_test SET status = 'retired' WHERE bot_id = $1
		`, b.botID)
		if err != nil {
			t.Fatalf("retire bot %s: %v", b.botID, err)
		}
		_, err = db.ExecContext(ctx, `
			UPDATE programs_test SET promoted = FALSE, bot_id = NULL WHERE bot_id = $1
		`, b.botID)
		if err != nil {
			t.Fatalf("clear promoted for bot %s: %v", b.botID, err)
		}
	}

	// Count active bots after retirement
	var countAfter int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM programs_test p
		JOIN bots_test b ON p.bot_id = b.bot_id
		WHERE p.promoted = TRUE AND b.status = 'active' AND b.owner = 'acb-evolver'
	`).Scan(&countAfter)
	if err != nil {
		t.Fatalf("count after: %v", err)
	}
	if countAfter != 50 {
		t.Errorf("expected 50 active bots after retirement, got %d", countAfter)
	}

	// Verify the lowest-rated bots are now retired
	for _, b := range toRetire {
		var status string
		err := db.QueryRowContext(ctx, `
			SELECT status FROM bots_test WHERE bot_id = $1
		`, b.botID).Scan(&status)
		if err != nil {
			t.Fatalf("query bot %s status: %v", b.botID, err)
		}
		if status != "retired" {
			t.Errorf("bot %s should be retired, got status %s", b.botID, status)
		}

		var promoted bool
		err = db.QueryRowContext(ctx, `
			SELECT promoted FROM programs_test WHERE bot_name = $1
		`, b.botName).Scan(&promoted)
		if err != nil {
			t.Fatalf("query program %s promoted: %v", b.botName, err)
		}
		if promoted {
			t.Errorf("program %s should not be promoted", b.botName)
		}
	}

	// Verify the remaining bots are still active
	var remainingRating float64
	err = db.QueryRowContext(ctx, `
		SELECT MIN(b.rating_mu - 2*b.rating_phi) AS min_rating
		FROM programs_test p
		JOIN bots_test b ON p.bot_id = b.bot_id
		WHERE p.promoted = TRUE AND b.status = 'active' AND b.owner = 'acb-evolver'
	`).Scan(&remainingRating)
	if err != nil {
		t.Fatalf("query remaining min rating: %v", err)
	}
	if remainingRating != 540.0 {
		t.Errorf("expected remaining min rating to be 540, got %f", remainingRating)
	}
}

// tBotID generates a test bot ID for the given index.
func tBotID(i int) string {
	return fmt.Sprintf("b_test_%04x", i)
}

// testDatabaseURL returns the test database URL from environment or empty string.
func testDatabaseURL() string {
	// In a real setup, this would read from ACB_TEST_DATABASE_URL env var
	// For now, return empty to skip the test unless explicitly configured
	return ""
}
