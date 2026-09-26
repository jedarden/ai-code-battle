/**
 * Live end-to-end flows over the /api transport (bead aicodeba-6c793229).
 *
 * The unit suites (src/pages/register.test.ts, src/pages/predictions.test.ts,
 * src/pages/replay-mapvote.test.ts, src/api-transport.test.ts) pin each
 * client's contract with a mocked fetch; web/test-api-workflows.js probes
 * the origin with raw HTTP. Neither can say that the REAL client code — the
 * page modules and the /api client functions a visitor's browser runs —
 * completes its flow against the REAL origin. This suite can: no fetch is
 * mocked. A transparent shim resolves the clients' same-origin relative URLs
 * against ACB_ORIGIN (default the canonical origin
 * https://ai-code-battle.pages.dev; override for a local `wrangler pages
 * dev`) and records every response's wire content-type, so every flow must
 * see `application/json` before an assertion believes the body — the SPA
 * HTML fallback is a 200 and must fail the flow, mirroring the
 * isJsonResponse / throwIfMatchTierOffline gates in api-transport.ts and
 * api-types.ts.
 *
 * Four flows, from the parent bead (aicodeba-84d1d61b):
 *
 *   a. registration — the form submits and renders the server's 503
 *      match_tier_offline answer. That 503 is the designed answer, so this
 *      stays green without acb-api deployed; a revived compute tier would
 *      change what the server says, and this suite reports it.
 *   b. predictions — open/history render their unavailable notices from the
 *      same 503s; predict surfaces it as MatchTierOfflineError. While open
 *      matches answer 503 the page renders no card to click, so the client
 *      boundary is the farthest the live wire can take the predict flow (the
 *      click-path rendering is pinned by src/pages/predictions.test.ts).
 *   c. map vote — POST /api/vote/map then GET /api/vote/map/{map_id} round
 *      trip through the real client functions (submitMapVote/fetchMapVotes)
 *      against the live store.
 *   d. replay feedback — POST /api/feedback, GET /api/feedback/{match_id},
 *      POST upvote through the real client functions
 *      (submitAnnotation/fetchFeedback/upvoteFeedback).
 *
 * Writes land under an `acb-e2e-probe-<timestamp>-<rand>` match/map id and a
 * matching voter id. Feedback and tallies are keyed per id and no aggregate
 * listing route exists, so nothing a page renders is touched — the same
 * namespace argument as the opt-in write probe in web/test-api-workflows.js.
 * Everything else here is read-only or refused-with-503, and the write volume
 * sits far under the function's per-isolate limits (2 of 30 map votes, 1 of
 * 20 feedback posts, 2 of 60 upvotes per window).
 *
 * Run (needs the network; deliberately not part of the default gate):
 *   cd web && npm run test:e2e-api
 *   ACB_ORIGIN=http://127.0.0.1:8788 npm run test:e2e-api   # local function
 */

import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { randomUUID } from 'node:crypto';
import { MatchTierOfflineError } from '../src/lib/api-transport';
import { cleanupPredictionsPage } from '../src/pages/predictions';

const ORIGIN = process.env.ACB_ORIGIN || 'https://ai-code-battle.pages.dev';

// Timestamped + random, like the write probe, so re-runs and concurrent
// workers never share a namespace. Both ids pass the server's isId()
// ([A-Za-z0-9_.:-]{1,128}) and the __proto__ guard.
const STAMP = new Date().toISOString().replace(/[^0-9]/g, '').slice(0, 14) + '-' + randomUUID().slice(0, 8);
const PROBE_MATCH_ID = `acb-e2e-probe-match-${STAMP}`;
const PROBE_MAP_ID = `acb-e2e-probe-map-${STAMP}`;
const PROBE_VOTER_ID = `acb-e2e-probe-voter-${STAMP}`;

// Text the live POST writes under the probe match id. It must clear the
// function's spam gate (>=10 normalized chars, no blocklisted term) because
// it is a real submission.
const FEEDBACK_BODY =
  'End-to-end live flow probe from web/e2e/api-flows.e2e.test.ts. ' +
  'This entry lives under a probe match id no replay page ever loads.';

