# Forgejo Webhook Registration for ai-code-battle

## Finding

The Forgejo webhook for ai-code-battle is **already registered and active**.

## Webhook Details

- **URL**: `https://webhooks-ci.ardenone.com/ai-code-battle`
- **Events**: `push`
- **Active**: `true`
- **Created**: 2026-07-02T14:28:39Z
- **Updated**: 2026-07-02T14:35:22Z
- **Webhook ID**: 5

## Resources

- **EventSource**: `forgejo-webhooks` in argo-events namespace (iad-ci)
- **Sensor**: `ai-code-battle-github-ci-sensor` in argo-events namespace (iad-ci)
- **IngressRoute**: `github-webhook-ingressroute` routes `webhooks-ci.ardenone.com/ai-code-battle` to `forgejo-webhooks-eventsource-svc:12000`

## Verification

Verified via Forgejo API:
```bash
curl -H "Authorization: token $FORGEJO_TOKEN" \
  "https://git.ardenone.com/api/v1/repos/jedarden/ai-code-battle/hooks"
```

The webhook triggers the `acb-build` WorkflowTemplate on push to the master branch.
