/**
 * Mobile half of the leaderboard skeleton parity criterion (§16.14).
 *
 * skeletonLeaderboard()'s placeholder cards promise the space the live
 * renderMobileCard() cards occupy once the data lands, so the skeleton-to-
 * content swap on a phone causes no layout shift. The jsdom guards in
 * src/components/skeleton.test.ts can only hold that promise against the CSS
 * *text*; here both renderings go through a real layout engine side by side
 * (see ./fixture.ts, which lays the real renderer's markup out next to the
 * skeleton under the app stylesheets) at the criterion's 375px viewport and
 * the card boxes are compared as actually measured: per-card height, the
 * toggle row that drives it, and the container's shape and list role.
 *
 * Measuring matters more here than on the desktop half: the live toggle is a
 * <button>, so base.css's 44px tap-target floor is in play against whatever
 * the text metrics resolve to — a quantity no CSS-text parse can see.
 *
 * Like the desktop spec, every assertion is relative — skeleton vs live — and
 * pins no card height of its own: both stacks consume the same
 * .leaderboard-mobile-* rules, so a redesign that retunes the card moves the
 * two together and stays green. What this spec bites on is one side leaving
 * the shared geometry — a retuned placeholder height, a wrapper class typo,
 * a role dropped from either container.
 */

import { expect, test } from '@playwright/test';
import { openFixture } from './fixture';
import { measure, measureAll, readAttrs } from './measure';

// Sub-pixel allowance, same rationale as the desktop spec: rounding at
// fractional DPR is not the drift this spec exists for — a retuned placeholder
// height moves a card by multiples of a text line, not fractions.
const PX_TOLERANCE = 1;

// The skeleton renders 8 cards (its CARD_COUNT); the live fixture renders 3.
// Pairwise comparison runs over the live count — the shared geometry is per
// card, not per stack.
const LIVE_CARD_COUNT = 3;

const SKELETON_CONTAINER = '#skeleton-fixture #lb-mobile';
const LIVE_CONTAINER = '#live-lb-mobile';
const SKELETON_CARDS = `${SKELETON_CONTAINER} .leaderboard-mobile-card`;
const LIVE_CARDS = `${LIVE_CONTAINER} .leaderboard-mobile-card`;

test.skip(() => test.info().project.name === 'desktop', 'mobile (375px) criterion only');

// The criterion is stated at 375px. The shared 'phone' project sits at 390px,
// which proves the same media blocks but is not the width the criterion names,
// so this spec pins its own viewport rather than moving the project.
test.use({ viewport: { width: 375, height: 667 } });

function within(actual: number, expected: number, label: string): void {
  expect(
    Math.abs(actual - expected),
    `${label}: ${actual.toFixed(2)}px vs ${expected.toFixed(2)}px`
  ).toBeLessThanOrEqual(PX_TOLERANCE);
}

test.describe('mobile leaderboard skeleton-vs-live parity', () => {
  test('the containers swap shape and list role together at 375px', async ({ page }) => {
    await openFixture(page);
    const [skeleton, live] = await Promise.all([
      measure(page, SKELETON_CONTAINER),
      measure(page, LIVE_CONTAINER),
    ]);

    // Both stacks answer to the same (max-width: 639px) .mobile-cards block.
    expect(skeleton.display, 'skeleton container must actually show at 375px').toBe('flex');
    expect(live.display, 'live container must actually show at 375px').toBe('flex');

    // The attribute the swap replaces: renderLeaderboard renders #lb-mobile
    // with role="list", so the skeleton's container must hand the exact same
    // role back when the content lands.
    const [skeletonAttrs, liveAttrs] = await Promise.all([
      readAttrs(page, SKELETON_CONTAINER, ['role', 'class']),
      readAttrs(page, LIVE_CONTAINER, ['role', 'class']),
    ]);
    expect(skeletonAttrs['role']).toBe('list');
    expect(liveAttrs['role']).toBe('list');
    expect(skeletonAttrs['class']).toBe(liveAttrs['class']);
  });

  test('each skeleton card occupies the same space as the live card', async ({ page }) => {
    await openFixture(page);
    const skeletonCards = await measureAll(page, SKELETON_CARDS);
    const liveCards = await measureAll(page, LIVE_CARDS);

    expect(skeletonCards, 'skeleton must render its card stack').toHaveLength(8);
    expect(liveCards, 'live fixture must render its card stack').toHaveLength(LIVE_CARD_COUNT);
    for (const card of [...skeletonCards, ...liveCards]) {
      expect(card.display, 'card must actually render at 375px').not.toBe('none');
    }

    // The placeholders are identical rows, so their cards must be identical
    // boxes — one odd card is a placeholder typo, not a redesign.
    for (const card of skeletonCards.slice(1)) {
      within(card.height, skeletonCards[0].height, 'skeleton cards must be uniform');
    }

    // Collapsed card for collapsed card: the swap replaces each placeholder
    // with a real card of the same height and nothing below it moves.
    for (let i = 0; i < LIVE_CARD_COUNT; i++) {
      within(skeletonCards[i].height, liveCards[i].height, `card ${i + 1} height`);
    }
  });

  test('the toggle stand-in matches the live toggle the card height is made of', async ({ page }) => {
    await openFixture(page);
    // A collapsed card is its padding plus its toggle row (the details block is
    // max-height:0 on both sides), so the toggle is where a height drift shows.
    const skeletonToggles = await measureAll(page, `${SKELETON_CARDS} > .mobile-card-toggle`);
    const liveToggles = await measureAll(page, `${LIVE_CARDS} > .mobile-card-toggle`);
    expect(skeletonToggles).toHaveLength(8);
    expect(liveToggles).toHaveLength(LIVE_CARD_COUNT);

    for (let i = 0; i < LIVE_CARD_COUNT; i++) {
      within(skeletonToggles[i].height, liveToggles[i].height, `card ${i + 1} toggle height`);
    }

    // The collapsed details contribute zero on both sides — if either side ever
    // opens by default, the per-card comparison above is no longer about the
    // collapsed swap and this makes the cause obvious.
    for (const [side, selector, count] of [
      ['skeleton', SKELETON_CARDS, 8],
      ['live', LIVE_CARDS, LIVE_CARD_COUNT],
    ] as const) {
      const blocks = await measureAll(page, `${selector} .leaderboard-mobile-details`);
      expect(blocks, `${side} must render a details block per card`).toHaveLength(count);
      for (const block of blocks) {
        expect(block.height, `${side} details must stay collapsed`).toBe(0);
      }
    }
  });
});
