/**
 * Layout-parity tests for the leaderboard and bot-profile skeletons (§16.14).
 * skeletonLeaderboard() must render the same #lb-desktop row structure as the
 * live page (renderDesktopRow in pages/leaderboard.ts), with column widths
 * sourced from the --lb-col-* custom properties declared once on .lb-row in
 * styles/components.css — never duplicated as px literals here — plus the same
 * #lb-mobile .mobile-cards card structure as renderMobileCard, whose geometry
 * comes from the .leaderboard-mobile-* rules in styles/mobile.css.
 * skeletonBotProfile() must render the same .bot-profile-page structure as
 * renderBotProfilePage's swap (pages/bot-profile.ts): breadcrumb, header, grid
 * sections in the live order, every wrapper reusing the live class so the
 * shared rules govern it. skeletonReplay() must render the same .replay-page
 * structure as initReplayViewerWithClass's markup template (pages/replay.ts):
 * the shared .skeleton-page root around the live .replay-page, the
 * h1.page-title, then .replay-layout of .replay-main (the canvas stand-in plus
 * the two mobile chrome blocks) and .replay-sidebar last, every wrapper
 * reusing the live class so the shared rules govern it. It keeps the shared
 * .skeleton-page root, which is
 * layout-transparent here (max-width + auto margins, no padding), so the
 * wrapped 900px column sits where the live page puts it and the swap shifts
 * nothing. The breadcrumb is mirrored as a bar because the swap renders it:
 * leaving it out let the real row (44px — its <a> picks up the base tap-target
 * floor — plus a 24px margin) appear above the header on the swap and shove
 * the whole page down. The live header renders no avatar, so no avatar-area
 * circle is invented for it.
 */

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';
import { skeletonLeaderboard, skeletonBotProfile, skeletonReplay, Skeleton } from './skeleton';
import { renderDesktopRow } from '../pages/leaderboard';
import type { LeaderboardEntry } from '../api-types';

// vitest stubs CSS imports by default, so read the stylesheet source straight
// off disk — it is the single source of truth the bars must resolve from.
// (A static new URL(x, import.meta.url) is rewritten by vite as an asset
// reference, so resolve the path manually.)
const stylesDir = resolve(dirname(fileURLToPath(import.meta.url)), '../styles');
const componentsCss = readFileSync(resolve(stylesDir, 'components.css'), 'utf8');
const mobileCss = readFileSync(resolve(stylesDir, 'mobile.css'), 'utf8');
const baseCss = readFileSync(resolve(stylesDir, 'base.css'), 'utf8');
// The shipped copy of the replay layout: index.html's inline <style> is the
// only stylesheet the browser loads (no src/styles/*.css reaches the bundle),
// so it carries its own copy of the rules the skeleton and live page share.
const indexHtml = readFileSync(resolve(stylesDir, '../../index.html'), 'utf8');
const indexCss = indexHtml.split('<style>')[1].split('</style>')[0];

// Desktop columns in renderDesktopRow order: rank, name, rating, wl,
// winrate, status, expand.
const COL_VARS = [
  '--lb-col-rank',
  '--lb-col-name-min',
  '--lb-col-rating',
  '--lb-col-wl',
  '--lb-col-winrate',
  '--lb-col-status',
  '--lb-col-expand',
] as const;

const lbRowRule = componentsCss.match(/\.lb-row\s*\{([^}]*)\}/)?.[1] ?? '';

// Mobile card rules from the phone block of mobile.css. The `:active`,
// `.rank-N` and `.expanded` variants all carry an extra class before the
// brace, so these plain-class regexes only ever match the base rules.
const cardRule = mobileCss.match(/\.leaderboard-mobile-card\s*\{([^}]*)\}/)?.[1] ?? '';
const rankRule = mobileCss.match(/\.leaderboard-mobile-rank\s*\{([^}]*)\}/)?.[1] ?? '';
const detailsRule = mobileCss.match(/\.leaderboard-mobile-details\s*\{([^}]*)\}/)?.[1] ?? '';

/** First `prop: value` declaration in a CSS/style-attribute fragment. */
function decl(css: string, prop: string): string {
  const m = css.match(new RegExp(`(?:^|[;{])\\s*${prop}\\s*:\\s*([^;]+)`, 'm'));
  expect(m, `expected "${prop}" declared (got: ${css.slice(0, 120)}…)`).toBeTruthy();
  return m![1].trim();
}

function desktopContainer(): HTMLElement {
  const doc = new DOMParser().parseFromString(skeletonLeaderboard(), 'text/html');
  const el = doc.getElementById('lb-desktop');
  expect(el, 'skeleton must wrap its desktop rows in #lb-desktop').toBeTruthy();
  return el!;
}

describe('Skeleton component style concatenation', () => {
  function styleOf(html: string): string {
    return html.match(/style="([^"]*)"/)![1];
  }

  it('keeps every declaration valid when extra has no trailing semicolon', () => {
    const style = styleOf(Skeleton({ variant: 'rectangle', width: '100%', height: '40px', extra: 'margin-top:10px' }));
    // Used to emit `margin-top:10pxborder-radius:…`, which the browser drops whole.
    expect(decl(style, 'margin-top')).toBe('10px');
    expect(decl(style, 'border-radius')).toBe('var(--radius-md)');
  });

  it('lets extra override the rectangle border-radius default', () => {
    const style = styleOf(Skeleton({ variant: 'rectangle', width: '100%', height: '80px', extra: 'border-radius:8px' }));
    // The default and the override are both present; CSS takes the last.
    const radii = [...style.matchAll(/border-radius\s*:\s*([^;]+)/g)].map(m => m[1].trim());
    expect(radii[radii.length - 1]).toBe('8px');
  });
});

describe('skeletonLeaderboard desktop rows', () => {
  it('wraps the desktop rows in a #lb-desktop container of .lb-row rows', () => {
    const desktop = desktopContainer();
    const rows = Array.from(desktop.children);
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(row.classList.contains('lb-row')).toBe(true);
      // Geometry must come from the shared .lb-row rule, not inline copies.
      expect(row.getAttribute('style')).toBeNull();
      expect(row.getAttribute('role')).toBeNull();
      expect(row.getAttribute('tabindex')).toBeNull();
    }
  });

  it('renders 7 shimmer bars per row that inherit the base skeleton-bar class', () => {
    for (const row of desktopContainer().children) {
      const bars = Array.from(row.children);
      expect(bars).toHaveLength(7);
      for (const bar of bars) {
        expect(bar.classList.contains('skeleton-bar')).toBe(true);
      }
    }
    // No bespoke animation CSS in the generated markup.
    expect(desktopContainer().innerHTML).not.toMatch(/animation\s*:/);
    expect(skeletonLeaderboard()).not.toContain('<style');
  });

  it('orders the bars rank, name, rating, wl, winrate, status, expand', () => {
    const expected = COL_VARS.map(v => `var(${v})`);
    for (const row of desktopContainer().children) {
      const widths = Array.from(row.children).map(bar =>
        decl(bar.getAttribute('style') ?? '', 'width')
      );
      expect(widths).toEqual(expected);
    }
  });

  it('gives the name bar flex:1 floored at the shared min-width', () => {
    for (const row of desktopContainer().children) {
      const nameStyle = row.children[1].getAttribute('style') ?? '';
      expect(decl(nameStyle, 'flex')).toBe('1');
      expect(decl(nameStyle, 'min-width')).toBe('var(--lb-col-name-min)');
    }
  });

  it('resolves every bar width from the shared --lb-col-* declarations', () => {
    expect(lbRowRule, '.lb-row rule not found in components.css').not.toBe('');
    for (const v of COL_VARS) {
      const value = decl(lbRowRule, v);
      // The shared declaration must stay a concrete width, not a reference.
      expect(value).toMatch(/^[\d.]+px$/);
    }
    // ...and the rendered rows must reference them instead of the values.
    const desktopHtml = desktopContainer().innerHTML;
    expect(desktopHtml).not.toMatch(/width\s*:\s*[\d.]+px/);
    for (const v of COL_VARS) {
      expect(desktopHtml).toContain(`var(${v})`);
    }
  });

  it('inherits .lb-row row geometry (gap, padding, border, min-height, box-sizing)', () => {
    expect(decl(lbRowRule, '--lb-row-min-height')).toBe('48px');
    expect(decl(lbRowRule, '--lb-row-gap')).toBe('var(--space-md)');
    expect(decl(lbRowRule, '--lb-row-padding')).toBe('var(--space-sm) var(--space-md)');
    expect(decl(lbRowRule, 'min-height')).toBe('var(--lb-row-min-height)');
    expect(decl(lbRowRule, 'gap')).toBe('var(--lb-row-gap)');
    expect(decl(lbRowRule, 'padding')).toBe('var(--lb-row-padding)');
    expect(decl(lbRowRule, 'border-bottom')).toBe('1px solid var(--border)');
    expect(decl(lbRowRule, 'box-sizing')).toBe('border-box');
  });
});

// ─── Live-column parity ─────────────────────────────────────────────────────────
// The bars promise the columns the live renderDesktopRow() row draws once the
// data lands. The guards above hold the bars against the --lb-col-*
// declarations; these hold the two sides against each other: the column list
// comes from renderDesktopRow() itself — the same source of truth the
// rendered-layout harness lays out (web/layout-tests/
// leaderboard-desktop-parity.spec.ts) — and every live column rule must take
// its width from the same shared declaration its bar references, so a
// hardcoded column width, a stale bar var, or a column added to the renderer
// fails here in `npm test` and not only in the browser harness.

// Column data is geometry-blind; only .lb-name's text varies with it.
const parityEntry: LeaderboardEntry = {
  rank: 4,
  bot_id: 'bot-parity',
  name: 'Parity Bot',
  owner_id: 'owner-parity',
  rating: 1000,
  rating_deviation: 50,
  matches_played: 24,
  matches_won: 10,
  win_rate: 41.7,
  health_status: 'healthy',
};

function liveColumnClasses(): string[] {
  const doc = new DOMParser().parseFromString(renderDesktopRow(parityEntry, 0), 'text/html');
  const row = doc.querySelector('.lb-row');
  expect(row, 'renderDesktopRow must render a .lb-row').toBeTruthy();
  return Array.from(row!.children).map(el => el.classList[0]);
}

