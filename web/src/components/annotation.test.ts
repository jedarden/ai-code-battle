/**
 * Annotation API behaviour with no transport (bead aicodeba-07caa4ec).
 *
 * Community feedback degrades instead of throwing: submissions always land
 * in localStorage, upvotes honestly report failure, and reads skip the dead
 * `/api` attempt entirely — on the Pages host that route is the SPA HTML
 * fallback, so requesting it just delays the static-file fallback that
 * answers anyway.
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

function jsonResponse(url: string, body: unknown, ok = true): Response {
  return {
    url,
    ok,
    status: ok ? 200 : 404,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  } as unknown as Response;
}

function htmlFallback(url: string): Response {
  // What Cloudflare Pages actually returns for /api/* — 200 + text/html.
  return {
    url,
    ok: true,
    status: 200,
    headers: new Headers({ 'content-type': 'text/html' }),
    json: async () => {
      throw new Error('Unexpected JSON parse of HTML fallback');
    },
  } as unknown as Response;
}

beforeEach(() => {
  localStorage.clear();
  calls.length = 0;
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    calls.push(url);
    if (url.startsWith('/api/')) return htmlFallback(url);
    if (url === '/data/matches/match-1/feedback.json') {
      return jsonResponse(url, {
        feedback: [
          { feedback_id: 'f1', match_id: 'match-1', turn: 3, type: 'insight', body: 'b', author: 'a', upvotes: 2, created_at: '2026-09-25T00:00:00Z' },
        ],
      });
    }
    return jsonResponse(url, { feedback: [] }, false);
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

describe('submitAnnotation with no transport', () => {
  it('saves to localStorage and reports the annotation did NOT reach a server', async () => {
    const reachedServer = await submitAnnotation(sampleAnnotation());

    expect(reachedServer).toBe(false);
    expect(calls).toEqual([]); // no fetch — not even a doomed one
    const stored = loadLocalAnnotations('match-1');
    expect(stored).toHaveLength(1);
    expect(stored[0].id).toBe('ann_test_1');
  });
});

describe('upvoteFeedback with no transport', () => {
  it('reports failure without issuing the POST', async () => {
    await expect(upvoteFeedback('f1')).resolves.toBe(false);
    expect(calls).toEqual([]);
  });
});

describe('fetchFeedback with no transport', () => {
  it('goes straight to the static /data fallback — exactly one fetch, never /api', async () => {
    const annotations = await fetchFeedback('match-1');

    expect(calls).toEqual(['/data/matches/match-1/feedback.json']);
    expect(annotations).toHaveLength(1);
    expect(annotations[0].id).toBe('f1');
    expect(annotations[0].upvotes).toBe(2);
  });

  it('returns [] when the static index is missing too', async () => {
    const annotations = await fetchFeedback('match-unknown');
    expect(annotations).toEqual([]);
    expect(calls).toEqual(['/data/matches/match-unknown/feedback.json']);
  });
});
