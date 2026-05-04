# iad-acb R2 Credentials Fix

## Problem

The `acb-r2-credentials` ExternalSecret on iad-acb is syncing values from OpenBao, but the stored values are **corrupted/swapped**:

| Secret Key | Current Value | Expected Value |
|------------|---------------|----------------|
| `endpoint` | `bdaf818e893d8691d2ff24bf1c120d34458a00be8d12b5b74037f930b20cabcd` | `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com` |
| `bucket` | `acb-data` | `acb-data` ✓ |
| `access-key` | `66aabf3cc401c74755910422a903a8af` | (R2 Access Key ID - 32 chars) |
| `secret-key` | `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com` | (R2 Secret Access Key - 64 chars) |

## Root Cause

The values stored in OpenBao at `secret/rs-manager/ai-code-battle/r2` are corrupted:
- The `endpoint` property contains a SHA256 hash
- The `secret-key` property contains the actual endpoint URL
- The `access-key` property contains what looks like a hash instead of the R2 access key ID

This is **not an ESO sync issue** - ESO is correctly syncing whatever values are stored in OpenBao.

## Impact

All R2 operations fail with "Custom endpoint was not a valid URI":
- Replay uploads to R2 fail (index-builder, worker)
- Thumbnail uploads to R2 fail
- Bot card uploads to R2 fail
- Website replay viewer cannot load real matches

## Fix Options

### Option 1: Fix the OpenBao Secret (Recommended)

1. Access OpenBao on rs-manager
2. Update the secret at `secret/rs-manager/ai-code-battle/r2` with correct values:
   ```bash
   # Via OpenBao UI or CLI
   vault kv put secret/rs-manager/ai-code-battle/r2 \
     endpoint="https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com" \
     bucket="acb-data" \
     access-key="<R2_ACCESS_KEY_ID>" \
     secret-key="<R2_SECRET_ACCESS_KEY>"
   ```
3. Force ESO to re-sync:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig annotate externalsecret acb-r2-credentials -n ai-code-battle force-sync=$(date +%s)
   ```

### Option 2: Replace with SealedSecret (Bypass ESO)

1. Generate R2 API credentials in Cloudflare dashboard (R2 > API Tokens)
2. Create SealedSecret with correct values:
   ```bash
   kubectl create secret generic acb-r2-credentials -n ai-code-battle \
     --from-literal=endpoint="https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com" \
     --from-literal=bucket="acb-data" \
     --from-literal=access-key="<R2_ACCESS_KEY_ID>" \
     --from-literal=secret-key="<R2_SECRET_ACCESS_KEY>" \
     --dry-run=client -o yaml | \
   kubeseal --controller-name=sealed-secrets -n ai-code-battle
   ```
3. Remove ExternalSecret from declarative-config
4. Commit SealedSecret to declarative-config

### Option 3: Fix Script (Automated Option 1)

Run `/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh` with:
- OpenBao root token OR
- R2 credentials (will update OpenBao directly)

## Required R2 Credentials

To fix this, you need:
1. **R2 Access Key ID** (32 characters, starts with digits like `1234567890abcdef...`)
2. **R2 Secret Access Key** (64 characters, base64-like)

Get these from Cloudflare Dashboard:
1. Go to: R2 > acb-data > Settings > R2 API
2. Click "Create API Token" or use existing token
3. Copy Access Key ID and Secret Access Key

## Verification

After fix, verify:
```bash
# Check secret values
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig get secret acb-r2-credentials -n ai-code-battle -o json | jq -r '.data | map_values(@base64d)'

# Check index-builder pod can start
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig get pods -n ai-code-battle -l app.kubernetes.io/name=acb-index-builder

# Check logs for R2 errors
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig logs -n ai-code-battle -l app.kubernetes.io/name=acb-index-builder --tail=50
```

## Files Modified

- `/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh` - Fix script (to be created)
- `/home/coding/ai-code-battle/IAD-ACB-R2-CREDENTIALS-FIX.md` - This document
