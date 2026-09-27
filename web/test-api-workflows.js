#!/usr/bin/env node
/**
 * Live smoke: SPA→API workflows over the Pages Function transport (bead aicodeba-84d1d61b)
 *
 * Two halves:
 *
 *  A. Built site carries the transport contract — the production bundle
 *     (dist/assets/*.js) must carry the transport contract markers, and the
 *     function bundle (web/functions/) must sit beside it with both
 *     catch-alls present and wired (bead aicodeba-92bd4385): wrangler
 *     resolves the Pages Functions directory as <cwd>/functions at deploy
 *     time and silently skips the bundle when it is missing, so a catch-all
 *     absent here means the deploy about to happen ships the SPA alone.
 *     Run `npm run build` first; a stale dist fails here, which is the
 *     point.
 *
 *  B. Live origin transport — the deployed site must answer /api/* with the
 *     Pages Function's JSON (docs/notes/api-transport.md). Anything that
 *     answers text/html is the SPA fallback, i.e. the transport regressed to
 *     the pre-aicodeba-84d1d61b state — this script fails loudly so that
 *     cannot be missed. Every documented match-tier route (register,
 *     rotate-key, predict, predictions/open, predictions/history) must answer
 *     the 503 "match_tier_offline" envelope while acb-api is undeployed; the
 *     community reads must be live. Default probes are read-only or
 *     refused-with-503 — the match-tier branch short-circuits before any
 *     body is read or anything is stored, so nothing is written to the
 *     community store.
 *
 *  C. Live write-path probe — opt-in via ACB_WRITE_PROBE=1. Exercises the
 *     LIVE community routes end-to-end (bead aicodeba-046e3747): replay
 *     feedback POST → GET → upvote (plus its per-voter idempotency) and map
 *     vote POST → GET → vote switch. Everything is written under a
 *     `smoke-probe-write-<timestamp>` match/map id, a namespace no page ever
 *     renders (feedback and tallies are keyed per-match/per-map and the SPA
 *     only asks for real ids), so the probe leaves nothing user-visible
 *     behind and there is no delete route to clean up with.
 *
 *  D. Published replay retrieval — every match the bundled index advertises
 *     must be retrievable from the deployed origin as real replay JSON
 *     (bead aicodeba-26fa5fce). A 200 alone proves nothing: a missing deploy
 *     asset falls through to the SPA shell and answers 200 text/html, which
 *     is exactly how the published replays silently rotted to unretrievable.
 *     Each probe therefore asserts 200, not-text/html, gzip bytes, and a
 *     decoded replay with turns — the viewer's own decode path.
 *
 *  E. /api ↔ /r2 route isolation (bead aicodeba-a0c15607) — the origin ships
 *     two Pages Function mounts, and each must own exactly its own subtree at
 *     the platform routing layer, the one seam no offline test sees. /r2/*
 *     must be answered by the r2 function (text/plain seam answers while the
 *     bucket is empty, or real object metadata on a hit) — never text/html
 *     (the SPA fallback, i.e. the r2 catch-all missing from the deploy) and
 *     never the /api function's JSON envelopes. Conversely an r2-shaped path
 *     under /api must stay with the /api function's JSON 404. Every probe is
 *     a GET against a key nothing serves or stores, so it is read-only
 *     whatever the bucket's state.
 *
 * Override the origin with ACB_ORIGIN (e.g. a local `wrangler pages dev`).
 * Set ACB_SKIP_BUILD_CHECK=1 to gate the origin only when no local build is
 * present (scripts/verify-deployment.sh does this; the post-deploy CI gate
 * deliberately does not — there the fresh dist is the point).
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const assetsDir = path.join(__dirname, 'dist', 'assets');

const origin = process.env.ACB_ORIGIN || 'https://ai-code-battle.pages.dev';

const colors = {
  reset: '\x1b[0m', red: '\x1b[31m', green: '\x1b[32m',
  yellow: '\x1b[33m', blue: '\x1b[34m', cyan: '\x1b[36m',
};

function log(message, color = colors.reset) {
  console.log(`${color}${message}${colors.reset}`);
}

function logInfo(message) {
  log(`ℹ ${message}`, colors.blue);
}

function logTest(name, passed, message) {
  const icon = passed ? '✓' : '✗';
  const color = passed ? colors.green : colors.red;
  log(`${icon} ${name}: ${message}`, color);
  return passed;
}

let passed = 0;
let failed = 0;

// ─── Part A: built bundle carries the transport contract ─────────────────────

function checkBundle() {
  log('\n--- A. Built site carries the transport contract (dist/ + functions/) ---\n', colors.cyan);

  if (!fs.existsSync(assetsDir)) {
    logTest('dist/assets exists', false, 'not found — run `npm run build` in web/ first');
    failed++;
    return;
  }

  const files = fs.readdirSync(assetsDir).filter(f => f.endsWith('.js'));
  if (files.length === 0) {
    logTest('dist/assets JS chunks', false, 'no .js chunks found — run `npm run build` in web/ first');
    failed++;
    return;
  }
  logTest('dist/assets JS chunks', true, `${files.length} chunk(s)`);

  // The SPA bundle alone is not the deploy unit: wrangler resolves the
  // Pages Functions directory as <cwd>/functions — relative to the process
  // working directory, not to the assets dir it ships — and silently skips
  // the Functions bundle when that directory is missing (deploy-packaging
  // .test.ts guards the same rule offline; scripts/deploy-pages.sh trips
  // on it before deploying). Parts B–E prove the origin's answers after
  // the fact; this proves the site ABOUT to be deployed contains the
  // function bundle before anything ships (bead aicodeba-92bd4385). Each
  // catch-all is checked for its wiring marker, not mere existence: an
  // empty or stubbed file would pass an existsSync check and still not be
  // the function the transport contract ships.
  const functionsDir = path.join(__dirname, 'functions');
  const catchAlls = [
    ['functions/api/[[path]].ts', path.join(functionsDir, 'api', '[[path]].ts'), 'handleApiRequest'],
    ['functions/r2/[[path]].ts', path.join(functionsDir, 'r2', '[[path]].ts'), 'ACB_BUCKET'],
  ];
  for (const [name, file, marker] of catchAlls) {
    if (!fs.existsSync(file)) {
      failed++;
      logTest(name, false,
        'missing — wrangler resolves <cwd>/functions and silently skips the Functions bundle ' +
        'when it is absent; run the deploy (and this smoke) from web/ or every /api and /r2 ' +
        'route falls back to the SPA');
      continue;
    }
    if (fs.readFileSync(file, 'utf8').includes(marker)) {
      passed++;
      logTest(name, true, `present beside dist/ (wiring marker "${marker}" found)`);
    } else {
      failed++;
      logTest(name, false,
        `"${marker}" missing from the catch-all — not the function bundle the transport contract ships`);
    }
  }

  const bundle = files
    .map(f => fs.readFileSync(path.join(assetsDir, f), 'utf8'))
    .join('\n');

  // Each marker must be a contiguous substring on one source line, so it
  // survives minification as an unbroken string literal in the page chunk.
  // (Runtime-computed literals only: with API_TRANSPORT_ENABLED true the
  // minifier folds away the kill-switch branches and their strings.)
  const markers = [
    // The match-tier offline envelope the clients detect (api-transport.ts).
    ['match-tier offline code', 'match_tier_offline'],
    // The live map-vote client (api-types.ts voter identity).
    ['map-vote voter id', 'acb_voter_id'],
    // The live annotation client (components/annotation.ts local copy).
    ['annotation local store', 'acb_annotations_v2'],
  ];

  for (const [name, marker] of markers) {
    if (bundle.includes(marker)) {
      passed++;
      logTest(name, true, `found "${marker}"`);
    } else {
      failed++;
      logTest(name, false, `"${marker}" missing — rebuild or re-check the transport contract`);
    }
  }
}

// ─── Part B: live origin serves the /api Pages Function ──────────────────────

async function fetchProbe(route, method, body) {
  const url = `${origin}${route}`;
  try {
    const res = await fetch(url, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    const contentType = res.headers.get('content-type') || '';
    const text = await res.text();
    let json = null;
    try {
      json = JSON.parse(text);
    } catch {
      // non-JSON body: callers branch on contentType
    }
    return { status: res.status, contentType, json };
  } catch (e) {
    return { status: 0, contentType: String(e?.message || e), json: null };
  }
}

async function expectJson(name, route, method, body, verify) {
  const { status, contentType, json } = await fetchProbe(route, method, body);
  logInfo(`probed ${method} ${origin}${route} → ${status} ${contentType}`);

  if (!contentType.includes('application/json') || json === null) {
    failed++;
    logTest(
      name,
      false,
      `answered ${status} ${contentType || '(no content-type)'} — /api is not answering JSON. ` +
        'If this is text/html, the Pages Function is missing from the deploy ' +
        '(web/functions/api/) and the SPA fallback is answering again.',
    );
    return;
  }
  const problem = verify(json, status);
  if (problem) {
    failed++;
    logTest(name, false, problem);
  } else {
    passed++;
    logTest(name, true, `${status} JSON as expected`);
  }
}

/**
 * Every documented match-tier route must answer the honest offline envelope:
 * 503 JSON with code "match_tier_offline" and a user-facing message (the
 * capability table in docs/notes/api-transport.md). The router short-circuits
 * these before the body is read or the rate limiter/storage run, so probing
 * them is free and writes nothing — and each one proves that route is owned
 * by the function, not the SPA fallback.
 */
