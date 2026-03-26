package main

import (
	"context"
	"database/sql"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS bots (
    bot_id        VARCHAR(16) PRIMARY KEY,
    name          VARCHAR(32) UNIQUE NOT NULL,
    owner         VARCHAR(128) NOT NULL,
    endpoint_url  TEXT NOT NULL,
    shared_secret TEXT NOT NULL,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    rating_mu     DOUBLE PRECISION NOT NULL DEFAULT 1500.0,
    rating_phi    DOUBLE PRECISION NOT NULL DEFAULT 350.0,
    rating_sigma  DOUBLE PRECISION NOT NULL DEFAULT 0.06,
    evolved       BOOLEAN NOT NULL DEFAULT FALSE,
    island        VARCHAR(16),
    generation    INTEGER,
    parent_ids    JSONB,
    description   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active   TIMESTAMPTZ,
    consec_fails  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS matches (
    match_id      VARCHAR(32) PRIMARY KEY,
    map_id        VARCHAR(32) NOT NULL,
    map_seed      BIGINT,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    winner        INTEGER,
    condition     VARCHAR(32),
    turn_count    INTEGER,
    scores_json   JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS match_participants (
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    bot_id        VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    player_slot   INTEGER NOT NULL,
    score         INTEGER,
    status        VARCHAR(16),
    PRIMARY KEY (match_id, bot_id)
);

CREATE TABLE IF NOT EXISTS jobs (
    job_id        VARCHAR(32) PRIMARY KEY,
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    worker_id     VARCHAR(64),
    config_json   JSONB NOT NULL,
    claimed_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS rating_history (
    bot_id        VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    rating        DOUBLE PRECISION NOT NULL,
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, match_id)
);
CREATE INDEX IF NOT EXISTS idx_rating_history_bot ON rating_history(bot_id, recorded_at);
`

func ensureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schemaSQL)
	return err
}
