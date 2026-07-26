# Phase 6: Deployment & Production - Completion Checklist

> ⚠️ **Superseded (2026-07-26).** This checklist predates two changes and is retained for history:
> 1. The compute tier it describes (apexalgo-iad K8s) was **decommissioned 2026-07-21**
>    (`declarative-config` commit `0163324e`). See `notes/bf-1yj.md` to revive.
> 2. The storage/serving model moved off the Backblaze B2 CDN (Bandwidth-Alliance CNAME
>    `b2.aicodebattle.com`) to **R2 served via Cloudflare Pages Functions at `/r2/*`**, and the
>    canonical public domain is `https://ai-code-battle.pages.dev` (the `aicodebattle.com` zone was
>    never registered). The B2/CDN/DNS steps below are therefore obsolete; see
>    `docs/notes/canonical-public-domain.md` for the current URL patterns.

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

3. (Optional, future) Add custom domain — none is configured; the apex domain is not registered:
   - Go to: Workers & Pages > ai-code-battle > Settings > Custom domains
   - Add your registered domain
   - DNS CNAME will be auto-configured

### ⏳ Storage Serving (R2 via Pages Function) — supersedes the B2 CDN

The SPA no longer fetches replays from a CDN subdomain. Replay/match/thumbnail/card data is
served through the R2 bucket (`acb-data`) via the Cloudflare Pages Function at `/r2/*`
(`web/functions/r2/[[path]].ts`), reachable at `https://ai-code-battle.pages.dev/r2/*`. No
separate custom domain or Bandwidth-Alliance CNAME is used.

Historical note (obsolete): an earlier plan exposed the bucket under a `b2.aicodebattle.com`
subdomain via Cloudflare's Bandwidth Alliance (zero egress). That subdomain was never created
(the apex domain is not registered) and the architecture moved to the Pages Function proxy.

### ⏳ DNS Configuration

**No DNS records are required** for the current deployment. The canonical domain
`https://ai-code-battle.pages.dev` is managed by Cloudflare Pages, and storage is served via the
`/r2/*` Pages Function on that same host — there is no separate R2/B2 subdomain. The `aicodebattle.com`
zone was never registered, so no records were ever created.

> **If `aicodebattle.com` is registered later** (future consideration only), these are the records you
> would add in Cloudflare DNS:
> 1. Main domain (Pages): CNAME `@` → `ai-code-battle.pages.dev` (proxied, auto-configured when
>    adding the domain as a Pages custom domain).
> 2. No B2/R2 subdomain is required — storage is fronted by the `/r2/*` Pages Function.
> 3. API subdomain: only if the Go API is re-exposed publicly (it was decommissioned with the
>    apexalgo-iad cluster on 2026-07-21; see `notes/bf-1yj.md`). Not needed for v1.

---

## Verification

After completing the setup, run the verification script:

```bash
./scripts/verify-deployment.sh
```

Or manually check:
```bash
# SPA should be accessible (canonical domain)
curl -I https://ai-code-battle.pages.dev

# Replays served via the /r2/* Pages Function (a known replay file)
curl -I https://ai-code-battle.pages.dev/r2/replays/latest.json.gz

# API health — N/A for v1. The Go API lived on the now-decommissioned apexalgo-iad cluster
# (2026-07-21); the api.* subdomain was never registered. See notes/bf-1yj.md to revive.
```

---

## Expected URLs After Deployment

| Service | URL |
|---------|-----|
| SPA (Pages, canonical) | `https://ai-code-battle.pages.dev` |
| Replays (R2 via Pages Function) | `https://ai-code-battle.pages.dev/r2/replays/{match_id}.json.gz` |
| Match metadata (R2 via Pages Function) | `https://ai-code-battle.pages.dev/r2/matches/{match_id}.json` |
| Evolution feed (R2 via Pages Function) | `https://ai-code-battle.pages.dev/r2/evolution/live.json` |
| API (K8s) | decommissioned 2026-07-21 — see `notes/bf-1yj.md` to revive |

---

## Data Flow

```
┌──────────────────────────────────────────────────────────────────────┐
│                           Public Internet                             │
│  (No K8s services exposed here — cluster is write-only compute)      │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌──────────────────────┐    ┌────────────────────────────────────┐  │
│  │  Cloudflare Pages    │    │  Cloudflare R2 (via /r2/* Function) │  │
│  │  *.pages.dev         │    │  /r2/* (bucket: acb-data)           │  │
│  │                      │    │                                    │  │
│  │  SPA shell (HTML/    │    │  replays/*.json.gz                 │  │
│  │  JS/CSS)             │    │  matches/*.json                    │  │
│  │  data/*.json         │    │  evolution/live.json               │  │
│  │                      │    │  (R2 = zero egress)                │  │
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
   - Replays should be accessible at `https://ai-code-battle.pages.dev/r2/*` via the Pages Function

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
- [ ] R2 bucket (`acb-data`) bound to the Pages project and served via `/r2/*` Function
- [ ] No DNS configuration required (canonical domain `ai-code-battle.pages.dev` is Pages-managed)
- [ ] Platform publicly accessible at `https://ai-code-battle.pages.dev`

The Pages item requires Cloudflare account access. No custom DNS or B2 CDN setup is needed. The Go
API (formerly on the apexalgo-iad cluster) was decommissioned 2026-07-21 and is not required for v1 —
see `notes/bf-1yj.md` to revive.