function expectMatchTierOffline(name, route, method, body) {
  return expectJson(name, route, method, body, (json, status) => {
    if (status !== 503) return `expected 503, got ${status}`;
    if (json.code !== 'match_tier_offline') return `expected code "match_tier_offline", got ${JSON.stringify(json.code)}`;
    if (typeof json.error !== 'string' || !json.error) return 'user-facing error message missing';
    return null;
  });
}

// ─── Part C: live write-path probe (opt-in) ──────────────────────────────────

function logCheck(name, ok, message) {
  logTest(name, ok, message);
  if (ok) passed++; else failed++;
}

/**
 * Round-trip the LIVE community write routes against the origin. Runs only
 * with ACB_WRITE_PROBE=1 — see the header for why this cannot pollute
 * anything a visitor sees.
 */
async function probeWritePaths() {
  log('\n--- C. Live write-path probe (ACB_WRITE_PROBE=1) ---\n', colors.cyan);
  if (process.env.ACB_WRITE_PROBE !== '1') {
    logInfo('skipped — set ACB_WRITE_PROBE=1 to exercise the community write routes');
    return;
  }

  // Timestamped ids keep re-runs independent; both pass the server's
  // isId() pattern ([A-Za-z0-9_.:-]{1,128}).
  const stamp = new Date().toISOString().replace(/[^0-9]/g, '').slice(0, 14);
  const probeId = `smoke-probe-write-${stamp}`;
  const voterId = `smoke-probe-voter-${stamp}`;
  const body =
    'Automated transport write-probe from web/test-api-workflows.js. ' +
    'This entry lives under a probe match id no replay page ever loads.';
  let feedbackId = null;

  logInfo(`probe namespace: match/map id "${probeId}"`);

  // 1. Replay feedback POST → 201 recorded with an id.
  let res = await fetchProbe('/api/feedback', 'POST', {
    match_id: probeId, turn: 0, type: 'idea', body, author: 'acb smoke probe',
  });
  logCheck(
    'feedback POST recorded',
    res.status === 201 && res.json?.status === 'recorded' && typeof res.json?.feedback_id === 'string',
    res.status === 201 ? `201, feedback_id ${res.json?.feedback_id}` : `${res.status} ${JSON.stringify(res.json)}`,
  );
  feedbackId = res.json?.feedback_id;

  // 2. Feedback GET shows the entry with zero upvotes.
  res = await fetchProbe(`/api/feedback/${probeId}`, 'GET', null);
  const entry = Array.isArray(res.json?.feedback)
    ? res.json.feedback.find((f) => f.feedback_id === feedbackId)
    : null;
  logCheck(
    'feedback GET round-trips the entry',
    res.status === 200 && entry !== undefined && entry !== null && entry.upvotes === 0,
    entry ? `found ${feedbackId}, upvotes ${entry.upvotes}` : `${res.status}, entry missing`,
  );

  // 3. Upvote recorded, then idempotent for the same voter.
  if (feedbackId) {
    res = await fetchProbe(`/api/feedback/${feedbackId}/upvote`, 'POST', { voter_id: voterId });
    logCheck('feedback upvote recorded', res.status === 200 && res.json?.status === 'recorded',
      `${res.status} ${JSON.stringify(res.json)}`);

    res = await fetchProbe(`/api/feedback/${feedbackId}/upvote`, 'POST', { voter_id: voterId });
    logCheck('feedback upvote idempotent per voter',
      res.status === 200 && res.json?.status === 'already_upvoted',
      `${res.status} ${JSON.stringify(res.json)}`);
  } else {
    logCheck('feedback upvote recorded', false, 'no feedback_id from the POST — skipping');
    logCheck('feedback upvote idempotent per voter', false, 'no feedback_id from the POST — skipping');
  }

  // 4. GET reflects exactly one upvote.
  res = await fetchProbe(`/api/feedback/${probeId}`, 'GET', null);
  const upvoted = Array.isArray(res.json?.feedback)
    ? res.json.feedback.find((f) => f.feedback_id === feedbackId)
    : null;
  logCheck('feedback GET reflects the upvote',
    res.status === 200 && upvoted?.upvotes === 1,
    upvoted ? `upvotes ${upvoted.upvotes}` : `${res.status}, entry missing`);

  // 5. Map vote POST → tallied; GET reads it back with my_vote.
  res = await fetchProbe('/api/vote/map', 'POST', { map_id: probeId, voter_id: voterId, vote: 1 });
  logCheck('map vote POST recorded',
    res.status === 200 && res.json?.vote === 1 && res.json?.net_votes === 1,
    `${res.status} ${JSON.stringify(res.json)}`);

  res = await fetchProbe(`/api/vote/map/${probeId}?voter_id=${encodeURIComponent(voterId)}`, 'GET', null);
  logCheck('map vote GET reads the tally back',
    res.status === 200 && res.json?.net_votes === 1 && res.json?.my_vote === 1,
    `${res.status} ${JSON.stringify(res.json)}`);

  // 6. Same voter switching to -1 overwrites rather than adding a voter.
  res = await fetchProbe('/api/vote/map', 'POST', { map_id: probeId, voter_id: voterId, vote: -1 });
  logCheck('map vote switch overwrites per voter',
    res.status === 200 && res.json?.vote === -1 && res.json?.net_votes === -1,
    `${res.status} ${JSON.stringify(res.json)}`);
}

