/**
 * Contract tests for the Pages Function backend (bead aicodeba-84d1d61b).
 *
 * handleApiRequest is the whole /api/* server surface that ships in
 * web/functions/api/ — these tests drive it end-to-end (real Request in,
 * real Response out) against an in-memory bucket implementing the R2
 * conditional-put semantics the production binding has: a put carrying
 * `onlyIf.etagMatches` stores only when the etag still matches and resolves
 * null otherwise.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { handleApiRequest, type CommunityBucket, type ApiEnv } from './api-backend';

interface StoredDoc {
  etag: string;
  value: string;
}

let etagCounter = 0;

/** In-memory stand-in with R2's documented CAS semantics. */
class MemBucket implements CommunityBucket {
  private docs = new Map<string, StoredDoc>();
  /** When > 0, the next N conditional puts fail as if a concurrent writer won. */
  forcedConflicts = 0;
  lastPut: { key: string; conditional: boolean } | null = null;

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
    this.lastPut = { key, conditional: expected !== undefined };
    if (expected !== undefined) {
      if (!current || current.etag !== expected) return null;
      if (this.forcedConflicts > 0) {
        this.forcedConflicts--;
        // Simulate a concurrent writer winning the slot.
        current.etag = `concurrent-${++etagCounter}`;
        return null;
      }
    }
    const etag = `etag-${++etagCounter}`;
    this.docs.set(key, { etag, value });
    return { etag };
  }

  /** Raw peek for assertions on what was actually persisted. */
  peek(key: string): unknown {
    const doc = this.docs.get(key);
    return doc ? JSON.parse(doc.value) : undefined;
  }
}

class BrokenBucket extends MemBucket {
  async head(): Promise<{ etag: string } | null> {
    throw new Error('bucket unreachable');
  }
}

let bucket: MemBucket;
let env: ApiEnv;

function request(path: string, init?: { method?: string; body?: unknown; ip?: string }): Request {
  const headers = new Headers();
  if (init?.ip) headers.set('CF-Connecting-IP', init.ip);
  const body = init?.body !== undefined ? JSON.stringify(init.body) : undefined;
  return new Request(`https://ai-code-battle.pages.dev/api${path}`, {
    method: init?.method ?? 'GET',
    headers,
    body,
  });
}

async function call(path: string, init?: { method?: string; body?: unknown; ip?: string }) {
  // The deployed wrapper hands handleApiRequest the bare pathname (query
  // stripped by its own URL parse) — mirror that here.
  const route = path.replace(/\?.*$/, '');
  const response = await handleApiRequest(request(path, init), env, route);
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

// ─── Health ─────────────────────────────────────────────────────────────────

describe('GET /health', () => {
  it('reports ok with the live capability set', async () => {
    const res = await call('/health');

    expect(res.status).toBe(200);
    expect(res.contentType).toContain('application/json');
    expect(res.json.status).toBe('ok');
    expect(res.json.capabilities).toEqual({
      register: false,
      rotate_key: false,
      predictions: false,
      feedback: true,
      map_votes: true,
    });
  });

  it('flips the community capabilities off when storage is unreachable', async () => {
    env = { ACB_BUCKET: new BrokenBucket() };
    const res = await call('/health');

    expect(res.status).toBe(200);
    expect(res.json.capabilities).toEqual({
      register: false,
      rotate_key: false,
      predictions: false,
      feedback: false,
      map_votes: false,
    });
  });
});

// ─── Map voting ─────────────────────────────────────────────────────────────

describe('map voting', () => {
  it('round-trips a vote: POST is recorded, GET tallies it and reports my_vote', async () => {
    const post = await call('/vote/map', {
      method: 'POST',
      body: { map_id: 'map-1', voter_id: 'voter-a', vote: 1 },
      ip: 'vote-rt',
    });
    expect(post.status).toBe(200);
    expect(post.json).toEqual({ map_id: 'map-1', vote: 1, net_votes: 1 });

    const get = await call('/vote/map/map-1?voter_id=voter-a');
    expect(get.status).toBe(200);
    expect(get.json.net_votes).toBe(1);
    expect(get.json.my_vote).toBe(1);
  });

  it('omits my_vote when the GET carries no voter_id', async () => {
    await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'voter-a', vote: -1 }, ip: 'vote-omit' });
    const get = await call('/vote/map/map-1');

    expect(get.json.net_votes).toBe(-1);
    expect(get.json).not.toHaveProperty('my_vote');
  });

  it('a switched vote replaces the old one instead of stacking', async () => {
    const ip = 'vote-switch';
    await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'v', vote: 1 }, ip });
    const second = await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'v', vote: -1 }, ip });

    expect(second.json.net_votes).toBe(-1);
  });

  it('rejects malformed votes with 400', async () => {
    const badMap = await call('/vote/map', { method: 'POST', body: { map_id: 'bad id!', voter_id: 'v', vote: 1 }, ip: 'vote-bad1' });
    const badVote = await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'v', vote: 2 }, ip: 'vote-bad2' });
    const noVoter = await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', vote: 1 }, ip: 'vote-bad3' });
    const badBody = await call('/vote/map', { method: 'POST', body: undefined, ip: 'vote-bad4' });

    for (const res of [badMap, badVote, noVoter, badBody]) {
      expect(res.status).toBe(400);
    }
  });

  it('retries the document under CAS contention before answering', async () => {
    bucket.forcedConflicts = 3;
    const res = await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'v', vote: 1 }, ip: 'vote-cas' });

    expect(res.status).toBe(200);
    expect(res.json.net_votes).toBe(1);
    expect(bucket.peek('community/map-votes.json')).toMatchObject({
      votes: { 'map-1': { v: 1 } },
    });
  });

  it('answers 503 storage_busy when concurrent writers exhaust the retries', async () => {
    // The document must already exist so the write takes the conditional-put
    // path (a first-ever write is unconditional by design).
    await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'seed', vote: 1 }, ip: 'vote-busy-seed' });
    bucket.forcedConflicts = 999;
    const res = await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'v', vote: 1 }, ip: 'vote-busy' });

    expect(res.status).toBe(503);
    expect(res.json.code).toBe('storage_busy');
  });

  it('reseeds a corrupt stored document instead of wedging', async () => {
    bucket.put('community/map-votes.json', 'NOT JSON{');
    const res = await call('/vote/map', { method: 'POST', body: { map_id: 'map-1', voter_id: 'v', vote: 1 }, ip: 'vote-corrupt' });

    expect(res.status).toBe(200);
    expect(res.json.net_votes).toBe(1);
  });

  it('rate-limits votes per client IP with 429', async () => {
    const ip = 'vote-ratelimit';
    let last = { status: 0, json: {} as Record<string, unknown> };
    for (let i = 0; i < 31; i++) {
      last = await call('/vote/map', {
        method: 'POST',
        body: { map_id: `map-${i}`, voter_id: 'v', vote: 1 },
        ip,
      });
    }
    expect(last.status).toBe(429);
    expect(String(last.json.error)).toContain('too many votes');
  });
});

