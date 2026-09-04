/**
 * Rendered legend placement — the layout half of the ribbon's legend contract.
 *
 * jsdom has no layout engine, so the guards in
 * src/components/event-ribbon.test.ts can only pin the structure (the legend is
 * a sibling of the ribbon inside .event-ribbon-root) and the stylesheet text.
 * Whether that structure actually lays out as "legend below the ribbon, ribbon
 * exactly where it was, nothing covering the neighbours" is a question only a
 * real engine can answer, and it is the whole of the placement acceptance
 * criteria — hence this spec.
 *
 * The ribbon markup is not mirrored here: the real EventRibbon class builds it.
 * Playwright specs run in Node, where there is no DOM, so the class is
 * instantiated against jsdom purely to produce markup — no geometry is read
 * there — and that markup is then laid out by Chromium under the same
 * stylesheets the app ships: the app styles (inlineStyles(), the same list
 * fixture.ts inlines) plus EVENT_RIBBON_STYLES, which replay.ts injects into
 * the page in a <style> of its own. The one mirrored element is the container,
 * a single static line of replay.ts's template:
 *
 *   <div class="mobile-event-timeline" id="mobile-timeline">
 *
 * Reproducing it is what puts the ribbon under the same display:flex +
 * overflow-x:auto scroller the page uses — the layout the stack root exists to
 * survive — and each instance is followed by a plain sibling so "in flow with
 * the surrounding page" has something concrete to be asserted against.
 *
 * Widths: .mobile-event-timeline is display:flex only inside
 * @media (max-width: 639px); mobile.css hides it from 640px to 1023px and again
 * at 1024px and up. The ribbon and its legend therefore only ever render on
 * phones, so "wide and narrow" means within that range: 639 (the widest it
 * ever shows), 390 (a common phone) and 320 (the narrowest worth supporting).
 */

import { expect, test } from '@playwright/test';
import { JSDOM } from 'jsdom';
import { EventRibbon, EVENT_RIBBON_STYLES } from '../src/components/event-ribbon';
import type { SignificantEvent } from '../src/extract-significant-events';
import { inlineStyles } from './fixture';
import { measure, measureAll, readStyles } from './measure';

/** Same-turn pairs exercise the stack cascade; the rest spread across the track. */
const SAMPLE_EVENTS: SignificantEvent[] = [
  { type: 'combat', turn: 5, description: 'Opening skirmish', emoji: '⚔️' },
  { type: 'energy', turn: 20, description: 'Energy milestone', emoji: '⚡' },
  { type: 'combat', turn: 40, description: 'Flank engaged', emoji: '⚔️' },
  { type: 'combat', turn: 40, description: 'Counterattack', emoji: '⚔️' },
  { type: 'elimination', turn: 55, description: 'Bot lost', emoji: '💀' },
  { type: 'energy', turn: 70, description: 'Second milestone', emoji: '⚡' },
  { type: 'combat', turn: 85, description: 'Final push', emoji: '⚔️' },
  { type: 'elimination', turn: 95, description: 'Bot eliminated', emoji: '💀' },
];

/**
 * The ribbon component needs a `document` to build its DOM against, and it
 * reads the bare global rather than taking one — so the jsdom document is
 * swapped in for the construction and swapped back out afterwards. jsdom here
 * is a markup factory only: nothing measures layout against it.
 */
function ribbonMarkup(withLegend: boolean): string {
  const dom = new JSDOM('<!doctype html><html><body><div id="host"></div></body></html>');
  const previous = (globalThis as { document?: Document }).document;
  (globalThis as { document?: Document }).document = dom.window.document as unknown as Document;
  try {
    const host = dom.window.document.getElementById('host') as HTMLElement;
    const ribbon = new EventRibbon({ container: host, events: SAMPLE_EVENTS, totalTurns: 100 });
    if (withLegend) ribbon.renderLegend();
    return host.innerHTML;
  } finally {
    (globalThis as { document?: Document }).document = previous;
  }
}

