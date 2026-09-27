# Replay Serving — Retrieval Contract

**Date:** 2026-09-27
**Bead:** aicodeba-26fa5fce
**Status:** IMPLEMENTED (bundled-asset path — 8 replays bundled, index trued up); R2 path blocked on credentials

## The two retrieval sources

| Source | URL | Filled by | SPA uses it |
|---|---|---|---|
| Bundled Pages asset | `/data/replays/<id>.json.gz` | Files committed under `web/public/data/replays/` (vite copies `public/` into the deploy), plus `bundleWarmReplays` in the index-builder | **Yes — the only source** (`web/src/lib/replay-data.ts` `REPLAY_BASE`) |
| R2 Pages Function | `/r2/<key>` | R2 bucket `acb-data` (`web/functions/r2/[[path]].ts`) | No |

B2 (`acb-replays`) is the private cold archive the worker uploads to; R2
`acb-data` is the bucket behind the `/r2` function. Neither is fetched by the
viewer — a replay is "published" only when its `.json.gz` is bundled into the
Pages deploy.

## Failure mode: the SPA-fallback mask

A Pages request for a deploy asset that does not exist falls through to the
SPA shell and answers **200 text/html**. Status-code probes therefore pass
while the viewer gets HTML it cannot parse. This is how all eight matches in
`web/public/data/matches/index.json` were advertised with Watch Replay links
that served the SPA shell (bead aicodeba-26fa5fce): the index was committed
without the matching `data/replays/` assets.

Verification must branch on **content-type and body**, never on status alone.

## Regression gate

- `web/test-api-workflows.js` part D (runs as the `acb-site-pages-build`
  post-deploy release gate): for each advertised match, `200`, not
  `text/html`, gzip magic bytes, and a decode to replay JSON with a non-empty
  `turns` array — the same decode path as the viewer.
- `web/test-match-list.js` test 3: the bundled-asset probe is a hard failure
  on anything but 200 non-HTML; the `/r2` probe stays informational.

## Bundling a produced replay

`scripts/generate-test-replays.sh` produces **non-deterministic** matches:
the map walls are seeded from the wall clock, so two runs at the same commit
produce different games (only the bot roster per id is pinned by the script).
The contract is the **ids**, never the bytes. Output goes to `test-replays/`
(gitignored); copy the `.json.gz` files into `web/public/data/replays/` and
commit. Real pipeline equivalents: `acb-worker` uploads to B2,
`acb-index-builder` `bundleWarmReplays` pulls the warm set back into its
Pages output dir.

**The index must be trued up from whatever actually shipped.** The original
`index.json` was hand-authored fiction (April dates, 487–500 turn counts —
impossible under the current engine's 320-turn 1v1 cap — and winners no
regeneration can reproduce). The checked-in index is now derived from the
checked-in bundles: participants/scores from `result.scores`, `winner_id`
from `result.winner` (`null` on a draw — the schema and the matches page both
handle it), `turns`/`end_reason` from `result`, `completed_at` from
`end_time`, `enriched: false` (nothing narrated these; `true` renders a
"Narrated — AI commentary" badge on bot profiles and biases home-page
featuring). Re-bundle and the index drifts again — regenerate both together.

## R2 `acb-data`: empty, blocked on credentials

Both credential legs are dead (checked 2026-09-27, values never read —
property assertions only):

- `secret/rs-manager/ai-code-battle/r2` (v1, never rotated): `access-key` is
  a well-formed 32-char key, but `secret-key` is 65 chars and URL-shaped —
  the documented swap in `R2_ACCESS_KEY_SOURCE.md`, still the only version.
  S3 auth cannot succeed.
- `secret/rs-manager/ai-code-battle/cloudflare` (v1): the account API token
  fails `/user/tokens/verify` ("Invalid API Token").

The iad-ci `acb-site-pages-build` workflowtemplate does hold a live
`CLOUDFLARE_API_TOKEN` (it deploys Pages on every push to main), but cluster
secrets are outside every agent identity's reach (read-only proxy: `secrets
is forbidden`) and its scopes are unknown. Issuing an R2 API token is a
Cloudflare-dashboard action no agent identity can perform. Until an operator
mints one, `/r2/replays/<id>.json.gz` answers 404 text/plain — the
function's own "not found" over a live seam, not the SPA fallback (the
function and binding are deployed and working).

## R2 `acb-data`: empty, blocked on credentials

Both credential legs are dead (checked 2026-09-26, values never read —
property assertions only):

- `secret/rs-manager/ai-code-battle/r2`: `secret-key` is URL-shaped (the
  documented swap in `R2_ACCESS_KEY_SOURCE.md`, still the only version);
  S3 auth cannot succeed.
- `secret/rs-manager/ai-code-battle/cloudflare`: the account API token fails
  `/user/tokens/verify` ("Invalid API Token").

Issuing an R2 API token is a Cloudflare-dashboard action no agent identity
can perform. Until an operator mints one, `/r2/replays/<id>.json.gz` answers
404 text/plain — the function's own "not found" over a live seam, not the SPA
fallback (the function and binding are deployed and working).