// ─── Part D: published replay retrieval (the SPA-fallback mask) ──────────────

// The match index published with the deploy advertises matches; the SPA's only
// replay source is the bundled asset /data/replays/<id>.json.gz
// (web/src/lib/replay-data.ts REPLAY_BASE — B2 is the cold archive, R2 the
// /r2 function, and the viewer fetches neither). A status-200 probe is not
// enough: a missing deploy asset falls through to the SPA shell and answers
// 200 text/html, which is exactly how every advertised replay silently rotted
// to unretrievable while status checks stayed green (bead aicodeba-26fa5fce).
// So each advertised replay must answer 200, must NOT answer text/html, and
// must decode to replay JSON with a non-empty turns array — the same decode
// path the viewer runs (replay-data.ts: manual gunzip unless the transport
// already set Content-Encoding).
async function probePublishedReplays() {
  log('\n--- D. Published replay retrieval (no SPA-fallback mask) ---\n', colors.cyan);

  const indexPath = path.join(__dirname, 'public', 'data', 'matches', 'index.json');
  if (!fs.existsSync(indexPath)) {
    logInfo('no published match index — nothing advertised, nothing to retrieve');
    return;
  }
  const index = JSON.parse(fs.readFileSync(indexPath, 'utf-8'));
  const matches = index.matches || [];
  if (matches.length === 0) {
    logInfo('match index is empty — nothing advertised, nothing to retrieve');
    return;
  }

  // Bound the smoke: the first entries the index advertises.
  const probed = matches.slice(0, 8);

  for (const match of probed) {
    const url = `${origin}/data/replays/${match.id}.json.gz`;
    try {
      const res = await fetch(url);
      const contentType = res.headers.get('content-type') || '';
      if (res.status !== 200) {
        failed++;
        logTest(`replay retrievable: ${match.id}`, false, `HTTP ${res.status} ${contentType} — published replay not retrievable: ${url}`);
        continue;
      }
      if (contentType.includes('text/html')) {
        failed++;
        logTest(`replay retrievable: ${match.id}`, false, `200 text/html — the SPA fallback masked a missing deploy asset: ${url}`);
        continue;
      }
      const buf = Buffer.from(await res.arrayBuffer());
      const transportDecoded = ['gzip', 'x-gzip', 'deflate', 'br', 'zstd']
        .includes((res.headers.get('content-encoding') || '').toLowerCase());
      let text;
      if (!transportDecoded) {
        if (buf.length < 2 || buf[0] !== 0x1f || buf[1] !== 0x8b) {
          failed++;
          logTest(`replay retrievable: ${match.id}`, false, `200 ${contentType} but body is not gzip (${buf.length} bytes) — ${url}`);
          continue;
        }
        const stream = new Response(buf).body.pipeThrough(new DecompressionStream('gzip'));
        text = await new Response(stream).text();
      } else {
        text = buf.toString('utf8');
      }
      let replay;
      try {
        replay = JSON.parse(text);
      } catch (e) {
        failed++;
        logTest(`replay retrievable: ${match.id}`, false, `replay body does not parse as JSON: ${e.message} — ${url}`);
        continue;
      }
      if (!Array.isArray(replay.turns) || replay.turns.length === 0) {
        failed++;
        logTest(`replay retrievable: ${match.id}`, false, `replay JSON has no turns array (keys: ${Object.keys(replay).slice(0, 6).join(', ')}) — ${url}`);
        continue;
      }
      passed++;
      logTest(`replay retrievable: ${match.id}`, true, `200 ${contentType}, ${replay.turns.length} turns of real replay JSON`);
    } catch (e) {
      failed++;
      logTest(`replay retrievable: ${match.id}`, false, `${e?.message || e} — ${url}`);
    }
  }
}

