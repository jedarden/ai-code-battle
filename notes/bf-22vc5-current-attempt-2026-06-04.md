# ACB Enrichment Deployment - Current Attempt

**Date:** 2026-06-04  
**Commit:** 9795cde  
**Status:** BLOCKED - Infrastructure Access Required

## What Was Verified

### ✅ Completed
- Located acb-enrichment source at `cmd/acb-enrichment/`
- Verified Dockerfile is valid (`cmd/acb-enrichment/Dockerfile`)
- Located WorkflowTemplate: `acb-enrichment-build` in declarative-config
- Located deployment manifest with placeholder: `ronaldraygun/acb-enrichment@sha256:placeholder`

### ❌ Blockers

#### 1. iad-ci Kubeconfig Missing
Expected at `/home/coding/.kube/iad-ci.kubeconfig` but does not exist.
According to docs, this must be obtained from Rackspace Spot UI and manually saved.

#### 2. Docker Daemon Not Accessible
Docker client exists (`docker --version` works) but daemon is not running:
```bash
docker info
# Error: Cannot connect to the Docker daemon at unix:///var/run/docker.sock
```

Starting dockerd manually requires privileges and may have systemd conflicts.

#### 3. argo-ci.ardenone.com Returns 502
The Argo Workflows UI returns 502 Bad Gateway, likely indicating:
- Service is down
- Ingress is misconfigured  
- Network routing issue

## Required Actions

### Option A: Obtain iad-ci Kubeconfig (Recommended)
1. Log into Rackspace Spot UI at us-east-iad-1
2. Navigate to cluster credentials
3. Download kubeconfig for ServiceAccount `argocd-manager`
4. Save to `/home/coding/.kube/iad-ci.kubeconfig`
5. Trigger workflow manually

### Option B: Build Locally with Docker
1. Start Docker daemon (requires root/systemd)
2. Build image locally: `docker build -t ronaldraygun/acb-enrichment:sha-9795cde -f cmd/acb-enrichment/Dockerfile .`
3. Push to Docker Hub (requires ronaldraygun credentials)

### Option C: Fix argo-ci Service
Debug why argo-ci.ardenone.com returns 502:
- Check Traefik ingress configuration
- Verify Argo Workflows service is running
- Check network policies

## Next Steps (when unblocked)

1. Trigger build workflow:
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: acb-enrichment-build-manual-
  namespace: argo-workflows
  annotations:
    commit_sha: "9795cde"
spec:
  workflowTemplateRef:
    name: acb-enrichment-build
EOF
```

2. Monitor workflow completion and capture image SHA

3. Update deployment manifest:
```yaml
image: ronaldraygun/acb-enrichment@sha256:<real-sha>
```

4. Push to declarative-config

## Summary
All code is ready and verified. The only blocker is CI/CD infrastructure access. This requires manual setup of either:
- iad-ci kubeconfig from Rackspace Spot UI, OR
- Docker daemon and credentials for local build, OR  
- Debugging argo-ci service connectivity
