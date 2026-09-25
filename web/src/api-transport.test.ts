/**
 * SPA→API transport contract (bead aicodeba-84d1d61b).
 *
 * The transport is the same-origin Pages Function under web/functions/api/
 * (see docs/notes/api-transport.md). These tests pin the client side of that
 * contract: the flag is on because a real transport exists; the match-tier
 * 503s surface as MatchTierOfflineError; and the HTML-fallback backstop
 * holds — a response that is not JSON can never be mistaken for a server
 * answer, so a regression to the SPA fallback degrades instead of lying.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  API_TRANSPORT_ENABLED,
  ApiTransportUnavailableError,
  MatchTierOfflineError,
  matchTierOfflineMessage,
} from './lib/api-transport';
import {
  registerBot,
  rotateApiKey,
  fetchPredictionHistory,
  fetchOpenPredictions,
  submitPrediction,
  submitMapVote,
  fetchMapVotes,
} from './api-types';

const MATCH_TIER_503 = (action: string) => ({
  ok: false,
  status: 503,
  headers: new Headers({ 'content-type': 'application/json' }),
  json: async () => ({
    error: `${action} is offline: the compute tier that runs matches is not deployed.`,
    code: 'match_tier_offline',
  }),
});

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  } as unknown as Response;
}

function htmlFallback(): Response {
  // What the Pages SPA fallback returns for /api/* when no function is
  // deployed: HTTP 200 with the app's own index.html.
  return {
    ok: true,
    status: 200,
    headers: new Headers({ 'content-type': 'text/html' }),
    json: async () => {
      throw new Error('a client tried to JSON-parse the SPA HTML fallback');
    },
  } as unknown as Response;
}

let calls: { url: string; init?: RequestInit }[] = [];

beforeEach(() => {
  localStorage.clear();
  calls = [];
});

/** Install a fetch mock from a (url → response) recipe; URLs are matched with startsWith. */
function mockFetch(routes: [string, (url: string, init?: RequestInit) => Response][], fallback: (url: string) => Response) {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    calls.push({ url, init });
    for (const [prefix, respond] of routes) {
      if (url.startsWith(prefix)) return respond(url, init);
    }
    return fallback(url);
  });
}

describe('transport flag', () => {
  it('is enabled — the Pages Function transport exists (see api-transport.md)', () => {
    // Deliberate pin: this is only true because web/functions/api/ ships the
    // same-origin /api/* routes. Reverting it is the kill switch for every
    // /api code path at once.
    expect(API_TRANSPORT_ENABLED).toBe(true);
  });

  it('the kill-switch error type is still the pre-transport contract', () => {
    const err = new ApiTransportUnavailableError('Bot registration');
    expect(err).toBeInstanceOf(Error);
    expect(err.message).toContain('Bot registration');
    expect(err.message).toContain('no public endpoint');
  });
});

describe('matchTierOfflineMessage', () => {
  const body = { error: 'Bot registration is offline…', code: 'match_tier_offline' };

  it('detects the function 503 envelope and returns its message', () => {
    expect(matchTierOfflineMessage(503, body)).toBe('Bot registration is offline…');
  });

  it('refuses anything that is not exactly that envelope', () => {
    expect(matchTierOfflineMessage(500, body)).toBeNull();
    expect(matchTierOfflineMessage(503, { error: 'x', code: 'storage_busy' })).toBeNull();
    expect(matchTierOfflineMessage(503, { code: 'match_tier_offline' })).toBeNull();
    expect(matchTierOfflineMessage(503, { error: 42, code: 'match_tier_offline' })).toBeNull();
    expect(matchTierOfflineMessage(503, null)).toBeNull();
  });

  it('MatchTierOfflineError carries the code', () => {
    expect(new MatchTierOfflineError('x').code).toBe('match_tier_offline');
  });
});

