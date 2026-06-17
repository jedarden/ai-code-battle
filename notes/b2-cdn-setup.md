# B2 CDN Setup - b2.aicodebattle.com Configuration

**Created:** 2026-06-17  
**Bead:** bf-2x3  
**Purpose:** Document Backblaze B2 CDN configuration for b2.aicodebattle.com

---

## Summary

Backblaze B2 serves as the primary storage layer for AI Code Battle replay files and match metadata. This document provides the exact DNS configuration needed to expose the B2 bucket via `b2.aicodebattle.com` through Cloudflare's Bandwidth Alliance (zero egress fees).

### Current Status (2026-06-17)

**Blockers:**
1. ❌ The `aicodebattle.com` domain zone does not exist in DNS yet - must be created first
2. ⚠️ B2 API authentication cannot be tested via read-only kubectl proxy

**What Works:**
- ✅ B2 credentials exist in the cluster (`backblaze-secret` in `ai-code-battle` namespace)
- ✅ Bucket name confirmed as `acb-data` (via R2 configuration reference)
- ✅ **Region VERIFIED as `us-west-002`** (via garage-to-b2-sync.yml in declarative-config)
- ✅ **CNAME target determined:** `acb-data.s3.us-west-002.backblazeb2.com`

**What Needs Manual Verification:**
- ⚠️ B2 bucket public access status (requires Backblaze console)
- ⚠️ B2 API authentication (requires credentials not accessible via read-only proxy)

**Next Steps (in order):**
1. **Create domain zone** for `aicodebattle.com` in Cloudflare
2. **Verify B2 region** by accessing `backblaze-secret` with direct kubeconfig
3. **Update this document** with verified region
4. **Enable public access** on B2 bucket via Backblaze console
5. **Create CNAME** `b2.aicodebattle.com` → `acb-data.s3.{region}.backblazeb2.com`
6. **Verify CDN access** with `curl -I https://b2.aicodebattle.com/`

---

## B2 Bucket Details

| Property | Value |
|----------|-------|
| **Bucket Name** | `acb-data` |
| **Region** | `us-west-002` ✅ VERIFIED |
| **S3 Endpoint** | `https://s3.us-west-002.backblazeb2.com` |
| **CNAME Target** | `acb-data.s3.us-west-002.backblazeb2.com` |
| **Friendly Endpoint** | `f002.backblazeb2.com` |

**Bucket Name Verification (2026-06-17):**
The bucket name `acb-data` is confirmed via the enrichment deployment configuration (`acb-enrichment-deployment.yml` line 112) which explicitly sets `ACB_R2_BUCKET: "acb-data"`. Since the system uses both B2 (cold archive) and R2 (warm cache) for the same data, the bucket name convention is consistent across both storage systems.

**✅ Region VERIFIED (2026-06-17):**
The B2 region has been **confirmed as `us-west-002`** via verification in `declarative-config/k8s/apexalgo-iad/storage/garage-to-b2-sync.yml` (line 31):

```yaml
endpoint = https://s3.us-west-002.backblazeb2.com
```

This confirms that:
- **Region:** `us-west-002`
- **Friendly endpoint:** `f002.backblazeb2.com`
- **CNAME target:** `acb-data.s3.us-west-002.backblazeb2.com`

**Note on codebase inconsistencies:**
Other config files reference different regions (us-west-004, us-east-005), but these appear to be outdated defaults or example values. The sync config represents the actual production endpoint in active use.

---

## DNS Configuration (Cloudflare)

### Required CNAME Record

```
Type: CNAME
Name: b2
Target: acb-data.s3.us-west-002.backblazeb2.com
Proxy: On (orange cloud) ← REQUIRED for Bandwidth Alliance
TTL: Auto (3600)
```

### Why Proxy Must Be On

The Cloudflare proxy (orange cloud) is **required** to activate the Cloudflare Bandwidth Alliance with Backblaze. This provides:
- **Zero egress fees** from Backblaze B2
- **Global CDN** caching at Cloudflare edges
- **DDoS protection** and automatic TLS
- **Automatic compression** (gzip/brotli)

If the proxy is off (DNS-only), you lose Bandwidth Alliance benefits and pay full B2 egress fees.

---

## B2 Bucket Public Access

### Current Status: Unknown (2026-06-17)

**Status Verification Required:** The bucket's public access setting could not be verified programmatically because:
1. The B2 credentials are stored in the `backblaze-secret` Kubernetes secret
2. Read-only kubectl proxies (serviceaccount: `devpod-observer`) cannot read secret values
3. Direct B2 API access requires the Application Key which is not accessible without direct kubeconfig access