// Live column class → the shared .lb-row declaration its width must come from.
// Fixed columns are sized by `width`; the name column flexes, so the
// declaration that actually sizes it under flex:1 is its `min-width` floor.
const COLUMN_WIDTH: Record<string, { prop: 'width' | 'min-width'; varName: string; flex?: string }> = {
  'lb-rank': { prop: 'width', varName: '--lb-col-rank' },
  'lb-name': { prop: 'min-width', varName: '--lb-col-name-min', flex: '1' },
  'lb-rating': { prop: 'width', varName: '--lb-col-rating' },
  'lb-wl': { prop: 'width', varName: '--lb-col-wl' },
  'lb-winrate': { prop: 'width', varName: '--lb-col-winrate' },
  'lb-status': { prop: 'width', varName: '--lb-col-status' },
  'lb-expand-icon': { prop: 'width', varName: '--lb-col-expand' },
};

describe('skeletonLeaderboard bars vs the live columns', () => {
  it('renders exactly one bar per live column', () => {
    const columns = liveColumnClasses();
    expect(columns.length).toBeGreaterThan(0);
    for (const cls of columns) {
      expect(COLUMN_WIDTH[cls], `live column .${cls} must be mapped to its shared width var`)
        .toBeTruthy();
    }
    for (const row of desktopContainer().children) {
      expect(Array.from(row.children), `one bar per live column (${columns.join(', ')})`)
        .toHaveLength(columns.length);
    }
  });

  it('takes each bar width from the same declaration its live column consumes', () => {
    const columns = liveColumnClasses();
    const rows = Array.from(desktopContainer().children);

    columns.forEach((cls, i) => {
      const { prop, varName, flex } = COLUMN_WIDTH[cls];
      const rule = componentsCss.match(new RegExp(`\\.${cls}\\s*\\{([^}]*)\\}`))?.[1] ?? '';
      expect(rule, `.${cls} rule not found in components.css`).not.toBe('');
      // Live side: the column's own rule must reach its width through the
      // shared .lb-row declaration — a hardcoded px there is the drift that
      // silently puts the column and its bar on different widths.
      expect(decl(rule, prop), `.${cls} width must come from ${varName}`).toBe(`var(${varName})`);
      if (flex) {
        expect(decl(rule, 'flex'), `.${cls} must flex like its bar does`).toBe(flex);
      }
      // Skeleton side: the bar in that column's position references the same
      // declaration — one declaration per column, consumed from both sides.
      for (const row of rows) {
        const style = row.children[i].getAttribute('style') ?? '';
        expect(decl(style, prop), `bar ${i} must mirror .${cls}'s ${prop}`).toBe(`var(${varName})`);
        if (flex) {
          expect(decl(style, 'flex'), `bar ${i} must flex like .${cls} does`).toBe(flex);
        }
      }
    });
  });
});

describe('skeletonLeaderboard header stand-ins', () => {
  // The swap target's header (renderLeaderboardPage + renderLeaderboard,
  // pages/leaderboard.ts): h1.page-title, then the updated-at and hint lines,
  // all above #lb-desktop. Everything the swap needs to keep still lives in
  // the shared rules — the h1 and both line wrappers reuse the live classes,
  // and the bars are sized to the font metrics of the text they stand in
  // for: root font-size (base.css html rule) × the live rule's font-size ×
  // body line-height.
  const htmlRule = baseCss.match(/(?:^|\})\s*html\s*\{([^}]*)\}/)?.[1] ?? '';
  const bodyRule = baseCss.match(/(?:^|\})\s*body\s*\{([^}]*)\}/)?.[1] ?? '';
  const updatedAtRule = componentsCss.match(/\.updated-at\s*\{([^}]*)\}/)?.[1] ?? '';
  const lbHintRule = componentsCss.match(/\.lb-hint\s*\{([^}]*)\}/)?.[1] ?? '';

  const rootPx = parseFloat(decl(htmlRule, 'font-size'));
  const textLineHeight = parseFloat(decl(bodyRule, 'line-height'));
  const lineBoxPx = (ruleCss: string): number =>
    parseFloat(decl(ruleCss, 'font-size')) * rootPx * textLineHeight;

  function pageRoot(): HTMLElement {
    const doc = new DOMParser().parseFromString(skeletonLeaderboard(), 'text/html');
    const el = doc.querySelector('.skeleton-page');
    expect(el, 'skeleton must keep its .skeleton-page root').toBeTruthy();
    return el!;
  }

  function barHeight(wrapperClass: string): number {
    const wrapper = pageRoot().querySelector(`:scope > .${wrapperClass}`);
    expect(wrapper, `skeleton must render the .${wrapperClass} stand-in`).toBeTruthy();
    const bar = wrapper!.querySelector('.skeleton-bar');
    expect(bar, `the .${wrapperClass} stand-in must hold a shimmer bar`).toBeTruthy();
    return parseFloat(decl(bar!.getAttribute('style') ?? '', 'height'));
  }

  it('repeats the live header: page-title h1, updated-at line, hint line, in order', () => {
    const [h1, updatedAt, hint, desktop, mobile] = Array.from(pageRoot().children);
    expect(h1.tagName).toBe('H1');
    expect(h1.className).toBe('page-title');
    expect(h1.textContent).toBe('Leaderboard');
    expect(updatedAt.className).toBe('updated-at');
    expect(hint.className).toBe('lb-hint');
    expect(desktop.id).toBe('lb-desktop');
    expect(mobile.id).toBe('lb-mobile');
  });

  it('takes no spacing of its own: the live classes own the margins', () => {
    const root = pageRoot();
    for (const cls of ['updated-at', 'lb-hint']) {
      const wrapper = root.querySelector(`:scope > .${cls}`);
      expect(wrapper, `.${cls} stand-in must exist`).toBeTruthy();
      expect(wrapper!.getAttribute('style'), `${cls} margin comes from its live rule`).toBeNull();
    }
    // ...and the rules must actually declare the margins, so an inline-free
    // wrapper cannot silently fall back to nothing.
    expect(decl(updatedAtRule, 'margin-bottom')).toBe('var(--space-md)');
    expect(decl(lbHintRule, 'margin-bottom')).toBe('var(--space-sm)');
  });

  it('sizes the updated-at bar to the live line it stands in for', () => {
    expect(barHeight('updated-at')).toBeCloseTo(lineBoxPx(updatedAtRule), 6);
  });

  it('sizes the hint bar to the live line it stands in for', () => {
    expect(barHeight('lb-hint')).toBeCloseTo(lineBoxPx(lbHintRule), 6);
  });
});

