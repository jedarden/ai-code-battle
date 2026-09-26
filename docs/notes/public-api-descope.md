# Public API Endpoint — Descope Decision

**Decision Date:** 2026-09-25
**Bead:** aicodeba-a42aec94
**Status:** DESCOPED — no public hostname, unchanged. The same-origin `/api/*`
transport named below as the realistic option has since been implemented
([api-transport.md](api-transport.md)), which owns `/api` transport state
from here on; this note keeps the hostname descope and, since 2026-09-26,
the match-tier deferral decision (bottom of this file).

## Problem

User-facing documentation advertised a public API endpoint that does not exist:

- `web/src/pages/docs.ts` showed `curl -X POST https://ai-code-battle.pages.dev/api/register` as the way to register a bot.
- `docs/notes/canonical-public-domain.md` listed "API endpoint (deferred): Would be `api.ai-code-battle.pages.dev` via Traefik routing".
- No open bead tracks implementing a public route, and both candidate hostnames are dead.

## Evidence (probed live 2026-09-25)

- `aicodebattle.com` is NXDOMAIN — the zone was never registered (DEPLOYMENT.md states the same), so `api.aicodebattle.com` can never resolve.
- `api.ai-code-battle.pages.dev` resolves only through Cloudflare's `*.pages.dev` anycast and serves Cloudflare's 404 "no project" page. A Pages project gets exactly one `<project>.pages.dev` hostname; a nested `api.` name under it cannot be claimed.
- `https://ai-code-battle.pages.dev/api/register` and `/api/bots` return the SPA's `index.html` (`content-type: text/html`) via the Pages SPA fallback — there is no `/api` route on Pages (`web/functions/` contains only `r2/`). A real user POSTing the old docs example gets HTML, and `registerBot()` then fails parsing it.
  (Re-verified 2026-09-25, bead aicodeba-968bfa99: still true of the **deployed origin** — GETs answer the SPA fallback and a POST gets Pages' bare 405 — but no longer of the **repo**, which has carried `web/functions/api/[[path]].ts` since a6b425e. The origin keeps answering the fallback only because no site deploy has succeeded since; see "Deployment state" in `api-transport.md`. This bullet describes the origin, not the tree.)
- No `acb-api` Deployment is running in any fleet cluster (all eight read-only kubectl endpoints checked 2026-09-25). The Go service is built in-repo (`cmd/acb-api/`, with a manifest at `manifests/acb-api-deployment.yml`), but nothing is deployed.
- `DEPLOYMENT.md` already documents the operative truth: "the public api subdomain was never registered; the Go API is reached via internal cluster networking."

## Decision

**Descope the public API endpoint.** No public hostname is spec'd, no IngressRoute is planned, and no implementation beads are opened:

- A public route would front nothing — the service is not deployed anywhere.
- plan.md §9.6 already frames public exposure as deferred until social features need it; that is the trigger to revisit this decision.
- When revisited, the hostname cannot be `api.ai-code-battle.pages.dev` (impossible under Pages — see evidence above). The realistic options are a registered domain pointed at a Traefik IngressRoute, or a path-based route on the Pages site (e.g. a Pages Function proxy under `/api/*`).

## Changes Made

1. `web/src/pages/docs.ts` — "Register Your Bot" no longer shows a curl against a URL that resolves to HTML; it states the API has no public endpoint and keeps only the request contract.
2. `docs/notes/canonical-public-domain.md` — the deferred-API line now points here.
3. `docs/plan/plan.md` — removed the claims that `acb-api` fronts `api.ai-code-battle.pages.dev` (Go API Service overview, §8.2, §9.6, data-loading example, documented-data-paths block).

## Known remaining gaps (out of scope here)

(Resolved 2026-09-25: the `web/test-match-list.js` dev harness no longer references the dead `b2.aicodebattle.com` host — it live-probes the Pages origin and the `/r2/*` Pages Function instead.)

(Resolved 2026-09-25, bead aicodeba-07caa4ec: the on-site register form (`#/compete/register`), the predictions page's pick/history sections, community-feedback submission, and map voting no longer attempt their dead same-origin `/api` calls. Every `/api` client function in `web/src/api-types.ts` and `web/src/components/annotation.ts` refuses to issue the request while `API_TRANSPORT_ENABLED` is false in `web/src/lib/api-transport.ts`, and the pages render explicit "unavailable" notices with disabled forms/buttons (feedback degrades to clearly-labelled local-only storage). `web/test-api-workflows.js` smoke-checks the built bundle for the notices and live-probes the origin — it fails loudly if `/api` ever stops answering with the SPA fallback, which is the signal to flip the flag back on once a real transport (Pages Function proxy or registered-domain IngressRoute, with acb-api actually deployed) exists.)

(Superseded 2026-09-25, bead aicodeba-84d1d61b: the path-based route named above as the realistic option is now implemented — a Pages Function under `web/functions/api/` serves same-origin `/api/*`, putting the community flows (replay feedback, agentation site feedback, map voting) on the already-bound `ACB_BUCKET` R2 bucket. Corrected 2026-09-25, bead aicodeba-968bfa99, after live probing: the function is in the repo, not yet in any successful Pages deploy, so the community tier is **not live on the origin** yet — every push to `main` re-triggers the deploy pipeline, and `web/test-api-workflows.js` fails until one ships the function (that failure is the retained deploy signal, do not delete the probe). The match-tier routes (registration, key rotation, predictions) answer 503 JSON with code `match_tier_offline` — acb-api remains undeployed and exposing it is still an operator decision. See `api-transport.md` for the full decision record and the deployment-state verification.)

(Live 2026-09-26, bead aicodeba-a34b8a80: the deploy shipped — `acb-site-pages-build-fq8tb` landed the function on the origin, the community tier is **live** (`GET /api/health` → 200 JSON with `feedback` and `map_votes` capabilities true), and the match-tier routes answer their designed 503 `match_tier_offline` envelopes. The "not live" paragraph above described the origin only until that deploy; `api-transport.md` "Deployment state" is the current record.)

## Match-tier contract decision (2026-09-26)

**Decision Date:** 2026-09-26
**Bead:** aicodeba-a0052569
**Decision:** DEFER. The five match-tier routes (`POST /api/register`,
`POST /api/rotate-key`, `POST /api/predict`, `GET /api/predictions/open`,
`GET /api/predictions/history`) keep their clients, pages, documentation, and
designed 503 `match_tier_offline` envelope exactly as shipped. Restoring
acb-api and its databases is **rejected for now**; ripping the routes out is
**rejected outright**.

### Why not restore (yet)

Standing acb-api back up is not a manifest away — it is operator-scale work,
and nothing currently consumes it:

- **The compute tier's namespace is still draining.** `ai-code-battle` on
  `apexalgo-iad` reads `Terminating` (read-only kubectl, 2026-09-26), and no
  `acb-api` Deployment runs in any fleet cluster (all eight read-only
  endpoints re-checked 2026-09-26). The 2026-07-21 decommission never
  finished reversing because nobody asked it to.
- **Restore needs more than the one Deployment.** Per
  `manifests/acb-api-deployment.yml` it wants the CNPG PostgreSQL cluster
  with its credentials (`acb-app-credentials-acb-app`) and Valkey
  (`keydb-secret`), a live image build (`acb-build.yml`'s registry parameters
  still point at the dead `forgejo.ardenone.com` host — see "Deployment
  state" in [api-transport.md](api-transport.md)), and — decisive — a route
  from the Cloudflare-edge Pages Function to the service, which a ClusterIP
  cannot provide. That last piece is the same public-exposure question this
  note descoped, plus secrets provisioning. Every item is operator +
  declarative-config territory; none is a repo-level change.
- **No live consumer.** Registration is announced closed on-site
  (`web/src/pages/docs.ts`), and the only callers of these routes are the
  tests and probes that assert the 503.

### Why not remove

- **The shapes are frozen by design.** `web/src/pages/docs-api.ts` states the
  request shape is frozen to the service contract; deleting the routes,
  clients, and pages (`#/compete/register`, predictions) would destroy
  working, tested UI for a platform feature that is still intended, and turn
  revival into a rewrite instead of the no-client-change flip
  [api-transport.md](api-transport.md) already specifies ("Reviving the
  match tier").
- **The offline contract is the designed answer, not a debris state.** The
  function's router refuses all five routes before any body read, rate
  limit, or storage write (verified in `web/src/lib/api-backend.ts`
  `handleApiRequest` — free at runtime), `GET /api/health` publishes the
  capability set so the state is observable, the SPA renders its unavailable
  states from the server's own envelope, and the unit suite, the e2e flows,
  and the deploy release gate all verify the five 503s against the live
  origin. Removal would delete that coverage along with the feature.
- **Honesty is already achieved where it matters.** The remaining
  misrepresentations were plan.md's ("built and deployed", acb-api owning
  the live community flows) — reconciled by the same bead as this decision.

### Revival triggers (revisit when any becomes true)

1. An operator stands the compute tier back up and provisions acb-api with
   PostgreSQL and Valkey (manifests, SealedSecrets/OpenBao, image CI).
2. A Pages-reachable route to the service exists — a registered domain
   IngressRoute or a Pages-compatible proxy target — i.e. the public-exposure
   decision this note descopes, un-descoped.
3. Self-serve registration or predictions is wanted again (social features,
   per plan.md §9.6).

Until then, the contract of record for the five routes is: **documented,
frozen, and answered 503 `match_tier_offline` by
`web/functions/api/[[path]].ts`** — transport state owned by
[api-transport.md](api-transport.md).

## Static-data endpoint decision (2026-09-26)

**Decision Date:** 2026-09-26
**Bead:** aicodeba-4be62ac3
**Decision:** RETIRE the docs page's advertisements of asset URLs nothing
serves; the replay/media pipeline they came from stays deferred under the
same revival triggers as the match tier above. Implementing the assets under
the `/api` Pages Function was considered and **rejected** — the function
would have no data to serve.

### What docs-api.ts advertised vs. what the origin serves

Audited live against `https://ai-code-battle.pages.dev` on 2026-09-26
(content-type is the signal — a missing Pages asset answers the 200
`text/html` SPA fallback, never a 404):

| Advertised on the page | Reality 2026-09-26 |
|---|---|
| `GET /data/leaderboard.json`, `/data/bots/index.json`, `/data/bots/{bot_id}.json`, `/data/matches/index.json`, `/data/playlists/index.json`, `/data/playlists/{slug}.json`, `/data/blog/index.json` | **Live** — the committed seed data in `web/public/data/` ships with every Pages deploy (all probed 200 `application/json`, including a real bot id and playlist slug) |
| `GET /data/blog/{slug}.json` | **Wrong path** — the live file is `/data/blog/posts/{slug}.json` (verified); docs corrected to the real path |
| `GET /replay-schema-v1.json` | **Live** (200 `application/json`) |
| `GET /evolution/live.json`, `/replays/{match_id}.json.gz`, `/matches/{match_id}.json`, `/cards/{bot_id}.png`, `/thumbnails/{match_id}.png` (both "B2" sections) | **Dead** — B2-origin-era paths served nowhere; the `b2.aicodebattle.com` host they predate is itself retired, and at the Pages origin each answers the SPA fallback HTML |
| `GET /maps/index.json`, `/maps/{map_id}.json` | **Dead on the origin** — but this base is the designed one: it is what the builder writes (`generator.go` `mapsDir`) and what the SPA client fetches (`api-types.ts` `fetchMapsIndex`); only the producer is missing |
| Interactive `/api/*` section | **Live/accurate** — community routes answer, match-tier routes answer their designed 503s (see the match-tier decision above) |

### Why retire rather than implement

- **The dead URLs were the same misrepresentation the 2026-09-25 descope
  removed from docs.ts**: the page pointed its "B2" sections at the Pages
  origin (`B2_BASE = PAGES_BASE`), where those paths answer HTML — a client
  following the docs gets a 200 that is not data.
- **The designed contract is real and implemented on both ends — only the
  producer is missing.** `cmd/acb-index-builder` bundles
  `data/replays/{id}.json.gz`, `data/cards/{id}.png`,
  `data/thumbnails/{id}.png`, `data/evolution/live.json` and `maps/` into
  the Pages deploy (`deploy.go` `bundleWarm*`), and the SPA already fetches
  those bases (`web/src/lib/replay-data.ts` `REPLAY_BASE`,
  `api-types.ts` `fetchEvolutionLive`/`fetchMapsIndex`). But the builder
  needs the compute tier's PostgreSQL, and B2 has no public hostname at all
  — operator-scale revival, not a repo-level change.
- **A function route would be a stub, not an implementation.** The R2
  bucket behind the `/api` function holds no replay or map keys (probed
  404 `text/plain` from the live `/r2/*` function on 2026-09-26), so
  "implementing" these under `/api` today could only serve an empty index —
  exactly the fake surface this bead exists to remove.
- **`GET /matches/{match_id}.json` is retired outright, not deferred**: its
  payload (win-prob curve, critical moments) lives in the replay itself
  (`win_prob`/`critical_moments`, written by the enrichment pass), and no
  SPA code has ever fetched a per-match metadata route.

### What changed (docs-api.ts)

1. Both "B2 Endpoints" sections are gone. The designed replay/media/map
   paths now appear under **"Replay & Media Assets (Pipeline Offline)"**,
   every card carrying an explicit `OFFLINE — not served until the
   index-builder pipeline runs` marker (the static-tier analogue of the
   match tier's `match_tier_offline` envelope) — and with their real bases
   (`/data/replays/…`, `/data/evolution/live.json`, `/data/cards/…`,
   `/data/thumbnails/…`, `/maps/…`), not the retired origin-root forms.
2. The blog-post path is corrected to `/data/blog/posts/{slug}.json`.
3. The intro no longer claims "there is no live API" (the community tier
   has been live since 2026-09-26 ~06:00Z) and states that offline endpoints
   are marked, not advertised.
4. The fetching-pattern example now uses the designed `/data/replays/` base
   and says when it answers with data; the rate-limit note covers the /api
   routes.
5. `web/src/pages/docs-api.test.ts` pins the honesty contract: no retired
   path, every pipeline endpoint OFFLINE-marked, the live tiers stay
   documented, and the real blog path is advertised.

### Revival trigger

The same list as the match tier above — an operator standing the compute
tier (and with it the index builder's database) back up. The docs section
flips from OFFLINE to served by dropping the markers; no shape change is
expected on either side of the contract. See the "Docs honesty" note in
[api-transport.md](api-transport.md) for how this sits next to the /api
transport record.
