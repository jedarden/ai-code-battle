# SPA→API Transport — Decision Record

**Decision Date:** 2026-09-25
**Bead:** aicodeba-84d1d61b
**Status:** LIVE on the origin as of 2026-09-26 ~06:00Z (deploy
`acb-site-pages-build-fq8tb`, first attempt, after the clone-URL fix in
declarative-config `1d37a701`) — this note is the single source of truth
for `/api` transport state

## Decision

Same-origin `/api/*` routes are served by a **Cloudflare Pages Function** at
`web/functions/api/[[path]].ts`, deployed with the site by the existing
`acb-site-pages-build` pipeline. This is the "path-based route on the Pages
site" option named in `public-api-descope.md`; it needs no new account
resources (no new hostname, no IngressRoute, no cluster deployment) and works
on the one hostname the Pages project can ever have
(`ai-code-battle.pages.dev`).

Before this, every `/api/*` request was answered by the Pages SPA fallback
with `index.html` (HTTP 200, `text/html`), which is why the workflows were
explicitly disabled in bead aicodeba-07caa4ec.

## Deployment state (root-caused and unblocked 2026-09-26 — bead aicodeba-a34b8a80)

The function ships with the site: `wrangler pages deploy dist` bundles
whatever is under `web/functions/` at deploy time, so the SPA bundle and the
function always go live in the same deploy and cannot diverge.

**What was broken (2026-08-28 → 2026-09-26):** every push to `main` re-ran
`acb-site-pages-build` and lost all four retries with exit 128 in the
git-clone step — `acb-site-pages-build-rg9vx` (03912f0 push, 20:32Z 09-25),
`-pbmvp` (4ee9bfe, 22:01Z 09-25), `-cj75n` (5f51993, 01:05Z 09-26) and
`-97p2n` (f0f8d48, ~03:13Z 09-26) — with the sibling `acb-build` failing
the same way at its clone. This note previously suspected the
`FORGEJO_TOKEN` credential in the `forgejo-webhook-token` secret in `iad-ci`;
**that suspect is refuted.** The credential stored at OpenBao
`secret/rs-manager/iad-ci/forgejo/ci-token` was exercised directly
(`git ls-remote` against the real repo) and answered with the then-current
HEAD (f0f8d48) — healthy, never rotated, nothing to rotate. Forgejo's 200s
throughout the outage were real; the pipeline was simply never reaching it.

**Actual root cause:** the template's `git-repo` parameter pointed at
`forgejo.ardenone.com/ai-code-battle/ai-code-battle` — a hostname that does
not exist (established in declarative-config `29152867`, 2026-07-16: the
only real Forgejo instance is `git.ardenone.com`) **and** an org repo path
that does not exist (the repo is `jedarden/ai-code-battle`). git answers an
unresolvable host with exit 128 — the same code an auth failure produces,
which is what made the token look guilty. The bogus URL was introduced by
declarative-config `9a3ec073` (2026-08-28), which flipped the parameter
**from** the correct `git.ardenone.com/jedarden/ai-code-battle` **to** the
bogus one. Consequence: **no site deploy of any kind has shipped since
2026-08-28**, which is why the deployed bundle predates a6b425e by more
than the `/api` work — and why the deployed `/r2` function kept answering
(`GET /r2/<missing>` → its own `text/plain` 404) while every `/api` route
fell to the SPA fallback (**200 `text/html`**, `POST /api/register` a bare
405 from the static layer).

**Fix:** declarative-config `1d37a701` (2026-09-26) corrects the
`git-repo`/`repo` parameters of `acb-site-pages-build` and both templates in
`acb-build.yml` to `git.ardenone.com/jedarden/ai-code-battle`; ArgoCD synced
iad-ci and the live templates verify free of the bogus host. **Registry-side
sweep complete** (bead aicodeba-51921aa5, declarative-config `b7204551`,
2026-09-26): the `registry` parameter and kaniko destinations/cache-repo of
the dormant image templates (`acb-build.yml`'s `acb-build-images` and
`acb-build-site`, `acb-images-build`, `acb-enrichment-build`,
`acb-site-build`) now point at Docker Hub `ronaldraygun/*` with the shared
`docker-hub-registry` kaniko secret and `ronaldraygun/cache`; the
`git-repo` parameters of `acb-images-build`/`acb-site-build` carry the
correct clone URL; verified applied live in iad-ci — zero
`forgejo.ardenone.com` refs across all five templates after ArgoCD synced
`b7204551`. The one deployment the digest-pinning sweep had missed,
`acb-enrichment`, is pinned to `ronaldraygun/acb-enrichment@sha256:33622d8c`
(tag `f0f8d48`, verified present on Docker Hub) in both
`manifests/acb-enrichment-deployment.yml` and the rs-manager ArgoCD copy,
with its image-list annotation and pull secret moved off the dead registry.
Still dormant by design, recorded for completeness: the rs-manager
`acb-build.yml`/`acb-eventsensor.yml` CI copies (that cluster runs no Argo
Workflows — nothing applies them) and the `acb-evolved-bot-deploy` staging
template (deployed nowhere since the apexalgo-iad era; refs corrected, but
it is a deletion candidate). `acb-bots-build` never had the bogus host (its
unrelated `build-farmer` failures are a separate history). No rotation of
`forgejo-webhook-token` was needed or performed.