function buildFixtureHtml(): string {
  const withLegend = ribbonMarkup(true);
  const withoutLegend = ribbonMarkup(false);
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ACB event ribbon legend placement fixture</title>
  <style>${inlineStyles()}</style>
  <style>${EVENT_RIBBON_STYLES}</style>
</head>
<body>
  <section aria-label="ribbon with legend">
    <div class="mobile-event-timeline" id="tl-legend">${withLegend}</div>
    <p id="after-legend">Page content that follows the ribbon</p>
  </section>
  <section aria-label="ribbon without legend">
    <div class="mobile-event-timeline" id="tl-bare">${withoutLegend}</div>
    <p id="after-bare">Page content that follows the ribbon</p>
  </section>
</body>
</html>`;
}

async function openRibbonFixture(page: import('@playwright/test').Page, width: number): Promise<void> {
  await page.setContent(buildFixtureHtml(), { waitUntil: 'load' });
  await page.setViewportSize({ width, height: 900 });
}

/** Sub-pixel noise guard: two layouts of the same box agree to a hundredth of a px. */
const px = (value: number): number => Math.round(value * 100) / 100;

/** Marker rects relative to the timeline container that holds them. */
async function markerGeometry(page: import('@playwright/test').Page, containerId: string) {
  const container = await measure(page, `#${containerId}`);
  const markers = await measureAll(page, `#${containerId} .event-marker`);
  return markers.map((m) => ({
    left: px(m.left - container.left),
    top: px(m.top - container.top),
    width: px(m.width),
    height: px(m.height),
  }));
}

const WIDTHS = [639, 390, 320];
const EPS = 0.5;

for (const width of WIDTHS) {
  test(`[${width}px] the legend renders below the ribbon, never over it`, async ({ page }) => {
    await openRibbonFixture(page, width);

    const timeline = await measure(page, '#tl-legend');
    expect(timeline.display, 'the ribbon only renders inside the ≤639px phone block').toBe('flex');

    const ribbon = await measure(page, '#tl-legend .event-ribbon');
    const legend = await measure(page, '#tl-legend .event-ribbon-legend');

    expect(legend.display).not.toBe('none');
    // Below, with the legend's own 4px margin-top respected — never floating
    // over the timeline's vertical band, so no marker or turn label is covered
    expect(legend.top).toBeGreaterThanOrEqual(ribbon.bottom - EPS);
    expect(legend.top).toBeGreaterThan(ribbon.top);
  });

  test(`[${width}px] the ribbon is unmoved by the legend`, async ({ page }) => {
    await openRibbonFixture(page, width);

    // The two timelines hold the same markup apart from the legend, so the
    // ribbon's box *inside its own container* must come out identical. Relative
    // to the container, not the viewport: the legend makes its case taller,
    // which legitimately pushes the second case further down the page
    const legendCase = await measure(page, '#tl-legend');
    const bareCase = await measure(page, '#tl-bare');
    const withLegend = await measure(page, '#tl-legend .event-ribbon');
    const withoutLegend = await measure(page, '#tl-bare .event-ribbon');

    expect(px(withLegend.left - legendCase.left)).toBe(px(withoutLegend.left - bareCase.left));
    expect(px(withLegend.top - legendCase.top)).toBe(px(withoutLegend.top - bareCase.top));
    expect(px(withLegend.width)).toBe(px(withoutLegend.width));
    expect(px(withLegend.height)).toBe(px(withoutLegend.height));

    // ...and so must every marker's position within it
    const withLegendMarkers = await markerGeometry(page, 'tl-legend');
    const withoutLegendMarkers = await markerGeometry(page, 'tl-bare');
    expect(withLegendMarkers.length).toBeGreaterThan(0);
    expect(withLegendMarkers).toEqual(withoutLegendMarkers);
  });

  test(`[${width}px] the ribbon and legend stay in flow with the page`, async ({ page }) => {
    await openRibbonFixture(page, width);

    const [rootPosition, legendPosition] = await Promise.all([
      readStyles(page, '#tl-legend .event-ribbon-root', ['position']),
      readStyles(page, '#tl-legend .event-ribbon-legend', ['position']),
    ]);
    // relative (not absolute/fixed) keeps the group in flow; the legend is
    // static, so it takes up space rather than painting over anything
    expect(rootPosition.position).toBe('relative');
    expect(legendPosition.position).toBe('static');

    // The group ends before the page's next block begins: no neighbour is
    // overlapped, and the legend is part of that group rather than escaping it
    const root = await measure(page, '#tl-legend .event-ribbon-root');
    const after = await measure(page, '#after-legend');
    expect(after.top).toBeGreaterThanOrEqual(root.bottom - EPS);

    const legend = await measure(page, '#tl-legend .event-ribbon-legend');
    expect(legend.bottom).toBeLessThanOrEqual(root.bottom + EPS);
  });
}

