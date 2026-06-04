# BF-22VC5: BLOCKER - Missing iad-ci.kubeconfig

## Task Cannot Be Completed

The task to deploy acb-enrichment is **BLOCKED** on a missing infrastructure credential.

## What I Verified
✅ acb-enrichment source code exists at `cmd/acb-enrichment/`
✅ Dockerfile is correct and well-structured
✅ WorkflowTemplate `acb-build` includes enrichment build step
✅ Deployment manifest exists at `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-enrichment-deployment.yml`
✅ Deployment has placeholder SHA that needs real image

## The Blocker
**iad-ci.kubeconfig does not exist at `/home/coding/.kube/iad-ci.kubeconfig`**

This kubeconfig is required to:
- Submit Argo Workflows to iad-ci cluster
- Build Docker images via `acb-build` workflow
- Update declarative-config with new image SHAs

## What I Tried
1. ❌ Checked for existing kubeconfigs - none found
2. ❌ Checked read-only kubectl proxy - works but no write permissions
3. ❌ Checked for container runtime - none available
4. ❌ Checked for Docker Hub credentials - none available
5. ❌ Checked Forgejo Actions API - returns 404
6. ❌ Tried webhooks - require signatures I don't have
7. ❌ Checked GitHub Actions - disabled per project policy

## What Needs To Happen (External Action Required)
**Option 1: Obtain iad-ci kubeconfig (RECOMMENDED)**
1. Log into Rackspace Spot Console
2. Navigate to iad-ci cluster
3. Download kubeconfig for ServiceAccount `argocd-manager`
4. Save to `/home/coding/.kube/iad-ci.kubeconfig` on this machine
5. Then retry this task

**Option 2: Manual Docker build (workaround)**
1. Install docker/podman on this machine
2. Configure Docker Hub credentials
3. Build and push image manually
4. Update deployment manifest manually
5. Commit to declarative-config

**Option 3: Configure Forgejo webhook (long-term fix)**
1. Create Forgejo Actions workflow
2. Configure webhook to trigger on push
3. Workflow submits Argo Workflow to iad-ci

## Once Blocker Resolved
Run:
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

This will:
- Build acb-enrichment Docker image
- Push to Docker Hub
- Update declarative-config with real SHA
- ArgoCD will sync to apexalgo-iad

## Current Image Status
```
$ curl -s "https://hub.docker.com/v2/repositories/ronaldraygun/acb-enrichment/tags/"
{"message":"object not found","errinfo":{}}
```

Image does NOT exist on Docker Hub. Must be built first.

## Task Status
**CANNOT COMPLETE** - External action required to obtain iad-ci.kubeconfig.
