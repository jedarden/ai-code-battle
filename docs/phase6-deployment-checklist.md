# Phase 6: Deployment & Production - Completion Checklist

## Status: Code Complete, Infrastructure Setup Pending Cloudflare Access

This document outlines the remaining steps to complete Phase 6. All code is written and tested. The remaining tasks require Cloudflare account access to create resources.

---

## Completed (Code & K8s)

### ✅ Container Images
- [x] `acb-matchmaker` - Match scheduling, health checks, reaper
- [x] `acb-worker` - Match execution, B2 upload
- [x] `acb-index-builder` - PostgreSQL → JSON → Pages deploy, R2 management
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
- [x] `setup-r2.sh` - R2 bucket + custom domain
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

### ⏳ Cloudflare R2 Setup

**Automated via script:**
```bash
export CLOUDFLARE_API_TOKEN=your_token
export CLOUDFLARE_ACCOUNT_ID=your_account_id  # optional, auto-detected
./scripts/setup-r2.sh
```

**Or manual steps:**
1. Create R2 bucket:
   ```bash
   wrangler r2 bucket create acb-data
   ```

2. Add custom domain:
   - Go to: R2 > acb-data > Settings > Custom Domains
   - Add domain: `r2.aicodebattle.com`
   - DNS CNAME will be auto-configured

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

2. R2 subdomain:
   - Type: CNAME
   - Name: `r2`
   - Target: `acb-data.r2.cloudflarestorage.com`
   - Proxy: Off (gray cloud) - DNS only

3. API subdomain:
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

# R2 should be accessible
curl -I https://r2.aicodebattle.com

# API health (once K8s is running)
curl https://api.aicodebattle.com/health
```

---

## Expected URLs After Deployment

| Service | URL |
|---------|-----|
| SPA (Pages) | `https://aicodebattle.com` |
| SPA (Pages default) | `https://aicodebattle.pages.dev` |
| Replays (R2) | `https://r2.aicodebattle.com/replays/{match_id}.json.gz` |
| Match metadata (R2) | `https://r2.aicodebattle.com/matches/{match_id}.json` |
| Evolution feed (R2) | `https://r2.aicodebattle.com/evolution/live.json` |
| API (K8s) | `https://api.aicodebattle.com/health` |

---

## Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         Public Internet                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────┐    ┌─────────────────────────────────┐ │
│  │  Cloudflare Pages   │    │  Cloudflare R2                  │ │
│  │  aicodebattle.com    │    │  r2.aicodebattle.com            │ │
│  │                     │    │                                 │ │
│  │  SPA shell (HTML/   │    │  replays/*.json.gz              │ │
│  │  JS/CSS)            │    │  matches/*.json                 │ │
│  │  data/*.json        │    │  evolution/live.json            │ │
│  │                     │    │                                 │ │
│  └─────────────────────┘    └─────────────────────────────────┘ │
│           ▲                            ▲                        │
└───────────┼────────────────────────────┼────────────────────────┘
            │                            │
┌───────────┼────────────────────────────┼────────────────────────┐
│           │  apexalgo-iad cluster       │                        │
│           │                            │                        │
│  ┌────────▼─────────────────────────────┼────────────────────┐   │
│  │  Index Builder Deployment           │                    │   │
│  │  - Reads PostgreSQL                 │                    │   │
│  │  - Generates JSON indexes           │                    │   │
│  │  - Deploys to Pages (wrangler)      │                    │   │
│  │  - Promotes replays to R2           │                    │   │
│  │  - Prunes R2 warm cache             │                    │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Match Workers (Deployment)                                │  │
│  │  - Execute matches                                          │  │
│  │  - Upload replays to B2                                     │  │
│  │  - Write results to PostgreSQL                             │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Matchmaker Deployment                                      │  │
│  │  - Creates match jobs                                       │  │
│  │  - Enqueues to Valkey                                       │  │
│  │  - Health checks bots                                       │  │
│  │  - Reaps stale jobs                                         │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Evolver Deployment                                         │  │
│  │  - LLM evolution pipeline                                   │  │
│  │  - Writes evolution/live.json to R2                         │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Strategy Bot Deployments (x6)                              │  │
│  │  - HTTP servers on cluster-internal Services                │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  PostgreSQL (cnpg-apexalgo)                                 │  │
│  │  - Bots, matches, jobs, ratings, etc.                       │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Valkey StatefulSet                                          │  │
│  │  - Job queue (acb:jobs:pending)                              │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Backblaze B2 (cold archive)                                 │  │
│  │  - ALL replays, permanently                                  │  │
│  └─────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
```

---

## Post-Deployment Tasks

Once Cloudflare resources are created:

1. **Update environment variables in index builder:**
   - `CLOUDFLARE_API_TOKEN` - For Pages deployment
   - `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET`, `R2_ENDPOINT` - For R2 operations
   - `B2_KEY_ID`, `B2_APPLICATION_KEY`, `B2_BUCKET`, `B2_ENDPOINT` - For B2 operations

2. **Deploy to Kubernetes:**
   - K8s manifests are already in `ardenone-cluster` repo
   - ArgoCD will sync them automatically

3. **Verify data flow:**
   - Index builder should start deploying to Pages
   - Match workers should upload replays to B2
   - R2 warm cache should populate with recent replays

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
- [ ] Cloudflare R2 bucket created with custom domain
- [ ] DNS configured (aicodebattle.com, r2.aicodebattle.com, api.aicodebattle.com)
- [ ] Platform publicly accessible

The final 3 items require Cloudflare account access and must be completed by someone with admin access to the Cloudflare account.