**Verified live 2026-09-26 ~06:00Z:** the re-triggered
`acb-site-pages-build-fq8tb` (dispatched by the push of the root-cause
note above) ran clean — clone, tsc, build and `wrangler pages deploy` on
the first attempt, 05:58:29Z → 06:00:35Z — and the origin now answers the
transport contract: `GET /api/health` → **200 `application/json`**
(`{"status":"ok","capabilities":{"register":false,"rotate_key":false,
"predictions":false,"feedback":true,"map_votes":true}}`), and
`web/test-api-workflows.js` passes against the live origin (exit 0). The
community tier is **LIVE** (replay/site feedback + map voting); the
match-tier routes answer their 503 `match_tier_offline` envelopes as
designed until acb-api is revived.

The table below describes the contract the routes answer with; 
`web/test-api-workflows.js` is the arbiter of which state the origin is in
(see Probe).

## Capability split

The function implements what its storage can support and is honest about the
rest:

| Flow | Route | Once deployed |
|---|---|---|
| Replay feedback (annotations) | `POST /api/feedback`, `GET /api/feedback/{match_id}`, `POST /api/feedback/{id}/upvote` | **LIVE** |
| Agentation site feedback | `POST /api/feedback` (body carries `markdown`) | **LIVE** |
| Map voting | `POST /api/vote/map`, `GET /api/vote/map/{map_id}` | **LIVE** |
| Bot registration / key rotation | `POST /api/register`, `POST /api/rotate-key` | **503** `match_tier_offline` |
| Predictions | `GET /api/predictions/open`, `GET /api/predictions/history`, `POST /api/predict` | **503** `match_tier_offline` |

The match-tier flows need acb-api's PostgreSQL/Valkey backend, which is not
deployed anywhere (compute tier decommissioned 2026-07-21; the deferral is
the recorded decision — see "Match-tier contract decision" in
`public-api-descope.md`, bead aicodeba-a0052569). Those routes answer
**503 JSON with code `match_tier_offline`** so the SPA renders its
unavailable states from the server's answer instead of a compile-time flag.
When acb-api is revived and exposed, the function becomes a thin proxy for
those routes and **no client change is needed**.

Request/response shapes mirror `cmd/acb-api/server.go` so the SPA clients in
`web/src/api-types.ts` and `web/src/components/annotation.ts` work against
either backend unchanged. The mirror is enforced, not aspirational: every
handler answer is constructed against the client-facing types (`satisfies`
checks against `api-types.ts` and `types.ts`), so a shape change fails the
tsc gate on both sides instead of drifting from the Go server at runtime.

## Storage design (community tier)

State lives in the already-bound `ACB_BUCKET` R2 bucket (`acb-data`,
declared in `web/wrangler.toml`) as small JSON documents under
`community/*`:

- `community/map-votes.json` — `map_id → voter_id → ±1`
- `community/replay-feedback.json` — capped FIFO of feedback entries with
  per-entry voter dedupe sets
- `community/site-feedback.json` — capped FIFO of agentation overlay
  submissions

All writes go through read-modify-write loops with **etag-conditional puts**
(R2 is strongly consistent, so the conditional put is a compare-and-swap);
six failed attempts answer 503 `storage_busy`. Bounds: 32 KiB request bodies,
500 replay / 200 site feedback entries, per-isolate fixed-window rate limits
(feedback 20/h, votes 30/h, upvotes 60/h per IP), and the spam filter ported
from `cmd/acb-api/spamfilter.go`.

## Abuse controls (bead aicodeba-278507ce)

The community routes are anonymous public writes to shared storage feeding
user-visible pages, so the function enforces its own abuse controls before
any write reaches the bucket. Everything below answers JSON in the same
error envelope as every other route — no HTML, no silent drops.

