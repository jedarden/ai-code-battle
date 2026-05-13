# Bug Fix bf-3mx7: Match Index Winner Badge

## Problem
The match index winner badge was never showing because the `Won` field was incorrectly set using `p.BotID == m.WinnerID`. The issue: `m.WinnerID` is a player-slot integer as string (e.g. "2"), not a bot_id. The SQL query already computed the correct winner status in the `p.Won` field.

## Fix Applied
Fixed in commit `6fe778b` - Changed all occurrences from comparing `p.BotID == m.WinnerID` to using `p.Won` directly:

1. **matchToSummary** (line ~355): Changed `Won: p.BotID == m.WinnerID` to `Won: p.Won`
2. **buildPlaylistMatch** (line ~982): Changed `Won: p.BotID == m.WinnerID` to `Won: p.Won`
3. **ratingUpsetMagnitude**: Use `p.Won` to identify winner instead of comparing with `m.WinnerID`
4. **maxScoreDiff**: Use `p.Won` to identify winner instead of comparing with `m.WinnerID`
5. **isEvolutionBreakthrough**: Find winner using `p.Won` before checking if evolved

## Next Steps
To apply the fix to production data:
- Run the index builder to regenerate static JSON files with correct winner badges
- This requires database access and running: `go run cmd/acb-index-builder/main.go`

**Note**: The index builder cannot be run in the current environment (Go not installed).
This needs to be run in an environment with Go installed and database access.

## Impact
This fixes the issue where 984/1000 production matches had `winner_id` set but all participants showed `won: false`.
