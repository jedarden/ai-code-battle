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
 * blocks are document-global once the markup lands, so they can only style the
 * skeleton too: replayPageMarkup's block is therefore held to rules the
 * skeleton does not render (see skeleton.test.ts's "declares nothing the
 * skeleton answers to"), and the layout the two sides share lives in the
 * stylesheets alone.
 *
 * The second fixture exists because same-document parity cannot see a rule
 * that both sections answer to. Before the layout moved into the stylesheets,
 * replayPageMarkup carried its own max-width: 900px stacking block: applied
 * document-globally it styled the skeleton section of this fixture as well, so
 * both sides stacked at 768px and parity held here — while in the real page
 * the skeleton had already been laid out by the stylesheets alone, two-column
 * in the 640-900px band, and the swap dropped everything below the canvas.
 * `pre-swap cascade vs live page` measures that real pair: the skeleton in a
 * document holding the stylesheets alone, the live page in one holding the
 * page's own blocks too.
 *
 * Like the other parity specs, every geometry assertion is relative — skeleton
 * vs live, each measured within its own fixture section — so a redesign that
 * moves both sides together stays green, and only a swap that actually shifts
 * content bites.
 *
 * The swap's fade is measured here rather than held as text because the two
 * half-fades are behaviour: the skeleton must go out through a real 150ms
 * transition and the content in through the shared 150ms animation, animating
 * opacity alone. An animated property that lays out a box moves the page
 * mid-fade — which the geometry assertions cannot see, since they measure
 * after the swap has settled (skeleton.test.ts's "ships the fade the swap
 * runs" holds the shipped CSS text this run depends on).
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

/**
 * The two documents the real swap moves between, one section each.
 *
 * Pre-swap is what the route renders first: skeletonReplay() under the three
 * stylesheets and nothing else. Post-swap is what the swap writes: the live
 * markup, whose page-owned <style> blocks are document-global from that
 * moment. Holding the skeleton alone in the first document is the point —
 * in the same-document fixture above, those blocks style the skeleton section
 * too, so a page-owned rule that reaches a class the skeleton renders is
 * invisible here and shifts the real swap all the same.
 */
function buildPreSwapHtml(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ACB replay pre-swap cascade fixture</title>
  <style>${inlineStyles()}</style>
</head>
<body>
  <section id="skeleton-fixture" aria-label="skeleton replay viewer">
    ${skeletonReplay()}
  </section>
</body>
</html>`;
}

function buildPostSwapHtml(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ACB replay post-swap cascade fixture</title>
  <style>${inlineStyles()}</style>
</head>
<body>
  <section id="live-fixture" aria-label="live replay viewer fixture">
    ${replayPageMarkup()}
  </section>
</body>
</html>`;
}

async function openReplayFixture(page: Page, html: string): Promise<void> {
  await page.setContent(html, { waitUntil: 'load' });
}

/** Which side of REGIONS a section measures: the stand-ins or the content. */
type Side = 'skeleton' | 'live';

/** Every compared region of one fixture section, by its label in REGIONS. */
async function measureRegions(page: Page, side: Side): Promise<Record<string, Box>> {
  const section = side === 'skeleton' ? 'skeleton-fixture' : 'live-fixture';
  const boxes: Record<string, Box> = {};
  for (const [label, skeletonSelector, liveSelector] of REGIONS) {
    const selector = side === 'skeleton' ? skeletonSelector : liveSelector;
    boxes[label] = await boxWithinSection(page, selector, section);
  }
  return boxes;
}

function compareRegions(skeleton: Record<string, Box>, live: Record<string, Box>): void {
  for (const [label, , , fields] of REGIONS) {
    sameBox(label, skeleton[label], live[label], fields);
  }
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
      await openReplayFixture(page, buildReplayFixtureHtml());

      compareRegions(
        await measureRegions(page, 'skeleton'),
        await measureRegions(page, 'live')
      );

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

    test(`the swap moves nothing between the pre- and post-swap cascades at ${width}px`, async ({ page }) => {
      test.skip(
        test.info().project.name === 'phone',
        'drives its own three viewports — one pass is enough'
      );

      await page.setViewportSize({ width, height: 900 });

      // Measured before the document is replaced: the skeleton under the
      // stylesheets alone is what the route shows first, and the live page
      // under its own blocks is what replaces it.
      await openReplayFixture(page, buildPreSwapHtml());
      const preSwap = await measureRegions(page, 'skeleton');

      await openReplayFixture(page, buildPostSwapHtml());
      compareRegions(preSwap, await measureRegions(page, 'live'));

      // Same exception as the same-document sweep above: wherever the columns
      // stack and the main column is content-sized, the live sidebar hangs
      // lower — its x and width are the layout's and are compared in full.
      if (width === 375 || width === 1280) {
        await openReplayFixture(page, buildPreSwapHtml());
        const preSwapSidebar = await boxWithinSection(page, '.replay-sidebar', 'skeleton-fixture');
        await openReplayFixture(page, buildPostSwapHtml());
        const liveSidebar = await boxWithinSection(page, '.replay-sidebar', 'live-fixture');
        within(preSwapSidebar.top, liveSidebar.top, 'sidebar: top');
      }
    });
  }
});

