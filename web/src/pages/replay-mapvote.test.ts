/**
 * Map-vote disabled state on the replay page (bead aicodeba-07caa4ec).
 *
 * `Rate this map` posts to same-origin `/api/vote/map`, which the Pages SPA
 * fallback answers with HTML — there is no public API endpoint. Loading a
 * real replay through the page's URL flow must leave the map metadata panel
 * populated (it comes from the replay file), replace the vote controls with
 * an explicit unavailable notice, and never issue an `/api` request.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { Replay } from '../types';

const calls: string[] = [];

const REPLAY_URL = 'https://example.com/replays/mapvote-fixture.json';

function replayFixture(): Replay {
  return {
    format_version: 'v1' as const,
    match_id: 'mapvote-fixture',
    start_time: '2026-09-25T00:00:00Z',
    end_time: '2026-09-25T00:01:00Z',
    config: {
      rows: 40,
      cols: 40,
      max_turns: 100,
      attack_radius2: 25,
      vision_radius2: 49,
      spawn_cost: 3,
      energy_interval: 5,
      zone_enabled: true,
      zone_start_turn: 10,
      zone_shrink_interval: 1,
      zone_shrink_step: 1,
      zone_min_radius: 2,
      map_id: 'map-fixture-1',
    },
    map: {
      rows: 40,
      cols: 40,
      walls: [{ row: 5, col: 5 }],
      cores: [
        { position: { row: 10, col: 10 }, owner: 0 },
        { position: { row: 30, col: 30 }, owner: 1 },
      ],
      energy_nodes: [
        { row: 15, col: 15 },
        { row: 25, col: 25 },
      ],
    },
    players: [
      { id: 0, name: 'Player 0' },
      { id: 1, name: 'Player 1' },
    ],
    turns: [
      {
        turn: 0,
        bots: [
          { id: 0, owner: 0, position: { row: 12, col: 12 }, alive: true },
          { id: 1, owner: 1, position: { row: 28, col: 28 }, alive: true },
        ],
        cores: [
          { position: { row: 10, col: 10 }, owner: 0, active: true },
          { position: { row: 30, col: 30 }, owner: 1, active: true },
        ],
        energy: [
          { row: 15, col: 15, has_energy: true, tick: 0 },
          { row: 25, col: 25, has_energy: true, tick: 0 },
        ],
        scores: [1, 1],
        energy_held: [0, 0],
        events: [],
      },
    ],
    result: {
      winner: 0,
      reason: 'elimination',
      turns: 1,
      scores: [3, 1],
      energy: [0, 0],
      bots_alive: [1, 0],
      crashed: [false, false],
      combat_deaths: [0, 0],
    },
    combat_deaths: [0, 0],
  };
}

async function waitFor(condition: () => boolean, what: string, timeoutMs = 5000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    if (condition()) return;
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}`);
    await new Promise(resolve => setTimeout(resolve, 25));
  }
}

describe('replay page map vote with no API transport', () => {
  beforeEach(() => {
    document.body.innerHTML = '<div id="app"></div>';
    localStorage.clear();
    calls.length = 0;

    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      calls.push(url);
      if (url.startsWith('/api/')) {
        throw new Error(`unexpected /api fetch while transport is disabled: ${url}`);
      }
      const json = (body: unknown, ok = true) => ({
        ok,
        status: ok ? 200 : 404,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => body,
      }) as unknown as Response;
      if (url === REPLAY_URL) return json(replayFixture());
      if (url === '/data/matches/mapvote-fixture/feedback.json') return json({ feedback: [] });
      return json({}, false);
    });

    // Minimal 2D canvas mock so ReplayViewer can construct.
    const ctx = {
      canvas: {},
      clearRect: vi.fn(), save: vi.fn(), restore: vi.fn(), beginPath: vi.fn(),
      moveTo: vi.fn(), lineTo: vi.fn(), closePath: vi.fn(), stroke: vi.fn(),
      fill: vi.fn(), arc: vi.fn(), fillRect: vi.fn(), strokeRect: vi.fn(),
      fillText: vi.fn(), strokeText: vi.fn(), measureText: vi.fn(() => ({ width: 0 })),
      translate: vi.fn(), scale: vi.fn(), rotate: vi.fn(), transform: vi.fn(),
      rect: vi.fn(), clip: vi.fn(), setLineDash: vi.fn(), createLinearGradient: vi.fn(() => ({ addColorStop: vi.fn() })),
    };
    Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
      writable: true,
      configurable: true,
      value: vi.fn(() => ctx),
    });
  });

  afterEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
  });

  it('replaces vote controls with an unavailable notice and never fetches /api', async () => {
    const { renderReplayPage } = await import('./replay');
    renderReplayPage({});

    // Wait for the lazy-loaded content to mount.
    let urlInput = document.getElementById('url-input') as HTMLInputElement | null;
    const deadline = Date.now() + 5000;
    while (!urlInput) {
      if (Date.now() > deadline) throw new Error('url-input never appeared');
      await new Promise(resolve => setTimeout(resolve, 25));
      urlInput = document.getElementById('url-input') as HTMLInputElement | null;
    }

    urlInput.value = REPLAY_URL;
    (document.getElementById('load-url-btn') as HTMLButtonElement).click();

    // initMapVote runs after the replay loads; it unhides #map-vote-panel.
    await waitFor(
      () => (document.getElementById('map-vote-panel') as HTMLElement | null)?.style.display === '',
      'map-vote panel to become visible',
    );
    // Give the disabled-branch a tick to replace the section content.
    await new Promise(resolve => setTimeout(resolve, 50));

    const section = document.getElementById('map-vote-section') as HTMLElement;
    expect(section).not.toBeNull();
    expect(section.textContent).toContain('Map voting is unavailable');
    expect(section.textContent).toContain('voting API has no public endpoint');
    // The dead vote buttons must be gone entirely, not merely disabled.
    expect(document.getElementById('map-vote-up')).toBeNull();
    expect(document.getElementById('map-vote-down')).toBeNull();

    // Map metadata still renders — it comes from the replay file itself.
    expect(document.getElementById('map-info-dimensions')?.textContent).toBe('40 x 40');

    expect(calls.some(url => url.includes('/api/'))).toBe(false);
  });
});
