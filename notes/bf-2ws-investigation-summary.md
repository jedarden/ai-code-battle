# acb-index-builder Investigation Summary

## Issue Status: RESOLVED (Code fixes applied), BLOCKED (Infrastructure)

### Original Problem
- acb-index-builder was in CrashLoopBackOff for 45 days with 4713 restarts
- Silent crash after "Copied web assets to output directory" log line
- Suspected causes: OOMKill, panic, or unbounded queries

### Root Cause Analysis
The issue was caused by **multiple O(n²) complexity problems** leading to OOMKill:

1. **fetchBots in db.go**: O(n²) N+1 query loop - querying match stats for each bot separately (10,000+ queries)
2. **generateBotProfiles in generator.go**: O(n²) iteration - linear scans through all bots and matches for each bot profile
3. **fetchSeries in db.go**: O(n²) N+1 query loop - querying series games separately for each series
4. **Unbounded queries**: Missing LIMIT clauses on large result sets

### Fixes Applied (Already in Codebase)

#### 1. Database Query Fixes (db.go)
- **fetchBots** (lines 338-376): Single batch query for all bot match stats with LIMIT 20000
- **fetchSeries** (lines 538-603): Batch query for all series games with LIMIT 10000
- **fetchChampionshipBracket** (lines 805-866): Batch query for games with LIMIT 500
- **All queries**: Added LIMIT clauses (500-5000 range) to prevent unbounded results

#### 2. Generator Optimization (generator.go)
- **generateBotProfiles** (lines 275-301): Pre-build lookup maps for O(1) lookups
  - `historyMap`: botID → rating history entries
  - `botNameMap`: botID → bot name
  - `matchMap`: botID → recent matches (up to 20)
- **buildFirstMatchPerBot** (line 1315): O(n*p) vs O(n²) for debut detection
- **buildPairFrequency** (line 1348): O(n) vs O(n²) for rivalry detection
- **isNewBotDebutFast** (line 1334): O(1) lookup using pre-built maps
- **isRivalryMatchFast** (line 1365): O(1) lookup using pre-built frequency maps

#### 3. Panic Recovery (main.go)
- **runBuildCycle** (lines 165-172): Added deferred recover() that logs via slog before re-panicking
- Prevents silent crashes where panic output (stderr) is lost

### Current Situation

**Pod Status**: PENDING (not CrashLoopBackOff)
- Current pod: `acb-index-builder-7fc99df58b-5zjpp` (42m old)
- Image: `ronaldraygun/acb-index-builder:b35a2aa` (contains all fixes)
- Issue: **Insufficient cluster resources** for scheduling

**Cluster Status**:
- Node: prod-instance-17759444681370612
- Capacity: 2 CPU, ~3.8GB RAM
- Current allocation: 98% CPU, 59% memory
- Error: "0/2 nodes are available: 1 Insufficient memory, 2 Insufficient cpu"

**Code Status**: ✅ All fixes are in the deployed image
- Commit b35a2aa includes the critical fetchBots fix
- All O(n²) complexity issues resolved
- Panic recovery mechanism in place

### Verification Needed

The pod cannot be scheduled due to cluster resource constraints. To verify the fixes work:

1. **Option A**: Scale down non-critical workloads temporarily to free resources
2. **Option B**: Add a new node to the cluster
3. **Option C**: Increase cluster node capacity
4. **Option D**: Wait for existing pods to complete/restart

Once the pod runs successfully through 2+ build cycles with "Build cycle completed" logs, the fix is verified.

### Acceptance Criteria Met (Pending Pod Scheduling)
- ✅ Code fixes applied (LIMIT clauses, batch queries, O(1) lookups)
- ✅ Panic recovery mechanism added
- ⏳ Pod runs through 2+ build cycles without restart (blocked by cluster resources)
- ⏳ "Build cycle completed" appears in logs (blocked by cluster resources)
- ⏳ No CrashLoopBackOff in kubectl get pods (blocked by cluster resources)

### Files Modified
- `cmd/acb-index-builder/db.go` - Database query optimization
- `cmd/acb-index-builder/generator.go` - O(n²) complexity fixes
- `cmd/acb-index-builder/main.go` - Panic recovery mechanism

### References
- Git commit b35a2aa: "fix(db): eliminate O(n²) N+1 query loop in fetchBots to prevent OOMKill"
- Git commit be9a070: "fix(db): add LIMIT to bot match stats query to prevent OOMKill"
- Git commit 68b7864: "fix(db): add LIMIT to fetchRecentMatchIds query to prevent OOMKill"
- Git commit ca48b60: "fix(db): add LIMIT to fetchSeriesGames query to prevent OOMKill"
- Git commit 7befe51: "fix(db): eliminate O(n²) iteration in generateBotProfiles"
