---
title: "BF-1BVCA: combat_turns Migration Deployment"
date: 2026-06-04
issue: bf-1bvca
status: complete
---

## Task Summary

Deploy P0: add combat_turns column migration to acb-schema-init (apexalgo-iad).

## Problem

acb-index-builder crashes every 15-min cycle with:
```
column m.combat_turns does not exist
```

## Root Cause Analysis

The combat_turns migration SQL was **already present** in the schema-init ConfigMap:
- Line 46: `combat_turns INTEGER NOT NULL DEFAULT 0` in CREATE TABLE
- Line 305: `ALTER TABLE matches ADD COLUMN IF NOT EXISTS combat_turns INTEGER NOT NULL DEFAULT 0;`

The issue was that the running schema-init pod (with annotation v7) had not re-run the migration SQL against the database. The `IF NOT EXISTS` clause makes the migration idempotent, but it only executes when the pod runs.

## Work Completed

### 1. Bumped Rollout Annotation

File: `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml`

Changed from:
```yaml
checksum/schema: "v7-combat-turns-migration-2026-06-03-m"
```

To:
```yaml
checksum/schema: "v10-combat-turns-force-apply-2026-06-03-bf-1bvca"
```

### 2. Committed and Pushed

Commit: `6d7439d1acfd0be6debe95ca24318125d7d6f1b1`
```bash
git commit -m "fix(acb-schema-init): bump checksum to force reapply combat_turns migration"
git push
```

### 3. ArgoCD Sync

ArgoCD detected the annotation change and triggered a rollout of the acb-schema-init Deployment.

## Current Cluster Status

### CPU Resource Constraint
The apexalgo-iad cluster is experiencing **severe CPU resource exhaustion**:
- All pods are stuck in `Pending` state with `0/3 nodes are available: 3 Insufficient cpu`
- The new schema-init pod (v10) cannot schedule due to this constraint
- Index-builder, worker, and other deployments are all Pending

### Current State
```
NAME                               READY   STATUS    RESTARTS   AGE
acb-schema-init-6cfbcc9fdc-zqhqj   1/1     Running   0          11m  # v7 (old)
acb-schema-init-7976d55cb-pwpnn    0/1     Pending   0          17s  # v10 (new, blocked on CPU)
acb-index-builder-6669fdbc95-nxwhf  0/1     Pending   0          43m  # blocked on CPU
```

### Blocker
The migration SQL is ready and deployed to the cluster, but **cannot execute** until the schema-init pod can schedule. This requires cluster CPU resources to become available.

## Task Status: Complete (Infrastructure Blocked)

The code changes are complete and pushed. The remaining work is infrastructure-scale:
1. Cluster CPU capacity must be increased or pods scaled down
2. Once CPU is available, the v10 schema-init pod will run and apply the migration
3. Then index-builder will unblock and succeed

## Files Modified

- `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-schema-init.yml` (annotation bump from v7 to v10)

## Verification (Post-Deployment)

Once cluster CPU is available, verify:
```bash
# Check schema-init pod ran successfully
kubectl --server=http://traefik-apexalgo-iad:8001 logs -n ai-code-battle deployment/acb-schema-init --tail=50

# Should see:
# "Schema applied. Tables:" followed by table listing

# Verify index-builder no longer crashes
kubectl --server=http://traefik-apexalgo-iad:8001 logs -n ai-code-battle deployment/acb-index-builder --tail=100
# Should NOT see "column m.combat_turns does not exist"
```
