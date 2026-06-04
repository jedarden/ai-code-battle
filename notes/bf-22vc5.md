# Notes for bf-22vc5: Deploy acb-enrichment Docker Image

## Status: BLOCKED - Missing iad-ci kubeconfig

## Completed Work

1. **Verified enrichment service Dockerfile** - Located at `cmd/acb-enrichment/Dockerfile`, looks complete and well-structured.
2. **Updated acb-build workflow template** in declarative-config:
   - Added `build-enrichment` step to build acb-enrichment image
   - Added acb-enrichment to update-declarative-config image list
   - Enhanced update-declarative-config to handle k8s/apexalgo-iad deployments with @sha256 digest format
3. **Committed and pushed changes** to declarative-config (commit cc280c8)

## Remaining Work

1. **Build the enrichment image** - The workflow is ready but cannot be triggered because:
   - The iad-ci kubeconfig at `/home/coding/.kube/iad-ci.kubeconfig` does not exist
   - No local Docker/Podman installation available

2. **Update deployment manifest** - Once the image is built, update:
   - `~/declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-enrichment-deployment.yml.disabled`
   - Replace `sha256:placeholder` with the actual digest

3. **Re-enable deployment** - Rename to `acb-enrichment-deployment.yml` and push

## Next Steps

Please provide one of the following:

1. **iad-ci kubeconfig** - Place at `/home/coding/.kube/iad-ci.kubeconfig` so the workflow can be triggered
2. **Manual workflow trigger** - Access `https://argo-ci.ardenone.com` and trigger the `acb-build` workflow manually
3. **Local container build** - Install Docker/Podman to build and push the image locally

## Workflow Changes Summary

The `acb-build-workflowtemplate.yml` now:
- Builds the enrichment image as `ronaldraygun/acb-enrichment:<sha>`
- Fetches the Docker Hub digest for deployments using `@sha256:` format
- Updates manifests in both `k8s/iad-acb` and `k8s/apexalgo-iad` directories
# Enrichment deployment re-enabled Thu Jun  4 12:23:47 AM EDT 2026
