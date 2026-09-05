-- Initial schema for AI Code Battle
-- This migration creates all core tables for the application
-- Includes: bots, matches, jobs, ratings, seasons, series, maps, programs, playlists, feedback, enrichment

-- =====================================================
-- Core Tables
-- =====================================================

CREATE TABLE bots (
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
    archetype     VARCHAR(64),
    crash_strikes INTEGER NOT NULL DEFAULT 0,
    cooldown_until TIMESTAMPTZ,
    debug_public  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE matches (
    match_id      VARCHAR(32) PRIMARY KEY,
    map_id        VARCHAR(32) NOT NULL,
    map_seed      BIGINT,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    winner        INTEGER,
    condition     VARCHAR(32),
    turn_count    INTEGER,
    combat_turns  INTEGER NOT NULL DEFAULT 0,
    scores_json   JSONB,
    enrichment_requested_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);

CREATE TABLE match_participants (
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    bot_id        VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    player_slot   INTEGER NOT NULL,
    score         INTEGER,
    status        VARCHAR(16),
    PRIMARY KEY (match_id, bot_id)
);

CREATE TABLE jobs (
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

CREATE TABLE rating_history (
    bot_id        VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    rating        DOUBLE PRECISION NOT NULL,
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bot_id, match_id)
);
CREATE INDEX idx_rating_history_bot ON rating_history(bot_id, recorded_at);

-- =====================================================
-- Season & Series Tables (Phase 9)
-- =====================================================

CREATE TABLE seasons (
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

CREATE TABLE season_snapshots (
    id         BIGSERIAL PRIMARY KEY,
    season_id  BIGINT NOT NULL REFERENCES seasons(id),
    bot_id     VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    rank       INTEGER NOT NULL,
    rating     DOUBLE PRECISION NOT NULL,
    wins       INTEGER NOT NULL DEFAULT 0,
    losses     INTEGER NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_season_snapshots_season ON season_snapshots(season_id, rank);

CREATE TABLE series (
    id               BIGSERIAL PRIMARY KEY,
    bot_a_id         VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    bot_b_id         VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    format           INTEGER NOT NULL DEFAULT 5,
    a_wins           INTEGER NOT NULL DEFAULT 0,
    b_wins           INTEGER NOT NULL DEFAULT 0,
    status           VARCHAR(16) NOT NULL DEFAULT 'active',
    winner_id        VARCHAR(16),
    season_id        BIGINT REFERENCES seasons(id),
    bracket_round    VARCHAR(32),
    bracket_position INTEGER,
    featured         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_series_bots ON series(bot_a_id, bot_b_id);
CREATE INDEX idx_series_status ON series(status);
CREATE INDEX idx_series_season ON series(season_id);
CREATE INDEX idx_series_bracket ON series(season_id, bracket_round) WHERE bracket_round IS NOT NULL;
CREATE INDEX idx_series_featured ON series(featured, created_at DESC) WHERE featured = TRUE;

CREATE TABLE series_games (
    id        BIGSERIAL PRIMARY KEY,
    series_id BIGINT NOT NULL REFERENCES series(id),
    match_id  VARCHAR(32) REFERENCES matches(match_id),
    game_num  INTEGER NOT NULL,
    map_id    VARCHAR(64),
    winner_id VARCHAR(16),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_series_games_series ON series_games(series_id);

-- =====================================================
-- Prediction Tables
-- =====================================================

CREATE TABLE predictions (
    id            BIGSERIAL PRIMARY KEY,
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    predictor_id  VARCHAR(64) NOT NULL,
    predicted_bot VARCHAR(16) NOT NULL,
    confidence    SMALLINT CHECK (confidence >= 1 AND confidence <= 100),
    correct       BOOLEAN,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ,
    UNIQUE(match_id, predictor_id)
);
CREATE INDEX idx_predictions_match ON predictions(match_id);
CREATE INDEX idx_predictions_predictor ON predictions(predictor_id);

CREATE TABLE predictor_stats (
    predictor_id  VARCHAR(64) PRIMARY KEY,
    correct       INTEGER NOT NULL DEFAULT 0,
    incorrect     INTEGER NOT NULL DEFAULT 0,
    streak        INTEGER NOT NULL DEFAULT 0,
    best_streak   INTEGER NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =====================================================
-- Map Tables
-- =====================================================

CREATE TABLE maps (
    map_id          VARCHAR(32) PRIMARY KEY,
    player_count    INTEGER NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    engagement      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    wall_density    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    energy_count    INTEGER NOT NULL DEFAULT 0,
    grid_width      INTEGER NOT NULL,
    grid_height     INTEGER NOT NULL,
    map_json        JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retired_at      TIMESTAMPTZ
);
CREATE INDEX idx_maps_status ON maps(status, player_count);
CREATE INDEX idx_maps_engagement ON maps(player_count, engagement DESC);

CREATE TABLE map_scores (
    map_id          VARCHAR(32) PRIMARY KEY,
    engagement      DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    symmetry_score  DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    wall_density    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    last_used_at    TIMESTAMPTZ,
    match_count     INTEGER NOT NULL DEFAULT 0,
    avg_turns       DOUBLE PRECISION,
    scored_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE map_votes (
    id          BIGSERIAL PRIMARY KEY,
    map_id      VARCHAR(32) NOT NULL REFERENCES maps(map_id) ON DELETE CASCADE,
    voter_id    VARCHAR(64) NOT NULL,
    vote        SMALLINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(map_id, voter_id)
);
CREATE INDEX idx_map_votes_map ON map_votes(map_id);

CREATE TABLE map_fairness (
    map_id      VARCHAR(32) NOT NULL REFERENCES maps(map_id) ON DELETE CASCADE,
    player_slot INTEGER NOT NULL,
    games       INTEGER NOT NULL DEFAULT 0,
    wins        INTEGER NOT NULL DEFAULT 0,
    last_check  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (map_id, player_slot)
);

-- =====================================================
-- Program Tables (Evolution)
-- =====================================================

CREATE TABLE programs (
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
CREATE INDEX idx_programs_island ON programs(island);
CREATE INDEX idx_programs_island_fitness ON programs(island, fitness DESC);

-- =====================================================
-- Playlist Tables
-- =====================================================

CREATE TABLE playlists (
    slug        VARCHAR(64) PRIMARY KEY,
    title       VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    VARCHAR(32) NOT NULL DEFAULT 'featured',
    is_auto     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE playlist_matches (
    playlist_slug VARCHAR(64) NOT NULL REFERENCES playlists(slug) ON DELETE CASCADE,
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    sort_order    INTEGER NOT NULL DEFAULT 0,
    curation_tag  TEXT NOT NULL DEFAULT '',
    added_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (playlist_slug, match_id)
);
CREATE INDEX idx_playlist_matches_playlist ON playlist_matches(playlist_slug, sort_order);

-- =====================================================
-- Feedback Tables
-- =====================================================

CREATE TABLE replay_feedback (
    feedback_id   VARCHAR(32) PRIMARY KEY,
    match_id      VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    turn          INTEGER NOT NULL,
    type          VARCHAR(16) NOT NULL CHECK (type IN ('insight', 'mistake', 'idea', 'highlight')),
    body          TEXT NOT NULL,
    author        VARCHAR(128) NOT NULL,
    upvotes       INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_feedback_match ON replay_feedback(match_id, turn);

CREATE TABLE feedback_upvotes (
    feedback_id VARCHAR(32) NOT NULL REFERENCES replay_feedback(feedback_id) ON DELETE CASCADE,
    voter_id    VARCHAR(36) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (feedback_id, voter_id)
);

-- =====================================================
-- Enrichment Tables
-- =====================================================

CREATE TABLE enrichment_requests (
    request_id   VARCHAR(32) PRIMARY KEY,
    match_id     VARCHAR(32) NOT NULL REFERENCES matches(match_id),
    bot_id       VARCHAR(16) NOT NULL REFERENCES bots(bot_id),
    status       VARCHAR(16) NOT NULL DEFAULT 'pending',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    error_msg    TEXT,
    UNIQUE(match_id, bot_id)
);
CREATE INDEX idx_enrichment_requests_status ON enrichment_requests(status, requested_at);
CREATE INDEX idx_enrichment_requests_bot ON enrichment_requests(bot_id, requested_at);
