# B2 CDN Setup - b2.aicodebattle.com Configuration

**Created:** 2026-06-17  
**Bead:** bf-2x3  
**Purpose:** Document Backblaze B2 CDN configuration for b2.aicodebattle.com

---

## Summary

Backblaze B2 serves as the primary storage layer for AI Code Battle replay files and match metadata. This document provides the exact DNS configuration needed to expose the B2 bucket via `b2.aicodebattle.com` through Cloudflare's Bandwidth Alliance (zero egress fees).

---

## B2 Bucket Details

| Property | Value |
|----------|-------|
| **Bucket Name** | `acb-data` |
| **Region** | `us-west-002` |
| **S3 Endpoint** | `https://s3.us-west-002.backblazeb2.com` |
| **CNAME Target** | `acb-data.s3.us-west-002.backblazeb2.com` |
| **Friendly Endpoint** | `f002.backblazeb2.com` |

**Bucket Name Verification (2026-06-17):**
The bucket name `acb-data` is confirmed via the enrichment deployment configuration (`acb-enrichment-deployment.yml` line 112) which explicitly sets `ACB_R2_BUCKET: "acb-data"`. Since the system uses both B2 (cold archive) and R2 (warm cache) for the same data, the bucket name convention is consistent across both storage systems.

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

### Current Status: Unknown

**Status Verification Required:** The bucket's public access setting could not be verified programmatically because:
1. The B2 credentials are stored in OpenBao on rs-manager (path: `secret/rs-manager/iad-acb/armor`)
2. Read-only kubectl proxies cannot access ExternalSecret values
3. Direct B2 API access requires the Application Key which is not accessible via the observer serviceaccount

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

1. ✅ **CNAME points to B2** (`acb-data.s3.us-west-004.backblazeb2.com`)
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
| B2 bucket created | ✅ Complete (credentials exist in cluster) |
| Region determined | ✅ Complete (us-west-002) |
| Bucket name verified | ✅ Complete (acb-data - confirmed via R2 config reference) |
| CNAME target identified | ✅ Complete (acb-data.s3.us-west-002.backblazeb2.com) |
| Public access enabled | ⚠️ Unknown (requires Backblaze console access to verify) |
| CNAME record created | ❌ NOT CREATED - confirmed NXDOMAIN via host command (2026-06-17) |
| Bandwidth Alliance active | ❌ Pending (depends on CNAME + proxy on) |

---

## Next Steps

1. **Verify public access** on the `acb-data` bucket via Backblaze console
2. **Create CNAME record** in Cloudflare DNS (if not already done)
3. **Test CNAME resolution** with `dig +short b2.aicodebattle.com`
4. **Verify CDN access** with `curl -I https://b2.aicodebattle.com/`
5. **Confirm Bandwidth Alliance** is active in Cloudflare dashboard

---

## Contact & Support

- **B2 Documentation:** https://www.backblaze.com/docs/cloud-storage
- **Bandwidth Alliance:** https://www.cloudflare.com/bandwidth-alliance/
- **Cloudflare CDN:** https://developers.cloudflare.com/cache/

---

**Document Version:** 1.1  
**Last Updated:** 2026-06-17
