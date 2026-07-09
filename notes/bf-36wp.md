# bf-36wp: acb-site-build WorkflowTemplate Verification

## Finding

The `acb-site-build` WorkflowTemplate **exists** on iad-ci cluster but is **misconfigured** for its intended purpose.

## Current Configuration (INCORRECT)

The WorkflowTemplate currently:
- Clones the `ai-code-battle` repo from Forgejo
- Builds the `web/` directory using `npm ci && npm run build`
- Creates an nginx Dockerfile for the built site
- Uses Kaniko to build and push a container image to `forgejo.ardenone.com/ai-code-battle/acb-site`

**Problem:** This does NOT deploy to Cloudflare Pages.

## Required Configuration

Per acceptance criteria, the WorkflowTemplate should deploy the `web/` directory to Cloudflare Pages project `ai-code-battle`.

### Reference Pattern: `website-build` WorkflowTemplate

The correct pattern (from `website-build` which deploys jedarden.com):
```yaml
templates:
  - container:
      args: |
        set -e
        # Clone repo
        git clone --depth 1 --branch "$BRANCH" \
          "https://x-access-token:${GH_TOKEN}@github.com/{{workflow.parameters.repo}}.git" \
          /workspace

        # Build
        cd /workspace/{{workflow.parameters.build-dir}}
        {{workflow.parameters.build-command}}

        # Deploy to Cloudflare Pages
        npx wrangler@latest pages deploy {{workflow.parameters.output-dir}} \
          --project-name="{{workflow.parameters.cf-project}}" \
          --branch="$BRANCH" \
          --commit-dirty=true
      env:
        - name: CLOUDFLARE_API_TOKEN
          valueFrom:
            secretKeyRef:
              key: CF_API_TOKEN
              name: cloudflare-pages-secret
        - name: CLOUDFLARE_ACCOUNT_ID
          value: e26f015c7ba47a6ad6219385e77072b7
```

### Required Changes for acb-site-build

1. **Replace container image build with Cloudflare Pages deployment**
   - Remove Kaniko/executor and Dockerfile creation
   - Add `wrangler pages deploy` step

2. **Use Cloudflare Pages credentials**
   - Secret `cloudflare-pages-secret` exists ✓
   - Required env vars: `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`

3. **Parameters needed:**
   - `git-repo`: `forgejo.ardenone.com/ai-code-battle/ai-code-battle`
   - `branch`: `master`
   - `build-dir`: `web`
   - `build-command`: `npm ci && npm run build`
   - `output-dir`: `web/dist` (from vite.config.ts `outDir: 'dist'`)
   - `cf-project`: `ai-code-battle`

## Verification

- WorkflowTemplate exists: ✓
- Configured for web/ → ai-code-battle Pages deployment: ✗

## Next Steps

The WorkflowTemplate needs to be rewritten to follow the `website-build` pattern and deploy to Cloudflare Pages instead of building container images.
