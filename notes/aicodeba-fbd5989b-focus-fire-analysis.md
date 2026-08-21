# Focus-Fire Phase Implementation Analysis

**Date:** 2025-01-21  
**File:** `engine/turn.go` (lines 201-290)  
**Function:** `executeCombat()`

---

## Executive Summary

The focus-fire combat algorithm implements a **simultaneous resolution** system where combat outcomes are determined by local enemy density. A bot dies when it has equal or more nearby enemies compared to those enemies' own threat levels. This creates emergent tactical properties: superior numbers win cleanly, tight formations are defensive, and multi-player battles produce complex alliances.

---

## Algorithm Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    PHASE: COMBAT                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Step 1: Count Enemies (Lines 207-223)                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ FOR each living bot B:                               │  │
│  │   FOR each enemy bot E:                             │  │
│  │     IF InRadius(B.position, E.position, attackRadius2)│
│  │        enemies[B.ID]++                               │  │
│  │        botsInRadius[B.ID].append(E)                  │  │
│  └──────────────────────────────────────────────────────┘  │
│                    ↓                                        │
│  Step 2: Determine Deaths (Lines 225-248)                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ FOR each living bot B:                               │  │
│  │   myEnemyCount = enemies[B.ID]                       │  │
│  │   IF myEnemyCount == 0: CONTINUE (safe)             │  │
│  │   FOR each enemy E in botsInRadius[B.ID]:           │  │
│  │     theirEnemyCount = enemies[E.ID]                  │  │
│  │     IF myEnemyCount >= theirEnemyCount:             │  │
│  │       dead[B.ID] = true                              │  │
│  │       BREAK (already dead, no need to check more)   │  │
│  └──────────────────────────────────────────────────────┘  │
│                    ↓                                        │
│  Step 3: Apply Deaths & Emit Events (Lines 250-289)      │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ FOR each bot B where dead[B.ID] == true:            │  │
│  │   B.Alive = false                                    │  │
│  │   Players[B.Owner].BotCount--                        │  │
│  │   Build killers array from botsInRadius[B.ID]        │  │
│  │   FOR each killer K in botsInRadius[B.ID]:          │  │
│  │     Players[K.Owner].Score += killScore              │  │
│  │     CombatDeaths[K.Owner]++                          │  │
│  │   EMIT EventCombatDeath                             │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Code-Level Analysis

### Phase 1: Enemy Counting (Lines 207-223)

```go
// For each bot, count enemies within attack radius
enemyCounts := make(map[int]int)     // bot ID -> enemy count
botsInRadius := make(map[int][]*Bot) // bot ID -> enemies within radius

for _, b := range gs.Bots {
    if !b.Alive { continue }
    
    var enemies []*Bot
    for _, e := range gs.Bots {
        if !e.Alive || e.ID == b.ID || e.Owner == b.Owner { continue }
        if gs.Grid.InRadius(b.Position, e.Position, gs.Config.AttackRadius2) {
            enemies = append(enemies, e)
        }
    }
    enemyCounts[b.ID] = len(enemies)
    botsInRadius[b.ID] = enemies
}
```

**Key Points:**
- **Toroidal distance:** Uses `Grid.InRadius()` which accounts for wrap-around on all map edges
- **Self-filtering:** Excludes self (`e.ID == b.ID`) and friendly bots (`e.Owner == b.Owner`)
- **Dead bots excluded:** Only living bots participate (`!e.Alive` check)
- **Dual data structure:** Stores both count (`enemyCounts`) and actual enemy list (`botsInRadius`)

### Phase 2: Death Determination (Lines 225-248)

```go
// Determine which bots die (simultaneous - use pre-computed counts)
dead := make(map[int]bool)

for _, b := range gs.Bots {
    if !b.Alive { continue }
    
    myEnemyCount := enemyCounts[b.ID]
    if myEnemyCount == 0 { continue } // No enemies nearby, safe
    
    // Check if any enemy has <= myEnemyCount enemies
    for _, e := range botsInRadius[b.ID] {
        theirEnemyCount := enemyCounts[e.ID]
        if myEnemyCount >= theirEnemyCount {
            dead[b.ID] = true
            break // I die - no need to check further
        }
    }
}
```