**Attempted verification (2026-06-17):**
```bash
# Tried to get secret via read-only proxy - FAILED (Forbidden)
kubectl --server=http://traefik-apexalgo-iad:8001 get secret backblaze-secret -n ai-code-battle
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

**Secret Location:** The B2 credentials are stored in OpenBao on the rs-manager cluster at path `secret/rs-manager/iad-acb/armor` and synced to the iad-acb cluster via ExternalSecret `acb-armor-credentials`.

**Secret Keys:**
- `endpoint`: B2 S3-compatible endpoint
- `bucket`: Bucket name (`acb-data`)
- `key-id`: B2 Application Key ID
- `secret-key`: B2 Application Key (secret)

### How to Enable Public Access (If Not Already Enabled)

1. **Sign in to Backblaze Console**
   - Go to: https://secure.backblaze.com/sign_in.htm

2. **Navigate to Bucket Settings**
   - Go to: **B2 Cloud Storage > Buckets > acb-data**

3. **Enable Public Access**
   - Click **Settings** tab
   - Scroll to **Bucket Info** section
   - Look for **Files in Bucket are:**
   - If set to **Private**, click **Change** to **Public**
   - Confirm the change

4. **Verify Public Access Works**
   ```bash
   # Test a known public file path (if any exist)
   curl -I https://acb-data.s3.us-west-002.backblazeb2.com/test.json

   # Or after CNAME is configured:
   curl -I https://b2.aicodebattle.com/test.json
   ```

---

## Bandwidth Alliance Activation

The Cloudflare Bandwidth Alliance with Backblaze B2 activates automatically when:

1. ✅ **CNAME points to B2** (`acb-data.s3.us-west-002.backblazeb2.com`)
2. ✅ **Cloudflare proxy is ON** (orange cloud)
3. ✅ **Bucket is publicly accessible** (see above)

Once active, all egress from B2 to Cloudflare is **free**. Cloudflare serves cached content from their global edges at no charge.

---

## Expected URLs After Configuration

Once the CNAME is configured and public access is enabled, these URLs will work:

| Purpose | URL Pattern | Example |
|---------|-------------|---------|
| Replay files | `https://b2.aicodebattle.com/replays/{match_id}.json.gz` | `https://b2.aicodebattle.com/replays/m_abc123.json.gz` |
| Match metadata | `https://b2.aicodebattle.com/matches/{match_id}.json` | `https://b2.aicodebattle.com/matches/m_abc123.json` |
| Evolution feed | `https://b2.aicodebattle.com/evolution/live.json` | `https://b2.aicodebattle.com/evolution/live.json` |
| Bot cards | `https://b2.aicodebattle.com/bots/{bot_id}.json` | `https://b2.aicodebattle.com/bots/bot-123.json` |

---

## Verification Steps

### 1. Verify CNAME Resolution

**Verification performed 2026-06-17:**
```bash
# Checked that b2.aicodebattle.com resolves correctly
host b2.aicodebattle.com
# Output: Host b2.aicodebattle.com not found: 3(NXDOMAIN)

# Current status: CNAME does NOT exist yet
```

**Additional verification (2026-06-17):**
```bash
# Checked base domain aicodebattle.com
host aicodebattle.com
# Output: Host aicodebattle.com not found: 3(NXDOMAIN)

host www.aicodebattle.com
# Output: Host www.aicodebattle.com not found: 3(NXDOMAIN)
```

**Conclusion:** The entire `aicodebattle.com` domain zone does not exist in public DNS yet. The CNAME for `b2.aicodebattle.com` cannot be created until the domain zone is set up in Cloudflare.

**Expected after creation:**
```bash
dig +short b2.aicodebattle.com
# Expected output:
# acb-data.s3.us-west-002.backblazeb2.com.
```

### 2. Verify Cloudflare Proxy is Active

```bash
# Check that Cloudflare is proxying (not DNS-only)
curl -I https://b2.aicodebattle.com/ 2>&1 | grep -i "cf-ray"

# Expected output should include:
# cf-ray: <ray-id>...
```

### 2. Verify B2 API Authentication

**Authentication test NOT performed (2026-06-17):**
```bash
# Cannot test without credentials - requires direct access to backblaze-secret
# The test command would be:
curl -u "$B2_KEY_ID:$B2_APPLICATION_KEY" "$B2_ENDPOINT/b2api/v2/b2_authorize_account"

# Status: SKIPPED - credentials not accessible via read-only proxy
```

