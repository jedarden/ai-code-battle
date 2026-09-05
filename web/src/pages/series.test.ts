/**
 * Unit tests for the §14.7 series page spoiler toggle
 * (aicodeba-2c72ad52, child of the §14.7 umbrella aicodeba-d7914ca2).
 *
 * These exercise the real page module — renderSeriesPage() mounts the shell,
 * fetches /data/series/index.json, renders the bracket list and wires the
 * #spoiler-toggle change listener — the same jsdom approach replay.test.ts
 * takes with renderReplayPage(), so no markup is duplicated here.
 *
 * What the shipped spoiler mechanism actually is (pinned as shipped):
 *  - Checking the toggle toggles `.spoiler-hidden` on `.series-list`, the
 *    ancestor the page's own rule scopes against — not on `.series-page`.
 *  - Of the three selectors in that rule (`.bracket-progress`,
 *    `.bracket-dot`, `.game-result-text`) only `.bracket-dot` has a live
 *    target in the list view. `.bracket-progress` is emitted by no markup in
 *    the repo (the container ships as `.bracket-container`), and
 *    `.game-result-text` exists only in the detail view, which is a sibling
 *    of `.series-list` rather than a descendant of it.
 *  - The detail view hides result text by substituting `***` for the winner
 *    line when the toggle was checked at open time, and tags the affected
 *    nodes `spoiler-blur` — a class no stylesheet consumes yet.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { Series, SeriesGame } from '../types';
import { renderSeriesPage } from './series';

const GAMES: SeriesGame[] = [
  {
    match_id: 'm-1',
    game_number: 1,
    winner_id: 'bot-alpha',
    winner_slot: 0,
    turns: 214,
    completed_at: '2026-09-01T10:00:00Z',
  },
  {
    match_id: 'm-2',
    game_number: 2,
    winner_id: 'bot-beta',
    winner_slot: 1,
    turns: 188,
    completed_at: '2026-09-01T10:30:00Z',
  },
  {
    match_id: 'm-3',
    game_number: 3,
    winner_id: null,
    winner_slot: null,
    turns: null,
    completed_at: null,
  },
];

const SERIES: Series = {
  id: 'ser-alpha-beta',
  bot1_id: 'bot-alpha',
  bot2_id: 'bot-beta',
  bot1_name: 'Alpha',
  bot2_name: 'Beta',
  best_of: 5,
  status: 'active',
  bot1_wins: 1,
  bot2_wins: 1,
  winner_id: null,
  scheduled_at: null,
  completed_at: null,
  games: GAMES,
};

function jsonResponse(body: unknown): Response {
  return { ok: true, status: 200, json: async () => body } as unknown as Response;
}

/** Route the page's two fetches (index + detail payload) at the fixtures. */
function mockSeriesFetch(): void {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes('/data/series/index.json')) {
      return jsonResponse({ updated_at: '2026-09-05T00:00:00Z', series: [SERIES] });
    }
    if (url.includes(`/data/series/${SERIES.id}.json`)) {
      return jsonResponse(SERIES);
    }
    return { ok: false, status: 404, json: async () => ({}) } as unknown as Response;
  }) as unknown as typeof fetch;
}

function spoilerToggle(): HTMLInputElement {
  return document.getElementById('spoiler-toggle') as HTMLInputElement;
}

function seriesList(): HTMLElement {
  return document.querySelector('.series-list') as HTMLElement;
}

/** Open the detail view the way a user does — click the series card. */
async function openDetail(): Promise<HTMLElement> {
  (seriesList().querySelector('.series-card') as HTMLElement).click();
  await vi.waitFor(() => {
    const detail = document.getElementById('series-detail');
    if (!detail || detail.style.display !== 'block') {
      throw new Error('detail view did not open');
    }
  });
  return document.getElementById('series-detail-content') as HTMLElement;
}

