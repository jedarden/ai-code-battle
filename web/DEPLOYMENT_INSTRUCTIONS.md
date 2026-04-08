# Cloudflare Pages Deployment Instructions

## Current Status

- ✅ Site builds successfully (run `cd web && npm run build`)
- ✅ Build output is in `web/dist/`
- ✅ GitHub Actions workflow is configured (`.github/workflows/deploy-pages.yml`)
- ❌ Cloudflare API Token not configured

## Deployment Options

### Option 1: Set up GitHub Actions (Recommended)

1. Create a Cloudflare API Token:
   - Go to https://dash.cloudflare.com/profile/api-tokens
   - Create a token with **Edit Cloudflare Workers** permissions
   - Or use this custom token config:
     ```
     Account > Cloudflare Pages > Edit
     Zone > DNS > Edit (if using custom domain)
     ```

2. Get your Cloudflare Account ID:
   - Go to https://dash.cloudflare.com
   - Click on your domain/account
   - Find Account ID in the right sidebar

3. Add secrets to GitHub repository:
   ```bash
   gh secret set CLOUDFLARE_API_TOKEN --body "your_token_here"
   gh secret set CLOUDFLARE_ACCOUNT_ID --body "your_account_id_here"
   ```

4. Trigger the workflow:
   ```bash
   gh workflow run deploy-pages.yml
   ```

   Or push a change to `web/` or `.github/workflows/deploy-pages.yml`

### Option 2: Deploy with Wrangler (Manual)

1. Install wrangler:
   ```bash
   npm install -g wrangler
   ```

2. Set environment variables:
   ```bash
   export CLOUDFLARE_API_TOKEN="your_token_here"
   export CLOUDFLARE_ACCOUNT_ID="your_account_id_here"
   ```

3. Deploy:
   ```bash
   cd /home/coding/ai-code-battle/web
   npm run build
   wrangler pages deploy dist --project-name=ai-code-battle
   ```

### Option 3: Create via Cloudflare Dashboard

1. Go to https://dash.cloudflare.com/
2. Navigate to **Workers & Pages** > **Create application** > **Pages**
3. Choose **Upload Assets**
4. Project name: `ai-code-battle`
5. Upload the contents of `web/dist/`

## After Deployment

- **Pages URL**: `https://ai-code-battle.pages.dev`
- Verify by visiting the URL and checking the site loads

## Automated Deployment (Future)

Once the GitHub Actions workflow is set up with secrets:
- Any push to `master` that changes files in `web/` will trigger deployment
- Manual trigger: `gh workflow run deploy-pages.yml`

## Verification

```bash
# Check if site is accessible
curl -I https://ai-code-battle.pages.dev

# Or in browser
open https://ai-code-battle.pages.dev
```