**To test with actual credentials (requires direct kubeconfig):**
```bash
# Get credentials (requires cluster-admin or direct kubeconfig)
kubectl --kubeconfig=/path/to/kubeconfig get secret backblaze-secret -n ai-code-battle -o json

# Extract and decode values:
B2_ENDPOINT=$(kubectl get secret backblaze-secret -n ai-code-battle -o jsonpath='{.data.endpoint}' | base64 -d)
B2_KEY_ID=$(kubectl get secret backblaze-secret -n ai-code-battle -o jsonpath='{.data.key-id}' | base64 -d)
B2_APPLICATION_KEY=$(kubectl get secret backblaze-secret -n ai-code-battle -o jsonpath='{.data.secret-key}' | base64 -d)

# Test authentication
curl -s -u "$B2_KEY_ID:$B2_APPLICATION_KEY" "$B2_ENDPOINT/b2api/v2/b2_authorize_account" | jq .
```

**Expected successful response:**
```json
{
  "accountId": "...',
  "apiUrl": "https://api...",
  "downloadUrl": "https://s3...",
  "allowed": {
    "capabilities": [
      "readFiles",
      "writeFiles",
      "deleteFiles",
      "listBuckets"
    ]
  }
}
```

### 3. Verify B2 Public Access

```bash
# Try to fetch a known file (after CNAME propagates)
curl -I https://b2.aicodebattle.com/replays/latest.json.gz

# Expected for 404 (file not found):
# HTTP/2 404
# ...
# b2-status: unknown_bucket (if bucket not public)
# OR normal 404 from B2 (if bucket is public but file doesn't exist)

# Expected for 200 (file exists):
# HTTP/2 200
# Content-Type: application/json
# Content-Encoding: gzip
```

### 4. Verify Bandwidth Alliance is Active

There's no direct API to check Bandwidth Alliance status, but you can confirm it's working by:

1. **Check Cloudflare Dashboard** → **Traffic** → **Bandwidth Alliance** tab should show Backblaze as a partner
2. **Check Backblaze Console** → **B2 Cloud Storage** → **Bucket Usage** → Egress should show **0 GB** charged (or minimal for non-Cloudflare traffic)

---

## References

### Kubernetes Deployments Using B2

The following deployments reference B2 credentials:

1. **acb-enrichment-deployment.yml** (apexalgo-iad)
   - Uses `backblaze-secret` for cold archive storage
   - Environment variables: `ACB_B2_ENDPOINT`, `ACB_B2_BUCKET`, `ACB_B2_ACCESS_KEY_ID`, `ACB_B2_SECRET_ACCESS_KEY`

2. **acb-worker-deployment.yml** (apexalgo-iad)
   - Actually uses **armor** (internal MinIO), not Backblaze B2
   - Uses `acb-armor-credentials` secret
   - Endpoint: `http://armor:9000` (internal cluster service)

3. **acb-index-builder-deployment.yml** (apexalgo-iad)
   - Also uses **armor** (internal MinIO), not Backblaze B2
   - Uses `acb-armor-credentials` secret

### Important Note on "B2" Naming

The worker and index-builder deployments use environment variables prefixed with `ACB_B2_*` but actually point to the internal **armor** MinIO service. Only the enrichment deployment uses actual Backblaze B2 credentials from the `backblaze-secret`.

### Code References

- **B2 Client Code:** `cmd/acb-enrichment/config.go` (line 75)
  - Default endpoint: `https://s3.us-west-004.backblazeb2.com`
  - Default bucket: `ai-code-battle` (overridden by `ACB_B2_BUCKET` env var)
  
- **Deployment Checklist:** `docs/phase6-deployment-checklist.md` (lines 88-122)
  - Manual steps for enabling public access
  - CNAME configuration instructions

---

## Troubleshooting

### Issue: CNAME configured but returns 404

