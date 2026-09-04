/**
 * Page-level half of the replay skeleton criterion (§16.14 #2): the
 * skeleton → content swap itself.
 *
 * The jsdom guards (src/components/skeleton.test.ts) hold skeletonReplay()'s
 * markup against the CSS text; here both sides go through a real layout engine
 * (see ./fixture.ts, whose inlineStyles() this spec borrows for the same three
 * stylesheets). The live side is replayPageMarkup() — the markup the real swap
 * writes into #app, page-owned <style> blocks included — imported rather than
 * mirrored, so it cannot drift from what the app actually renders. Those
 * blocks are document-global once the markup lands, and the skeleton reuses
 * the live classes, so both sections answer to the same full cascade; at
 * 640-900px that cascade is replayPageMarkup's own max-width: 900px block
 * (column, full-width sidebar) rather than mobile.css's 640-1023px row, on
 * both sides alike.
 *
 * Like the other parity specs, every geometry assertion is relative — skeleton
 * vs live, each measured within its own fixture section — so a redesign that
 * moves both sides together stays green, and only a swap that actually shifts
 * content bites.
 *
 * Two comparisons are deliberately narrower than "the whole box":
 *  - the .canvas-wrapper's height: the live wrapper gains #no-replay, whose
 *    .no-replay-message rule pads 60px around the same body-text line the
 *    skeleton stands in for with a bare one-line bar, so the wrapper grows
 *    when the content lands and anything stacked under it moves down with it.
 *    On phone it cannot grow: mobile.css pins the wrapper to an aspect-ratio
 *    square with overflow hidden, which is also why the phone sweep can
 *    compare the sidebar's top. The canvas region inside the wrapper — the
 *    surface the page is for — is compared in full at every width.
 *  - the .replay-sidebar's top at 768px, for the same reason: there the two
 *    columns stack and the live main column is the one that grew, so the
 *    sidebar hangs lower on the live side. Its x and width are what the
 *    layout grid assigns and no content size can move, so they are compared
 *    at every width.
 */

import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';
import { inlineStyles } from './fixture';
import { skeletonReplay } from '../src/components/skeleton';
import { replayPageMarkup } from '../src/pages/replay';

// Sub-pixel allowance, same rationale as the other parity specs: rounding at
// fractional DPR is not the drift this spec exists for — a swap that shifts
// the canvas or the sidebar moves them by multiples of a padding or a gap.
const PX_TOLERANCE = 1;

interface Box {
  top: number;
  left: number;
  width: number;
  height: number;
  /** False once the element — or any ancestor — lays out no box at all. */
  visible: boolean;
}

/** Box fields of the first element matching `selector`, within its section. */
async function boxWithinSection(
  page: Page,
  selector: string,
  sectionId: string
): Promise<Box> {
  return page.evaluate(
    ({ sel, section }) => {
      // Section-scoped, not document-scoped: the live markup carries the real
      // page's ids (#replay-canvas, #url-input, …), so a document-wide query
      // could not tell the two sides apart.
      const sec = document.getElementById(section);
      const el = sec?.querySelector(sel);
      if (!el || !sec) throw new Error(`missing "${sel}" in #${section}`);
      const rect = el.getBoundingClientRect();
      const secRect = sec.getBoundingClientRect();
      return {
        top: rect.top - secRect.top,
        left: rect.left - secRect.left,
        width: rect.width,
        height: rect.height,
        // Not the element's own computed display: a block hidden by an
        // ancestor's display:none still reports its own `flex`, and only its
        // client rects going empty says it occupies no box.
        visible: el.getClientRects().length > 0,
      };
    },
    { sel: selector, section: sectionId }
  );
}

function within(actual: number, expected: number, label: string): void {
  expect(
    Math.abs(actual - expected),
    `${label}: ${actual.toFixed(2)}px vs ${expected.toFixed(2)}px`
  ).toBeLessThanOrEqual(PX_TOLERANCE);
}

const FIELDS = ['top', 'left', 'width', 'height'] as const;

/**
 * The same region must be visible on both sides before its geometry means
 * anything — a swap that hid the chrome on one side only would leave every
 * box comparing zeros against a rectangle. Hidden regions occupy no box, so
 * both hiding together is parity too.
 */
function sameBox(
  label: string,
  skeleton: Box,
  live: Box,
  fields: readonly string[] = FIELDS
): void {
  expect(
    skeleton.visible,
    `${label}: skeleton ${skeleton.visible ? 'occupies' : 'occupies no'} box, ` +
      `live ${live.visible ? 'occupies' : 'occupies no'} one`
  ).toBe(live.visible);
  if (!skeleton.visible) return;
  for (const field of fields) {
    within(skeleton[field], live[field], `${label}: ${field}`);
  }
}

