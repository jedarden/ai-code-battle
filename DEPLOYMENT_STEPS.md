# Cloudflare Pages Deployment - Next Steps

The deployment infrastructure is configured and ready. To complete the deployment, follow these steps.

> **CI note:** GitHub Actions are **disabled across all repos** in this org —
> `.github/workflows/deploy-pages.yml` is renamed `deploy-pages.yml.disabled` and does not run. Do not
> re-enable GitHub Actions. CI/CD is Argo Workflows on the `iad-ci` cluster, and secrets are provisioned
> as SealedSecrets / ExternalSecrets in `jedarden/declarative-config` (synced by ArgoCD) — **never**
> stored as GitHub repository secrets. See the CI/CD section of `CLAUDE.md`; for the real pattern, look
> at any `*-sealedsecret.yml` in `declarative-config/k8s/ardenone-cluster/kubernetes-reflector/` (e.g.
> `cloudflare-sealedsecret.yml`, `backblaze-sealedsecret.yml`). The old `acb-*` SealedSecrets under
> `k8s/apexalgo-iad/ai-code-battle/` were removed when that compute tier was decommissioned (2026-07-21).

## Quick Start

### 1. Get Your Cloudflare Credentials

**API Token:**
1. Go to [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)
2. Create a token with **Edit Cloudflare Workers** permissions
3. Copy the token

**Account ID:**
1. Go to any Cloudflare page in the dashboard
2. Find your Account ID in the right sidebar
3. Or run: `wrangler whoami` after logging in

### 2. Provision the Credentials for CI

Cloudflare credentials are **not** added to GitHub (Actions are disabled). For an automated Argo
Workflows deploy, commit them as a SealedSecret (or an ExternalSecret backed by OpenBao) in
`declarative-config` — e.g. an `acb-cloudflare-credentials-sealedsecret.yml` exposing
`CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`. ArgoCD syncs it into the namespace the deploy
workflow runs in. Model it on the existing `cloudflare-sealedsecret.yml` in
`k8s/ardenone-cluster/kubernetes-reflector/` (the old `acb-*` SealedSecrets were removed with the
apexalgo-iad decommission — see `notes/bf-1yj.md`).

```bash
# Seal the secret the org's standard way (kubeseal against the iad-ci cluster), then commit the
# resulting manifest to declarative-config — see CLAUDE.md "CI/CD — Argo Workflows (iad-ci)".
```

For a one-off local deploy you can instead export the values for `wrangler` (see step 4) — no
SealedSecret required.

### 3. Deploy via Argo Workflows

Submit a Cloudflare-Pages-deploy workflow to the `iad-ci` cluster (the org's CI system; the
`website-build` template is an example of a Pages-deploy workflow):

```bash
# Submit manually (replace <template-name> with the ai-code-battle Pages-deploy template)
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: acb-deploy-pages-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: <template-name>
EOF
```

If a Cloudflare Pages Git integration is configured for the project, a push to `master` also
triggers a build directly from Cloudflare (independent of GitHub Actions).

### 4. Manual Deployment with Wrangler

For a one-off local deploy (no CI needed):

```bash
# Install wrangler (if not already installed)
npm install -g wrangler

# Authenticate — either interactive login, or export the credentials first:
#   export CLOUDFLARE_API_TOKEN=your_token
#   export CLOUDFLARE_ACCOUNT_ID=your_account_id
wrangler login

# Deploy
cd /home/coding/ai-code-battle
./scripts/deploy-pages.sh
```

## What's Already Done

✓ Build configured (`web/package.json`, `vite.config.ts`)
✓ GitHub Actions workflow present but **disabled** (`.github/workflows/deploy-pages.yml.disabled`)
✓ Wrangler configuration (`wrangler.toml`)
✓ Deployment script (`scripts/deploy-pages.sh`)
✓ Documentation (`web/CLOUDFLARE_DEPLOYMENT.md`)
✓ Build tested and working (`web/dist`)

## After Deployment

Once deployed, the site is accessible at:
- **Pages URL (canonical):** `https://ai-code-battle.pages.dev`
- **Custom domain:** none — `aicodebattle.com` is not registered. It can be attached later as a
  Cloudflare Pages custom domain without changing the pages.dev origin.

## Verification

```bash
# View the live site (canonical domain)
curl -I https://ai-code-battle.pages.dev

# Inspect recent Argo Workflow runs on iad-ci
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflows -n argo-workflows --sort-by=.metadata.creationTimestamp | tail -10
```
