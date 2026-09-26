# CI test strategy — fast gate on push, full suite opt-in

Decision of 2026-09-26 (bead `aicodeba-4db578f7`), after `acb-build-gk675`
and `acb-build-bv9mv` (2026-09-25) both failed in the test step.

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
- **Full suite (`acb-full-tests` template, iad-ci)**: opt-in, not wired to
  the event sensor. Submit manually when engine semantics change:

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

  It runs `go test ./... -count=1 -timeout 1500s` with a 1800s step
  deadline (~2x headroom over the measured 411s floor; the 2-CPU CI pod is
  not faster than the measurement box).

Both templates live in `declarative-config/k8s/iad-ci/argo-workflows/`
(`acb-build-workflowtemplate.yml`, `acb-full-tests-workflowtemplate.yml`)
and reach the cluster through the `argo-workflows-ns-iad-ci` ArgoCD app.

## Not done here

- Raising the acb-build timeout: cannot fit 411s+ of engine plus web checks
  in the 600s step, and a slower push gate on every commit is the wrong
  trade when the slow tests self-declare as skippable.
- `acb-images-build` (the `-race` variant): dormant and out of scope — its
  test step installs no gcc, so its existing `-race` lines cannot compile
  there regardless.
