/**
 * Annotation API behaviour with the Pages Function transport live
 * (bead aicodeba-84d1d61b).
 *
 * Community feedback is a LIVE capability of the /api function: reads come
 * from the server (falling back to the pre-built static index when the API
 * fails), submissions reach the server while localStorage stays the durable
 * local copy, and upvotes report honestly either way. localStorage remains
 * written even on success — it is the "your annotations" log and the
 * fallback copy if the community store is ever reset.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  type Annotation,
  fetchFeedback,
  upvoteFeedback,
  submitAnnotation,
  loadLocalAnnotations,
} from './annotation';

const calls: string[] = [];

function jsonResponse(body: unknown, ok = true): Response {
  return {
    ok,
    status: ok ? 200 : 404,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  } as unknown as Response;
}

const API_ENTRY = {
  feedback_id: 'srv_1',
  match_id: 'match-1',
  turn: 3,
  type: 'insight',
  body: 'server-side insight',
  author: 'someone-else',
  upvotes: 2,
  created_at: '2026-09-25T00:00:00Z',
};

/** /api answers live JSON; the static index holds a different entry. */
function liveApiFetch(): Response {
  return jsonResponse({ match_id: 'match-1', feedback: [API_ENTRY] });
}

beforeEach(() => {
  localStorage.clear();
  calls.length = 0;
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    calls.push(url);
    void init;
    if (url === `/api/feedback/match-1`) return liveApiFetch();
    if (url.endsWith('/upvote')) return jsonResponse({ status: 'recorded' });
    if (url === `/api/feedback`) return jsonResponse({ status: 'recorded', feedback_id: 'srv_new' }, true);
    if (url === '/data/matches/match-1/feedback.json') {
      return jsonResponse({
        feedback: [
          { feedback_id: 'static_1', match_id: 'match-1', turn: 1, type: 'idea', body: 'b', author: 'a', upvotes: 0, created_at: '2026-09-25T00:00:00Z' },
        ],
      });
    }
    return jsonResponse({}, false);
  });
});

function sampleAnnotation(): Annotation {
  return {
    id: 'ann_test_1',
    match_id: 'match-1',
    turn: 2,
    type: 'highlight',
    body: 'nice flank',
    author: 'Anonymous',
    upvotes: 0,
    created_at: '2026-09-25T00:00:00Z',
  };
}

describe('fetchFeedback with the transport live', () => {
  it('reads from the API and maps entries to annotations', async () => {
    const annotations = await fetchFeedback('match-1');

    expect(calls[0]).toBe('/api/feedback/match-1');
    expect(annotations).toHaveLength(1);
    expect(annotations[0]).toMatchObject({
      id: 'srv_1',
      match_id: 'match-1',
      turn: 3,
      type: 'insight',
      body: 'server-side insight',
      upvotes: 2,
    });
  });

  it('falls back to the static index when the API request fails', async () => {
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      calls.push(url);
      if (url.startsWith('/api/')) throw new Error('network down');
      if (url === '/data/matches/match-1/feedback.json') {
        return jsonResponse({
          feedback: [
            { feedback_id: 'static_1', match_id: 'match-1', turn: 1, type: 'idea', body: 'b', author: 'a', upvotes: 0, created_at: '2026-09-25T00:00:00Z' },
          ],
        });
      }
      return jsonResponse({}, false);
    });

    const annotations = await fetchFeedback('match-1');

    expect(calls).toEqual(['/api/feedback/match-1', '/data/matches/match-1/feedback.json']);
    expect(annotations.map(a => a.id)).toEqual(['static_1']);
  });

  it('falls back when the API answers HTML (SPA fallback regression)', async () => {
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      calls.push(url);
      if (url.startsWith('/api/')) {
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'content-type': 'text/html' }),
          json: async () => {
            throw new Error('JSON-parsed HTML');
          },
        } as unknown as Response;
      }
      return jsonResponse({ feedback: [] });
    });

    // The static index returns no entries here, but the important part is
    // that the HTML answer was not trusted and no JSON-parse error escapes.
    await expect(fetchFeedback('match-1')).resolves.toEqual([]);
  });
});

describe('upvoteFeedback with the transport live', () => {
  it('posts the visitor id and reports success on a JSON answer', async () => {
    await expect(upvoteFeedback('srv_1')).resolves.toBe(true);

    expect(calls[0]).toBe('/api/feedback/srv_1/upvote');
    const body = JSON.parse((globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1].body);
    expect(body.voter_id).toBe(localStorage.getItem('acb_visitor_id'));
  });

  it('reports failure when the server answer is not JSON', async () => {
    globalThis.fetch = vi.fn(async () => ({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'text/html' }),
    }) as unknown as Response);

    await expect(upvoteFeedback('srv_1')).resolves.toBe(false);
  });

  it('reports failure on a network error', async () => {
    globalThis.fetch = vi.fn(async () => {
      throw new Error('down');
    });

    await expect(upvoteFeedback('srv_1')).resolves.toBe(false);
  });
});

describe('submitAnnotation with the transport live', () => {
  it('saves locally AND reaches the server, reporting both', async () => {
    const reachedServer = await submitAnnotation(sampleAnnotation());

    expect(reachedServer).toBe(true);
    expect(calls[0]).toBe('/api/feedback');
    const posted = JSON.parse((globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1].body);
    expect(posted).toMatchObject({ match_id: 'match-1', type: 'highlight', body: 'nice flank' });

    const stored = loadLocalAnnotations('match-1');
    expect(stored).toHaveLength(1);
    expect(stored[0].id).toBe('ann_test_1');
  });

  it('still keeps the local copy when the POST fails, and says it did not reach a server', async () => {
    globalThis.fetch = vi.fn(async () => {
      throw new Error('down');
    });

    const reachedServer = await submitAnnotation(sampleAnnotation());

    expect(reachedServer).toBe(false);
    expect(loadLocalAnnotations('match-1')).toHaveLength(1);
  });
});