test.describe('replay skeleton → content swap fade', () => {
  // §16.14: the swap is a 150ms cross-fade, not a cut. The skeleton side goes
  // out through .skeleton-page's opacity transition — the transition
  // renderReplayPage drives by setting the inline opacity and waiting it out —
  // and the content side comes in on .fade-in, the shared animation the
  // leaderboard and bot-profile swaps carry, which starts on insertion.
  test('the skeleton fades out through 150ms and the content fades in on opacity alone', async ({ page }) => {
    test.skip(
      test.info().project.name === 'phone',
      'the fade is viewport-independent — one pass is enough'
    );

    await page.setViewportSize({ width: 1280, height: 900 });
    await openReplayFixture(page, buildPreSwapHtml());

    // First half, exactly as replay.ts drives it: set the inline opacity, and
    // a transition — not a snap — must start on it.
    const fadeOut = await page.evaluate(() => {
      const skeleton = document.querySelector<HTMLElement>('.skeleton-page');
      if (!skeleton) throw new Error('no .skeleton-page in the pre-swap fixture');
      skeleton.style.opacity = '0';
      // Flushing style is what starts the transition: until the browser
      // recalculates, the before/after values are not both known and there is
      // nothing for getAnimations() to report.
      void getComputedStyle(skeleton).opacity;
      const transition = skeleton
        .getAnimations()
        .find(
          (a): a is CSSTransition =>
            a instanceof CSSTransition && a.transitionProperty === 'opacity'
        );
      if (!transition) {
        throw new Error('setting the skeleton opacity started no opacity transition');
      }
      return transition.effect!.getTiming().duration;
    });
    expect(fadeOut, 'the skeleton must fade out over the same 150ms the content fades in').toBe(150);

    // Second half: the swap itself. The live markup is what the route writes
    // into #app, page-owned <style> blocks included, and .fade-in must already
    // be running on it — no rAF handshake, the way the swap no longer does it.
    const fadeIn = await page.evaluate((LIVE) => {
      const section = document.getElementById('skeleton-fixture');
      if (!section) throw new Error('no #skeleton-fixture in the pre-swap fixture');
      section.innerHTML = LIVE;
      const live = document.querySelector<HTMLElement>('.replay-page');
      if (!live) throw new Error('the swapped-in markup carries no .replay-page');
      const fade = live
        .getAnimations()
        .find((a): a is CSSAnimation => a instanceof CSSAnimation && a.animationName === 'fade-in');
      if (!fade) throw new Error('the fade-in animation is not running on the swapped-in page');
      const timing = fade.effect!.getTiming();
      const frames = (fade.effect as KeyframeEffect).getKeyframes();
      // getKeyframes() also reports computed bookkeeping fields alongside the
      // properties the keyframes actually animate.
      const bookkeeping = new Set(['offset', 'computedOffset', 'composite', 'easing']);
      return {
        duration: timing.duration,
        iterations: timing.iterations,
        frames: frames.map((frame) => ({
          opacity: frame.opacity === undefined ? null : String(frame.opacity),
          animatedProps: Object.keys(frame).filter((prop) => !bookkeeping.has(prop)),
        })),
      };
    }, replayPageMarkup());

    expect(fadeIn.duration, '§16.14: the fade-in runs over 150ms').toBe(150);
    expect(fadeIn.iterations).toBe(1);
    expect(fadeIn.frames.map((frame) => frame.opacity)).toEqual(['0', '1']);
    for (const frame of fadeIn.frames) {
      expect(
        frame.animatedProps,
        'the fade may animate opacity only — anything else shifts layout'
      ).toEqual(['opacity']);
    }
  });
});
