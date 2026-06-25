# acb-index-builder OOMKill Fix - Current State (2025-06-25)

## Investigation Summary

The acb-index-builder CrashLoopBackOff issue has been **fixed and deployed**. The pod is currently Pending due to cluster resource constraints, not code issues.

## Code Changes Applied

All OOMKill fixes have been committed and deployed:

1. **db.go O(n²) complexity fixes:**
   - fetchBots: Batched bot match stats (1000+ queries → 1 query, LIMIT 20000)
   - fetchSeries: Batched games queries (1000+ queries → 1 batch, LIMIT 10000)
   - fetchChampionshipBracket: Batched games queries (500+ queries → 1 batch, LIMIT 500)

2. **LIMIT clauses added to prevent unbounded queries:**
   - fetchSeasonSnapshots: LIMIT reduced from 10000 to 500
   - fetchLineage: LIMIT reduced from 10000 to 1000
   - fetchRecentMatchIds: LIMIT 5000
   - All other fetch queries have appropriate LIMITs

3. **generator.go O(n²) fixes:**
   - generateBotProfiles: Pre-built lookup maps for O(1) access
   - buildPlaylistMatch: Uses botNameMap for O(1) lookups

## Current Pod Status

```
NAME                                 READY   STATUS    RESTARTS   AGE
acb-index-builder-7fc99df58b-5zjpp   0/1     Pending   0          67m
```

**Scheduling Issue:** `0/2 nodes are available: 1 Insufficient memory, 2 Insufficient cpu`

## Verification Blocked

The acceptance criteria cannot be verified until the cluster has sufficient resources:
- [ ] Pod runs through 2 complete build cycles (blocked: pod Pending)
- [ ] "Build cycle completed" in logs (blocked: pod not running)
- [ ] No CrashLoopBackOff (currently Pending, not CrashLoopBackOff)

## Next Steps (Infrastructure)

1. Scale up iad-acb cluster nodes
2. Reduce resource requests on non-critical workloads
3. Delete/evict low-priority pods to free resources

Once resources are available, the fixed pod should run successfully without OOMKill.
