# Bead bf-4u5 — 6 acb-strategy-* pods ImagePullBackOff (ronaldraygun/acb-strategy-*:latest)

## Outcome: NOT COMPLETED — bead is stale (pre-decommission snapshot); left OPEN for human decision

The bead asks to build/push the 6 `ronaldraygun/acb-strategy-{name}` images and pin the
Deployments away from `:latest` so the pods reach Running. Investigation shows
**ai-code-battle was intentionally decommissioned on apexalgo-iad on Jul 21** — the very
Deployment manifests this bead references were **deleted from GitOps and never re-added**.
The 6 ImagePullBackOff pods are **un-pruned orphans** of that retired workload. Building and
pinning images would un-do a deliberate operator decision and revive a known-broken namespace.
No image was built, no manifest changed, no tag pinned. Needs a human decision (see *Decision
needed*). This is the same stale-snapshot pattern already documented for
[bf-1yj](bf-1yj.md) / bf-687 / bf-cn9 / bf-wsf (all left open).

The bead itself carries labels `deferred, failure-count:1` — a prior attempt already reached
this conclusion.

---

## Bottom line

- The bead's central premise — *"The manifests (declarative-config
  `k8s/apexalgo-iad/ai-code-battle/acb-strategy-*-deployment.yml`) reference
  `ronaldraygun/acb-strategy-<name>:latest`"* — is **false as of now**. Those manifests do not
  exist. `git ls-files | grep acb-strategy` in `declarative-config` returns **nothing**.
- Decommission commit **`0163324e`** (jedarden, **Tue Jul 21 17:46 EDT 2026**) —
  *"chore(apexalgo-iad): take down ai-code-battle — remove compute/CI/ingress … Retire the
  ai-code-battle deployment on apexalgo-iad (already non-functional: all 14 deployments 0/1,
  CreateContainerConfigError/ImagePullBackOff)."* — deleted the entire
  `k8s/apexalgo-iad/ai-code-battle/` directory (14 Deployments, 11 Services, IngressRoute,
  EventSource/Sensor, SealedSecrets, the `acb-armor-credentials` ExternalSecret, etc.).
- `git log -- 'k8s/apexalgo-iad/ai-code-battle/*'`: last touch is the deletion `0163324e`;
  **nothing re-added it.** Decommission is the current intent. (`declarative-config` HEAD
  today is `7657621b`; the dir is still absent.)
- Therefore the ImagePullBackOff state this bead captures is a snapshot from **before** the
  Jul 21 decommission. The never-built/never-pulled strategy images were the *reason* the
  workload was retired, not a fresh defect to patch.
- The only artifact the bead references that **does** still exist is the CI template
  `k8s/iad-ci/argo-workflows/acb-bots-build-workflowtemplate.yml` — a WorkflowTemplate lives
  in the CI namespace and was not part of the deleted deployment dir. Its continued presence
  does not mean the workload was revived.

## Evidence — GitOps source of truth (`declarative-config` main, HEAD `7657621b`)

```
$ ls k8s/apexalgo-iad/ai-code-battle/   → No such file or directory
$ git ls-files | grep acb-strategy      → (empty — no strategy manifests anywhere)
$ git log --oneline -1 -- 'k8s/apexalgo-iad/ai-code-battle/*'
0163324e chore(apexalgo-iad): take down ai-code-battle — remove compute/CI/ingress   (Jul 21)
```

## Evidence — live cluster (apexalgo-iad, ns `ai-code-battle`, read-only proxy)

All 6 named bots are `ImagePullBackOff` orphans, in a broadly broken namespace (only
`acb-schema-init` is Running):

| Pod | Status | Age |
|-----|--------|-----|
| acb-strategy-gatherer | ImagePullBackOff | 51m |
| acb-strategy-guardian | ImagePullBackOff | 13d |
| acb-strategy-hunter   | ImagePullBackOff | 51m |
| acb-strategy-random   | ImagePullBackOff | 51m |
| acb-strategy-rusher   | ImagePullBackOff | 51m |
| acb-strategy-swarm    | ImagePullBackOff | 13d |

Sibling breakage (same namespace): `acb-api`/`acb-evolver`/`acb-index-builder`/`acb-map-evolver`
`CreateContainerConfigError`, `acb-matchmaker` `CrashLoopBackOff` (39 restarts),
`acb-enrichment` `ImagePullBackOff`. This matches [bf-1yj](bf-1yj.md)'s "namespace broadly
broken" finding exactly. (Pod ages here are lower than the bead's "16h–16d" because the
controller has re-created some pods; the underlying ImagePullBackOff persists.)