**The Focus-Fire Rule:**
A bot B dies if there exists an enemy E within attack radius such that:
```
enemies_near(B) >= enemies_near(E)
```

**Examples:**
- **2v1:** Bot A has 1 enemy, Bot B has 1 enemy, Bot C has 2 enemies
  - Bot C checks: `2 >= 1` → C dies
  - Bot A checks: `1 >= 2` → FALSE, `1 >= 1` → A dies (mutual destruction with B)
  - Bot B checks: `1 >= 2` → FALSE, `1 >= 1` → B dies
  - Result: The pair survives, the lone bot dies
  
- **1v1:** Both bots have 1 enemy each
  - Bot A checks: `1 >= 1` → A dies
  - Bot B checks: `1 >= 1` → B dies
  - Result: Both die (mutual destruction)
  
- **3v2:** Three bots in a cluster face two scattered enemies
  - Each cluster bot has 2 enemies nearby
  - Each scattered bot has 3 enemies nearby
  - Scattered bots check: `3 >= 2` → all scattered bots die
  - Cluster bots check: `2 >= 3` → FALSE (nobody in the cluster dies)
  - Result: Formation wins decisively

### Phase 3: Death Application (Lines 250-289)

```go
for _, b := range gs.Bots {
    if dead[b.ID] {
        b.Alive = false
        gs.DeadBots = append(gs.DeadBots, b)
        
        if b.Owner < len(gs.Players) {
            gs.Players[b.Owner].BotCount--
        }
        
        // Build killers array (enemies within attack radius)
        var killers []map[string]interface{}
        for _, e := range botsInRadius[b.ID] {
            killers = append(killers, map[string]interface{}{
                "bot_id":   e.ID,
                "owner":    e.Owner,
                "position": e.Position,
            })
            if e.Owner < len(gs.CombatDeaths) {
                gs.CombatDeaths[e.Owner]++
            }
            if e.Owner < len(gs.Players) {
                gs.Players[e.Owner].Score += gs.Config.KillScore
            }
        }
        
        gs.Events = append(gs.Events, Event{
            Type: EventCombatDeath,
            Turn: gs.Turn,
            Details: map[string]interface{}{
                "bot_id":   b.ID,
                "owner":    b.Owner,
                "position": b.Position,
                "killers":  killers,
            },
        })
    }
}
```

**Side Effects:**
1. Bot state changes (`b.Alive = false`)
2. Player bot count decrements
3. Kill score awarded to each killer's player
4. Combat death statistics tracked per player
5. Event emitted for replay (includes all killers for visualization)

---

## Relationship with Zone Shrink

### Turn Execution Order

```go
func (gs *GameState) ExecuteTurn() *MatchResult {
    gs.Turn++
    
    gs.executeMoves()    // Phase 1: Move
    gs.executeCombat()   // Phase 2: Combat ← FOCUS-FIRE HERE
    gs.executeZone()     // Phase 3: Zone (kills AFTER combat)
    gs.executeCaptures() // Phase 4: Capture
    // ... more phases
}
```

**Critical Insight:** Combat resolves **before** the zone kills. This means:
1. Bots can fight in the zone boundary region
2. Zone kills after combat removes stragglers who didn't engage
3. The zone is a "forcing function" - it pushes bots together, combat resolves the engagement

### Zone Parameters (from plan.md §3.7.1)

| Parameter | 2-Player | 3+ Player | Purpose |
|-----------|----------|-----------|---------|
| ZoneStartTurn | 10 | 10 | Start shrinking early |
| ZoneShrinkInterval | 1 | 1 | Shrink every turn |
| ZoneShrinkStep | 1 | 1 | Match bot movement speed |
| ZoneMinRadius | 2 | 1 | Final combat arena size |

### Combat Density Convergence

The attack radius is tuned per player count to ensure the zone forces combat:

**2-Player:**
- Attack radius² = 25 (distance ≈ 5 tiles)
- Zone min radius = 2 (diameter = 4 tiles)
- Final zone diameter (4) ≤ 2 × attack radius (10)
- Result: Bots at opposite zone edges are within attack range

