/**
 * Layout-parity tests for the leaderboard skeleton (§16.14).
 * skeletonLeaderboard() must render the same #lb-desktop row structure as the
 * live page (renderDesktopRow in pages/leaderboard.ts), with column widths
 * sourced from the --lb-col-* custom properties declared once on .lb-row in
 * styles/components.css — never duplicated as px literals here — plus the same
 * #lb-mobile .mobile-cards card structure as renderMobileCard, whose geometry
 * comes from the .leaderboard-mobile-* rules in styles/mobile.css.
 */

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';
import { skeletonLeaderboard, Skeleton } from './skeleton';

// vitest stubs CSS imports by default, so read the stylesheet source straight
// off disk — it is the single source of truth the bars must resolve from.
// (A static new URL(x, import.meta.url) is rewritten by vite as an asset
// reference, so resolve the path manually.)
const stylesDir = resolve(dirname(fileURLToPath(import.meta.url)), '../styles');
const componentsCss = readFileSync(resolve(stylesDir, 'components.css'), 'utf8');
const mobileCss = readFileSync(resolve(stylesDir, 'mobile.css'), 'utf8');

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
    for (const card of mobileContainer().children) {
      const toggle = card.children[0];
      expect(toggle.classList.contains('mobile-card-toggle')).toBe(true);
      expect(toggle.tagName).toBe('DIV');
      const [rank, info, arrow] = Array.from(toggle.children);
      expect(rank.classList.contains('leaderboard-mobile-rank')).toBe(true);
      expect(rank.querySelectorAll('.skeleton-bar').length).toBe(1);
      expect(info.classList.contains('leaderboard-mobile-info')).toBe(true);
      expect(arrow.classList.contains('mobile-card-arrow')).toBe(true);
      expect(arrow.querySelectorAll('.skeleton-bar').length).toBe(1);

      const infoBars = Array.from(info.children);
      expect(infoBars[0].classList.contains('skeleton-bar')).toBe(true);
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
    expect(container.getAttribute('role')).toBeNull();
    expect(container.innerHTML).not.toContain('<button');
    expect(container.innerHTML).not.toContain('<a ');
    expect(container.innerHTML).not.toContain('data-bot-id');
    expect(container.innerHTML).not.toMatch(/animation\s*:/);
  });
});
