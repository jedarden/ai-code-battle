# Bug Fix bf-5nap: Match Creation Stopped - Investigation Report

## Summary

Matches stopped being created after 2026-05-09T13:29:34Z (1000 matches total, May 8-9). The iad-acb kubeconfig on ex44 has expired credentials, preventing access to the production cluster.

## Problem Analysis

### Timeline
- **Last successful match**: 2026-05-09T13:29:34Z
- **Total matches created**: 1000 (May 8-9)
- **Current date**: 2026-05-13
- **Duration of outage**: ~4 days

### Root Cause (Suspected)
The iad-acb Kubernetes cluster kubeconfig on ex44 has expired credentials. The server is asking for client credentials, indicating the authentication token has expired.

**Note**: This is a different issue from the previous R2 credentials corruption (documented in IAD-ACB-R2-CREDENTIALS-FIX.md and IAD-ACB-OPENBAO-FIX.md).

## Cluster Architecture

### iad-acb Cluster Components
1. **acb-matchmaker** (Deployment, 1 replica)
   - Computes pairings
   - Enqueues job IDs into Valkey
   - Health-checks bots
   - Reaps stale jobs
   - Image: `ronaldraygun/acb-matchmaker@sha256:1a322b94e32e6cd843abe3c2beb1478f2c4893ce5d963a8d2eeff92cfe7c0e06`

2. **acb-worker** (Deployment, 2 replicas)
   - BRPOPs jobs from Valkey
   - Runs matches
   - Uploads replays to B2 (armor)
   - Writes results and Glicko-2 ratings to PostgreSQL
   - Image: `ronaldraygun/acb-worker@sha256:edd9616aaefb684a59779ea4b46b2bfe72679eecf6867e1be658273648e86bbe`

### Dependencies
- PostgreSQL: `acb-postgres:5432`
- Valkey: `valkey:6379`
- Armor (B2): `armor:9000`

## Diagnostic Steps Required

### Step 1: Renew iad-acb Token from Rackspace Spot UI

The kubeconfig token needs to be renewed from the Rackspace Spot dashboard:

1. Log in to Rackspace Spot dashboard
2. Navigate to Kubernetes clusters
3. Locate the iad-acb cluster
4. Verify the cluster still exists (may have been terminated)
5. Generate/download new kubeconfig
6. Update `/home/coding/.kube/iad-acb.kubeconfig` on ex44

### Step 2: Verify Cluster Access

Once the kubeconfig is updated:

```bash
# On ex44 server
export KUBECONFIG=/home/coding/.kube/iad-acb.kubeconfig

# Test cluster access
kubectl cluster-info
kubectl get nodes

# Check namespace
kubectl get namespace ai-code-battle
```

### Step 3: Check Matchmaker Pod Status

```bash
# Check matchmaker deployment
kubectl get deployment acb-matchmaker -n ai-code-battle

# Check matchmaker pods
kubectl get pods -n ai-code-battle -l app.kubernetes.io/name=acb-matchmaker

# Check matchmaker logs
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-matchmaker --tail=100

# Check for crash loops
kubectl describe pod -n ai-code-battle -l app.kubernetes.io/name=acb-matchmaker
```

**Expected findings:**
- Pod may be in CrashLoopBackOff or Error state
- Logs may show authentication errors or database connection issues
- Pod may be stuck trying to connect to PostgreSQL or Valkey

### Step 4: Check Worker Pod Status

```bash
# Check worker deployment
kubectl get deployment acb-worker -n ai-code-battle

# Check worker pods
kubectl get pods -n ai-code-battle -l app.kubernetes.io/name=acb-worker

# Check worker logs
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-worker --tail=100

# Check for crash loops
kubectl describe pod -n ai-code-battle -l app.kubernetes.io/name=acb-worker
```

**Expected findings:**
- Workers may be idle (no jobs from matchmaker)
- May show R2/armor connection issues
- May show database connection errors

### Step 5: Check Dependencies

```bash
# Check PostgreSQL
kubectl get pods -n ai-code-battle -l app.kubernetes.io/name=acb-postgres
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-postgres --tail=50

# Check Valkey
kubectl get pods -n ai-code-battle -l app.kubernetes.io/name=valkey
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=valkey --tail=50

# Check Armor (B2 gateway)
kubectl get pods -n ai-code-battle -l app.kubernetes.io/name=armor
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=armor --tail=50
```

### Step 6: Check Database State

