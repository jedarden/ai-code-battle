# Bead bf-4ur: Secret Documentation and Templates Review

## Task Completion Summary

Reviewed secret documentation and existing templates for AI Code Battle on apexalgo-iad cluster.

## Credential Documentation Reviewed

### 1. R2_ACCESS_KEY_SOURCE.md

**Purpose:** Documents the R2 access credential source for the `acb-data` bucket.

**Credential Path:**
```
Cloudflare R2 Dashboard → OpenBao (rs-manager) → ESO → Kubernetes Secret → Application Pods
```

**OpenBao Secret Path:** `secret/rs-manager/ai-code-battle/r2`

**Expected Structure:**
```json
{
  "endpoint": "https://e26f015c7ba47a6ad6219385e77072b7.r2.cloudflarestorage.com",
  "bucket": "acb-data",
  "access-key": "<32-char R2 Access Key ID>",
  "secret-key": "<64-char R2 Secret Access Key>"
}
```

**Status:** CORRUPTED - values in OpenBao are swapped/corrupted (documented in IAD-ACB-R2-CREDENTIALS-FIX.md)

**Note:** This secret is for **iad-acb cluster**, not apexalgo-iad.

### 2. IAD-ACB-R2-CREDENTIALS-FIX.md

**Purpose:** Documents the corruption issue with `acb-r2-credentials` ExternalSecret on **iad-acb** cluster.

**Key Issue:**
- `endpoint` contains a SHA256 hash instead of URL
- `secret-key` contains the endpoint URL (swapped)
- `access-key` contains a hash instead of the R2 access key ID

**Fix Options:**
1. Fix OpenBao directly at `secret/rs-manager/ai-code-battle/r2`
2. Replace with SealedSecret (bypass ESO)
3. Run automated fix script

**Note:** This documentation is for iad-acb cluster. The apexalgo-iad cluster uses different secrets.

## Secret Templates in declarative-config (apexalgo-iad)

### 1. acb-armor-credentials (ExternalSecret)

**File:** `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-armor-credentials-externalsecret.yml`

**Type:** ExternalSecret (pulls from OpenBao via ESO)

**OpenBao Remote Path:** `rs-manager/iad-acb/armor` (note: no `secret/` prefix in the remoteRef)

**ClusterSecretStore:** `openbao` (defined in `declarative-config/k8s/apexalgo-iad/external-secrets/cluster-secret-store.yml`)

**Secret Keys:**
- `bucket` - ARMOR MinIO bucket name
- `auth-access-key` - MinIO access key
- `auth-secret-key` - MinIO secret key

**Used By:**
- `acb-index-builder-deployment.yml` - uses as ACB_B2_ENDPOINT (warm cache)

**Environment Variables (mapped from secret):**
- `ACB_B2_ENDPOINT` = `http://armor:9000` (static, not from secret)
- `ACB_B2_BUCKET` ← `bucket`
- `ACB_B2_ACCESS_KEY` ← `auth-access-key`
- `ACB_B2_SECRET_KEY` ← `auth-secret-key`

**Purpose:** ARMOR is an internal MinIO service providing S3-compatible storage for staging files before promotion to Cloudflare R2.

### 2. acb-cloudflare-api-token (Secret Template)

**File:** `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-cloudflare-api-token-secret.yml.template`

**Type:** Template for SealedSecret (needs to be sealed)

**Secret Keys:**
- `token` - Cloudflare API Token
- `account-id` - Cloudflare Account ID (32-char hex string)

**Required Token Permissions:**
- Account > Cloudflare Pages > Edit
- Account > Cloudflare R2 > Edit
- User > User Details > Read

**Used By:**
- `acb-index-builder-deployment.yml` - deploys static indexes to Cloudflare Pages

**Environment Variables (mapped from secret):**
- `ACB_CLOUDFLARE_API_TOKEN` ← `token`
- `ACB_CLOUDFLARE_ACCOUNT_ID` ← `account-id`

**Sealing Command:**
```bash
kubeseal --controller-name=sealed-secrets-apexalgo-iad \
  --controller-namespace=sealed-secrets \
  --server=http://traefik-apexalgo-iad:8001 \
  --format yaml < acb-cloudflare-api-token-secret.yml.template > acb-cloudflare-api-token-sealedsecret.yml
```

**Account ID:** Found in Cloudflare Dashboard URL when viewing Workers & Pages or R2 (e.g., `https://dash.cloudflare.com/<ACCOUNT_ID>/pages/view/...`)

## ESO Configuration (apexalgo-iad)

**ClusterSecretStore:** `openbao`

**File:** `declarative-config/k8s/apexalgo-iad/external-secrets/cluster-secret-store.yml`

**OpenBao Server:** `http://openbao.external-secrets.svc.cluster.local:8200`

**Vault Path:** `secret`

**Vault Version:** `v2`

**Auth Method:** Token authentication via `openbao-eso-token` secret in `external-secrets` namespace

## Summary Table

| Secret Name | Type | Source Path | Keys | Used By |
|-------------|------|-------------|------|---------|
| acb-armor-credentials | ExternalSecret | OpenBao remoteRef: `rs-manager/iad-acb/armor` | bucket, auth-access-key, auth-secret-key | index-builder |
| acb-cloudflare-api-token | SealedSecret (template) | Cloudflare Dashboard | token, account-id | index-builder |

## Credential Sources

| Secret | Credential Source | How to Obtain |
|--------|------------------|---------------|
| acb-armor-credentials | OpenBao (rs-manager cluster) | Already stored in OpenBao at path `rs-manager/iad-acb/armor` (ESO adds `secret/` prefix per ClusterSecretStore config) |
| acb-cloudflare-api-token | Cloudflare Dashboard | Create at https://dash.cloudflare.com/profile/api-tokens with Pages+R2 Edit permissions |

## Notes

1. **acb-r2-credentials** documented in R2_ACCESS_KEY_SOURCE.md is for iad-acb cluster, NOT apexalgo-iad
2. apexalgo-iad uses ARMOR (internal MinIO) as staging storage, not direct R2 access
3. The acb-cloudflare-api-token needs to be created and sealed before use - template exists but no sealed secret yet
4. The acb-armor-credentials ExternalSecret references OpenBao path `rs-manager/iad-acb/armor` - ESO's ClusterSecretStore has `path: secret` so the full path becomes `secret/rs-manager/iad-acb/armor`
5. The ExternalSecret for acb-armor-credentials exists but the corresponding OpenBao secret must exist at the correct path for ESO to sync it
