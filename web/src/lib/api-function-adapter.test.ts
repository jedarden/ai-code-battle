/**
 * Contract tests for the deployed Pages Function adapter itself
 * (bead aicodeba-91377341).
 *
 * api-backend.test.ts drives handleApiRequest — the whole /api surface
 * except one link in the chain: the exported `onRequest` in
 * functions/api/[[path]].ts. Its entire job is the wiring the Pages runtime
 * hands it — URL parse, a one-time `/api` prefix strip, env hand-off — so
 * these tests drive that exported handler end-to-end the way the runtime
 * would (a full https Request in, its Response out) and pin:
 *
 *   - every documented route answers under /api/* (health, feedback
 *     create/read/upvote, map vote write/read, match-tier 503s)
 *   - the prefix strip happens exactly once: /api/api/health is not health
 *   - the query string survives — the backend reads voter_id off the very
 *     request object the adapter forwards
 *   - the env handed to onRequest is the one that answers: a write served
 *     through /api lands in the bucket we supplied
 *   - every answer class keeps application/json — the founding failure mode
 *     of this transport was JSON routes answered as text/html
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { onRequest } from '../../functions/api/[[path]]';
import type { CommunityBucket, ApiEnv } from './api-backend';

interface StoredDoc {
  etag: string;
  value: string;
}

let etagCounter = 0;

/** In-memory stand-in with R2's documented CAS semantics. */
class MemBucket implements CommunityBucket {
  private docs = new Map<string, StoredDoc>();

  async get(key: string): Promise<{ etag: string; text(): Promise<string> } | null> {
    const doc = this.docs.get(key);
    if (!doc) return null;
    return { etag: doc.etag, text: async () => doc.value };
  }

  async head(key: string): Promise<{ etag: string } | null> {
    const doc = this.docs.get(key);
    return doc ? { etag: doc.etag } : null;
  }

  async put(
    key: string,
    value: string,
    opts?: { onlyIf?: { etagMatches?: string } },
  ): Promise<unknown | null> {
    const current = this.docs.get(key);
    const expected = opts?.onlyIf?.etagMatches;
    if (expected !== undefined && (!current || current.etag !== expected)) return null;
    this.docs.set(key, { etag: `etag-${++etagCounter}`, value });
    return {};
  }
}

let bucket: MemBucket;
let env: ApiEnv;

/** The slice of the Pages event context the adapter actually touches. */
function pagesContext(request: Request): Parameters<typeof onRequest>[0] {
  return {
    request,
    env,
    params: {},
    data: {},
    functionPath: '/api',
    next: () => {
      throw new Error('next() must not be called by the /api adapter');
    },
    waitUntil: () => {},
    passThroughOnException: () => {},
  };
}

async function call(path: string, init?: { method?: string; body?: unknown; rawBody?: string; ip?: string }) {
  const headers = new Headers();
  if (init?.ip) headers.set('CF-Connecting-IP', init.ip);
  if (init?.body !== undefined || init?.rawBody !== undefined) {
    headers.set('Content-Type', 'application/json');
  }
  const body = init?.rawBody ?? (init?.body !== undefined ? JSON.stringify(init.body) : undefined);
  const request = new Request(`https://ai-code-battle.pages.dev${path}`, {
    method: init?.method ?? 'GET',
    headers,
    body,
  });
  const response = await onRequest(pagesContext(request));
  const text = await response.text();
  let json: unknown = null;
  try {
    json = JSON.parse(text);
  } catch {
    // non-JSON body: tests assert on the raw text in that case
  }
  return {
    status: response.status,
    contentType: response.headers.get('content-type') ?? '',
    json: json as Record<string, unknown>,
    text,
  };
}

beforeEach(() => {
  bucket = new MemBucket();
  env = { ACB_BUCKET: bucket };
});

function feedbackBody(overrides: Record<string, unknown> = {}) {
  return {
    match_id: 'match-1',
    turn: 3,
    type: 'insight',
    body: 'the losing bot never scouted the expansion',
    author: 'reviewer',
    ...overrides,
  };
}

