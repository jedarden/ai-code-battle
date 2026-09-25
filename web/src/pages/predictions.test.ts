/**
 * Predictions page view-only contract (bead aicodeba-07caa4ec).
 *
 * Picking bots, personal history, and the 15s resolution poll all target the
 * same-origin `/api/predictions*` routes, which the Pages SPA fallback
 * answers with HTML — there is no public API endpoint yet. With the transport
 * disabled the page must render explicit unavailable notices for the two
 * live sections, never issue an `/api` request, skip the poll timer, and
 * keep the static `/data`-backed Top Predictors leaderboard working.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderPredictionsPage, cleanupPredictionsPage } from './predictions';

const calls: string[] = [];

beforeEach(() => {
  vi.useFakeTimers();
  document.body.innerHTML = '<div id="app"></div>';
  calls.length = 0;
  localStorage.clear();
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    calls.push(url);
    if (url.startsWith('/api/')) {
      throw new Error(`unexpected /api fetch while transport is disabled: ${url}`);
    }
    if (url === '/data/predictions/leaderboard.json') {
      return {
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({
          updated_at: '2026-09-25T00:00:00Z',
          entries: [
            { predictor_id: 'p-1', correct: 5, incorrect: 1, streak: 3, best_streak: 4 },
          ],
        }),
      } as unknown as Response;
    }
    // Bot profile lookups (and anything else static) may 404 harmlessly.
    return {
      ok: false,
      status: 404,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({}),
    } as unknown as Response;
  });
});

afterEach(() => {
  cleanupPredictionsPage();
  vi.useRealTimers();
  document.body.innerHTML = '';
});

describe('renderPredictionsPage with no API transport', () => {
  it('renders the view-only notice naming the missing predictions API', async () => {
    await renderPredictionsPage();

    const notice = document.getElementById('predictions-transport-notice');
    expect(notice).not.toBeNull();
    // The notice copy wraps across source lines — compare on collapsed space.
    const text = notice!.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('Predictions are view-only right now');
    expect(text).toContain('predictions API');
    expect(text).toContain('has no public endpoint');
  });

  it('shows unavailable notices for open matches and history — no dead fetches', async () => {
    await renderPredictionsPage();

    const openMatches = document.getElementById('open-matches-unavailable');
    expect(openMatches).not.toBeNull();
    expect(openMatches!.textContent).toContain('predictions API has no public endpoint');

    const history = document.getElementById('history-unavailable');
    expect(history).not.toBeNull();
    expect(history!.textContent).toContain('predictions API has no public endpoint');

    expect(calls).not.toContainEqual(expect.stringContaining('/api/'));
    expect(calls.some(url => url.includes('/api/'))).toBe(false);
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
    expect(calls.some(url => url.includes('/api/'))).toBe(false);
  });

  it('does not schedule the 15s resolution poll', async () => {
    await renderPredictionsPage();
    const callsAfterRender = calls.length;

    await vi.advanceTimersByTimeAsync(60000);

    // Only the static leaderboard reads may have happened — nothing new.
    expect(calls.length).toBe(callsAfterRender);
    expect(calls.some(url => url.includes('/api/'))).toBe(false);
  });
});
