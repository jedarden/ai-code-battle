# Bead bf-wsf — acb-api CreateContainerConfigError: Secret `acb-app-credentials-acb-app` missing `uri` key

## Outcome: NOT COMPLETED — bead premise is stale; contradicts a deliberate decommission. Bead left OPEN for a human decision.

The bead asks to fix acb-api's `CreateContainerConfigError` by providing the missing `uri`
key in Secret `acb-app-credentials-acb-app` so the pod reaches Ready. Independent
verification shows **ai-code-battle was intentionally decommissioned on apexalgo-iad 5
days ago** and the live acb-api pod is an **un-pruned orphan** whose Deployment manifest no
longer exists in GitOps. Providing the secret would revive a workload the operator
deliberately retired — and on its own would not make the workload healthy anyway. No
secret was minted, no manifest changed. This is the same situation bead **bf-1yj** (closed
today, see `notes/bf-1yj.md`) resolved the same way.

---

## Bottom line

- The GitOps source of truth (`declarative-config` `main`, HEAD **`72239248`**) has **no**
  `k8s/apexalgo-iad/ai-code-battle/` directory. Commit **`0163324e`** (jedarden,
  **Tue Jul 21 17:46 EDT**) deleted it. The commit message names the exact symptom in this
  bead as the *reason* for retirement:
  *"Retire the ai-code-battle deployment on apexalgo-iad (already non-functional: all 14
  deployments 0/1, **CreateContainerConfigError**/ImagePullBackOff)."* It deleted all 14
  Deployments (incl. `acb-api-deployment.yml`, −137 lines), 11 Services, IngressRoute,
  EventSource/Sensor + 3 WorkflowTemplates, SealedSecrets, and the
  `acb-armor-credentials` ExternalSecret. CNPG postgres **data was intentionally
  preserved**.
- `git log -- 'k8s/apexalgo-iad/ai-code-battle/*'`: last touch is the deletion
  `0163324e`; **nothing re-added it.** The only commits to `k8s/apexalgo-iad/` since are
  `vista` work — no revive intent.
- Therefore the bead's premise ("mint/repair the `uri` key so acb-api runs") is a snapshot
  from **before** the Jul 21 decommission. The `CreateContainerConfigError` it cites was
  the *reason* the workload was retired, not a fresh defect to patch.
- The `acb-app-credentials-acb-app` secret **does** still exist on the cluster (123d old,
  8 data keys), but the apexalgo-iad read-only observer SA **forbids reading secrets**
  (`Forbidden: … cannot get resource "secrets"`), so its keys cannot be inspected from here.
  It was never a declarative manifest in `declarative-config` history
  (`git log --all -- '*acb-app-credentials*'` → empty) — it is operator/CR-generated, so
  its absence of a `uri` key is an operator/CNPG-version schema matter, not a GitOps file
  to edit.

## Why I did not provide the `uri` key

1. **It would contradict committed GitOps intent.** The operator retired this workload on
   Jul 21. Per `CLAUDE.md`, all cluster writes go through `declarative-config` GitOps —
   never direct `kubectl` — and reviving acb-api via GitOps means re-adding the entire
   decommissioned `k8s/apexalgo-iad/ai-code-battle/` dir, which is the opposite of the
   recorded decision.
2. **No write path exists anyway.** The apexalgo-iad proxy is read-only RBAC and even
   denies secret reads; a direct secret patch is not possible from this server. The only
   legitimate path is a human-authored GitOps change.
3. **A single secret key is insufficient to revive the workload.** The Deployment manifest
   is gone; even with a corrected secret, the namespace needs every secret restored,
   cluster capacity recovered (1 of 3 nodes NotReady, 2 Ready nodes CPU-saturated → most
   pods `Pending`/`FailedScheduling`), the crashing `sealed-secrets-web` pod fixed, and a
   fresh image rolled. Reviving is the full Branch-B effort from bf-1yj, not a one-key fix.
