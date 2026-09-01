# Seasonal System - Backward Compatibility (§14.9)

## Overview
The seasonal system implements competitive seasons with full backward compatibility. All changes are additive, ensuring old bots remain competitive while new features enhance the ecosystem.

## What Persists Across Seasons

### Bot Registrations
Bot registrations **never expire**. Once registered, a bot persists forever unless explicitly retired by its owner.

### Match History
All match results are **permanently preserved**. The `matches` and `match_participants` tables grow indefinitely. Historical matches can be replayed, analyzed, and referenced across all seasons.

### Evolution Population
The `programs` table persists across seasons. Evolution islands, generations, and parentage chains are never reset. This allows long-term studies of evolutionary dynamics.

## What Resets Each Season

### Glicko-2 Ratings
Ratings are **decayed** but **not reset**. The formula preserves relative ordering:
```
new_mu = 1500 + (current_mu - 1500) * decay_factor
```
- Default decay factor: 70%
- Top bots retain ~85% of their rating advantage
- Relative skill ordering is preserved
- Default phi/sigma restored for fresh volatility estimates

**Backward compatibility:** Old bots start the new season with competitive ratings, not from 1500. This respects demonstrated skill while allowing new competitors to climb faster.

### Map Pool
Maps are **rotated** seasonally, not fully replaced:
- Maps with low engagement (<30%) and age >90 days move to `status='classic'`
- Classic maps remain playable but weighted lower in selection
- New maps are continuously added via external processes
- Active map pool refreshes naturally over time

**Backward compatibility:** Old bots remain competitive because:
- Grid dimensions (60x60) never change
- Tile types are additive only (new types treated as open by old bots)
- Core mechanics (energy, walls, combat) are stable
- Map changes are gradual, not disruptive

### Prediction Standings
Predictor stats are **fully reset** each season:
- `correct`, `incorrect`, `streak`, `best_streak` → 0
- Predictor identities persist (can track across seasons)
- Season-long accuracy requires fresh predictions

**Backward compatibility:** Predictions are optional meta-gaming. Core bot functionality is unaffected.

### Playlist Contents
Auto-generated playlists are **cleared** each season:
- `is_auto=TRUE` playlists → emptied
- Curated playlists (`is_auto=FALSE`) → preserved
- Featured matches rebuild from the new season's activity

**Backward compatibility:** Playlists are curation tools, not core gameplay.

## Rule Versioning System

Each season carries a `rules_version` string (e.g., "1.0", "2.0"). This documents the competitive environment but **does not gate functionality**.

### Additive Changes Only

**Tile Types (New)**
- New tile types are added as optional fields
- Old bots that don't recognize them treat them as open/walls
- Example: A "teleporter" tile is just an open tile to a bot from v1.0

**Scoring Bonuses (New)**
- New scoring opportunities add to the existing framework
- Old bots can still win via traditional methods
- Example: Season 3 adds "energy efficiency" bonus, but elimination still wins matches

**Numeric Parameters (Adjusted)**
- Parameter changes are within acceptable ranges
- Turn limits may increase (500→600) but core gameplay unchanged
- Energy costs may shift but strategies remain valid

**Map Pool Changes (Rotational)**
- Maps come and go, but the grid and tile set are stable
- New maps respect existing tile semantics
- Classic maps remain playable

### Breaking Changes Are Prohibited

The following **never** change mid-season or without major version increments:
- Grid dimensions (60x60 toroidal is canonical)
- Turn timeout (5s per decision)
- Basic tile semantics (energy, walls, empty)
- Win conditions (elimination, turn limit)
- Network protocol (Bot API)

## Season Archive

Each completed season is fully archived in the `season_snapshots` table:
- Final leaderboard (rank, rating, wins, losses)
- Champion identification
- Championship bracket results
- Match history preserved indefinitely

The `/seasons` web page provides:
- Active season overview with live standings
- Historical season browser
- Championship bracket viewer
- Per-season leaderboards

## Migration Path

New bot authors can:
1. Read the current `rules_version` from the active season
2. Implement basic gameplay (movement, energy, combat)
3. Gradually adopt advanced features (new tile types, scoring bonuses)
4. Remain competitive in both old and new seasons

The system guarantees that a bot written for Season 1 will continue to function in Season 10, even if Season 10 has added 5 new tile types and 3 new scoring mechanisms.

## Implementation Notes

See `cmd/acb-matchmaker/series_season.go:processSeasonEnd()` for the complete reset workflow:
1. Snapshot ratings to `season_snapshots`
2. Determine champion
3. Mark season as completed
4. Reset predictor stats
5. Rotate map pool (active→classic for underperforming maps)
6. Clear auto-playlists
7. Apply rating decay (preserve ordering, pull toward 1500)
8. Create championship bracket (top 8, best-of-7)

The ticker runs every 5 minutes (`ACB_SEASON_RESET_INTERVAL`, default 300s) and automatically transitions seasons when `ends_at <= NOW()`.
