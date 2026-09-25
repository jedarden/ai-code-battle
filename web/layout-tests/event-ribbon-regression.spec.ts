/**
 * Event ribbon browser regression net — the behaviors that survived the
 * 2026-09-24 convergence on the ribbon as the replay page's ONLY event
 * timeline (event-timeline.ts deleted, commit 537b9df).
 *
 * Four behaviors, each pinned here against the real component running in a
 * real browser rather than against jsdom or mirrored markup:
 *
 *  1. Responsive rendering — .mobile-event-timeline is the page-wide
 *     timeline: visible at every viewport width (a breakpoint hide-list
 *     regaining the class, or the scroller rules dropping back into the
 *     phone block, must fail here). jsdom has no layout engine; the
 *     legend-placement spec owns the deep placement geometry, this spec
 *     owns the "renders and stays usable at every width" sweep.
 *  2. The E shortcut — replay.ts's KeyE case flips the ribbon's container
 *     via toggleEventTimeline() (lib/event-timeline-toggle.ts). That flip is
 *     exercised here as the real function against the real container, with
 *     the container id coming from the same EVENT_TIMELINE_CONTAINER_ID
 *     constant the page template interpolates — shortcut and markup cannot
 *     drift apart without failing here. The listener wired below mirrors
 *     replay.ts's case; the `viewer.getReplay()` gate stays on the replay
 *     side, where it is page state this fixture does not have.
 *  3. Legend persistence — hide (close button) must survive a real reload
 *     via real localStorage, and the toggle chip must bring it back and
 *     persist that too. jsdom covers the preference logic
 *     (src/components/event-ribbon.test.ts); only a browser proves the real
 *     storage round trip. That is why this spec serves its fixture from a
 *     synthetic http origin (page.route fulfills before any network) —
 *     localStorage throws on the opaque origin a page.setContent document
 *     gets, and the ribbon would silently fall back to "visible".
 *  4. Registry color consistency — markers, their glow, and the legend all
 *     read the event-type-registry, and the per-type marker CSS is
 *     generated from it; asserted here on computed styles in the real
 *     cascade (inline style vs the !important generated rule both landing
 *     on the registry color), including the unknown-type fallback, which is
 *     what unvalidated replay data renders as.
 *
 * The component is not mirrored: the real EventRibbon class is bundled with
 * esbuild (which arrives with vite, pinned by package-lock) and injected as
 * an IIFE, so everything the page runs — markup, styles contract, storage
 * reads — is the shipped source. The app stylesheets are inlined the same
 * way fixture.ts does it (nothing imports them into the app bundle yet);
 * EVENT_RIBBON_STYLES comes from the same module the page injects.
 */

import { expect, test } from '@playwright/test';
import { build } from 'esbuild';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  EVENT_TYPE_REGISTRY,
  UNKNOWN_EVENT_TYPE,
  eventTypeColorWithAlpha,
  type EventTypeDescriptor,
} from '../src/components/event-type-registry';
import { EVENT_RIBBON_STYLES } from '../src/components/event-ribbon';
import type { SignificantEvent, SignificantEventType } from '../src/extract-significant-events';
import { EVENT_TIMELINE_CONTAINER_ID } from '../src/lib/event-timeline-toggle';
import { inlineStyles } from './fixture';
import { readAttrs } from './measure';

const FIXTURE_ORIGIN = 'http://acb-ribbon.fixture';
const stylesDir = resolve(dirname(fileURLToPath(import.meta.url)), '../src/styles');

/** One event per registry type (spread across the track), plus a stale type. */
const RIBBON_EVENTS: SignificantEvent[] = [
  ...(Object.entries(EVENT_TYPE_REGISTRY) as Array<[SignificantEventType, EventTypeDescriptor]>)
    .map(([type, style], i) => ({
      type,
      turn: 5 + i * 12,
      description: `${style.name} regression probe`,
    })),
  // Not in the registry or the union: stands in for replay data that crossed
  // a version boundary — it must render as UNKNOWN_EVENT_TYPE, never empty.
  { type: 'prehistory_mass_firing' as SignificantEventType, turn: 88, description: 'Unknown-type probe' },
];

let ribbonBundle = '';
let toggleBundle = '';

test.beforeAll(async () => {
  const bundle = async (entry: string, globalName: string): Promise<string> => {
    const result = await build({
      entryPoints: [resolve(stylesDir, '..', entry)],
      bundle: true,
      write: false,
      format: 'iife',
      globalName,
      logLevel: 'silent',
    });
    return result.outputFiles[0].text;
  };
  ribbonBundle = await bundle('components/event-ribbon.ts', 'ACBEventRibbonBundle');
  toggleBundle = await bundle('lib/event-timeline-toggle.ts', 'ACBTimelineToggle');
});

