# Match Pipeline Verification Report (Bead bf-4dy)

## Executive Summary

**Status: ❌ CRITICAL - Match Pipeline Non-Operational**

The end-to-end match pipeline is **completely non-functional** due to cluster infrastructure constraints. Critical components (matchmaker, workers, index-builder) cannot schedule, preventing:
- Bot pairing and match creation
- Match execution
- Replay upload to B2
- Static JSON index regeneration

## Cluster Infrastructure Status

### Node Status (2026-06-27 08:40 UTC)

| Node | Status | Issue |
|------|--------|-------|
| prod-instance-17767388520094079 | ✅ Ready | Running at capacity |
| prod-instance-17825486055310528 | ❌ NotReady | 4h9m in NotReady state |

### Pod Status

| Component | Pods | State | Issue |
|-----------|------|-------|-------|
| **acb-matchmaker** | 1/1 | ❌ Pending (6h22m) | Cannot schedule - insufficient CPU |
| **acb-worker** | 0/2 | ❌ Pending (6h22m) | Cannot schedule - insufficient CPU |
| **acb-index-builder** | 0/1 | ❌ Pending (2d2h) | Cannot schedule - insufficient CPU |
| acb-api | 0/2 | ❌ Pending (4h19m) | Cannot schedule |
| acb-evolver | 0/1 | ❌ Pending (4h19m) | Cannot schedule |
| acb-strategy-gatherer | 0/1 | ❌ Pending (6h22m) | Cannot schedule |
| acb-strategy-guardian | 0/1 | ❌ Pending (6h22m) | Cannot schedule |
| **acb-strategy-hunter** | 1/1 | ✅ Running | Operational |
| **acb-strategy-random** | 1/1 | ✅ Running | Operational |
| **acb-strategy-rusher** | 1/1 | ✅ Running | Operational |
| **acb-strategy-swarm** | 1/1 | ✅ Running | Operational |
| acb-map-evolver | 1/1 | ⚠️ Running | High restarts (13167 in 54d) |
| acb-postgres | 2/2 | ✅ Running | Operational |

## Match Pipeline Component Analysis

### 1. Matchmaker (`acb-matchmaker`)

**Status:** ❌ Pending (cannot schedule)

**Expected Behavior:**
- Poll database for active bots
- Find bot pairs with similar ratings
- Create match jobs in database
- Run every 60 seconds (ACB_MATCHMAKER_INTERVAL)

**Actual State:**
- Pod cannot schedule due to insufficient CPU on cluster
- Last successful scheduling attempt: 6h22m ago
- Error: `0/2 nodes are available: 1 Insufficient cpu, 1 node(s) had untolerated taint {node.kubernetes.io/not-ready}`

**Verification:** ❌ Cannot verify - logs unavailable (pod not running)

---

### 2. Worker (`acb-worker`)

**Status:** ❌ Pending (cannot schedule) × 2 replicas

**Expected Behavior:**
- Poll job queue for pending matches
- Claim jobs via Valkey
- Execute match engine (spawn units, run turns, determine winner)
- Upload replay JSON to B2
- Mark job complete in database

**Actual State:**
- Both replicas cannot schedule due to insufficient CPU
- Last successful scheduling attempt: 6h22m ago
- Error: `0/2 nodes are available: 1 Insufficient cpu, 1 node(s) had untolerated taint {node.kubernetes.io/not-ready}`

**Verification:** ❌ Cannot verify - logs unavailable (pods not running)

---

### 3. Index Builder (`acb-index-builder`)

**Status:** ❌ Pending (cannot schedule)

**Expected Behavior:**
- Fetch all data from database (matches, bots, ratings, etc.)
- Generate static JSON index files
- Upload to B2/Cloudflare Pages
- Run on 30-minute cycle

**Actual State:**
- Pod cannot schedule for 2d2h
- Previous iteration had OOMKill issues (fixed in code but not deployed)
- Error: `0/2 nodes are available: 1 Insufficient cpu, 1 node(s) had untolerated taint {node.kubernetes.io/not-ready}`

