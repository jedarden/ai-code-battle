# BF-22VC5 Infrastructure Blocker Summary - 2026-06-04

## Task
Deploy P0: build acb-enrichment Docker image and re-enable deployment (apexalgo-iad)

## Summary
**Status: BLOCKED - Multiple Infrastructure Issues**

The deployment manifests are correctly configured with `sha-97b4b0f`, but the service cannot be deployed due to multiple infrastructure blockers across two clusters.

## Current State (2026-06-04)

### Manifests (Correct)
- **declarative-config**: `forgejo.ardenone.com/ai-code-battle/acb-enrichment:sha-97b4b0f` ✅
- **ai-code-battle**: Synced with declarative-config ✅
- **Deployment enabled**: replicas=1 ✅

### Cluster State (Broken)
- **apexalgo-iad deployment**: Still showing `sha-8f1dcc4` (ArgoCD not synced or image doesn't exist)
- **Pod status**: ImagePullBackOff (image doesn't exist in registry OR secret missing)

## Infrastructure Blockers

### 1. Missing Image Pull Secret (apexalgo-iad)
```
kubectl get secrets -n ai-code-battle
# Shows: docker-hub-registry
# Missing: forgejo-container-registry
```

The deployment requires `forgejo-container-registry` secret but only `docker-hub-registry` exists in the ai-code-battle namespace. Other ACB services use `ronaldraygun/*` from Docker Hub, but enrichment is configured for Forgejo registry.

**Impact**: Even if the image exists, the pod will fail to pull it.

**Required Action**: Create `forgejo-container-registry` secret in ai-code-battle namespace on apexalgo-iad.

### 2. CI/CD Cluster Timeouts (iad-ci)
```
kubectl get workflows -n argo-workflows
# Shows: Multiple acb-* workflows failed with "Pod was active on the node longer than the specified deadline"
```

The test phase is timing out, preventing image builds from completing.

**Impact**: Cannot trigger enrichment image builds via CI.

**Required Action**: Fix iad-ci cluster capacity or increase test deadline.

### 3. Cluster CPU Exhaustion (apexalgo-iad)
```
kubectl get nodes -n ai-code-battle
# All 3 nodes at or near capacity
kubectl get pods -n ai-code-battle
# Multiple pods in Pending, CrashLoopBackOff, CreateContainerConfigError
```

**Impact**: Even if the image pull worked, pods may not schedule.

**Required Action**: Scale down non-critical workloads or add node capacity.

## Registry Pattern Mismatch

### Current ACB Services (Docker Hub)
- `ronaldraygun/acb-api@sha256:...`
- `ronaldraygun/acb-evolver@sha256:...`
- `ronaldraygun/acb-worker@sha256:...`
- All use `docker-hub-registry` secret (exists)

### Enrichment (Forgejo - Different Pattern)
- `forgejo.ardenone.com/ai-code-battle/acb-enrichment:sha-97b4b0f`
- Requires `forgejo-container-registry` secret (missing)

### WorkflowTemplate Tag Format
- `acb-build.yml`: Uses `sha-` prefix: `{{workflow.parameters.sha}}`
- `acb-images-build-workflowtemplate.yml`: No prefix: `{{workflow.parameters.commit-sha}}`

This inconsistency may cause tag mismatches between what CI pushes and what deployments expect.

## Recommended Fix Path

### Option A: Add Forgejo Secret (Align with Current Config)
1. Copy/create `forgejo-container-registry` secret in ai-code-battle namespace
2. Trigger CI build for enrichment
3. Verify ArgoCD syncs the deployment

### Option B: Use Docker Hub (Align with Existing Services)
1. Update deployment manifest to use `ronaldraygun/acb-enrichment:sha-{commit}`
2. Update CI to push to Docker Hub
3. Use existing `docker-hub-registry` secret

Option B is simpler as Docker Hub secret already exists and matches other services.

## What Has Been Done
1. ✅ Verified enrichment source at `cmd/acb-enrichment/` (Dockerfile valid)
2. ✅ Synced manifests between ai-code-battle and declarative-config
3. ✅ Confirmed enrichment is included in acb-images-build WorkflowTemplate
4. ❌ Cannot build image (CI timing out)
5. ❌ Cannot deploy (secret missing, cluster full)

## Next Steps (Infrastructure Required)
1. Fix iad-ci cluster timeout issues OR build image locally
2. Add forgejo-container-registry secret OR change to Docker Hub pattern
3. Scale apexalgo-iad cluster capacity
4. Trigger fresh build after fixing CI
5. Verify ArgoCD syncs deployment

## Commit Reference
- ai-code-battle: ca0093d (synced enrichment manifest with sha-97b4b0f)
- declarative-config: 640df1d (synced from ai-code-battle)
