# bf-58z — Resolve authoritative ai-code-battle cluster (apexalgo-iad vs iad-acb)

**Status:** RESOLVED. **Closed 2026-07-26.** The bead's premise (two live,
conflicting manifest trees in `declarative-config`) is **already resolved at
the manifest level** — both trees are gone. Authoritative cluster confirmed as
`apexalgo-iad`, now decommissioned. Doc fixes applied to `plan.md` and
`DEPLOYMENT.md`.

## Question the bead asked

`declarative-config` appeared to hold two manifest trees for the same workload
(`k8s/apexalgo-iad/ai-code-battle/` and `k8s/iad-acb/ai-code-battle/`), and the
bf-2ws OOMKill investigation notes referenced a `iad-acb.kubeconfig` to describe
pods whose node names match the ones live on apexalgo-iad. Which cluster is
authoritative, and which tree is stale?

## Answer — authoritative cluster: `apexalgo-iad`

`apexalgo-iad` was the **only** cluster ever to run live `ai-code-battle` pods.
`iad-acb` was a **separate, earlier dedicated-cluster attempt that was retired
and deleted** — it was never a second live copy, so there was no real
split-brain. The "duplication" was an abandoned cluster, not a divergent twin.

This is **not** one of the acb-* "fix the pods so they reach Running" beads that
must be left open: this is an audit/documentation bead, and all three
acceptance criteria are addressed below. The underlying workload was
decommissioned on `apexalgo-iad` on 2026-07-21 (separate event), which makes the
authoritative-cluster question moot going forward.

## Evidence

### 1. `declarative-config` git history (HEAD `388e6125`)

- `k8s/apexalgo-iad/ai-code-battle/` → **ABSENT**. Last touch was the
  decommission commit `0163324e` ("chore(apexalgo-iad): take down ai-code-battle
  — remove compute/CI/ingress"), 2026-07-21. It deleted 14 Deployments,
  Services, IngressRoute, EventSource/Sensor, SealedSecrets, and the
  `acb-armor-credentials` ExternalSecret.
- `k8s/iad-acb/` → **ABSENT on disk and untracked**. `git ls-files | grep iad-acb`
  returns nothing at HEAD. The iad-acb tree existed historically (e.g.
  `k8s/iad-acb/ai-code-battle/restore-verifier.yaml`, `k8s/iad-acb/drawrace/*`)
  and was removed in commit `53fc54a1` ("chore(iad-acb): remove deleted cluster
  — manifests, ArgoCD wiring, docs"), preceded by `ab908bf8` ("docs(iad-acb):
  drop deleted iad-acb from cluster-list comments"). The commit message calls
  iad-acb a **"deleted cluster"** — i.e. the cluster itself was torn down, not
  just the manifests.
- **Conclusion:** at `declarative-config` HEAD today, **neither tree exists**.
  The bead's split-brain premise no longer holds; criterion #3 (remove/mark the
  stale dir deprecated) is already satisfied by `53fc54a1` + `0163324e`.

### 2. Live cluster state (verified 2026-07-26)

- **apexalgo-iad** (`kubectl --server=http://traefik-apexalgo-iad:8001 get pods
  -n ai-code-battle`): the `ai-code-battle` namespace is present and populated
  with **orphaned, failing pods** — `CreateContainerConfigError`,
  `ImagePullBackOff`, `CrashLoopBackOff`, plus one `acb-schema-init` Running.
  This is the decommission's leftover (ArgoCD on this cluster is unhealthy, so
  the `manifest-appset-apexalgo-iad` ApplicationSet never pruned them). These
  are pre-decommission snapshots, **not** fresh defects — see
  `notes/bf-1yj.md`.
- **iad-acb** (`~/.kube/iad-acb.kubeconfig`, server
  `http://traefik-iad-acb:8001`): **completely unreachable** —
  `context deadline exceeded` / `request canceled while waiting for connection`.
  The `traefik-iad-acb` Tailscale endpoint is gone, consistent with the cluster
  having been deleted (`53fc54a1`). There are **no** live `ai-code-battle` pods
  on iad-acb because there is no cluster.

### 3. The bf-2ws cluster-confusion symptom (explained, now moot)

