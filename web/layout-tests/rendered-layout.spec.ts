/**
 * Smoke tests for the rendered-layout harness itself.
 *
 * These prove the harness measures real layout — nothing more. Assertions are
 * tied to known stylesheet quantities (the 40px calibration element,
 * --lb-row-min-height, --lb-col-wl, the .mobile-cards visibility blocks), and
 * deliberately include no skeleton-vs-live comparisons: parity children own
 * those and should build them on measure()/measureAll() from ./measure.
 *
 * Why this cannot live in the vitest suite: jsdom has no layout engine, so
 * every getBoundingClientRect() there returns zeros and every media query is
 * unevaluated. A passing width assertion here is therefore proof the number
 * came out of a real rendering engine.
 */

import { expect, test } from '@playwright/test';
import { openFixture } from './fixture';
import { measure, measureAll } from './measure';

// The two projects defined in ../playwright.config.ts. TestInfo exposes the
// project as `.project` (a FullProject) — there is no `.projectName` property,
// and comparing against it would make this silently always-false, taking the
// desktop branch on the phone project too.
const IS_PHONE = () => test.info().project.name === 'phone';

test('calibration: a known 40px element measures exactly 40px wide', async ({ page }) => {
  await openFixture(page);
  // A bare div at width:40px under base.css's box-sizing:border-box reset —
  // 40.00 in Chromium, 0 in jsdom.
  const rect = await measure(page, '#calibration-40');
  expect(rect.display).toBe('block');
  expect(rect.width).toBe(40);
});

test('measures skeletonLeaderboard() output in the rendered layout', async ({ page }) => {
  await openFixture(page);
  // skeletonLeaderboard() renders 15 rows into #lb-desktop (skeleton.ts).
  const rows = await measureAll(page, '#skeleton-fixture #lb-desktop .lb-row');
  expect(rows.length).toBe(15);
  for (const row of rows) {
    expect(row.display).not.toBe('none');
    // .lb-row carries min-height: var(--lb-row-min-height) (48px).
    expect(row.height).toBeGreaterThanOrEqual(48);
    expect(row.width).toBeGreaterThan(0);
  }
});

test('measures the live .lb-row / .mobile-cards fixture', async ({ page }) => {
  await openFixture(page);
  const row = await measure(page, '#live-lb-desktop .lb-row');
  expect(row.display).not.toBe('none');
  expect(row.height).toBeGreaterThanOrEqual(48);
  expect(row.width).toBeGreaterThan(0);

  const cards = await measureAll(page, '#live-lb-mobile .leaderboard-mobile-card');
  expect(cards.length).toBe(3);

  // .mobile-cards visibility comes from mobile.css's media blocks — flex on
  // phones (max-width: 639px), display:none !important on desktop
  // (min-width: 1024px). jsdom cannot evaluate either rule.
  const mobile = await measure(page, '#live-lb-mobile');
  expect(mobile.display).toBe(IS_PHONE() ? 'flex' : 'none');
});

test('applies display rules to hidden skeleton columns', async ({ page }) => {
  await openFixture(page);
  // The wl bar reuses the live .lb-wl class, so the (max-width: 768px) rule
  // hiding that column reaches the skeleton too. Off-phone it keeps its
  // var(--lb-col-wl) width — 60px measured, not parsed.
  const wl = await measure(page, '#skeleton-fixture #lb-desktop .lb-row .lb-wl');
  if (IS_PHONE()) {
    expect(wl.display).toBe('none');
    expect(wl.width).toBe(0);
  } else {
    expect(wl.display).not.toBe('none');
    expect(wl.width).toBe(60);
  }
});
