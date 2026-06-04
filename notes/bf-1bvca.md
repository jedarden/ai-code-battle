# P0: combat_turns Column Migration (bf-1bvca)

## Problem
`acb-index-builder` crashed every 15-min cycle with error: `column m.combat_turns does not exist`

## Solution
Added migration to declarative-config `k8s/iad-acb/ai-code-battle/acb-schema-init.yml`:

```sql
ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;
```

Also bumped rollout annotation from `v6-playlists-tables` to `v7-combat-turns` to force schema-init Pod restart.

## Commit
jedarden/declarative-config@845d59d

## Verification
- ArgoCD will auto-sync the ConfigMap and Deployment changes
- Schema-init Pod will restart and apply the migration
- acb-index-builder should succeed on next cycle
