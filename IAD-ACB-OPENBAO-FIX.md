# iad-acb Cluster Secret Issues - Comprehensive Fix

## Summary

Two separate issues affecting iad-acb cluster secrets:

1. **Orphaned openbao namespace** (RESOLVED) - Was causing DNS conflicts for ESO
2. **Corrupted R2 credentials in OpenBao** (ACTIVE) - R2 operations failing

---

## Issue 1: Orphaned openbao Namespace (RESOLVED)

### Problem
An orphaned `openbao` namespace existed on iad-acb containing a sealed local OpenBao deployment. This caused DNS conflicts where ESO would sometimes resolve to the local service (HTTP 503) instead of the correct Tailscale egress proxy.

### Status
**RESOLVED** - The orphaned namespace has been deleted.

### Verification
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig get namespace openbao
# Output: Error from server (NotFound) - namespace is gone
```

---

## Issue 2: Corrupted R2 Credentials in OpenBao (ACTIVE)

### Problem
The `acb-r2-credentials` ExternalSecret on iad-acb is syncing corrupted values from OpenBao.

**Current Secret Values (corrupted):**
| Secret Key | Current Value | Expected Value |
|------------|---------------|----------------|
| `endpoint` | `bdaf818e893d8691d2ff24bf1c120d34458a00be8d12b5b74037f930b20cabcd` | `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com` |
| `bucket` | `acb-data` | `acb-data` ✓ |
| `access-key` | `66aabf3cc401c74755910422a903a8af` | (R2 Access Key ID - 32 chars) |
| `secret-key` | `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com` | (R2 Secret Access Key - 64 chars) |

**Note:** The values are swapped - the endpoint URL is stored in the `secret-key` field!

### Impact

All R2 operations fail with "Custom endpoint was not a valid URI":
- Replay uploads to R2 fail (index-builder, worker)
- Thumbnail uploads to R2 fail
- Bot card uploads to R2 fail
- Website replay viewer cannot load real matches

**Evidence from index-builder logs:**
```
"error":"upload to R2: upload object data/meta/archetypes.json: operation error S3: PutObject, resolve auth scheme: resolve endpoint: endpoint rule error, Custom endpoint `bdaf818e893d8691d2ff24bf1c120d34458a00be8d12b5b74037f930b20cabcd` was not a valid URI"
```

### Root Cause
The values stored in OpenBao at `secret/rs-manager/ai-code-battle/r2` are corrupted. This is **not an ESO sync issue** - ESO is correctly syncing whatever values are stored in OpenBao.

---

## Fix Options

### Option 1: Fix the OpenBao Secret (Recommended)

1. Access OpenBao on rs-manager cluster
2. Update the secret at `secret/rs-manager/ai-code-battle/r2` with correct values:

```bash
# Via OpenBao CLI
vault login <root-token>
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

1. Generate R2 API credentials in Cloudflare dashboard (R2 > acb-data > Settings > R2 API)
2. Create SealedSecret with correct values:

```bash
kubectl create secret generic acb-r2-credentials -n ai-code-battle \
  --from-literal=endpoint="https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com" \
  --from-literal=bucket="acb-data" \
  --from-literal=access-key="<R2_ACCESS_KEY_ID>" \
  --from-literal=secret-key="<R2_SECRET_ACCESS_KEY>" \
  --dry-run=client -o yaml | \
kubeseal --controller-name=sealed-secrets -n ai-code-battle \
  > /home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-r2-credentials-sealedsecret.yml
```

3. Remove the ExternalSecret from declarative-config:
```bash
# Remove from /home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-externalsecrets.yml
# Delete the acb-r2-credentials ExternalSecret section
```

4. Delete the ExternalSecret from the cluster:
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig delete externalsecret acb-r2-credentials -n ai-code-battle
```

### Option 3: Automated Fix Script

Run the provided fix script:
```bash
/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh
```

The script supports:
- Updating OpenBao directly (with OpenBao root token)
- Creating a SealedSecret (bypasses OpenBao)

---

## Required R2 Credentials

To fix this, you need:

1. **R2 Access Key ID** (32 characters, starts with digits)
2. **R2 Secret Access Key** (64 characters)

**Get these from Cloudflare Dashboard:**
1. Go to: R2 > acb-data > Settings > R2 API
2. Click "Create API Token" or use existing token
3. Copy Access Key ID and Secret Access Key

---

## Verification

After applying the fix, verify:

```bash
# Check secret values
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig get secret acb-r2-credentials -n ai-code-battle -o json | jq -r '.data | map_values(@base64d)'

# Expected output:
# {
#   "access-key": "<32-char access key>",
#   "bucket": "acb-data",
#   "endpoint": "https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com",
#   "secret-key": "<64-char secret key>"
# }

# Check index-builder logs for R2 errors (should be gone)
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig logs -n ai-code-battle -l app.kubernetes.io/name=acb-index-builder --tail=50 | grep -i r2

# Check pod is healthy
kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig get pods -n ai-code-battle -l app.kubernetes.io/name=acb-index-builder
```

---

## ClusterSecretStore Configuration

The ClusterSecretStore in `/home/coding/declarative-config/k8s/iad-acb/external-secrets/cluster-secret-store.yml` is correctly configured:

```yaml
spec:
  provider:
    vault:
      server: "http://openbao.external-secrets.svc.cluster.local:8200"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "k8s-iad-acb"
          role: "eso"
          serviceAccountRef:
            name: external-secrets-iad-acb
            namespace: external-secrets
```

**Status:** Ready and validated

---

## Files

- `/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh` - Automated fix script
- `/home/coding/ai-code-battle/IAD-ACB-R2-CREDENTIALS-FIX.md` - R2-specific fix documentation
- `/home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-externalsecrets.yml` - ExternalSecret definitions
