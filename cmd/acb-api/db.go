package main

import (
	"context"
	"database/sql"
)

const schemaSQL = `
-- ---- Phase 9 tables ----

CREATE TABLE IF NOT EXISTS predictions (
    id            BIGSERIAL PRIMARY KEY,
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    predictor_id  VARCHAR(64) NOT NULL,
    predicted_bot VARCHAR(16) NOT NULL,
    correct       BOOLEAN,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ,
    UNIQUE(match_id, predictor_id)
);
CREATE INDEX IF NOT EXISTS idx_predictions_match ON predictions(match_id);
CREATE INDEX IF NOT EXISTS idx_predictions_predictor ON predictions(predictor_id);

CREATE TABLE IF NOT EXISTS predictor_stats (
    predictor_id  VARCHAR(64) PRIMARY KEY,
    correct       INTEGER NOT NULL DEFAULT 0,
    incorrect     INTEGER NOT NULL DEFAULT 0,
    streak        INTEGER NOT NULL DEFAULT 0,
    best_streak   INTEGER NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS series (
    id         BIGSERIAL PRIMARY KEY,
    bot_a_id   VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    bot_b_id   VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    format     INTEGER NOT NULL DEFAULT 5,  -- best of N (3, 5, 7...)
    a_wins     INTEGER NOT NULL DEFAULT 0,
    b_wins     INTEGER NOT NULL DEFAULT 0,
    status     VARCHAR(16) NOT NULL DEFAULT 'active',
    winner_id  VARCHAR(16),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_series_bots ON series(bot_a_id, bot_b_id);
CREATE INDEX IF NOT EXISTS idx_series_status ON series(status);

CREATE TABLE IF NOT EXISTS series_games (
    id        BIGSERIAL PRIMARY KEY,
    series_id BIGINT NOT NULL REFERENCES series(id),
    match_id  VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    game_num  INTEGER NOT NULL,
    winner_id VARCHAR(16),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_series_games_series ON series_games(series_id);

CREATE TABLE IF NOT EXISTS seasons (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(64) NOT NULL,
    theme         VARCHAR(128),
    rules_version VARCHAR(32) NOT NULL DEFAULT '1.0',
    status        VARCHAR(16) NOT NULL DEFAULT 'active',
    champion_id   VARCHAR(16),
    starts_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS season_snapshots (
    id         BIGSERIAL PRIMARY KEY,
    season_id  BIGINT NOT NULL REFERENCES seasons(id),
    bot_id     VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    rank       INTEGER NOT NULL,
    rating     DOUBLE PRECISION NOT NULL,
    wins       INTEGER NOT NULL DEFAULT 0,
    losses     INTEGER NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_season_snapshots_season ON season_snapshots(season_id, rank);

-- Map engagement scores (written by acb-mapgen or evolution pipeline)
CREATE TABLE IF NOT EXISTS map_scores (
    map_id          VARCHAR(32) PRIMARY KEY,
    engagement      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    symmetry_score  DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    wall_density    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    last_used_at    TIMESTAMPTZ,
    match_count     INTEGER NOT NULL DEFAULT 0,
    avg_turns       DOUBLE PRECISION,
    scored_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Map lifecycle management (§14.6 Map Evolution)
CREATE TABLE IF NOT EXISTS maps (
    map_id          VARCHAR(32) PRIMARY KEY,
    player_count    INTEGER NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active',  -- active, probation, retired, classic
    engagement      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    wall_density    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    energy_count    INTEGER NOT NULL DEFAULT 0,
    grid_width      INTEGER NOT NULL,
    grid_height     INTEGER NOT NULL,
    map_json        JSONB NOT NULL,  -- Full map layout with walls, energy, cores
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retired_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_maps_status ON maps(status, player_count);
CREATE INDEX IF NOT EXISTS idx_maps_engagement ON maps(player_count, engagement DESC);

-- User voting on maps (§14.6 Map Evolution)
CREATE TABLE IF NOT EXISTS map_votes (
    id          BIGSERIAL PRIMARY KEY,
    map_id      VARCHAR(32) NOT NULL REFERENCES maps(map_id) ON DELETE CASCADE,
    voter_id    VARCHAR(64) NOT NULL,  -- localStorage UUID
    vote        SMALLINT NOT NULL,  -- +1 or -1
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(map_id, voter_id)
);
CREATE INDEX IF NOT EXISTS idx_map_votes_map ON map_votes(map_id);

-- Positional fairness tracking (§14.6 Map Evolution)
CREATE TABLE IF NOT EXISTS map_fairness (
    map_id      VARCHAR(32) NOT NULL REFERENCES maps(map_id) ON DELETE CASCADE,
    player_slot INTEGER NOT NULL,
    games       INTEGER NOT NULL DEFAULT 0,
    wins        INTEGER NOT NULL DEFAULT 0,
    last_check  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (map_id, player_slot)
);

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
    consec_fails  INTEGER NOT NULL DEFAULT 0,
    archetype     VARCHAR(64)
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
    completed_at  TIMESTAMPTZ,
    heartbeat_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rating_history (
    bot_id        VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    rating        DOUBLE PRECISION NOT NULL,
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, match_id)
);
CREATE INDEX IF NOT EXISTS idx_rating_history_bot ON rating_history(bot_id, recorded_at);

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
`

func ensureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schemaSQL)
	return err
}
