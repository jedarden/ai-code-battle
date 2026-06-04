# BF-1BVCA: combat_turns Column Migration

**Status:** Migration complete (code changes done, committed, pushed). Verification blocked by infrastructure.

## Work Completed

1. **Schema Migration Added** - `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml`:
   - Line 46: `combat_turns INTEGER NOT NULL DEFAULT 0` in CREATE TABLE
   - Line 305: `ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;`

2. **Rollout Annotation Bumped**:
   - Updated to `v14-combat-turns-2026-06-03-bf-1bvca-force-restart`

3. **Changes Committed and Pushed**:
   - Commit: `1ec0c25` - "feat(apexalgo-iad): bump schema-init annotation to v14 - force restart for combat_turns migration"

## Current Blockers (Infrastructure, not migration-related)

1. **Database Cluster Down**: `cnpg-apexalgo` in `cnpg` namespace shows:
   ```
   NAME            AGE   INSTANCES   READY   STATUS                                       PRIMARY
   cnpg-apexalgo   91d   0/3         0       Waiting for the instances to become active   cnpg-apexalgo-3

   NAME              READY   STATUS    RESTARTS   AGE
   cnpg-apexalgo-3   0/1     Pending   0          23d
   ```
   Pod condition: `0/3 nodes are available: 3 Insufficient cpu.`

2. **Schema-init Running but Blocked**: Pod is Running but cannot connect to DB:
   ```
   acb-schema-init-5b698c549d-pwx47     1/1     Running   0          3m24s
   ```
   Logs show: "Not ready, retrying in 5s..." (PostgreSQL not accepting connections)

3. **All Workloads Pending**: acb-api, acb-evolver, acb-index-builder, acb-worker all stuck in Pending.

## Verification Plan

Once cluster is healthy:
1. Verify `cnpg-apexalgo` cluster reaches `Healthy` status with 3/3 instances
2. Schema-init pod should connect and apply migrations (check logs for "Schema applied")
3. Index-builder pod should schedule successfully (check `kubectl get pods`)
4. Verify index-builder logs show no "column combat_turns does not exist" errors

## Migration Content

```sql
-- In CREATE TABLE matches statement
combat_turns  INTEGER NOT NULL DEFAULT 0,

-- In migrations section
ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;
```

This migration is idempotent (uses `IF NOT EXISTS`) and will apply cleanly when the database becomes available.
