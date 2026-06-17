# Bead bf-z9m Verification

## Date
2026-06-16

## Task
Award score for combat kills in engine (turn.go)

## Finding
**Already implemented** in commit c1acd83 (2026-06-16 23:40:23 -0400)

### Implementation Details

The feature was fully implemented with:

1. **Configurable kill score** (`engine/types.go:182`):
   ```go
   KillScore int `json:"kill_score"` // score awarded per combat kill
   ```

2. **Default value set to 1** (`engine/types.go:201`):
   ```go
   KillScore: 1,  // Default score per combat kill
   ```

3. **Score awarding in executeCombat()** (`engine/turn.go:272-275`):
   ```go
   // Award score for the kill
   if e.Owner < len(gs.Players) {
       gs.Players[e.Owner].Score += gs.Config.KillScore
   }
   ```

### Implementation Quality
✅ Matches requirements exactly:
- Score incremented immediately after CombatDeaths tracking
- Configurable via Config struct
- Default value of 1 point per kill
- Proper bounds checking on array access

The implementation ensures that killing enemy bots earns real score points, making combat a viable strategy compared to pure foraging.