| Control | Enforcement | Answer |
|---|---|---|
| Payload size | A declared oversized `Content-Length` is rejected before a byte is read; bodies are then consumed **incrementally** and aborted mid-stream (`reader.cancel()`) past the cap, so the isolate never buffers more than 32 KiB of any upload — a `text()`-then-check shape would be an OOM lever on a public write path. The ceiling is enforced in bytes. | 413 |
| Field validation | Replay feedback: `match_id` ≤128 chars of `[A-Za-z0-9_.:-]`, `type` ≤32 chars and one of `insight`/`mistake`/`idea`/`highlight`, `body` 1–2000 chars, `turn` integer 0–1M, `author` ≤64 chars (missing → `Anonymous`). Site feedback (markdown-carrying): `markdown` 1–8000 chars, `annotations` must be an array (first 50 kept), `submitted_at` ≤40 chars (else server timestamp). Extra client-local fields are ignored, never stored. | 400 |
| Content screening | Spam filter ported from `cmd/acb-api/spamfilter.go`: block list, leetspeak/homoglyph normalization, 10-character floor — applied to both feedback flavors. | 422 |
| Per-identity dedup | One upvote per `voter_id` per feedback entry (repeat → idempotent `already_upvoted`); one vote per `voter_id` per map (a switched vote replaces, never stacks). Ids equal to `__proto__` are rejected outright: they double as object keys in the stored documents, where the write would route through the inherited setter and be **silently dropped**, and a JSON round-trip can hand the read path an own `__proto__` key no legitimate voter produced (`my_vote` is only ever echoed for a genuine stored `±1`). | 200 / 400 |
| Dedup-set bounds | ≤1000 voters per feedback entry, ≤5000 per map; a full set declines new writers in place (tally unchanged) rather than growing unboundedly. | 200 (no-op) |
| Per-IP rate limit | Fixed windows keyed on `CF-Connecting-IP` (`X-Forwarded-For` first hop as fallback): feedback 20/h, map votes 30/h, upvotes 60/h. Checked after validation but before any storage write, so a rejected write costs nothing but the answer. | 429 |
| Storage contention | etag CAS with six retries so concurrent writers serialize instead of clobbering. | 503 `storage_busy` |

The rate limiter is **per-isolate** by design: Workers isolates come and go,
so these windows bound accidental hammering and make abuse cost something —
they are not a global guarantee. Cloudflare's edge (WAF / rate-limiting
rules) is the global layer for a real flood; the in-function limiter keeps
any single origin from filling the community documents in the meantime.

Storage-side bounds cap total exposure even under abuse: 500 replay / 200
site-feedback entries FIFO, and each entry is field-capped, so the worst
case the documents can grow to is a few hundred KiB regardless of traffic.

## Client contract

- `web/src/lib/api-transport.ts` — `API_TRANSPORT_ENABLED` compile-time kill
  switch (false restores the pre-transport disabled behavior); 
  `MatchTierOfflineError` + `matchTierOfflineMessage()` for the 503s.
- Every `/api` client requires `content-type: application/json` before
  believing a response — the SPA HTML fallback is a 200, so JSON is the
  transport's proof of life.
- `GET /api/health` returns `{status, capabilities}` — the live capability
  set, for operators and the live probe. Storage unreadable flips the
  community capabilities off so the failure is observable before a submit.

## Operations

- **Probe:** `npm run test:api-workflows` in `web/`
  (`web/test-api-workflows.js` — read-only against the live origin) is the
  one retained live probe for this transport. It fails loudly while `/api`
  answers anything but JSON — the origin's state as of the verification
  above — and keeps guarding the transport after it ships. Do not revert it
  to the descope-era expectation (fail unless `/api` answers the SPA
  fallback); that contract is dead and the flip is the deploy signal.
- **Reviving the match tier:** deploy acb-api with its databases, expose it
  to the function (same-zone service or a Pages-compatible route), then
  replace the `matchTierOffline()` branch in `web/src/lib/api-backend.ts`
  with a proxy. The routes, shapes, and clients stay as-is. Deferring until
  then is the recorded decision ("Match-tier contract decision" in
  `public-api-descope.md`), and its revival triggers live there too.

## Testing

Two tiers cover the transport; both matter, and they answer different
questions. The local suite proves the function's *contract*; the live smoke
proves the *origin's state*.

### Local regression suite (offline — no network, no build, no bucket)

```bash
cd web && npm run test:unit -- src/lib/api-backend.test.ts src/lib/api-function-adapter.test.ts
```

Drives `handleApiRequest` in-process against an in-memory CAS bucket and
pins the whole server contract the probe only samples: health and capability
reporting, map voting, replay + site feedback round trips, match-tier 503
envelopes, per-IP rate limits (429), request bounds (413 plus the FIFO and
dedupe-set caps), CAS retry/storage-busy (503) behavior, and the JSON
content-type contract on every route and status class — the same gate the
probe applies live. Also runs as part of plain `npm run test:unit`.