/** The static page: app stylesheets, the ribbon's own style block, and the container replay.ts's template mounts. */
function fixtureHtml(): string {
  const appCss = ['base.css', 'components.css', 'mobile.css']
    .map((f) => readFileSync(resolve(stylesDir, f), 'utf8'))
    .join('\n');
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ACB event ribbon regression fixture</title>
  <style>${appCss}</style>
  <style>${EVENT_RIBBON_STYLES}</style>
</head>
<body>
  <div class="mobile-event-timeline" id="${EVENT_TIMELINE_CONTAINER_ID}" aria-label="Event timeline"></div>
</body>
</html>`;
}

async function openRibbonPage(page: import('@playwright/test').Page): Promise<void> {
  await page.route((url) => url.hostname === 'acb-ribbon.fixture', (route) =>
    route.fulfill({ body: fixtureHtml(), contentType: 'text/html' }));
  await page.goto(`${FIXTURE_ORIGIN}/`, { waitUntil: 'load' });
  await mountRibbon(page);
}

/**
 * Inject the real bundles and construct the ribbon the way replay.ts's init
 * does: wipe the placeholder, construct into the container, setEvents, then
 * renderLegend. Called after every load/reload — the constructor reads the
 * legend preference from localStorage at construction time, which is exactly
 * the persistence path being tested.
 *
 * The scrub handlers record into window.__ribbonClicks the way replay.ts's
 * record into viewer.setTurn — the click-to-jump test reads them back out.
 */
async function mountRibbon(page: import('@playwright/test').Page): Promise<void> {
  await page.addScriptTag({ content: ribbonBundle });
  await page.addScriptTag({ content: toggleBundle });
  await page.evaluate(({ id, events }: { id: string; events: SignificantEvent[] }) => {
    (window as unknown as { __ribbonClicks: { events: unknown[]; turns: number[] } }).__ribbonClicks =
      { events: [], turns: [] };
    const container = document.getElementById(id) as HTMLElement;
    container.innerHTML = '';
    const ribbon = new (window as unknown as {
      ACBEventRibbonBundle: { EventRibbon: new (options: Record<string, unknown>) => unknown };
    }).ACBEventRibbonBundle.EventRibbon({
      container,
      events,
      totalTurns: 100,
      onEventClick: (event: unknown) => {
        (window as unknown as { __ribbonClicks: { events: unknown[] } }).__ribbonClicks
          .events.push(event);
      },
      onTurnClick: (turn: number) => {
        (window as unknown as { __ribbonClicks: { turns: number[] } }).__ribbonClicks.turns.push(turn);
      },
    });
    (ribbon as { setEvents(e: SignificantEvent[], t: number): void }).setEvents(events, 100);
    (ribbon as { renderLegend(): void }).renderLegend();
  }, { id: EVENT_TIMELINE_CONTAINER_ID, events: RIBBON_EVENTS });
}

/** Sub-pixel noise guard, matching the other specs. */
const EPS = 0.5;

// ── 1. Responsive rendering ──────────────────────────────────────────────────

const WIDTHS = [320, 390, 768, 1280];

for (const width of WIDTHS) {
  test(`[${width}px] the page-wide timeline renders, holds its markers, and overflows nothing`, async ({ page }) => {
    await openRibbonPage(page);
    await page.setViewportSize({ width, height: 900 });

    const container = await page.evaluate((id: string) => {
      const el = document.getElementById(id) as HTMLElement;
      const rect = el.getBoundingClientRect();
      return {
        display: getComputedStyle(el).display,
        visible: rect.width > 0 && rect.height > 0,
        left: rect.left,
        right: rect.right,
        scrollWidth: document.documentElement.scrollWidth,
      };
    }, EVENT_TIMELINE_CONTAINER_ID);

    expect(container.display, 'no breakpoint hides the one timeline').toBe('flex');
    expect(container.visible).toBe(true);

    const ribbon = await page.locator('#' + EVENT_TIMELINE_CONTAINER_ID + ' .event-ribbon')
      .boundingBox();
    expect(ribbon, 'the ribbon is inside the container').toBeTruthy();
    expect(ribbon!.height).toBeCloseTo(48, 1);

    // Every marker stays within the container's horizontal band
    const markers = await page.locator('#' + EVENT_TIMELINE_CONTAINER_ID + ' .event-marker').all();
    expect(markers.length).toBe(RIBBON_EVENTS.length);
    for (const marker of markers) {
      const box = (await marker.boundingBox())!;
      expect(box.x).toBeGreaterThanOrEqual(container.left - EPS);
      expect(box.x + box.width).toBeLessThanOrEqual(container.right + EPS);
    }

    // The legend renders below the ribbon (light check — the placement spec
    // owns the deep geometry) and the page gains no horizontal scrollbar
    const legend = await page.locator('.event-ribbon-legend').boundingBox();
    expect(legend).toBeTruthy();
    expect(legend!.y).toBeGreaterThanOrEqual(ribbon!.y + ribbon!.height - EPS);
    expect(container.scrollWidth).toBeLessThanOrEqual(width + EPS);

    // The legacy timeline — event-timeline.ts, deleted at 537b9df — used to
    // mount its own strip under the canvas at EVERY width, which is exactly
    // the regression to guard: none of its markup may exist at any viewport
    // width. The ribbon's container is the page's one event timeline.
    const legacyNodes = await page.evaluate(() => document.querySelectorAll(
      '#event-timeline-container, .timeline-track, .timeline-progress, #timeline-progress, '
      + '#timeline-current, #timeline-total, .timeline-turn-label, .timeline-event, '
      + '.timeline-annotation, .timeline-empty, .ann-marker-count',
    ).length);
    expect(legacyNodes, `no legacy event-timeline.ts markup at ${width}px`).toBe(0);
  });
}

// ── 2. The E shortcut ────────────────────────────────────────────────────────

test('KeyE toggles the timeline container off and back on', async ({ page }) => {
  await openRibbonPage(page);

  // Mirror of replay.ts's KeyE case, minus the replay-loaded gate (page state
  // this fixture does not have). The flip runs the real bundled function.
  await page.evaluate(() => {
    document.addEventListener('keydown', (e) => {
      if (e.code === 'KeyE') {
        e.preventDefault();
        (window as unknown as {
          ACBTimelineToggle: { toggleEventTimeline(): void };
        }).ACBTimelineToggle.toggleEventTimeline();
      }
    });
  });

  const container = page.locator('#' + EVENT_TIMELINE_CONTAINER_ID);
  await expect(container).toBeVisible();

  await page.keyboard.press('e');
  await expect(container).toBeHidden();
  expect(await container.evaluate((el) => el.style.display)).toBe('none');

  await page.keyboard.press('e');
  await expect(container).toBeVisible();
  expect(await container.evaluate((el) => el.style.display)).toBe('');
});

// ── 3. Legend persistence ────────────────────────────────────────────────────

test('hiding the legend survives a real reload; the toggle chip brings it back and persists too', async ({ page }) => {
  await openRibbonPage(page);
  await expect(page.locator('.event-ribbon-legend')).toBeVisible();

  // Close button: hide + save
  await page.locator('.event-legend-close').click();
  await expect(page.locator('.event-ribbon-legend')).toBeHidden();
  await expect(page.locator('.event-legend-toggle')).toBeVisible();

  // The reload is the product flow: fresh page, fresh constructor, stored
  // preference read back from real localStorage
  await page.reload({ waitUntil: 'load' });
  await mountRibbon(page);
  await expect(page.locator('.event-ribbon-legend')).toBeHidden();
  await expect(page.locator('.event-legend-toggle')).toBeVisible();

  // Toggle chip: show + save, and that survives the next reload
  await page.locator('.event-legend-toggle').click();
  await expect(page.locator('.event-ribbon-legend')).toBeVisible();

  await page.reload({ waitUntil: 'load' });
  await mountRibbon(page);
  await expect(page.locator('.event-ribbon-legend')).toBeVisible();
});

// ── 4. Registry color consistency ────────────────────────────────────────────

test('markers, glow and legend all land on the registry color in the real cascade', async ({ page }) => {
  await openRibbonPage(page);

  const types = await page.evaluate((id: string) => {
    const root = document.getElementById(id)!;
    return Array.from(root.querySelectorAll('.event-marker'), (m) =>
      (m as HTMLElement).dataset.eventType ?? '');
  }, EVENT_TIMELINE_CONTAINER_ID);
  expect(types.length).toBe(RIBBON_EVENTS.length);

  // Expected values come from the real registry objects on the Node side of
  // the same source tree; the selectors walk the DOM the browser actually built
  for (const type of types) {
    const known = (EVENT_TYPE_REGISTRY as Record<string, EventTypeDescriptor>)[type];
    const style = known ?? UNKNOWN_EVENT_TYPE;
    const expectedRgb = hexToRgbString(style.color);
    const expectedGlow = eventTypeColorWithAlpha(style.color, 0.6);
    const markerSel = `.event-marker[data-event-type="${type}"] .event-marker-icon`;
    const legendSel = `.event-legend-item[data-event-type="${type}"] .event-legend-icon`;

    const marker = await readComputed(page, markerSel);
    expect(marker.color, `marker color for ${type}`).toBe(expectedRgb);
    if (known) {
      expect(marker.textShadow, `marker glow for ${type}`).toContain(expectedGlow);
    } else {
      // The per-type glow CSS is generated from the registry, so it exists
      // only for known types: an unknown replay type must not reach a class
      // name raw (that would let arbitrary data pick a CSS selector), so it
      // carries the fallback color inline and no glow at all
      expect(marker.textShadow, `glow for unregistered ${type}`).toBe('none');
    }

    const legend = await readComputed(page, legendSel);
    expect(legend.color, `legend color for ${type}`).toBe(expectedRgb);

    // The label lives in the icon's sibling span, not in the icon itself
    const legendLabel = await page
      .locator(`.event-legend-item[data-event-type="${type}"] .event-legend-label`)
      .textContent();
    expect(legendLabel, `legend label for ${type}`).toBe(style.name);
  }

  // The unknown type renders the fallback in both places — visible and
  // consistent, never an empty marker
  const unknownType = RIBBON_EVENTS[RIBBON_EVENTS.length - 1].type;
  expect((EVENT_TYPE_REGISTRY as Record<string, EventTypeDescriptor>)[unknownType]).toBeUndefined();
  const unknownLegendItem = await readAttrs(page,
    `.event-legend-item[data-event-type="${unknownType}"]`, ['class']);
  expect(unknownLegendItem.class).toContain('event-legend-item-unknown');
});

// ── 5. Registry icon consistency ─────────────────────────────────────────────
// The color test above pins the computed cascade; this one pins the glyph
// itself, per type, on both surfaces the registry feeds — the marker renders
// `event.emoji || style.icon` (no fixture event carries an emoji, so the
// registry half is what must show), and the legend row must agree with it.

test('every registry type renders its own icon glyph on marker and legend alike', async ({ page }) => {
  await openRibbonPage(page);

  const glyphs = await page.evaluate((id: string) => {
    const root = document.getElementById(id)!;
    return {
      markers: Array.from(root.querySelectorAll('.event-marker'), (m) => ({
        type: (m as HTMLElement).dataset.eventType ?? '',
        glyph: (m.querySelector('.event-marker-icon') as HTMLElement).textContent?.trim() ?? '',
      })),
      legend: Array.from(root.querySelectorAll('.event-legend-item'), (el) => ({
        type: (el as HTMLElement).dataset.eventType ?? '',
        glyph: (el.querySelector('.event-legend-icon') as HTMLElement).textContent?.trim() ?? '',
      })),
    };
  }, EVENT_TIMELINE_CONTAINER_ID);

  expect(glyphs.markers.length).toBe(RIBBON_EVENTS.length);
  for (const { type, glyph } of glyphs.markers) {
    const style = (EVENT_TYPE_REGISTRY as Record<string, EventTypeDescriptor>)[type] ?? UNKNOWN_EVENT_TYPE;
    expect(glyph, `marker glyph for ${type}`).toBe(style.icon);
  }
  for (const { type, glyph } of glyphs.legend) {
    const style = (EVENT_TYPE_REGISTRY as Record<string, EventTypeDescriptor>)[type] ?? UNKNOWN_EVENT_TYPE;
    expect(glyph, `legend glyph for ${type}`).toBe(style.icon);
  }
});

// ── 6. Tooltips in the real browser ──────────────────────────────────────────
// jsdom owns the tooltip's content and hide-timing logic; only a real hover
// proves the mouseenter wiring raises the shared tooltip with the registry
// facts, swaps between markers, and hides once the pointer is gone.

test('hovering a marker shows the shared tooltip with the registry facts, and it hides again', async ({ page }) => {
  await openRibbonPage(page);

  const tooltip = page.locator('.event-tooltip');
  const markerFor = (type: string) =>
    page.locator(`#${EVENT_TIMELINE_CONTAINER_ID} .event-marker[data-event-type="${type}"]`);

  await markerFor('combat').hover();
  await expect(tooltip).toHaveClass(/event-tooltip-visible/);
  expect(await tooltip.getAttribute('aria-hidden')).toBe('false');
  await expect(tooltip.locator('.event-tooltip-icon')).toHaveText(EVENT_TYPE_REGISTRY.combat.icon);
  await expect(tooltip.locator('.event-tooltip-type')).toHaveText(EVENT_TYPE_REGISTRY.combat.name);
  await expect(tooltip.locator('.event-tooltip-description')).toHaveText('Combat regression probe');
  await expect(tooltip.locator('.event-tooltip-turn')).toHaveText('Turn 5');

  // Sliding to the unknown-type marker keeps the same tooltip up and swaps
  // the content to the fallback descriptor — never the raw type string
  await markerFor('prehistory_mass_firing').hover();
  await expect(tooltip.locator('.event-tooltip-icon')).toHaveText(UNKNOWN_EVENT_TYPE.icon);
  await expect(tooltip.locator('.event-tooltip-type')).toHaveText(UNKNOWN_EVENT_TYPE.name);

  // Pointer gone: the 100ms grace passes and the tooltip hides
  await page.mouse.move(4, 4);
  await expect(tooltip).not.toHaveClass(/event-tooltip-visible/);
  expect(await tooltip.getAttribute('aria-hidden')).toBe('true');
});

