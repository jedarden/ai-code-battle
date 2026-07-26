# Bead bf-2vf — Add Phase 13 combat bots to K8s ladder (kamikaze, defender, assassin, raider)

## Outcome: NOT COMPLETED — bead is stale (pre-decommission snapshot); left OPEN for human decision

The bead asks to add K8s Deployment+Service manifests for 4 combat bots (kamikaze, defender,
assassin, raider) under `declarative-config/k8s/apexalgo-iad/ai-code-battle/`, plus CI
WorkflowTemplates to build their images, following the bf-3dv pattern. Investigation shows
**ai-code-battle was intentionally decommissioned on apexalgo-iad on Jul 21** — the entire
target directory `k8s/apexalgo-iad/ai-code-battle/` was **deleted from GitOps and never
re-added**, and the dependency bead bf-3dv's own manifests were deleted in the same commit.
Creating the combat-bot Deployments + CI would un-do a deliberate operator decision and revive
a known-broken namespace. No manifest was created, no CI template added, no image built. Needs
a human decision (see *Decision needed*). This is the same stale-snapshot pattern already
documented for [bf-1yj](bf-1yj.md) / bf-4u5 / bf-687 / bf-cn9 / bf-wsf / bf-5ll (all left open).

---

## Bottom line

- **The target directory does not exist.** `git ls-files 'k8s/apexalgo-iad/ai-code-battle/*'` in
  `declarative-config` (HEAD `388e6125`) returns **0 files**. There is no place to drop the
  requested `acb-strategy-{kamikaze,defender,assassin,raider}-deployment.yml` files without first
  reviving a workload the operator deliberately retired.
- Decommission commit **`0163324e`** (jedarden, **Tue Jul 21 17:46 EDT 2026**) —
  *"chore(apexalgo-iad): take down ai-code-battle — remove compute/CI/ingress … Retire the
  ai-code-battle deployment on apexalgo-iad (already non-functional: all 14 deployments 0/1,
  CreateContainerConfigError/ImagePullBackOff)."* — deleted the entire
  `k8s/apexalgo-iad/ai-code-battle/` directory (14 Deployments, 11 Services, IngressRoute,
  EventSource/Sensor, SealedSecrets incl. `acb-bot-secrets`, the `acb-armor-credentials`
  ExternalSecret, etc.).
- **The dependency bead's output is gone too.** bf-3dv's 6 manifests
  (`acb-strategy-{random,farmer,gatherer,guardian,hunter,rusher,swarm}-deployment.yml`) were all
  among the files deleted in `0163324e`. So the "pattern established" by bf-3dv no longer exists
  in GitOps — there is no in-tree manifest to copy from.
- **None of these 4 combat bots ever had `acb-strategy-*` manifests in declarative-config.**
  Confirmed against the deletion commit: it only removed gatherer/guardian/hunter/random/
  rusher/swarm — no kamikaze/defender/assassin/raider. So bf-2vf was filed as forward "Phase 13"
  work while ai-code-battle was still active; the whole workload was then retired 5 days before
  today, rendering the bead moot.
- `git log -- 'k8s/apexalgo-iad/ai-code-battle/*'`: last touch is the deletion `0163324e`;
  **nothing re-added it.** Decommission is the current intent.
- The only artifacts the bead references that *do* still exist are (a) the 4 combat-bot
  **Dockerfiles** in this repo (`bots/{kamikaze,defender,assassin,raider}/Dockerfile`) and
  (b) staging manifests under this repo's `manifests/acb-bots/`
  (`bot-{kamikaze,defender,assassin,raider}-{deployment,service,externalsecret}.yml`). Those are
  **staging/source files, not the GitOps source of truth** (see [bf-1yj](bf-1yj.md) note on
  `manifests/`). Their existence does not mean the workload was revived or that GitOps Deployments
  should be (re-)created.

## Why I did not create the manifests / CI

