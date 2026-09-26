#!/usr/bin/env node
/**
 * Live smoke: SPA→API workflows over the Pages Function transport (bead aicodeba-84d1d61b)
 *
 * Two halves:
 *
 *  A. Local build markers — the production bundle (dist/assets/*.js) must
 *     carry the transport contract markers. Run `npm run build` first; a
 *     stale dist fails here, which is the point.
 *
 *  B. Live origin transport — the deployed site must answer /api/* with the
 *     Pages Function's JSON (docs/notes/api-transport.md). Anything that
 *     answers text/html is the SPA fallback, i.e. the transport regressed to
 *     the pre-aicodeba-84d1d61b state — this script fails loudly so that
 *     cannot be missed. Match-tier routes must answer 503 JSON with code
 *     "match_tier_offline" while acb-api is undeployed; the community reads
 *     must be live. Default probes are read-only or refused-with-503 —
 *     nothing is written to the community store.
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
 * Override the origin with ACB_ORIGIN (e.g. a local `wrangler pages dev`).
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
  log('\n--- A. Local build markers (dist/assets) ---\n', colors.cyan);

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

async function main() {
  log('\n=== SPA→API Workflows Smoke (Pages Function transport) ===\n', colors.cyan);
  logInfo(`live origin: ${origin} (override with ACB_ORIGIN)`);

  checkBundle();

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

  // Match-tier routes answer the honest offline envelope (no write happens).
  await expectJson(
    'register answers match_tier_offline',
    '/api/register',
    'POST',
    { name: 'smoke-probe-84d1d61b', endpoint_url: 'https://example.com/move', owner_id: 'smoke-probe' },
    (json, status) => {
      if (status !== 503) return `expected 503, got ${status}`;
      if (json.code !== 'match_tier_offline') return `expected code "match_tier_offline", got ${JSON.stringify(json.code)}`;
      if (typeof json.error !== 'string' || !json.error) return 'user-facing error message missing';
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
