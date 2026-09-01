-- Add predictable flag to matches table for prediction system (§14.5)
-- Matches flagged as predictable when:
-- - Both bots are in the top 20
-- - It's a rivalry match
-- - It's a series match
-- - An evolved bot faces a top-10 human-written bot

ALTER TABLE matches ADD COLUMN predictable BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_matches_predictable ON matches(predictable, created_at) WHERE predictable = TRUE;
