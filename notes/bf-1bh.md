# Bead bf-1bh — Resolve and validate Cloudflare deploy credentials

## Outcome: NOT COMPLETED — no resolvable token exists; blocked on a human action. Bead left OPEN.

The bead asks to locate a `CLOUDFLARE_API_TOKEN` (from the apexalgo-iad SealedSecret or a
local source), export it, and validate it with `npx wrangler whoami`. All three of those
sources are empty/non-existent, and the only remaining path — generating a token in the
Cloudflare dashboard — is a human-only action. **No credential was exported, no token was
created, and `wrangler whoami` was not run (there is nothing to authenticate with).** This
matches the conclusion reached the same day by the companion bead [[bf-1yj]].

---

## Acceptance criteria — all three fail

| # | Criterion | Result |
|---|-----------|--------|
| 1 | Source of `CLOUDFLARE_API_TOKEN` identified & documented | **No usable source exists.** The only historical source was a `.template` (never a real SealedSecret), now deleted by the Jul 21 decommission. See evidence below. |
| 2 | `echo ${CLOUDFLARE_API_TOKEN:0:4}` prints non-empty | **Empty (`''`)** — and there is no token to populate it with. |
| 3 | `npx wrangler whoami` succeeds and shows expected account | **Cannot run** — no token and no cached `wrangler login` session exist on this host. |

---

## Bottom line / TL;DR

- **Local env is empty.** `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `CF_API_TOKEN`
  are all unset (prefix `''`).
- **No local Cloudflare/wrangler credential exists.** Absent: `~/.wrangler/config/default.toml`,
  `~/.config/.wrangler/...`, `~/.cloudflare/config`, `~/.cloudflare/token`. No
  `CLOUDFLARE_*` reference in shell dotfiles. No cached `wrangler login` OAuth token, so
  `wrangler whoami` has nothing to authenticate with.
- **The cluster SealedSecret source does not exist and never did.** Only
  `acb-cloudflare-api-token-secret.yml.template` was ever committed; no real
  `acb-cloudflare-api-token-sealedsecret.yml` was ever produced (confirmed in [[bf-1yj]]).
  That template was **deleted** in the Jul 21 decommission (see below), so even the
  instructions to mint it are gone from GitOps.
- **The apexalgo-iad read-only proxy denies secret reads** (`Forbidden: … cannot get
  resource "secrets"`), so even if a SealedSecret existed, its encrypted ciphertext could
  not be read from here — and a SealedSecret's plaintext is only decrypted in-cluster by
  the controller, not client-side, in any case.
- **A real token requires human dashboard access.** This is stated by both the original
  template (Pages-Edit + R2-Edit + User-Read scopes, generated in the Cloudflare UI) and
  by [[bf-1yj]]. There is no token anywhere this worker can reuse headlessly.

## Evidence

### Local
```
CLOUDFLARE_API_TOKEN prefix: ''
CLOUDFLARE_ACCOUNT_ID: ''
CF_API_TOKEN prefix: ''
# ~/.wrangler/config/default.toml … absent
# ~/.config/.wrangler/config/default.toml … absent
# ~/.cloudflare/config … absent
# ~/.cloudflare/token … absent
# no CLOUDFLARE_* refs in ~/.bashrc ~/.profile ~/.zshrc ~/.config/environment.d ~/.env
```

### GitOps source of truth (`declarative-config`)
- `k8s/apexalgo-iad/ai-code-battle/` **no longer exists** on `main`.
- Commit **`0163324e`** (jedarden, **Tue Jul 21 2026**) — *"chore(apexalgo-iad): take down
  ai-code-battle — remove compute/CI/ingress"* — deleted all 14 Deployments, 11 Services,
  IngressRoute, EventSource/Sensor, WorkflowTemplates, SealedSecrets, and explicitly
  **`ai-code-battle/acb-cloudflare-api-token-secret.yml.template` (58 lines)**.
- `git log -- 'k8s/apexalgo-iad/ai-code-battle/*'`: last touch is that deletion; nothing
  re-added it.
- The only `*cloudflare-api-token*` files left anywhere in `declarative-config` are for
  `feelings-wheel` (`k8s/iad-ci/utilities/cloudflare-api-token-feelings-wheel-*.yml`) — a
  different project, and its scope (Pages-only per [[bf-1yj]]) is wrong for acb (which also
  needs R2-Edit). Reusing it would be a cross-project credential decision a human should make.

### Live cluster (apexalgo-iad, ns `ai-code-battle`, read-only proxy)
- Namespace **still exists** — the orphans have **not** been pruned (ArgoCD on this cluster
  remains unable to self-heal/prune; see [[bf-1yj]]).
- Pods by phase: `6 CreateContainerConfigError`, `8 ImagePullBackOff`,
  `1 CrashLoopBackOff`, `1 Running`.
- `acb-index-builder-6669fdbc95-h4sg7` is `0/1 CreateContainerConfigError` — still failing
  on the missing `acb-cloudflare-api-token` secret, exactly the symptom the bead describes.
- `kubectl … get secret acb-cloudflare-api-token` → **Forbidden** (proxy has no secret
  access) — consistent with "secret not found" from the pod's perspective.

## Why I did not mint or export a token

1. **No token to locate.** Every source the bead names is empty or deleted (above).
2. **It requires a human.** A Cloudflare API token of the required scopes must be created in
   the dashboard. `wrangler login` is interactive browser OAuth and cannot be completed
   headlessly on the operator's behalf without their credentials — which would be an
   unauthorized credential-creation step, not a lookup. The Pixel 6 browser failover is for
   *reading* pages, not for performing authenticated logins as the operator.
3. **It would contradict committed GitOps intent.** The workload that consumes this token
   was deliberately retired on Jul 21. Minting/exporting the credential would prop up the
   decommission's orphaned pods rather than respect the operator's decision.

## Decision needed (human)

This is the same fork identified in [[bf-1yj]]:

**Branch A — decommission stands (most likely).** Then the task is *cleanup*, not a
credential: get ArgoCD on apexalgo-iad healthy so the `manifest-appset-apexalgo-iad`
ApplicationSet prunes the `ai-code-battle-ns-apexalgo-iad` app and its 14 orphaned
Deployments (still not done as of this check). Once pruned, **this bead and [[bf-1dk]]'s
`wrangler pages deploy` step are moot** — close as obsolete.

**Branch B — revive the public Pages deploy (intent changed).** If the goal is purely to
re-publish the SPA (parent [[bf-1dk]]) independent of the decommissioned index-builder, then:
1. A human generates a Cloudflare API token with **Pages-Edit** scope in the dashboard
   (R2-Edit too if the index builder is ever revived).
2. Export it in the deploy shell and run `npx wrangler whoami` to confirm the account.
3. Then `wrangler pages deploy web/dist --project-name=aicodebattle --branch=production`.
Note this only republishes the SPA shell; `/data/leaderboard.json` will 404 until a data
source is restored (bf-1dk's acceptance criteria already tolerate this).

Either way, **bf-1bh is a prerequisite that a worker cannot satisfy** — it gates on human
Cloudflare access. Recommend reassigning to a human or folding into [[bf-1yj]]'s open decision.

## Cross-references

- **[[bf-1dk]]** (blocked, parent): Deploy SPA to Cloudflare Pages — waits on this credential.
- **[[bf-1pc]]** (closed): `/r2/` → `/b2/` path fix — done, independent of credentials.
- **[[bf-1yj]]** (in_progress, sibling): the missing-secret root cause + full decommission
  analysis. This bead is the credential-resolution half of the same finding.
- Decommission commit: `declarative-config` `0163324e` (Jul 21 2026).
