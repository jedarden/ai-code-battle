/**
 * Desktop half of the leaderboard skeleton parity criterion (§16.14).
 *
 * skeletonLeaderboard()'s placeholder bars promise the columns the live
 * renderDesktopRow() row draws once the data lands. The jsdom guards in
 * src/components/skeleton.test.ts can only hold that promise against the CSS
 * *text*; here both renderings go through a real layout engine side by side
 * (see ./fixture.ts, which lays the real renderer's markup out next to the
 * skeleton under the app stylesheets) and the bars are held against the
 * columns as actually measured: left edge and width of each of the seven,
 * plus the row geometry the shared .lb-row rule resolves to.
 *
 * Every assertion is relative — skeleton vs live — and pins no column width of
 * its own. That is deliberate: both rows consume the same --lb-col-*
 * declarations on .lb-row, so a redesign that retunes a column width moves the
 * two together and stays green (the harness smoke spec is where a measured
 * absolute belongs). What this spec bites on is one side no longer consuming
 * the shared declaration — a stale hardcoded bar width, a renamed var, a
 * wrapper whose width no longer matches the skeleton's — i.e. exactly the
 * rendered drift the text guards cannot see. The one absolute it carries is
 * the row floor: min-height must still resolve to the 48px the shared rule
 * declares, on both rows.
 */

import { expect, test } from '@playwright/test';
import { openFixture } from './fixture';
import { measure, measureAll, readStyles } from './measure';

// Sub-pixel allowance. The rows lay out in whole pixels at the desktop
// viewport, but rounding at fractional zoom/DPR is not the drift this spec
// exists for — a stale bar width moves a column by tens of px, not fractions.
const PX_TOLERANCE = 1;

// Desktop columns in renderDesktopRow order: rank, name, rating, wl,
// winrate, status, expand. Same order the bars are emitted in (skeleton.ts).
const COLUMN_NAMES = ['rank', 'name', 'rating', 'wl', 'winrate', 'status', 'expand'] as const;

// Row-level declarations from the shared .lb-row rule whose resolved values
// must come out identical on both rows.
const ROW_GEOMETRY_PROPS = [
  'display',
  'gap',
  'padding-top',
  'padding-right',
  'padding-bottom',
  'padding-left',
  'min-height',
] as const;

const SKELETON_ROW = '#skeleton-fixture #lb-desktop .lb-row:first-child';
const LIVE_ROW = '#live-lb-desktop .lb-row:first-child';

function within(actual: number, expected: number, label: string): void {
  expect(
    Math.abs(actual - expected),
    `${label}: ${actual.toFixed(2)}px vs ${expected.toFixed(2)}px`
  ).toBeLessThanOrEqual(PX_TOLERANCE);
}

test.describe('desktop leaderboard skeleton-vs-live parity', () => {
  // The desktop parity criterion is a >=1024px viewport statement; the phone
  // project's media blocks collapse the wl/status columns on both rows, which
  // the mobile parity child owns.
  test.skip(() => test.info().project.name === 'phone', 'desktop (>=1024px) criterion only');

  test('each skeleton bar sits on the live column it stands in for', async ({ page }) => {
    await openFixture(page);
    const bars = await measureAll(page, `${SKELETON_ROW} > *`);
    const columns = await measureAll(page, `${LIVE_ROW} > *`);

    expect(bars, 'skeleton row must render 7 bars').toHaveLength(COLUMN_NAMES.length);
    expect(columns, 'live row must render 7 columns').toHaveLength(COLUMN_NAMES.length);

    for (let i = 0; i < COLUMN_NAMES.length; i++) {
      const name = COLUMN_NAMES[i];
      expect(bars[i].display, `${name} bar must actually render`).not.toBe('none');
      expect(columns[i].display, `${name} column must actually render`).not.toBe('none');
      within(bars[i].left, columns[i].left, `${name}: left edge`);
      within(bars[i].width, columns[i].width, `${name}: width`);
    }
  });

  test('both rows measure the same geometry out of the shared .lb-row rule', async ({ page }) => {
    await openFixture(page);
    const skeletonRow = await measure(page, SKELETON_ROW);
    const liveRow = await measure(page, LIVE_ROW);

    // Equal row width is what makes "aligned columns" a meaningful comparison:
    // everything right of the flexing name column shifts with it, so two rows
    // of different widths could not align bar-for-bar at all.
    within(skeletonRow.width, liveRow.width, 'row width');

    // min-height (48px, declared once on .lb-row) is a floor neither row's
    // content reaches, so both must measure the same height — and actually
    // reach the floor rather than collapsing under it.
    within(skeletonRow.height, liveRow.height, 'row height');
    expect(skeletonRow.height).toBeGreaterThanOrEqual(48);
    expect(liveRow.height).toBeGreaterThanOrEqual(48);

    const [skeletonStyles, liveStyles] = await Promise.all([
      readStyles(page, SKELETON_ROW, [...ROW_GEOMETRY_PROPS]),
      readStyles(page, LIVE_ROW, [...ROW_GEOMETRY_PROPS]),
    ]);
    for (const prop of ROW_GEOMETRY_PROPS) {
      expect(skeletonStyles[prop], `${prop} resolves differently on the two rows`).toBe(liveStyles[prop]);
    }

    // ...and the shared values must be real laid-out quantities, so an
    // unstyled or rule-less side cannot pass by matching an empty string.
    expect(skeletonStyles['display']).toBe('flex');
    expect(parseFloat(skeletonStyles['gap'])).toBeGreaterThan(0);
    expect(parseFloat(skeletonStyles['padding-left'])).toBeGreaterThan(0);
    expect(parseFloat(skeletonStyles['padding-top'])).toBeGreaterThan(0);
    expect(parseFloat(skeletonStyles['min-height'])).toBe(48);
  });

  test('the shared rule resolves to what the boxes actually do', async ({ page }) => {
    await openFixture(page);
    for (const [label, rowSelector] of [
      ['skeleton', SKELETON_ROW],
      ['live', LIVE_ROW],
    ] as const) {
      const row = await measure(page, rowSelector);
      const children = await measureAll(page, `${rowSelector} > *`);
      const styles = await readStyles(page, rowSelector, [
        'gap',
        'padding-left',
        'padding-right',
        'min-height',
      ]);

      // The .lb-row quantities, read off the laid-out boxes instead of trusted
      // from the declaration: the gap is the space between the first two
      // columns, the horizontal padding is what the row has left over at each
      // end, and min-height is a floor the row genuinely reaches.
      expect(children.length).toBeGreaterThanOrEqual(2);
      within(children[1].left - children[0].right, parseFloat(styles['gap']), `${label}: measured gap`);
      within(children[0].left - row.left, parseFloat(styles['padding-left']), `${label}: measured padding-left`);
      within(
        row.right - children[children.length - 1].right,
        parseFloat(styles['padding-right']),
        `${label}: measured padding-right`
      );
      expect(
        row.height,
        `${label}: row must reach its min-height floor`
      ).toBeGreaterThanOrEqual(parseFloat(styles['min-height']));
    }
  });
});
