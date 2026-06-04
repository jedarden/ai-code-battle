# bf-1bvca: combat_turns column migration

## Task Summary
Add `combat_turns` column migration to acb-schema-init to fix index-builder crashes.

## Work Completed

### Schema Migration (Already Done)
The `combat_turns` migration was already present in `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml`:

1. **Line 46** - CREATE TABLE includes the column:
   ```sql
   combat_turns INTEGER NOT NULL DEFAULT 0
   ```

2. **Line 305** - Migration for existing tables:
   ```sql
   ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;
   ```

3. **Line 508** - Checksum bumped to force reapply:
   ```yaml
   checksum/schema: "v10-combat-turns-force-apply-2026-06-03-bf-1bvca"
   ```

### Git History
Multiple commits exist for this migration (declarative-config):
- `6d7439d` - fix(acb-schema-init): bump checksum to force reapply combat_turns migration
- `a6b9f46` - fix(ai-code-battle): bump schema-init annotation to force reapply combat_turns migration
- `5e65253` - fix(acb): bump schema-init annotation to apply combat_turns migration
- `503724e` - fix(apexalgo-iad): bump schema-init annotation to v7 for combat_turns migration

## Current Blocker: Cluster CPU Exhaustion

The migration **cannot be applied** because the apexalgo-iad cluster is out of CPU:

### Postgres Database Status
- **Cluster**: `cnpg-apexalgo` in `cnpg` namespace
- **Pod Status**: `cnpg-apexalgo-3` is **Pending** (23+ days)
- **Reason**: `0/3 nodes are available: 3 Insufficient cpu`
- **Service Endpoints**: `acb-postgres` service has **no endpoints** (no active postgres pod)

### Schema-init Pod Status
- **Pod**: `acb-schema-init-7976d55cb-pwpnn` is **Running**
- **Logs**: Stuck in retry loop waiting for postgres

### Index-builder Status
- **Pod**: `acb-index-builder-6669fdbc95-nxwhf` is **Pending**
- **Reason**: `0/3 nodes are available: 3 Insufficient cpu`

### Node Capacity
Total cluster capacity is ~3 vCPU across 3 nodes.

## Migration Status
- **Code**: ✅ Complete (already in declarative-config)
- **Applied**: ❌ Blocked (no postgres running)
- **Verified**: ❌ Blocked (index-builder not running)

## Next Actions
Infrastructure issue: Add more CPU to apexalgo-iad cluster or scale down workloads.
