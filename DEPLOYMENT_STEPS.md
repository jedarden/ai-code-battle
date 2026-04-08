# Cloudflare Pages Deployment - Next Steps

The deployment infrastructure is configured and ready. To complete the deployment, follow these steps:

## Quick Start (Recommended)

### 1. Get Your Cloudflare Credentials

**API Token:**
1. Go to [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)
2. Create a token with **Edit Cloudflare Workers** permissions
3. Copy the token

**Account ID:**
1. Go to any Cloudflare page in the dashboard
2. Find your Account ID in the right sidebar
3. Or run: `wrangler whoami` after logging in

### 2. Add Secrets to GitHub

1. Go to your repository: https://github.com/jedarden/ai-code-battle/settings/secrets/actions
2. Click **New repository secret**
3. Add `CLOUDFLARE_API_TOKEN` with your API token
4. Add `CLOUDFLARE_ACCOUNT_ID` with your account ID

### 3. Trigger Deployment

The workflow will automatically run on the next push to `master`, or you can trigger it manually:

```bash
gh workflow run deploy-pages.yml
```

Or via the GitHub UI: https://github.com/jedarden/ai-code-battle/actions/workflows/deploy-pages.yml

## Manual Deployment with Wrangler

If you prefer to deploy locally:

```bash
# Install wrangler (if not already installed)
npm install -g wrangler

# Login to Cloudflare
wrangler login

# Deploy
cd /home/coding/ai-code-battle
./scripts/deploy-pages.sh
```

## What's Already Done

✓ Build configured (`web/package.json`, `vite.config.ts`)
✓ GitHub Actions workflow configured (`.github/workflows/deploy-pages.yml`)
✓ Wrangler configuration (`wrangler.toml`)
✓ Deployment script (`scripts/deploy-pages.sh`)
✓ Documentation (`web/CLOUDFLARE_DEPLOYMENT.md`)
✓ Build tested and working (`web/dist`)

## After Deployment

Once deployed, the site will be accessible at:
- **Pages URL**: https://ai-code-battle.pages.dev
- **Custom domain**: https://aicodebattle.com (if configured in Cloudflare)

## Verification

```bash
# Check deployment status
gh run list --workflow=deploy-pages.yml

# View the live site
curl https://ai-code-battle.pages.dev
```