// ─── The origin shim ─────────────────────────────────────────────────────────
// Resolves the clients' relative fetches against the origin — what a browser
// at that origin does — and records the wire facts each flow asserts on.

interface WireCall {
  path: string;
  method: string;
  status: number;
  contentType: string;
}

const realFetch = globalThis.fetch.bind(globalThis);
const wire: WireCall[] = [];

const originFetch: typeof fetch = async (input, init) => {
  const url = new URL(String(input), ORIGIN);
  const res = await realFetch(url.toString(), init);
  wire.push({
    path: url.pathname,
    method: (init?.method ?? 'GET').toUpperCase(),
    status: res.status,
    contentType: res.headers.get('content-type') ?? '',
  });
  return res;
};

beforeAll(async () => {
  globalThis.fetch = originFetch;

  // Precondition, not a flow: the community tier must be live or the two
  // round-trip flows below would fail for a boring reason. Failing here
  // names it.
  const res = await realFetch(new URL('/api/health', ORIGIN).toString());
  if (!(res.headers.get('content-type') ?? '').includes('application/json')) {
    throw new Error(`${ORIGIN}/api/health answered non-JSON — the Pages Function is missing from the deploy and the SPA fallback is answering`);
  }
  if (res.status !== 200) {
    throw new Error(`${ORIGIN}/api/health answered ${res.status} — the origin is unhealthy`);
  }
  const health = (await res.json()) as { capabilities?: Record<string, boolean> };
  for (const cap of ['feedback', 'map_votes'] as const) {
    if (health.capabilities?.[cap] !== true) {
      throw new Error(`${ORIGIN} reports capability ${cap}=false — the community store is unreadable; the live round trips would fail for storage reasons, not client reasons`);
    }
  }
});

afterAll(() => {
  globalThis.fetch = realFetch;
});

afterEach(() => {
  wire.length = 0;
});

// ─── Helpers ─────────────────────────────────────────────────────────────────

/** The last wire call to path+method; assertions read it, not the cache. */
function lastCall(path: string, method: string): WireCall {
  const call = [...wire].reverse().find(c => c.path === path && c.method === method);
  expect(call, `no ${method} call to ${path} ever left the client`).toBeDefined();
  return call!;
}

/**
 * The transport's proof of life: content-type application/json before any
 * assertion believes the body. A 200 text/html SPA fallback fails here.
 */
function expectWireJson(what: string, path: string, method: string): void {
  const call = lastCall(path, method);
  expect(
    call.contentType.includes('application/json'),
    `${what} answered ${call.status} "${call.contentType}" — /api must answer application/json; ` +
      'text/html is the SPA fallback and can never be believed',
  ).toBe(true);
}

/** Direct wire probe (bypasses the shim so probes are not mistaken for flow traffic). */
async function probeJson(route: string, init?: RequestInit): Promise<{ status: number; body: any }> {
  const res = await realFetch(new URL(route, ORIGIN).toString(), init);
  const contentType = res.headers.get('content-type') ?? '';
  expect(
    contentType.includes('application/json'),
    `probe ${route} answered "${contentType}" — not JSON, the origin is not in the expected state`,
  ).toBe(true);
  return { status: res.status, body: await res.json() };
}

