# bf-6c3 — Hygiene sweep: purge tracked artifacts, dead CI workflows, doc drift

**Repo:** ai-code-battle (decommissioned Jul 21 2026 — see beads bf-2vf / bf-4u5 / bf-5ll / bf-687)
**Date:** 2026-07-26
**Checker:** `jeds-curated-skills/repo-hygiene/scripts/repo_hygiene.sh --json`

## Outcome: all four assigned fix categories were ALREADY CLEAN — no file changes required

The task (step 3 a–d) names exactly four fix categories. The checker reports **zero
findings** in all four. Acceptance criteria from step 6 are met by the pre-existing
state of the repo, before any action by this bead.

### (a) gitignore-gaps → 0  (clean)
`.gitignore` already covers every detected language's build/cache output: `node_modules/`,
`dist/`, `target/`, `__pycache__/`, `.DS_Store`, `wasm/bots/*/build/`, plus root binaries
(`/acb-*`, `/arena.test`), env files, and test/map output. Checker found no gap.

### (b) tracked-build-artifacts → 0  (clean)
Checker `tracked-build-artifacts` category absent (count 0). The dir-pattern artifacts
(`target/`, `node_modules/`, `dist/`, `build/`, `__pycache__/`, `*.pyc`, `.DS_Store`) are
not tracked — all covered by `.gitignore`. See "Out of scope" below re: large binaries.

### (c) dead-ci-workflows → 0  (clean)
`git ls-files '.github/workflows/*'` returns only:
- `.github/workflows/ci.yml.disabled`
- `.github/workflows/deploy-pages.yml.disabled`

Both are renamed `.disabled` (policy-dead already). There are **no** active
`.github/workflows/*.yml|yaml` files for the checker to flag. Estate policy (GitHub Actions
disabled estate-wide; CI is Argo Workflows on iad-ci) is already honored here.

### (d) README version/CI-badge drift → 0  (clean)
README contains no `shields.io`/GitHub-Actions badges (grep for `badge|shields|workflows`
returns nothing) and the repo has **no git tags**, so there is no version string to drift.
Both `readme-dead-ci-badges` and `readme-version-drift` are N/A.

## Out-of-scope findings (reported, NOT acted on — per step 3 "ONLY these fixes" + step 4)

The checker did report other categories. None are in the task's fix list, so they are left
untouched:

| Category | Count | Why not acted on |
|----------|-------|------------------|
| `large-tracked-files` (high) | 10 | **Not a step-3 fix category.** Includes compiled Go binary `bots/gatherer/gatherer` (7.6 MB), `web/public/wasm/*.wasm` build outputs, and large `*.json` replay **data** files. Untracking these is a needs-review decision (some are intentional committed static assets for the web app); the bead did not assign it, and they belong to the checker's size-based category, not the dir-pattern `tracked-build-artifacts` category named in the acceptance criteria. Left for a human. |
| `root-ad-hoc-files` (medium) | 1 | `test_routes.sh` — not in the step-3 fix list. Left as-is. |
| `dirty-working-tree` (low) | 2 | **FORBIDDEN by step 4** — report-only. (`M .needle-predispatch-sha`, `?? .wrangler/cache/pages.json`) |
| `stash-pileup` (low) | 22 | **FORBIDDEN by step 4** — report-only. |

## Infrastructure notes (forgejo outage + reconciliation)

- **`git pull origin` initially failed: forgejo (`git.ardenone.com`) returned HTTP 502** on
  both the web root and the `git-upload-pack` endpoint, across multiple retries during the
  audit phase. forgejo later recovered (see Push).
- The **`github` remote was reachable** throughout and held the same `master` ref as the
  cached `origin/master` (`22cf14a`).
- Local HEAD had diverged from `origin/master`: `e7b5a6f` (local) and `22cf14a` (origin) are
  the same `docs(bf-2vf)` change with an **identical tree** (`f584db9`), differing only in
  committer metadata (an amend/rewrite). Local also carried a concurrent agent's `1a051d0`
  (`docs(bf-58z)`). Reconciled via **merge** (the sanctioned "merge only" path — no
  force-push, no reset, no stash): merge commit `ec626c7` now descends from `origin/master`,
  making the notes commit pushable non-fast-forward-free.

## Push

- `git push origin` → **succeeded** (`22cf14a..911cb6b`); forgejo had recovered by push time.
- `git push github` → **succeeded** (`22cf14a..911cb6b`); GitHub mirror in sync.
- Both remotes now carry the notes commit.

## Final checker summary

After this bead (notes commit only — no source/hygiene file changes, since none were
needed), the four assigned categories remain at 0:

- tracked-build-artifacts = **0**
- dead-ci-workflows = **0**
- gitignore-gaps = **0**
- readme-dead-ci-badges / readme-version-drift = **0** (no badges, no tags)

Remaining (unchanged, out of scope): `large-tracked-files` (10), `root-ad-hoc-files` (1),
`dirty-working-tree` (2 — report-only), `stash-pileup` (22 — report-only).
