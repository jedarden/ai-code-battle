# Matchmaker Job Creation Verification - bf-3u9

**Date:** 2026-06-27
**Cluster:** apexalgo-iad
**Namespace:** ai-code-battle

## Critical Finding: Cluster Capacity Blocks Job Creation

The acb-matchmaker logs **cannot be checked** because the matchmaker pod has never been able to start. All pods in the ai-code-battle namespace are stuck in Pending state due to insufficient cluster CPU capacity.

## Current Cluster Status

### Nodes (3 total)
- **prod-instance-17781842321795040**: Ready, 32% CPU (1152m/3500m used), 15% memory
- **prod-instance-17825487911280674**: Ready, 47% CPU (1667m/3500m used), 65% memory  
- **prod-instance-17825591427380770**: **NotReady**, 2% CPU (83m), 12% memory

### Pod Status
- **Running**: Only `acb-schema-init-5b698c549d-wzhnc` (1/1)
- **Pending**: All other pods including:
  - `acb-matchmaker-64f6dc5985-9vh67` (pending for 63+ minutes)
  - `acb-api-5646489f75-fs7wx` 
  - `acb-worker-bf5bfdb98-68k4r`
  - 8 bot strategy pods (random, rusher, gatherer, guardian, hunter, swarm, farmer)
  - `acb-evolver`, `acb-enrichment`, `acb-index-builder`

### Job Creation Status
**No jobs exist** in the ai-code-battle namespace. Job creation cannot occur because:
1. The matchmaker pod cannot schedule due to insufficient CPU
2. Even if scheduled, the matchmaker requires PostgreSQL connection (from pending pods)
3. Workers are also pending, so no jobs could execute even if created

## Scheduling Failure Details

All pending pods show this pattern:
```
0/3 nodes are available: 1 node(s) had untolerated taint {node.kubernetes.io/not-ready: }, 2 Insufficient cpu
```

The `NotReady` node (`prod-instance-17825591427380770`) appears to be a newly added node (7h8m old) that may still be initializing or has issues.

## Resource Analysis

### Available CPU (Ready nodes only)
- Node 1: ~2348m available (3500m - 1152m used)
- Node 2: ~1833m available (3500m - 1667m used)
- **Total available: ~4181m CPU**

### Pending pod CPU requests (estimated)
- acb-matchmaker: 50m
- acb-api (2 pods): 200m
- acb-enrichment (2 pods): 400m  
- acb-evolver (2 pods): 1000m
- acb-worker (2 pods): ~200m
- 8 bot strategy pods: ~400m
- acb-index-builder: 50m
- **Total requests: ~2250m**

Theoretically there should be enough CPU (~4181m available vs ~2250m needed), but scheduler reports insufficient CPU. This suggests:
1. Other workloads on the cluster consuming CPU not shown in `kubectl top nodes`
2. Resource fragmentation preventing scheduling of larger pods
3. The NotReady node blocking some scheduling attempts

## Verification Conclusion

**Status: ❌ VERIFICATION FAILED - Infrastructure Issue**

The matchmaker job creation cannot be verified because:
1. **Cluster capacity insufficient** - Matchmaker pod cannot schedule
2. **No jobs in queue** - Query returns 0 jobs (expected since matchmaker never ran)
3. **No logs available** - Pod never started, so no logs to check

## Next Steps Required

1. **Fix cluster capacity** - Either:
   - Add more nodes to the cluster
   - Scale down resource requests for ACB pods
   - Move other workloads off apexalgo-iad to free capacity

2. **Fix NotReady node** - Investigate why `prod-instance-17825591427380770` is NotReady

3. **Re-deploy ACB stack** - Once capacity is available, delete and recreate pods

4. **Re-run verification** - Check matchmaker logs after pods are running

## Acceptance Criteria Status

- ❌ acb-matchmaker logs show successful job creation - **CANNOT VERIFY** (pod never started)
- ❌ Jobs appear in the queue with valid bot pairs - **NO JOBS** (matchmaker never ran)
- ❌ No errors in matchmaker scheduling logic - **CANNOT VERIFY** (no logs)

## Recommendation

This verification should be **re-attempted** after cluster capacity is restored. The current apexalgo-iad cluster appears under-provisioned for the ACB workload.
