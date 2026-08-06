# Bead bf-1yj — acb-cloudflare-api-token secret / acb-index-builder stuck

## Outcome: NOT COMPLETED — bead is stale; flagged for human decision. Bead left OPEN.

The bead asks to mint the `acb-cloudflare-api-token` secret so `acb-index-builder`
can run. Investigation shows **ai-code-battle was intentionally decommissioned on
apexalgo-iad 5 days ago** and the live pods are **un-pruned orphans**. Creating the
secret would un-do that decommission and revive a workload the operator deliberately
retired. No credential was minted, no manifest changed. This needs a human decision
(see *Decision needed*).

---

## Bottom line

- The GitOps source of truth (`declarative-config` `main`, HEAD `72239248`) has **no**
  `k8s/apexalgo-iad/ai-code-battle/` directory and **no** ArgoCD app for ai-code-battle.
  Commit **`0163324e`** (jedarden, **Tue Jul 21 17:46 EDT**) deleted it:
  *"chore(apexalgo-iad): take down ai-code-battle — remove compute/CI/ingress … Retire
  the ai-code-battle deployment on apexalgo-iad (already non-functional: all 14
  deployments 0/1, CreateContainerConfigError/ImagePullBackOff)."* It deleted all 14
  Deployments, 11 Services, IngressRoute, EventSource/Sensor + 3 WorkflowTemplates,
  SealedSecrets, the `acb-armor-credentials` ExternalSecret — and explicitly
  `acb-cloudflare-api-token-secret.yml.template` (58 lines). Data (CNPG postgres, B2
  bucket `armor-apexalgo-iad`, OpenBao MEK) was intentionally **preserved**.
- `git log -- 'k8s/apexalgo-iad/ai-code-battle/*'`: last touch is the deletion
  `0163324e`; **nothing re-added it.** Decommission is the current intent.
- The live `acb-index-builder` pod (and all 14 sibling deployments) are **orphans** that
  ArgoCD failed to prune. The `manifest-appset-apexalgo-iad` ApplicationSet is a git-dir
  generator over `k8s/apexalgo-iad/*` with `prune: true`, `selfHeal: true`,
  `allowEmpty: true`, foreground propagation — deleting the dir *should* have removed the
  app and pruned its resources. It didn't, which lines up with ArgoCD on this cluster
  being unhealthy (its read-only API `https://argocd-ro-ardenone-manager-ts…/api/v1/applications`
  returned an **empty body** during this investigation).
- Therefore the bead's premise ("stuck 9+ days, mint the secret so it runs") is a
  snapshot from **before** the Jul 21 decommission. The CreateContainerConfigError it
  cites was the *reason* the workload was retired, not a fresh defect to patch.

## Evidence — live cluster (apexalgo-iad, ns `ai-code-battle`, read-only proxy)

- Nodes: **1 of 3 NotReady** (`prod-instance-17826304223870832`); the 2 Ready nodes are
  CPU-saturated → most acb pods `Pending` (`FailedScheduling: 0/3 nodes available …
  Insufficient cpu`). Namespace is broadly broken:
  `Pending` (enrichment, evolver, **index-builder**, map-evolver, gatherer, hunter,
  random, rusher), `CreateContainerConfigError` (api, worker), `ImagePullBackOff`
  (enrichment 8d, guardian 13d, swarm 13d), `CrashLoopBackOff` (matchmaker ×34).
  Only `acb-schema-init` is Running (9d).
- `acb-index-builder-6669fdbc95-h4sg7` is currently **`Pending`/Unschedulable**, NOT
  `CreateContainerConfigError` — so even the cited symptom no longer matches; the pod
  never reaches container creation, so the missing secret isn't the active blocker.
- `sealed-secrets-apexalgo-iad-sealed-secrets-web` pod is itself in `CrashLoopBackOff`
  (10 restarts) — SealedSecret machinery on this cluster is partially degraded. (Main
  controller `sealed-secrets-apexalgo-iad-…-5d4nt` is Running, so unsealing may still
  work, but the validating webhook path is down.)

## Why I did not mint the secret

1. **Credentials require a human** (the bead itself says so): a real Cloudflare API token
   with Pages-Edit + R2-Edit + User-Read perms must be generated in the Cloudflare
   dashboard. No token exists anywhere I can reuse headlessly (no env var; `wrangler`
   uses interactive `login`; the only in-cluster Cloudflare token, reflected
   `cloudflare-pages-secret` in `utilities`, is Pages-only and lacks R2-Edit).
2. **It would contradict committed GitOps intent.** The operator retired this workload.
   Reviving it via a one-off secret is the opposite of the recorded decision and would
   just re-create a known-broken deployment (it needs every secret, recovered cluster
   capacity, and the freshly compiled image — see below).
3. **It wouldn't unblock anything on its own.** The pod can't even be scheduled today.

## Decision needed (human)

**Branch A — decommission stands (most likely).** Then the real task is *cleanup*, not a
secret: get ArgoCD on apexalgo-iad healthy so the ApplicationSet prunes the
`ai-code-battle-ns-apexalgo-iad` app and its 14 orphaned Deployments. Investigate why
ArgoCD stopped self-healing/pruning (its RO API is currently unreachable). Once pruned,
this bead and bf-1yj's acceptance criteria are moot — close as obsolete.

**Branch B — revive ai-code-battle on apexalgo-iad (intent changed).** Then minting one
secret is insufficient; the full restore is:
1. Restore `k8s/apexalgo-iad/ai-code-battle/` to `declarative-config` (recover from
   `0163324e^`), or re-sync from `manifests/` here (note: those are staging files; the
   deployment header says "sync to declarative-config/k8s/iad-acb/ai-code-battle/" but
   `iad-acb` was itself removed — `53fc54a1`, cluster deleted).
2. Recreate **all** missing secrets, not just cloudflare: `acb-postgres-credentials`,
   `acb-r2-credentials` (note: per `R2_ACCESS_KEY_SOURCE.md` / bf-4ur the R2 creds were
   **corrupted** — endpoint/keys swapped), `acb-armor-credentials` (OpenBao
   `rs-manager/iad-acb/armor`), `acb-api`, `acb-evolver`, `acb-matchmaker` secrets, plus
   `acb-cloudflare-api-token`. Apexalgo-iad secret pattern = **kubeseal SealedSecret**
   (controller `sealed-secrets-apexalgo-iad`, ns `sealed-secrets`); cf. existing
   `k8s/apexalgo-iad/kubernetes-reflector/cloudflare-pages-sealedsecret.yml`.
3. Recover cluster capacity: bring the NotReady node back / resize so pods can schedule.
4. Deploy the freshly compiled image. The bf-16k compile break is **fixed** (commits
   `724f1d9`, `2a59625`; binary builds at `acb-index-builder`, ~20MB) — but that image is
   **not yet built/pushed** by CI, so the Deployment still references a stale digest
   (`@sha256:88db4dd2…`). A new `ronaldraygun/acb-index-builder` build + image-updater
   roll is required.
5. Fix the crashing `sealed-secrets-web` pod if relying on SealedSecrets.

## Cross-references

- **bf-16k** (closed): `acb-index-builder` compile fix — binary now builds; image not yet
  shipped. See `notes/bf-16k.md`.
- **bf-4ur** (prior): secret/template review — accurate *at the time*, but its claim that
  `k8s/apexalgo-iad/ai-code-battle/acb-cloudflare-api-token-secret.yml.template` exists is
  now false: that dir was deleted in `0163324e`.
- Decommission commit: `declarative-config` `0163324e` (Jul 21 2026); cluster-dir removal
  `53fc54a1`.
