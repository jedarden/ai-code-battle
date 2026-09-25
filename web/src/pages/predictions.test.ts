/**
 * Predictions page contract with the transport live (bead aicodeba-84d1d61b).
 *
 * Predictions are a match-tier flow: the function answers the reads with 503
 * JSON (code match_tier_offline), and the page renders those server-driven
 * offline notices for open matches and history. The compile-time kill switch
 * is off — the requests really do go out — and the static /data-backed Top
 * Predictors leaderboard keeps working. The 15s poll runs and keeps the
 * offline state fresh, so a revived compute tier turns the sections on with
 * zero client changes.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderPredictionsPage, cleanupPredictionsPage } from './predictions';

const calls: string[] = [];

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  } as unknown as Response;
}

function matchTierOffline(): Response {
  return jsonResponse(
    {
      error: 'Predictions is offline: the compute tier that runs matches is not deployed.',
      code: 'match_tier_offline',
    },
    503,
  );
}

beforeEach(() => {
  vi.useFakeTimers();
  document.body.innerHTML = '<div id="app"></div>';
  calls.length = 0;
  localStorage.clear();
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    calls.push(url);
    if (url.startsWith('/api/predictions/')) return matchTierOffline();
    if (url === '/data/predictions/leaderboard.json') {
      return jsonResponse({
        updated_at: '2026-09-25T00:00:00Z',
        entries: [
          { predictor_id: 'p-1', correct: 5, incorrect: 1, streak: 3, best_streak: 4 },
        ],
      });
    }
    // Bot profile lookups (and anything else static) may 404 harmlessly.
    return jsonResponse({}, 404);
  });
});

afterEach(() => {
  cleanupPredictionsPage();
  vi.useRealTimers();
  document.body.innerHTML = '';
});

describe('renderPredictionsPage with the transport live', () => {
  it('renders no compile-time transport notice — the transport exists', async () => {
    await renderPredictionsPage();

    expect(document.getElementById('predictions-transport-notice')).toBeNull();
  });

  it('renders server-driven offline notices for open matches and history', async () => {
    await renderPredictionsPage();

    const openMatches = document.getElementById('open-matches-unavailable');
    expect(openMatches).not.toBeNull();
    expect(openMatches!.textContent).toContain('Predictions is offline');
    expect(openMatches!.textContent).toContain('compute tier');

    const history = document.getElementById('history-unavailable');
    expect(history).not.toBeNull();
    expect(history!.textContent).toContain('Predictions is offline');

    // The reads really went to the same-origin function.
    expect(calls.some(url => url.startsWith('/api/predictions/open'))).toBe(true);
    expect(calls.some(url => url.startsWith('/api/predictions/history'))).toBe(true);
  });

  it('offline is not a wrong answer — a 500 renders the generic failure instead', async () => {
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      calls.push(url);
      if (url.startsWith('/api/predictions/')) return jsonResponse({ error: 'boom' }, 500);
      if (url === '/data/predictions/leaderboard.json') return jsonResponse({ updated_at: '', entries: [] });
      return jsonResponse({}, 404);
    });

    await renderPredictionsPage();

    expect(document.getElementById('open-matches-unavailable')).toBeNull();
    expect(document.getElementById('history-unavailable')).toBeNull();
    expect(document.getElementById('open-matches-container')!.textContent).toContain('Failed to load open matches');
  });

  it('still renders the static leaderboard from /data', async () => {
    await renderPredictionsPage();

    const leaderboard = document.getElementById('leaderboard-container');
    expect(leaderboard).not.toBeNull();
    expect(leaderboard!.textContent).not.toContain('Failed to load leaderboard');
    // The entry renders (bot-name lookup 404s in the mock, so the predictor
    // id is the fallback label). The fetch itself may be satisfied by the
    // module-level SWR cache from an earlier test in this file — what
    // matters is it never comes from /api.
    expect(leaderboard!.textContent).toContain('p-1');
    expect(calls.some(url => url.includes('/api/') && url.includes('leaderboard'))).toBe(false);
  });

  it('keeps polling and the offline state stays honest', async () => {
    await renderPredictionsPage();
    const callsAfterRender = calls.length;

    await vi.advanceTimersByTimeAsync(60000);

    // The 15s poll re-issued the two match-tier reads (the transport is
    // live — polling is no longer skipped) and re-rendered the notices.
    expect(calls.length).toBeGreaterThan(callsAfterRender);
    expect(document.getElementById('open-matches-unavailable')).not.toBeNull();
    expect(document.getElementById('history-unavailable')).not.toBeNull();
  });
});