// ─── The adapter's own wiring ───────────────────────────────────────────────

describe('the /api prefix is stripped exactly once', () => {
  it('a documented route answers under /api', async () => {
    const res = await call('/api/health');
    expect(res.status).toBe(200);
    expect(res.json).toEqual({
      status: 'ok',
      capabilities: {
        register: false,
        rotate_key: false,
        predictions: false,
        feedback: true,
        map_votes: true,
      },
    });
  });

  it('/api/api/health is NOT health — the strip does not loop', async () => {
    const res = await call('/api/api/health');
    expect(res.status).toBe(404);
    expect(res.json).toEqual({ error: 'not found' });
  });

  it('the bare /api root answers a JSON 404, not the SPA fallback', async () => {
    const res = await call('/api');
    expect(res.status).toBe(404);
    expect(res.contentType).toContain('application/json');
  });

  it('the strip is case-sensitive: /API/health is not health', async () => {
    const res = await call('/API/health');
    expect(res.status).toBe(404);
  });
});

// ─── Documented routes, end to end through onRequest ────────────────────────

describe('replay feedback answers through /api', () => {
  it('POST records, GET serves the entry, and the write landed in the handed env', async () => {
    const post = await call('/api/feedback', { method: 'POST', body: feedbackBody(), ip: 'ad-fb-create' });
    expect(post.status).toBe(201);
    expect(post.json.status).toBe('recorded');
    const id = String(post.json.feedback_id);
    expect(id.startsWith('fb_')).toBe(true);

    const get = await call('/api/feedback/match-1');
    expect(get.status).toBe(200);
    const feedback = get.json.feedback as Record<string, unknown>[];
    expect(feedback).toHaveLength(1);
    expect(feedback[0]).toMatchObject({
      feedback_id: id,
      match_id: 'match-1',
      turn: 3,
      type: 'insight',
      author: 'reviewer',
      upvotes: 0,
    });
    expect(feedback[0]).not.toHaveProperty('voters');

    // Env hand-off: the write the adapter served is in THIS bucket — proof
    // the adapter forwards the very env it was given, not a fresh one.
    const stored = await bucket.get('community/replay-feedback.json');
    expect(stored).not.toBeNull();
    expect(JSON.parse(await stored!.text())).toMatchObject({
      feedback: [{ feedback_id: id }],
    });
  });

  it('POST /api/feedback/{id}/upvote records once per voter and 404s unknown ids', async () => {
    const created = await call('/api/feedback', { method: 'POST', body: feedbackBody(), ip: 'ad-fb-up' });
    const id = String(created.json.feedback_id);

    const first = await call(`/api/feedback/${id}/upvote`, {
      method: 'POST',
      body: { voter_id: 'voter-a' },
      ip: 'ad-up1',
    });
    expect(first.status).toBe(200);
    expect(first.json.status).toBe('recorded');

    const again = await call(`/api/feedback/${id}/upvote`, {
      method: 'POST',
      body: { voter_id: 'voter-a' },
      ip: 'ad-up2',
    });
    expect(again.status).toBe(200);
    expect(again.json.status).toBe('already_upvoted');

    const missing = await call('/api/feedback/fb_nobody/upvote', {
      method: 'POST',
      body: { voter_id: 'voter-a' },
      ip: 'ad-up3',
    });
    expect(missing.status).toBe(404);
  });

  it('a malformed body is refused with a JSON 400', async () => {
    const res = await call('/api/feedback', { method: 'POST', rawBody: 'not json at all', ip: 'ad-fb-raw' });
    expect(res.status).toBe(400);
    expect(res.json).toEqual({ error: 'invalid request body' });
    expect(res.contentType).toContain('application/json');
  });
});

