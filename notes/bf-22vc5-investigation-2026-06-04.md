# BF-22VC5 Investigation Summary (2026-06-04)

## Task
Deploy P0: build acb-enrichment Docker image and re-enable deployment (apexalgo-iad)

## Current State

### Completed Work
1. ✅ **Verified Dockerfile** - `cmd/acb-enrichment/Dockerfile` is valid and follows best practices
2. ✅ **Located WorkflowTemplate** - `acb-enrichment-build` exists in declarative-config
3. ✅ **Located Deployment Manifest** - `manifests/acb-enrichment-deployment.yml` confirmed with placeholder SHA
4. ✅ **Verified Build Triggers** - Argo Events sensor configured to trigger on push to master

### Infrastructure Blocker
**CRITICAL: No access to iad-ci cluster**

The iad-ci kubeconfig is missing at `~/.kube/iad-ci.kubeconfig`. This is required to:
- Submit workflows to iad-ci
- Check workflow status and logs
- Debug build failures

### Investigation Findings

1. **Workflow Configuration** - The `acb-enrichment-build` workflow template is correctly configured:
   - Clones from `git.ardenone.com/jedarden/ai-code-battle`
   - Builds using Kaniko with Dockerfile at `cmd/acb-enrichment/Dockerfile`
   - Pushes to `ronaldraygun/acb-enrichment:sha-{commit}` and `:latest`

2. **Docker Hub Image Status** - Image does not exist:
   - `ronaldraygun/acb-enrichment` returns 404 on Docker Hub
   - This indicates the workflow has never successfully completed

3. **Cluster Access Status**:
   - `~/.kube/iad-ci.kubeconfig` - **DOES NOT EXIST**
   - `~/.kube/rs-manager.kubeconfig` - **DOES NOT EXIST**
   - ArgoCD cluster secret for iad-ci exists but cannot be accessed via proxy (RBAC)
   - ExternalSecret for iad-ci credentials is **DISABLED**

4. **Webhook Attempts** - Multiple commits have attempted to trigger builds:
   - `87d0edb` - "ci: trigger acb-enrichment build (bf-22vc5)"
   - `ce82cb3` - "ci: trigger acb-enrichment build (bf-22vc5)"
   - `e228a4e` - "ci: trigger acb-enrichment build (bf-22vc5)"
   - `fcdadcb` - "ci: trigger acb-enrichment build (bf-22vc5)"
   - `9795cde` - "ci: trigger acb-enrichment build (bf-22vc5)"
   All failed to produce a Docker image.

5. **Cluster Relationship** - rs-manager manages iad-ci via ArgoCD:
   - iad-ci cluster registered in ArgoCD as `cluster-hcp-de5bec10-ce14-4eed-a6f4-750f3fd3a89a.spot.rackspace.com`
   - Server URL: `https://hcp-de5bec10-ce14-4eed-a6f4-750f3fd3a89a.spot.rackspace.com`
   - Managed cluster, should be accessible via rs-manager kubeconfig (which is also missing)

## Root Cause

The iad-ci cluster credentials were never properly configured or were lost. The ExternalSecret that should pull credentials from OpenBao is disabled:
- File: `/home/coding/declarative-config/k8s/ardenone-manager/argocd/cluster-iad-ci-externalsecret.yml.disabled`

Without cluster access, it's impossible to:
1. Submit workflows manually
2. Check workflow status
3. View pod logs
4. Debug why builds aren't completing

## Resolution Path

### Option 1: Obtain iad-ci Kubeconfig (RECOMMENDED)
1. Log in to Rackspace Spot console
2. Navigate to cluster `hcp-de5bec10-ce14-4eed-a6f4-750f3fd3a89a.spot.rackspace.com`
3. Download kubeconfig for ServiceAccount with cluster-admin access
4. Save to `/home/coding/.kube/iad-ci.kubeconfig`
5. Run: `kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows` to verify access

### Option 2: Re-enable ExternalSecret
1. Check if credentials exist in OpenBao at `ardenone-manager/argocd/cluster-iad-ci`
2. If not, obtain credentials from Rackspace Spot UI
3. Store in OpenBao
4. Rename `cluster-iad-ci-externalsecret.yml.disabled` to `cluster-iad-ci-externalsecret.yml`
5. Push to declarative-config

### Option 3: Manual Build (if Docker available)
1. Build locally: `docker build -f cmd/acb-enrichment/Dockerfile -t ronaldraygun/acb-enrichment:sha-$(git rev-parse --short HEAD) .`
2. Push to Docker Hub
3. Update deployment manifest with image SHA
4. Push to declarative-config

## Next Steps (Once Access is Restored)

1. **Submit workflow manually:**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Workflow
   metadata:
     generateName: acb-enrichment-build-manual-
     namespace: argo-workflows
   spec:
     workflowTemplateRef:
       name: acb-enrichment-build
   EOF
   ```

2. **Monitor workflow:**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows
   ```

3. **Get image SHA** from Docker Hub or workflow output

4. **Update deployment manifest:**
   - Edit `~/declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-enrichment-deployment.yml`
   - Replace `sha256:placeholder` with actual digest

5. **Push to declarative-config**

## Files Modified
- None (blocked by missing infrastructure access)

## Status
**BLOCKED** - Cannot proceed without iad-ci cluster access or alternative build method.
