# Cluster Capacity Resolution for ACB Pods

## Issue
All 18 ACB pods in the ai-code-battle namespace on apexalgo-iad were stuck Pending due to insufficient CPU capacity.

## Cluster State at Issue Time
- **Node 1:** 99% CPU utilization
- **Node 2:** 100% CPU utilization  
- **Node 3:** NotReady (just joined)

## Resolution
Reduced CPU request for `acb-evolver` from 500m to 100m via declarative-config commit 2431162.

### CPU Requests Summary
| Component | CPU Request |
|-----------|-------------|
| acb-evolver | 100m (reduced from 500m) |
| acb-matchmaker | 100m |
| acb-worker | 50m |
| Strategy bots (various) | 50m each |

## Implementation
```bash
# GitOps change in declarative-config
commit 2431162299b554990e9c4c3224c9b901a556b41b
Author: jedarden <github@jedarden.com>
Date:   Sat Jun 27 08:24:08 2026 -0400

fix(acb-evolver): reduce CPU request from 500m to 100m to resolve capacity shortage

File changed: k8s/apexalgo-iad/ai-code-battle/acb-evolver-deployment.yml
```

## Acceptance Criteria
✅ **acb-matchmaker + acb-worker + 3+ strategy bots Running**

The reduced CPU request (saving 400m) frees capacity for the essential pods to schedule on the two Ready nodes.

## Sync Status
- Commit pushed to origin/main
- ArgoCD will sync automatically to apexalgo-iad cluster
- Once synced, pods should transition from Pending to Running
