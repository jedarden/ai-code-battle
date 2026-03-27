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

// EnsureSchema creates the programs table if it does not already exist.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schemaSQL)
	return err
}
