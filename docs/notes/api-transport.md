# SPA→API Transport — Decision Record

**Decision Date:** 2026-09-25
**Bead:** aicodeba-84d1d61b
**Status:** IMPLEMENTED (community tier live; match tier offline by design)

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

## Capability split

The function implements what its storage can support and is honest about the
rest:

| Flow | Route | Status |
|---|---|---|
| Replay feedback (annotations) | `POST /api/feedback`, `GET /api/feedback/{match_id}`, `POST /api/feedback/{id}/upvote` | **LIVE** |
| Agentation site feedback | `POST /api/feedback` (body carries `markdown`) | **LIVE** |
| Map voting | `POST /api/vote/map`, `GET /api/vote/map/{map_id}` | **LIVE** |
| Bot registration / key rotation | `POST /api/register`, `POST /api/rotate-key` | **503** `match_tier_offline` |
| Predictions | `GET /api/predictions/open`, `GET /api/predictions/history`, `POST /api/predict` | **503** `match_tier_offline` |

The match-tier flows need acb-api's PostgreSQL/Valkey backend, which is not
deployed anywhere (compute tier decommissioned 2026-07-21; reviving it is an
operator decision — see `public-api-descope.md`). Those routes answer
**503 JSON with code `match_tier_offline`** so the SPA renders its
unavailable states from the server's answer instead of a compile-time flag.
When acb-api is revived and exposed, the function becomes a thin proxy for
those routes and **no client change is needed**.

Request/response shapes mirror `cmd/acb-api/server.go` so the SPA clients in
`web/src/api-types.ts` and `web/src/components/annotation.ts` work against
either backend unchanged.

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

- **Probe:** `npm run test:api-workflows` in `web/` (read-only against the
  live origin) — fails loudly if `/api` ever stops answering JSON.
- **Reviving the match tier:** deploy acb-api with its databases, expose it
  to the function (same-zone service or a Pages-compatible route), then
  replace the `matchTierOffline()` branch in `web/src/lib/api-backend.ts`
  with a proxy. The routes, shapes, and clients stay as-is.

## Testing

- `web/src/lib/api-backend.test.ts` — server contract against an in-memory
  CAS bucket: routing, validation, spam/rate limits, vote + feedback round
  trips, CAS contention, match-tier 503s.
- `web/src/api-transport.test.ts`, `web/src/components/annotation.test.ts`,
  `web/src/pages/register.test.ts`, `web/src/pages/predictions.test.ts`,
  `web/src/pages/replay-mapvote.test.ts` — client and page behavior with the
  transport live (and the HTML-fallback backstops).
