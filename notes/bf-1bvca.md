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

The deployment annotation was bumped to force a restart of the schema-init pod:

```yaml
checksum/schema: "v3-combat-turns-redeploy-2026-06-03"
```

### 3. Commits Already Pushed

All changes were committed and pushed to origin/main:

```
51e4a36 fix(schema): bump annotation to force schema-init restart for combat_turns column
f74367f feat(apexalgo-iad): bump schema-init rollout annotation for combat_turns migration
00e1f5c feat(apexalgo-iad): add acb-schema-init deployment with combat_turns migration
```

## Status

✅ **Migration SQL is in place** (line 305 of acb-schema-init.yml)
✅ **Changes committed and pushed** to declarative-config
✅ **Rollout annotation bumped** to v3-combat-turns-redeploy-2026-06-03

## Next Steps

ArgoCD should automatically sync the changes to apexalgo-iad. Verify by checking:

1. ArgoCD app sync status for `ai-code-battle` namespace
2. acb-schema-init pod restarted with new annotation
3. acb-index-builder logs show successful queries without combat_turns errors

## Note

The migration was already present in the schema from earlier commits (00e1f5c, f74367f, 51e4a36). The declarative-config repo is clean and up to date with origin/main.