// ─── Part E: /api ↔ /r2 route isolation ──────────────────────────────────────

/**
 * /r2/* must always be answered by the r2 function. Its signatures are a
 * text/plain answer (404 "Not Found" on a missing key, 503 "R2 binding not
 * configured" on a missing binding — the acb-data bucket is empty pending
 * operator-issued R2 credentials, R2_ACCESS_KEY_SOURCE.md, so that is the
 * expected live seam today) or a real object's own metadata on a hit.
 *
 *   - text/html anywhere is the SPA fallback, i.e. the r2 catch-all is
 *     missing from the deploy — the same silent-skip packaging rot
 *     deploy-packaging.test.ts guards offline, now gated on the live origin.
 *   - a JSON body is the /api function bleeding across the mount boundary.
 *     Safe to assert only because every probe below hits a key nothing
 *     stores: the /r2 function can answer JSON only by serving a stored
 *     object, and no writer ever creates these keys.
 */
async function expectR2Seam(name, route) {
  const { status, contentType, json } = await fetchProbe(route, 'GET', null);
  logInfo(`probed GET ${origin}${route} → ${status} ${contentType}`);

  let problem = null;
  if (contentType.includes('text/html')) {
    problem = `answered ${status} text/html — the SPA fallback is answering /r2, i.e. ` +
      'web/functions/r2/ is missing from the deploy (the same silent-skip packaging ' +
      'rot the /api catch-all guard exists for)';
  } else if (json !== null) {
    problem = `answered ${status} ${contentType} with a JSON body — the /api function is ` +
      'answering inside /r2; the two function mounts are not isolated';
  } else if ((status === 404 || status === 503) && !contentType.includes('text/plain')) {
    problem = `${status} answered ${contentType} — expected the r2 function's text/plain seam`;
  }

  if (problem) {
    failed++;
    logTest(name, false, problem);
  } else {
    passed++;
    logTest(name, true, `${status} ${contentType || '(no content-type)'} — r2 function owns it`);
  }
}

