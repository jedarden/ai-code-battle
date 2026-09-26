/**
 * Live-feed integration: worker → R2 → SPA, end to end (ADR-001).
 *
 * The producer half — the worker PUTting matches/live-tail.json and
 * leaderboard/live-delta.json to R2 through the real write path, wire
 * assertions included — is pinned in cmd/acb-worker/live_feed_integration_test.go.
 * This file holds the consumer half: fixtures carrying exactly the field
 * names that test asserts on the wire, fed through the real merge helpers in
 * api-types (fetchMatchIndexWithTail, fetchLeaderboardWithDeltas) and then
 * through the three pages that call them — home, leaderboard and matches —
 * against a stubbed fetch that plays R2 and the batch build's /data files.
 *
 * Degradation matrix, per the feeds' failure modes:
 *   missing    (404)  → batch-only page, no error state
 *   stale      (older than the batch build) → batch values win; tail matches
 *                the batch already absorbed are deduped out
 *   malformed  (invalid JSON) → api-types throws; home degrades to
 *                batch-only via its .catch, matches and leaderboard surface
 *                their error state. The worker rewrites a fresh feed within
 *                seconds (max-age=10), so the outage is transient by design —
 *                these tests pin which pages ride it out and which don't.
 *
 * Every test gets a fresh module registry: api-types keeps the batch files in
 * a module-level SWR cache, so a shared import would serve the first test's
 * batch to every later one.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const BATCH_UPDATED = '2026-09-26T09:00:00Z';
const LIVE_UPDATED = '2026-09-26T09:10:00Z';

// Batch build (the index builder's full rebuild): two matches already
// indexed, one bot on the board. Participants are named distinctly from the
// live-tail ones so page assertions can tell which feed a row came from.
const batchMatch1 = {
  id: 'm-batch-1',
  completed_at: '2026-09-26T08:59:00Z',
  participants: [
    { bot_id: 'gamma', name: 'Gamma', score: 9, won: true },
    { bot_id: 'delta', name: 'Delta', score: 2, won: false },
  ],
  winner_id: 'gamma',
  map_id: 'map-duel-01',
  turns: 88,
  end_reason: 'annihilation',
};
const batchMatch2 = {
  id: 'm-batch-2',
  completed_at: '2026-09-26T08:00:00Z',
  participants: [
    { bot_id: 'gamma', name: 'Gamma', score: 5, won: false },
    { bot_id: 'delta', name: 'Delta', score: 11, won: true },
  ],
  winner_id: 'delta',
  map_id: 'map-duel-01',
  turns: 130,
  end_reason: 'zone-hold',
};

// The worker's post-batch publications, field-for-field what
// live_feed_integration_test.go asserts on the wire.
const liveMatch = {
  id: 'm-live-1',
  completed_at: '2026-09-26T09:09:59Z',
  participants: [
    { bot_id: 'alpha', name: 'Alpha', score: 12, won: true },
    { bot_id: 'beta', name: 'Beta', score: 4, won: false },
  ],
  winner_id: 'alpha',
  map_id: 'map-duel-01',
  turns: 142,
  end_reason: 'annihilation',
};

const batchLeaderboard = {
  updated_at: BATCH_UPDATED,
  entries: [
    {
      rank: 1,
      bot_id: 'alpha',
      name: 'Alpha',
      owner_id: 'owner-int',
      rating: 800,
      rating_deviation: 350,
      matches_played: 10,
      matches_won: 5,
      win_rate: 50,
      health_status: 'healthy',
    },
  ],
};

// A post-batch loss-free win published by the worker: 800 -> 820 display,
// record now 6/11. new_win_rate carries the full percentage precision the
// worker writes (100 * 6 / 11); the page renders one decimal.
const freshDelta = {
  updated_at: LIVE_UPDATED,
  deltas: {
    alpha: {
      rating_delta: 20,
      new_rating: 820,
      new_rating_deviation: 300,
      new_matches_played: 11,
      new_matches_won: 6,
      new_win_rate: 100 * (6 / 11),
    },
  },
};

interface Route {
  status?: number;
  body?: unknown;
  /** The body is wire garbage: response.ok is true but json() throws. */
  malformed?: boolean;
}

/** Plays R2 and the batch build: routes match by URL substring, unseen URLs 404. */
function stubFetch(routes: Record<string, Route>): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: string | URL | Request) => {
      const url =
        typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const hit = Object.keys(routes).find((key) => url.includes(key));
      if (!hit) return { ok: false, status: 404, json: async () => ({}) };
      const { status = 200, body = {}, malformed = false } = routes[hit];
      return {
        ok: status === 200,
        status,
        // The round trip through JSON is the wire: an undefined field the
        // worker's omitempty dropped must arrive absent, not present-but-null.
        json: async () => {
          if (malformed) throw new SyntaxError('Unexpected end of JSON input');
          return JSON.parse(JSON.stringify(body));
        },
      };
    })
  );
}

