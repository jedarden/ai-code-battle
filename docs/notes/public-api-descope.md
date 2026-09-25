# Public API Endpoint — Descope Decision

**Decision Date:** 2026-09-25
**Bead:** aicodeba-a42aec94
**Status:** DESCOPED

## Problem

User-facing documentation advertised a public API endpoint that does not exist:

- `web/src/pages/docs.ts` showed `curl -X POST https://ai-code-battle.pages.dev/api/register` as the way to register a bot.
- `docs/notes/canonical-public-domain.md` listed "API endpoint (deferred): Would be `api.ai-code-battle.pages.dev` via Traefik routing".
- No open bead tracks implementing a public route, and both candidate hostnames are dead.

## Evidence (probed live 2026-09-25)

- `aicodebattle.com` is NXDOMAIN — the zone was never registered (DEPLOYMENT.md states the same), so `api.aicodebattle.com` can never resolve.
- `api.ai-code-battle.pages.dev` resolves only through Cloudflare's `*.pages.dev` anycast and serves Cloudflare's 404 "no project" page. A Pages project gets exactly one `<project>.pages.dev` hostname; a nested `api.` name under it cannot be claimed.
- `https://ai-code-battle.pages.dev/api/register` and `/api/bots` return the SPA's `index.html` (`content-type: text/html`) via the Pages SPA fallback — there is no `/api` route on Pages (`web/functions/` contains only `r2/`). A real user POSTing the old docs example gets HTML, and `registerBot()` then fails parsing it.
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
