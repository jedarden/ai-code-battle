# Fix acb-index-builder CrashLoopBackOff (Bead bf-2ws)

## Problem
acb-index-builder (iad-acb cluster) was in CrashLoopBackOff for 45 days with 4713 restarts. The pod crashed silently after the log line:
```
{"msg":"Copied web assets to output directory","source":"/app/web/dist"}
```

No error was logged, indicating either:
1. A panic in fetchAllData or generateAllIndexes (unlikely - defer recover() logs panics)
2. OOMKill (most likely - kernel SIGKILL terminates before any Go code can log)

## Root Cause
Investigation of cmd/acb-index-builder/db.go revealed multiple O(n²) N+1 query problems causing unbounded memory growth:

1. **fetchSeries** (line 531): For each of up to 1000 series, called fetchSeriesGames making 1000 separate queries
2. **fetchChampionshipBracket** (line 736): For each of up to 500 series, called fetchSeriesGames making 500 separate queries  
3. **fetchSeasonSnapshots**: LIMIT 10000 was excessive
4. **fetchLineage**: LIMIT 10000 was excessive

The crash occurred in fetchAllData() which runs immediately after copyWebAssets().

## Fix Applied
Modified cmd/acb-index-builder/db.go:

1. **fetchSeries**: Batched games queries (1000 queries → 1 query with WHERE IN clause, LIMIT 10000)
2. **fetchChampionshipBracket**: Batched games queries (500 queries → 1 query with WHERE IN clause, LIMIT 64)  
3. **fetchSeasonSnapshots**: Reduced LIMIT from 10000 to 500
4. **fetchLineage**: Reduced LIMIT from 10000 to 1000
5. Added `strings` import for strings.Join in batch queries

## Deployment
- Committed and pushed changes to ai-code-battle repo
- Submitted container-build workflow to iad-ci cluster to rebuild acb-index-builder container
- Workflow name: acb-index-builder-build-v25d2 (status: Running)

## Expected Outcome
Once the new image is deployed to iad-acb cluster:
- acb-index-builder should complete build cycles without OOMKill
- "Build cycle completed" log line should appear
- Pod should stop restarting and exit CrashLoopBackOff

## Verification Needed
After deployment, monitor:
```bash
kubectl --server=http://traefik-iad-acb:8001 logs -n ai-code-battle acb-index-builder-XXX -f
```
Look for:
- "Build cycle completed" log line
- No crashes after "Copied web assets to output directory"
- Stable pod state (not CrashLoopBackOff)
