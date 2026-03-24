-- Migration: 0001_initial
-- Description: Initial database schema for AI Code Battle
-- Created: 2025-03-24

-- ============================================
-- Core Tables
-- ============================================

-- Bots table: stores registered bots
CREATE TABLE IF NOT EXISTS bots (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  owner_id TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  api_key_hash TEXT NOT NULL,
  rating REAL NOT NULL DEFAULT 1500.0,
  rating_deviation REAL NOT NULL DEFAULT 350.0,
  rating_volatility REAL NOT NULL DEFAULT 0.06,
  evolved INTEGER NOT NULL DEFAULT 0,
  island TEXT,
  generation INTEGER,
  parent_ids TEXT,
  description TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_health_check TEXT,
  health_status TEXT DEFAULT 'unknown',
  matches_played INTEGER NOT NULL DEFAULT 0,
  matches_won INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_bots_owner ON bots(owner_id);
CREATE INDEX IF NOT EXISTS idx_bots_rating ON bots(rating DESC);
CREATE INDEX IF NOT EXISTS idx_bots_evolved ON bots(evolved);

-- Matches table: stores match metadata
CREATE TABLE IF NOT EXISTS matches (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'pending',
  winner_id TEXT,
  turns INTEGER,
  end_reason TEXT,
  map_id TEXT NOT NULL,
  scores_json TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  started_at TEXT,
  completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_matches_status ON matches(status);
CREATE INDEX IF NOT EXISTS idx_matches_created ON matches(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_matches_map ON matches(map_id);

-- Match participants: links bots to matches
CREATE TABLE IF NOT EXISTS match_participants (
  id TEXT PRIMARY KEY,
  match_id TEXT NOT NULL,
  bot_id TEXT NOT NULL,
  player_index INTEGER NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  status TEXT,
  rating_before REAL NOT NULL,
  rating_after REAL,
  rating_deviation_before REAL NOT NULL,
  rating_deviation_after REAL,
  FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE,
  FOREIGN KEY (bot_id) REFERENCES bots(id) ON DELETE CASCADE,
  UNIQUE(match_id, bot_id),
  UNIQUE(match_id, player_index)
);

CREATE INDEX IF NOT EXISTS idx_match_participants_match ON match_participants(match_id);
CREATE INDEX IF NOT EXISTS idx_match_participants_bot ON match_participants(bot_id);

-- Jobs table: match execution jobs for workers
CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  match_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  worker_id TEXT,
  claimed_at TEXT,
  heartbeat_at TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  completed_at TEXT,
  error_message TEXT,
  FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_worker ON jobs(worker_id);
CREATE INDEX IF NOT EXISTS idx_jobs_heartbeat ON jobs(heartbeat_at);

-- Rating history: tracks rating changes over time
CREATE TABLE IF NOT EXISTS rating_history (
  id TEXT PRIMARY KEY,
  bot_id TEXT NOT NULL,
  match_id TEXT NOT NULL,
  rating_before REAL NOT NULL,
  rating_after REAL NOT NULL,
  rating_deviation REAL NOT NULL,
  recorded_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (bot_id) REFERENCES bots(id) ON DELETE CASCADE,
  FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_rating_history_bot ON rating_history(bot_id);
CREATE INDEX IF NOT EXISTS idx_rating_history_time ON rating_history(recorded_at DESC);

-- Maps table: stores generated maps
CREATE TABLE IF NOT EXISTS maps (
  id TEXT PRIMARY KEY,
  width INTEGER NOT NULL,
  height INTEGER NOT NULL,
  player_count INTEGER NOT NULL DEFAULT 2,
  walls TEXT NOT NULL,
  spawns TEXT NOT NULL,
  cores TEXT NOT NULL,
  energy_nodes TEXT NOT NULL,
  wall_density REAL NOT NULL DEFAULT 0.15,
  status TEXT NOT NULL DEFAULT 'active',
  engagement_score REAL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_maps_status ON maps(status);
CREATE INDEX IF NOT EXISTS idx_maps_player_count ON maps(player_count);
CREATE INDEX IF NOT EXISTS idx_maps_engagement ON maps(engagement_score DESC);

-- Bot secrets: stores API keys for bots (separate for security)
CREATE TABLE IF NOT EXISTS bot_secrets (
  bot_id TEXT PRIMARY KEY,
  api_key_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (bot_id) REFERENCES bots(id) ON DELETE CASCADE
);

-- ============================================
-- Prediction System
-- ============================================

-- Predictions: visitor predictions on match outcomes
CREATE TABLE IF NOT EXISTS predictions (
  id TEXT PRIMARY KEY,
  match_id TEXT NOT NULL,
  predictor_id TEXT NOT NULL,
  predictor_name TEXT,
  predicted_bot_id TEXT NOT NULL,
  correct INTEGER,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE,
  FOREIGN KEY (predicted_bot_id) REFERENCES bots(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_predictions_match ON predictions(match_id);
CREATE INDEX IF NOT EXISTS idx_predictions_predictor ON predictions(predictor_id);

-- Predictor stats: aggregate prediction accuracy
CREATE TABLE IF NOT EXISTS predictor_stats (
  predictor_id TEXT PRIMARY KEY,
  predictor_name TEXT,
  correct INTEGER NOT NULL DEFAULT 0,
  incorrect INTEGER NOT NULL DEFAULT 0,
  streak INTEGER NOT NULL DEFAULT 0,
  best_streak INTEGER NOT NULL DEFAULT 0,
  rating REAL NOT NULL DEFAULT 1000.0
);

CREATE INDEX IF NOT EXISTS idx_predictor_stats_rating ON predictor_stats(rating DESC);

-- ============================================
-- Map Voting
-- ============================================

-- Map votes: community voting on map quality
CREATE TABLE IF NOT EXISTS map_votes (
  id TEXT PRIMARY KEY,
  map_id TEXT NOT NULL,
  voter_id TEXT NOT NULL,
  vote INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (map_id) REFERENCES maps(id) ON DELETE CASCADE,
  UNIQUE(map_id, voter_id)
);

CREATE INDEX IF NOT EXISTS idx_map_votes_map ON map_votes(map_id);

-- ============================================
-- Replay Feedback
-- ============================================

-- Replay feedback: community annotations on replays
CREATE TABLE IF NOT EXISTS replay_feedback (
  id TEXT PRIMARY KEY,
  match_id TEXT NOT NULL,
  turn INTEGER NOT NULL,
  type TEXT NOT NULL,
  body TEXT NOT NULL,
  author TEXT NOT NULL,
  upvotes INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_feedback_match ON replay_feedback(match_id, turn);
CREATE INDEX IF NOT EXISTS idx_feedback_type ON replay_feedback(type);
CREATE INDEX IF NOT EXISTS idx_feedback_upvotes ON replay_feedback(upvotes DESC);

-- ============================================
-- Multi-Game Series
-- ============================================

-- Series: best-of-N match series between two bots
CREATE TABLE IF NOT EXISTS series (
  id TEXT PRIMARY KEY,
  bot_a_id TEXT NOT NULL,
  bot_b_id TEXT NOT NULL,
  format INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  a_wins INTEGER NOT NULL DEFAULT 0,
  b_wins INTEGER NOT NULL DEFAULT 0,
  season_id TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  completed_at TEXT,
  FOREIGN KEY (bot_a_id) REFERENCES bots(id) ON DELETE CASCADE,
  FOREIGN KEY (bot_b_id) REFERENCES bots(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_series_status ON series(status);
CREATE INDEX IF NOT EXISTS idx_series_bots ON series(bot_a_id, bot_b_id);
CREATE INDEX IF NOT EXISTS idx_series_season ON series(season_id);

-- Series games: individual games within a series
CREATE TABLE IF NOT EXISTS series_games (
  series_id TEXT NOT NULL,
  game_number INTEGER NOT NULL,
  match_id TEXT,
  map_id TEXT NOT NULL,
  winner INTEGER,
  PRIMARY KEY (series_id, game_number),
  FOREIGN KEY (series_id) REFERENCES series(id) ON DELETE CASCADE,
  FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_series_games_match ON series_games(match_id);

-- ============================================
-- Seasonal Rotations
-- ============================================

-- Seasons: seasonal leaderboards with rule variations
CREATE TABLE IF NOT EXISTS seasons (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  theme TEXT NOT NULL,
  rules_version INTEGER NOT NULL DEFAULT 1,
  started_at TEXT NOT NULL,
  ended_at TEXT,
  champion_id TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  FOREIGN KEY (champion_id) REFERENCES bots(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_seasons_status ON seasons(status);
CREATE INDEX IF NOT EXISTS idx_seasons_dates ON seasons(started_at, ended_at);
