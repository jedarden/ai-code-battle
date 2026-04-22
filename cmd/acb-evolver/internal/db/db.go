// Package db provides database access for the evolution pipeline.
package db

import (
	"context"
	"database/sql"
)

// schemaSQL creates the programs and validation_log tables with their indexes.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS programs (
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
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_programs_island ON programs(island);
CREATE INDEX IF NOT EXISTS idx_programs_island_fitness ON programs(island, fitness DESC);

CREATE TABLE IF NOT EXISTS validation_log (
    id          BIGSERIAL PRIMARY KEY,
    island      VARCHAR(16)  NOT NULL,
    language    VARCHAR(32)  NOT NULL,
    stage       VARCHAR(16)  NOT NULL,  -- last stage attempted: syntax / schema / sandbox
    passed      BOOLEAN      NOT NULL,
    error_text  TEXT         NOT NULL DEFAULT '',
    llm_output  TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_validation_log_island ON validation_log(island);
CREATE INDEX IF NOT EXISTS idx_validation_log_island_passed ON validation_log(island, passed);
`

// crosspollStateSQL creates the crosspoll_state table for persisting per-island
// last-pollinated generation numbers across evolver restarts.
const crosspollStateSQL = `
CREATE TABLE IF NOT EXISTS crosspoll_state (
    island             VARCHAR(16) PRIMARY KEY,
    last_pollinated_gen INTEGER NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

// migrationSQL holds additive migrations run after the base schema is ensured.
// Each statement is idempotent (ALTER TABLE … ADD COLUMN IF NOT EXISTS).
const migrationSQL = `
ALTER TABLE programs ADD COLUMN IF NOT EXISTS bot_id VARCHAR(16);
ALTER TABLE programs ADD COLUMN IF NOT EXISTS bot_name VARCHAR(64);
ALTER TABLE programs ADD COLUMN IF NOT EXISTS bot_secret TEXT;
`

// EnsureSchema creates the programs and validation_log tables if they do not
// already exist, then applies any pending additive migrations.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, crosspollStateSQL); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, migrationSQL)
	return err
}
