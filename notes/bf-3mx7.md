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

## Status
✅ **FIXED** - The bug has been fixed in commit 9bcbd56 (2026-05-13)

## Production Deployment
The index builder runs automatically every 15 minutes in the Kubernetes production environment:
- Deployment: `acb-index-builder` in namespace `ai-code-battle`
- Build interval: 15 minutes (ACB_BUILD_INTERVAL)
- The fixed code will automatically regenerate static JSON with correct winner badges on the next cycle
- Manual intervention not required - the index builder will pick up the code change automatically

## Impact
This fixes the issue where 984/1000 production matches had `winner_id` set but all participants showed `won: false`.
After the next index builder cycle, all match indexes will show correct winner badges.
