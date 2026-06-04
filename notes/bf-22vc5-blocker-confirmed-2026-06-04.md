# Blocker Confirmed: bf-22vc5 - acb-enrichment Deployment

**Date:** 2026-06-04
**Updated:** 2026-06-04 (ee3fee6)

## Issue
The task requires triggering the acb-enrichment build on iad-ci cluster via Argo Workflows, but:
- The expected kubeconfig `/home/coding/.kube/iad-ci.kubeconfig` does not exist
- No kubectl proxy is available for iad-ci (DNS fails for `kubectl-proxy-iad-ci.tail1b1987.ts.net`)
- The workflow template `acb-enrichment-build` exists in declarative-config but cannot be triggered without cluster access
- Docker is not available on this machine for local builds

## What Was Found
1. The acb-enrichment source code exists at `/home/coding/ai-code-battle/cmd/acb-enrichment/`
2. The Dockerfile is valid and ready to build
3. The workflow template exists at `/home/coding/declarative-config/k8s/iad-ci/argo-workflows/acb-enrichment-build-workflowtemplate.yml`
4. Current commit SHA: `ee3fee6`
5. The image `ronaldraygun/acb-enrichment` does not exist on Docker Hub (no tags found)

## What's Needed
To proceed, one of the following is required:
1. The iad-ci.kubeconfig file with ServiceAccount credentials
2. A kubectl-proxy deployment for iad-ci (similar to other clusters)
3. An alternative trigger mechanism (webhook URL or API)

## Deployment Manifest Ready
The deployment manifest at:
- `/home/coding/ai-code-battle/manifests/acb-enrichment-deployment.yml` (staging)
- `/home/coding/declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-enrichment-deployment.yml` (active - replicas: 0)

Both have `image: ronaldraygun/acb-enrichment@sha256:placeholder` and need the real SHA after build.

## Next Steps (when cluster access is available)
1. Trigger the workflow:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Workflow
   metadata:
     generateName: acb-enrichment-build-manual-
     namespace: argo-workflows
     annotations:
       commit_sha: "ee3fee6"
   spec:
     workflowTemplateRef:
       name: acb-enrichment-build
   EOF
   ```

2. Monitor the build and capture the image SHA

3. Update both deployment manifests with the real SHA

4. Set replicas: 1 to enable the deployment

5. Push to declarative-config

## Status
**BLOCKED** - Requires external infrastructure setup (iad-ci kubeconfig provision). All code is ready; only CI/CD access is missing.
