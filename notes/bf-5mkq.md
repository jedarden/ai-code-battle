# Bug Fix bf-5mkq: Enrichment Pipeline Not Running - Investigation Report

## Summary

All 1000 matches in production have `enriched: false`. The acb-enrichment service should process completed matches and set `enriched: true` with AI commentary, but it's not working.

## Problem Analysis

### Root Cause
The enrichment pipeline is not functioning due to **corrupted R2 credentials** in OpenBao, which prevents the acb-enrichment service from uploading AI commentary to R2.

### Evidence
1. **Match index shows all matches unenriched** - The `data/matches/index.json` file has `enriched: false` for all matches
2. **R2 credentials are corrupted** - According to `IAD-ACB-R2-CREDENTIALS-FIX.md`:
   - The `endpoint` property contains a SHA256 hash instead of the R2 endpoint URL
   - The `secret-key` property contains the actual endpoint URL instead of the secret key
   - The `access-key` property contains a hash instead of the R2 access key ID

### How Enrichment Works

1. **acb-enrichment service** (Deployment) runs on a 30-minute cycle
2. **Selector** finds completed matches without commentary (`commentary_json IS NULL`)
3. **Generator** downloads replays from B2, generates AI commentary via LLM
4. **Storage client** uploads commentary to R2 at `commentary/{match_id}.json`
5. **Index builder** checks R2 for commentary files and sets `enriched: true` in match index

### Why It's Failing

The acb-enrichment service cannot upload commentary to R2 because:
1. Service tries to use R2 credentials from `cloudflare-pages-secret` Secret
2. This Secret is synced from OpenBao via ExternalSecret
3. The OpenBao values at `secret/rs-manager/ai-code-battle/r2` are corrupted
4. Upload fails with authentication/endpoint errors
5. No commentary files are created in R2
6. Index builder sees no commentary files, sets `enriched: false` for all matches

## Diagnostic Steps

### Step 1: Check acb-enrichment Deployment Status

```bash
# Requires valid kubeconfig at /home/coding/.kube/iad-acb.kubeconfig
export KUBECONFIG=/home/coding/.kube/iad-acb.kubeconfig

# Check deployment
kubectl get deployment acb-enrichment -n ai-code-battle

# Check pods
kubectl get pods -n ai-code-battle -l app.kubernetes.io/name=acb-enrichment

# Check logs
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-enrichment --tail=100
```

**Expected findings:**
- Pod may be running but failing to upload to R2
- Logs may show "Custom endpoint was not a valid URI" or authentication errors
- Service may be skipping matches due to storage check failures

### Step 2: Verify R2 Credentials

```bash
# Check secret values
kubectl get secret acb-r2-credentials -n ai-code-battle -o json | jq -r '.data | map_values(@base64d)'

# Check enrichment service's secret (cloudflare-pages-secret)
kubectl get secret cloudflare-pages-secret -n ai-code-battle -o json | jq -r '.data | map_values(@base64d)'
```

**Expected findings:**
- Values will be corrupted (see IAD-ACB-R2-CREDENTIALS-FIX.md for details)
- `endpoint` will be a hash instead of `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com`
- `secret-key` will be the endpoint URL instead of the actual secret key

### Step 3: Check R2 for Commentary Files

```bash
# Check if any commentary files exist
curl -s "https://r2.aicodebattle.com/commentary/" | head -20

# Try to fetch a specific commentary file
curl -I "https://r2.aicodebattle.com/commentary/m_XXXXXX.json"
```

**Expected findings:**
- No commentary files exist in R2
- Directory may not exist yet

## Fix Required

### Option 1: Fix OpenBao Secret (Recommended)

Follow the steps in `IAD-ACB-R2-CREDENTIALS-FIX.md`:

1. Access OpenBao on rs-manager
2. Update the secret at `secret/rs-manager/ai-code-battle/r2` with correct values
3. Force ESO to re-sync:
   ```bash
   kubectl annotate externalsecret acb-r2-credentials -n ai-code-battle force-sync=$(date +%s)
   ```

### Option 2: Fix Enrichment Service Secret Directly

The enrichment service uses `cloudflare-pages-secret` for R2 credentials. This can be fixed directly:

```bash
# Get correct R2 credentials from Cloudflare Dashboard
# R2 > acb-data > Settings > R2 API

# Update the secret
kubectl create secret generic cloudflare-pages-secret -n ai-code-battle \
  --from-literal=r2-endpoint="https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com" \
  --from-literal=r2-bucket="acb-data" \
  --from-literal=r2-access-key="<R2_ACCESS_KEY_ID>" \
  --from-literal=r2-secret-key="<R2_SECRET_ACCESS_KEY>" \
  --dry-run=client -o yaml | \
  kubectl apply -f -

# Restart enrichment service to pick up new credentials
kubectl rollout restart deployment/acb-enrichment -n ai-code-battle
```

### Option 3: Run Fix Script

```bash
/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh
```

## Post-Fix Verification

### 1. Verify R2 Credentials

```bash
kubectl get secret cloudflare-pages-secret -n ai-code-battle -o json | jq -r '.data | map_values(@base64d)'
```

Expected values:
- `r2-endpoint`: `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com`
- `r2-bucket`: `acb-data`
- `r2-access-key`: 32-character access key ID
- `r2-secret-key`: 64-character secret access key

### 2. Verify Enrichment Service

```bash
# Check pod is running
kubectl get pods -n ai-code-battle -l app.kubernetes.io/name=acb-enrichment

# Check logs for successful enrichment
kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-enrichment --tail=50

# Look for:
# - "Enriched replay" messages
# - "commentary/{match_id}.json" upload confirmations
# - No R2 authentication errors
```

### 3. Verify Commentary Files in R2

