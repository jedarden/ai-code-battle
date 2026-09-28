# Zone Semantics — Activation, Shrink, Boundary, and Damage Contract

**Date:** 2026-09-28
**Bead:** aicodeba-8db582c2
**Status:** CONSOLIDATED — every rule below is live in the engine and pinned by `engine/zone_test.go`

The public rules (README "Game Rules") summarize the zone as "a shrinking storm
that deals damage to units caught in it." This note consolidates the full
documented contract — activation timing, shrink schedule, geometry, boundary
behavior, and damage — from `engine/turn.go` (`executeZone`, which states the
contract at the source), `engine/match.go` (activation), `engine/game.go`
(state initialization), `engine/types.go` (config defaults), and
`docs/plan/plan.md` §3.7.1 (design rationale). Nothing here is new behavior;
each rule cites the test that pins it.

## 1. Activation

- Config field: `zone_start_turn` (default **10**, identical for every player
  count — chosen to force combat before energy farming dominates).
- The `MatchRunner` activates the zone **before collecting moves** on the first
  turn whose number is ≥ `zone_start_turn` (`engine/match.go:167`): it sets
  `ZoneActive = true` and snaps the radius to the initial zone radius (§4).
- Consequence: bots see the active zone in the same turn's `/turn` request and
  can react to it before their first shrink-delayed death is possible. The
  activation turn itself **never shrinks** (§3) but **does kill** bots already
  outside the snapped initial radius.
- While active, the zone is reported to bots in every `/turn` request as
  `zone: {center, radius, active}` (see `docs/bot-protocol.md`).

Pinned by: `TestZoneActivationTurnKillsWithoutShrinking` (kills without
shrinking on the activation turn), `TestZoneActivationAndShrinkTimingInMatch`
(per-turn replay bounds across a full match).

## 2. Geometry

- **Center:** fixed at the map center `(Rows/2, Cols/2)` for the whole match
  (`engine/game.go:62`). It never moves and the check never wraps the center
  itself — distance is measured from the bot's position to the center.
- **Pre-activation radius:** `min(Rows, Cols) / 2` — the whole map
  (`engine/game.go:63`), so nothing is outside before activation.
- **Activation snap:** `90%` of the distance from the center to the nearest
  edge, i.e. `(min(Rows/2, Cols/2) * 90) / 100`, with a floor of **7** on very
  small maps (`setInitialZoneRadius`, `engine/turn.go`). On the standard 40×40
  2-player map: 20 → **18**. The 90% snap keeps every spawn position (which
  cluster within 30% of center) inside the initial zone.

Pinned by: `TestZoneShrinkPathForRealConfigs` (pre-activation radius, snap
value, and anchor for every player count),
`TestZoneDocumentedContactGuaranteeAtMinimumRadius` (center anchoring).

## 3. Shrink schedule — first shrink and interval

- A shrink step lands on every executed turn where **both** hold
  (`engine/turn.go:141`):
  - `Turn > zone_start_turn` (the activation turn never shrinks), and
  - `(Turn − zone_start_turn) mod zone_shrink_interval == 0`.
- Therefore the **first shrink is on turn `zone_start_turn +
  zone_shrink_interval`**, then one step every `zone_shrink_interval` turns.
- Config field: `zone_shrink_interval` (default **1**, every player count —
  steady per-turn pressure).

Pinned by: `TestZoneShrinkCadence` (interval 3, exact per-turn radii),
`TestZoneActivationAndShrinkTimingInMatch` (interval 2 through a full replay).

## 4. Shrink step and minimum radius

- Each shrink step subtracts `zone_shrink_step` (default **1**, every player
  count — the zone must never shrink faster than a bot can move, 1 tile/turn).
- The radius is **clamped at `zone_min_radius`** and never goes below it:
  - a step that would overshoot the minimum lands **exactly** on it (e.g.
    radius 5, step 3, min 4 → 4, not 2), and
  - once at the minimum the radius holds no matter how many shrink turns pass.
- Config fields: `zone_min_radius` — **2** for 2-player, **1** for 3+ player
  (§7).

Pinned by: `TestZoneMinimumRadiusEnforcement` (overshoot clamp + hold at the
floor), `TestZoneShrinkPathForRealConfigs` (bottoms out exactly at the
documented minimum on real configs).

## 5. Boundary behavior

- Distance is **toroidal**: `Grid.Distance2` measures the shortest wrapped
  path across the map seam, so the zone's reach is judged on the torus, not by
  raw Euclidean offset. A bot at (11,11) on a 12×12 map with the center at
  (2,2) is 3 tiles away through the seam, not 9.
- The kill check is judged **after movement** (the zone phase runs after the
  move phase, §6), so a bot that crosses the seam is judged at the tile it
  wrapped to.