// The legend narrows in two deliberate regimes (EVENT_RIBBON_STYLES):
// wider than 480px it wraps its entries onto further rows; at 480px and under
// it becomes a single swipeable row (flex-wrap: nowrap, overflow-x: auto,
// flex-shrink: 0 on the entries). Either way the constraint that matters is the
// same — the extra entries cost vertical space or a swipe, never horizontal
// spill out of the legend, and never a nudge to the ribbon or the page.
test('wide, the legend wraps; narrow, it scrolls — never overflowing', async ({ page }) => {
  const legendScroll = (page: import('@playwright/test').Page) =>
    page.evaluate(() => {
      const content = document.querySelector('#tl-legend .event-legend-content') as HTMLElement;
      return { clientWidth: content.clientWidth, scrollWidth: content.scrollWidth };
    });

  // Wider than the 480px breakpoint: rows, and nothing hidden past the edge
  await openRibbonFixture(page, 639);
  const wideContent = await measure(page, '#tl-legend .event-legend-content');
  const wide = await legendScroll(page);
  expect(wide.scrollWidth, 'wrapped rows leave nothing scrolled out of view')
    .toBeLessThanOrEqual(wide.clientWidth + EPS);
  expect(wideContent.height).toBeGreaterThan(30);

  // 480px and under: one row the user can swipe
  await page.setViewportSize({ width: 320, height: 900 });
  const narrowRoot = await measure(page, '#tl-legend .event-ribbon-root');
  const narrowLegend = await measure(page, '#tl-legend .event-ribbon-legend');
  const narrowTimeline = await measure(page, '#tl-legend');
  const narrow = await legendScroll(page);
  expect(narrow.scrollWidth).toBeGreaterThan(narrow.clientWidth);

  // The strip really scrolls — the clipped entries stay reachable, which is
  // what makes "scrolls" a valid answer to a narrow container
  const scrolled = await page.evaluate(() => {
    const content = document.querySelector('#tl-legend .event-legend-content') as HTMLElement;
    content.scrollLeft = 999;
    return content.scrollLeft;
  });
  expect(scrolled).toBeGreaterThan(0);

  // ...and none of that spills: the legend stays inside its root, the ribbon
  // stays inside its container, and the page gains no horizontal scrollbar
  expect(narrowLegend.right).toBeLessThanOrEqual(narrowRoot.right + EPS);
  expect(narrowLegend.width).toBeLessThanOrEqual(narrowRoot.width + EPS);
  const narrowRibbon = await measure(page, '#tl-legend .event-ribbon');
  expect(narrowRibbon.width).toBeLessThanOrEqual(narrowTimeline.width + EPS);
  expect(await page.evaluate(() => document.documentElement.scrollWidth))
    .toBeLessThanOrEqual(320 + EPS);
});

