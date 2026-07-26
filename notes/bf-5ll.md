# Bead bf-5ll — 11 of 21 strategy bots have no K8s Deployment/Service manifests

## Outcome: NOT COMPLETED — bead premise is stale; contradicts a deliberate decommission. Bead left OPEN for a human decision.

The bead asks to add `acb-strategy-{name}-deployment.yml` + Service manifests for 11 bots
(farmer, coordinator, economist, leader-targeter, nomad, opportunist, pacifist, phalanx,
scout, siege, zone-driver) into `declarative-config/k8s/apexalgo-iad/ai-code-battle/`,
pinned to a sha tag. Independent verification shows **ai-code-battle was intentionally
decommissioned on apexalgo-iad 5 days ago** and the target directory — including the 6
existing strategy manifests this bead takes as its baseline — was **deleted**. Adding the
11 manifests would recreate a deliberately-retired, broadly-broken namespace: the opposite
of the recorded GitOps decision. No manifests were written, no file in `declarative-config`
was changed. This is the same situation the companion beads **[[bf-1yj]]**, **[[bf-wsf]]**,
**[[bf-cn9]]**, **[[bf-687]]**, and **[[bf-1bh]]** resolved the same day.

---

## Bottom line

- **The bead's baseline no longer exists.** The bead (verified 2026-07-19) takes as its
  starting point *"only 6 of 21 bots have manifests (`acb-strategy-{gatherer,guardian,hunter,
  random,rusher,swarm}-deployment.yml`)"* in `k8s/apexalgo-iad/ai-code-battle/`. As of the
  2026-07-21 decommission that count is **0 of 21**: the entire directory is gone.
- **Decommission is the committed intent.** `declarative-config` `main` (HEAD `72239248`)
  has **no** `k8s/apexalgo-iad/ai-code-battle/` directory. Commit **`0163324e`** (jedarden,
  **Tue Jul 21 17:46 EDT**) — *"chore(apexalgo-iad): take down ai-code-battle — remove
  compute/CI/ingress … Retire the ai-code-battle deployment on apexalgo-iad (already
  non-functional: all 14 deployments 0/1, CreateContainerConfigError/ImagePullBackOff)."* —
  deleted the whole namespace dir. The deletion explicitly includes the 6 strategy manifests
  this bead builds on:
  ```
  .../acb-strategy-gatherer-deployment.yml  | 86 -
  .../acb-strategy-guardian-deployment.yml  | 86 -
  .../acb-strategy-hunter-deployment.yml    | 86 -
  .../acb-strategy-random-deployment.yml    | 86 -
  .../acb-strategy-rusher-deployment.yml    | 86 -
  .../acb-strategy-swarm-deployment.yml     | 86 -
  ```
  along with 14 Deployments, 11 Services, the IngressRoute, EventSource/Sensor + 3
  WorkflowTemplates, SealedSecrets, and the `acb-armor-credentials` ExternalSecret.
  `git log -- 'k8s/apexalgo-iad/ai-code-battle/*'`: last touch is that deletion;
  **nothing re-added it.**
- **No surviving deploy target exists anywhere.** A repo-wide grep for
  `ronaldraygun/acb-strategy` in `declarative-config/k8s` returns **zero** Deployments
  outside the iad-ci build templates. The migration target named in the deployment headers —
  `k8s/iad-acb/ai-code-battle/` — is also gone: the **`iad-acb` cluster itself was deleted**
  (`53fc54a1 chore(iad-acb): remove deleted cluster`), and dropped from the cluster-list
  comments (`ab908bf8`). There is no namespace, on any cluster, where these 11 manifests
  could land.
- **Therefore the bead's premise is a snapshot from before the Jul 21 decommission**, not a
  fresh gap. Its acceptance criteria ("11 bots have manifests committed to
  `k8s/apexalgo-iad/ai-code-battle/`") cannot be met without **recreating a directory the
  operator deliberately deleted** — i.e. reversing the decommission.

## What is still true from the bead (and why it doesn't change the outcome)

- **Images still build.** The bead's framing — "missing-manifest gap, not a missing-image
  gap" — still holds on the image side: `k8s/iad-ci/argo-workflows/acb-bots-build-
  workflowtemplate.yml` still has a `build-<bot>` step for **all 11** bots (farmer,
  coordinator, economist, leader-targeter, nomad, opportunist, pacifist, phalanx, scout,
  siege, zone-driver — all confirmed present). So CI keeps producing the images. But with no
  Deployment referencing them and no deploy namespace, the built images have no consumer.
- **The `:latest` caveat (bf-4u5) is doubly moot.** The bead correctly warns to pin a sha
  tag, not `:latest` (per **[[bf-4u5]]**). With the Deployments deleted there is no image
  reference to tag at all; bf-4u5 itself is now stale for the same reason (the original 6
  manifests it flags are gone).

## Why I did not write the 11 manifests

1. **It would contradict committed GitOps intent.** Per `CLAUDE.md`, all cluster writes go
   through `declarative-config` GitOps + ArgoCD sync. Committing 11 new manifests into a
   deleted namespace dir would revive a deliberately-retired workload — the recorded decision
   (`0163324e`) was to retire it because it was *"already non-functional."* Reversing that
   without an operator decision is exactly the kind of hard-to-undo, intent-contradicting
   change to surface, not silently apply.
2. **There is no place to put them.** Both candidate directories are gone:
   `k8s/apexalgo-iad/ai-code-battle/` (deleted `0163324e`) and `k8s/iad-acb/ai-code-battle/`
   (cluster deleted `53fc54a1`). Inventing a new path would be an unrequested
   re-architecture, not this bead's task.
3. **The dependency bead is equally stale.** bf-511 builds on **[[bf-2vf]]** (kamikaze/
   defender/assassin/raider, still **open**) for its manifest convention — but bf-2vf targets
   the same deleted directory, so its 4 manifests are in the same boat. There is no
   established in-tree convention left to mirror.
4. **Even if written, they would not run.** Per [[bf-1yj]]/[[bf-687]], the live namespace is
   broadly broken (nodes CPU-saturated / one NotReady; `sealed-secrets-web` CrashLoopBackOff;
   `forgejo.ardenone.com` unresolvable from cluster nodes). The 11 bots would join the
   existing orphans in `ImagePullBackOff`/`Pending`, not reach the ladder.

## Decision needed (human) — owned by [[bf-1yj]]

This bead's resolution is downstream of the decommission-vs-revive decision already tracked
by **[[bf-1yj]]** (still **open/in_progress**):

**Branch A — decommission stands (most likely, per [[bf-1yj]]).** Then the real task is
*cleanup*, not new manifests: get ArgoCD on apexalgo-iad healthy so the ApplicationSet prunes
the `ai-code-battle` orphan app and its Deployments. Once pruned, bf-5ll's acceptance
criteria are **moot** — there should be **no** strategy-bot manifests — and this bead should
be **closed as obsolete** alongside [[bf-1yj]].

**Branch B — revive ai-code-battle (intent changed).** Then authoring these 11 manifests is
a sub-step of the full human-directed restore (recover `k8s/apexalgo-iad/ai-code-battle/`
from `0163324e^`, recreate **all** secrets, recover node capacity, fix sealed-secrets,
resolve the registry DNS blocker, ship fresh images — see [[bf-1yj]] Branch B). The 11 new
manifests would be written then, against a live convention; doing it now, in isolation,
would be premature.

Left **open** so the work item survives a potential Branch-B revive and so the operator
adjudicates; a retry that re-derives "stale" is harmless given this note.

## Acceptance criteria — cannot be met without reviving a decommissioned workload

| # | Criterion | Result |
|---|-----------|--------|
| 1 | All 11 bots have a Deployment + Service manifest in `k8s/apexalgo-iad/ai-code-battle/` | **Not done.** Target dir was deleted (`0163324e`); writing them recreates a retired namespace. |
| 2 | Manifests reference a pinned sha tag, not `:latest` | **Moot.** No manifests exist; the original 6 the `:latest` warning (bf-4u5) targets are also gone. |
| 3 | `git push` to origin (GitOps, no direct kubectl) | **Only this doc note pushed.** No manifests committed anywhere. |

## Cross-references

- **[[bf-1yj]]** — first bead to establish the Jul 21 decommission; owns the Branch-A/ Branch-B
  decision. See `notes/bf-1yj.md`.
- **[[bf-687]]** — companion bead **closed** stale the same day (enrichment registry secret).
- **[[bf-wsf]]**, **[[bf-cn9]]**, **[[bf-1bh]]** — sibling beads left open stale (same decommission).
- **[[bf-2vf]]** — dependency bead (kamikaze/defender/assassin/raider manifests), still **open**,
  equally stale (same deleted dir).
- **[[bf-4u5]]** — the `:latest` issue on the original 6 manifests; now stale (manifests deleted).
- Decommission commit: `declarative-config` `0163324e` (Jul 21 2026); cluster-dir removal `53fc54a1`.
