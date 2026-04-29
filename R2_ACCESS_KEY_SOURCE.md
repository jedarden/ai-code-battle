# R2 Access Key Source for acb-data Bucket

## Summary

The R2 access credentials for the `acb-data` bucket follow this path:

```
Cloudflare R2 Dashboard (user creates)
         ↓
OpenBao (rs-manager cluster) ← AUTHORTIATIVE SOURCE
         ↓
External Secrets Operator (ESO)
         ↓
Kubernetes Secret (acb-r2-credentials)
         ↓
Application Pods (index-builder, worker, evolver)
```

## Canonical Source

**OpenBao Secret Path:** `secret/rs-manager/ai-code-battle/r2`

**Cluster:** rs-manager (Rackspace Spot, us-east-iad-1)

**Expected Structure:**
```json
{
  "endpoint": "https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com",
  "bucket": "acb-data",
  "access-key": "<32-char R2 Access Key ID>",
  "secret-key": "<64-char R2 Secret Access Key>"
}
```

**R2 Account ID:** `e26f015c7ba47a6ad6219385e77072b7`

## Current Status: CORRUPTED

The values in OpenBao are corrupted/swapped:

| OpenBao Property | Current Value | Expected Value |
|-----------------|---------------|----------------|
| `endpoint` | `bdaf818e893d8691d2ff24bf1c120d34458a00be8d12b5b74037f930b20cabcd` (SHA256 hash) | `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com` |
| `bucket` | `acb-data` | `acb-data` ✓ |
| `access-key` | `66aabf3cc401c74755910422a903a8af` (hash) | `<32-char R2 Access Key ID>` |
| `secret-key` | `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com` (swapped!) | `<64-char R2 Secret Access Key>` |

## ESO Configuration

**ExternalSecret:** `acb-r2-credentials` (namespace: `ai-code-battle`)

**ClusterSecretStore:** `openbao` (on iad-acb cluster)

**Store Config:** `/home/coding/declarative-config/k8s/iad-acb/external-secrets/cluster-secret-store.yml`
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
```

ESO is correctly syncing - the problem is upstream in OpenBao.

## Where to Get Valid Credentials

**Cloudflare Dashboard Path:**
1. R2 > acb-data > Settings > R2 API
2. Click "Create API Token" or use existing token
3. Copy Access Key ID (32 chars) and Secret Access Key (64 chars)

**R2 Endpoint Format:**
```
https://<account-id>.r2.cloudflarestorage.com
```

For this project: `https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com`

## Impact of Corruption

All R2 operations fail with "Custom endpoint was not a valid URI":
- Replay uploads to R2 fail (index-builder, worker)
- Thumbnail uploads to R2 fail
- Bot card uploads to R2 fail
- Website replay viewer cannot load real matches

## Fix Options

### Option 1: Fix OpenBao Directly (Recommended)
```bash
vault login <root-token>
vault kv put secret/rs-manager/ai-code-battle/r2 \
  endpoint="https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com" \
  bucket="acb-data" \
  access-key="<R2_ACCESS_KEY_ID>" \
  secret-key="<R2_SECRET_ACCESS_KEY>"

kubectl --kubeconfig=/home/coding/.kube/iad-acb.kubeconfig annotate \
  externalsecret acb-r2-credentials -n ai-code-battle force-sync=$(date +%s)
```

### Option 2: Replace with SealedSecret (Bypass ESO)
```bash
kubeseal --controller-name=sealed-secrets -n ai-code-battle \
  > /home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-r2-credentials-sealedsecret.yml
```

Then remove the ExternalSecret from declarative-config.

### Option 3: Automated Script
```bash
/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh
```

## Related Files

- `/home/coding/ai-code-battle/IAD-ACB-R2-CREDENTIALS-FIX.md` - R2-specific fix documentation
- `/home/coding/ai-code-battle/IAD-ACB-OPENBAO-FIX.md` - Comprehensive OpenBao fix documentation
- `/home/coding/ai-code-battle/fix-iad-acb-r2-credentials.sh` - Automated fix script
- `/home/coding/ai-code-battle/manifests/acb-index-builder-deployment.yml` - Deployment using the secret
- `/home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-externalsecrets.yml` - ExternalSecret definitions

## Environment Variables Used

Applications use these environment variables (populated from `acb-r2-credentials` secret):
- `ACB_R2_ENDPOINT` - R2 endpoint URL
- `ACB_R2_BUCKET` - Bucket name (`acb-data`)
- `ACB_R2_ACCESS_KEY` - R2 Access Key ID (from secret's `access-key`)
- `ACB_R2_SECRET_KEY` - R2 Secret Access Key (from secret's `secret-key`)
