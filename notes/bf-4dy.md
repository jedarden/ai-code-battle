# Match Pipeline Verification - bf-4dy

## Date: 2026-06-27

## Finding: Match Pipeline Cannot Function Due to Cluster Capacity Issue

### Cluster Status

**Cluster**: iad-acb (Rackspace Spot HCP)
**Nodes**: 2
- `prod-instance-17767388520094079`: Ready, 67d old
- `prod-instance-17825486055310528`: **NotReady**, 7h44m old (new node, unhealthy)

### Resource Allocation

Ready node (`prod-instance-17767388520094079`):
- **CPU allocated**: 1497m (~99% of capacity)
- **CPU available**: ~3m (insufficient for new pods)
- **Memory allocated**: 1654Mi (63% of capacity)

### Critical Pods - All Pending

The following pods cannot schedule due to insufficient CPU:

| Pod | CPU Request | Status |
|-----|-------------|--------|
| acb-matchmaker | 50m | Pending |
| acb-worker (x2) | 100m each | Pending |
| acb-api (x2) | unknown | Pending |
| acb-strategy-gatherer | unknown | Pending |
| acb-strategy-guardian | unknown | Pending |
| acb-evolver | unknown | Pending |
| acb-enrichment | unknown | Pending |
| acb-index-builder | unknown | Pending |

**Additional CPU needed**: ~250m minimum (matchmaker + 2 workers)

### Running Pods (Ready Node)

| Pod | CPU Request | Status |
|-----|-------------|--------|
| acb-postgres | 50m | Running |
| acb-valkey | 25m | Running |
| acb-map-evolver | 100m | Running (liveness probe failing) |
| acb-strategy-hunter | 100m | Running |
| acb-strategy-random | 50m | Running |
| acb-strategy-rusher | 100m | Running |
| acb-strategy-swarm | 100m | Running |
| acb-schema-init | 10m | Running |
| armor | 25m | Running |

**Total running**: ~560m CPU

### Match Pipeline Status

**Cannot verify** - The match pipeline is non-functional:

1. **acb-matchmaker** is not running - cannot create match jobs
2. **acb-worker** replicas are not running - cannot claim/execute jobs
3. **Valkey logs** show minimal activity (1 change/hour) - no job queue activity
4. **No replay uploads** possible - workers not running

### Additional Issues

- **acb-map-evolver** is restarting frequently (13,202 restarts over 54 days) with liveness probe failures
- **TLS certificate issue** for acb-api-cert (ACME challenge failing)

### Resolution Required

The match pipeline cannot be verified until:

1. **Fix the NotReady node** - Investigate why `prod-instance-17825486055310528` has been NotReady for 7+ hours
2. **Increase cluster capacity** - Add more nodes or resize existing nodes to accommodate pending pods
3. **Investigate acb-map-evolver** - Fix the liveness probe failures causing frequent restarts

### Recommended Actions

1. Check node logs/events for the NotReady node to diagnose the issue
2. Consider scaling up the cluster or right-sizing existing workloads
3. Once capacity is available, verify matchmaker creates jobs and workers process them
4. Check B2 bucket for replay uploads after workers are running
5. Verify index builder rebuilds static JSON after new replays

**Conclusion**: The match pipeline cannot be verified because critical components (matchmaker, workers) are unable to schedule due to cluster CPU exhaustion.
