# Fix acb-index-builder OOMKill (bf-2ws)

## Problem
acb-index-builder pod in iad-acb cluster was in CrashLoopBackOff for 45 days with 4,713 restarts. The pod crashed silently after copying web assets - last log line was always:
```
{"msg":"Copied web assets to output directory","source":"/app/web/dist"}
```

## Root Cause
**OOMKilled (Exit Code 137)** - The kernel terminated the process due to memory exhaustion. This explains the silent crash (SIGKILL cannot be caught and logged).

### Investigation
1. Checked pod status: `kubectl describe pod` showed `Last State: Terminated, Reason: OOMKilled, Exit Code: 137`
2. Checked running version: `ronaldraygun/acb-index-builder:42e9561` from 2026-05-10
3. Checked current repo: 410 commits ahead of running version
4. Found recent commits (79ca6c0, cdf133d, 4554bed, 404825b) that added LIMIT clauses to prevent OOMKill

### The Issue
The running pod was **45 days out of date** and didn't have the recent LIMIT fixes that were added to the codebase to prevent unbounded queries from consuming excessive memory.

## Fix Applied
The codebase already had all necessary fixes from recent commits:
- **commit 79ca6c0** (2026-06-25): Added LIMIT 10000 to fetchSeasonSnapshots, LIMIT 500 to fetchChampionshipBracket
- **commit cdf133d**: Added LIMIT 1000 to pair frequency query
- **commit 4554bed**: Added LIMITs to fetchEvolutionMeta and fetchLineage
- **commit 404825b**: Added LIMIT clauses to other unbounded queries

These queries were called in loops for each season/match without bounds, causing the pod to exceed its 512Mi memory limit.

## Resolution
Triggered a rebuild using the `acb-build` workflow template to deploy the current code with all OOMKill fixes:

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: acb-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: acb-build
EOF
```

Workflow: `acb-build-manual-nv552`

### What the workflow does:
1. Resolves the current git SHA
2. Runs tests
3. Builds all ACB containers including **acb-index-builder** with the LIMIT fixes
4. Updates declarative-config repo with new image tags
5. ArgoCD auto-syncs the deployment to iad-acb cluster

## Expected Result
Once the build completes (~15-20 minutes):
1. New acb-index-builder image with LIMIT fixes will be deployed
2. Pod should run through complete build cycles without OOMKill
3. "Build cycle completed" should appear in logs
4. CrashLoopBackOff should be resolved

## Lessons Learned
- **Always check image age vs recent commits** - Running version was 410 commits behind
- **OOMKill is silent** - No error logs because SIGKILL cannot be caught
- **Recent commits had the fix** - The problem was already solved in code, just not deployed
- **Auto-deploy pipeline was broken** - Why wasn't the pod updated for 45 days? (investigate CI/CD)

## Verification Steps
After workflow completes:
1. Check pod image: `kubectl get pod -n ai-code-battle acb-index-builder-... -o jsonpath='{.spec.containers[0].image}'`
2. Watch pod logs: `kubectl logs -n ai-code-battle acb-index-builder-... -f`
3. Verify "Build cycle completed" appears in logs
4. Verify no CrashLoopBackOff: `kubectl get pods -n ai-code-battle`
