# BF-22VC5: acb-enrichment Deployment - Current Status

## Task Summary
Deploy P0: Build acb-enrichment Docker image and re-enable deployment on apexalgo-iad.

## Investigation Complete

### Verified Components
✅ Enrichment service source: `cmd/acb-enrichment/` exists
✅ Dockerfile verified correct: `cmd/acb-enrichment/Dockerfile` (multi-stage Go build)
✅ WorkflowTemplate includes enrichment: `acb-build` has `build-enrichment` step (lines 93-102)
✅ Deployment manifest location: `declarative-config/k8s/apexalgo-iad/ai-code-battle/acb-enrichment-deployment.yml`

### The Blocker
The deployment manifest has placeholder SHA (`sha256:placeholder` on line 40). The acb-enrichment image does not exist on Docker Hub.

**Access Issue:**
- iad-ci.kubeconfig does NOT exist at `/home/coding/.kube/iad-ci.kubeconfig`
- Read-only proxy at `http://traefik-iad-ci.tail1b1987.ts.net:8001` works but cannot submit workflows
- No container runtime (docker/podman) available locally to build manually
- No Forgejo Actions workflow configured for automatic builds on push
- GitHub Actions is disabled per project policy

### What Needs to Happen

**Option 1: Obtain iad-ci kubeconfig (Recommended)**
1. Download kubeconfig from Rackspace Spot Console:
   - Navigate to iad-ci cluster
   - Generate kubeconfig for ServiceAccount `argocd-manager`
   - Save to `/home/coding/.kube/iad-ci.kubeconfig`
2. Submit the workflow:
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
3. The workflow will:
   - Build all ACB images including acb-enrichment
   - Run tests
   - Push images to Docker Hub (`ronaldraygun/acb-enrichment:<sha>`)
   - Update declarative-config with real image SHA via `update-declarative-config` step
   - Push changes to declarative-config repo
4. ArgoCD will sync the updated manifest to apexalgo-iad cluster

**Option 2: Configure Forgejo Actions webhook**
1. Create a workflow file in `.forgejo/workflows/` or `.gitea/workflows/`
2. Configure it to trigger on push to master
3. Workflow should submit the acb-build workflow to iad-ci via API

**Option 3: Manual Docker build (Last resort)**
1. Install container runtime on this machine
2. Configure Docker Hub credentials
3. Build image manually:
   ```bash
   docker build -f cmd/acb-enrichment/Dockerfile -t ronaldraygun/acb-enrichment:latest .
   docker push ronaldraygun/acb-enrichment:latest
   ```
4. Get image digest and update deployment manifest manually
5. Commit and push to declarative-config

## Current State (2026-06-04)
- **BLOCKER:** Missing iad-ci.kubeconfig for workflow submission
- **Image Status:** acb-enrichment image does not exist on Docker Hub
- **Dockerfile:** Verified correct
- **WorkflowTemplate:** Verified - `acb-images-build-workflowtemplate.yml` includes enrichment
- **Deployment:** Has placeholder SHA at line 40, needs real image
- **iad-ci Proxy:** Confirmed accessible at `http://traefik-iad-ci.tail1b1987.ts.net:8001` but read-only

## Verified Access Attempts (2026-06-04)
```bash
# iad-ci proxy exists but is read-only (devpod-observer SA)
$ kubectl --server=http://traefik-iad-ci.tail1b1987.ts.net:8001 create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
...
EOF
Error from server (Forbidden): User "system:serviceaccount:devpod-observer:devpod-observer" cannot create resource "workflows"

# No workflows with acb-images-build template found
$ kubectl --server=http://traefik-iad-ci.tail1b1987.ts.net:8001 get workflows -n argo-workflows
No resources found

# Kubeconfig files not present
$ ls ~/.kube/*.kubeconfig
ls: cannot access: No such file or directory
```

## Recommendation
Set up the iad-ci.kubeconfig file. This is a one-time infrastructure task that will unblock all future iad-ci workflow operations. The kubeconfig provides cluster-admin access to the CI/CD cluster where all Argo Workflows run.

## Resolution Path
1. **External Action Required**: Obtain iad-ci.kubeconfig from Rackspace Spot Console
2. Submit `acb-images-build` workflow to build enrichment image
3. Retrieve image SHA from completed workflow
4. Update deployment manifest in declarative-config
5. Push to declarative-config (ArgoCD syncs to apexalgo-iad)
