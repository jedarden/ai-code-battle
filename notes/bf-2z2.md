# Fix acb-map-evolver ImagePullBackOff

**Date:** 2025-06-27
**Bead:** bf-2z2

## Problem
Pod `acb-map-evolver` was in ImagePullBackOff state for 3+ hours. The image `ronaldraygun/acb-map-evolver:e5dc3bc` was not found on Docker Hub.

## Root Cause
The image was never built and pushed to Docker Hub. The CI workflow (`acb-build-images` in `iac-ci` cluster) builds to the Forgejo registry (`forgejo.ardenone.com/ai-code-battle/`), but the deployment manifest references Docker Hub (`ronaldraygun/`).

## Resolution
1. **Built image locally:**
   ```bash
   docker build -t ronaldraygun/acb-map-evolver:e5dc3bc -t ronaldraygun/acb-map-evolver:latest \
     -f cmd/acb-map-evolver/Dockerfile .
   ```

2. **Pushed to Docker Hub:**
   ```bash
   docker push ronaldraygun/acb-map-evolver:e5dc3bc
   docker push ronaldraygun/acb-map-evolver:latest
   ```
   Digest: `sha256:3d5a4a4dfa8bb73e46b3ec2d937846f5289d556853d5c3d41b180a42d4ed66d9`

3. **Manifest in declarative-config:**
   The manifest at `/home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-map-evolver-deployment.yml` already exists (contrary to task description stating it was missing).

## Status
- ✅ Image built and pushed to Docker Hub
- ✅ Deployment manifest exists in declarative-config
- ⏳ Pod recovery pending (kubectl-proxy connection issues prevented verification)

## Follow-up Needed
The CI workflow (`acb-build-images`) builds to Forgejo registry, but deployments use Docker Hub. Consider either:
1. Updating CI to also push to Docker Hub, OR
2. Updating deployment manifests to use Forgejo registry images