async function probeRouteIsolation() {
  log('\n--- E. /api ↔ /r2 route isolation ---\n', colors.cyan);

  // 1. The r2 catch-all owns /r2/* — a missing replay key answers the
  //    function's text/plain 404, never the SPA fallback.
  await expectR2Seam(
    'r2 catch-all owns /r2/* (not the SPA fallback)',
    '/r2/replays/route-isolation-probe-a0c15607.json.gz',
  );

  // 2. The bare /r2/ root (an empty object key) is still the function's.
  await expectR2Seam('bare /r2/ root answers the r2 function', '/r2/');

  // 3. No cross-serve /r2 → /api: an api-shaped path under /r2 is a bucket
  //    key lookup, never the /api function's health envelope.
  await expectR2Seam('/r2/api/health is not the /api function', '/r2/api/health');

  // 4. No cross-serve /api → /r2: an r2-shaped path under /api stays with
  //    the /api function's JSON 404 — the r2 function's text/plain seam must
  //    not answer inside /api.
  await expectJson(
    '/api path shaped like an /r2 key stays with the /api function',
    '/api/r2/replays/route-isolation-probe-a0c15607.json.gz',
    'GET',
    null,
    (json, status) => {
      if (status !== 404) return `expected 404, got ${status}`;
      if (json.error !== 'not found') return `expected error "not found", got ${JSON.stringify(json.error)}`;
      return null;
    },
  );
}

