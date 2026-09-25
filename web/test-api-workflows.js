#!/usr/bin/env node
/**
 * Live smoke: SPA→API workflows disabled without a transport (bead aicodeba-07caa4ec)
 *
 * Two halves:
 *
 *  A. Local build markers — the production bundle (dist/assets/*.js) must
 *     carry the per-workflow "unavailable" notices (registration,
 *     predictions, community feedback, map voting). Run `npm run build`
 *     first; a stale dist fails here, which is the point.
 *
 *  B. Live origin premise — the deployed site must still answer /api/* with
 *     the SPA HTML fallback (200, text/html). That fallback is exactly why
 *     the workflows are disabled; if it ever returns JSON, a transport has
 *     appeared and web/src/lib/api-transport.ts (API_TRANSPORT_ENABLED)
 *     needs flipping back on — this script fails loudly so that cannot be
 *     missed.
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

function logTest(name, passed, message) {
  const icon = passed ? '✓' : '✗';
  const color = passed ? colors.green : colors.red;
  log(`${icon} ${name}: ${message}`, color);
  return passed;
}

let passed = 0;
let failed = 0;

// ─── Part A: built bundle carries the disable notices ────────────────────────

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
  const markers = [
    ['register notice', 'registration API has no public endpoint'],
    ['predictions banner', 'Predictions are view-only right now'],
    ['predictions open-matches notice', 'Predicting is unavailable right now'],
    ['feedback banner', 'Community sync is unavailable'],
    ['feedback saved-locally status', 'Annotation saved in this browser only'],
    ['map-vote notice', 'Map voting is unavailable'],
  ];

  for (const [name, marker] of markers) {
    if (bundle.includes(marker)) {
      passed++;
      logTest(name, true, `found "${marker}"`);
    } else {
      failed++;
      logTest(name, false, `"${marker}" missing — rebuild or re-check the disabled states`);
    }
  }
}

// ─── Part B: live origin still has no /api transport ─────────────────────────

async function fetchProbe(url, method, body) {
  try {
    const res = await fetch(url, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    const contentType = res.headers.get('content-type') || '';
    res.body?.cancel?.().catch(() => {});
    return { status: res.status, contentType };
  } catch (e) {
    return { status: 0, contentType: String(e?.message || e) };
  }
}

async function checkLivePremise(route, method, body) {
  const url = `${origin}${route}`;
  const { status, contentType } = await fetchProbe(url, method, body);
  logInfo(`probed ${method} ${url} → ${status} ${contentType}`);

  // A JSON body on an /api route means something answered as an API.
  if (contentType.includes('application/json')) {
    failed++;
    return logTest(
      `${route} premise`,
      false,
      'answered with JSON — a transport appears LIVE: flip API_TRANSPORT_ENABLED in web/src/lib/api-transport.ts and remove the disabled states',
    );
  }

  // No transport, two shapes: GETs fall through to the SPA (200 text/html),
  // and writes are refused by Pages before any fallback (405; unmatched
  // routes can also surface 404). Either way no server saw the request.
  const noTransport =
    (status === 200 && contentType.includes('text/html')) ||
    status === 404 || status === 405;
  if (noTransport) {
    passed++;
    return logTest(
      `${route} premise`,
      true,
      `${status} ${contentType || '(no content-type)'} — no transport on /api (disable decision holds)`,
    );
  }

  failed++;
  return logTest(
    `${route} premise`,
    false,
    `unexpected answer for a transport-less /api route: ${status} ${contentType}`,
  );
}

function logInfo(message) {
  log(`ℹ ${message}`, colors.blue);
}

async function main() {
  log('\n=== SPA→API Workflows Smoke (transport disabled) ===\n', colors.cyan);
  logInfo(`live origin: ${origin} (override with ACB_ORIGIN)`);

  checkBundle();

  log('\n--- B. Live origin premise (/api is the SPA fallback) ---\n', colors.cyan);
  await checkLivePremise('/api/register', 'POST', {
    name: 'smoke-probe', endpoint_url: 'https://example.com/move', owner_id: 'smoke',
  });
  await checkLivePremise('/api/predictions/open', 'GET', null);
  await checkLivePremise('/api/vote/map/probe', 'GET', null);

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