**3+ Player:**
- Attack radius² = 12 (distance ≈ 3.46 tiles)
- Zone min radius = 1 (diameter = 2 tiles)
- Final zone diameter (2) < attack radius (3.46)
- Result: ANY two bots in final zone are within attack range

**Verified Metrics (from plan.md):**
- 2-player: 65-80% of matches have combat deaths; ~1 death per 20 turns
- 6-player: 100% of matches have combat deaths; ~1 death per 5-6 turns

---

## Algorithm Properties

### Emergent Tactical Behaviors

1. **Superior Numbers Win Clean:** 2v1 → lone bot dies, pair survives
2. **Mutual Destruction:** 1v1 → both die
3. **Formation Defense:** Tight clusters reduce individual enemy count
4. **Multi-Player Alliances:** In 3+ player battles, third parties exploit 1v1 engagements

### Why Formations Are Defensive

Consider two formations of 3 bots each (Red vs Blue):

**Tight Formation (bots within 1 tile of each other):**
- Each Red bot sees 3 Blue enemies (entire formation)
- Each Blue bot sees 3 Red enemies (entire formation)
- Check: `3 >= 3` → All bots die (mutual annihilation)

**Loose Formation (bots spread 5+ tiles apart):**
- Each Red bot sees 1 Blue enemy (nearest)
- Each Blue bot sees 1 Red enemy (nearest)
- Check: `1 >= 1` → All bots die (mutual annihilation)

**Mixed: Tight vs Loose:**
- Each Tight bot sees 3 Loose enemies
- Each Loose bot sees 1 Tight enemy (nearest)
- Loose bots check: `1 >= 3` → FALSE (loose bots survive!)
- Tight bots check: `3 >= 1` → All tight bots die

**Conclusion:** Loose formations can defeat tight formations if the loose formation can engage piecemeal. However, loose formations are vulnerable to being surrounded individually.

### Simultaneous Resolution Guarantees

The algorithm guarantees **no cascading deaths** within a single turn:

1. Enemy counts are pre-computed and stored before any deaths
2. Death determination uses only pre-computed counts
3. A bot's death in step 2 doesn't affect another bot's check in the same turn

This prevents the scenario where "Bot A kills Bot B, then Bot C dies because B was its only nearby enemy."

---

## Edge Cases and Limitations

### Edge Case 1: Zero Enemy Count
```go
if myEnemyCount == 0 { continue } // No enemies nearby, safe
```
Bots with no enemies within attack radius automatically survive, regardless of global situation.

### Edge Case 2: Isolated Bots
A bot surrounded by enemies but with no allies:
- Bot has 5 enemies
- Each enemy has 1 enemy (the isolated bot)
- Bot checks: `5 >= 1` → Dies immediately
- Enemies check: `1 >= 5` → FALSE (enemies survive)
Result: Isolated bot dies quickly, as expected

### Edge Case 3: Circular Threats
Three bots in a triangle, each seeing the other two:
- Bot A: 2 enemies (B, C)
- Bot B: 2 enemies (A, C)
- Bot C: 2 enemies (A, B)
- Each checks: `2 >= 2` → All die
Result: Mutual annihilation (fair outcome)

### Edge Case 4: Asymmetric Threat
Bot A threatens 5 enemies, but each of those 5 enemies only sees A:
- Bot A: 5 enemies
- Each enemy: 1 enemy (A)
- Bot A checks: `5 >= 1` → A dies (outnumbered)
- Enemies check: `1 >= 5` → FALSE (all enemies survive)
Result: The many win over the one, even though the one could theoretically kill all of them individually

### Limitation: No Damage Accumulation
The algorithm is binary: alive or dead. There's no:
- Partial damage (wounding)
- Damage accumulation over turns
- Variable damage based on number of attackers
This simplifies the game but removes some strategic depth (e.g., "weaken a bot for next turn")

### Limitation: No Range Advantage
All bots within `AttackRadius2` are treated equally. There's no:
- First-mover advantage
- Range-based damage falloff
- Initiative system
This is intentional - simultaneous resolution creates tactical positioning rather than reactive play.

---

## Data Structures

### Input State (from GameState)