describe('skeletonLeaderboard mobile cards', () => {
  function mobileContainer(): HTMLElement {
    const doc = new DOMParser().parseFromString(skeletonLeaderboard(), 'text/html');
    const el = doc.getElementById('lb-mobile');
    expect(el, 'skeleton must render its mobile cards in #lb-mobile').toBeTruthy();
    expect(el!.classList.contains('mobile-cards')).toBe(true);
    return el!;
  }

  it('renders a .mobile-cards container after the desktop rows', () => {
    const doc = new DOMParser().parseFromString(skeletonLeaderboard(), 'text/html');
    const desktop = doc.getElementById('lb-desktop');
    const mobile = doc.getElementById('lb-mobile');
    expect(desktop).toBeTruthy();
    expect(mobile).toBeTruthy();
    expect(desktop!.nextElementSibling).toBe(mobile);
    const cards = Array.from(mobileContainer().children);
    expect(cards.length).toBeGreaterThan(0);
    for (const card of cards) {
      expect(card.classList.contains('leaderboard-mobile-card')).toBe(true);
    }
  });

  it('takes card geometry from the shared .leaderboard-mobile-card rule', () => {
    expect(cardRule, '.leaderboard-mobile-card rule not found in mobile.css').not.toBe('');
    expect(decl(cardRule, 'gap')).toBe('var(--space-md)');
    expect(decl(cardRule, 'padding')).toBe('var(--space-md)');
    expect(decl(cardRule, 'background-color')).toBe('var(--bg-secondary)');
    expect(decl(cardRule, 'border-radius')).toBe('var(--radius-md)');
    expect(decl(cardRule, 'margin-bottom')).toBe('var(--space-sm)');
    // Geometry comes from the shared rule, so the card root carries no inline
    // copy of it (the placeholder bars inside do carry inline dimensions).
    for (const card of mobileContainer().children) {
      expect(card.getAttribute('style')).toBeNull();
    }
  });

  it('mirrors the toggle row: rank, name, rating with smaller deviation, arrow', () => {
    // The live toggle is a <button>, so base.css's tap-target rule floors it at
    // 44px — the quantity that actually sets its height. The stand-in is a div
    // (a skeleton has nothing to activate) and cannot inherit an element
    // selector, so it must carry that floor inline, matching the rule exactly.
    const tapTargetRule = baseCss.match(/button,\s*a,\s*input,\s*select,\s*textarea\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(tapTargetRule, 'tap-target rule not found in base.css').not.toBe('');
    expect(decl(tapTargetRule, 'min-height')).toBe('44px');
    // The name bar stands in for .leaderboard-mobile-name, which carries no
    // margin of its own — and the stand-in columns must stay under the floor
    // that governs both toggles (the rendered parity spec measures the result).
    for (const card of mobileContainer().children) {
      const toggle = card.children[0];
      expect(toggle.classList.contains('mobile-card-toggle')).toBe(true);
      expect(toggle.tagName).toBe('DIV');
      expect(decl(toggle.getAttribute('style') ?? '', 'min-height')).toBe('44px');
      const [rank, info, arrow] = Array.from(toggle.children);
      expect(rank.classList.contains('leaderboard-mobile-rank')).toBe(true);
      expect(rank.querySelectorAll('.skeleton-bar').length).toBe(1);
      expect(info.classList.contains('leaderboard-mobile-info')).toBe(true);
      expect(arrow.classList.contains('mobile-card-arrow')).toBe(true);
      expect(arrow.querySelectorAll('.skeleton-bar').length).toBe(1);

      const infoBars = Array.from(info.children);
      const nameBar = infoBars[0];
      expect(nameBar.classList.contains('skeleton-bar')).toBe(true);
      expect(nameBar.getAttribute('style'), 'name bar must not add a margin the live name lacks')
        .not.toContain('margin-bottom');
      const rating = infoBars[1];
      expect(rating.classList.contains('leaderboard-mobile-rating')).toBe(true);
      const [ratingBar, deviationBar] = Array.from(rating.children);
      expect(ratingBar.classList.contains('skeleton-bar')).toBe(true);
      expect(deviationBar.classList.contains('skeleton-bar')).toBe(true);
      // The ±deviation stand-in must be smaller than the rating stand-in,
      // mirroring the live 0.8em span.
      expect(parseFloat(decl(deviationBar.getAttribute('style') ?? '', 'height'))).toBeLessThan(
        parseFloat(decl(ratingBar.getAttribute('style') ?? '', 'height'))
      );
      expect(parseFloat(decl(deviationBar.getAttribute('style') ?? '', 'width'))).toBeLessThan(
        parseFloat(decl(ratingBar.getAttribute('style') ?? '', 'width'))
      );
    }
  });

  it('sources the rank placeholder width from the shared min-width: 40px rule', () => {
    expect(rankRule, '.leaderboard-mobile-rank rule not found in mobile.css').not.toBe('');
    expect(decl(rankRule, 'min-width')).toBe('40px');
    for (const card of mobileContainer().children) {
      const rank = card.children[0].children[0];
      const style = rank.getAttribute('style');
      expect(style, 'rank box must not duplicate the shared min-width inline').toBeNull();
    }
  });

  it('mirrors the collapsed details block: four stat rows plus a full-width button bar', () => {
    expect(detailsRule, '.leaderboard-mobile-details rule not found in mobile.css').not.toBe('');
    // Collapsed exactly like the live card: no skeleton-only expansion.
    expect(decl(detailsRule, 'max-height')).toBe('0');
    expect(decl(detailsRule, 'overflow')).toBe('hidden');

    for (const card of mobileContainer().children) {
      const details = card.children[1];
      expect(details.classList.contains('leaderboard-mobile-details')).toBe(true);
      const children = Array.from(details.children);
      const statRows = children.filter(el => el.classList.contains('leaderboard-mobile-stat'));
      expect(statRows).toHaveLength(4);
      for (const row of statRows) {
        const bars = Array.from(row.children);
        expect(bars).toHaveLength(2);
        for (const bar of bars) {
          expect(bar.classList.contains('skeleton-bar')).toBe(true);
        }
      }
      // Button-shaped bar closes the block: full width, rectangular radius.
      const buttonBar = children[children.length - 1];
      expect(buttonBar.classList.contains('skeleton-bar')).toBe(true);
      const buttonStyle = buttonBar.getAttribute('style') ?? '';
      expect(decl(buttonStyle, 'width')).toBe('100%');
      expect(decl(buttonStyle, 'border-radius')).toBe('var(--radius-md)');
    }
  });

  it('keeps the mobile placeholders decorative and shimmer-only', () => {
    const container = mobileContainer();
    // The container carries the live list role (renderLeaderboard's
    // #lb-mobile markup in pages/leaderboard.ts); the cards inside stay plain
    // divs — a skeleton announces nothing, unlike the live role="listitem"s.
    expect(container.getAttribute('role')).toBe('list');
    expect(container.innerHTML).not.toContain('<button');
    expect(container.innerHTML).not.toContain('<a ');
    expect(container.innerHTML).not.toContain('data-bot-id');
    expect(container.innerHTML).not.toContain('role="listitem"');
    expect(container.innerHTML).not.toMatch(/animation\s*:/);
  });
});

// ─── Responsive visibility parity ─────────────────────────────────────────────────
// jsdom applies no stylesheets, so these checks evaluate the real media queries
// off disk: rules are parsed out of the stylesheet text and asked which `display`
// a class ends up with at a viewport width. The skeleton must be governed by
// exactly those rules — it introduces no breakpoint and no display rule of its
// own, so it swaps shape where the live page swaps (renderLeaderboard uses the
// same #lb-desktop / #lb-mobile.mobile-cards containers).

interface CssRule {
  selector: string;
  decls: string;
  media: string | null;
}

/** Flat rule list parsed from stylesheet text (comments stripped, grouped
 *  selector lists split, @media conditions preserved). These stylesheets never
 *  nest @media; other at-rules (@keyframes, …) are skipped wholesale. */
function parseCssRules(css: string): CssRule[] {
  const rules: CssRule[] = [];
  const text = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const visit = (src: string, media: string | null): void => {
    let i = 0;
    while (i < src.length) {
      const open = src.indexOf('{', i);
      if (open === -1) break;
      const header = src.slice(i, open).trim();
      let close = open;
      let depth = 1;
      while (++close < src.length && depth > 0) {
        if (src[close] === '{') depth++;
        else if (src[close] === '}') depth--;
      }
      const body = src.slice(open + 1, close - 1);
      if (header.startsWith('@media')) {
        visit(body, header.replace(/^@media\s*/i, ''));
      } else if (!header.startsWith('@')) {
        for (const selector of header.split(',')) {
          rules.push({ selector: selector.trim(), decls: body, media });
        }
      }
      i = close;
    }
  };
  visit(text, null);
  return rules;
}

/** Does a media condition hold at this viewport width? Width features decide;
 *  any other feature (prefers-*, hover, …) makes the block a no-op here, which
 *  matches a width-only evaluation. */
function matchesWidth(condition: string | null, width: number): boolean {
  if (condition === null) return true;
  return condition.split(/\band\b/i).every(feature => {
    const m = feature.match(/\(\s*(min-width|max-width)\s*:\s*([\d.]+)px\s*\)/);
    if (!m) return false;
    const px = parseFloat(m[2]);
    return m[1] === 'min-width' ? width >= px : width <= px;
  });
}

/** Effective `display` for a bare class selector at a viewport width, last
 *  matching declaration winning as in the cascade; null when none applies. */
function displayAtWidth(rules: CssRule[], className: string, width: number): string | null {
  let display: string | null = null;
  for (const rule of rules) {
    if (rule.selector !== `.${className}`) continue;
    if (!matchesWidth(rule.media, width)) continue;
    const m = rule.decls.match(/(?:^|[;{])\s*display\s*:\s*([^;]*)/);
    if (m) display = m[1].replace(/!important/i, '').trim();
  }
  return display;
}

const cssRules = [...parseCssRules(componentsCss), ...parseCssRules(mobileCss)];

const ROW_COUNT = 15;
const CARD_COUNT = 8;
const BAR_COUNT = 7;

describe('skeletonLeaderboard responsive visibility', () => {
  it('hides the wl and status bars through the live .lb-wl / .lb-status rule', () => {
    // The rule the real row depends on (styles/components.css, max-width: 768px).
    for (const width of [375, 768]) {
      expect(displayAtWidth(cssRules, 'lb-wl', width)).toBe('none');
      expect(displayAtWidth(cssRules, 'lb-status', width)).toBe('none');
    }
    for (const width of [769, 1280]) {
      expect(displayAtWidth(cssRules, 'lb-wl', width)).toBeNull();
      expect(displayAtWidth(cssRules, 'lb-status', width)).toBeNull();
    }
    // …and it reaches the skeleton because the bars carry the live classes in
    // the live column order (wl 4th, status 6th of the seven bars).
    for (const row of Array.from(desktopContainer().children)) {
      expect(row.children[3].classList.contains('lb-wl')).toBe(true);
      expect(row.children[5].classList.contains('lb-status')).toBe(true);
      for (const bar of Array.from(row.children)) {
        expect(['skeleton-bar', 'skeleton-bar lb-wl', 'skeleton-bar lb-status'])
          .toContain(bar.className);
      }
    }
  });

  it('swaps the mobile card container at the live breakpoints', () => {
    expect(displayAtWidth(cssRules, 'mobile-cards', 375)).toBe('flex');
    expect(displayAtWidth(cssRules, 'mobile-cards', 639)).toBe('flex');
    expect(displayAtWidth(cssRules, 'mobile-cards', 640)).toBe('none');
    expect(displayAtWidth(cssRules, 'mobile-cards', 1023)).toBe('none');
    expect(displayAtWidth(cssRules, 'mobile-cards', 1024)).toBe('none');
    expect(displayAtWidth(cssRules, 'mobile-cards', 1280)).toBe('none');
    // The wide-viewport hide is the live desktop block's !important one — the
    // same rule that beats everything else on the real page.
    expect(cssRules.some(rule =>
      rule.selector === '.mobile-cards' &&
      rule.media === '(min-width: 1024px)' &&
      /display\s*:\s*none\s*!important\s*;?\s*$/.test(rule.decls.trim())
    )).toBe(true);
    // Cards themselves are hidden only through the >=1024px block.
    expect(displayAtWidth(cssRules, 'leaderboard-mobile-card', 375)).toBe('flex');
    expect(displayAtWidth(cssRules, 'leaderboard-mobile-card', 1024)).toBe('none');
  });

  it('takes the live page shape at 375px and 1280px', () => {
    const doc = new DOMParser().parseFromString(skeletonLeaderboard(), 'text/html');
    const rows = doc.querySelectorAll('#lb-desktop .lb-row');
    const cards = doc.querySelectorAll('#lb-mobile .leaderboard-mobile-card');
    expect(rows.length).toBe(ROW_COUNT);
    expect(cards.length).toBe(CARD_COUNT);

    // Node counts per viewport, derived only from the shared stylesheet: a
    // hidden .mobile-cards hides its cards, and the collapsed wl/status
    // columns shrink every desktop row the way the real .lb-row shrinks.
    const shapeAt = (width: number) => {
      const wlShown = displayAtWidth(cssRules, 'lb-wl', width) !== 'none';
      const statusShown = displayAtWidth(cssRules, 'lb-status', width) !== 'none';
      const cardsShown =
        displayAtWidth(cssRules, 'mobile-cards', width) !== 'none' &&
        displayAtWidth(cssRules, 'leaderboard-mobile-card', width) !== 'none';
      return {
        visibleCards: cardsShown ? cards.length : 0,
        visibleWlBars: wlShown ? rows.length : 0,
        visibleStatusBars: statusShown ? rows.length : 0,
        barsPerRow: BAR_COUNT - (wlShown ? 0 : 1) - (statusShown ? 0 : 1),
      };
    };

    // Phone: cards show and the wl/status columns are collapsed (the live page
    // keeps #lb-desktop itself rendered at every width — no rule hides it — so
    // the rows stay put with 5 of 7 bars, exactly like the real rows).
    expect(shapeAt(375)).toEqual({
      visibleCards: CARD_COUNT,
      visibleWlBars: 0,
      visibleStatusBars: 0,
      barsPerRow: 5,
    });
    // 768px boundary: wl/status still collapsed, cards already hidden by the
    // tablet block (640–1023px).
    expect(shapeAt(768)).toEqual({
      visibleCards: 0,
      visibleWlBars: 0,
      visibleStatusBars: 0,
      barsPerRow: 5,
    });
    // Desktop: only desktop rows, every column back.
    expect(shapeAt(1280)).toEqual({
      visibleCards: 0,
      visibleWlBars: ROW_COUNT,
      visibleStatusBars: ROW_COUNT,
      barsPerRow: BAR_COUNT,
    });
  });

  it('adds no visibility rules of its own', () => {
    const html = skeletonLeaderboard();
    expect(html).not.toContain('<style');
    // The containers are the live page's own (renderLeaderboard markup in
    // pages/leaderboard.ts), so the stylesheet's media queries reach the
    // skeleton through the same selectors — no skeleton-only class and no
    // inline display that could override a breakpoint.
    const doc = new DOMParser().parseFromString(html, 'text/html');
    expect(doc.getElementById('lb-desktop')!.className).toBe('');
    expect(doc.getElementById('lb-mobile')!.className).toBe('mobile-cards');
    expect(doc.getElementById('lb-desktop')!.getAttribute('style')).toBeNull();
    expect(doc.getElementById('lb-mobile')!.getAttribute('style')).toBeNull();
    for (const row of Array.from(doc.querySelectorAll('#lb-desktop .lb-row'))) {
      expect(row.getAttribute('style')).toBeNull();
    }
  });
});

// ─── skeletonBotProfile ───────────────────────────────────────────────────────────
// renderBotProfilePage + renderProfile (pages/bot-profile.ts) is the shape
// being mirrored: a nav.breadcrumb, then a .profile-header (§16.14 avatar-area
// circle + .profile-header-main of name h1 + .profile-status chip, plus the
// share-card button), then a .profile-grid holding the ratings, stats, meta,
// rivals and lazy history sections in that order.

describe('skeletonBotProfile mirrors renderProfile', () => {
  // Rules the skeleton's geometry must come from, read off disk like the
  // leaderboard rules above.
  const pageRule = componentsCss.match(/\.bot-profile-page\s*\{([^}]*)\}/)?.[1] ?? '';
  const skeletonPageRule = componentsCss.match(/\.skeleton-page\s*\{([^}]*)\}/)?.[1] ?? '';
  const headerRule = componentsCss.match(/\.profile-header\s*\{([^}]*)\}/)?.[1] ?? '';
  const headerMainRule = componentsCss.match(/\.profile-header-main\s*\{([^}]*)\}/)?.[1] ?? '';
  const gridRule = componentsCss.match(/\.profile-grid\s*\{([^}]*)\}/)?.[1] ?? '';
  const sectionRule = componentsCss.match(/\.profile-section\s*\{([^}]*)\}/)?.[1] ?? '';
  const toggleRule = componentsCss.match(/\.section-toggle\s*\{([^}]*)\}/)?.[1] ?? '';
  const contentRule = componentsCss.match(/\.section-content\s*\{([^}]*)\}/)?.[1] ?? '';
  const displayRule = componentsCss.match(/\.rating-display\s*\{([^}]*)\}/)?.[1] ?? '';
  const rangeRule = componentsCss.match(/\.rating-range\s*\{([^}]*)\}/)?.[1] ?? '';
  const statsGridRule = componentsCss.match(/\.stats-grid\s*\{([^}]*)\}/)?.[1] ?? '';
  const statRule = componentsCss.match(/\.stat\s*\{([^}]*)\}/)?.[1] ?? '';
  const lazyPlaceholderRule = componentsCss.match(/\.lazy-placeholder\s*\{([^}]*)\}/)?.[1] ?? '';
  // The live rules the stand-in bars re-declare, plus the shared
  // .skeleton-profile-* rules that re-declare them (components.css, next to
  // .skeleton-page/.skeleton-row). The h1 stand-in cannot carry the live
  // `.profile-header h1` rule or the heading bar the base `h1, h2, …` group,
  // and the chip bar cannot carry .profile-status wholesale — its
  // display:inline-block would reflow the stand-in — so each takes a skeleton
  // class re-declaring that one quantity from the same custom property.
  const headerH1Rule = componentsCss.match(/(?:^|\})\s*\.profile-header h1\s*\{([^}]*)\}/)?.[1] ?? '';
  const statusRule = componentsCss.match(/(?:^|\})\s*\.profile-status\s*\{([^}]*)\}/)?.[1] ?? '';
  const breadcrumbRule = componentsCss.match(/\.breadcrumb\s*\{([^}]*)\}/)?.[1] ?? '';
  // The base heading group is the only `h1, h2` selector in base.css, and the
  // base tap-target group is the only `button, a` selector there.
  const headingBaseRule = baseCss.match(/h1,\s*h2[^{]*\{([^}]*)\}/)?.[1] ?? '';
  const h2BaseRule = baseCss.match(/(?:^|\n)h2\s*\{([^}]*)\}/)?.[1] ?? '';
  const tapTargetRule = baseCss.match(/button,\s*a[^{]*\{([^}]*)\}/)?.[1] ?? '';
  const skeletonNameRule = componentsCss.match(/\.skeleton-profile-name\s*\{([^}]*)\}/)?.[1] ?? '';
  const skeletonHeadingRule = componentsCss.match(/\.skeleton-profile-heading\s*\{([^}]*)\}/)?.[1] ?? '';
  const skeletonChipRule = componentsCss.match(/\.skeleton-profile-chip\s*\{([^}]*)\}/)?.[1] ?? '';
  const skeletonBtnRule = componentsCss.match(/\.skeleton-profile-btn\s*\{([^}]*)\}/)?.[1] ?? '';
  // Phone re-declarations live in mobile.css's (max-width: 639px) block, where
  // the live rules they mirror (h2 1.25rem, .btn min-height 48px) also live.
  const phoneBlockStart = mobileCss.indexOf('@media (max-width: 639px)');
  const phoneHeadingRule = mobileCss.match(/\.skeleton-profile-heading\s*\{([^}]*)\}/)?.[1] ?? '';
  const phoneHeadingInPhoneBlock = phoneHeadingRule !== '' && mobileCss.indexOf('.skeleton-profile-heading') > phoneBlockStart;
  const phoneBtnRule = mobileCss.match(/\.skeleton-profile-btn\s*\{([^}]*)\}/)?.[1] ?? '';
  const phoneBtnInPhoneBlock = phoneBtnRule !== '' && mobileCss.indexOf('.skeleton-profile-btn') > phoneBlockStart;
  const phoneH2Rule = mobileCss.match(/^\s{2}h2\s*\{([^}]*)\}/m)?.[1] ?? '';
  const phoneBtnFloorRule = mobileCss.match(/\.btn\s*\{([^}]*)\}/)?.[1] ?? '';

  function profileDoc(): Document {
    return new DOMParser().parseFromString(skeletonBotProfile(), 'text/html');
  }

  function profileRoot(): HTMLElement {
    const el = profileDoc().querySelector('.bot-profile-page');
    expect(el, 'skeleton must render inside the live .bot-profile-page container').toBeTruthy();
    return el!;
  }

  /** The .profile-grid, addressed by class rather than by child index — the
   * breadcrumb row ahead of it is part of the mirrored structure, not a
   * reason for every test here to count children from the top. */
  function profileGrid(): HTMLElement {
    const grid = Array.from(profileRoot().children).find(el => el.className === 'profile-grid');
    expect(grid, 'skeleton must render the .profile-grid after the breadcrumb and header').toBeTruthy();
    return grid!;
  }

  function barOf(el: Element): HTMLElement {
    expect(el.classList.contains('skeleton-bar'), `expected a skeleton bar, got <${el.tagName} class="${el.className}">`).toBe(true);
    return el as HTMLElement;
  }

  it('keeps the .skeleton-page root around the live .bot-profile-page', () => {
    expect(pageRule, '.bot-profile-page rule not found in components.css').not.toBe('');
    expect(decl(pageRule, 'max-width')).toBe('900px');
    expect(decl(pageRule, 'padding')).toBe('var(--space-lg)');
    const doc = profileDoc();
    const root = doc.body.children[0];
    expect(root.className).toBe('skeleton-page');
    expect(root.getAttribute('style')).toBeNull();

    // The wrapper must be layout-transparent so the swap shifts nothing: the
    // live page replaces it with a bare .bot-profile-page, and that page lands
    // at the same width and offset only if .skeleton-page adds no padding and
    // nothing else. (Its max-width + auto margins are inert below 1200px and
    // centre the same column above it.)
    expect(skeletonPageRule, '.skeleton-page rule not found in components.css').not.toBe('');
    expect(skeletonPageRule).not.toContain('padding');
    expect(skeletonPageRule).not.toContain('border');

    const page = root.children[0];
    expect(page.className).toBe('bot-profile-page');
    expect(page.getAttribute('style')).toBeNull();
    expect(root.children).toHaveLength(1);
  });

  it('holds the breadcrumb row the swap renders above the content', () => {
    // The swap replaces app.innerHTML with .bot-profile-page > nav.breadcrumb
    // + the content block, so the skeleton must hold the row's space: the live
    // nav is 44px tall (its <a> picks up the base tap-target floor) plus the
    // live .breadcrumb rule's margin — dropping it let the real row appear on
    // the swap and shove the entire page down by 68px.
    expect(breadcrumbRule, '.breadcrumb rule not found in components.css').not.toBe('');
    expect(decl(breadcrumbRule, 'display')).toBe('flex');
    expect(decl(breadcrumbRule, 'margin-bottom')).toBe('var(--space-lg)');
    expect(decl(tapTargetRule, 'min-height')).toBe('44px');

    const nav = profileRoot().children[0];
    expect(nav.tagName).toBe('NAV');
    expect(nav.className).toBe('breadcrumb');
    expect(nav.getAttribute('style')).toBeNull();

    const bars = Array.from(nav.children).map(barOf);
    expect(bars).toHaveLength(1);
    expect(decl(bars[0].getAttribute('style') ?? '', 'height')).toBe('44px');
    // The row's height comes from the floor, not from a text metric, so the
    // bar carries no skeleton class of its own — nothing here to re-declare.
    expect(bars[0].className).toBe('skeleton-bar');
  });

  it('renders the real header: name, status chip, share-card button — no avatar', () => {
    expect(headerRule, '.profile-header rule not found in components.css').not.toBe('');
    expect(decl(headerRule, 'justify-content')).toBe('space-between');
    expect(decl(headerRule, 'margin-bottom')).toBe('var(--space-lg)');
    expect(headerMainRule, '.profile-header-main rule not found in components.css').not.toBe('');
    expect(decl(headerMainRule, 'flex')).toBe('1');

    // Second child: the breadcrumb row the swap renders first holds its space
    // ahead of the header (see the breadcrumb test above).
    const header = profileRoot().children[1];
    expect(header.className).toBe('profile-header');
    expect(header.getAttribute('style')).toBeNull();
    expect(header.children).toHaveLength(2);

    // The live header renders no avatar and neither does the skeleton.
    expect(profileDoc().querySelector('.skeleton-circle')).toBeNull();
    expect(skeletonBotProfile()).not.toContain('skeleton-circle');

    const [main, button] = Array.from(header.children);
    expect(main.className).toBe('profile-header-main');
    const headerBars = Array.from(main.children).map(barOf);
    expect(headerBars).toHaveLength(2);
    // Name bar above the status chip bar, both via the shared Skeleton output.
    expect(parseFloat(decl(headerBars[1].getAttribute('style') ?? '', 'height'))).toBeLessThan(
      parseFloat(decl(headerBars[0].getAttribute('style') ?? '', 'height'))
    );
    // The name bar takes its margin from the shared class, which must keep
    // re-declaring exactly what the live `.profile-header h1` rule declares.
    expect(skeletonNameRule, '.skeleton-profile-name rule not found in components.css').not.toBe('');
    expect(headerBars[0].className).toBe('skeleton-bar skeleton-profile-name');
    // The live rule declares the h1's margin as a shorthand (0 0 … 0); its
    // bottom component is what the shared class re-declares.
    expect(decl(skeletonNameRule, 'margin-bottom')).toBe(decl(headerH1Rule, 'margin').split(/\s+/)[2]);
    expect(headerBars[0].getAttribute('style'), 'the h1 margin comes from the shared class').not.toContain('margin');
    // The chip is rounded like .profile-status — the shared class re-declares
    // the radius from the same custom property the live rule uses.
    expect(skeletonChipRule, '.skeleton-profile-chip rule not found in components.css').not.toBe('');
    expect(headerBars[1].className).toBe('skeleton-bar skeleton-profile-chip');
    expect(decl(skeletonChipRule, 'border-radius')).toBe(decl(statusRule, 'border-radius'));
    expect(headerBars[1].getAttribute('style'), 'the chip radius comes from the shared class').not.toContain('border-radius');
    // The share-card stand-in is a rectangle, rounded like .btn.
    expect(barOf(button).className).toBe('skeleton-bar skeleton-profile-btn');
    expect(decl(button.getAttribute('style') ?? '', 'border-radius')).toBe('var(--radius-md)');
    // .btn's height is the base tap-target floor, not its text: the button's
    // own content (0.875rem × 1.5 line-height + 2 × var(--space-sm) padding)
    // is 37px and sits below the 44px floor, so the stand-in's height is the
    // floor — owned by the shared class, because mobile.css raises the floor
    // to 48px on phone and the inline declaration could not follow it.
    const btnRule = componentsCss.match(/\.btn\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(decl(btnRule, 'padding')).toBe('var(--space-sm) var(--space-md)');
    expect(decl(btnRule, 'font-size')).toBe('0.875rem');
    expect(skeletonBtnRule, '.skeleton-profile-btn rule not found in components.css').not.toBe('');
    expect(decl(skeletonBtnRule, 'height')).toBe(decl(tapTargetRule, 'min-height'));
    expect(button.getAttribute('style'), 'the button height comes from the shared class').not.toContain('height');
    expect(phoneBtnInPhoneBlock, '.skeleton-profile-btn must be re-declared inside mobile.css phone block').toBe(true);
    expect(decl(phoneBtnRule, 'height')).toBe(decl(phoneBtnFloorRule, 'min-height'));
  });

  it('takes no spacing of its own: the shared .skeleton-profile-* rules own the margins', () => {
    // The bars carry inline only the dimensions of the text they stand in
    // for; every margin comes from a shared components.css class, and the
    // class-based assertions above hold each of those rules against the live
    // rule it re-declares. (The history placeholder's inline min-height
    // mirrors the live page's own inline placeholder string — the caller
    // passes it to lazySection — so it is not a skeleton bar, and the
    // lazy-placeholder test pins it verbatim.)
    const doc = profileDoc();
    const bars = doc.querySelectorAll('.skeleton-bar, .skeleton-circle');
    expect(bars.length).toBeGreaterThan(0);
    for (const bar of Array.from(bars)) {
      expect(bar.getAttribute('style') ?? '', `no inline margin/gap on ${bar.className}`)
        .not.toMatch(/(?:^|;)\s*(?:margin|gap)[a-z-]*\s*:/);
    }
  });

  it('reuses the shared shimmer rules and declares none of its own', () => {
    // Every bar reaches the base .skeleton-bar rule (components.css) through
    // the shared Skeleton() output class alone — the same rule the leaderboard
    // skeleton uses — and the history block reuses the live .lazy-placeholder
    // rule. Nothing here copies the animation inline.
    const shimmerRule = componentsCss.match(/\.skeleton-bar,\s*\.skeleton-circle,\s*\.skeleton-canvas\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(shimmerRule, 'shared skeleton shimmer rule not found in components.css').not.toBe('');
    expect(shimmerRule).toContain('skeleton-shimmer 1.5s');
    expect(skeletonBotProfile()).not.toMatch(/animation\s*:/);
    expect(skeletonBotProfile()).not.toContain('<style');
  });

  it('orders the grid sections ratings, stats, meta, rivals, lazy history', () => {
    expect(gridRule, '.profile-grid rule not found in components.css').not.toBe('');
    expect(decl(gridRule, 'grid-template-columns')).toBe('repeat(auto-fit, minmax(280px, 1fr))');
    const grid = profileGrid();
    expect(grid.className).toBe('profile-grid');
    const shapes = Array.from(grid.children).map(el => el.className);
    expect(shapes).toEqual([
      'profile-section ratings',
      'profile-section stats expandable-section',
      'profile-section meta expandable-section',
      'profile-section rivals expandable-section',
      'lazy-section',
    ]);
    // Section chrome (background/border/radius) comes from the shared rule.
    expect(sectionRule, '.profile-section rule not found in components.css').not.toBe('');
    expect(decl(sectionRule, 'background-color')).toBe('var(--bg-secondary)');
    for (const section of Array.from(grid.children).slice(0, 4)) {
      expect(section.classList.contains('profile-section')).toBe(true);
      expect(section.getAttribute('style')).toBeNull();
    }
  });

  it('stands in for the rating block: heading, main/dev pair, chart with range row', () => {
    expect(displayRule, '.rating-display rule not found in components.css').not.toBe('');
    expect(decl(displayRule, 'margin-bottom')).toBe('var(--space-md)');
    const ratings = profileGrid().children[0];

    // First child is the <h2>Rating</h2> stand-in: its height is the text
    // metric it stands in for and its margin the base heading rule's — both
    // through the shared class, which must keep re-declaring exactly what the
    // base h1..h6 rule declares, since a div cannot carry that rule. The
    // height lives in the class rather than inline because mobile.css shrinks
    // the bare h2 on phone, and an inline declaration could not follow it.
    const heading = barOf(ratings.children[0]);
    expect(heading.className).toBe('skeleton-bar skeleton-profile-heading');
    expect(skeletonHeadingRule, '.skeleton-profile-heading rule not found in components.css').not.toBe('');
    // 1.5rem (base h2) × 1.25 line-height = 30px on tablet and up …
    expect(decl(h2BaseRule, 'font-size')).toBe('1.5rem');
    expect(decl(skeletonHeadingRule, 'height')).toBe('30px');
    expect(decl(skeletonHeadingRule, 'margin-bottom')).toBe(decl(headingBaseRule, 'margin-bottom'));
    expect(heading.getAttribute('style'), 'the h2 height and margin come from the shared class').not.toMatch(/(?:^|;)\s*(?:height|margin)/);
    // … and 1.25rem × 1.25 = 25px on phone, where mobile.css re-declares the
    // stand-in's height from the same rule the live h2 answers to.
    expect(phoneHeadingInPhoneBlock, '.skeleton-profile-heading must be re-declared inside mobile.css phone block').toBe(true);
    expect(decl(phoneH2Rule, 'font-size')).toBe('1.25rem');
    expect(decl(phoneHeadingRule, 'height')).toBe('25px');

    // .rating-display main/dev bars, the main one taller (2.5rem vs 1rem text).
    const display = ratings.querySelector('.rating-display');
    expect(display).toBeTruthy();
    const [main, deviation] = Array.from(display!.children).map(barOf);
    expect(parseFloat(decl(deviation.getAttribute('style') ?? '', 'height'))).toBeLessThan(
      parseFloat(decl(main.getAttribute('style') ?? '', 'height'))
    );

    // .rating-chart holds the sparkline rectangle and the min/max range row.
    const chart = ratings.querySelector('.rating-chart');
    expect(chart).toBeTruthy();
    const [sparkline, range] = Array.from(chart!.children);
    expect(barOf(sparkline).className).toBe('skeleton-bar');
    expect(decl(sparkline.getAttribute('style') ?? '', 'border-radius')).toBe('var(--radius-md)');
    expect(range.className).toBe('rating-range');
    expect(decl(rangeRule, 'justify-content')).toBe('space-between');
    expect(range.querySelectorAll('.skeleton-bar')).toHaveLength(2);
  });

  it('renders the stats section expanded with four .stat cells', () => {
    expect(toggleRule, '.section-toggle rule not found in components.css').not.toBe('');
    expect(decl(toggleRule, 'padding')).toBe('var(--space-md)');
    const stats = profileGrid().children[1];

    const [toggle, content] = Array.from(stats.children);
    expect(toggle.className).toBe('section-toggle');
    expect(toggle.tagName).toBe('DIV');
    const toggleBars = Array.from(toggle.children).map(barOf);
    expect(toggleBars).toHaveLength(2);
    expect(parseFloat(decl(toggleBars[1].getAttribute('style') ?? '', 'height'))).toBeLessThan(
      parseFloat(decl(toggleBars[0].getAttribute('style') ?? '', 'height'))
    );

    expect(content.className).toBe('section-content expanded');
    const statsGrid = content.children[0];
    expect(statsGrid.className).toBe('stats-grid');
    expect(decl(statsGridRule, 'grid-template-columns')).toBe('repeat(4, 1fr)');
    // …and the skeleton must resolve from that live rule, never duplicate it:
    // the element carries no inline grid, so the shared declaration governs.
    expect(statsGrid.getAttribute('style'), 'stats grid must not duplicate the shared rule inline').toBeNull();
    expect(skeletonBotProfile()).not.toMatch(/style="[^"]*grid-template-columns/);
    expect(statRule, '.stat rule not found in components.css').not.toBe('');
    expect(decl(statRule, 'flex-direction')).toBe('column');

    const cells = statsGrid.querySelectorAll('.stat');
    expect(cells).toHaveLength(4);
    for (const cell of cells) {
      expect(cell.getAttribute('style')).toBeNull();
      const [value, label] = Array.from(cell.children).map(barOf);
      expect(parseFloat(decl(label.getAttribute('style') ?? '', 'height'))).toBeLessThan(
        parseFloat(decl(value.getAttribute('style') ?? '', 'height'))
      );
    }
  });

  it('leaves meta and rivals collapsed through the shared .section-content rule', () => {
    expect(decl(contentRule, 'max-height')).toBe('0');
    expect(decl(contentRule, 'overflow')).toBe('hidden');
    const grid = profileGrid();
    for (const name of ['meta', 'rivals']) {
      const section = grid.querySelector(`.profile-section.${name}`);
      expect(section, `${name} section missing`).toBeTruthy();
      const content = section!.querySelector('.section-content');
      expect(content, `${name} must keep its collapsed content box`).toBeTruthy();
      expect(content!.classList.contains('expanded')).toBe(false);
      expect(content!.children).toHaveLength(0);
    }
  });

  it('shows the history section as the live lazy placeholder', () => {
    expect(lazyPlaceholderRule, '.lazy-placeholder rule not found in components.css').not.toBe('');
    // The shimmer comes from the shared .lazy-placeholder rule — the same one
    // lazySection renders below the fold — never from inline animation CSS.
    expect(lazyPlaceholderRule).toContain('skeleton-shimmer');

    const lazy = profileGrid().children[4];
    expect(lazy.className).toBe('lazy-section');
    const placeholder = lazy.children[0];
    expect(placeholder.className).toBe('lazy-placeholder');
    expect(decl(placeholder.getAttribute('style') ?? '', 'min-height')).toBe('80px');
    expect(placeholder.getAttribute('style')).toBe('min-height:80px');
    expect(placeholder.querySelectorAll('.skeleton-bar')).toHaveLength(0);
  });

  it('stays decorative: no text, no interactive elements, no CSS of its own', () => {
    const doc = profileDoc();
    // Nothing is readable either — every placeholder is an empty shimmer bar,
    // so the load never flashes content that looks real but means nothing.
    expect(doc.body.textContent?.trim() ?? '', 'the skeleton must carry no text content').toBe('');
    // No interactive elements anywhere — the share-card button and the section
    // toggles all stand in as plain divs.
    expect(doc.querySelectorAll('button, a, input, select, textarea')).toHaveLength(0);
    expect(skeletonBotProfile()).not.toContain('<style');
    expect(skeletonBotProfile()).not.toMatch(/animation\s*:/);
    const hooks = doc.querySelectorAll('[data-section], [aria-expanded], [id]');
    expect(hooks, 'skeleton must not carry live-page hooks (data-section/aria/id)').toHaveLength(0);
    // The phone layout (single-column grid, 2-up stats) is reached only through
    // the live classes — the mobile rules must be there for the skeleton to
    // inherit, and the skeleton must carry no inline display that would beat them.
    const mobileGrid = mobileCss.match(/\.profile-grid\s*\{([^}]*)\}/)?.[1] ?? '';
    const mobileStats = mobileCss.match(/\.stats-grid\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(mobileGrid).toContain('grid-template-columns: 1fr');
    expect(mobileStats).toContain('repeat(2, 1fr)');
    for (const wrapper of doc.querySelectorAll('.skeleton-page, .bot-profile-page, .profile-header, .profile-header-main, .profile-grid, .profile-section, .rating-display, .rating-chart, .rating-range, .stats-grid, .stat, .section-toggle, .section-content, .lazy-section')) {
      const style = wrapper.getAttribute('style');
      expect(style === null || style === 'min-height:80px', `no inline layout on ${wrapper.className}`).toBe(true);
    }
  });
});

