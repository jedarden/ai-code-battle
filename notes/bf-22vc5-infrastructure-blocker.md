# Infrastructure Blocker: bf-22vc5 - acb-enrichment Deployment

## Problem
The `acb-enrichment-deployment.yml` is disabled because it contains a placeholder SHA:
```yaml
image: ronaldraygun/acb-enrichment@sha256:placeholder
```

## Root Cause
The `acb-enrichment` Docker image has never been built. Docker Hub repository exists but has no tags:
```bash
curl -sk https://hub.docker.com/v2/repositories/ronaldraygun/acb-enrichment/tags/
# Returns: {"count":0,"next":null,"previous":null,"results":[]}
```

## Infrastructure Blocker
Cannot trigger the acb-build workflow on iad-ci because:
- The iad-ci kubeconfig (`/home/coding/.kube/iad-ci.kubeconfig`) is missing
- The rs-manager kubeconfig (`/home/coding/.kube/rs-manager.kubeconfig`) is also missing
- The kubectl-proxy on `traefik-iad-ci:8001` is read-only (ServiceAccount: `devpod-observer:devpod-observer`)
- Cannot create workflows via read-only proxy

## Checked Alternatives (2024-06-04)
1. **Docker runtime**: Not available on this Hetzner server
2. **Podman runtime**: Not available on this Hetzner server
3. **GitHub Actions**: Disabled across all repos per CLAUDE.md
4. **ArgoCD read-only API**: Cannot submit workflows via read-only access
5. **Argo UI**: Available at https://argo-ci.ardenone.com but requires Google SSO (not programmatic)

## Available Access
- Read-only kubectl-proxy: `kubectl --server=http://traefik-iad-ci:8001` works
- Argo UI: `https://argo-ci.ardenone.com` (requires Google SSO)
- rs-manager cluster: Available via traefik-rs-manager:8001 (no Argo Workflows CRDs)

## Expected Workflow
The `acb-build` WorkflowTemplate in `declarative-config/k8s/iad-ci/argo-workflows/acb-build-workflowtemplate.yml` includes:
1. Run Go tests
2. Build all ACB images including `acb-enrichment` (line 93-102)
3. Update deployment manifests with the new digest (line 103-108, 216-262)

The workflow should be triggered with:
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: acb-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: acb-build
EOF
```

## Resolution Options
1. **Obtain iad-ci kubeconfig**: Get the kubeconfig from Rackspace Spot console or OpenBao
2. **Use rs-manager**: If rs-manager can execute workflows on iad-ci (multi-cluster setup)
3. **Manual build**: Build image locally and push to Docker Hub (requires Docker/Kaniko - not available)
4. **Manual UI trigger**: Access https://argo-ci.ardenone.com via browser and trigger manually
5. **Request manual trigger**: Ask someone with access to trigger the workflow

## Status
**BLOCKED**: Waiting for iad-ci kubeconfig or alternative workflow trigger method.

## Next Steps
- [ ] Obtain iad-ci.kubeconfig with cluster-admin ServiceAccount credentials
- [ ] Submit acb-build workflow manually
- [ ] Verify image builds successfully
- [ ] Confirm deployment manifest is updated with real SHA