async function main() {
  log('\n=== SPA→API Workflows Smoke (Pages Function transport) ===\n', colors.cyan);
  logInfo(`live origin: ${origin} (override with ACB_ORIGIN)`);

  if (process.env.ACB_SKIP_BUILD_CHECK === '1') {
    logInfo('Part A skipped — ACB_SKIP_BUILD_CHECK=1 (origin-only mode)');
  } else {
    checkBundle();
  }

  log('\n--- B. Live origin transport (/api answers JSON) ---\n', colors.cyan);

  // The transport's proof of life and its live capability set.
  await expectJson('health + capabilities', '/api/health', 'GET', null, (json, status) => {
    if (status !== 200) return `expected 200, got ${status}`;
    if (json.status !== 'ok') return `expected status "ok", got ${JSON.stringify(json.status)}`;
    const caps = json.capabilities;
    if (!caps || typeof caps !== 'object') return 'capabilities object missing';
    for (const key of ['register', 'rotate_key', 'predictions', 'feedback', 'map_votes']) {
      if (typeof caps[key] !== 'boolean') return `capability "${key}" is not a boolean`;
    }
    if (caps.feedback !== true || caps.map_votes !== true) {
      return `community capabilities are off (feedback=${caps.feedback}, map_votes=${caps.map_votes}) — storage is unhealthy`;
    }
    return null;
  });

  // Community reads are live (both read-only).
  await expectJson(
    'map vote tallies read',
    '/api/vote/map/probe-nonexistent-map-84d1d61b',
    'GET',
    null,
    (json, status) => {
      if (status !== 200) return `expected 200, got ${status}`;
      if (json.map_id !== 'probe-nonexistent-map-84d1d61b') return 'map_id not echoed';
      if (typeof json.net_votes !== 'number') return 'net_votes is not a number';
      return null;
    },
  );

  await expectJson(
    'replay feedback read',
    '/api/feedback/probe-nonexistent-match-84d1d61b',
    'GET',
    null,
    (json, status) => {
      if (status !== 200) return `expected 200, got ${status}`;
      if (!Array.isArray(json.feedback)) return 'feedback array missing';
      return null;
    },
  );

  // Every documented match-tier route answers the honest offline envelope
  // (no write happens — the router refuses these before reading a body).
  await expectMatchTierOffline(
    'register answers match_tier_offline',
    '/api/register',
    'POST',
    { name: 'smoke-probe-84d1d61b', endpoint_url: 'https://example.com/move', owner_id: 'smoke-probe' },
  );
  await expectMatchTierOffline(
    'rotate-key answers match_tier_offline',
    '/api/rotate-key',
    'POST',
    { key_id: 'smoke-probe-84d1d61b' },
  );
  await expectMatchTierOffline(
    'predict answers match_tier_offline',
    '/api/predict',
    'POST',
    { match_id: 'smoke-probe-84d1d61b' },
  );
  await expectMatchTierOffline('predictions/open answers match_tier_offline', '/api/predictions/open', 'GET', null);
  await expectMatchTierOffline(
    'predictions/history answers match_tier_offline',
    '/api/predictions/history',
    'GET',
    null,
  );

  // An unknown /api path must still be answered by the function — a JSON 404,
  // never the SPA fallback. The offline suites pin the handler's router
  // (JSON 404 for unknown routes); this pins the deployed side of that seam:
  // functions/api/[[path]].ts is a Pages catch-all, so every /api/* path is
  // handed to the handler at the platform routing layer, which no offline
  // test sees. If that catch-all ever stops intercepting (renamed away from
  // [[path]], a shadowing route, a routing regression), this starts answering
  // 200 text/html and the smoke fails loudly.
  await expectJson(
    'unknown /api path answers JSON 404 (not the SPA fallback)',
    '/api/not-a-route-4a9e0c6a',
    'GET',
    null,
    (json, status) => {
      if (status !== 404) return `expected 404, got ${status}`;
      if (json.error !== 'not found') return `expected error "not found", got ${JSON.stringify(json.error)}`;
      return null;
    },
  );

  // The SPA itself is still served normally.
  const home = await fetchProbe('/', 'GET', null);
  if (home.status === 200 && home.contentType.includes('text/html')) {
    passed++;
    logTest('SPA still served at /', true, '200 text/html');
  } else {
    failed++;
    logTest('SPA still served at /', false, `unexpected: ${home.status} ${home.contentType}`);
  }

  await probeWritePaths();

  await probePublishedReplays();

  await probeRouteIsolation();

  log('');
  if (failed > 0) {
    log(`✗ FAILED: ${failed} check(s) failed, ${passed} passed\n`, colors.red);
    process.exit(1);
  }
  log(`✓ All ${passed} checks passed\n`, colors.green);
}

main().catch(e => {
  console.error(e);
  process.exit(1);
});