// ─── skeletonReplay ─────────────────────────────────────────────────────────────
// initReplayViewerWithClass's markup template (web/src/pages/replay.ts) is the
// shape being mirrored: .replay-page opens with the h1.page-title, then
// .replay-layout holds .replay-main (the .canvas-wrapper of canvas stand-in +
// #no-replay bar, then .mobile-replay-controls of .mobile-playback-bar +
// scrubber, then .mobile-event-timeline) and .replay-sidebar last. Every
// wrapper reuses the live class, so the whole layout — the flex row, the main
// column's flex:1, the sidebar widths, the 900px stacking, the canvas box and
// the title margin — comes from the stylesheets (components.css + mobile.css),
// which are also the only cascade the skeleton is ever laid out under: the
// skeleton renders *before* the live page's own <style> blocks exist, so a
// rule for a class it renders cannot live in them. The last guard here holds
// the page's blocks to that.

describe('skeletonReplay mirrors replayPageMarkup', () => {
  // The replay page's rules live partly in the stylesheets and partly in its
  // own inline <style> block, so both are read off disk: replay.ts is the
  // source of truth for the template's markup and for the rules declared only
  // there (.no-replay-message, the scrubber input's inline margin, the
  // timeline placeholder span's inline style).
  const replaySource = readFileSync(
    resolve(stylesDir, '../pages/replay.ts'), 'utf8'
  );

  const skeletonPageRule = componentsCss.match(/\.skeleton-page\s*\{([^}]*)\}/)?.[1] ?? '';
  const replayPageRule = componentsCss.match(/\.replay-page\s*\{([^}]*)\}/)?.[1] ?? '';
  const layoutRule = componentsCss.match(/\.replay-layout\s*\{([^}]*)\}/)?.[1] ?? '';
  const mainRule = componentsCss.match(/\.replay-main\s*\{([^}]*)\}/)?.[1] ?? '';
  const sidebarRule = componentsCss.match(/\.replay-sidebar\s*\{([^}]*)\}/)?.[1] ?? '';
  const canvasRule = componentsCss.match(/\.canvas-wrapper canvas\s*\{([^}]*)\}/)?.[1] ?? '';
  const noReplayRule = replaySource.match(/\.no-replay-message\s*\{([^}]*)\}/)?.[1] ?? '';
  // The replay-scoped boxes the page block gave up: the title margin and the
  // canvas wrapper, both declared against .replay-page in components.css so
  // the skeleton answers to them too, and the 900px stacking block in
  // mobile.css — the one cascade both states of the page are laid out by.
  const replayPageTitleRule =
    componentsCss.match(/\.replay-page \.page-title\s*\{([^}]*)\}/)?.[1] ?? '';
  const replayPageCanvasRule =
    componentsCss.match(/\.replay-page \.canvas-wrapper\s*\{([^}]*)\}/)?.[1] ?? '';
  const replayStackingBlock =
    mobileCss.match(/@media \(max-width: 900px\)\s*\{([\s\S]*?)\n\}/)?.[1] ?? '';
  // Phone-block re-declarations (mobile.css, max-width: 639px): the title size
  // is scoped to .replay-page — the reason the skeleton keeps that wrapper —
  // and the .btn floor is what sets the playback controls' height.
  const phoneTitleRule = mobileCss.match(/\.replay-page \.page-title\s*\{([^}]*)\}/)?.[1] ?? '';
  const playbackBtnRule = mobileCss.match(/\.mobile-playback-bar \.btn\s*\{([^}]*)\}/)?.[1] ?? '';
  const speedDisplayRule = mobileCss.match(/\.mobile-speed-display\s*\{([^}]*)\}/)?.[1] ?? '';
  const htmlRule = baseCss.match(/(?:^|\})\s*html\s*\{([^}]*)\}/)?.[1] ?? '';
  const bodyRule = baseCss.match(/(?:^|\})\s*body\s*\{([^}]*)\}/)?.[1] ?? '';
  const rootVars = baseCss.match(/:root\s*\{([^}]*)\}/)?.[1] ?? '';
  // The base tap-target group is the only `button, a` selector in base.css.
  const tapTargetRule = baseCss.match(/button,\s*a[^{]*\{([^}]*)\}/)?.[1] ?? '';
  const shimmerRule = componentsCss.match(/\.skeleton-bar,\s*\.skeleton-circle,\s*\.skeleton-canvas\s*\{([^}]*)\}/)?.[1] ?? '';

  const rootPx = parseFloat(decl(htmlRule, 'font-size'));
  const textLineHeight = parseFloat(decl(bodyRule, 'line-height'));
  const spaceXs = parseFloat(decl(rootVars, '--space-xs'));

  // The live timeline placeholder's inline style string, straight out of the
  // template — it is the only thing sizing that span.
  const timelineSpanStyle =
    replaySource.match(/mobile-event-timeline[^>]*>\s*<span style="([^"]*)"/)?.[1] ?? '';

  function one(selector: string, message: string): HTMLElement {
    const el = new DOMParser().parseFromString(skeletonReplay(), 'text/html').querySelector(selector);
    expect(el, message).toBeTruthy();
    return el as HTMLElement;
  }

  it('keeps the .skeleton-page root around the live .replay-page', () => {
    const doc = new DOMParser().parseFromString(skeletonReplay(), 'text/html');
    const root = doc.body.children[0];
    expect(root.className).toBe('skeleton-page');
    expect(root.getAttribute('style')).toBeNull();

    // Layout-transparent as on every page skeleton — no padding, nothing the
    // swap would shift: the live page replaces it with a bare .replay-page,
    // and both wrappers cap at the same shared 1200px, so the nesting cannot
    // narrow the column the live page lands in.
    expect(skeletonPageRule, '.skeleton-page rule not found in components.css').not.toBe('');
    expect(skeletonPageRule).not.toContain('padding');
    expect(skeletonPageRule).not.toContain('border');
    expect(replayPageRule, '.replay-page rule not found in components.css').not.toBe('');
    expect(decl(replayPageRule, 'max-width')).toBe(decl(skeletonPageRule, 'max-width'));

    const page = root.children[0];
    expect(page.className).toBe('replay-page');
    expect(page.getAttribute('style')).toBeNull();
    expect(root.children).toHaveLength(1);
  });

  it('renders the live title inside .replay-page, where its phone rule can reach it', () => {
    // The phone block re-declares the title size against .replay-page
    // .page-title — a bare .page-title keeps the base size on phone, so the
    // nested wrapper the live template renders is the only thing that carries
    // the swap-neutral title through the breakpoint.
    expect(phoneTitleRule, '.replay-page .page-title must be re-declared in mobile.css').not.toBe('');
    expect(decl(phoneTitleRule, 'font-size')).toBe('1.25rem');
    expect(decl(phoneTitleRule, 'margin-bottom')).toBe('12px');

    const page = one('.skeleton-page > .replay-page', 'skeleton must wrap the live .replay-page');
    const h1 = page.children[0];
    expect(h1.tagName).toBe('H1');
    expect(h1.className).toBe('page-title');
    expect(h1.textContent).toBe('Replay Viewer');
    expect(h1.getAttribute('style')).toBeNull();
  });

  it('nests .replay-layout of .replay-main and .replay-sidebar, in that order', () => {
    expect(layoutRule, '.replay-layout rule not found in components.css').not.toBe('');
    expect(decl(layoutRule, 'display')).toBe('flex');
    expect(decl(layoutRule, 'gap')).toBe('20px');
    expect(mainRule, '.replay-main rule not found in components.css').not.toBe('');
    expect(decl(mainRule, 'flex')).toBe('1');
    expect(decl(mainRule, 'min-width')).toBe('0');
    expect(sidebarRule, '.replay-sidebar rule not found in components.css').not.toBe('');
    expect(decl(sidebarRule, 'width')).toBe('300px');
    expect(decl(sidebarRule, 'flex-shrink')).toBe('0');

    // Live template order: main first, sidebar last — the columns mobile.css
    // re-orders on phone and re-widths per breakpoint, all through these
    // classes.
    const page = one('.skeleton-page > .replay-page', 'skeleton must wrap the live .replay-page');
    const layout = page.children[1];
    expect(layout.className).toBe('replay-layout');
    const [main, sidebar] = Array.from(layout.children);
    expect(main.className).toBe('replay-main');
    expect(sidebar.className).toBe('replay-sidebar');
    expect(layout.children).toHaveLength(2);
  });

  it('reuses the live class on every wrapper and declares no inline layout', () => {
    const doc = new DOMParser().parseFromString(skeletonReplay(), 'text/html');
    const wrappers = doc.querySelectorAll(
      '.skeleton-page, .replay-page, .replay-layout, .replay-main, .canvas-wrapper, ' +
      '.mobile-replay-controls, .mobile-playback-bar, .mobile-event-timeline, .replay-sidebar'
    );
    // One of each — the wrappers are the live template's, so every breakpoint
    // rule reaches the skeleton through the same selectors and nothing here
    // can override a media query with an inline declaration. The live
    // .canvas-wrapper does carry style="position:relative", but only to anchor
    // the overlays the skeleton does not render (theater button, follow
    // indicator, minimap), and it is box-neutral without an offset.
    expect(wrappers).toHaveLength(9);
    for (const wrapper of Array.from(wrappers)) {
      expect(wrapper.getAttribute('style'), `no inline layout on .${wrapper.className}`).toBeNull();
    }
  });

  it('stands in for the canvas with an aspect-ratio bar, not a fixed height', () => {
    // The live canvas has no width/height attributes until a replay loads, so
    // it lays out at its intrinsic 300×150 ratio under the shared
    // `.canvas-wrapper canvas` rule — an aspect-ratio bar tracks that height
    // at every viewport, where a fixed pixel height could not.
    expect(canvasRule, '.canvas-wrapper canvas rule not found in components.css').not.toBe('');
    expect(decl(canvasRule, 'width')).toBe('100%');
    expect(decl(canvasRule, 'height')).toBe('auto');

    const wrapper = one('.canvas-wrapper', 'skeleton must render the live .canvas-wrapper');
    const [canvasBar, noReplayBar] = Array.from(wrapper.children);
    expect(canvasBar.classList.contains('skeleton-bar')).toBe(true);
    const style = canvasBar.getAttribute('style') ?? '';
    expect(decl(style, 'width')).toBe('100%');
    expect(decl(style, 'aspect-ratio')).toBe('2/1');
    expect(style, 'the canvas bar must not fix a height').not.toMatch(/(?:^|;)\s*height\s*:/);
    // The no-replay bar follows it, as #no-replay follows the canvas.
    expect(noReplayBar.classList.contains('skeleton-bar')).toBe(true);
  });

  it('sizes the no-replay bar to the body-text line it stands in for', () => {
    // The live #no-replay div is .no-replay-message: body-sized text (the rule
    // declares colour, alignment and padding, no font-size), so its line box
    // is the body metric — the derivation the leaderboard header bars use.
    expect(noReplayRule, '.no-replay-message rule not found in replay.ts').not.toBe('');
    expect(noReplayRule).not.toMatch(/font-size/);

    const noReplayBar = one('.canvas-wrapper', 'skeleton must render the live .canvas-wrapper').children[1];
    expect(parseFloat(decl(noReplayBar.getAttribute('style') ?? '', 'height')))
      .toBeCloseTo(rootPx * textLineHeight, 6);
  });

  it('mirrors the playback bar: button-height controls around a text-sized readout', () => {
    // The live bar holds four .btn.small controls, the .mobile-speed-display
    // readout and the speed button, in that order. .btn.small alone floors at
    // 32px — it is the phone block's .mobile-playback-bar .btn rule that
    // restores the 44px tap-target floor, and that is the only viewport the
    // bar shows at (mobile.css hides it at >=640px). The readout is its own
    // 0.75rem text line plus var(--space-xs) padding on both sides.
    expect(playbackBtnRule, '.mobile-playback-bar .btn rule not found in mobile.css').not.toBe('');
    expect(decl(playbackBtnRule, 'min-height')).toBe('44px');
    expect(speedDisplayRule, '.mobile-speed-display rule not found in mobile.css').not.toBe('');
    expect(decl(speedDisplayRule, 'font-size')).toBe('0.75rem');
    expect(decl(speedDisplayRule, 'padding')).toBe('var(--space-xs)');
    const readoutPx = 0.75 * rootPx * textLineHeight + 2 * spaceXs;

    const controls = one('.mobile-replay-controls', 'skeleton must render the live .mobile-replay-controls');
    const playbackBar = controls.children[0];
    expect(playbackBar.className).toBe('mobile-playback-bar');
    const bars = Array.from(playbackBar.children);
    expect(bars, 'one stand-in per live control and readout').toHaveLength(6);
    bars.forEach((bar, i) => {
      const height = parseFloat(decl(bar.getAttribute('style') ?? '', 'height'));
      if (i === 4) expect(height, 'the readout stand-in is text-sized').toBeCloseTo(readoutPx, 6);
      else expect(height, `control stand-in ${i} takes the phone .btn floor`).toBe(44);
    });
  });

  it("mirrors the scrubber: a tap-target bar with the live input's own margin", () => {
    // The live scrubber is a bare <input type="range" style="width:100%;
    // margin-top:4px"> after .mobile-playback-bar — an input, so base.css's
    // tap-target group floors its box at 44px, and the margin comes from the
    // input's inline declaration alone (no rule declares it), so the stand-in
    // carries it inline too.
    expect(tapTargetRule, 'tap-target rule not found in base.css').not.toBe('');
    expect(decl(tapTargetRule, 'min-height')).toBe('44px');
    expect(replaySource, 'the live input must still declare its width and margin inline')
      .toMatch(/<input type="range" id="mobile-turn-slider"[^>]*style="width:100%;margin-top:4px"/);

    const controls = one('.mobile-replay-controls', 'skeleton must render the live .mobile-replay-controls');
    const [playbackBar, scrubber] = Array.from(controls.children);
    expect(playbackBar.className).toBe('mobile-playback-bar');
    expect(scrubber.classList.contains('skeleton-bar')).toBe(true);
    const style = scrubber.getAttribute('style') ?? '';
    expect(decl(style, 'width')).toBe('100%');
    expect(decl(style, 'height')).toBe(decl(tapTargetRule, 'min-height'));
    expect(decl(style, 'margin-top')).toBe('4px');
  });

  it('sizes the timeline bar to the live placeholder span it stands in for', () => {
    // The live .mobile-event-timeline opens with a placeholder span whose
    // inline style is the only declaration sizing it — read out of the
    // template above rather than restated here.
    expect(timelineSpanStyle, 'placeholder span not found in the replay template').not.toBe('');
    expect(decl(timelineSpanStyle, 'font-size')).toBe('0.75rem');
    const padding = decl(timelineSpanStyle, 'padding').split(/\s+/);
    expect(padding).toHaveLength(2);
    const expected = 0.75 * rootPx * textLineHeight + 2 * parseFloat(padding[0]);

    const timeline = one('.mobile-event-timeline', 'skeleton must render the live .mobile-event-timeline');
    const bars = Array.from(timeline.children);
    expect(bars, 'the placeholder span gets one stand-in').toHaveLength(1);
    expect(bars[0].classList.contains('skeleton-bar')).toBe(true);
    expect(parseFloat(decl(bars[0].getAttribute('style') ?? '', 'height'))).toBeCloseTo(expected, 6);
  });

  it('fills the sidebar with fixed-height rectangles', () => {
    // The sidebar's panels are content-sized on the live page, so their
    // stand-ins fix their own heights; the column's x and width are what the
    // rendered parity spec holds, through the shared .replay-sidebar rule
    // pinned above.
    const sidebar = one('.replay-sidebar', 'skeleton must render the live .replay-sidebar');
    const panels = Array.from(sidebar.children);
    expect(panels.length).toBeGreaterThan(0);
    for (const panel of panels) {
      expect(panel.classList.contains('skeleton-bar')).toBe(true);
      const style = panel.getAttribute('style') ?? '';
      expect(decl(style, 'width')).toBe('100%');
      expect(decl(style, 'height'), 'a content-sized panel needs a fixed stand-in height')
        .toMatch(/^[\d.]+px$/);
    }
  });

  it('lets no stand-in override the shared shimmer background', () => {
    // A `background` shorthand resets the shared .skeleton-bar gradient and
    // kills the shimmer — the canvas stand-in especially, whose aspect-ratio
    // extra would tempt one.
    const bars = new DOMParser().parseFromString(skeletonReplay(), 'text/html')
      .querySelectorAll('.skeleton-bar, .skeleton-circle');
    expect(bars.length).toBeGreaterThan(0);
    for (const bar of Array.from(bars)) {
      expect(bar.getAttribute('style') ?? '', `no inline background on ${bar.className}`)
        .not.toMatch(/(?:^|;)\s*background/);
    }
    // …which is why the shimmer must reach every bar from the shared rule.
    expect(shimmerRule, 'shared skeleton shimmer rule not found in components.css').not.toBe('');
    expect(shimmerRule).toContain('skeleton-shimmer 1.5s');
  });

  it('stays decorative: only the title speaks, nothing is interactive, no CSS of its own', () => {
    const doc = new DOMParser().parseFromString(skeletonReplay(), 'text/html');
    // The title is the only text — every placeholder is an empty shimmer bar,
    // so the load never flashes content that looks real but means nothing.
    expect(doc.body.textContent?.trim() ?? '').toBe('Replay Viewer');
    // The live page is controls and inputs end to end; the skeleton stands in
    // with plain divs, and carries none of the live page's ids or aria hooks
    // (the live template's #mobile-controls / #mobile-timeline / labels have
    // nothing to wire to here).
    expect(doc.querySelectorAll('button, a, input, select, textarea')).toHaveLength(0);
    expect(doc.querySelectorAll('[id], [role], [aria-label], [aria-expanded]')).toHaveLength(0);
    expect(skeletonReplay()).not.toContain('<style');
    expect(skeletonReplay()).not.toMatch(/animation\s*:/);
  });

  it('is laid out by the stylesheets alone: the page block owns nothing the skeleton renders', () => {
    // replayPageMarkup's <style> blocks are document-global the moment the
    // swap writes them, and the skeleton is laid out before they exist — so a
    // rule in them for a class the skeleton renders styles a page the skeleton
    // was never shown, and the swap moves it. That was the 640-900px defect:
    // the block carried its own max-width: 900px stacking (and the shared
    // layout, canvas-box and title-margin rules), which shadowed mobile.css's
    // tablet row in the band while the skeleton stayed two-column. The rules
    // live in components.css/mobile.css now, and this guard keeps that block
    // page-local: every selector in it must fail to match the skeleton.
    //
    // Scope: this reads the one block authored here. replayPageMarkup emits
    // four more (<style>${...}</style> of imported component constants), whose
    // bodies are not in this file to read; none of them name a class the
    // skeleton renders, and the swap-parity spec's pre-swap fixture is what
    // holds them — it lays out both real cascades at three viewports, so a
    // rule in any of the five that reached the skeleton would move the
    // measured regions.
    const pageBlock = replaySource.match(/<style>\n([\s\S]*?)<\/style>/)?.[1] ?? '';
    expect(pageBlock, 'replayPageMarkup page-owned <style> block not found').not.toBe('');

    // Every selector in the block, media-wrapped or not: a rule opens with a
    // brace preceded by a brace-free run, so collecting those runs (comments
    // stripped first, at-rule headers included and harmless — none of them
    // name a class) walks the nested @media blocks the bottom sheets live in
    // without parsing the whole grammar.
    const selectors = Array.from(
      pageBlock
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .matchAll(/(?:^|\})\s*([^{}]+?)\{/g),
      (match) => match[1].trim()
    ).filter(Boolean);
    expect(selectors.length).toBeGreaterThan(0);

    // Classes the skeleton renders, plus the shared page-title rule that
    // reaches its h1. A selector matching any of these is a rule the skeleton
    // answers to only before the swap — i.e. a rule that changes the page when
    // the content lands.
    const skeletonClasses = [
      '.replay-page',
      '.page-title',
      '.replay-layout',
      '.replay-main',
      '.canvas-wrapper',
      '.mobile-replay-controls',
      '.mobile-playback-bar',
      '.mobile-event-timeline',
      '.replay-sidebar',
    ];
    for (const selector of selectors) {
      for (const className of skeletonClasses) {
        expect(
          selectorIncludes(selector, className),
          `replayPageMarkup's block must not reach ${className}: "${selector}"`
        ).toBe(false);
      }
    }

    // …and the rules the block gave up are the stylesheets', still there: the
    // shared layout and canvas box in components.css, the 900px stack in
    // mobile.css (written against the phone block's column, so the tablet row
    // only starts above it).
    expect(replayPageTitleRule, '.replay-page .page-title not found in components.css')
      .not.toBe('');
    expect(decl(replayPageTitleRule, 'margin-bottom')).toBe('20px');
    expect(replayPageCanvasRule, '.replay-page .canvas-wrapper not found in components.css')
      .not.toBe('');
    expect(decl(replayPageCanvasRule, 'padding')).toBe('10px');
    expect(decl(replayPageCanvasRule, 'max-height')).toBe('80vh');
    expect(decl(layoutRule, 'display')).toBe('flex');
    expect(replayStackingBlock, 'the 900px stacking block not found in mobile.css').not.toBe('');
    expect(decl(replayStackingBlock.match(/\.replay-layout\s*\{([^}]*)\}/)?.[1] ?? '', 'flex-direction'))
      .toBe('column');
    expect(decl(replayStackingBlock.match(/\.replay-sidebar\s*\{([^}]*)\}/)?.[1] ?? '', 'width'))
      .toBe('100%');
  });
  it('ships the shared layout with the values the stylesheets declare', () => {
    // index.html's inline <style> is the only stylesheet the browser loads —
    // nothing imports src/styles/*.css into the bundle — so the cascade the
    // parity harness lays out under exists in production only through this
    // copy. It is the shipped half of "one cascade": if it drifted from the
    // stylesheets, production would lay the swap out differently than every
    // test here says it does, and no harness would notice. index.html has no
    // --radius-* custom properties, so its copy spells the radius as the
    // literal the token resolves to.
    const radiusLg = decl(rootVars, '--radius-lg');
    expect(radiusLg, '--radius-lg not found in base.css').not.toBe('');

    // Selector -> declaration body, whitespace-collapsed. \s*\{ cannot run
    // past a descendant selector, so '.replay-page' matches only the bare rule
    // and not '.replay-page .page-title'.
    const shipped = (selector: string): string =>
      indexCss
        .match(new RegExp(`${selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*\\{([^}]*)\\}`))
        ?.[1]?.replace(/\s+/g, ' ').trim() ?? '';
    const canon = (body: string): string =>
      body.replace(/var\(--radius-lg\)/g, radiusLg).replace(/\s+/g, ' ').trim();

    const pairs: ReadonlyArray<readonly [selector: string, canonicalBody: string]> = [
      ['.replay-page', replayPageRule],
      ['.replay-page .page-title', replayPageTitleRule],
      ['.replay-layout', layoutRule],
      ['.replay-main', mainRule],
      ['.replay-sidebar', sidebarRule],
      ['.replay-page .canvas-wrapper', replayPageCanvasRule],
    ];
    for (const [selector, canonicalBody] of pairs) {
      expect(canonicalBody, `${selector} not found in the stylesheets`).not.toBe('');
      expect(
        shipped(selector),
        `index.html must ship ${selector} exactly as the stylesheets declare it`
      ).toBe(canon(canonicalBody));
    }

    // …and the 900px stacking, whose canonical home is mobile.css.
    const shippedStacking =
      indexCss.match(/@media \(max-width: 900px\)\s*\{([\s\S]*?)\n    \}/)?.[1] ?? '';
    expect(shippedStacking, 'no 900px replay stacking in index.html').not.toBe('');
    expect(canon(shippedStacking.match(/\.replay-layout\s*\{([^}]*)\}/)?.[1] ?? '')).toBe(
      canon(replayStackingBlock.match(/\.replay-layout\s*\{([^}]*)\}/)?.[1] ?? '')
    );
    expect(canon(shippedStacking.match(/\.replay-sidebar\s*\{([^}]*)\}/)?.[1] ?? '')).toBe(
      canon(replayStackingBlock.match(/\.replay-sidebar\s*\{([^}]*)\}/)?.[1] ?? '')
    );
  });
});

/** Whether a selector's class list contains `className` as a whole class. */
function selectorIncludes(selector: string, className: string): boolean {
  return selector.split('.').some((part) => {
    const name = part.match(/^[a-zA-Z0-9_-]+/)?.[0];
    return name !== undefined && `.${name}` === className;
  });
}
