import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Replay } from '../types';

// replay.ts loads the viewer dynamically. Keep this test focused on the real
// page keydown binding and gate, without involving canvas rendering.
vi.mock('../replay-viewer', () => ({
  ReplayViewer: class {
    private replay: Replay | null = null;
    private turn = 0;

    constructor() {
      return new Proxy(this, {
        get: (target, property, receiver) => {
          if (Reflect.has(target, property)) return Reflect.get(target, property, receiver);
          return () => undefined;
        },
      });
    }

    loadReplay(replay: Replay): void {
      this.replay = replay;
      this.turn = 0;
    }

    getReplay(): Replay | null {
      return this.replay;
    }

    getTurn(): number {
      return this.turn;
    }

    getTotalTurns(): number {
      return this.replay?.turns.length ?? 0;
    }

    getIsPlaying(): boolean {
      return false;
    }

    getTurnEvents(): [] {
      return [];
    }

    getCommentaryForTurn(): null {
      return null;
    }

    getDebugForCurrentTurn(): null {
      return null;
    }

    getDebugPlayerEnabled(): boolean {
      return true;
    }

    getFollowPlayer(): null {
      return null;
    }

    getViewMode(): 'standard' {
      return 'standard';
    }

    generateTranscript(): [] {
      return [];
    }

    setTurn(turn: number): void {
      this.turn = turn;
    }
  },
}));

import { renderReplayPage } from './replay';

const TEST_REPLAY: Replay = {
  format_version: '2.0',
  match_id: 'm_event_timeline_shortcut',
  config: {
    rows: 2,
    cols: 2,
    max_turns: 2,
    vision_radius2: 1,
    attack_radius2: 1,
    spawn_cost: 1,
    energy_interval: 1,
  },
  start_time: '2026-01-01T00:00:00Z',
  end_time: '2026-01-01T00:00:01Z',
  result: {
    winner: -1,
    reason: 'draw',
    turns: 2,
    scores: [0, 0],
    energy: [0, 0],
    bots_alive: [0, 0],
  },
  players: [
    { id: 0, name: 'Alpha' },
    { id: 1, name: 'Beta' },
  ],
  map: { rows: 2, cols: 2, walls: [], cores: [], energy_nodes: [] },
  turns: [
    { turn: 0, bots: [], cores: [], energy: [], scores: [0, 0], energy_held: [0, 0] },
    { turn: 1, bots: [], cores: [], energy: [], scores: [0, 0], energy_held: [0, 0] },
  ],
};

async function waitForElement(selector: string, timeoutMs = 3000): Promise<HTMLElement> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const element = document.querySelector<HTMLElement>(selector);
    if (element) return element;
    if (Date.now() > deadline) throw new Error(`${selector} did not appear within ${timeoutMs}ms`);
    await new Promise(resolve => setTimeout(resolve, 20));
  }
}

function keyE(): KeyboardEvent {
  const event = new KeyboardEvent('keydown', { code: 'KeyE', bubbles: true, cancelable: true });
  document.dispatchEvent(event);
  return event;
}

describe('replay event timeline shortcut', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
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
    document.body.innerHTML = '<div id="app"></div>';

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      const body = url.endsWith('/event-timeline-shortcut.json')
        ? TEST_REPLAY
        : url.includes('/commentary/')
          ? null
          : { feedback: [] };
      return {
        ok: true,
        status: 200,
        headers: new Headers(),
        body: null,
        json: async () => body,
      } as Response;
    }));
  });

  it('uses the replay page KeyE binding to hide and restore the ribbon container after load', async () => {
    renderReplayPage({});
    await waitForElement('#load-url-btn');

    const container = await waitForElement('#mobile-timeline');
    const beforeLoad = keyE();
    expect(beforeLoad.defaultPrevented).toBe(false);
    expect(container.style.display).toBe('');

    (document.querySelector('#url-input') as HTMLInputElement).value =
      'https://example.test/event-timeline-shortcut.json';
    (document.querySelector('#load-url-btn') as HTMLButtonElement).click();
    await waitForElement('#mobile-timeline .event-ribbon');

    const hide = keyE();
    expect(hide.defaultPrevented).toBe(true);
    expect(container.style.display).toBe('none');

    const show = keyE();
    expect(show.defaultPrevented).toBe(true);
    expect(container.style.display).toBe('');
  });
});
