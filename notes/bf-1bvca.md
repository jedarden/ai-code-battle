# P0: combat_turns Column Migration (bf-1bvca)

## Problem
`acb-index-builder` crashed every 15-min cycle with error: `column m.combat_turns does not exist`

## Solution
The schema-init manifest at `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml` already contained the combat_turns column definition:

1. **CREATE TABLE** statement (line 46): `combat_turns INTEGER NOT NULL DEFAULT 0`
2. **ALTER TABLE migration** (line 305): `ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;`

The issue was that the schema-init pod had not run since the schema was updated, so the migration had not been applied to the existing database.

## Work Completed
1. **Found schema-init manifest**: `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml`
2. **Verified schema already includes combat_turns**: Both CREATE TABLE and ALTER TABLE migrations present
3. **Bumped rollout annotation**: Updated `checksum/schema` from `v2-combat-turns-redeploy-2025-06-03` to `v3-combat-turns-redeploy-2026-06-03`
4. **Pushed to declarative-config**: Commit `51e4a36` pushed to main

## Verification
- ArgoCD will auto-sync the Deployment annotation change
- Schema-init Pod will restart and apply the migration
- acb-index-builder should succeed on next cycle

**Note:** The migration SQL was already present in the schema. The issue was that the schema-init pod had not been restarted since the migration was added, so the production database still lacked the `combat_turns` column. The annotation bump forces the restart.