1. **It would contradict committed GitOps intent.** The operator retired ai-code-battle on
   apexalgo-iad on Jul 21. Re-creating 4 combat-bot Deployments (plus the `acb-bot-secrets`
   SealedSecret they consume, which was also deleted in `0163324e`) is the opposite of that
   recorded decision and revives a namespace that is broken for many other reasons — see the
   live-cluster table in [bf-1yj](bf-1yj.md) / [bf-4u5](bf-4u5.md) (1 of 3 nodes was NotReady;
   api/worker `CreateContainerConfigError`; matchmaker `CrashLoopBackOff`; secrets corrupted per
   bf-4ur; ArgoCD on the cluster unhealthy and failing to prune orphans).
2. **The dependency the bead rests on is moot.** bf-3dv "established the base bot manifest
   pattern" in `k8s/apexalgo-iad/ai-code-battle/`, but every manifest it produced was deleted in
   `0163324e`. There is no in-repo pattern left to follow, and re-creating the pattern from
   scratch is exactly the revive the decommission forecloses.
3. **Adding the CI is moot and the CI is currently broken anyway.** The bead's CI option
   (`k8s/iad-ci/argo-workflows/`) lives in the CI namespace and was not part of the deleted
   deployment dir — but building 4 more bot images for a dead workload only feeds orphans. Per
   [bf-4u5](bf-4u5.md), the existing `acb-bots-build` WorkflowTemplate was running **6×
   concurrently on the resource-starved iad-ci cluster** with `build-hunter` already `Failed` and
   nothing completing. Adding combat bots to that fan-out does not help, and the consuming
   Deployments don't exist to pin tags to.
4. **A real revive needs far more than 4 manifests.** Per [bf-1yj](bf-1yj.md) Branch B, reviving
   ai-code-battle requires restoring all 14 Deployments + Services + IngressRoute + EventSource/
   Sensor, **all** missing secrets (postgres, R2 — corrupted per bf-4ur — armor/OpenBao, api,
   evolver, matchmaker, cloudflare, and the `acb-bot-secrets` these bots need), recovered cluster
   capacity, and fresh image builds. Four combat-bot Deployments alone leave the namespace just as
   dead.

## Decision needed (human)

**Branch A — decommission stands (most likely; consistent with [bf-1yj](bf-1yj.md) and 5 sibling
beads).** Then this bead is obsolete: there is nothing to add to a directory that was
intentionally deleted. The real task is *cleanup*, not new manifests — (1) get ArgoCD on
apexalgo-iad healthy so the `manifest-appset-apexalgo-iad` ApplicationSet prunes the
`ai-code-battle-ns-apexalgo-iad` app and its orphaned Deployments (ArgoCD's RO API was returning
empty — see [bf-1yj](bf-1yj.md)); (2) if a revive is genuinely wanted later, restore the whole
directory from `0163324e^` (or re-sync the staging `manifests/` here) as one deliberate act, then
add combat bots as part of it. Once intent is confirmed, this bead and its acceptance criteria are
moot — close as obsolete.

**Branch B — revive ai-code-battle on apexalgo-iad (intent changed).** Do not start by adding 4
combat-bot manifests — see the full restore list in [bf-1yj](bf-1yj.md) Branch B. The combat bots
would be the *last* addition, after the base workload is healthy and `acb-bot-secrets` is
restored. Do not start here without confirming the decommission is reversed.

## Cross-references

- **[bf-1yj](bf-1yj.md)** (open): primary decommission finding — `0163324e` deleted the whole
  dir; live pods are orphans; ArgoCD on apexalgo-iad unhealthy. Canonical investigation.
- **[bf-4u5](bf-4u5.md)** (open): 6 strategy bots ImagePullBackOff — same stale pattern; also
  documents the broken/looping `acb-bots-build` CI on iad-ci.
- **bf-3dv** (the cited dependency): its manifests were deleted in `0163324e` — the pattern is
  no longer in GitOps.
- **bf-687 / bf-cn9 / bf-wsf / bf-5ll** (open): same stale-bead pattern on the decommissioned
  workload.
- **bf-4ur**: secret/template review (R2 creds corrupted) — relevant only if Branch B is taken.
- Decommission commit: `declarative-config` `0163324e` (Jul 21 2026).
