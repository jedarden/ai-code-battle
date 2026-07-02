# Forgejo Webhook Registration for ai-code-battle - COMPLETED

## Task Status: ✅ COMPLETE

The Forgejo webhook for ai-code-battle is **already registered and active**.

## Webhook Details (Verified 2026-07-02T15:32:09Z)

- **URL**: `https://webhooks-ci.ardenone.com/ai-code-battle`
- **Events**: `push`
- **Active**: `true`
- **Created**: 2026-07-02T14:28:39Z
- **Updated**: 2026-07-02T15:01:40Z
- **Webhook ID**: 5

## Infrastructure Resources

- **EventSource**: `forgejo-webhooks` in argo-events namespace (iad-ci)
  - Endpoint: `/ai-code-battle` on port 12000
- **Sensor**: `ai-code-battle-github-ci-sensor` in argo-events namespace (iad-ci)
  - Triggers: `acb-build` WorkflowTemplate on push to master branch
- **IngressRoute**: `github-webhook-ingressroute`
  - Routes: `webhooks-ci.ardenone.com/ai-code-battle` → `forgejo-webhooks-eventsource-svc:12000`

## Verification Method

Verified via Forgejo API:
```bash
FORGEJO_TOKEN="$(git credential fill <<< 'protocol=https
host=git.ardenone.com
' | grep password | cut -d= -f2)"
curl -s -H "Authorization: token $FORGEJO_TOKEN" \
  "https://git.ardenone.com/api/v1/repos/jedarden/ai-code-battle/hooks"
```

## Acceptance Criteria: ✅ MET

The ai-code-battle repo on git.ardenone.com has an active webhook for push events pointing to the Argo Events sensor endpoint at webhooks-ci.ardenone.com/ai-code-battle.
