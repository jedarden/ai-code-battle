# Phase 6: Deployment & Production - Completion Checklist

## Status: Code Complete, Infrastructure Setup Pending Cloudflare Access

This document outlines the remaining steps to complete Phase 6. All code is written and tested. The remaining tasks require Cloudflare account access to create resources.

---

## Completed (Code & K8s)

### ✅ Container Images
- [x] `acb-matchmaker` - Match scheduling, health checks, reaper
- [x] `acb-worker` - Match execution, B2 upload
- [x] `acb-index-builder` - PostgreSQL → JSON → Pages deploy
- [x] `acb-evolver` - LLM evolution pipeline
- [x] `acb-strategy-random` - Python RandomBot
- [x] `acb-strategy-gatherer` - Go GathererBot
- [x] `acb-strategy-rusher` - Rust RusherBot
- [x] `acb-strategy-guardian` - PHP GuardianBot
- [x] `acb-strategy-swarm` - TypeScript SwarmBot
- [x] `acb-strategy-hunter` - Java HunterBot

### ✅ Kubernetes Deployment
All K8s manifests are in the `ardenone-cluster` repo at:
`declarative-config/k8s/apexalgo-iad/ai-code-battle/`

- [x] Namespace configuration
- [x] PostgreSQL schema (ext-postgres-operator)
- [x] Deployments for all services
- [x] Services for internal communication
- [x] SealedSecrets for credentials
- [x] ArgoCD Application manifest

### ✅ CI/CD
- [x] GitHub Actions workflow (`.github/workflows/ci.yml`)
- [x] Go tests for engine and cmd packages
- [x] Web build with Vite
- [x] Build artifact upload

### ✅ Monitoring & Alerting
- [x] Health endpoints (`/health`, `/ready`)
- [x] Prometheus metrics (`/metrics`)
- [x] Discord/Slack alerting webhooks
- [x] Liveness and readiness probes configured

### ✅ Deployment Scripts
All scripts in `scripts/` directory are ready:
- [x] `cloudflare-setup.sh` - Full Cloudflare setup
- [x] `setup-b2.sh` - B2 bucket configuration (obsolete — B2 credentials already in SealedSecrets)
- [x] `deploy-pages.sh` - Deploy SPA to Pages
- [x] `configure-dns.sh` - DNS configuration
- [x] `verify-deployment.sh` - End-to-end verification

---

## Remaining (Requires Cloudflare Account Access)

### ⏳ Cloudflare Pages Setup

**Automated via script:**
```bash
./scripts/cloudflare-setup.sh
```

**Or manual steps:**
1. Create Pages project:
   - Go to Workers & Pages > Create application > Pages > Upload assets
   - Project name: `aicodebattle`
   - Or use wrangler:
     ```bash
     wrangler pages project create aicodebattle --production-branch master
     ```

2. Deploy the SPA:
   ```bash
   cd web
   npm install
   npm run build
   cd ..
   wrangler pages deploy web/dist --project-name=aicodebattle
   ```

3. Add custom domain:
   - Go to: Workers & Pages > aicodebattle > Settings > Custom domains
   - Add domain: `aicodebattle.com`
   - DNS CNAME will be auto-configured

### ⏳ Backblaze B2 Custom Domain

B2 credentials are already provisioned (SealedSecret in cluster). The remaining step is to
expose the bucket under a `b2.aicodebattle.com` subdomain so the SPA can fetch replays
via Cloudflare's Bandwidth Alliance (zero egress fees).

**Manual steps:**
1. In Backblaze console, enable public access on the `acb-data` bucket.
2. Note the native B2 endpoint: `{bucket}.s3.{region}.backblazeb2.com`
3. Add a Cloudflare DNS CNAME (see DNS section below) — Cloudflare proxies the request,
   activating the Bandwidth Alliance and serving files via CDN.

No script required — no Cloudflare account credentials needed for this step (DNS-only change).

### ⏳ DNS Configuration

**Automated via script:**
```bash
export CLOUDFLARE_API_TOKEN=your_token
export TRAEFIK_IP=$(kubectl --server=http://kubectl-apexalgo-iad:8001 get svc -n traefik traefik -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
./scripts/configure-dns.sh
```

**Or manual steps:**
1. Main domain (Pages):
   - Type: CNAME
   - Name: `@` (or `aicodebattle.com`)
   - Target: `aicodebattle.pages.dev`
   - Proxy: On (orange cloud)

2. B2 subdomain (Bandwidth Alliance):
   - Type: CNAME
   - Name: `b2`
   - Target: `acb-data.s3.<region>.backblazeb2.com`  ← replace `<region>` with actual B2 region
   - Proxy: On (orange cloud) — required to activate Cloudflare Bandwidth Alliance (zero egress)

3. API subdomain (deferred — not needed for v1 static-first launch):
   - Type: A
   - Name: `api`
   - Target: `<Traefik LoadBalancer IP>`
   - Proxy: On (orange cloud)

**Get Traefik IP:**
```bash
kubectl --server=http://kubectl-apexalgo-iad:8001 get svc -n traefik
```

---

## Verification

After completing the setup, run the verification script:

