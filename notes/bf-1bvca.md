# bf-1bvca: Deploy combat_turns Column Migration to apexalgo-iad

## Summary

The `combat_turns` column migration has been successfully deployed to declarative-config. The migration code is complete, committed, and pushed. Verification is **blocked by cluster CPU resource constraints** - all pods including the PostgreSQL database are stuck in Pending state.

## Changes Made (This Session)

### 1. Schema Migration - Already Present
File: `~/declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml`

- **Line 46** - CREATE TABLE statement:
  ```sql
  combat_turns  INTEGER NOT NULL DEFAULT 0,
  ```

- **Line 305** - Migration ALTER TABLE:
  ```sql
  ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;
  ```

### 2. Rollout Annotation Bumped (This Session)
Deployment annotation bumped to trigger schema-init pod restart:
```yaml
checksum/schema: "v10-combat-turns-force-apply-2026-06-03-bf-1bvca"
```
Previous: `v9-combat-turns-migration-2026-06-03-bf-1bvca-force-reapply`

### 3. Commit to declarative-config (This Session)
- `6d7439d` fix(acb-schema-init): bump checksum to force reapply combat_turns migration

## Current Cluster Status (BLOCKING)

apexalgo-iad cluster has **Insufficient CPU** - all pods stuck in Pending:

```
NAME                                 READY   STATUS    RESTARTS   AGE
acb-api-5646489f75-l4zmq             0/1     Pending   0          29m
acb-evolver-7654d8b866-psvk5         0/1     Pending   0          29m
acb-index-builder-6669fdbc95-nxwhf   0/1     Pending   0          40m
acb-map-evolver-79ff4cdf6c-7ghg4     0/1     Pending   0          40m
acb-matchmaker-64f6dc5985-vkbbl      0/1     Pending   0          40m
acb-schema-init-6cfbcc9fdc-zqhqj     1/1     Running   0          9m
acb-worker-bf5bfdb98-g9jnn           0/1     Pending   0          40m
```

**Critical:** No PostgreSQL cluster/pod exists - the CNPG cluster resource is not present in the namespace. The schema-init pod is running but cannot connect to PostgreSQL because:
1. The PostgreSQL pod isn't scheduled (CPU constraints)
2. The CNPG Cluster resource may not exist

Error: `0/3 nodes are available: 3 Insufficient cpu`

## Migration Status

| Step | Status |
|------|--------|
| Migration SQL added to schema | ✅ Complete (was already present) |
| Changes committed | ✅ Complete |
| Changes pushed | ✅ Complete |
| Rollout annotation bumped | ✅ Complete (v9 → v10) |
| Schema-init applies migration | ⏸️ Blocked (no PostgreSQL, cluster CPU) |
| Index-builder verifies fix | ⏸️ Blocked (cluster CPU, no DB) |

## Next Steps (Requires Cluster Resources)

Once apexalgo-iad has available CPU and PostgreSQL is running:
1. ArgoCD will sync schema-init deployment (checksum already bumped to v10)
2. Schema-init pod will connect to DB and apply migration
3. Index-builder will schedule and run
4. Verify index-builder logs no longer show "column combat_turns does not exist"

## Notes

- Migration is idempotent (`IF NOT EXISTS`) - safe to reapply
- Index-builder runs on 15-min cycle once scheduled
- The cluster needs CPU capacity to schedule the PostgreSQL pod first
- Without PostgreSQL, schema-init cannot apply migrations and index-builder cannot run
