/**
 * Map voting on the replay page with the Pages Function transport live
 * (bead aicodeba-84d1d61b).
 *
 * Map voting is a LIVE capability of the /api function. Loading a real
 * replay through the page's URL flow must populate the map metadata panel
 * (it comes from the replay file), fetch the current tallies from
 * `/api/vote/map/{map_id}`, and record a click as a real POST — updating the
 * count and disabling the buttons for this voter. A match-tier 503 renders
 * the server's offline notice with inert buttons instead of dead-looking
 * controls.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { Replay } from '../types';

const calls: string[] = [];

const REPLAY_URL = 'https://example.com/replays/mapvote-fixture.json';
const MAP_ID = 'map-fixture-1';

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
      map_id: MAP_ID,
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

async function loadReplayThroughThePage(): Promise<void> {
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
}

describe('replay page map vote with the transport live', () => {
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

    globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      calls.push(url);
      const json = (body: unknown, status = 200) => ({
        ok: status >= 200 && status < 300,
        status,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => body,
      }) as unknown as Response;

      if (url === REPLAY_URL) return json(replayFixture());
      // GET tallies (query carries the voter id) and POST votes are distinct.
      if (url === '/api/vote/map' && init?.method === 'POST') {
        return json({ map_id: MAP_ID, vote: 1, net_votes: 4 });
      }
      if (url.startsWith(`/api/vote/map/${MAP_ID}?`)) {
        return json({ map_id: MAP_ID, net_votes: 3 });
      }
      if (url === '/data/matches/mapvote-fixture/feedback.json') return json({ feedback: [] });
      return json({}, 404);
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

  it('loads tallies from the API and records a click as a real vote', async () => {
    await loadReplayThroughThePage();

    // The GET populated the count from the function's answer.
    await waitFor(
      () => document.getElementById('map-vote-count')?.textContent === '3',
      'vote count to be fetched',
    );
    const up = document.getElementById('map-vote-up') as HTMLButtonElement;
    const down = document.getElementById('map-vote-down') as HTMLButtonElement;
    expect(up.disabled).toBe(false);
    expect(down.disabled).toBe(false);
    expect(calls.some(url => url.startsWith(`/api/vote/map/${MAP_ID}?voter_id=`))).toBe(true);

    // Click upvote → POST /api/vote/map with this browser's voter id.
    up.click();
    await waitFor(
      () => document.getElementById('map-vote-count')?.textContent === '4',
      'the POSTed net votes to render',
    );

    const postCall = calls.find(url => url.endsWith('/api/vote/map'));
    expect(postCall).toBeTruthy();
    const body = JSON.parse(String(vi.mocked(globalThis.fetch).mock.calls.find(
      c => String(c[0]).endsWith('/api/vote/map'),
    )?.[1]?.body));
    expect(body).toMatchObject({ map_id: MAP_ID, vote: 1, voter_id: localStorage.getItem('acb_voter_id') });

    expect(document.getElementById('map-vote-status')!.textContent).toBe('You upvoted this map');
    expect((document.getElementById('map-vote-up') as HTMLButtonElement).disabled).toBe(true);
    expect((document.getElementById('map-vote-down') as HTMLButtonElement).disabled).toBe(true);
  });

  it('keeps the map metadata panel working — it comes from the replay file', async () => {
    await loadReplayThroughThePage();

    expect(document.getElementById('map-info-dimensions')?.textContent).toBe('40 x 40');
    expect(document.getElementById('map-info-wall-density')?.textContent).toContain('0.1%');
    expect(document.getElementById('map-info-energy-count')?.textContent).toBe('2');
  });

  it('renders the server offline notice with inert buttons on a match-tier 503', async () => {
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      calls.push(url);
      const json = (body: unknown, status = 200) => ({
        ok: status >= 200 && status < 300,
        status,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => body,
      }) as unknown as Response;

      if (url === REPLAY_URL) return json(replayFixture());
      if (url.startsWith('/api/')) {
        return json(
          {
            error: 'Map voting is offline: the compute tier that runs matches is not deployed.',
            code: 'match_tier_offline',
          },
          503,
        );
      }
      if (url === '/data/matches/mapvote-fixture/feedback.json') return json({ feedback: [] });
      return json({}, 404);
    });

    await loadReplayThroughThePage();
    await waitFor(
      () => document.getElementById('map-vote-status')?.textContent?.includes('offline'),
      'the offline notice to render',
    );

    expect(document.getElementById('map-vote-status')!.textContent).toContain('Map voting is offline');
    expect((document.getElementById('map-vote-up') as HTMLButtonElement).disabled).toBe(true);
    expect((document.getElementById('map-vote-down') as HTMLButtonElement).disabled).toBe(true);
  });
});