```bash
# After next enrichment cycle (30 minutes)
curl -s "https://r2.aicodebattle.com/commentary/index.json"

# Should show entries like:
# {
#   "updated_at": "2026-05-13T...",
#   "entries": [
#     {"match_id": "m_XXXXXX", "criteria": ["upset_250", "back_and_forth"]}
#   ]
# }
```

### 4. Verify Match Index Updates

```bash
# Check data/matches/index.json for enriched: true
curl -s "https://aicodebattle.com/data/matches/index.json" | jq '.matches[] | select(.enriched == true)'

# After index builder runs (every 5 minutes), some matches should show enriched: true
```

### 5. Test Enrichment Endpoint

```bash
# Test the manual enrichment request endpoint
curl -X POST "https://api.aicodebattle.com/api/request-enrichment" \
  -H "Content-Type: application/json" \
  -d '{"match_id":"m_XXXXXX","shared_secret":"<bot_secret>"}'

# Should return:
# {
#   "status": "pending",
#   "request_id": "req_XXXXXX",
#   "match_id": "m_XXXXXX",
#   "estimated_wait_s": 300
# }
```

## Expected Timeline

1. **Immediate** (after fix):
   - Enrichment service can connect to R2
   - Commentary files start being uploaded

2. **After 30 minutes** (next enrichment cycle):
   - First batch of matches enriched (up to 20/hour)
   - Commentary files appear in R2

3. **After 35 minutes** (next index builder cycle):
   - Match index updated with `enriched: true` for enriched matches
   - Frontend shows "AI Commentary Available" badge

4. **After several hours**:
   - Historical matches gradually enriched (up to 20/hour)
   - Newest completed matches enriched first

## Configuration

### Enrichment Service Settings

From `manifests/acb-enrichment-deployment.yml`:
- **Cycle interval**: 30 minutes
- **Rate limit**: 20 enrichments per hour
- **Max concurrent**: 3 enrichment requests
- **Min turns**: 100 (matches must have 100+ turns)
- **Min crossings**: 3 (win probability must cross 0.5 three times)
- **Upset threshold**: 150 rating points
- **LLM model**: gpt-4o-mini
- **Storage**: R2 (preferred), B2 (fallback)

### Enrichment Criteria

Matches are selected for enrichment based on:
1. **Back-and-forth**: Win prob crosses 0.5 at least 3 times
2. **Upset**: Lower-rated bot wins by >150 rating points
3. **Close finish**: Final score difference ≤2
4. **High interest score**: Composite score ≥5.0
5. **Evolution milestone**: Evolved bot's first top-10 appearance

## Related Issues

1. **R2 Credentials Corruption** (IAD-ACB-R2-CREDENTIALS-FIX.md)
   - Status: KNOWN, requires fix
   - Impact: All R2 operations fail

2. **Expired Kubeconfig** (notes/bf-5nap.md)
   - Status: KNOWN, requires renewal
   - Impact: Cannot access cluster to diagnose

## Files Modified

- Created: `/home/coding/ai-code-battle/notes/bf-5mkq.md` (this file)

## Current Status (2026-05-13)

### Blocker
**Expired iad-acb kubeconfig** (see `notes/bf-5nap.md`) prevents access to the production cluster. Without cluster access, we cannot:
- Run the fix script (`fix-iad-acb-r2-credentials.sh`)
- Update OpenBao secrets
- Restart the enrichment service
- Verify the fix

### Environment Verification
- **Local machine**: No kubeconfig at `~/.kube/iad-acb.kubeconfig`
- **API endpoint**: `api.aicodebattle.com` not reachable from local environment
- **Fix script**: Exists at `/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh`
- **Fix documentation**: Complete in `IAD-ACB-R2-CREDENTIALS-FIX.md`

### Action Plan (when cluster access is restored)

1. **Restore cluster access** (prerequisite):
   ```bash
   # On ex44 server
   export KUBECONFIG=/home/coding/.kube/iad-acb.kubeconfig
   kubectl cluster-info  # Verify access
   ```

2. **Fix R2 credentials** (choose one):
   - **Option A - Run fix script**: `/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh`
   - **Option B - Manual OpenBao update**: See `IAD-ACB-R2-CREDENTIALS-FIX.md`
   - **Option C - Create SealedSecret**: Bypass ESO with SealedSecret

3. **Restart enrichment service**:
   ```bash
   kubectl rollout restart deployment/acb-enrichment -n ai-code-battle
   ```

4. **Verify enrichment resumes**:
   - Check logs: `kubectl logs -n ai-code-battle -l app.kubernetes.io/name=acb-enrichment`
   - Monitor R2 for new commentary files
   - Verify `enriched: true` appears in match index

### Expected Timeline After Fix
- **Immediate**: Service can connect to R2
- **30 minutes**: First enrichment cycle runs, up to 20 matches enriched
- **35 minutes**: Index builder updates match index with `enriched: true`
- **Hours**: Historical matches gradually enriched (20/hour rate limit)

## Next Steps

**This bead is blocked by expired kubeconfig**. Complete `bf-5nap` first to restore cluster access, then:
1. Fix R2 credentials using the fix script
2. Restart acb-enrichment deployment
3. Monitor logs for successful enrichments
4. Verify commentary files appear in R2
5. Confirm match index updates with `enriched: true`
6. Close bead with retrospective

## Prevention

To prevent future enrichment pipeline failures:
1. **Monitor R2 credentials health** - Alert when uploads fail
2. **Track enrichment rate** - Alert if <10 enrichments/hour for 2+ hours
3. **Verify commentary directory** - Check R2 for new files every hour
4. **Test enrichment endpoint** - Periodic health check of `/api/request-enrichment`
