# BF-22VC5: acb-enrichment Deployment - Infrastructure Blocker

## Task Summary
Deploy P0: Build acb-enrichment Docker image and re-enable deployment on apexalgo-iad.

## Investigation Results

### What Works
- ✅ Located enrichment service source: `cmd/acb-enrichment/`
- ✅ Verified Dockerfile at `cmd/acb-enrichment/Dockerfile` is correct
- ✅ Confirmed `acb-build` WorkflowTemplate includes enrichment build (lines 93-102)
- ✅ Located deployment manifest in declarative-config: `k8s/apexalgo-iad/ai-code-battle/acb-enrichment-deployment.yml`

### The Blocker
The deployment manifest has placeholder SHA (`sha256:placeholder` on line 40). To build the real image, the `acb-build` workflow must be submitted to the iad-ci cluster.

**Infrastructure Issue:** The iad-ci.kubeconfig file referenced in project instructions (`/home/coding/.kube/iad-ci.kubeconfig`) does not exist on this machine.

**Access Attempts:**
- kubectl proxy at `http://traefik-iad-ci.tail1b1987.ts.net:8001` - works but is **read-only**
- Cannot submit workflows through proxy (ServiceAccount lacks create permissions)
- acb-enrichment image doesn't exist on Docker Hub (confirmed via API: `{"message":"object not found"}`)

### What Needs to Happen (Prerequisites)
1. **Obtain iad-ci kubeconfig** - Download from Rackspace Spot Console → iad-ci cluster → Access
   - Generate kubeconfig for ServiceAccount `argocd-manager`
   - Save to `/home/coding/.kube/iad-ci.kubeconfig`
2. **Submit acb-build workflow:**
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
3. Workflow builds all ACB images including acb-enrichment
4. Workflow's `update-declarative-config` step updates deployment manifest with real SHA
5. ArgoCD syncs the updated manifest to apexalgo-iad cluster

### Current Status
- **BLOCKED:** Missing iad-ci.kubeconfig for workflow submission
- **Enrichment Dockerfile:** Verified correct
- **Workflow template:** Verified includes enrichment
- **Deployment manifest:** Has placeholder SHA, needs real image

## Alternative Approaches Considered
1. **GitHub webhook trigger** - No webhook configured for acb-build on ai-code-battle repo
2. **Argo UI submission** - UI not accessible via Tailscale proxy
3. **Manual Docker build** - Possible but would bypass the CI/CD pipeline and wouldn't update declarative-config automatically

## Recommendation
Set up the iad-ci.kubeconfig file on this machine (ex44) to enable workflow submission. This is a one-time setup task that will unblock all future iad-ci workflow operations.
