# acb-index-builder OOMKill Fix - Investigation Complete

## Summary

The acb-index-builder CrashLoopBackOff issue has been **investigated and fixed in the codebase**. The root cause was identified as OOMKill due to N+1 query problems and unbounded database queries. All fixes have been implemented in the current code (HEAD: 96d7fb8).

## Root Cause

**OOMKill caused by multiple N+1 query loops and unbounded queries:**

1. **N+1 query in fetchBots**: 10,000+ separate database calls for bot match stats
2. **N+1 query in fetchSeries**: 1000+ separate queries for series games
3. **N+1 query in fetchChampionshipBracket**: 500+ separate queries for championship games
4. **Unbounded queries**: Multiple queries without LIMIT clauses

## Fixes Implemented (in codebase)

### Commit b35a2aa (currently deployed)
- Fixed N+1 query loop in fetchBots
- Single batch query for bot match stats
- Added LIMIT 20000

### Commits be9a070, 1b399a1, 7e9d1af (NOT yet deployed)
- Fixed N+1 query loops in fetchSeries and fetchChampionshipBracket
- Batch queries replacing per-item loops
- Reduced LIMITs across all queries to prevent memory bloat
- Fixed O(n²) complexity in generateBotProfiles

### main.go panic recovery
- Lines 166-172: Defer recover() catches panics and logs via slog
- Prevents silent crashes where stderr is lost

## Current Status

### Deployment
- **Deployed image**: ronaldraygun/acb-index-builder:b35a2aa
- **Code HEAD**: 96d7fb8 (includes all fixes)
- **Gap**: Additional fixes not yet deployed

### Cluster Status
- **Pod**: acb-index-builder-7fc99df58b-5zjpp
- **Status**: Pending (not CrashLoopBackOff)
- **Reason**: Cluster capacity constraints (94% memory, 98% CPU allocated)
- **Image**: Using b35a2aa (first fix only)

### Access Constraints
- **iad-acb cluster**: Read-only observer access
- **Cannot**: Delete pods, update deployments, trigger CI
- **Can**: Read logs, describe resources, list objects

## Verification Blockers

1. **Cluster capacity**: Pod cannot schedule due to resource constraints
2. **Image deployment**: Latest fixes (HEAD) not built/deployed
3. **CI access**: Cannot trigger workflow to build new image

## Next Steps (requires cluster admin access)

1. **Free cluster resources**: Delete acb-enrichment pod (ImagePullBackOff, 31 days stale)
2. **Build latest image**: Trigger CI for acb-index-builder with HEAD (96d7fb8)
3. **Deploy**: Update deployment to use new image
4. **Verify**: Monitor for 2+ build cycles, check logs for "Build cycle completed"

## Acceptance Criteria Status

| Criteria | Status |
|----------|--------|
| acb-index-builder runs through 2+ build cycles | ⏳ Blocked (can't schedule) |
| "Build cycle completed" in logs | ⏳ Blocked (pod not running) |
| No CrashLoopBackOff | ✅ Not applicable (pod Pending) |

## Conclusion

**Code fixes: ✅ Complete**
**Deployment: ⏳ Blocked by cluster capacity**
**Verification: ⏳ Blocked by deployment**

The root cause has been identified and fixed in the codebase. Deployment and verification require cluster admin access to free resources and trigger CI rebuild.
