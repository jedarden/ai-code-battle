# Bead bf-67io: SQL Migration File Already Exists

## Status: COMPLETE (pre-existing)

## Finding
The SQL migration file `migrations/0001_initial.sql` already exists in the codebase, added in commit 31678be.

## Migration File Location
- **Path:** `migrations/0001_initial.sql`
- **Commit:** 31678be "Infra: Add initial SQL migration file (0001_initial.sql)"
- **Date:** 2026-05-21

## Schema Coverage
The migration file contains the complete initial schema including:
- Core tables: bots, matches, match_participants, jobs, rating_history
- Season & Series tables: seasons, season_snapshots, series, series_games
- Prediction tables: predictions, predictor_stats
- Map tables: maps, map_scores, map_votes, map_fairness
- Program tables (evolution): programs
- Playlist tables: playlists, playlist_matches
- Feedback tables: replay_feedback, feedback_upvotes
- Enrichment tables: enrichment_requests

## Current State
- The embedded schema in `cmd/acb-api/db.go` remains as a runtime convenience (uses `CREATE TABLE IF NOT EXISTS` for idempotency)
- The standalone migration file serves as the canonical schema definition and can be used with migration tools
- Both sources are equivalent in terms of schema definition

## No Action Required
This bead was filed based on a plan inventory that noted the migration file was missing, but it was created before this bead was picked up.