function batchRoutes(): Record<string, Route> {
  return {
    '/data/matches/index.json': {
      body: { updated_at: BATCH_UPDATED, matches: [batchMatch1, batchMatch2] },
    },
    '/data/leaderboard.json': { body: batchLeaderboard },
  };
}

beforeEach(() => {
  vi.resetModules();
  document.body.innerHTML = '<div id="app"></div>';
  // jsdom has neither observer; the pages' lazy sections and show-more
  // affordances only need them to exist. Re-stubbed per test — the afterEach
  // unstub below strips globals, not just fetch.
  for (const api of ['IntersectionObserver', 'ResizeObserver']) {
    vi.stubGlobal(
      api,
      class {
        observe(): void {}
        unobserve(): void {}
        disconnect(): void {}
      }
    );
  }
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('live-feed ingestion: merge helpers', () => {
  it('prepends tail matches the batch build has not absorbed yet', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': {
        body: { updated_at: LIVE_UPDATED, matches: [liveMatch] },
      },
    });

    const { fetchMatchIndexWithTail } = await import('./api-types');
    const merged = await fetchMatchIndexWithTail();

    expect(merged.matches.map((m) => m.id)).toEqual(['m-live-1', 'm-batch-1', 'm-batch-2']);
    expect(merged.updated_at).toBe(LIVE_UPDATED);
  });

  it('dedupes tail matches the batch build already absorbed', async () => {
    // The index builder rebuilt after m-batch-1 landed, but the tail still
    // leads with it: the merged index must list it once, not twice.
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': {
        body: { updated_at: LIVE_UPDATED, matches: [liveMatch, batchMatch1] },
      },
    });

    const { fetchMatchIndexWithTail } = await import('./api-types');
    const merged = await fetchMatchIndexWithTail();

    expect(merged.matches.map((m) => m.id)).toEqual(['m-live-1', 'm-batch-1', 'm-batch-2']);
  });

  it('serves the batch index untouched when the tail is missing', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': { status: 404, body: {} },
    });

    const { fetchMatchIndexWithTail } = await import('./api-types');
    const merged = await fetchMatchIndexWithTail();

    expect(merged.matches.map((m) => m.id)).toEqual(['m-batch-1', 'm-batch-2']);
    expect(merged.updated_at).toBe(BATCH_UPDATED);
  });
});

describe('live-feed ingestion: matches page', () => {
  it('renders the live-tail match above the batch history', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': {
        body: { updated_at: LIVE_UPDATED, matches: [liveMatch] },
      },
    });

    const { renderMatchesPage } = await import('./pages/matches');
    await renderMatchesPage();

    const cards = document.querySelectorAll('.match-card');
    expect(cards).toHaveLength(3);
    // Newest first: the tail publication leads the page.
    expect(cards[0].getAttribute('data-match-id')).toBe('m-live-1');
    expect(document.querySelector('.match-card[data-match-id="m-batch-1"]')).toBeTruthy();
    // The winner badge the tail's won flags drive.
    const liveCard = document.querySelector('.match-card[data-match-id="m-live-1"]')!;
    expect(liveCard.querySelector('.participant.winner .participant-name')?.textContent?.trim()).toBe('Alpha');
    expect(liveCard.querySelectorAll('.participant')).toHaveLength(2);
  });

  it('dedupes a tail match the batch already absorbed', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': {
        body: { updated_at: LIVE_UPDATED, matches: [liveMatch, batchMatch1] },
      },
    });

    const { renderMatchesPage } = await import('./pages/matches');
    await renderMatchesPage();

    expect(document.querySelectorAll('.match-card')).toHaveLength(3);
    expect(
      document.querySelectorAll('.match-card[data-match-id="m-batch-1"]')
    ).toHaveLength(1);
  });

  it('falls back to batch-only when the tail is missing', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': { status: 404, body: {} },
    });

    const { renderMatchesPage } = await import('./pages/matches');
    await renderMatchesPage();

    expect(document.querySelectorAll('.match-card')).toHaveLength(2);
    expect(document.querySelector('.matches-page .error')).toBeNull();
  });

  it('surfaces the error state when the tail is malformed', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': { malformed: true },
    });

    const { renderMatchesPage } = await import('./pages/matches');
    await renderMatchesPage();

    // The truncated feed takes the whole page into its error state even
    // though the batch data was fine — the pinned trade-off for the worker
    // self-healing the object within seconds.
    expect(document.querySelector('.matches-page .error')).toBeTruthy();
    expect(document.querySelector('.error')?.textContent).toContain('Failed to load match history');
    expect(document.querySelectorAll('.match-card')).toHaveLength(0);
  });
});

