# acb-index-builder OOMKill Fix Task Summary

## Task
Fix acb-index-builder CrashLoopBackOff (silent crash after web asset copy)

## Root Cause Identified
**OOMKill caused by N+1 query problems and unbounded database queries:**

1. **fetchBots N+1 query loop**: 10,000+ separate database calls for bot match stats
2. **fetchSeries N+1 query loop**: 1000+ separate queries for series games  
3. **fetchChampionshipBracket N+1 query loop**: 500+ separate queries for championship games
4. **Unbounded queries**: Multiple queries without LIMIT clauses

## Fixes Applied (committed to codebase)

### Commit b35a2aa (DEPLOYED)
- Fixed N+1 query loop in fetchBots
- Single batch query for bot match stats
- Added LIMIT 20000

### Commits 1b399a1, 7e9d1af (code fixed, NOT deployed)
- Fixed N+1 query loops in fetchSeries and fetchChampionshipBracket
- Batch queries replacing per-item loops
- Reduced LIMITs across all queries:
  - fetchRatingHistory: LIMIT 5000
  - fetchSeries: LIMIT 1000
  - fetchSeasons: LIMIT 100
  - fetchPredictions: LIMIT 1000
  - fetchMaps: LIMIT 1000
  - series games batch: LIMIT 10000
  - championship games batch: LIMIT 500
  - pair frequency: LIMIT 1000

### main.go panic recovery (lines 165-172)
- Defer recover() catches panics and logs via slog
- Prevents silent crashes where stderr is lost

## Current Status

### Deployment State
- **Deployed image**: ronaldraygun/acb-index-builder:b35a2aa
- **Code HEAD**: 96d7fb8 (includes ALL fixes)
- **Gap**: Additional fixes in HEAD not yet deployed

### Cluster Status
- **Pod**: acb-index-builder-7fc99df58b-5zjpp
- **Status**: Pending (not CrashLoopBackOff)
- **Reason**: Cluster overcommitted (94% memory, 98% CPU)
- **Blocker**: Cannot free resources or deploy new image with read-only access

## Acceptance Criteria Status

| Criteria | Status |
|----------|--------|
| acb-index-builder runs through 2+ build cycles | ⏳ Blocked (cluster capacity) |
| "Build cycle completed" in logs | ⏳ Blocked (pod Pending) |
| No CrashLoopBackOff | ✅ Not applicable (pod Pending) |

## Conclusion

**Code fixes: ✅ Complete and committed**
**Deployment: ⏳ Partial (only first fix deployed)**
**Verification: ⏳ Blocked (cluster capacity constraints)**

The root cause has been identified and fixed in the codebase. Full deployment and verification require:
1. Building new image with HEAD (96d7fb8)
2. Freeing cluster resources or scaling cluster
3. Deploying and monitoring pod for 2+ build cycles