4. **The `uri` expectation may have been unsatisfiable from the start.** CNPG's auto
   `-app` secret historically exposes `user/password/host/port/dbname/jdbc-uri`, not a
   plain `uri`. The preserved CNPG manifest (`k8s/apexalgo-iad/cnpg/database-ai-code-battle.yml`)
   references cluster `cnpg-apexalgo` and exposes a `acb-postgres` Service — it does not
   obviously produce a secret literally named `acb-app-credentials-acb-app`, so the
   deployment's `key: uri` binding was likely already broken pre-decommission.

## Evidence — live cluster (apexalgo-iad, ns `ai-code-battle`, read-only proxy)

- `acb-api-5646489f75-69ctc` is `CreateContainerConfigError` (0 restarts, 32m). A second
  replica `…-rzhg6` is `Terminating`. The whole namespace is broken: `Pending`
  (enrichment, evolver, index-builder, map-evolver, gatherer, hunter, random, rusher),
  `CreateContainerConfigError` (api, worker), `ImagePullBackOff` (enrichment 8d, guardian
  13d, swarm 13d), `CrashLoopBackOff` (matchmaker ×35). Only `acb-schema-init` runs (9d).
  This matches bf-1yj's snapshot; nothing has recovered.
- Secret list confirms `acb-app-credentials-acb-app` (Opaque, 8 keys, 123d) and
  `acb-api-secrets` (2 keys, 52d) are present; reads of secret *contents* are forbidden.

## Decision needed (human) — same fork as bf-1yj

**Branch A — decommission stands (most likely).** Then the task is *cleanup*, not a
secret: get ArgoCD on apexalgo-iad healthy so the `manifest-appset-apexalgo-iad`
ApplicationSet prunes the orphaned `ai-code-battle` app and its 14 Deployments (the git-dir
generator has `prune: true, selfHeal: true, allowEmpty: true`, so deleting the dir *should*
have pruned them — ArgoCD's RO API was unreachable, consistent with it being unhealthy).
Once pruned, this bead and its acceptance criteria are moot — close as obsolete. No action
on the `uri` key is wanted or needed.

**Branch B — revive ai-code-battle on apexalgo-iad (intent changed).** Then fixing the
`uri` key is necessary but not sufficient. The full restore (per bf-1yj) is:
1. Restore `k8s/apexalgo-iad/ai-code-battle/` to `declarative-config` (recover from
   `0163324e^`), or re-sync from `manifests/` in this repo.
2. Recreate **all** secrets (not just the `uri` key): `acb-app-credentials-acb-app`,
   `acb-postgres-credentials`, `acb-r2-credentials` (R2 creds were **corrupted** — per
   `R2_ACCESS_KEY_SOURCE.md`/bf-4ur, endpoint/keys swapped), `acb-armor-credentials`
   (OpenBao `rs-manager/iad-acb/armor`), `acb-api-secrets`, `acb-evolver-secrets`,
   `acb-matchmaker-secrets`. Apexalgo-iad pattern = **kubeseal SealedSecret** (controller
   `sealed-secrets-apexalgo-iad`, ns `sealed-secrets`). Either upgrade/annotate the CNPG
   Cluster to emit `uri`, or (more robust) change `acb-api-deployment.yml` to build
   `ACB_DATABASE_URL` from the keys CNPG *does* provide (`host/port/user/password/dbname`).
3. Recover cluster capacity (NotReady node / resize) so pods schedule.
4. Fix the crashing `sealed-secrets-web` pod if relying on SealedSecrets.
5. Roll a fresh `ronaldraygun/acb-api` image.

This requires human GitOps authorship + dashboard credential access; it is not a headless
single-secret mint.

## Cross-references

- **bf-1yj** (today, open→decision handoff): identical finding for the
  `acb-cloudflare-api-token` / acb-index-builder secret. See `notes/bf-1yj.md`.
- Decommission commit: `declarative-config` `0163324e` (Jul 21 2026).
- Preserved data: CNPG manifest `k8s/apexalgo-iad/cnpg/database-ai-code-battle.yml`
  (cluster `cnpg-apexalgo`), B2 bucket `armor-apexalgo`, OpenBao MEK.
