# AI Code Battle - Deployment Guide

> ⚠️ **Status (2026-07-26): DECOMMISSIONED.** The compute tier described here
> ran on the `apexalgo-iad` Kubernetes cluster and was taken down on 2026-07-21
> (`declarative-config` commit `0163324e`, "take down ai-code-battle"). Both
> manifest trees (`k8s/apexalgo-iad/ai-code-battle/` and the retired
> `k8s/iad-acb/ai-code-battle/`) have been removed from `declarative-config`;
> the live `ai-code-battle` namespace on apexalgo-iad holds only orphaned pods
> the unhealthy ArgoCD sync did not prune. This guide documents the original
> deployment design. `apexalgo-iad` was the authoritative cluster; `iad-acb`
> was a separate, earlier dedicated-cluster attempt that was itself retired and
> deleted (`declarative-config` commit `53fc54a1`). To revive, see
> `notes/bf-1yj.md`.

This document describes how to deploy AI Code Battle to production.

## Architecture Overview

The platform is split across two tiers:

1. **Cloudflare (free tier)** - Web-facing infrastructure
   - Pages: SPA shell + pre-computed JSON index files
   - R2: Replays, match metadata, maps, thumbnails (served via Pages Functions at /r2/*)

2. **Kubernetes (apexalgo-iad)** - Compute tier
   - Matchmaker: Pairs bots, creates jobs in PostgreSQL
   - Match workers: Execute matches, upload replays to B2
   - Bot containers: Run strategy bot HTTP servers
   - Index builder: Generates JSON indexes, uploads to B2/Pages
   - PostgreSQL: Bots, matches, ratings, job queue
   - Traefik: Ingress for the Go API server (internal; the api subdomain was never publicly registered)

## Prerequisites

- Cloudflare account with:
  - Pages project created (aicodebattle)
  - R2 bucket bound to the Pages project (served via /r2/* Functions — no custom domain)
- Kubernetes cluster with:
  - PostgreSQL database
  - Traefik ingress
- Docker and docker-compose installed (for local development)

## Environment Setup

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and fill in your values:
   - `ACB_DATABASE_URL`: PostgreSQL connection URL
   - `ACB_R2_*`: B2/R2 credentials for replay storage
   - `BOT_SECRET_*`: Generate unique secrets for each bot

## Deploying Strategy Bots

The strategy bots run as HTTP servers that the match workers call during games.

```bash
# Build and start all 6 strategy bots
docker-compose -f docker-compose.bots.yml up -d

# Check status
docker-compose -f docker-compose.bots.yml ps

# View logs
docker-compose -f docker-compose.bots.yml logs -f
```

Bot endpoints will be available at:
- RandomBot: http://localhost:8081/turn
- GathererBot: http://localhost:8082/turn
- RusherBot: http://localhost:8083/turn
- GuardianBot: http://localhost:8084/turn
- SwarmBot: http://localhost:8085/turn
- HunterBot: http://localhost:8086/turn

## Deploying Match Workers

Match workers poll the Worker API for pending jobs and execute matches.

```bash
# Build and start match workers
docker-compose -f docker-compose.workers.yml up -d

# Scale workers based on load
docker-compose -f docker-compose.workers.yml up -d --scale worker=3
```

## Running the Index Builder

The index builder generates static JSON files for the web platform.

```bash
# Run once to generate index files
docker-compose -f docker-compose.workers.yml run indexer

# For automatic deployment, set DEPLOY_COMMAND in .env:
# DEPLOY_COMMAND=wrangler pages deploy /app/data --project-name=aicodebattle
```

## Cloudflare Configuration

### Pages Project

Create the Pages project in Cloudflare dashboard:
1. Go to Workers & Pages > Create application > Pages > Upload assets
2. Project name: `aicodebattle`
3. Upload the `web/dist/` directory

Or use wrangler CLI:
```bash
npm install -g wrangler
wrangler login
cd web
npm install
npm run build
wrangler pages deploy dist --project-name=aicodebattle
```

### Custom Domain for Pages (future consideration)

The site is served at the canonical `https://ai-code-battle.pages.dev` — no custom domain is
required, and `aicodebattle.com` is not currently registered. If the domain is registered later,
configure it in the Cloudflare dashboard:
1. Go to your Pages project > Custom domains
2. Add your domain
3. DNS will be automatically configured

The pages.dev origin will continue to work.

### Storage Bucket (R2 / B2)

Replays, match metadata, maps, thumbnails, and bot cards are written by the match workers to an
S3-compatible bucket (`acb-data`) and read by the web tier through a Cloudflare R2 binding, served
to the browser via the Pages Function at `/r2/*` (`web/functions/r2/[[path]].ts`). No R2 custom
domain is used — the canonical public path is `https://ai-code-battle.pages.dev/r2/*`.

```bash
# Informational: prints the bucket endpoint, the (optional) B2 CDN CNAME target, verifies B2
# credentials, and lists the expected public URLs. Requires no Cloudflare credentials.
./scripts/setup-b2.sh
```

Create the bucket manually if it does not yet exist:
```bash
wrangler r2 bucket create acb-data
```

### DNS Configuration

No custom DNS records are required: the canonical domain `https://ai-code-battle.pages.dev` is
managed by Cloudflare Pages, and R2 is served via the `/r2/*` Pages Function on that same host.
The `aicodebattle.com` zone is not registered, so none of the records below were ever created —
they are listed only for reference if the domain is registered later:
- `aicodebattle.com` → CNAME to Pages (auto-configured when adding a custom domain)
- `api` subdomain → A record pointing to the Traefik LoadBalancer IP (proxied), for the Go API
- No separate R2/B2 DNS record — storage is fronted by the `/r2/*` Pages Function

## Monitoring

### Health Endpoints

The API server provides health endpoints for Kubernetes probes:

- **Liveness**: `GET /health` or `GET /api/health`
  - Returns 200 if the process is running

- **Readiness**: `GET /ready` or `GET /api/ready`
  - Returns 200 if database is connected
  - Returns 503 if database is unavailable

### Kubernetes Monitoring

- Use `kubectl --server=http://kubectl-apexalgo-iad:8001` for read-only cluster access
- Check pod status: `kubectl get pods -n ai-code-battle`
- View logs: `kubectl logs -n ai-code-battle deployment/acb-matchmaker`

### Cloudflare Monitoring

- Cloudflare Analytics: Available in Cloudflare dashboard
- Pages deployments: Workers & Pages > aicodebattle

## Troubleshooting

### Worker can't connect to API

Check that the API service is running and reachable through the Traefik ingress (the public api
subdomain was never registered; the Go API is reached via internal cluster networking).

### Bot authentication failures

Verify `BOT_SECRET_*` values match what's registered in the database.

### B2/R2 upload failures

Check B2/R2 credentials and bucket permissions.

### Index builder not deploying

Ensure `DEPLOY_COMMAND` is set correctly and credentials have upload permissions.