/**
 * The regions the criterion names, as [label, skeleton selector, live
 * selector, compared fields]. The two sides' selectors differ only where the
 * stand-in and the content are different elements by design (the canvas bar
 * vs #replay-canvas, the scrubber bar vs the range input).
 */
const REGIONS: ReadonlyArray<readonly [
  label: string,
  skeletonSelector: string,
  liveSelector: string,
  fields: readonly string[]
]> = [
  // The shared box both sides cap at — its width is the only thing a redesign
  // could move without moving both sides together.
  ['page', '.replay-page', '.replay-page', ['width']],
  ['title', '.replay-page > h1.page-title', '.replay-page > h1.page-title', ['top', 'width', 'height']],
  ['main column', '.replay-main', '.replay-main', ['top', 'left', 'width']],
  // The video surface: the stand-in tracks the attribute-default canvas's
  // 2:1 box, so the region the page exists for must not move or resize.
  ['canvas region', '.canvas-wrapper > .skeleton-bar', '.canvas-wrapper > canvas', FIELDS],
  ['canvas wrapper', '.canvas-wrapper', '.canvas-wrapper', ['top', 'left', 'width']],
  // The phone chrome, and the two blocks inside it the stand-ins mirror.
  ['mobile controls', '.mobile-replay-controls', '.mobile-replay-controls', FIELDS],
  ['playback bar', '.mobile-playback-bar', '.mobile-playback-bar', FIELDS],
  [
    'scrubber',
    '.mobile-replay-controls > .skeleton-bar',
    '.mobile-replay-controls > input[type="range"]',
    FIELDS,
  ],
  ['event timeline', '.mobile-event-timeline', '.mobile-event-timeline', FIELDS],
  // Content-sized live panels over fixed-height stand-ins (see
  // skeletonReplay's derivation notes), so the column's x/width are the
  // contract; its top joins them at the widths that stack the columns and
  // keep the two main columns content-identical.
  ['sidebar', '.replay-sidebar', '.replay-sidebar', ['left', 'width']],
];

/** Widths that cross mobile.css's phone / tablet / desktop blocks. */
const VIEWPORT_WIDTHS = [375, 768, 1280] as const;

function buildReplayFixtureHtml(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ACB replay swap-parity fixture</title>
  <style>${inlineStyles()}</style>
</head>
<body>
  <section id="skeleton-fixture" aria-label="skeleton replay viewer">
    ${skeletonReplay()}
  </section>

  <section id="live-fixture" aria-label="live replay viewer fixture">
    ${replayPageMarkup()}
  </section>
</body>
</html>`;
}

async function openReplayFixture(page: Page): Promise<void> {
  await page.setContent(buildReplayFixtureHtml(), { waitUntil: 'load' });
}

test.describe('replay skeleton → content swap parity', () => {
  for (const width of VIEWPORT_WIDTHS) {
    test(`the four replay regions hang in the same boxes at ${width}px`, async ({ page }) => {
      // Each test drives its own viewport, so one project's pass covers the
      // matrix; the second would only repeat it.
      test.skip(
        test.info().project.name === 'phone',
        'drives its own three viewports — one pass is enough'
      );

      await page.setViewportSize({ width, height: 900 });
      await openReplayFixture(page);

      for (const [label, skeletonSelector, liveSelector, fields] of REGIONS) {
        const [skeleton, live] = await Promise.all([
          boxWithinSection(page, skeletonSelector, 'skeleton-fixture'),
          boxWithinSection(page, liveSelector, 'live-fixture'),
        ]);
        sameBox(label, skeleton, live, fields);
      }

      // Wherever the two sides stack a content-identical main column — phone
      // (the square wrapper cannot grow) and desktop (the columns sit side by
      // side, so both tops are the layout's) — the sidebar hangs at the same
      // y too. The 640-900px stack is the exception; see the header note.
      if (width === 375 || width === 1280) {
        const [skeletonSidebar, liveSidebar] = await Promise.all([
          boxWithinSection(page, '.replay-sidebar', 'skeleton-fixture'),
          boxWithinSection(page, '.replay-sidebar', 'live-fixture'),
        ]);
        within(skeletonSidebar.top, liveSidebar.top, 'sidebar: top');
      }
    });
  }
});
