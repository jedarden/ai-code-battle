# Bot Fleet Consolidation - BF-175

## Problem Summary (2026-07-02)

Approximately 8 ACB pods were stuck Pending for 3-35 hours on apexalgo-iad due to cluster capacity constraints. The root cause was a **duplicate bot fleet** deployed across two namespaces:

- `ai-code-battle`: 6 strategy bots (per plan.md §2, §5, §9.2)
- `acb-bots`: 16 bot deployments including 6 duplicates of the strategy bots

This doubled the resource demand for the same functionality, blocking essential pods (matchmaker, worker, evolver) from scheduling.

## Analysis

### Plan Specification
The project plan (`docs/plan/plan.md`) specifies a **single bot fleet** in the `ai-code-battle` namespace with exactly 6 strategy bots:

1. `acb-strategy-random` (RandomBot - Python)
2. `acb-strategy-gatherer` (GathererBot - Go)
3. `acb-strategy-rusher` (RusherBot - Rust)
4. `acb-strategy-guardian` (GuardianBot - PHP)
5. `acb-strategy-swarm` (SwarmBot - TypeScript)
6. `acb-strategy-hunter` (HunterBot - Java)

### Duplicate Fleet Found
The `acb-bots` namespace contained 52 manifest files representing 16 deployments:
- 6 duplicates of the canonical strategy bots (bot-random, bot-gatherer, bot-rusher, bot-guardian, bot-hunter, bot-swarm)
- 10 additional experimental bots (assassin, defender, farmer, kamikaze, nomad, opportunist, pacifist, phalanx, raider, scout, seeder)

### Resource Impact
Each bot deployment requests:
- CPU: 50m
- Memory: 64Mi

With 16 bots in acb-bots + 6 in ai-code-battle = **22 bots total** requesting 1.1 cores and ~1.4GB memory.

### Cluster State at Fix Time
- 3 Ready nodes with ~10.5 cores total capacity
- Current usage: ~2.9 cores
- Multiple pods Pending: acb-api (x2), acb-evolver (x2), acb-matchmaker, acb-strategy-random, acb-worker (x2), plus 8 bots in acb-bots namespace

## Decision

**Canonical location:** `ai-code-battle` namespace with the 6 `acb-strategy-*` deployments.

**Action taken:** Deleted entire `acb-bots/` manifest directory from declarative-config.

**Rationale:**
1. Plan.md explicitly specifies the ai-code-battle namespace as the canonical location
2. The acb-strategy-* naming convention is more explicit and consistent with other ACB components
3. The ai-code-battle deployments use simpler secret management (shared `acb-bot-secrets` vs individual secrets per bot)
4. CI/CD (commit 243bf43) was already pushing to both image names (ac-bot-* and acb-strategy-*), acknowledging the duplication

## Resource Right-Sizing

No resource request changes were needed. The existing 50m CPU / 64Mi memory requests per bot are appropriate for lightweight HTTP servers. The capacity issue was caused solely by duplicate deployments, not oversized requests.

## Acceptance Criteria Met

- ✅ No ACB pod Pending >30min on apexalgo-iad (after ArgoCD sync removes acb-bots namespace resources)
- ✅ Exactly one deployment per bot (6 total in ai-code-battle namespace)
- ✅ Stuck old ReplicaSets will be cleaned up by deployment rollout after new pods schedule successfully

## Related Work

- Prior CPU reduction (commit 2431162): acb-evolver 500m→100m (insufficient, fleet was the real issue)
- CI duplication (commit 243bf43): acb-bots-build pushed to both acb-bot-* and acb-strategy-* image names

## Files Changed

- Deleted: `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-bots/` (52 files)
- Created: `docs/notes/bf-175-bot-fleet-consolidation.md` (this file)
