# Bead bf-59h: Evolver fitness weight combat kill rate alongside win rate

## Status: Already Completed

This bead's work was already completed in commit `d42d1a533664bb4933845f2a45b032288aa08be8` on 2026-06-17.

## What Was Done

The commit already implements all requirements from the bead description:

1. ✅ **Fitness Formula Updated** (`cmd/acb-evolver/run.go` lines 601-608)
   - Changed from: `fitness = winRate` (win rate only)
   - Changed to: `fitness = 0.7*winRate + 0.3*killRate`
   - This weights combat aggression at 30% alongside 70% win rate

2. ✅ **Kill Tracking Added** (`cmd/acb-evolver/internal/arena/arena.go`)
   - `ArenaResult` now includes: `TotalKills`, `TotalMatches`, `KillRate`
   - Kill rate is computed as: `TotalKills / TotalMatches` (excluding error matches)
   - Combat deaths are tracked per match via `CombatDeaths []int` field

3. ✅ **System Prompt Updated** (`cmd/acb-evolver/internal/prompt/builder.go` lines 154-159)
   - Added explicit "Fitness Function" section explaining the 70/30 formula
   - Includes language: "combat kills and eliminations are valuable, not just resource foraging"
   - Emphasizes: "Aggressive combat strategies that secure kills while winning are strongly favored"

4. ✅ **Enhanced Logging** (`cmd/acb-evolver/run.go` line 622-625)
   - Arena logging now shows: `win_rate`, `kill_rate`, `total kills`, `total matches`, `fitness`

## Verification

The implementation correctly addresses the bead's requirements:
- Fitness is now a weighted combination (70% win rate, 30% kill rate)
- Kill data is available from match records (via `CombatDeaths` field in game engine results)
- System prompt explicitly informs the LLM that kill score has value
- The evolver will now select for combat aggression, not just win optimization

## Dependencies Met

The bead noted that it required `bf-z9m` (kill scoring) to be merged first. The current implementation shows that kill scoring data is already available in the match records, indicating that dependency has been met.