// ─── Replay feedback ────────────────────────────────────────────────────────

function feedbackBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    match_id: 'match-1',
    turn: 3,
    type: 'insight',
    body: 'the flank collapse starts around turn 30',
    author: 'reviewer',
    ...overrides,
  };
}

describe('replay feedback', () => {
  it('creates, then serves feedback without leaking the internal voter set', async () => {
    const post = await call('/feedback', { method: 'POST', body: feedbackBody(), ip: 'fb-create' });

    expect(post.status).toBe(201);
    expect(post.json.status).toBe('recorded');
    const id = String(post.json.feedback_id);
    expect(id.startsWith('fb_')).toBe(true);

    const get = await call('/feedback/match-1');
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
  });

  it('defaults a missing author to Anonymous', async () => {
    await call('/feedback', { method: 'POST', body: feedbackBody({ author: undefined }), ip: 'fb-anon' });
    const get = await call('/feedback/match-1');
    expect((get.json.feedback as Record<string, unknown>[])[0].author).toBe('Anonymous');
  });

  it('upvotes once per voter, idempotently, and 404s unknown ids', async () => {
    const created = await call('/feedback', { method: 'POST', body: feedbackBody(), ip: 'fb-up' });
    const id = String(created.json.feedback_id);

    const first = await call(`/feedback/${id}/upvote`, { method: 'POST', body: { voter_id: 'voter-a' }, ip: 'fb-up1' });
    expect(first.json.status).toBe('recorded');

    const again = await call(`/feedback/${id}/upvote`, { method: 'POST', body: { voter_id: 'voter-a' }, ip: 'fb-up2' });
    expect(again.status).toBe(200);
    expect(again.json.status).toBe('already_upvoted');

    const other = await call(`/feedback/${id}/upvote`, { method: 'POST', body: { voter_id: 'voter-b' }, ip: 'fb-up3' });
    expect(other.json.status).toBe('recorded');

    const get = await call('/feedback/match-1');
    expect((get.json.feedback as Record<string, unknown>[])[0].upvotes).toBe(2);

    const missing = await call('/feedback/fb_doesnotexist/upvote', { method: 'POST', body: { voter_id: 'voter-a' }, ip: 'fb-up4' });
    expect(missing.status).toBe(404);
  });

  it('rejects invalid shapes with 400', async () => {
    const cases: Record<string, unknown>[] = [
      feedbackBody({ match_id: 'bad id!' }),
      feedbackBody({ type: 'unknown-type' }),
      feedbackBody({ turn: 'three' }),
      feedbackBody({ body: '' }),
      feedbackBody({ author: 'x'.repeat(65) }),
      { match_id: 'match-1' },
    ];
    for (const [i, body] of cases.entries()) {
      const res = await call('/feedback', { method: 'POST', body, ip: `fb-invalid-${i}` });
      expect(res.status).toBe(400);
    }
  });

  it('applies the ported spam filter with 422', async () => {
    const blocked = await call('/feedback', {
      method: 'POST',
      body: feedbackBody({ body: 'this is total shit, delete it now' }),
      ip: 'fb-spam1',
    });
    expect(blocked.status).toBe(422);
    expect(String(blocked.json.error)).toContain('blocked terms');

    const short = await call('/feedback', {
      method: 'POST',
      body: feedbackBody({ body: 'too short' }),
      ip: 'fb-spam2',
    });
    expect(short.status).toBe(422);
    expect(String(short.json.error)).toContain('too short');
  });

  it('answers 413 when the body exceeds the cap', async () => {
    const res = await call('/feedback', {
      method: 'POST',
      body: feedbackBody({ body: 'x'.repeat(33 * 1024) }),
      ip: 'fb-big',
    });
    expect(res.status).toBe(413);
  });

  it('rate-limits feedback per client IP with 429', async () => {
    const ip = 'fb-ratelimit';
    let last = { status: 0, json: {} as Record<string, unknown> };
    for (let i = 0; i < 21; i++) {
      last = await call('/feedback', {
        method: 'POST',
        body: feedbackBody({ match_id: `match-${i}` }),
        ip,
      });
    }
    expect(last.status).toBe(429);
    expect(String(last.json.error)).toContain('too much feedback');
  });
});

