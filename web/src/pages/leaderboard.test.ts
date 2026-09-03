/**
 * Skeleton → content swap tests for the leaderboard page (§16.14 #2).
 *
 * renderLeaderboardPage() renders skeletonLeaderboard() first and then
 * replaces #app wholesale, so the transition criteria live entirely in what
 * it renders next: the swapped-in page must be the shared .fade-in 150ms
 * opacity animation (data path, error path and empty path alike), both the
 * desktop rows and the mobile cards must sit inside the fading page, and the
 * header block above the first row must be the exact structure
 * skeletonLeaderboard() stands in for — otherwise every row and card moves
 * on the swap and the skeleton has lied about the space.
 *
 * jsdom has no animation engine, so the 150ms itself is held against the CSS
 * text (components.css is the single source of truth the .fade-in rule lives
 * in); the rendered-layout harness (web/layout-tests/
 * leaderboard-swap-parity.spec.ts) reads the same rule out of a real cascade.
 *
 * The last unpinned drift vector was numeric rather than structural: the
 * VirtualList ROW_HEIGHT constant versus the --lb-row-min-height the .lb-row
 * rule declares. The guard at the bottom holds the two against each other.
 */

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';
import { renderLeaderboardPage, ROW_HEIGHT } from './leaderboard';
import { fetchLeaderboardWithDeltas, type LeaderboardEntry } from '../api-types';
import { skeletonLeaderboard } from '../components/skeleton';

vi.mock('../api-types', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api-types')>();
  return {
    ...actual,
    fetchLeaderboardWithDeltas: vi.fn(),
  };
});

// vitest stubs CSS imports, so read the stylesheet source straight off disk —
// the .fade-in rule and its keyframes are what the browser actually applies.
const stylesDir = resolve(dirname(fileURLToPath(import.meta.url)), '../styles');
const componentsCss = readFileSync(resolve(stylesDir, 'components.css'), 'utf8');

// The virtual-list path (>50 entries) activates lazy sections for the mobile
// cards and observes its container, and jsdom has neither
// IntersectionObserver nor ResizeObserver.
beforeAll(() => {
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

function entry(rank: number): LeaderboardEntry {
  return {
    rank,
    bot_id: `bot-${rank}`,
    name: `Bot ${rank}`,
    owner_id: 'owner-swap',
    rating: 1000 + rank,
    rating_deviation: 50,
    matches_played: 24,
    matches_won: 10 + rank,
    win_rate: 41.7,
    health_status: 'healthy',
  };
}

/**
 * Extracts the body of `@keyframes <name>` by brace counting, and refuses a
 * rule with intermediate steps — a three-step opacity ramp or a silently
 * added transform keyframe would both change what the swap looks like.
 */
function keyframesBody(css: string, name: string): { from: string; to: string } {
  const start = css.indexOf(`@keyframes ${name}`);
  expect(start, `expected @keyframes ${name} in components.css`).toBeGreaterThanOrEqual(0);
  const open = css.indexOf('{', start);
  let depth = 0;
  let end = open;
  for (; end < css.length; end++) {
    if (css[end] === '{') depth++;
    else if (css[end] === '}') {
      depth--;
      if (depth === 0) break;
    }
  }
  const body = css.slice(open + 1, end);
  const from = body.match(/from\s*\{([^}]*)\}/)?.[1] ?? '';
  const to = body.match(/to\s*\{([^}]*)\}/)?.[1] ?? '';
  const rest = body.replace(/from\s*\{[^}]*\}/, '').replace(/to\s*\{[^}]*\}/, '').trim();
  expect(rest, 'fade keyframes must be a plain from → to, no intermediate steps').toBe('');
  return { from, to };
}

beforeEach(() => {
  document.body.innerHTML = '<div id="app"></div>';
});

