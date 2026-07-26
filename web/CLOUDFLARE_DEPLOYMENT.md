# Cloudflare Pages Deployment Guide

This guide explains how to deploy the AI Code Battle SPA to Cloudflare Pages.

## Quick Start

### Option 1: GitHub Integration (Recommended)

1. Go to [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. Navigate to **Workers & Pages** > **Create application** > **Pages** > **Connect to Git**
3. Select the `ai-code-battle` repository
4. Configure the build settings:
   - **Project name**: `ai-code-battle`
   - **Production branch**: `master`
   - **Build command**: `cd web && npm run build`
   - **Build output directory**: `web/dist`
5. Click **Save and Deploy**

The site will now automatically deploy whenever you push to the `master` branch.

### Option 2: Manual Deployment with Wrangler

1. Install wrangler:
   ```bash
   npm install -g wrangler
   ```

2. Authenticate with Cloudflare:
   ```bash
   wrangler login
   ```

3. Build the site:
   ```bash
   cd web
   npm install
   npm run build
   ```

4. Deploy to Pages:
   ```bash
   wrangler pages deploy dist --project-name=ai-code-battle
   ```

## Environment Variables

For the GitHub Actions workflow to work, you need to set these secrets in your GitHub repository:

- `CLOUDFLARE_API_TOKEN`: Your Cloudflare API token
- `CLOUDFLARE_ACCOUNT_ID`: Your Cloudflare account ID

### Getting Your Credentials

1. **API Token**:
   - Go to [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)
   - Create a token with **Edit Cloudflare Workers** permissions
   - Copy the token

2. **Account ID**:
   - Go to any Cloudflare page
   - Find your Account ID in the right sidebar
   - Or run: `wrangler whoami` after logging in

3. **Add to GitHub**:
   - Go to your repository **Settings** > **Secrets and variables** > **Actions**
   - Click **New repository secret**
   - Add `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`

## Custom Domain (future consideration)

The site is live at the Cloudflare-provided domain `https://ai-code-battle.pages.dev` — no custom
domain is required, and `aicodebattle.com` is not currently registered. If a custom domain is
registered later, attach it in Cloudflare Pages:

1. Go to your Pages project in the Cloudflare Dashboard
2. Navigate to **Custom domains**
3. Add your domain
4. Cloudflare will automatically configure the DNS

The pages.dev URLs will continue to work as the origin.

## Verification

After deployment, verify the site is accessible:

- **Pages URL**: `https://ai-code-battle.pages.dev` (canonical — no custom domain registered)

## Troubleshooting

### Build fails

- Make sure all dependencies are installed: `cd web && npm install`
- Check the build logs in the Cloudflare Dashboard

### Authentication fails

- Make sure your API token has the correct permissions
- Verify your account ID is correct

### Custom domain not working

No custom domain is configured — the site is served at the canonical
`https://ai-code-battle.pages.dev`. If a custom domain is added later, check DNS propagation and
verify its status in the Cloudflare Dashboard.
