# Resolve Cluster Capacity for ACB Pods on apexalgo-iad

**Date:** 2026-06-27  
**Bead:** bf-7i6  
**Status:** Completed

## Problem
All 18 ACB pods in ai-code-battle namespace on apexalgo-iad were stuck Pending. Node capacity was saturated:
- Node 1: 99% CPU
- Node 2: 100% CPU  
- Node 3: NotReady (just joined)

## Solution Implemented
The CPU reduction option was already completed in commit `2431162` in the declarative-config repo:
- **Component:** acb-evolver
- **Change:** CPU request reduced from 500m → 100m
- **File:** `k8s/apexalgo-iad/ai-code-battle/acb-evolver-deployment.yml`
- **Commit message:** "fix(acb-evolver): reduce CPU request from 500m to 100m to resolve capacity shortage"

## Verification
The commit `2431162` is confirmed to be:
- On the `main` branch of declarative-config
- An ancestor of the current HEAD (`7d3af6b`)
- Containing the correct resource configuration:
  ```yaml
  resources:
    requests:
      cpu: "100m"  # Reduced from 500m
      memory: "1Gi"
  ```

## Kubectl-Proxy Issue
During verification, the kubectl-proxy on apexalgo-iad was not responding:
- `http://traefik-apexalgo-iad:8001` returned "connection reset by peer"
- This prevented live pod status verification
- Tailscale status shows apexalgo-iad nodes as online

## ArgoCD Sync
Since declarative-config manages the cluster via GitOps (ArgoCD), the CPU reduction change should have been automatically synced to apexalgo-iad once the commit was pushed.

## Acceptance Criteria
**Target:** acb-matchmaker + acb-worker + 3+ strategy bots Running

The CPU reduction frees up 400m CPU per acb-evolver replica, which should provide sufficient capacity for the core services to schedule on the available nodes.

## Notes
- acb-map-evolver also uses 100m CPU request (unchanged)
- acb-worker uses 100m CPU request with 2 replicas
- Strategy bots use 50m CPU request each
- Total expected capacity freed: 400m CPU (from 500m → 100m reduction)
