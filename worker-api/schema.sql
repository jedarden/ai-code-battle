-- AI Code Battle D1 Schema
-- Phase 4: Match Orchestration

-- Bots table: stores registered bots
CREATE TABLE IF NOT EXISTS bots (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  owner_id TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  api_key_hash TEXT NOT NULL,
  rating REAL NOT NULL DEFAULT 1500.0,
  rating_deviation REAL NOT NULL DEFAULT 350.0,
  rating_volatility REAL NOT NULL DEFAULT 0.06,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_health_check TEXT,
  health_status TEXT DEFAULT 'unknown',
  matches_played INTEGER NOT NULL DEFAULT 0,
  matches_won INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_bots_owner ON bots(owner_id);
CREATE INDEX IF NOT EXISTS idx_bots_rating ON bots(rating DESC);

-- Matches table: stores match metadata
CREATE TABLE IF NOT EXISTS matches (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'pending',
  winner_id TEXT,
  turns INTEGER,
  end_reason TEXT,
  map_id TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  started_at TEXT,
  completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_matches_status ON matches(status);
CREATE INDEX IF NOT EXISTS idx_matches_created ON matches(created_at DESC);

-- Match participants: links bots to matches
CREATE TABLE IF NOT EXISTS match_participants (
  id TEXT PRIMARY KEY,
  match_id TEXT NOT NULL,
  bot_id TEXT NOT NULL,
  player_index INTEGER NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
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
  walls TEXT NOT NULL,
  spawns TEXT NOT NULL,
  cores TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Bot secrets: stores API keys for bots (separate for security)
CREATE TABLE IF NOT EXISTS bot_secrets (
  bot_id TEXT PRIMARY KEY,
  api_key_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (bot_id) REFERENCES bots(id) ON DELETE CASCADE
);