```bash
# Access PostgreSQL
kubectl exec -it -n ai-code-battle deployment/acb-postgres -- psql -U postgres -d ai_code_battle

# In psql, check:
-- Last match created
SELECT id, created_at FROM matches ORDER BY created_at DESC LIMIT 5;

-- Check for failed jobs
SELECT * FROM jobs WHERE status = 'failed' ORDER BY created_at DESC LIMIT 10;

-- Check for stuck jobs
SELECT * FROM jobs WHERE status = 'pending' ORDER BY created_at DESC LIMIT 10;

-- Check bot health
SELECT * FROM bots ORDER BY last_health_check DESC;
```

### Step 7: Restart Services (If Needed)

```bash
# Restart matchmaker
kubectl rollout restart deployment/acb-matchmaker -n ai-code-battle

# Restart workers
kubectl rollout restart deployment/acb-worker -n ai-code-battle

# Watch rollout status
kubectl rollout status deployment/acb-matchmaker -n ai-code-battle
kubectl rollout status deployment/acb-worker -n ai-code-battle
```

### Step 8: Verify Match Creation Resumes

```bash
# Watch matchmaker logs for activity
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-matchmaker -f

# In PostgreSQL, verify new matches are being created
# Run every 30 seconds:
SELECT id, created_at FROM matches ORDER BY created_at DESC LIMIT 1;

# Check worker activity
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-worker -f
```

## Potential Issues

### Issue 1: Cluster Terminated
**Symptoms**: `kubectl cluster-info` fails with connection refused
**Resolution**: Cluster may have been terminated in Rackspace Spot. Need to recreate cluster and restore from backups.

### Issue 2: Pod Image Pull Errors
**Symptoms**: Pods stuck in `ImagePullBackOff` state
**Resolution**: Check Docker Hub credentials, verify image tags exist, update `imagePullSecrets`

### Issue 3: Database Connection Failures
**Symptoms**: Logs show "connection refused" to PostgreSQL
**Resolution**: Check PostgreSQL pod is running, verify credentials in `acb-postgres-credentials` secret

### Issue 4: Valkey Connection Failures
**Symptoms**: Matchmaker can't enqueue jobs
**Resolution**: Check Valkey pod is running, verify network policies allow traffic

### Issue 5: R2/Armor Connection Failures
**Symptoms**: Workers can't upload replays
**Resolution**: Check R2 credentials (see IAD-ACB-R2-CREDENTIALS-FIX.md), verify armor pod is running

## Known Issues from Prior Incidents

1. **R2 Credentials Corruption** (IAD-ACB-R2-CREDENTIALS-FIX.md)
   - OpenBao secret at `secret/rs-manager/ai-code-battle/r2` has corrupted values
   - Endpoint and secret-key values are swapped
   - Fix: Run `/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh`

2. **Orphaned openbao Namespace** (IAD-ACB-OPENBAO-FIX.md)
   - Status: RESOLVED
   - Was causing DNS conflicts for ESO
   - Namespace has been deleted

## Verification Checklist

After fixing the issue, verify:

- [ ] iad-acb cluster is accessible via kubectl
- [ ] Matchmaker pod is running and healthy
- [ ] Worker pods are running and healthy
- [ ] PostgreSQL is accepting connections
- [ ] Valkey is accepting connections
- [ ] Armor (B2 gateway) is accessible
- [ ] New matches are being created in the database
- [ ] Workers are processing matches and uploading replays
- [ ] No errors in matchmaker or worker logs
- [ ] Index builder can successfully run and upload to R2

## Monitoring Setup

To prevent future outages, consider:

1. **Set up alerts** for:
   - Matchmaker pod down
   - Worker pods down
   - No matches created in 1 hour
   - Failed jobs exceeding threshold

2. **Regular health checks**:
   - `kubectl get pods -n ai-code-battle`
   - Monitor database for stuck jobs
   - Check R2 upload success rate

3. **Token renewal reminders**:
   - Rackspace Spot kubeconfig tokens expire
   - Set calendar reminder for renewal 30 days before expiration

## Files Modified

- Created: `/home/coding/ai-code-battle/notes/bf-5nap.md` (this file)

## Next Steps

1. Access Rackspace Spot UI and renew iad-acb kubeconfig token
2. Update kubeconfig on ex44 at `/home/coding/.kube/iad-acb.kubeconfig`
3. Follow diagnostic steps above to identify why match creation stopped
4. Restart services as needed
5. Verify match creation resumes
6. Close bead with retrospective