describe('live-feed ingestion: leaderboard page', () => {
  it('renders the delta-adjusted rating and record', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/leaderboard/live-delta.json': { body: freshDelta },
    });

    const { renderLeaderboardPage } = await import('./pages/leaderboard');
    await renderLeaderboardPage();

    const row = document.querySelector('.lb-row[data-bot-id="alpha"]')!;
    expect(row.querySelector('.rating-value')?.textContent).toBe('820');
    expect(row.querySelector('.rating-dev')?.textContent).toBe('±300');
    expect(row.querySelector('.lb-wl')?.textContent).toBe('6/11');
    expect(row.querySelector('.lb-winrate')?.textContent).toBe(`${(100 * (6 / 11)).toFixed(1)}%`);
  });

  it('keeps the batch rating when the delta predates the batch build', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/leaderboard/live-delta.json': {
        body: {
          updated_at: '2026-09-26T08:00:00Z', // stale: the batch already absorbed it
          deltas: { alpha: { ...freshDelta.deltas.alpha, new_rating: 9999 } },
        },
      },
    });

    const { renderLeaderboardPage } = await import('./pages/leaderboard');
    await renderLeaderboardPage();

    const row = document.querySelector('.lb-row[data-bot-id="alpha"]')!;
    expect(row.querySelector('.rating-value')?.textContent).toBe('800');
    expect(row.querySelector('.lb-wl')?.textContent).toBe('5/10');
  });

  it('keeps the batch rating when the delta file is missing', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/leaderboard/live-delta.json': { status: 404, body: {} },
    });

    const { renderLeaderboardPage } = await import('./pages/leaderboard');
    await renderLeaderboardPage();

    expect(
      document.querySelector('.lb-row[data-bot-id="alpha"] .rating-value')?.textContent
    ).toBe('800');
    expect(document.querySelector('.leaderboard-page .error')).toBeNull();
  });

  it('surfaces the error state when the delta file is malformed', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/leaderboard/live-delta.json': { malformed: true },
    });

    const { renderLeaderboardPage } = await import('./pages/leaderboard');
    await renderLeaderboardPage();

    expect(document.querySelector('.leaderboard-page .error')).toBeTruthy();
    expect(document.querySelector('.error')?.textContent).toContain('Failed to load leaderboard');
  });
});

describe('live-feed ingestion: home page', () => {
  it('features the newest live-tail match as the hero replay', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': {
        body: { updated_at: LIVE_UPDATED, matches: [liveMatch] },
      },
      '/r2/leaderboard/live-delta.json': { body: freshDelta },
    });

    const { renderHomePage } = await import('./pages/home');
    await renderHomePage();

    const title = document.querySelector('.home-replay-title')?.textContent ?? '';
    expect(title).toContain('Alpha');
    expect(title).toContain('Beta');
    expect(title).toContain('Winner:'); // the tail match has a winner, no stalemates featured
    const iframe = document.querySelector('.home-replay-embed iframe')!;
    expect(iframe.getAttribute('src')).toContain('match_id=m-live-1');
    // The Top 5 rows read the batch build — only the leaderboard page applies
    // the live deltas to the board.
    expect(document.querySelector('.home-lb-row .home-lb-rating')?.textContent).toBe('800');
  });

  it('falls back to the batch featured replay when the tail is missing', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': { status: 404, body: {} },
    });

    const { renderHomePage } = await import('./pages/home');
    await renderHomePage();

    const title = document.querySelector('.home-replay-title')?.textContent ?? '';
    expect(title).toContain('Gamma'); // batchMatch1's winner
    const iframe = document.querySelector('.home-replay-embed iframe')!;
    expect(iframe.getAttribute('src')).toContain('match_id=m-batch-1');
  });

  it('still renders batch-only with the demo replay when the tail is malformed', async () => {
    stubFetch({
      ...batchRoutes(),
      '/r2/matches/live-tail.json': { malformed: true },
    });

    const { renderHomePage } = await import('./pages/home');
    await renderHomePage();

    // Unlike matches and leaderboard, home rides out a broken feed: its
    // fetchers all carry .catch fallbacks, so the page renders with no
    // matches and the demo replay in the hero slot.
    expect(document.querySelector('.home-page')).toBeTruthy();
    expect(document.querySelector('.home-replay-title')?.textContent).toContain('Demo Replay');
  });
});