describe('series page spoiler toggle (§14.7)', () => {
  beforeEach(() => {
    document.body.innerHTML = '<div id="app"></div>';
    mockSeriesFetch();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('mounts the real page and renders the bracket list from the index payload', async () => {
    await renderSeriesPage();

    expect(document.querySelector('.series-page')).toBeTruthy();
    expect(spoilerToggle()).toBeTruthy();

    const list = seriesList();
    expect(list.querySelectorAll('.series-card')).toHaveLength(1);

    // One dot per best_of slot, marked with the result each finished game had.
    const dots = list.querySelectorAll('.bracket-dot');
    expect(dots).toHaveLength(SERIES.best_of);
    expect(dots[0]).toHaveClass('win-a');
    expect(dots[1]).toHaveClass('win-b');
    expect(dots[2]).toHaveClass('pending');
  });

  it('flips .spoiler-hidden on the list root with the toggle, both ways, with no one-way leak', async () => {
    await renderSeriesPage();

    const toggle = spoilerToggle();
    const list = seriesList();

    expect(toggle.checked).toBe(false);
    expect(list.classList.contains('spoiler-hidden')).toBe(false);

    toggle.click();
    expect(toggle.checked).toBe(true);
    expect(list.classList.contains('spoiler-hidden')).toBe(true);

    toggle.click();
    expect(toggle.checked).toBe(false);
    expect(list.classList.contains('spoiler-hidden')).toBe(false);

    // A third flip proves the class tracks the box rather than sticking.
    toggle.click();
    expect(toggle.checked).toBe(true);
    expect(list.classList.contains('spoiler-hidden')).toBe(true);

    // The detail view's own tag never leaks into the list root.
    expect(list.classList.contains('spoiler-blur')).toBe(false);
  });

  it('brings the bracket result marks under the shipped rule while checked and restores them on uncheck', async () => {
    await renderSeriesPage();

    const dots = Array.from(seriesList().querySelectorAll('.bracket-dot'));
    expect(dots.length).toBeGreaterThan(0);

    // Unchecked: the shipped selector matches nothing.
    for (const dot of dots) {
      expect(dot.matches('.spoiler-hidden .bracket-dot')).toBe(false);
    }

    spoilerToggle().click();
    // Checked: every result-bearing dot now sits under the rule's scope, so it
    // takes the rule's blur — while staying in the DOM, so unchecking undoes it.
    for (const dot of dots) {
      expect(dot.matches('.spoiler-hidden .bracket-dot')).toBe(true);
      expect(dot.isConnected).toBe(true);
    }

    spoilerToggle().click();
    for (const dot of dots) {
      expect(dot.matches('.spoiler-hidden .bracket-dot')).toBe(false);
      expect(dot.isConnected).toBe(true);
    }
  });

  it('masks per-game result text in the detail view when opened hidden and restores it when opened visible', async () => {
    await renderSeriesPage();

    // Spoilers visible: the winner line is spelled out, per game.
    let content = await openDetail();
    let results = Array.from(content.querySelectorAll('.game-result-text'));
    expect(results).toHaveLength(GAMES.length);
    expect(results[0].textContent).toContain('Winner: Alpha');
    expect(results[1].textContent).toContain('Winner: Beta');
    expect(results[2].textContent).toContain('Not yet played');
    expect(content.querySelector('.game-result-text.spoiler-blur')).toBeFalsy();

    // Back to the list, hide spoilers, open the same series again.
    (document.getElementById('back-btn') as HTMLButtonElement).click();
    expect(spoilerToggle().checked).toBe(false);
    spoilerToggle().click();
    expect(seriesList().classList.contains('spoiler-hidden')).toBe(true);

    content = await openDetail();
    results = Array.from(content.querySelectorAll('.game-result-text'));
    expect(results).toHaveLength(GAMES.length);
    for (const result of results) {
      expect(result.textContent).toContain('***');
      expect(result.textContent).not.toContain('Winner:');
    }

    // The detail view also tags the affected nodes for its own blur hook —
    // the mask above is what actually hides the text today.
    expect(content.querySelectorAll('.game-result-text.spoiler-blur')).toHaveLength(GAMES.length);
    expect(content.querySelectorAll('.bracket-dot.spoiler-blur').length).toBeGreaterThan(0);
  });
});
