/**
 * Layout-parity tests for the leaderboard and bot-profile skeletons (§16.14).
 * skeletonLeaderboard() must render the same #lb-desktop row structure as the
 * live page (renderDesktopRow in pages/leaderboard.ts), with column widths
 * sourced from the --lb-col-* custom properties declared once on .lb-row in
 * styles/components.css — never duplicated as px literals here — plus the same
 * #lb-mobile .mobile-cards card structure as renderMobileCard, whose geometry
 * comes from the .leaderboard-mobile-* rules in styles/mobile.css.
 * skeletonBotProfile() must render the same .bot-profile-page structure as
 * renderProfile (pages/bot-profile.ts): header, grid sections in the live
 * order, every wrapper reusing the live class so the shared rules govern it.
 * It keeps the shared .skeleton-page root, which is layout-transparent here
 * (max-width + auto margins, no padding), so the wrapped 900px column sits
 * where the live page puts it and the swap shifts nothing. The breadcrumb is
 * not part of that structure — it belongs to renderBotProfilePage's shell,
 * outside the content the skeleton stands in for — and the live header renders
 * no avatar, so no avatar-area circle is invented for it either.
 */

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';
import { skeletonLeaderboard, skeletonBotProfile, Skeleton } from './skeleton';
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

  function profileDoc(): Document {
    return new DOMParser().parseFromString(skeletonBotProfile(), 'text/html');
  }

  function profileRoot(): HTMLElement {
    const el = profileDoc().querySelector('.bot-profile-page');
    expect(el, 'skeleton must render inside the live .bot-profile-page container').toBeTruthy();
    return el!;
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

  it('renders the real header: name, status chip, share-card button — no avatar', () => {
    expect(headerRule, '.profile-header rule not found in components.css').not.toBe('');
    expect(decl(headerRule, 'justify-content')).toBe('space-between');
    expect(decl(headerRule, 'margin-bottom')).toBe('var(--space-lg)');
    expect(headerMainRule, '.profile-header-main rule not found in components.css').not.toBe('');
    expect(decl(headerMainRule, 'flex')).toBe('1');

    // The header is the page's first child: the breadcrumb it used to follow
    // is renderBotProfilePage's shell, not part of renderProfile's content.
    const header = profileRoot().children[0];
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
    // The chip is rounded like .profile-status (border-radius: var(--radius-sm)).
    expect(decl(headerBars[1].getAttribute('style') ?? '', 'border-radius')).toBe('var(--radius-sm)');
    // The share-card stand-in is a rectangle, rounded like .btn.
    expect(barOf(button).className).toBe('skeleton-bar');
    expect(decl(button.getAttribute('style') ?? '', 'border-radius')).toBe('var(--radius-md)');
    // .btn height: 0.875rem × 1.5 line-height + 2 × var(--space-sm) padding.
    const btnRule = componentsCss.match(/\.btn\s*\{([^}]*)\}/)?.[1] ?? '';
    expect(decl(btnRule, 'padding')).toBe('var(--space-sm) var(--space-md)');
    expect(decl(btnRule, 'font-size')).toBe('0.875rem');
    expect(decl(button.getAttribute('style') ?? '', 'height')).toBe('37px');
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
    const grid = profileRoot().children[1];
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
    const ratings = profileRoot().children[1].children[0];

    // First child is the <h2>Rating</h2> stand-in, spaced like the base h2 rule.
    const heading = barOf(ratings.children[0]);
    expect(decl(heading.getAttribute('style') ?? '', 'height')).toBe('30px');
    expect(decl(heading.getAttribute('style') ?? '', 'margin-bottom')).toBe('var(--space-md)');

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
    const stats = profileRoot().children[1].children[1];

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
    const grid = profileRoot().children[1];
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

    const lazy = profileRoot().children[1].children[4];
    expect(lazy.className).toBe('lazy-section');
    const placeholder = lazy.children[0];
    expect(placeholder.className).toBe('lazy-placeholder');
    expect(decl(placeholder.getAttribute('style') ?? '', 'min-height')).toBe('80px');
    expect(placeholder.getAttribute('style')).toBe('min-height:80px');
    expect(placeholder.querySelectorAll('.skeleton-bar')).toHaveLength(0);
  });

  it('stays decorative: no interactive elements, no CSS of its own', () => {
    const doc = profileDoc();
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