describe('leaderboard skeleton → content swap', () => {
  it('fades the swapped-in content in over 150ms of opacity', async () => {
    vi.mocked(fetchLeaderboardWithDeltas).mockResolvedValue({
      entries: [entry(1), entry(2), entry(3)],
      updated_at: '2026-09-03T12:00:00Z',
    });
    await renderLeaderboardPage();

    // The whole page fades as one node — the class is on the container the
    // swap inserts, so rows and cards cannot appear before it or beside it.
    const page = document.querySelector('#app > .leaderboard-page');
    expect(page, 'the swap must render .leaderboard-page into #app').toBeTruthy();
    expect(page!.classList.contains('fade-in')).toBe(true);

    // And the class it carries is the shared 150ms opacity fade: the rule
    // names the fade-in keyframes at 150ms, and the keyframes move opacity —
    // and only opacity — so the swap can never slide or resize into place.
    const fadeRule = componentsCss.match(/\.fade-in\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(fadeRule.replace(/\s+/g, ' ')).toContain('fade-in 150ms');
    const { from, to } = keyframesBody(componentsCss, 'fade-in');
    expect(from.replace(/\s+/g, '').replace(/;$/, '')).toBe('opacity:0');
    expect(to.replace(/\s+/g, '').replace(/;$/, '')).toBe('opacity:1');
  });

  it('keeps the desktop rows and the mobile cards inside the fading page', async () => {
    vi.mocked(fetchLeaderboardWithDeltas).mockResolvedValue({
      entries: [entry(1), entry(2), entry(3)],
      updated_at: '2026-09-03T12:00:00Z',
    });
    await renderLeaderboardPage();

    const page = document.querySelector('#app > .leaderboard-page')!;
    expect(page.querySelector('#lb-desktop .lb-row'), 'desktop rows path').toBeTruthy();
    expect(page.querySelector('#lb-mobile .leaderboard-mobile-card'), 'mobile cards path').toBeTruthy();
  });

  it('renders the header block the skeleton stands in for, in the same order', async () => {
    vi.mocked(fetchLeaderboardWithDeltas).mockResolvedValue({
      entries: [entry(1), entry(2), entry(3)],
      updated_at: '2026-09-03T12:00:00Z',
    });
    await renderLeaderboardPage();

    // Live side: h1.page-title, then the updated-at and hint lines, then the
    // two list containers. #leaderboard-content is the wrapper the skeleton
    // has no equivalent of, so it must stay rule-free to be layout-neutral.
    const page = document.querySelector('#app > .leaderboard-page')!;
    const h1 = page.querySelector(':scope > h1');
    expect(h1?.className).toBe('page-title');
    expect(h1?.textContent).toBe('Leaderboard');
    const content = page.querySelector('#leaderboard-content')!;
    expect(content.className, 'the wrapper carries no rule of its own').toBe('');
    expect(content.getAttribute('style')).toBeNull();
    const [updatedAt, hint, desktop, mobile] = Array.from(content.children);
    expect(updatedAt.className).toBe('updated-at');
    expect(updatedAt.textContent).toMatch(/^Last updated: /);
    expect(hint.className).toBe('lb-hint');
    expect(hint.textContent).toBe('Click a row to see full stats');
    expect(desktop.id).toBe('lb-desktop');
    expect(mobile.id).toBe('lb-mobile');

    // Skeleton side: the same header, same order, same title — the stand-ins
    // are divs rather than p only because the HTML parser closes a p around
    // a block child (the classes carry identical geometry either way).
    const doc = new DOMParser().parseFromString(skeletonLeaderboard(), 'text/html');
    const skeletonPage = doc.querySelector('.skeleton-page')!;
    const skeletonH1 = skeletonPage.querySelector(':scope > h1');
    expect(skeletonH1?.className).toBe('page-title');
    expect(skeletonH1?.textContent).toBe('Leaderboard');
    const [skeletonH1El, skeletonUpdatedAt, skeletonHint, skeletonDesktop, skeletonMobile] =
      Array.from(skeletonPage.children);
    expect(skeletonH1El).toBe(skeletonH1);
    expect(skeletonUpdatedAt.className).toBe('updated-at');
    expect(skeletonHint.className).toBe('lb-hint');
    expect(skeletonUpdatedAt.querySelector('.skeleton-bar'), 'updated-at stand-in bar').toBeTruthy();
    expect(skeletonHint.querySelector('.skeleton-bar'), 'hint stand-in bar').toBeTruthy();
    expect(skeletonDesktop.id).toBe('lb-desktop');
    expect(skeletonMobile.id).toBe('lb-mobile');
  });

  it('renders the same header lines for the static and the virtual list', async () => {
    // The skeleton is data-blind, so the swap can only be shift-free if the
    // header block above the first row does not depend on the entry count.
    const header = () => {
      const content = document.querySelector('#leaderboard-content')!;
      return Array.from(content.children)
        .slice(0, 2)
        .map(el => `${el.className}:${el.textContent?.trim() ? 'text' : 'EMPTY'}`);
    };

    vi.mocked(fetchLeaderboardWithDeltas).mockResolvedValue({
      entries: [entry(1), entry(2), entry(3)],
      updated_at: '2026-09-03T12:00:00Z',
    });
    await renderLeaderboardPage();
    const staticHeader = header();

    const many = Array.from({ length: 60 }, (_, i) => entry(i + 1));
    vi.mocked(fetchLeaderboardWithDeltas).mockResolvedValue({
      entries: many,
      updated_at: '2026-09-03T12:00:00Z',
    });
    document.body.innerHTML = '<div id="app"></div>';
    await renderLeaderboardPage();
    const virtualHeader = header();

    expect(virtualHeader).toEqual(staticHeader);
    expect(virtualHeader).toEqual(['updated-at:text', 'lb-hint:text']);
  });

  it('fades the error swap too', async () => {
    vi.mocked(fetchLeaderboardWithDeltas).mockRejectedValue(new Error('network gone'));
    await renderLeaderboardPage();

    const page = document.querySelector('#app > .leaderboard-page');
    expect(page, 'the error state renders into the same page shell').toBeTruthy();
    expect(page!.classList.contains('fade-in')).toBe(true);
    expect(page!.querySelector('.error')).toBeTruthy();
    expect(page!.querySelector(':scope > h1')?.className).toBe('page-title');
  });

  it('fades the empty swap too', async () => {
    vi.mocked(fetchLeaderboardWithDeltas).mockResolvedValue({
      entries: [],
      updated_at: '2026-09-03T12:00:00Z',
    });
    await renderLeaderboardPage();

    const page = document.querySelector('#app > .leaderboard-page');
    expect(page!.classList.contains('fade-in')).toBe(true);
    expect(page!.querySelector('.empty-state')).toBeTruthy();
  });
});

// ─── ROW_HEIGHT ↔ the shared row min-height ─────────────────────────────────────
// renderDesktopList passes ROW_HEIGHT to VirtualList as rowHeight on
// >50-entry boards, while the rows it renders take their height from the
// --lb-row-min-height custom property declared on .lb-row in
// styles/components.css. Nothing else ties the two together: if either side
// changes without the other, VirtualList rows overlap or gap against
// .lb-row min-height on exactly the large leaderboards the virtual list
// exists for, shifting the §16.14 #2 skeleton→content swap there.
// skeleton.test.ts pins the CSS literal alone — this guard holds it against
// the constant the renderer actually consumes.
describe('VirtualList ROW_HEIGHT vs the shared .lb-row min-height', () => {
  it('keeps ROW_HEIGHT equal to the --lb-row-min-height the rows render at', () => {
    const lbRowRule = componentsCss.match(/\.lb-row\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(lbRowRule, '.lb-row rule not found in components.css').not.toBe('');
    const minHeight = lbRowRule.match(/--lb-row-min-height\s*:\s*([^;]+)/)?.[1]?.trim() ?? '';
    expect(minHeight, '--lb-row-min-height must stay a concrete px length').toMatch(/^[\d.]+px$/);
    expect(parseFloat(minHeight)).toBe(ROW_HEIGHT);
  });
});