**Verification:** ❌ Cannot verify - logs unavailable (pod not running)

---

### 4. B2/R2 Storage

**Status:** ⚠️ Credentials corrupted (known issue)

**Known Issues:**
- R2 credentials in OpenBao are corrupted/swapped (see IAD-ACB-R2-CREDENTIALS-FIX.md)
- `endpoint` contains SHA256 hash instead of URL
- `secret-key` contains endpoint URL instead of actual secret key
- This would cause replay upload failures even if workers were running

**Bucket:** `acb-data` on Cloudflare R2
**Expected replay path pattern:** `replays/<match_id>.json.gz`

**Verification:** ⚠️ B2 accessible but credentials broken

---

### 5. Database (PostgreSQL)

**Status:** ✅ Running

**Connection:** `acb-postgres-869c59f86b-v9jmr` (2/2 ready)

**Expected tables:** matches, bots, ratings, jobs, series, seasons, etc.

**Verification:** ⚠️ Cannot query directly (readonly access prevents exec into pods)

---

### 6. Strategy Bots

**Status:** ✅ 4/6 Running

**Running Bots:**
- acb-strategy-hunter: ✅ Running (55d uptime)
- acb-strategy-random: ✅ Running (55d uptime)
- acb-strategy-rusher: ✅ Running (55d uptime)
- acb-strategy-swarm: ✅ Running (55d uptime)

**Pending Bots:**
- acb-strategy-gatherer: ❌ Pending (cannot schedule)
- acb-strategy-guardian: ❌ Pending (cannot schedule)

**Bot logs:** Empty - no incoming HTTP requests (workers not running to make requests)

**Verification:** ✅ Bots operational but receiving no traffic

---

## Match Pipeline Flow Analysis

### Expected Flow (Normal Operation)

```
1. Matchmaker (60s interval)
   └─> Polls active bots from database
   └─> Pairs bots by similar rating
   └─> Creates job records in database

2. Worker (continuous polling)
   └─> Polls Valkey for pending job IDs
   └─> Claims job via Valkey SETNX
   └─> Fetches job details from database
   └─> Calls strategy bot HTTP endpoints for each turn
   └─> Runs engine: spawn units, execute turns, determine winner
   └─> Uploads replay JSON to B2
   └─> Updates database with match result

3. Index Builder (30 min interval)
   └─> Fetches all matches, bots, ratings from database
   └─> Generates static JSON index files
   └─> Uploads to B2/Cloudflare Pages
```

### Actual Flow (Current State)

```
1. Matchmaker: ❌ BLOCKED
   └─> Pod pending (cannot schedule)
   └─> No jobs created in database

2. Worker: ❌ BLOCKED
   └─> Pods pending (cannot schedule)
   └─> No matches executed
   └─> No replays uploaded

3. Strategy Bots: ⚠️ Idle
   └─> Running but receiving 0 HTTP requests
   └─> No workers to call them

4. Index Builder: ❌ BLOCKED
   └─> Pod pending (cannot schedule)
   └─> No index updates
```

---

## Root Cause Analysis

### Primary Issue: Cluster Capacity

**Node 1 (prod-instance-17767388520094079):**
- Status: Ready
- Capacity: At limits (98% CPU, 94% memory allocated)
- Issue: Cannot schedule additional pods

**Node 2 (prod-instance-17825486055310528):**
- Status: NotReady (4h9m)
- Issue: Node is unhealthy - cannot schedule pods
- Likely causes: Kubelet crash, network issues, resource exhaustion

### Secondary Issue: R2 Credentials Corruption

Even if pods could schedule, the R2 credentials are corrupted:
- `endpoint` field contains a SHA256 hash
- `secret-key` field contains the actual endpoint URL
- Replay uploads would fail with "Custom endpoint was not a valid URI"

This is tracked separately in bf-2ws and documented in IAD-ACB-R2-CREDENTIALS-FIX.md.

---

## Verification Results Summary

