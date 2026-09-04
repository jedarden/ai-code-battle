/**
 * Bot-profile skeleton parity (§16.14): the skeletonBotProfile() → rendered
 * profile transition must not move the page.
 *
 * The jsdom guards in src/components/skeleton.test.ts hold the skeleton's
 * geometry against the CSS *text*; this spec holds it against the layout the
 * two sides actually produce (see ./bot-profile-fixture.ts, which lays
 * skeletonBotProfile() out next to renderProfileMarkup's output under the app
 * stylesheets, in headless Chromium). The swap replaces app.innerHTML
 * wholesale, so "no layout shift" is a claim about positions, not just box
 * heights: the breadcrumb row, the header, and every section below them are
 * measured on both sides — per-section height, and top offset within the page
 * — at the criterion's 375px and 1280px viewports.
 *
 * Every assertion is relative — skeleton vs live, each measured within its own
 * fixture section — so a redesign that moves both sides together stays green,
 * and only a swap that actually shifts content bites.
 */

import { expect, test } from '@playwright/test';
import { openBotProfileFixture } from './bot-profile-fixture';
import type { Page } from '@playwright/test';

// Sub-pixel allowance, same rationale as the leaderboard parity specs:
// rounding at fractional DPR is not the drift this spec exists for — a
// placeholder that misses its content moves a section by multiples of a text
// line, not fractions.
const PX_TOLERANCE = 1;

/**
 * The criterion is stated at 375px and 1280px. The shared projects sit at 390
 * and 1280, which prove the same media blocks but are not the widths the
 * criterion names, so this spec pins its own viewports rather than moving the
 * projects.
 */
const CRITERION_VIEWPORTS: Record<string, { width: number; height: number }> = {
  phone: { width: 375, height: 667 },
  desktop: { width: 1280, height: 800 },
};

test.beforeEach(async ({ page }, testInfo) => {
  const viewport = CRITERION_VIEWPORTS[testInfo.project.name];
  expect(viewport, `unknown project ${testInfo.project.name}`).toBeTruthy();
  await page.setViewportSize(viewport);
});

/**
 * The blocks the swap renders, in document order, with the selectors that find
 * them inside each side's .bot-profile-page. The breadcrumb is first — it is
 * part of the swap's innerHTML, so its row is measured like any other block.
 */
const BLOCKS = [
  ['breadcrumb', '.breadcrumb'],
  ['header', '.profile-header'],
  ['ratings', '.profile-section.ratings'],
  ['stats', '.profile-section.stats'],
  ['meta', '.profile-section.meta'],
  ['rivals', '.profile-section.rivals'],
  ['history', '.lazy-section'],
] as const;

function within(actual: number, expected: number, label: string): void {
  expect(
    Math.abs(actual - expected),
    `${label}: ${actual.toFixed(2)}px vs ${expected.toFixed(2)}px`
  ).toBeLessThanOrEqual(PX_TOLERANCE);
}

/**
 * Height and top-of-block (within its own .bot-profile-page) for every block
 * the swap renders, read off one side of the fixture.
 */
async function measureBlocks(
  page: Page,
  pageRootSelector: string
): Promise<Record<string, { height: number; top: number }>> {
  return page.evaluate((rootSel: string) => {
    const root = document.querySelector(rootSel);
    if (!root) throw new Error(`no element matches "${rootSel}"`);
    const origin = root.getBoundingClientRect().top;
    const selectors: [string, string][] = [
      ['breadcrumb', '.breadcrumb'],
      ['header', '.profile-header'],
      ['ratings', '.profile-section.ratings'],
      ['stats', '.profile-section.stats'],
      ['meta', '.profile-section.meta'],
      ['rivals', '.profile-section.rivals'],
      ['history', '.lazy-section'],
    ];
    const out: Record<string, { height: number; top: number }> = {};
    for (const [name, sel] of selectors) {
      const el = root.querySelector(sel);
      if (!el) throw new Error(`no ${sel} inside "${rootSel}"`);
      const rect = el.getBoundingClientRect();
      out[name] = {
        height: rect.height,
        top: rect.top - origin,
      };
    }
    return out;
  }, pageRootSelector);
}

test.describe('bot-profile skeleton → content swap parity', () => {
  test('both pages occupy the same width box', async ({ page }) => {
    await openBotProfileFixture(page);
    const [skeleton, live] = await Promise.all([
      page.evaluate(() => {
        const el = document.querySelector('#skeleton-fixture .bot-profile-page');
        if (!el) throw new Error('no skeleton .bot-profile-page');
        return el.getBoundingClientRect().width;
      }),
      page.evaluate(() => {
        const el = document.querySelector('#live-profile-root');
        if (!el) throw new Error('no #live-profile-root');
        return el.getBoundingClientRect().width;
      }),
    ]);
    within(skeleton, live, 'page width');
  });

  test('each block measures the same height as the content it stands in for', async ({ page }) => {
    await openBotProfileFixture(page);
    const [skeleton, live] = await Promise.all([
      measureBlocks(page, '#skeleton-fixture .bot-profile-page'),
      measureBlocks(page, '#live-profile-root'),
    ]);
    for (const [name] of BLOCKS) {
      within(skeleton[name].height, live[name].height, `${name} height`);
    }
  });

  test('the swap moves no block on the page', async ({ page }) => {
    await openBotProfileFixture(page);
    const [skeleton, live] = await Promise.all([
      measureBlocks(page, '#skeleton-fixture .bot-profile-page'),
      measureBlocks(page, '#live-profile-root'),
    ]);
    for (const [name] of BLOCKS) {
      within(skeleton[name].top, live[name].top, `${name} top`);
    }
  });

  test('the collapsed sections hold the toggle row alone on both sides', async ({ page }) => {
    // The criterion calls out "single collapsed bar rows" for meta and rivals:
    // their .section-content boxes must be collapsed on both sides, so the
    // section is exactly its toggle row high. A skeleton that left them
    // expanded (or a live change that opened them) would measure wrong in the
    // block test above only if the drift crossed the tolerance — this pins the
    // collapse itself.
    await openBotProfileFixture(page);
    const heights = await page.evaluate(() => {
      const read = (rootSel: string, sel: string) => {
        const root = document.querySelector(rootSel);
        const el = root?.querySelector(sel);
        if (!root || !el) throw new Error(`no ${sel} inside "${rootSel}"`);
        return el.getBoundingClientRect().height;
      };
      return {
        skeletonMetaContent: read('#skeleton-fixture .profile-section.meta', '.section-content'),
        liveMetaContent: read('#live-profile-root .profile-section.meta', '.section-content'),
        skeletonRivalsContent: read('#skeleton-fixture .profile-section.rivals', '.section-content'),
        liveRivalsContent: read('#live-profile-root .profile-section.rivals', '.section-content'),
      };
    });
    for (const [label, value] of Object.entries(heights)) {
      expect(value, `${label} must be collapsed`).toBeLessThanOrEqual(PX_TOLERANCE);
    }
  });

  test('the history block is the 80px placeholder on both sides', async ({ page }) => {
    await openBotProfileFixture(page);
    const heights = await page.evaluate(() => {
      const read = (rootSel: string) => {
        const root = document.querySelector(rootSel);
        const el = root?.querySelector('.lazy-placeholder');
        if (!root || !el) throw new Error(`no .lazy-placeholder inside "${rootSel}"`);
        return el.getBoundingClientRect().height;
      };
      return { skeleton: read('#skeleton-fixture'), live: read('#live-profile-root') };
    });
    expect(heights.skeleton).toBe(80);
    expect(heights.live).toBe(80);
  });
});
