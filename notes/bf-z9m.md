# Task bf-z9m: Award Score for Combat Kills

## Status: ALREADY COMPLETED

This task was already completed in commit `c1acd83` (2026-06-16 23:40:23).

## Implementation Details

The commit `c1acd83 feat(combat): award score for combat kills` added:

1. **Config Field**: `KillScore int` in `engine/types.go:182` with default value of 1 point per kill
2. **Score Award**: In `engine/turn.go:272-275`, score is incremented immediately after tracking `CombatDeaths`

```go
// Track combat deaths for the killer's player
if e.Owner < len(gs.CombatDeaths) {
    gs.CombatDeaths[e.Owner]++
}
// Award score for the kill
if e.Owner < len(gs.Players) {
    gs.Players[e.Owner].Score += gs.Config.KillScore
}
```

## Verification

- Kill score is configurable via `Config.KillScore`
- Default value is 1 point per kill (matches request)
- Score is awarded to the killer's player in the combat loop
- Implementation matches the bead requirements exactly

## Notes

No code changes were required for this bead - the feature was already implemented.