| Component | Expected | Actual | Status |
|-----------|----------|--------|--------|
| Matchmaker pod running | ✅ | ❌ Pending | BLOCKED |
| Matchmaker creates jobs | ✅ | ❌ No pod | CANNOT VERIFY |
| Worker pods running | ✅ | ❌ Pending | BLOCKED |
| Workers claim jobs | ✅ | ❌ No pods | CANNOT VERIFY |
| Workers execute matches | ✅ | ❌ No pods | CANNOT VERIFY |
| Replays uploaded to B2 | ✅ | ❌ No workers + R2 broken | CANNOT VERIFY |
| Index builder runs | ✅ | ❌ Pending | BLOCKED |
| Static JSON updated | ✅ | ❌ No pod | CANNOT VERIFY |
| Strategy bots operational | ✅ | ⚠️ 4/6 running | PARTIAL |

---

## Access Constraints

During verification, the following access limitations were encountered:

| Cluster | Access | Limitations |
|---------|--------|-------------|
| iad-acb | Readonly observer | Cannot: delete pods, update deployments, exec into pods, port-forward (partial) |
| iad-ci | Cluster-admin | Can trigger CI rebuilds (not relevant for this verification) |

**Impact:** Could not query database directly or trigger manual pod restarts.

---

## Recommendations

### Immediate (Requires Cluster Admin Access)

1. **Fix Node 2 health:**
   - Investigate why `prod-instance-17825486055310528` is NotReady
   - Check kubelet logs: `journalctl -u kubelet`
   - Check node resource exhaustion
   - Consider node replacement if unrecoverable

2. **Free capacity on Node 1:**
   - Evict non-critical pods (acb-enrichment has been in ImagePullBackOff for 31 days)
   - Scale down non-essential replicas
   - Consider vertical pod autoscaling adjustments

3. **Fix R2 credentials (see IAD-ACB-R2-CREDENTIALS-FIX.md):**
   - Update OpenBao secret at `secret/rs-manager/ai-code-battle/r2`
   - Force ESO re-sync
   - Verify secret values in cluster

### Short-term (After Cluster Health)

1. **Deploy latest acb-index-builder image:**
   - Current deployed image: b35a2aa (first OOM fix only)
   - Latest code: 05512a5 (all OOM fixes)
   - Trigger CI rebuild on iad-ci

2. **Verify match pipeline end-to-end:**
   - Check matchmaker logs for job creation
   - Check worker logs for match execution
   - Check B2 for replay uploads
   - Check index builder logs for completion

### Long-term

1. **Cluster capacity planning:**
   - Monitor resource utilization trends
   - Consider node autoscaling
   - Add capacity if utilization consistently >80%

2. **Infrastructure monitoring:**
   - Set up alerts for node NotReady state
   - Alert on pod scheduling failures
   - Monitor pod restart counts (acb-map-evolver: 13167 restarts needs investigation)

---

## Conclusion

**The match pipeline is completely non-functional.**

- ❌ **Matchmaker**: Cannot schedule - no jobs being created
- ❌ **Workers**: Cannot schedule - no matches being executed
- ❌ **Index Builder**: Cannot schedule - no index updates
- ⚠️ **R2 Credentials**: Corrupted - would block replay uploads even if workers ran
- ✅ **Database**: Running but cannot query directly
- ⚠️ **Strategy Bots**: 4/6 running, receiving 0 requests

**Primary blocker:** Cluster infrastructure - one node NotReady, one node at capacity
**Secondary blocker:** R2 credentials corruption

**Verification incomplete:** Cannot verify match execution, replay uploads, or index building while critical components are unschedulable.

---

## Next Steps

This verification requires cluster admin intervention to:
1. Restore Node 2 to Ready state OR add capacity
2. Free resources on Node 1 to schedule pending pods
3. Fix R2 credentials
4. Redeploy latest images
5. Re-run verification

**No matches have been observed running** because the matchmaker and workers have been unable to schedule for at least 6 hours.

---

**Verified:** 2026-06-27 08:40 UTC
**Verified by:** bf-4dy (needle:claude-code-glm47-acb-1:bf-4dy:auto)