async function waitFor(what: string, condition: () => boolean, timeoutMs = 10_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!condition()) {
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}`);
    await new Promise(resolve => setTimeout(resolve, 25));
  }
}

function install(): void {
  document.body.innerHTML = '<div id="app"></div>';
  // Seeded so the clients' getOrCreate*Id helpers never mint an unnamed
  // identity — every write this suite makes is probe-namespaced on the wire.
  localStorage.setItem('acb_voter_id', PROBE_VOTER_ID);
  localStorage.setItem('acb_visitor_id', PROBE_VOTER_ID);
  localStorage.setItem('acb_predictor_id', PROBE_VOTER_ID);
}

// ─── (a) Registration: form submit renders the server's offline answer ──────

describe('registration flow (match-tier 503 is the designed answer)', () => {
  it('submits the real form and renders the server 503 match_tier_offline state', async () => {
    // What the server actually says, straight from the wire — the rendered
    // notice must be this exact message, not a client-invented fallback.
    const probe = await probeJson('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: 'E2E_Probe_Bot',
        endpoint_url: 'https://example.com/move',
        owner_id: PROBE_VOTER_ID,
      }),
    });
    expect(probe.status).toBe(503);
    expect(probe.body.code).toBe('match_tier_offline');
    expect(typeof probe.body.error).toBe('string');
    expect(probe.body.error.length).toBeGreaterThan(0);

    install();
    const { renderRegisterPage } = await import('../src/pages/register');
    renderRegisterPage();

    // The form is fully enabled while the transport is live — the kill
    // switch is off, so nothing here is a disabled stub.
    expect(document.getElementById('register-unavailable-notice')).toBeNull();
    const form = document.getElementById('register-form') as HTMLFormElement;
    expect(form).not.toBeNull();

    (document.getElementById('bot-name') as HTMLInputElement).value = 'E2E_Probe_Bot';
    (document.getElementById('endpoint-url') as HTMLInputElement).value = 'https://example.com/move';
    (document.getElementById('owner-id') as HTMLInputElement).value = PROBE_VOTER_ID;
    form.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));

    await waitFor('the server error to render', () => document.querySelector('.error-message') !== null);

    expectWireJson('POST /api/register (form submit)', '/api/register', 'POST');
    const rendered = document.querySelector('.error-message')!.textContent;
    expect(rendered).toBe(probe.body.error);
    expect(document.querySelector('.register-success')).toBeNull();
  });
});

// ─── (b) Predictions: open/history render the 503 states; predict throws it ─

describe('predictions flow (same 503s, same designed answer)', () => {
  afterEach(() => {
    // The live page arms a real 15s poll; stop it or it outlives the test.
    cleanupPredictionsPage();
    document.body.innerHTML = '';
  });

  it('renders the open-matches and history unavailable states from the live 503s', async () => {
    const openProbe = await probeJson('/api/predictions/open');
    expect(openProbe.status).toBe(503);
    expect(openProbe.body.code).toBe('match_tier_offline');
    const historyProbe = await probeJson(`/api/predictions/history?predictor_id=${encodeURIComponent(PROBE_VOTER_ID)}`);
    expect(historyProbe.status).toBe(503);
    expect(historyProbe.body.code).toBe('match_tier_offline');

    install();
    const { renderPredictionsPage } = await import('../src/pages/predictions');
    await renderPredictionsPage();

    expectWireJson('GET /api/predictions/open (page load)', '/api/predictions/open', 'GET');
    expectWireJson('GET /api/predictions/history (page load)', '/api/predictions/history', 'GET');

    // The notices carry the server's own message — not the compile-time
    // notice, not a generic failure.
    expect(document.getElementById('predictions-transport-notice')).toBeNull();
    const openNotice = document.getElementById('open-matches-unavailable');
    expect(openNotice).not.toBeNull();
    expect(openNotice!.textContent).toContain(openProbe.body.error);
    const historyNotice = document.getElementById('history-unavailable');
    expect(historyNotice).not.toBeNull();
    expect(historyNotice!.textContent).toContain(historyProbe.body.error);
  });

  it('predict surfaces the server 503 as MatchTierOfflineError with the server message', async () => {
    const probe = await probeJson('/api/predict', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ match_id: PROBE_MATCH_ID, bot_id: 'probe-bot', predictor_id: PROBE_VOTER_ID }),
    });
    expect(probe.status).toBe(503);
    expect(probe.body.code).toBe('match_tier_offline');

    install();
    const { submitPrediction } = await import('../src/api-types');

    let err: unknown;
    try {
      await submitPrediction(PROBE_MATCH_ID, 'probe-bot', PROBE_VOTER_ID);
    } catch (e) {
      err = e;
    }
    expect(err).toBeInstanceOf(MatchTierOfflineError);
    expectWireJson('POST /api/predict (client submit)', '/api/predict', 'POST');
    expect((err as MatchTierOfflineError).message).toBe(probe.body.error);
    expect((err as MatchTierOfflineError).code).toBe('match_tier_offline');
  });
});

// ─── (c) Map vote: POST then GET round trip against the live store ───────────

describe('map-vote round trip (community tier, live)', () => {
  it('records a vote through the client and reads the tally back with my_vote', async () => {
    install();
    const { submitMapVote, fetchMapVotes } = await import('../src/api-types');

    // POST the vote (the replay page's up-arrow does exactly this call).
    const posted = await submitMapVote(PROBE_MAP_ID, 1);
    expectWireJson('POST /api/vote/map', '/api/vote/map', 'POST');
    expect(posted).toMatchObject({ map_id: PROBE_MAP_ID, vote: 1, net_votes: 1 });

    // GET reads the tally back — this voter's vote included.
    const read = await fetchMapVotes(PROBE_MAP_ID);
    expectWireJson('GET /api/vote/map/{map_id}', `/api/vote/map/${PROBE_MAP_ID}`, 'GET');
    expect(read).toMatchObject({ map_id: PROBE_MAP_ID, net_votes: 1, my_vote: 1 });

    // The same voter switching overwrites rather than adding a voter.
    const switched = await submitMapVote(PROBE_MAP_ID, -1);
    expectWireJson('POST /api/vote/map (switch)', '/api/vote/map', 'POST');
    expect(switched).toMatchObject({ vote: -1, net_votes: -1 });
    expect(await fetchMapVotes(PROBE_MAP_ID)).toMatchObject({ net_votes: -1, my_vote: -1 });
  });
});

// ─── (d) Replay feedback: POST, GET, upvote against the live store ───────────

describe('community feedback round trip (community tier, live)', () => {
  it('records, reads back, and upvotes through the annotation client', async () => {
    install();
    const { submitAnnotation, fetchFeedback, upvoteFeedback } = await import('../src/components/annotation');

    // POST the entry (the replay page's annotation form does this call).
    const submitted = await submitAnnotation({
      id: `e2e-probe-ann-${STAMP}`,
      match_id: PROBE_MATCH_ID,
      turn: 0,
      type: 'idea',
      body: FEEDBACK_BODY,
      author: 'acb e2e probe',
      upvotes: 0,
      created_at: new Date().toISOString(),
    });
    expectWireJson('POST /api/feedback', '/api/feedback', 'POST');
    expect(submitted).toBe(true);

    // GET returns the entry, mapped back into the client's Annotation shape.
    let entries = await fetchFeedback(PROBE_MATCH_ID);
    expectWireJson('GET /api/feedback/{match_id}', `/api/feedback/${PROBE_MATCH_ID}`, 'GET');
    expect(entries.length).toBe(1);
    expect(entries[0]).toMatchObject({
      match_id: PROBE_MATCH_ID,
      turn: 0,
      type: 'idea',
      body: FEEDBACK_BODY,
      author: 'acb e2e probe',
      upvotes: 0,
    });
    const feedbackId = entries[0]!.id;
    expect(feedbackId).toMatch(/^fb_/); // the server minted it, not the client

    // Upvote, read back exactly one.
    expect(await upvoteFeedback(feedbackId)).toBe(true);
    expectWireJson('POST /api/feedback/{id}/upvote', `/api/feedback/${feedbackId}/upvote`, 'POST');
    entries = await fetchFeedback(PROBE_MATCH_ID);
    expect(entries[0]!.upvotes).toBe(1);

    // The same visitor upvoting again must not double-count: the client
    // still answers true (the server's already_upvoted is a JSON ok), and
    // the tally stays 1.
    expect(await upvoteFeedback(feedbackId)).toBe(true);
    entries = await fetchFeedback(PROBE_MATCH_ID);
    expect(entries[0]!.upvotes).toBe(1);
  });
});
