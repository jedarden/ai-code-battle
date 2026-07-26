# Bead bf-cn9 — acb-armor-credentials ExternalSecret absent from apexalgo-iad

## Outcome: NOT COMPLETED — bead premise is stale; contradicts a deliberate decommission that is still the current GitOps intent. Bead left OPEN for a human decision.

The bead asks to make the `acb-armor-credentials` ExternalSecret exist (and reach
`SecretSynced=True`) in `ai-code-battle` on apexalgo-iad so `acb-worker` and
`acb-index-builder` progress past `CreateContainerConfigError`. Independent
verification shows **ai-code-battle was intentionally decommissioned on
apexalgo-iad on Jul 21 2026**, and the ExternalSecret was **deliberately deleted**
in that same decommission commit — it was not "never applied/synced" as the bead
diagnoses. Restoring it would revive a workload the operator deliberately retired.
No manifest was changed, no credential minted, no ExternalSecret created. This is
the **same situation** as beads **bf-1yj** and **bf-wsf** (both still `in_progress`,
see `notes/bf-1yj.md` and `notes/bf-wsf.md`), resolved the same way.

---

## Bottom line

- The bead's premise is a **misdiagnosis**. It states *"the manifest was never
  applied/synced by ArgoCD."* In fact the manifest **was** applied, then
  **deliberately deleted**. Decommission commit **`0163324e`** (jedarden,
  **Tue Jul 21 17:46 EDT**) in `declarative-config` removed the entire
  `k8s/apexalgo-iad/ai-code-battle/` directory, including
  `acb-armor-credentials-externalsecret.yml` (−35 lines, status `D`). The commit
  message states the rationale explicitly:
  *"Removing the acb-armor-credentials ExternalSecret stops syncing the in-cluster
  [Secret]…"* — i.e. its absence is **intended**, not a sync failure.
- **The decommission still stands at remote HEAD.** I fetched `declarative-config`
  and verified against `origin/main` (`7657621b`, 10 commits ahead of where prior
  beads investigated at `72239248`): `git ls-tree -d origin/main
  k8s/apexalgo-iad/ai-code-battle` → **empty**. Nothing has re-added it. The
  decommission is the current, committed intent — not stale local state.
- **Data was intentionally preserved**, per `0163324e`: CNPG postgres
  (`k8s/apexalgo-iad/cnpg/database-ai-code-battle.yml`, cluster `cnpg-apexalgo`),
  B2 bucket `armor-apexalgo` (~366 MB replays/thumbnails/cards), and — directly
  answering the bead's Fix-step-2 — **OpenBao `rs-manager/iad-acb/armor`, preserved
  as "the bucket's MEK."** So the OpenBao path the bead asks me to confirm is
  intentionally kept; I did not query the vault directly because (a) the decommission
  commit documents it as preserved, and (b) the conclusion is invariant to its
  contents.
- The live `acb-worker` / `acb-index-builder` pods (and all 14 sibling deployments)
  are **un-pruned orphans**: their Deployment manifests no longer exist in GitOps,
  so ArgoCD no longer manages them. They persist only because ArgoCD on apexalgo-iad
  is **unhealthy** — its read-only API
  (`https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications`)
  returned an **empty body** during this investigation, so the `manifest-appset-apexalgo-iad`
  ApplicationSet (`prune: true, selfHeal: true, allowEmpty: true`) never pruned them.
- Therefore the bead's cited symptom ("secret acb-armor-credentials not found for
  20h+") is the *expected state of a retired workload's orphan pods*, not a fresh
  defect to patch.

## Evidence — live cluster (apexalgo-iad, ns `ai-code-battle`, read-only proxy)

- `kubectl … get externalsecret -n ai-code-battle` → **No resources found.** (ExternalSecret
  gone, as the decommission intends.)
- Namespace is broadly broken, all 14 deployments 0 ready:
  `CreateContainerConfigError` (api, worker ×2, evolver, **index-builder**, map-evolver),
  `ImagePullBackOff`/`ErrImagePull` (enrichment ×2, gatherer, guardian 13d, hunter,
  random, rusher, swarm 13d), `CrashLoopBackOff` (matchmaker ×36). Only
  `acb-schema-init` is Running (9d). Deployments are 39–123 days old — they predate
  the Jul 21 decommission and are orphans, not actively-reconciled workloads.
- This matches bf-1yj's and bf-wsf's snapshots (taken 5 days earlier); nothing has
  recovered. The `acb-worker` pod the bead names (`bf5bfdb98-fvn4q`) has been
  replaced by newer orphan replicas (`-6wz9s` 166m, `-w4gn8` 8d), all still
  `CreateContainerConfigError`.

## Why I did not restore / create the ExternalSecret