- The boundary is **safe**: the kill condition is strictly
  `dist2 > radius²`. A bot exactly on the boundary (`dist2 == radius²`)
  survives — including at the final minimum radius, where bots on opposite
  edges of the zone sit exactly on the boundary by construction.

Pinned by: `TestZoneToroidalBoundary` (wrapped short path, seam crossing,
corner coverage), boundary cases inside `TestZoneDamageOutsideSafeZone` and
`TestZoneDamageCatchesBotsAcrossTurns`.

## 6. Damage

- The zone damages bots only when `zone_enabled` is true **and** the zone is
  active (from the activation turn on).
- Per-turn phase order is **Move → Combat → Zone → Capture → Collect → Spawn
  → Energy tick → Endgame** (`ExecuteTurn`), so:
  - a bot killed by the zone has already moved and fought that turn, and
  - it does not participate in any later phase that turn.
- A living bot **dies iff `dist2 > radius²`** (§5's toroidal distance, strict
  inequality). Deaths are immediate — there is no per-turn damage accrual or
  grace period; the shrinking radius is the damage.
- Each zone kill: sets `Alive = false`, appends the bot to `DeadBots`,
  decrements the owner's `BotCount`, and emits exactly **one `zone_death`
  event** stamped with the turn and carrying `{bot_id, owner, position}`.
- **No score is awarded to anyone** for a zone kill — zone deaths are not
  combat kills.

Pinned by: `TestZoneDamageOutsideSafeZone` (who dies, event payload, no
score), `TestZoneDamageCatchesBotsAcrossTurns` (caught exactly on the turn the
radius first crosses each bot's distance², never before; boundary soaker
survives).

## 7. Config tiers (plan §3.7.1, as `ConfigForPlayers` materializes)

| Parameter | 2-Player | 3+ Player |
|-----------|----------|-----------|
| `zone_enabled` | true | true |
| `zone_start_turn` | 10 | 10 |
| `zone_shrink_interval` | 1 | 1 |
| `zone_shrink_step` | 1 | 1 |
| `zone_min_radius` | 2 | 1 |
| `attack_radius2` | 25 (5 tiles) | 12 (3.5 tiles) |

**Contact guarantee (the zone's purpose):** the final zone diameter must stay
within `2 × attack radius`, so bots at opposite edges of the final zone are
always within attack range of each other — the match ends in a fight, not a
standoff. 2-player: diameter 4 ≤ 2 × 5. 3+ player: diameter 2 < 2 × 3.5.

Pinned by: `TestZoneConfigDocumentedPlayerCountDifferences` (table above),
`TestZoneDocumentedContactGuaranteeAtMinimumRadius` (the guarantee itself,
walked end to end on every tier).

## 8. Disabled zones

With `zone_enabled: false`, `executeZone` returns immediately
(`engine/turn.go:129`):

- no shrink and no activation snap ever happens,
- no bot ever dies to the zone (and `zone_death` never fires),
- replay turns record **no `ZoneBounds`** — bounds are only recorded when the
  zone is enabled,
- the match otherwise runs normally and still ends on its win conditions (an
  idle-bot match completes on the turn limit with every bot alive).

Pinned by: `TestZoneDisabledMatchHasNoZoneActivity` (full-match absence
contract) and the "disabled zone does not damage" case of
`TestZoneDamageOutsideSafeZone`.

## Rule → pinning test index

| Rule | Pinned by |
|---|---|
| Activation before moves on `zone_start_turn` | `TestZoneActivationTurnKillsWithoutShrinking`, `TestZoneActivationAndShrinkTimingInMatch` |
| Activation turn kills but never shrinks | `TestZoneActivationTurnKillsWithoutShrinking` |
| First shrink at `start + interval`, exact cadence | `TestZoneShrinkCadence`, `TestZoneActivationAndShrinkTimingInMatch` |
| Step size, overshoot clamp, hold at minimum | `TestZoneMinimumRadiusEnforcement`, `TestZoneShrinkPathForRealConfigs` |
| Center anchored at map center; 90% snap | `TestZoneShrinkPathForRealConfigs`, `TestZoneDocumentedContactGuaranteeAtMinimumRadius` |
| Toroidal distance; seam judged post-move; boundary safe | `TestZoneToroidalBoundary`, `TestZoneDamageCatchesBotsAcrossTurns` |
| Kill iff `dist2 > radius²`; one `zone_death`; no score | `TestZoneDamageOutsideSafeZone` |
| Caught exactly when the radius crosses a bot | `TestZoneDamageCatchesBotsAcrossTurns` |
| Per-tier config table + contact guarantee | `TestZoneConfigDocumentedPlayerCountDifferences`, `TestZoneDocumentedContactGuaranteeAtMinimumRadius` |
| Disabled zone: no bounds, no deaths, match completes | `TestZoneDisabledMatchHasNoZoneActivity` |