// ── 7. Click-to-jump in the real browser ─────────────────────────────────────
// jsdom owns the guard matrix (secondary button, zero width, double-fire);
// this proves the real click paths land: a marker click jumps to its event
// and its turn — once — and a track click seeks the turn under the pointer.

test('clicking a marker jumps to its event; clicking the track seeks the turn under the pointer', async ({ page }) => {
  await openRibbonPage(page);

  const readClicks = () => page.evaluate(() => (
    (window as unknown as {
      __ribbonClicks: { events: Array<{ type: string }>; turns: number[] };
    }).__ribbonClicks));

  const markerFor = (type: string) =>
    page.locator(`#${EVENT_TIMELINE_CONTAINER_ID} .event-marker[data-event-type="${type}"]`);
  await markerFor('combat').click();

  const clicks = await readClicks();
  expect(clicks.events.map((e) => e.type), 'the marker jump carries its event').toEqual(['combat']);
  expect(clicks.turns, 'and its turn — once, the track seek kept out by stopPropagation')
    .toEqual([5]);

  // Track click in the widest stretch of free track (merge the tap targets,
  // take the largest gap): the seek is the inverse of the marker placement —
  // the turn at the click's x, clamped to the last turn
  const gap = await page.evaluate((id: string) => {
    const root = document.getElementById(id)!;
    const ribbon = root.querySelector('.event-ribbon') as HTMLElement;
    const rect = ribbon.getBoundingClientRect();
    const targets = Array.from(root.querySelectorAll('.event-marker'),
      (m) => (m as HTMLElement).getBoundingClientRect())
      .map((b) => ({ l: b.left, r: b.right }))
      .sort((a, b) => a.l - b.l);
    const gaps: Array<{ l: number; r: number }> = [];
    let cursor = rect.left;
    for (const t of targets) {
      if (t.l > cursor) gaps.push({ l: cursor, r: Math.min(t.l, rect.right) });
      cursor = Math.max(cursor, t.r);
    }
    if (cursor < rect.right) gaps.push({ l: cursor, r: rect.right });
    const widest = gaps.reduce((a, b) => (b.r - b.l > a.r - a.l ? b : a));
    return { x: (widest.l + widest.r) / 2, left: rect.left, width: rect.width };
  }, EVENT_TIMELINE_CONTAINER_ID);

  const ribbonBox = await page.locator(`#${EVENT_TIMELINE_CONTAINER_ID} .event-ribbon`)
    .boundingBox();
  await page.mouse.click(gap.x, ribbonBox!.y + ribbonBox!.height / 2);

  const after = await readClicks();
  const ratio = (gap.x - gap.left) / gap.width;
  const expectedTurn = Math.max(0, Math.min(99, Math.round(ratio * 100)));
  expect(after.events.length, 'a track click never fires the event jump').toBe(1);
  expect(after.turns.length).toBe(2);
  expect(after.turns[1], `the seek lands on turn ${expectedTurn} at this width`).toBe(expectedTurn);
});

async function readComputed(
  page: import('@playwright/test').Page,
  selector: string,
): Promise<{ color: string; textShadow: string }> {
  return page.evaluate((sel: string) => {
    const el = document.querySelector(sel) as HTMLElement | null;
    if (!el) throw new Error(`no element matches ${sel}`);
    const computed = getComputedStyle(el);
    return { color: computed.color, textShadow: computed.textShadow };
  }, selector);
}

function hexToRgbString(hex: string): string {
  const value = hex.replace('#', '');
  return `rgb(${parseInt(value.slice(0, 2), 16)}, ${parseInt(value.slice(2, 4), 16)}, ${parseInt(value.slice(4, 6), 16)})`;
}