// ─── Site feedback (agentation overlay) ─────────────────────────────────────

describe('site feedback', () => {
  it('stores markdown submissions on a separate document', async () => {
    const post = await call('/feedback', {
      method: 'POST',
      body: { markdown: 'the event timeline skips turns under load', annotations: [{ turn: 4 }], submitted_at: '2026-09-25T00:00:00Z' },
      ip: 'site-1',
    });

    expect(post.status).toBe(201);
    expect(String(post.json.feedback_id).startsWith('site_')).toBe(true);

    const stored = bucket.peek('community/site-feedback.json') as {
      feedback: { feedback_id: string; markdown: string; annotations: unknown[] }[];
    };
    expect(stored.feedback).toHaveLength(1);
    expect(stored.feedback[0].markdown).toContain('event timeline');
    expect(stored.feedback[0].annotations).toEqual([{ turn: 4 }]);
  });

  it('requires markdown and applies the spam filter', async () => {
    const noMarkdown = await call('/feedback', { method: 'POST', body: { annotations: [] }, ip: 'site-2' });
    expect(noMarkdown.status).toBe(400);

    const spam = await call('/feedback', {
      method: 'POST',
      body: { markdown: 'click here to win free money now' },
      ip: 'site-3',
    });
    expect(spam.status).toBe(422);
  });
});

// ─── Match-tier routes ──────────────────────────────────────────────────────

describe('match-tier routes answer an honest 503', () => {
  const cases: [string, string][] = [
    ['/register', 'POST'],
    ['/rotate-key', 'POST'],
    ['/predict', 'POST'],
    ['/predictions/open', 'GET'],
    ['/predictions/history', 'GET'],
  ];

  for (const [path, method] of cases) {
    it(`${method} ${path} → 503 match_tier_offline`, async () => {
      const res = await call(path, { method, body: method === 'POST' ? {} : undefined, ip: `mt-${path}` });

      expect(res.status).toBe(503);
      expect(res.contentType).toContain('application/json');
      expect(res.json.code).toBe('match_tier_offline');
      expect(String(res.json.error)).toContain('offline');
    });
  }
});

// ─── Router surface ─────────────────────────────────────────────────────────

describe('router', () => {
  it('returns JSON 404 for unknown routes and wrong methods', async () => {
    const unknown = await call('/nope', { ip: 'rt-1' });
    const wrongMethod = await call('/register', { ip: 'rt-2' });

    expect(unknown.status).toBe(404);
    expect(wrongMethod.status).toBe(404);
    expect(unknown.contentType).toContain('application/json');
  });

  it('ignores a trailing slash', async () => {
    const res = await call('/health/', { ip: 'rt-3' });
    expect(res.status).toBe(200);
  });

  it('every answer is JSON — never the SPA fallback content type', async () => {
    const paths: [string, string][] = [
      ['/health', 'GET'],
      ['/vote/map/map-1', 'GET'],
      ['/feedback/match-1', 'GET'],
      ['/nope', 'GET'],
    ];
    for (const [path, method] of paths) {
      const res = await call(path, { method, ip: 'rt-json' });
      expect(res.contentType).toContain('application/json');
    }
  });
});
