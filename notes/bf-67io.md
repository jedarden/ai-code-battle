# Bead bf-67io: SQL Migration File Verification

## Task
Infra: SQL migration file (migrations/0001_initial.sql) — schema is embedded in db.go but no standalone migration file exists per §11.1 plan inventory

## Status
**ALREADY COMPLETE** - Work was done in commit 31678be prior to bead assignment

## Verification
- Migration file exists at: `migrations/0001_initial.sql`
- Created in commit: `31678be` (2026-05-22 15:52:54)
- Contains all 20 tables from `cmd/acb-api/db.go`:
  - bots, matches, match_participants, jobs, rating_history
  - seasons, season_snapshots, series, series_games
  - predictions, predictor_stats
  - maps, map_scores, map_votes, map_fairness
  - programs, playlists, playlist_matches
  - replay_feedback, feedback_upvotes
  - enrichment_requests

## Schema Completeness
All columns from `db.go` are present in the migration file, including:
- `matches.combat_turns` (line 41)
- `matches.enrichment_requested_at` (line 43)
- `bots.debug_public` (line 30)
- `series.bracket_round`, `bracket_position`, `featured` (lines 116-118)

## Notes
The migration file uses clean `CREATE TABLE` statements (not `CREATE TABLE IF NOT EXISTS`), which is the correct approach for an initial migration. The idempotent migrations in `db.go` are appropriate for runtime schema enforcement.