The bf-2ws OOMKill notes (`notes/bf-2ws-final-status.md`) describe the
`acb-index-builder` pod using `iad-acb.kubeconfig`, while the pod's
`prod-instance-*` node names match the pods currently live on **apexalgo-iad**.
This was a real symptom of iad-acb being a confusingly-named separate cluster
that the kubeconfig pointed at. It is now **moot**: iad-acb is deleted and
unreachable, and the workload it was describing is decommissioned. The remediation
steps in those notes should not be followed against iad-acb (dead endpoint);
any live pods of interest are on apexalgo-iad via the proxy in CLAUDE.md.

### 4. ArgoCD Application registration — NOT verifiable from this host

The bead asked for the authoritative cluster backed by ArgoCD Application
registration. The read-only ArgoCD API
(`https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications`)
returned **HTTP 000 (connection failed)** from this host, and per prior
investigation (`notes/bf-1yj.md`) returns an empty body even when reachable —
ArgoCD on ardenone-manager is unhealthy. The authoritative-cluster conclusion
above is therefore backed by live-pod evidence + declarative-config git history
rather than a live ArgoCD Application listing.

**To complete criterion #1 from a host with ArgoCD reachability (the ex44 /
primary dev host, not lab), run:**

```bash
curl -sk https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications \
  | jq '.items[] | select((.spec.source.path|test("ai-code-battle")) or (.metadata.name|test("acb")))'
```

Expect: no live Application targeting either `k8s/apexalgo-iad/ai-code-battle/`
or `k8s/iad-acb/ai-code-battle/` (both removed); the `manifest-appset-*`
ApplicationSets that once covered them no longer match any path. The only
ai-code-battle "application" left in GitOps terms is the static Cloudflare
Pages site, which is not ArgoCD-managed.

## Fixes applied (this bead)

All four edits are surgical decommission/clarification markers — the underlying
architecture prose is left intact as design documentation.

- **`docs/plan/plan.md` §2 (System Architecture):** added a DECOMMISSIONED
  banner stating apexalgo-iad was taken down 2026-07-21 (`0163324e`) and the
  text below is the designed architecture, not current live state.
- **`docs/plan/plan.md` §9 (Deployment & Infrastructure):** added a
  DECOMMISSIONED banner noting both manifest trees are removed from
  `declarative-config`.
- **`docs/plan/plan.md` §9.2 (Historical note):** rewrote the doubly-stale
  claim — the old note said iad-acb "exists as a stale tree" (false: removed in
  `53fc54a1`) and that "active resources are consolidated on apexalgo-iad"
  (false: decommissioned `0163324e`). Replaced with the accurate resolution and
  the authoritative-cluster statement.
- **`DEPLOYMENT.md`:** added a DECOMMISSIONED banner at the top identifying
  apexalgo-iad as authoritative and iad-acb as the retired separate cluster
  (`53fc54a1`).

## Not changed (out of bead scope, recorded for awareness)

- `IAD-ACB-OPENBAO-FIX.md`, `IAD-ACB-R2-CREDENTIALS-FIX.md`, `R2_ACCESS_KEY_SOURCE.md`,
  and the `fix-iad-acb-*.sh` scripts: historical runbooks/scripts targeting the
  now-deleted iad-acb cluster. They are inherently historical and reference
  `declarative-config/k8s/iad-acb/...` paths that no longer exist. Left as-is;
  a future cleanup pass could archive them.
- `PROGRESS.md` (lines 50/59/321): dated changelog entries stating manifests
  "live in" `declarative-config/k8s/apexalgo-iad/ai-code-battle/`. Changelog
  content — left as-is.

## Decision handoff

No operator action is required for this bead. The authoritative-cluster
ambiguity is resolved and documented. The remaining open items belong to other
beads:

- **Get apexalgo-iad ArgoCD healthy** so its ApplicationSet prunes the orphaned
  `ai-code-battle` pods (Branch A of `notes/bf-1yj.md`).
- **Full revive** of the workload, if ever desired (Branch B of `notes/bf-1yj.md`):
  needs all secrets (incl. corrupted R2 per bf-4ur), cluster capacity, and fresh
  image builds.

Related: `notes/bf-1yj.md` (canonical decommission investigation),
`notes/bf-687.md`, `notes/bf-cn9.md`, `notes/bf-wsf.md`, `notes/bf-5ll.md`,
`notes/bf-4u5.md` (stale acb-* pod beads left open).