```bash
./scripts/verify-deployment.sh
```

Or manually check:
```bash
# SPA should be accessible
curl -I https://aicodebattle.com

# B2 CDN should be accessible (a known replay file)
curl -I https://b2.aicodebattle.com/replays/latest.json.gz

# API health (deferred — not required for v1)
# curl https://api.aicodebattle.com/health
```

---

## Expected URLs After Deployment

| Service | URL |
|---------|-----|
| SPA (Pages) | `https://aicodebattle.com` |
| SPA (Pages default) | `https://aicodebattle.pages.dev` |
| Replays (B2 via CDN) | `https://b2.aicodebattle.com/replays/{match_id}.json.gz` |
| Match metadata (B2 via CDN) | `https://b2.aicodebattle.com/matches/{match_id}.json` |
| Evolution feed (B2 via CDN) | `https://b2.aicodebattle.com/evolution/live.json` |
| API (K8s, deferred) | `https://api.aicodebattle.com/health` |

---

## Data Flow

```
┌──────────────────────────────────────────────────────────────────────┐
│                           Public Internet                             │
│  (No K8s services exposed here — cluster is write-only compute)      │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌──────────────────────┐    ┌────────────────────────────────────┐  │
│  │  Cloudflare Pages    │    │  Backblaze B2 (via Cloudflare CDN) │  │
│  │  aicodebattle.com    │    │  b2.aicodebattle.com               │  │
│  │                      │    │                                    │  │
│  │  SPA shell (HTML/    │    │  replays/*.json.gz                 │  │
│  │  JS/CSS)             │    │  matches/*.json                    │  │
│  │  data/*.json         │    │  evolution/live.json               │  │
│  │                      │    │  (Bandwidth Alliance = free egress) │  │
│  └──────────────────────┘    └────────────────────────────────────┘  │
│           ▲                               ▲                          │
└───────────┼───────────────────────────────┼──────────────────────────┘
     writes (wrangler)               writes (S3-compatible API)
            │                               │
┌───────────┼───────────────────────────────┼──────────────────────────┐
│           │    apexalgo-iad cluster        │                          │
│           │    (compute only — no         │                          │
│           │     inbound user traffic)     │                          │
│           │                               │                          │
│  ┌────────▼───────────────────────────────┼────────────────────────┐ │
│  │  Index Builder Deployment              │                        │ │
│  │  - Reads PostgreSQL                    │                        │ │
│  │  - Generates JSON indexes              │                        │ │
│  │  - Deploys data/*.json to Pages (wrangler pages deploy)        │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Match Workers (Deployment)                                  │   │
│  │  - Execute matches (battles happen here)                     │   │
│  │  - Build replay JSON                                         │   │
│  │  - Upload replays to B2                                      │   │
│  │  - Write results to PostgreSQL                               │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Matchmaker Deployment                                       │   │
│  │  - Creates match jobs                                        │   │
│  │  - Enqueues to Valkey                                        │   │
│  │  - Health checks bots                                        │   │
│  │  - Reaps stale jobs                                          │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Evolver Deployment                                          │   │
│  │  - LLM evolution pipeline                                    │   │
│  │  - Writes evolution/live.json to B2                          │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Strategy Bot Deployments (x6)                               │   │
│  │  - HTTP servers on cluster-internal Services only            │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  PostgreSQL (cnpg-apexalgo)                                  │   │
│  │  - Bots, matches, jobs, ratings, etc.                        │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Valkey StatefulSet                                          │   │
│  │  - Job queue (acb:jobs:pending)                              │   │
│  └──────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Post-Deployment Tasks

Once Cloudflare resources are created:

1. **Update environment variables in index builder:**
   - `CLOUDFLARE_API_TOKEN` - For Pages deployment
   - `B2_KEY_ID`, `B2_APPLICATION_KEY`, `B2_BUCKET`, `B2_ENDPOINT` - For B2 operations (workers and evolver)

2. **Deploy to Kubernetes:**
   - K8s manifests are already in `ardenone-cluster` repo
   - ArgoCD will sync them automatically

3. **Verify data flow:**
   - Index builder should start deploying JSON indexes to Pages
   - Match workers should upload replay files to B2
   - Replays should be accessible at `b2.aicodebattle.com` via Cloudflare CDN

4. **Monitor:**
   - Check ArgoCD for sync status
   - Check pod logs for any errors
   - Run `./scripts/verify-deployment.sh`

---

## Exit Criteria

Phase 6 is complete when:

- [x] All container images built and pushed
- [x] All K8s manifests committed to ardenone-cluster repo
- [x] CI/CD pipeline working
- [x] Monitoring and alerting configured
- [ ] Cloudflare Pages project created and deployed
- [ ] B2 bucket public access enabled and `b2.aicodebattle.com` CNAME added (Bandwidth Alliance)
- [ ] DNS configured (aicodebattle.com, b2.aicodebattle.com)
- [ ] Platform publicly accessible

The Pages and DNS items require Cloudflare account access. The B2 item requires Backblaze console access. The `api.aicodebattle.com` DNS entry is deferred — the Go API is not required for v1.
