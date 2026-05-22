# Bead bf-3krf: K8s deployments and ExternalSecrets for extended bot fleet

## Status: Already Complete

This bead's work was completed in commit `1c8f0ae` on 2026-05-22.

## Commit: 1c8f0ae

The commit created the following K8s manifests in `manifests/acb-bots/`:

### Deployments
- bot-assassin-deployment.yml
- bot-defender-deployment.yml
- bot-farmer-deployment.yml
- bot-kamikaze-deployment.yml
- bot-nomad-deployment.yml
- bot-opportunist-deployment.yml
- bot-pacifist-deployment.yml
- bot-phalanx-deployment.yml
- bot-raider-deployment.yml
- bot-scout-deployment.yml

### ExternalSecrets
- bot-assassin-externalsecret.yml
- bot-defender-externalsecret.yml
- bot-farmer-externalsecret.yml
- bot-kamikaze-externalsecret.yml
- bot-nomad-externalsecret.yml
- bot-opportunist-externalsecret.yml
- bot-pacifist-externalsecret.yml
- bot-phalanx-externalsecret.yml
- bot-raider-externalsecret.yml
- bot-scout-externalsecret.yml

### Services (also created)
- bot-assassin-service.yml
- bot-defender-service.yml
- bot-farmer-service.yml
- bot-kamikaze-service.yml
- bot-nomad-service.yml
- bot-opportunist-service.yml
- bot-pacifist-service.yml
- bot-phalanx-service.yml
- bot-raider-service.yml
- bot-scout-service.yml

All manifests follow the established patterns:
- ArgoCD image updater annotations
- OpenBao ExternalSecrets for shared secrets
- Proper labels and annotations
- Health checks and resource limits