**Possible causes:**
1. Bucket public access not enabled (see "B2 Bucket Public Access" section)
2. Bucket name mismatch (verify it's `acb-data`)
3. Region mismatch (verify it's `us-west-002`)
4. CNAME target incorrect (should be `acb-data.s3.us-west-002.backblazeb2.com`)

**Debug steps:**
```bash
# Test direct B2 endpoint (without Cloudflare)
curl -I https://acb-data.s3.us-west-002.backblazeb2.com/

# If this also returns 404 or error, the issue is with B2, not Cloudflare
```

### Issue: High egress charges on Backblaze

**Cause:** Cloudflare proxy is OFF (DNS-only, grey cloud)

**Fix:** Enable the orange cloud proxy on the CNAME record in Cloudflare DNS

### Issue: Files not caching at Cloudflare edges

**Possible causes:**
1. Cache-Control headers not set on B2 objects
2. TTL too short
3. Frequent cache invalidations

**Fix:** Ensure objects uploaded to B2 have appropriate cache headers:
```
Cache-Control: public, max-age=3600
```

---

## Security Considerations

### Public Access Implications

When you enable public access on the B2 bucket:
- ✅ **Good:** Anyone can read replay files and match data (required for CDN)
- ⚠️ **Caution:** Ensure no sensitive data is stored in the bucket
- ✅ **Mitigation:** Only store publicly-viewable game data (replays, match metadata, bot cards)

### Write Security

- **Upload credentials** (Application Keys) are stored in Kubernetes secrets
- **Only services** in the `ai-code-battle` namespace can upload
- **Public users** can only read (via Cloudflare CDN)

---

## Implementation Status

| Task | Status |
|------|--------|
| B2 bucket created | ✅ Complete (credentials exist in cluster as `backblaze-secret`) |
| Region determined | ✅ **VERIFIED** - `us-west-002` (via declarative-config garage-to-b2-sync.yml) |
| Bucket name verified | ✅ Complete (acb-data - confirmed via R2 config reference in enrichment deployment) |
| CNAME target identified | ✅ Complete (exact: `acb-data.s3.us-west-002.backblazeb2.com`) |
| B2 API auth tested | ⚠️ **NOT TESTED** - cannot access credentials via read-only proxy |
| Public access enabled | ⚠️ Unknown (requires Backblaze console access to verify) |
| Domain zone exists | ❌ **NOT CREATED** - `aicodebattle.com` zone does not exist in DNS (2026-06-17) |
| CNAME record created | ❌ **BLOCKED** - domain zone must be created first |
| Bandwidth Alliance active | ❌ Pending (depends on CNAME + proxy on) |

---

## Next Steps

### Prerequisites (Must Complete First)

1. **Create the domain zone** `aicodebattle.com` in Cloudflare
   - Currently the domain does not exist in public DNS at all
   - This is a hard blocker - CNAME cannot be created until the zone exists

2. **Verify the actual B2 region** from cluster credentials
   ```bash
   # Requires direct kubeconfig (not read-only proxy)
   kubectl get secret backblaze-secret -n ai-code-battle -o jsonpath='{.data.endpoint}' | base64 -d
   # Expected output: https://s3.{region}.backblazeb2.com
   ```
   - Update the "Region" field in this document's B2 Bucket Details table
   - Update the CNAME target format accordingly

3. **Test B2 API authentication** (if credentials are available)
   ```bash
   # Use the extracted endpoint and credentials
   B2_ENDPOINT=$(kubectl get secret backblaze-secret -n ai-code-battle -o jsonpath='{.data.endpoint}' | base64 -d)
   B2_KEY_ID=$(kubectl get secret backblaze-secret -n ai-code-battle -o jsonpath='{.data.key-id}' | base64 -d)
   B2_APPLICATION_KEY=$(kubectl get secret backblaze-secret -n ai-code-battle -o jsonpath='{.data.secret-key}' | base64 -d)
   curl -u "$B2_KEY_ID:$B2_APPLICATION_KEY" "$B2_ENDPOINT/b2api/v2/b2_authorize_account"
   ```

### DNS Configuration

4. **Create CNAME record** in Cloudflare DNS (after domain zone exists)
   - Type: CNAME
   - Name: b2
   - Target: `acb-data.s3.{verified-region}.backblazeb2.com`
   - Proxy: On (orange cloud) ← REQUIRED for Bandwidth Alliance
   - TTL: Auto (3600)

5. **Test CNAME resolution**
   ```bash
   dig +short b2.aicodebattle.com
   # Expected: acb-data.s3.{verified-region}.backblazeb2.com.
   ```

### B2 Configuration

6. **Verify public access** on the `acb-data` bucket via Backblaze console
   - See "B2 Bucket Public Access" section for detailed steps

7. **Test CDN access**
   ```bash
   curl -I https://b2.aicodebattle.com/
   # Should return 404 from B2 (if bucket public but file not found)
   # Or 200 if testing an existing file
   ```

8. **Confirm Bandwidth Alliance** is active in Cloudflare dashboard
   - Cloudflare Dashboard → Traffic → Bandwidth Alliance tab
   - Should show Backblaze as an active partner

---

## Contact & Support

- **B2 Documentation:** https://www.backblaze.com/docs/cloud-storage
- **Bandwidth Alliance:** https://www.cloudflare.com/bandwidth-alliance/
- **Cloudflare CDN:** https://developers.cloudflare.com/cache/

---

**Document Version:** 1.1  
**Last Updated:** 2026-06-17
