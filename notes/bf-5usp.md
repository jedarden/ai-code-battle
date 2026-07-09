# bf-5usp: Forgejo Webhook Registration for ai-code-battle

## Status: Already Configured

The Forgejo webhook for ai-code-battle is **already registered** and active.

## Webhook Details

- **URL**: `https://webhooks-ci.ardenone.com/ai-code-battle`
- **Events**: `push`
- **Active**: `true`
- **Created**: `2026-07-02T14:28:39Z`
- **Updated**: `2026-07-02T15:52:22Z`

## Backend Configuration

The webhook is handled by:
- **EventSource**: `forgejo-webhooks` (`k8s/iad-ci/argo-events/forgejo-eventsource.yml`)
- **Sensor**: `ai-code-battle-ci-sensor` (`k8s/iad-ci/argo-events/ai-code-battle-sensor.yml`)
- **IngressRoute**: `github-webhook-ingressroute` (`k8s/iad-ci/argo-events/webhook-ingressroute.yml`)

The sensor triggers multiple workflow templates on push to master:
1. `acb-images-build` - Builds all Docker images
2. `acb-site-pages-build` - Builds and deploys to Cloudflare Pages
3. `acb-bots-build` - Builds strategy bot images
4. `acb-enrichment-build` - Builds enrichment service
5. `acb-build` - Builds core service images

## Conclusion

No action was required — the webhook was already properly configured.