describe('map voting answers through /api', () => {
  it('POST records a vote; GET tallies it and echoes my_vote from the query string', async () => {
    const post = await call('/api/vote/map', {
      method: 'POST',
      body: { map_id: 'map-1', voter_id: 'voter-a', vote: 1 },
      ip: 'ad-vote',
    });
    expect(post.status).toBe(200);
    expect(post.json).toEqual({ map_id: 'map-1', vote: 1, net_votes: 1 });

    // The query string must survive the adapter: the backend reads voter_id
    // off the very request object it is handed.
    const get = await call('/api/vote/map/map-1?voter_id=voter-a');
    expect(get.status).toBe(200);
    expect(get.json).toEqual({ map_id: 'map-1', net_votes: 1, my_vote: 1 });

    const anonymous = await call('/api/vote/map/map-1');
    expect(anonymous.json).toEqual({ map_id: 'map-1', net_votes: 1 });
  });

  it('a malformed body is refused with a JSON 400', async () => {
    const res = await call('/api/vote/map', { method: 'POST', rawBody: '{not json', ip: 'ad-vote-raw' });
    expect(res.status).toBe(400);
    expect(res.json).toEqual({ error: 'invalid request body' });
    expect(res.contentType).toContain('application/json');
  });
});

describe('the match tier stays honestly offline under /api', () => {
  it.each([
    ['POST', '/api/register'],
    ['POST', '/api/rotate-key'],
    ['POST', '/api/predict'],
    ['GET', '/api/predictions/open'],
    ['GET', '/api/predictions/history'],
  ])('%s %s answers 503 match_tier_offline', async (method, path) => {
    const res = await call(path, { method, ip: `ad-mt-${path}` });
    expect(res.status).toBe(503);
    expect(res.json.code).toBe('match_tier_offline');
    expect(String(res.json.error)).toContain('offline');
    expect(res.contentType).toContain('application/json');
  });
});

// ─── The founding contract: every answer is JSON ────────────────────────────

describe('every /api answer keeps application/json — never text/html', () => {
  type CallResult = Awaited<ReturnType<typeof call>>;
  type Probe = () => Promise<CallResult>;

  const cases: [string, Probe][] = [
    ['GET /api/health (200)', () => call('/api/health', { ip: 'ad-ct-health' })],
    ['POST /api/vote/map (200)', () =>
      call('/api/vote/map', { method: 'POST', body: { map_id: 'ad-ct-map', voter_id: 'v', vote: -1 }, ip: 'ad-ct-vote' })],
    ['GET /api/vote/map/{id} (200)', () => call('/api/vote/map/ad-ct-map', { ip: 'ad-ct-votes' })],
    ['POST /api/feedback (201)', () =>
      call('/api/feedback', { method: 'POST', body: feedbackBody({ match_id: 'ad-ct-match' }), ip: 'ad-ct-fb' })],
    ['POST /api/feedback (spam → 422)', () =>
      call('/api/feedback', { method: 'POST', body: feedbackBody({ body: 'this is total shit, delete it now' }), ip: 'ad-ct-spam' })],
    ['POST /api/feedback (malformed body → 400)', () =>
      call('/api/feedback', { method: 'POST', rawBody: 'not json at all', ip: 'ad-ct-raw' })],
    ['POST /api/feedback (invalid shape → 400)', () =>
      call('/api/feedback', { method: 'POST', body: { type: 'insight' }, ip: 'ad-ct-bad' })],
    ['POST /api/feedback/{id}/upvote (unknown id → 404)', () =>
      call('/api/feedback/fb_nobody/upvote', { method: 'POST', body: { voter_id: 'ad-ct-up' }, ip: 'ad-ct-up' })],
    ['POST /api/register (match-tier → 503)', () => call('/api/register', { method: 'POST', ip: 'ad-ct-mt' })],
    ['GET /api/nope (unknown route → 404)', () => call('/api/nope', { ip: 'ad-ct-404' })],
    ['GET /api (bare root → 404)', () => call('/api', { ip: 'ad-ct-root' })],
  ];

  for (const [name, probe] of cases) {
    it(`${name} answers application/json`, async () => {
      const res = await probe();
      expect(res.contentType).toContain('application/json');
      expect(() => JSON.parse(res.text)).not.toThrow();
    });
  }
});
