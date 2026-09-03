/**
 * Page-level half of the leaderboard skeleton criterion (§16.14 #2): the
 * skeleton → content transition itself.
 *
 * The per-row and per-card halves (leaderboard-desktop-parity /
 * leaderboard-mobile-parity) prove the placeholders measure the same as the
 * content they stand in for. This spec proves the swap: the header block
 * above the first row hangs in exactly the same place on both sides, so no
 * row or card moves when the data lands, and the content that arrives does
 * it through the shared 150ms opacity fade. The jsdom guards
 * (src/pages/leaderboard.test.ts) hold the same contract against the CSS
 * text; here both sides go through a real layout engine (see ./fixture.ts).
 *
 * Like the other parity specs, every geometry assertion is relative —
 * skeleton vs live, each measured within its own fixture section — so a
 * redesign that moves both sides together stays green, and only a swap that
 * actually shifts content bites.
 */

import { expect, test } from '@playwright/test';
import { openFixture, inlineStyles } from './fixture';

// Sub-pixel allowance, same rationale as the desktop spec: rounding at
// fractional DPR is not the drift this spec exists for — a header mismatch
// moves every row by multiples of a text line, not fractions.
const PX_TOLERANCE = 1;

/**
 * The `top` of the first element matching `selector`, measured within its
 * fixture section. Both sections are unstyled blocks in the same document,
 * so within-section offsets are what the swap actually compares: the skeleton
 * section and the live section sit at different absolute y positions.
 */
async function topWithinSection(
  page: import('@playwright/test').Page,
  selector: string,
  sectionId: string
): Promise<number> {
  return page.evaluate(
    ({ sel, section }) => {
      const el = document.querySelector(sel);
      const sec = document.getElementById(section);
      if (!el || !sec) throw new Error(`missing "${sel}" or #${section}`);
      return el.getBoundingClientRect().top - sec.getBoundingClientRect().top;
    },
    { sel: selector, section: sectionId }
  );
}

/** Box fields of the first element matching `selector`, within its section. */
async function boxWithinSection(
  page: import('@playwright/test').Page,
  selector: string,
  sectionId: string
): Promise<{ top: number; width: number; height: number }> {
  return page.evaluate(
    ({ sel, section }) => {
      const el = document.querySelector(sel);
      const sec = document.getElementById(section);
      if (!el || !sec) throw new Error(`missing "${sel}" or #${section}`);
      const rect = el.getBoundingClientRect();
      const secRect = sec.getBoundingClientRect();
      return {
        top: rect.top - secRect.top,
        width: rect.width,
        height: rect.height,
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

test.describe('leaderboard skeleton → content swap parity', () => {
  test('both pages occupy the same width box', async ({ page }) => {
    await openFixture(page);
    const [skeletonPage, livePage] = await Promise.all([
      boxWithinSection(page, '#skeleton-fixture .skeleton-page', 'skeleton-fixture'),
      boxWithinSection(page, '#live-fixture .leaderboard-page', 'live-fixture'),
    ]);
    within(skeletonPage.width, livePage.width, 'page width');
  });

  test('the first row hangs at the same y under both headers', async ({ page }) => {
    await openFixture(page);
    const [skeletonRow, liveRow] = await Promise.all([
      topWithinSection(page, '#skeleton-fixture .lb-row', 'skeleton-fixture'),
      topWithinSection(page, '#live-fixture .lb-row', 'live-fixture'),
    ]);
    within(skeletonRow, liveRow, 'first row top');
  });

  test('every header box measures the same on both sides', async ({ page }) => {
    await openFixture(page);
    for (const [label, skeletonSel, liveSel] of [
      ['h1', '#skeleton-fixture .skeleton-page > h1', '#live-fixture .leaderboard-page > h1'],
      ['updated-at', '#skeleton-fixture .updated-at', '#live-fixture .updated-at'],
      ['lb-hint', '#skeleton-fixture .lb-hint', '#live-fixture .lb-hint'],
    ] as const) {
      const [skeleton, live] = await Promise.all([
        boxWithinSection(page, skeletonSel, 'skeleton-fixture'),
        boxWithinSection(page, liveSel, 'live-fixture'),
      ]);
      within(skeleton.top, live.top, `${label}: top`);
      within(skeleton.width, live.width, `${label}: width`);
      within(skeleton.height, live.height, `${label}: height`);
    }
  });

  test('the first card hangs the same distance below the desktop block', async ({ page }) => {
    // Cards only render below the (max-width: 1023px) blocks, so this is the
    // phone half of the swap; the desktop project's viewport hides them.
    test.skip(test.info().project.name === 'desktop', 'mobile cards criterion only');

    await openFixture(page);
    // The stacks have different row counts above the cards (15 placeholders
    // vs 3 live rows), so the shift-free quantity is the gap between the
    // desktop block and the first card — not the card's absolute position.
    const gap = async (sectionId: string, cardSel: string, desktopSel: string) => {
      return page.evaluate(
        ({ section, card, desktop }) => {
          const sec = document.getElementById(section);
          const cardEl = document.querySelector(card);
          const desktopEl = document.querySelector(desktop);
          if (!sec || !cardEl || !desktopEl) {
            throw new Error(`missing #${section}, "${card}" or "${desktop}"`);
          }
          return (
            cardEl.getBoundingClientRect().top - desktopEl.getBoundingClientRect().bottom
          );
        },
        { section: sectionId, card: cardSel, desktop: desktopSel }
      );
    };
    const [skeletonGap, liveGap] = await Promise.all([
      gap('skeleton-fixture', '#skeleton-fixture .leaderboard-mobile-card', '#skeleton-fixture #lb-desktop'),
      gap('live-fixture', '#live-fixture .leaderboard-mobile-card', '#live-fixture #live-lb-desktop'),
    ]);
    within(skeletonGap, liveGap, 'desktop block → first card gap');
  });

  test('the swapped-in page fades in over 150ms of opacity only', async ({ page }) => {
    await page.setContent(
      `<!DOCTYPE html><html><head><style>${inlineStyles()}</style></head>
       <body><div class="leaderboard-page fade-in">Leaderboard</div></body></html>`,
      { waitUntil: 'load' }
    );

    const anim = await page.evaluate(() => {
      const el = document.querySelector<HTMLElement>('.leaderboard-page');
      if (!el) throw new Error('no .leaderboard-page in the measurement document');
      const fade = el
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
          offset: frame.offset,
          opacity: frame.opacity === undefined ? null : String(frame.opacity),
          animatedProps: Object.keys(frame).filter((prop) => !bookkeeping.has(prop)),
        })),
      };
    });

    expect(anim.duration, '§16.14: the fade-in runs over 150ms').toBe(150);
    expect(anim.iterations).toBe(1);
    expect(anim.frames.map((frame) => frame.opacity)).toEqual(['0', '1']);
    for (const frame of anim.frames) {
      expect(
        frame.animatedProps,
        'the fade may animate opacity only — anything else shifts layout'
      ).toEqual(['opacity']);
    }
  });
});
