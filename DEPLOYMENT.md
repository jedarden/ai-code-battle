# AI Code Battle - Deployment Guide

This document describes how to deploy AI Code Battle to production.

## Architecture Overview

The platform is split across two tiers:

1. **Cloudflare (free tier)** - Web-facing infrastructure
   - Pages: SPA shell + pre-computed JSON index files
   - Worker: API endpoints for registration, job coordination
   - D1: SQLite database for bots, matches, ratings
   - R2: Replays, match metadata, maps, thumbnails

2. **Rackspace Spot** - Compute tier
   - Match workers: Execute matches, upload replays to R2
   - Bot containers: Run strategy bot HTTP servers
   - Index builder: Generates JSON indexes, deploys to Pages

## Prerequisites

- Cloudflare account with:
  - Pages project created
  - Worker deployed
  - D1 database created
  - R2 bucket with custom domain configured
- Rackspace Spot account or equivalent container hosting
- Docker and docker-compose installed

## Environment Setup

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and fill in your values:
   - `ACB_API_ENDPOINT`: Your Cloudflare Worker URL
   - `ACB_API_KEY`: Worker API key
   - `ACB_R2_*`: R2 credentials from Cloudflare dashboard
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

### Worker API

Deploy the worker:
```bash
cd worker-api
npm install
wrangler deploy
```

### D1 Database

Create the database:
```bash
wrangler d1 create acb-db
```

Apply migrations (if any):
```bash
wrangler d1 execute acb-db --file=./schema.sql
```

### R2 Bucket

Create the bucket:
```bash
wrangler r2 bucket create acb-data
```

Configure custom domain in Cloudflare dashboard:
- Domain: `data.aicodebattle.com`
- Bucket: `acb-data`

### Pages

Deploy the web SPA:
```bash
cd web
npm install
npm run build
wrangler pages deploy dist --project-name=aicodebattle
```

## Monitoring

- Cloudflare Analytics: Available in Cloudflare dashboard
- Worker Logs: `wrangler tail`
- Container Logs: `docker-compose logs -f`

## Troubleshooting

### Worker can't connect to API

Check that `ACB_API_ENDPOINT` is correct and accessible from the worker container.

### Bot authentication failures

Verify `BOT_SECRET_*` values match what's registered in the Worker API.

### R2 upload failures

Check R2 credentials and bucket permissions.

### Index builder not deploying

Ensure `DEPLOY_COMMAND` is set correctly and Cloudflare API token has Pages deploy permissions.