The deployed adapter itself — `functions/api/[[path]].ts`, the URL parse,
one-time `/api` prefix strip, and env hand-off the Pages runtime invokes —
is pinned end-to-end by `src/lib/api-function-adapter.test.ts` (same
runner, same in-memory bucket): every documented route answered through
`/api/*`, the strip proven one-time (`/api/api/health` is not health), the
query string proven to survive, the handed env proven to answer, and the
JSON content-type contract re-asserted over every adapter-reachable answer
class. `web/tsconfig.json` includes `web/functions/**` (runtime globals via
`functions/pages-runtime.d.ts`, minimal stand-ins for the uninstalled
`@cloudflare/workers-types`), so the adapter file the deploy actually ships
is inside the tsc gate too.

The client and page halves (kill switch, `MatchTierOfflineError` rendering,
HTML-fallback backstops) live in `web/src/api-transport.test.ts`,
`web/src/components/annotation.test.ts`, `web/src/pages/register.test.ts`,
`web/src/pages/predictions.test.ts`, and `web/src/pages/replay-mapvote.test.ts`
— covered by the default `npm test`.

### Live smoke (needs a fresh build and a reachable origin)

```bash
cd web && npm run build && npm run test:api-workflows
# against a local function instead of production:
ACB_ORIGIN=http://127.0.0.1:8788 npm run test:api-workflows   # wrangler pages dev
```

`web/test-api-workflows.js` is read-only against the origin. Part A asserts
the built bundle still carries the transport contract markers; part B probes
health + capabilities, one map tally, one feedback read, and one refused
register. It is the arbiter of which state the origin is in (see Deployment
state above): it fails loudly while `/api` answers anything but JSON — which
is currently the deploy signal, not a code bug.

### Live end-to-end flows (needs the network; opt-in like the smoke)

```bash
cd web && npm run test:e2e-api
# against a local function instead of production:
ACB_ORIGIN=http://127.0.0.1:8788 npm run test:e2e-api
```

`web/e2e/api-flows.e2e.test.ts` (run by `vitest.e2e.config.ts`, whose include
never overlaps the default gate) completes the four client flows against the
real origin with no mocked fetch: the register form submitting and rendering
the server's 503 match_tier_offline answer, the predictions page rendering
its open/history unavailable states from the same 503s plus predict
surfacing `MatchTierOfflineError`, and — through the real client functions —
the map-vote POST→GET round trip and the replay-feedback POST→GET→upvote
round trip. Every flow asserts `application/json` on the wire before
believing a body, so an SPA-fallback regression fails the suite rather than
passing as a 200. Round-trip writes land under an
`acb-e2e-probe-<timestamp>-<rand>` match/map id no page ever renders (same
namespace argument as the write probe above); the 503-aware flows are green
without acb-api — that is their designed answer.

### Release gate — a deploy is not done until the function answers (bead aicodeba-9e883051)

The smoke is also the release gate, at two layers, both hard-failing on any
SPA-fallback answer:

- **Deploy pipeline** — `acb-site-pages-build` (declarative-config
  `k8s/iad-ci/argo-workflows/acb-site-pages-build-workflowtemplate.yml`,
  gate added in `3fc3cc55`, 2026-09-26) runs the full smoke
  (`node test-api-workflows.js` — build markers in the dist just shipped,
  health, community reads, and **all five** documented match-tier 503
  routes: register, rotate-key, predict, predictions/open,
  predictions/history) as a post-deploy step, after polling `/api/health`
  for edge propagation (90 s cap). A transport regression now fails the
  deploy run itself instead of shipping silently — the failure mode the
  2026-08-28 → 2026-09-26 outage ran for weeks unnoticed.
- **Manual verification** — `scripts/verify-deployment.sh` runs the same
  smoke against the deployed origin (origin-only mode: the smoke skips the
  local-build markers under `ACB_SKIP_BUILD_CHECK=1`) and exits non-zero on
  any regression; it is the only hard-failing check in that script. Its
  pre-transport checks (`aicodebattle.com` apex, `api.aicodebattle.com`
  K8s health) were stale — the apex is NXDOMAIN (see
  `canonical-public-domain.md`) and the K8s API host has answered nothing
  since the compute-tier descope, which is exactly why the SPA fallback
  could go unnoticed.

The match-tier 503 probes cost nothing and write nothing: the router
refuses those routes before any body is read, rate-limited, or stored, so
proving all five are function-owned is free on every deploy.