1. **It would contradict committed GitOps intent.** Per `CLAUDE.md`, *all cluster
   writes go through `jedarden/declarative-config`* — never direct `kubectl`. The
   operator retired this workload on Jul 21; restoring the ExternalSecret in
   GitOps means re-adding the entire decommissioned dir, which is the opposite of
   the recorded decision. The bead's own diagnosis ("never applied") would have me
   re-apply something that was *intentionally removed*.
2. **No write path exists anyway.** The apexalgo-iad proxy is **read-only RBAC**
   (`CLAUDE.md`: "cannot create, delete, or modify resources") — a direct create is
   not possible from this server. The only legitimate path is human-authored GitOps.
3. **The ExternalSecret alone would not unblock the pods.** The namespace is broadly
   failed (every deployment 0/1; multiple distinct failure modes; 1 of 3 nodes was
   NotReady per bf-1yj). Reviving requires the full Branch-B effort below, not one
   ExternalSecret.
4. **The acceptance criteria are unsatisfiable without reversing the decommission:**
   ExternalSecret `SecretSynced=True`, pods past `CreateContainerConfigError` — all
   presuppose a live workload that was deliberately taken down.

## Decision needed (human) — same fork as bf-1yj / bf-wsf

**Branch A — decommission stands (most likely).** Then the real task is *cleanup*,
not a secret: get ArgoCD on apexalgo-iad healthy so the ApplicationSet prunes the
orphaned `ai-code-battle` app and its 14 Deployments (the git-dir generator has
`prune: true, selfHeal: true, allowEmpty: true`, so deleting the dir *should* have
pruned them — ArgoCD's RO API is unreachable, consistent with it being unhealthy).
Once pruned, bf-cn9 / bf-1yj / bf-wsf and their acceptance criteria are all moot —
close as obsolete. No action on `acb-armor-credentials` is wanted or needed.

**Branch B — revive ai-code-battle on apexalgo-iad (intent changed).** Then restoring
`acb-armor-credentials` is necessary but not sufficient. The full restore (per bf-1yj /
bf-wsf) is:
1. Restore `k8s/apexalgo-iad/ai-code-battle/` to `declarative-config` (recover from
   `0163324e^`), or re-sync from `manifests/` in this repo (note: those are staging
   files; the deployment header points at `k8s/iad-acb/ai-code-battle/` but `iad-acb`
   was itself removed — `53fc54a1`, cluster deleted).
2. Recreate **all** secrets, not just armor: `acb-app-credentials-acb-app`,
   `acb-postgres-credentials`, `acb-r2-credentials` (R2 creds were **corrupted** —
   per `R2_ACCESS_KEY_SOURCE.md`/bf-4ur, endpoint/keys swapped), `acb-api-secrets`,
   `acb-evolver-secrets`, `acb-matchmaker-secrets`, plus `acb-cloudflare-api-token`.
   The `acb-armor-credentials` ExternalSecret (OpenBao `rs-manager/iad-acb/armor`,
   keys `bucket`/`auth-access-key`/`auth-secret-key`) can be re-added from
   `0163324e^` since its source data (B2 bucket + OpenBao MEK) was preserved.
   Apexalgo-iad secret pattern = **kubeseal SealedSecret** (controller
   `sealed-secrets-apexalgo-iad`, ns `sealed-secrets`); note the `sealed-secrets-web`
   pod was in `CrashLoopBackOff` per bf-1yj.
3. Recover cluster capacity (NotReady node / resize) so pods can schedule.
4. Roll fresh images (`ronaldraygun/acb-worker`, `ronaldraygun/acb-index-builder` —
   the latter's compile break is fixed per bf-16k `724f1d9`, but the image is not yet
   built/pushed; Deployment references a stale digest `@sha256:88db4dd2…`).

This requires human GitOps authorship + dashboard credential access; it is not a
headless single-ExternalSecret restore.

## Cross-references

- **bf-1yj** (`in_progress`): identical finding for `acb-cloudflare-api-token` /
  acb-index-builder. See `notes/bf-1yj.md`.
- **bf-wsf** (`in_progress`): identical finding for `acb-api` /
  `acb-app-credentials-acb-app` missing `uri`. See `notes/bf-wsf.md`.
- **bf-16k** (closed): `acb-index-builder` compile fix — binary builds; image not yet shipped.
- **bf-4ur** (prior): secret/template review; flagged R2 cred corruption.
- Decommission commit: `declarative-config` `0163324e` (Jul 21 2026); stands at
  `origin/main` `7657621b`.
- Preserved data: CNPG manifest `k8s/apexalgo-iad/cnpg/database-ai-code-battle.yml`,
  B2 bucket `armor-apexalgo`, OpenBao `rs-manager/iad-acb/armor` (MEK).
