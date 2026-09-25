/**
 * Merge contract for fetchLeaderboardWithDeltas (ADR-001 reconciliation).
 *
 * leaderboard.json is the full batch build; leaderboard/live-delta.json is the
 * worker's per-match patch of the rows that moved since that build (producer
 * side pinned in cmd/acb-worker/live_delta_test.go). These tests hold the
 * consumer half: a fresh delta replaces the absolute fields — including a
 * legitimate 0 matches_won / win_rate from a fresh loss, which must overwrite
 * the stale batch values rather than fall back to them — a delta older than
 * the batch is skipped (already absorbed by the full build), and an empty or
 * missing delta file leaves the batch untouched.
 *
 * Each test gets a fresh module instance: the swr cache api-types keeps for
 * leaderboard.json is module-level, so sharing the import across tests would
 * serve the first test's batch to every later one.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LeaderboardEntry } from './api-types';

type ApiTypes = typeof import('./api-types');

const BATCH_UPDATED = '2026-09-25T12:00:00Z';
const DELTA_UPDATED = '2026-09-25T12:30:00Z';

function entry(botId: string, over: Partial<LeaderboardEntry> = {}): LeaderboardEntry {
  return {
    rank: 1,
    bot_id: botId,
    name: botId,
    owner_id: 'owner-delta',
    rating: 1000,
    rating_deviation: 350,
    matches_played: 10,
    matches_won: 5,
    win_rate: 50,
    health_status: 'healthy',
    ...over,
  };
}

// First-win live delta the worker publishes for a fresh bot — the reference
// vectors pinned in cmd/acb-worker/glicko2_test.go (win from defaults:
// 1662.310894 mu / 290.318964 RD on the 1500 scale, 1081.672966 display).
const FIRST_WIN_DELTA = {
  rating_delta: 281.672966,
  new_rating: 1081.672966,
  new_rating_deviation: 290.318964,
  new_matches_played: 1,
  new_matches_won: 1,
  new_win_rate: 100,
};

function stubFetch(batch: unknown, delta: unknown, deltaStatus = 200): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string | URL | Request) => {
      const path = String(url);
      const [status, body] = path.includes('/data/leaderboard.json')
        ? [200, batch]
        : [deltaStatus, delta];
      return {
        ok: status === 200,
        status,
        json: async () => body,
      };
    })
  );
}

describe('fetchLeaderboardWithDeltas merge contract', () => {
  let fetchLeaderboardWithDeltas: ApiTypes['fetchLeaderboardWithDeltas'];

  beforeEach(async () => {
    vi.resetModules();
    ({ fetchLeaderboardWithDeltas } = await import('./api-types'));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('applies a fresh delta to the matching row, honoring zero-valued fields', async () => {
    stubFetch(
      {
        updated_at: BATCH_UPDATED,
        entries: [
          entry('fresh-winner', { matches_played: 0, matches_won: 0, win_rate: 0 }),
          entry('fresh-loser', { rank: 2, rating: 1200, rating_deviation: 300 }),
          entry('unmoved', { rank: 3, name: 'not in the delta file' }),
        ],
      },
      {
        updated_at: DELTA_UPDATED,
        deltas: {
          'fresh-winner': FIRST_WIN_DELTA,
          // The loss record: every absolute field present, won/win_rate at 0.
          'fresh-loser': {
            rating_delta: -20,
            new_rating: 1180,
            new_rating_deviation: 280,
            new_matches_played: 11,
            new_matches_won: 0,
            new_win_rate: 0,
          },
        },
      },
    );

    const { entries, updated_at } = await fetchLeaderboardWithDeltas();

    expect(updated_at).toBe(DELTA_UPDATED);

    const winner = entries.find((e) => e.bot_id === 'fresh-winner')!;
    expect(winner.rating).toBe(1081.672966);
    expect(winner.rating_deviation).toBe(290.318964);
    expect(winner.matches_played).toBe(1);
    expect(winner.matches_won).toBe(1);
    expect(winner.win_rate).toBe(100); // percentage scale, same as the batch rows

    // The heart of the merge: a fresh loss legitimately carries 0s, and those
    // 0s must replace the stale batch record — a ?? fallback on an omitted
    // field would keep displaying 5/10 = 50% after the loss.
    const loser = entries.find((e) => e.bot_id === 'fresh-loser')!;
    expect(loser.rating).toBe(1180);
    expect(loser.matches_played).toBe(11);
    expect(loser.matches_won).toBe(0);
    expect(loser.win_rate).toBe(0);

    const unmoved = entries.find((e) => e.bot_id === 'unmoved')!;
    expect(unmoved.rating).toBe(1000);
    expect(unmoved.win_rate).toBe(50);
  });

  it('skips a delta older than the batch build', async () => {
    stubFetch(
      { updated_at: BATCH_UPDATED, entries: [entry('stale-delta-bot')] },
      {
        updated_at: '2026-09-25T11:00:00Z', // predates the batch
        deltas: { 'stale-delta-bot': { ...FIRST_WIN_DELTA, new_rating: 9999 } },
      },
    );

    const { entries } = await fetchLeaderboardWithDeltas();
    expect(entries[0].rating).toBe(1000); // batch value survived
  });

  it('returns the batch untouched when the delta file is empty or missing', async () => {
    stubFetch({ updated_at: BATCH_UPDATED, entries: [entry('solo-bot')] }, { updated_at: '', deltas: {} });
    const empty = await fetchLeaderboardWithDeltas();
    expect(empty.updated_at).toBe(BATCH_UPDATED);
    expect(empty.entries[0].rating).toBe(1000);

    stubFetch({ updated_at: BATCH_UPDATED, entries: [entry('solo-bot')] }, undefined, 404);
    const missing = await fetchLeaderboardWithDeltas();
    expect(missing.entries[0].rating).toBe(1000);
  });
});