```go
type GameState struct {
    Bots      []*Bot           // All bots on the grid
    Players   []Player         // Player state (scores, energy, bot counts)
    Grid      *Grid            // Toroidal grid with distance functions
    Config    MatchConfig      // AttackRadius2, kill scores, etc.
    Turn      int              // Current turn number
    Events    []Event          // Event log for replay
    CombatDeaths []int         // Per-player combat death counts
    // ... other fields
}
```

### Key Config Values

```go
type MatchConfig struct {
    AttackRadius2 int // Squared attack radius (25 for 2p, 12 for 3p+)
    KillScore      int // Score awarded per kill (typically 1)
    // ... other config
}
```

### Output Events

```go
type Event struct {
    Type    string                 // "combat_death"
    Turn    int                    // Turn number
    Details map[string]interface{} // Event-specific data
}
```

For `EventCombatDeath`, `Details` contains:
```json
{
    "bot_id": 42,
    "owner": 0,
    "position": {"row": 10, "col": 15},
    "killers": [
        {"bot_id": 45, "owner": 1, "position": {"row": 10, "col": 16}},
        {"bot_id": 47, "owner": 1, "position": {"row": 11, "col": 15}}
    ]
}
```

---

## Complexity Analysis

### Time Complexity

Let N = number of living bots

**Phase 1 (Enemy Counting):**
- Outer loop: N bots
- Inner loop: N bots
- Distance check: O(1) with toroidal math
- Total: O(N²)

**Phase 2 (Death Determination):**
- Outer loop: N bots
- Inner loop: up to N enemies (worst case: all bots in range)
- Total: O(N²)

**Phase 3 (Death Application):**
- Single loop over N bots
- Inner loop over killers (average small, < N)
- Total: O(N × K) where K = average attackers per victim

**Overall:** O(N²) - quadratic in bot count

For typical matches (N ≈ 20-50 bots), this is negligible (< 2500 distance checks).

### Space Complexity

**Per-Turn Allocation:**
```go
enemyCounts  := make(map[int]int)      // O(N)
botsInRadius := make(map[int][]*Bot)   // O(N²) worst case (everyone sees everyone)
dead         := make(map[int]bool)     // O(N)
```

Total: O(N²) worst case, but typically much less in practice (bots only see nearby enemies).

---

## Testing Recommendations

### Unit Tests

1. **2v1 Scenario:** Verify lone bot dies, pair survives
2. **1v1 Mutual Destruction:** Verify both die
3. **Formation Defense:** Verify tight clusters beat scattered enemies
4. **Zero Enemies:** Verify isolated bots survive
5. **Simultaneity:** Verify deaths don't cascade (use pre-computed counts)
6. **Toroidal Wrap:** Verify combat works across map edges

### Integration Tests

1. **Zone + Combat:** Verify zone kills AFTER combat, not before
2. **Multi-Player:** Verify 3+ player battles produce expected alliances
3. **Replay Events:** Verify `EventCombatDeath` contains correct killers array

### Property-Based Tests

1. **Total Bot Count:** Total living bots never increase during combat
2. **Death Symmetry:** If A dies and B is within A's radius, B should be in A's killers
3. **Determinism:** Same input state → same output deaths (no randomness)

---

## References

- **Plan Document:** `docs/plan/plan.md` §3.4 (Combat), §3.7 (Turn Structure), §3.7.1 (Zone Parameters)
- **Grid Implementation:** `engine/grid.go` (toroidal distance, InRadius)
- **Related Functions:** `executeMoves()` (lines 52-113), `executeZone()` (lines 115-175)

---

## Conclusion

The focus-fire algorithm achieves its design goals:

✅ **Tactical Depth:** Formations matter, positioning matters, multi-player dynamics emerge  
✅ **Simplicity:** Binary alive/dead state, no damage accumulation  
✅ **Determinism:** No randomness, same inputs → same outputs  
✅ **Performance:** O(N²) but fast in practice (N ≈ 20-50)  
✅ **Zone Integration:** Zone forces contact, combat resolves engagements cleanly  

The algorithm is a refined implementation of the aichallenge ants system, adapted for a grid-based game with toroidal topology and multi-player support.
