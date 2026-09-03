/**
 * Layout-parity tests for the leaderboard desktop skeleton (§16.14).
 * skeletonLeaderboard() must render the same #lb-desktop row structure as the
 * live page (renderDesktopRow in pages/leaderboard.ts), with column widths
 * sourced from the --lb-col-* custom properties declared once on .lb-row in
 * styles/components.css — never duplicated as px literals here.
 */

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';
import { skeletonLeaderboard } from './skeleton';

// vitest stubs CSS imports by default, so read the stylesheet source straight
// off disk — it is the single source of truth the bars must resolve from.
// (A static new URL(x, import.meta.url) is rewritten by vite as an asset
// reference, so resolve the path manually.)
const componentsCss = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../styles/components.css'),
  'utf8'
);

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