describe('client functions against the live transport', () => {
  it('registerBot posts and parses a JSON answer', async () => {
    mockFetch(
      [['/api/register', () => jsonResponse({ success: true, bot_id: 'bot-1', api_key: 'key' })]],
      htmlFallback,
    );

    const result = await registerBot({ name: 'x', endpoint_url: 'https://x', owner_id: 'o' });

    expect(result.success).toBe(true);
    expect(calls[0]?.url).toContain('/api/register');
  });

  it('rotateApiKey posts to /api/rotate-key', async () => {
    mockFetch(
      [['/api/rotate-key', () => jsonResponse({ success: true, api_key: 'new-key' })]],
      htmlFallback,
    );

    const result = await rotateApiKey('bot-1', 'current-key');

    expect(result.success).toBe(true);
    expect(calls[0]?.url).toContain('/api/rotate-key');
  });

  it('map voting round-trips and mints a stable voter id', async () => {
    mockFetch(
      [
        ['/api/vote/map', (_url, init) =>
          init?.method === 'POST'
            ? jsonResponse({ map_id: 'map-1', vote: 1, net_votes: 1 })
            : jsonResponse({ map_id: 'map-1', net_votes: 1, my_vote: 1 })],
      ],
      htmlFallback,
    );

    const submitted = await submitMapVote('map-1', 1);
    expect(submitted.net_votes).toBe(1);

    const voterId = localStorage.getItem('acb_voter_id');
    expect(voterId).toBeTruthy();
    expect(String(calls[0]?.init?.body)).toContain(voterId!);

    const fetched = await fetchMapVotes('map-1');
    expect(fetched.my_vote).toBe(1);
    // The GET carries the same voter identity.
    expect(calls[1]?.url).toContain(`voter_id=${encodeURIComponent(voterId!)}`);
  });

  it('prediction submits hit /api/predict', async () => {
    mockFetch(
      [['/api/predict', () => jsonResponse({ success: true })]],
      htmlFallback,
    );

    await expect(submitPrediction('match-1', 'bot-1', 'p-1')).resolves.toBeDefined();
    expect(calls[0]?.url).toContain('/api/predict');
  });
});

describe('match-tier 503s surface as MatchTierOfflineError', () => {
  it('fetchOpenPredictions', async () => {
    mockFetch([['/api/predictions/open', () => MATCH_TIER_503('Predictions') as unknown as Response]], htmlFallback);
    await expect(fetchOpenPredictions('p-1')).rejects.toBeInstanceOf(MatchTierOfflineError);
  });

  it('fetchPredictionHistory', async () => {
    mockFetch([['/api/predictions/history', () => MATCH_TIER_503('Predictions') as unknown as Response]], htmlFallback);
    await expect(fetchPredictionHistory('p-1', 20)).rejects.toBeInstanceOf(MatchTierOfflineError);
  });

  it('submitPrediction', async () => {
    mockFetch([['/api/predict', () => MATCH_TIER_503('Match predictions') as unknown as Response]], htmlFallback);
    await expect(submitPrediction('match-1', 'bot-1', 'p-1')).rejects.toBeInstanceOf(MatchTierOfflineError);
  });

  it('submitMapVote', async () => {
    mockFetch([['/api/vote/map', () => MATCH_TIER_503('Map voting') as unknown as Response]], htmlFallback);
    await expect(submitMapVote('map-1', 1)).rejects.toBeInstanceOf(MatchTierOfflineError);
  });

  it('fetchMapVotes', async () => {
    mockFetch([['/api/vote/map/', () => MATCH_TIER_503('Map voting') as unknown as Response]], htmlFallback);
    await expect(fetchMapVotes('map-1')).rejects.toBeInstanceOf(MatchTierOfflineError);
  });

  it('the error text is the server message, ready to render', async () => {
    mockFetch([['/api/predictions/open', () => MATCH_TIER_503('Predictions') as unknown as Response]], htmlFallback);
    const err = await fetchOpenPredictions('p-1').catch((e: Error) => e);
    expect((err as MatchTierOfflineError).message).toContain('offline');
  });
});

describe('HTML-fallback backstop', () => {
  it('registerBot reports failure instead of parsing HTML as JSON', async () => {
    mockFetch([], htmlFallback);

    const result = await registerBot({ name: 'x', endpoint_url: 'https://x', owner_id: 'o' });

    expect(result.success).toBe(false);
    expect(result.error).toContain('non-JSON');
  });

  it('rotateApiKey reports failure instead of parsing HTML as JSON', async () => {
    mockFetch([], htmlFallback);

    const result = await rotateApiKey('bot-1', 'secret');

    expect(result.success).toBe(false);
    expect(result.error).toContain('non-JSON');
  });

  it('fetchMapVotes throws rather than believing HTML', async () => {
    mockFetch([], htmlFallback);

    await expect(fetchMapVotes('map-1')).rejects.toThrow(/non-JSON/);
  });
});
