# AI Code Battle - Deployment Guide

This document describes how to deploy AI Code Battle to production.

## Architecture Overview

The platform is split across two tiers:

1. **Cloudflare (free tier)** - Web-facing infrastructure
   - Pages: SPA shell + pre-computed JSON index files
   - R2: Replays, match metadata, maps, thumbnails (custom domain: r2.aicodebattle.com)

2. **Kubernetes (apexalgo-iad)** - Compute tier
   - Matchmaker: Pairs bots, creates jobs in PostgreSQL
   - Match workers: Execute matches, upload replays to B2
   - Bot containers: Run strategy bot HTTP servers
   - Index builder: Generates JSON indexes, uploads to B2/Pages
   - PostgreSQL: Bots, matches, ratings, job queue
   - Traefik: Ingress for api.aicodebattle.com

## Prerequisites

- Cloudflare account with:
  - Pages project created (aicodebattle)
  - R2 bucket with custom domain configured (r2.aicodebattle.com)
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

### Custom Domain for Pages

Configure custom domain in Cloudflare dashboard:
1. Go to your Pages project > Custom domains
2. Add domain: `aicodebattle.com`
3. DNS will be automatically configured

### R2 Bucket

Create the bucket:
```bash
wrangler r2 bucket create acb-data
```

Configure custom domain in Cloudflare dashboard:
1. Go to R2 > acb-data > Settings > Custom Domains
2. Add domain: `r2.aicodebattle.com`

### DNS Configuration

In Cloudflare DNS settings:
- `aicodebattle.com` → CNAME to Pages (auto-configured when adding custom domain)
- `api.aicodebattle.com` → A record pointing to Traefik LoadBalancer IP (proxied)
- `r2.aicodebattle.com` → CNAME to R2 (auto-configured when adding custom domain)

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

Check that the API service is running and accessible via Traefik ingress at `api.aicodebattle.com`.

### Bot authentication failures

Verify `BOT_SECRET_*` values match what's registered in the database.

### B2/R2 upload failures

Check B2/R2 credentials and bucket permissions.

### Index builder not deploying

Ensure `DEPLOY_COMMAND` is set correctly and credentials have upload permissions.
