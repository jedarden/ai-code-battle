# Verification: Directed Attack Arrows from killers[] in combat_death events

## Task
Viewer: directed attack arrows from killers[] in combat_death events (§16.9)

## Status: ALREADY IMPLEMENTED

The feature was fully implemented in commits 8e0aa5e and 323c1e8.

## Implementation Details

### 1. Engine (engine/turn.go, commit 8e0aa5e)
The executeCombat function emits `combat_death` events with a `killers` array containing:
- bot_id: ID of the killer bot
- owner: Player who owns the killer bot
- position: Grid position of the killer bot

### 2. Types (web/src/types.ts)
The CombatDeathDetails interface includes:
```typescript
export interface CombatDeathKiller {
  bot_id: number;
  owner: number;
  position: Position;
}

export interface CombatDeathDetails {
  bot_id: number;
  owner: number;
  position: Position;
  killers: CombatDeathKiller[];
}
```

### 3. Viewer (web/src/replay-viewer.ts, commit 323c1e8)
The drawCombatEffects() method:
- Checks for `combat_death` events with the `killers` array
- For each killer, draws a solid arrow from the killer's position to the victim's position
- Arrows are colored with the killer's player color
- Includes a red explosion flash and X marker for the death
- Maintains backward compatibility with old replays (uses proximity-inference lines for events without killers array)

## Verification
The implementation is complete and working as specified. Directed arrows are now shown from each killer to the victim in combat_death events.
