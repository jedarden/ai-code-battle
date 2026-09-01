-- Add commentary_json column to matches table for storing AI-generated commentary
-- This allows the enrichment service to mark matches as enriched and store commentary metadata

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'matches' AND column_name = 'commentary_json'
    ) THEN
        ALTER TABLE matches ADD COLUMN commentary_json JSONB;
    END IF;
END $$;

-- Add index for quickly finding matches without commentary
CREATE INDEX IF NOT EXISTS idx_matches_commentary_null
    ON matches(match_id)
    WHERE commentary_json IS NULL;

-- Add index for recently enriched matches
CREATE INDEX IF NOT EXISTS idx_matches_commentary_completed
    ON matches(completed_at DESC)
    WHERE commentary_json IS NOT NULL;