## Evidence — the CI the bead wants me to fix is itself currently broken

Per the bead's step 1, I checked `acb-bots-build` history on iad-ci. It is **running 6 times
concurrently right now** (4m–29m old) and **none have completed**:

```
acb-bots-build-4fmzw   Running   12m      (and 5 siblings: gg96z, gzxrp, k72jf, p4llj, pjthr)
```

Inspecting `acb-bots-build-4fmzw`: it fans out ~20 parallel kaniko `build-<bot>` steps. The 6
strategy bots are among them (build-gatherer/guardian/hunter/random/rusher/swarm). It is
**resource-starved** — many `build-*(0)` pods are `Pending / Unschedulable` ("0/6 nodes
available: 1 Insufficient cpu, 3 Insufficient memory, 3 node(s) didn't match Pod's node
affinity") — and **`build-hunter` has already `Failed` (exit code 1)**.

So even setting the decommission aside: (a) the bots-build CI is currently looping and not
producing successful strategy images, and (b) this looks like a stuck/over-submitted run
(6 concurrent copies of a ~20-bot fan-out). Why 6 copies are running concurrently is itself
unexplained (the ai-code-battle EventSource/Sensor that would auto-trigger it was deleted in
`0163324e`). This is worth a separate look — but **fixing the CI would not help this bead**,
because there are no GitOps Deployment manifests left to receive a pinned tag.

## Why I did not build/pin the images

1. **It would contradict committed GitOps intent.** The operator retired this workload on
   Jul 21. Building images and re-deploying the strategy bots is the opposite of that recorded
   decision and revives a namespace that is broken for many other reasons (see live table +
   [bf-1yj](bf-1yj.md)).
2. **Step 3 of the bead is impossible as written.** There are no `acb-strategy-*-deployment.yml`
   manifests in `declarative-config` to switch from `:latest` to a `sha-<commit>` tag — they
   were deleted in `0163324e`. The `:latest` policy violation the bead cites existed in a file
   that no longer exists.
3. **The CI is not currently able to produce the images anyway** (resource-starved, looping,
   `build-hunter` failing) — and is concurrently running 6 times for unknown reasons.
4. **Reviving the namespace needs far more than 6 images.** Per [bf-1yj](bf-1yj.md), a real
   revive requires restoring all 14 Deployments, **all** missing secrets (postgres, R2 — which
   were corrupted per bf-4ur, armor/OpenBao, api, evolver, matchmaker, cloudflare), recovering
   cluster capacity (1 of 3 nodes was NotReady), and a fresh image build. Pinning 6 tags alone
   leaves the namespace just as dead.

## Decision needed (human)

**Branch A — decommission stands (most likely; consistent with [bf-1yj](bf-1yj.md)).** The real
task is *cleanup*, not image builds: (1) investigate/kill the 6 looping `acb-bots-build` runs
on iad-ci and stop whatever is re-submitting them (its EventSource was deleted, so the trigger
is unclear); (2) get ArgoCD on apexalgo-iad healthy so the `manifest-appset-apexalgo-iad`
ApplicationSet prunes the `ai-code-battle-ns-apexalgo-iad` app and its 14 orphaned Deployments
(ArgoCD's RO API was returning empty — see bf-1yj). Once pruned, this bead and its acceptance
criteria are moot — close as obsolete.

**Branch B — revive ai-code-battle on apexalgo-iad (intent changed).** Building/pinning the 6
strategy images is necessary but nowhere near sufficient — see the full restore list in
[bf-1yj](bf-1yj.md) (Branch B). Do not start here without confirming the decommission is
reversed.

## Cross-references

- **[bf-1yj](bf-1yj.md)** (open): primary decommission finding — `0163324e` deleted the whole
  dir; live pods are orphans; ArgoCD on apexalgo-iad unhealthy. This bead is a duplicate of
  that conclusion for the 6 strategy bots specifically.
- **bf-687 / bf-cn9 / bf-wsf** (open): same stale-bead pattern on the decommissioned workload.
- **bf-4ur**: secret/template review (R2 creds corrupted) — relevant only if Branch B is taken.
- Decommission commit: `declarative-config` `0163324e` (Jul 21 2026).
