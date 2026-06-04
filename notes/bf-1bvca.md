# bf-1bvca: Deploy combat_turns Column Migration to apexalgo-iad

## Summary

Deployed the `combat_turns` column migration to fix acb-index-builder crashes on apexalgo-iad cluster.

## Changes Made

### 1. Schema Migration (Already Present)

The `combat_turns` column was already present in `~/declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml`:

- **Line 46** - In CREATE TABLE statement:
  ```sql
  combat_turns  INTEGER NOT NULL DEFAULT 0,
  ```

- **Line 305** - Migration ALTER TABLE statement:
  ```sql
  ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;
  ```

### 2. Rollout Annotation Bumped

The deployment annotation was bumped multiple times to force restart of the schema-init pod:

```yaml
checksum/schema: "v5-combat-turns-migration-2026-06-03-i"
```

Latest commit: `9908a18 fix(acb-schema-init): bump rollout annotation for combat_turns migration (v5-combat-turns-migration-2026-06-03-i)`

## Status

✅ **Migration SQL is in place** (line 305 of acb-schema-init.yml)
✅ **Changes committed and pushed** to declarative-config (9908a18)
✅ **Rollout annotation bumped** to v5-combat-turns-migration-2026-06-03-i
🔄 **ArgoCD syncing** (ai-code-battle-ns-apexalgo-iad app: OutOfSync, Operation: Running)
⏸️ **BLOCKED: apexalgo-iad cluster has insufficient CPU resources**

## Cluster Resource Issue (BLOCKING VERIFICATION)

All pods in ai-code-battle namespace are stuck in Pending state:

```
acb-api-5646489f75-l4zmq             0/1     Pending   (Insufficient cpu)
acb-evolver-7654d8b866-psvk5         0/1     Pending   (Insufficient cpu)
acb-index-builder-6669fdbc95-nxwhf   0/1     Pending   (Insufficient cpu)
acb-schema-init-5c68bc7bbc-4497t     1/1     Running   (low CPU req: 10m)
acb-worker-bf5bfdb98-g9jnn           0/1     Pending   (Insufficient cpu)
```

Error message: `0/3 nodes are available: 3 Insufficient cpu. preemption: 0/3 nodes are available: 3 No preemption victims found for incoming pod`

The index-builder pod cannot schedule to verify the fix. The schema-init pod IS running because it has very low CPU requests (10m).

Once cluster resources are available:
1. ArgoCD will complete sync of schema-init deployment
2. New schema-init pod (with updated annotation) will apply migration
3. Index-builder will schedule and no longer crash on "combat_turns does not exist"

## Migration Code Complete

The migration code is **complete and pushed** - only cluster CPU resources are blocking verification of the fix.
