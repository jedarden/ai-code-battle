# acb-index-builder CrashLoopBackOff Fix Summary (Bead bf-2ws)

## Problem
acb-index-builder (iad-acb cluster) was in CrashLoopBackOff for 45 days with 4713 restarts. The pod crashed silently after the log line:
```
{"msg":"Copied web assets to output directory","source":"/app/web/dist"}
```

## Root Cause
Investigation revealed multiple O(n²) N+1 query problems causing unbounded memory growth:

1. **fetchBots**: Called getBotMatchStats for each bot (1000+ separate queries)
2. **fetchSeries**: Called fetchSeriesGames for each series (1000+ separate queries)  
3. **fetchChampionshipBracket**: Called fetchSeriesGames for each series (500+ separate queries)
4. **fetchSeasonSnapshots**: LIMIT 10000 was excessive
5. **fetchLineage**: LIMIT 10000 was excessive

The crash occurred due to OOMKill in fetchAllData() which runs immediately after copyWebAssets().

## Fix Applied
Modified cmd/acb-index-builder/db.go:

1. **fetchBots**: Batched bot match stats query (1000+ queries → 1 query with LIMIT 20000)
2. **fetchSeries**: Batched games queries with WHERE IN clause (1000+ queries → 1 batch query, LIMIT 10000)
3. **fetchChampionshipBracket**: Batched games queries with WHERE IN clause (500+ queries → 1 batch query, LIMIT 500)
4. **fetchSeasonSnapshots**: Reduced LIMIT from 10000 to 500
5. **fetchLineage**: Reduced LIMIT from 10000 to 1000

## Commits
- be9a070: fix(db): add LIMIT to bot match stats query to prevent OOMKill
- b35a2aa: fix(db): eliminate O(n²) N+1 query loop in fetchBots to prevent OOMKill
- ca48b60: fix(db): add LIMIT to fetchSeriesGames query to prevent OOMKill
- 68b7864: fix(db): add LIMIT to fetchRecentMatchIds query to prevent OOMKill
- 7befe51: fix(db): eliminate O(n²) iteration in generateBotProfiles
- 7e9d1af + 1b399a1: fix(db): reduce query LIMITs and fix O(n²) complexity to prevent OOMKill
- c1cfcde: fix(k8s): update acb-index-builder to latest image with OOMKill fixes

## Current Status (2025-06-25)
The deployment has been updated with the fixed image (ronaldraygun/acb-index-builder:b35a2aa), but the new pod cannot be scheduled due to cluster resource constraints:

```
NAME                                     READY   STATUS    RESTARTS   AGE
acb-index-builder-7fc99df58b-5zjpp       0/1     Pending   0          52m
```

**Scheduling failure reason:**
```
Warning  FailedScheduling  0/2 nodes are available: 1 Insufficient memory, 2 Insufficient cpu. 
preemption: 0/2 nodes are available: 2 No preemption victims found for incoming pod.
```

**Cluster resource pressure:**
- CPU requests: 98% capacity
- Memory limits: 293% overcommitted (node 1), 150% overcommitted (node 2)
- Memory requests: 94% capacity (node 1)

## Next Steps
The code fix is complete and committed. The cluster needs additional resources or workload rebalancing before the acb-index-builder pod can run and verify the fix:

1. Scale up iad-acb cluster nodes
2. Reduce resource requests/limits on non-critical workloads
3. Delete/evict pods with low priority to free up resources

Once the pod can be scheduled, the fix should be verified by checking logs for:
- "Build cycle completed" log line
- No crashes after "Copied web assets to output directory"
- Stable pod state (not CrashLoopBackOff)
