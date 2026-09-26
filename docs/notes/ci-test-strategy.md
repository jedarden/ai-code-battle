# CI test strategy — fast gate on push, full suite on engine-semantics change

Decision of 2026-09-26 (bead `aicodeba-4db578f7`), after `acb-build-gk675`
and `acb-build-bv9mv` (2026-09-25) both failed in the test step. The
un-enforced half — "someone must remember the full suite" — closed
2026-09-26 with the `acb-engine-semantics-gate` push trigger (bead
`aicodeba-e94f8d48`).

## What happened

`acb-build`'s `run-tests` step ran the engine suite with `-timeout 120s`,
but the full suite takes ~411s at `0bc8949` (measured 2026-09-25 in a clean
`git archive` extraction, `-timeout 420s -v`: exit 0, SLOW — not a
deadlock). Every push therefore failed CI in the test step after ~4 minutes,
and no timeout setting can fix it: the step deadline is 600s and must also
hold the `cmd` packages plus the web `npm ci` / `tsc` / lint checks.

## Why the suite is slow, and why that is by design

The engine integration tests are Monte-Carlo studies, not smoke tests —
`TestCombatDensityMetrics` alone runs 100 complete matches. They all gate
themselves on `testing.Short()` (e.g. `engine/integration_test.go`), so:

- `go test -short ./...` — the repo's own test convention — runs in ~9.8s
  and is what the NEEDLE close gate already uses.
- `go test ./...` runs the full Monte-Carlo set in ~411s on this box.

## The split

- **Push gate (`acb-build` test step)**: `go test ./conformance/...` first
  (cheap, deterministic, per-push signal per `docs/bot-protocol.md`), then
  `go test -short ./... -timeout 300s`. Matches the local gate; leaves the
  600s step budget to the web checks.
- **Full suite, automated (`acb-engine-semantics-gate` template, iad-ci)**:
  since bead `aicodeba-e94f8d48` the suite no longer depends on a human
  remembering it. The `ai-code-battle-ci-sensor` carries a second dependency
  (`acb-push-engine-watch`) and trigger that submit the gate workflow on
  every push to `main`. The workflow clones the pushed tip, deepens the
  shallow clone to 64 commits, and diffs `before → after` (both SHAs come
  from the Forgejo push payload) for files under `engine/` that are not
  `*_test.go`:

  ```
  git diff --name-only "$BEFORE" "$AFTER" -- engine/ | grep -v '_test\.go$'
  ```

  Empty result → the workflow logs `No engine/ semantics change` and exits
  0 in well under a minute. Non-empty → it runs the full
  `go test ./... -count=1 -timeout 1500s` (same command and 1800s step
  deadline as `acb-full-tests`, ~2x headroom over the measured 411s floor;
  the 2-CPU CI pod is not faster than the measurement box).

  Design notes:

  - *Why the path decision lives in the workflow, not the sensor*: Argo
    Events data filters match one JSON path against string values and cannot
    glob over the push payload's `commits[].added/removed/modified` arrays.
    The payload's `before`/`after` SHAs are all the sensor needs to forward.
  - *Why a comment-only change triggers the suite*: git sees paths, not
    intent, and the gate fails closed — a semantics file of any kind counts.
  - *Fail-closed on undecidable ranges*: an all-zero `before` (new-branch
    push) or a base SHA outside the deepened fetch falls back to the parent
    of the tip; a step that cannot decide errors out instead of silently
    skipping the suite.
  - *Additive wiring*: the gate is a separate dependency + trigger on the
    sensor, so it cannot disturb the acb-build / acb-bots / acb-site
    triggers even if it misbehaves.

- **Full suite, manual (`acb-full-tests` template, iad-ci)**: still there
  for running the suite at an arbitrary ref or before pushing:

  ```
  kubectl --kubeconfig=~/.kube/iad-ci.kubeconfig create -f - <<EOF
  apiVersion: argoproj.io/v1alpha1
  kind: Workflow
  metadata:
    generateName: acb-full-tests-
    namespace: argo-workflows
  spec:
    workflowTemplateRef:
      name: acb-full-tests
  EOF
  ```

All three templates live in
`declarative-config/k8s/iad-ci/argo-workflows/`
(`acb-build-workflowtemplate.yml`,
`acb-engine-semantics-gate-workflowtemplate.yml`,
`acb-full-tests-workflowtemplate.yml`) and reach the cluster through the
`argo-workflows-ns-iad-ci` ArgoCD app; the sensor change reaches
`argo-events-ns-iad-ci`.

## Not done here

- Raising the acb-build timeout: cannot fit 411s+ of engine plus web checks
  in the 600s step, and a slower push gate on every commit is the wrong
  trade when the slow tests self-declare as skippable.
- A close-gate requirement for beads touching `engine/` (record an
  acb-full-tests run in the bead notes): the NEEDLE close gate re-runs
  commands inside a `git archive` extraction, which carries no `.git` — a
  gate script there can neither see which paths a change touched nor read
  bead state, so the requirement would be unenforceable where it would
  live. The push-side gate above is also strictly earlier: the suite runs
  on the semantics-changing push itself, before any bead could close.
- `acb-images-build` (the `-race` variant): dormant and out of scope — its
  test step installs no gcc, so its existing `-race` lines cannot compile
  there regardless.
