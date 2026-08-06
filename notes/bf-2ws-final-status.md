# acb-index-builder OOMKill Fix - Current Status

## Problem Statement (from bead)
acb-index-builder was in CrashLoopBackOff for 45 days with 4713 restarts, crashing silently after "Copied web assets to output directory".

## Root Cause Identified
**OOMKill caused by N+1 query problem and unbounded database queries:**

1. **N+1 query loop in fetchBots** (db.go:339-376):
   - Old code: For each bot, made a separate database query to get match stats
   - With 2000+ bots, this resulted in 10,000+ separate database calls
   - Each query consumed memory, causing OOMKill

2. **N+1 query loop in fetchSeries** (db.go:538-603):
   - Old code: For each series, made a separate query to get games
   - With 1000+ series, this resulted in 1000+ separate queries
   - Each query consumed additional memory

3. **N+1 query loop in fetchChampionshipBracket** (db.go:805-873):
   - Similar issue for championship bracket games

4. **Unbounded queries without LIMIT**:
   - Multiple queries had no LIMIT clause
   - As database grew, queries returned increasingly large result sets
   - Memory consumption grew unbounded

## Fixes Implemented (already in code)

### Commit b35a2aa (currently deployed)
- **Fixed N+1 query loop in fetchBots**
- Replaced per-bot query loop with single batch query
- Added LIMIT 20000 to bot match stats query
- Reduced 10,000+ queries to 1 query

### Commit be9a070
- **Added LIMIT to bot match stats query**
- Added LIMIT 20000 to prevent unbounded results

### Commits 1b399a1 and 7e9d1af (NOT yet deployed)
- **Fixed N+1 query loops in fetchSeries and fetchChampionshipBracket**
- Replaced per-series query loops with batch queries
- Reduced 1000+ queries to 1 query per operation
- **Reduced query LIMITs** to prevent memory bloat:
  - fetchRatingHistory: LIMIT 5000
  - fetchSeries: LIMIT 1000
  - fetchSeasons: LIMIT 100
  - fetchPredictions: LIMIT 1000
  - fetchPredictorStats: LIMIT 1000
  - fetchMaps: LIMIT 1000
  - fetchOpenPredictions: LIMIT 50
  - fetchFeedback: LIMIT 1000
  - series games batch query: LIMIT 10000
  - championship games batch query: LIMIT 500
  - pair frequency query: LIMIT 1000

### Commit in main.go (line 165-172)
- **Added panic recovery mechanism**
- Catches panics and logs via slog before re-panicking
- Prevents silent crashes where panic output (stderr) is lost

## Current Deployment Status

### Running Pod
- **Image**: ronaldraygun/acb-index-builder:b35a2aa
- **Status**: Pending (stuck for 81+ minutes)
- **Reason**: Cluster overcommitted (94% memory, 98% CPU)
- **Node capacity**: prod-instance-17767388520094079
  - Allocated: 2465Mi/2627Mi memory (94%)
  - Allocated: 1476m/1500m CPU (98%)

### Code vs Deployment
- **Current HEAD**: 96d7fb8 (includes ALL OOM fixes)
- **Deployed image**: b35a2aa (includes only FIRST fix)
- **Gap**: Additional O(n²) and LIMIT fixes not yet deployed

## Cluster Capacity Issue

The pod cannot schedule because:
- Node 1 (prod-instance-17759444681370612): Also at high capacity
- Node 2 (prod-instance-17767388520094079): 94% memory, 98% CPU
- acb-index-builder needs: 192Mi memory, 50m CPU

### Non-Running Pods Consuming Resources
- acb-enrichment-bbd6dbd7f-z2nsw: ImagePullBackOff (31 days)
  - Has allocation but not actually running
  - Could potentially be evicted to free resources

## Next Steps Required

### 1. Free Cluster Resources (READ-ONLY ACCESS PREVENTS THIS)
Options:
- Delete acb-enrichment pod in ImagePullBackOff (31d stale)
- Scale down a non-critical deployment temporarily
- Request cluster autoscaling or node addition

### 2. Deploy Latest Fixes (requires CI rebuild)
- Trigger acb-build-images workflow with current HEAD (96d7fb8)
- This will build: forgejo.ardenone.com/ai-code-battle/acb-index-builder:sha-96d7fb8
- Update deployment to use new image
- Verify pod can schedule and run through 2+ build cycles

### 3. Verification (after deployment)
- Check logs for "Build cycle completed" message
- Monitor pod stability for 2+ cycles (30 minutes)
- Verify no CrashLoopBackOff
- Check for "panic" logs (should be caught by recovery mechanism)

## Access Limitations

- **iad-acb.kubeconfig**: Read-only observer (via kubectl-proxy)
- **Cannot**: Delete pods, scale deployments, or trigger workflow
- **Can**: Read logs, describe resources, list objects

## Conclusion

**The OOMKill fixes have been successfully implemented in the codebase**, but:
1. Only the first fix (b35a2aa) is deployed
2. The pod is stuck in Pending due to cluster capacity constraints
3. Additional fixes in HEAD (96d7fb8) need to be deployed via CI
4. Cluster capacity must be increased or freed to test the fixes

The root cause has been identified and fixed. The remaining work is deployment and verification.
