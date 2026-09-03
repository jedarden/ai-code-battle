/**
 * Fixture page for the rendered-layout harness (§16.14 skeleton parity).
 *
 * jsdom has no layout engine — getBoundingClientRect() returns zeros there, so
 * the CSS-parsing guards in src/components/skeleton.test.ts can never check the
 * rendered geometry the parity criteria actually talk about. This module builds
 * the page those checks need: skeletonLeaderboard() output next to a
 * live-classed .lb-row / .mobile-cards fixture, with the app's stylesheets
 * inlined so a headless Chromium (see ../playwright.config.ts) lays it out for
 * real and measure() can read geometry off it. The fixture is set directly via
 * page.setContent — no dev server, no port, no temp files.
 */

import { readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Page } from '@playwright/test';
import { skeletonLeaderboard } from '../src/components/skeleton';
import { renderDesktopRow, renderMobileCard } from '../src/pages/leaderboard';
import type { LeaderboardEntry } from '../src/api-types';

const stylesDir = resolve(dirname(fileURLToPath(import.meta.url)), '../src/styles');

/**
 * The stylesheets the fixture lays out under, in cascade order. These three
 * files are the single source of truth the skeleton guards already parse
 * (base.css owns the reset + custom properties, components.css owns .lb-row,
 * mobile.css owns the .mobile-cards visibility blocks).
 *
 * Note: nothing imports these files into the app bundle yet (no `import
 * './styles/*.css'` anywhere in src/, no <link rel="stylesheet"> in
 * index.html, and dist/ ships no .css), so inlining them here is the only way
 * a browser can apply them. If the app starts shipping its CSS through another
 * path, this list is what follows it.
 */
const STYLESHEETS = ['base.css', 'components.css', 'mobile.css'] as const;

function inlineStyles(): string {
  return STYLESHEETS.map((file) => readFileSync(join(stylesDir, file), 'utf8')).join('\n');
}

/**
 * A static entry shaped like the API payload, fed to the real renderDesktopRow
 * and renderMobileCard (web/src/pages/leaderboard.ts). Ranks start at 4 so no
 * row or card picks up the .rank-1/2/3 podium tint — the parity assertions
 * compare geometry, and a class-plain row is easier to reason about than a
 * tinted one. The renderers themselves are what run in the app; nothing here
 * mirrors their markup any more.
 */
function sampleEntry(rank: number): LeaderboardEntry {
  return {
    rank,
    bot_id: `bot-${rank}`,
    name: `Bot ${rank}`,
    owner_id: 'owner-parity',
    rating: 1000 + rank,
    rating_deviation: 50,
    matches_played: 24,
    matches_won: 10 + rank,
    win_rate: 41.7,
    health_status: 'healthy',
  };
}

export function buildFixtureHtml(): string {
  // One document carries both blocks so parity assertions compare skeleton and
  // live geometry under the same viewport and the same stylesheet set. The
  // live containers get their own ids (#live-lb-*) — the skeleton markup owns
  // the real page's #lb-desktop/#lb-mobile — while the live *classes* stay
  // identical so the same rules govern them.
  //
  // The live block sits inside .leaderboard-page because that is the container
  // the real data-load swap renders into (renderLeaderboard in
  // web/src/pages/leaderboard.ts): the skeleton measures under .skeleton-page,
  // so the live row must measure under the live page's own wrapper for
  // "aligned columns" to compare like with like. #leaderboard-content is
  // omitted — it carries no rule of its own.
  const rows = [4, 5, 6].map((rank) => renderDesktopRow(sampleEntry(rank), 0)).join('\n');
  const cards = [4, 5, 6].map((rank) => renderMobileCard(sampleEntry(rank))).join('\n');
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ACB rendered-layout fixture</title>
  <style>${inlineStyles()}</style>
</head>
<body>
  <div id="calibration-40" style="width:40px;height:12px"></div>

  <section id="skeleton-fixture" aria-label="skeleton leaderboard">
    ${skeletonLeaderboard()}
  </section>

  <section id="live-fixture" aria-label="live leaderboard fixture">
    <div class="leaderboard-page">
      <div id="live-lb-desktop">${rows}</div>
      <div id="live-lb-mobile" class="mobile-cards" role="list">${cards}</div>
    </div>
  </section>
</body>
</html>`;
}

/** Loads the fixture into a page. Call once at the top of each test. */
export async function openFixture(page: Page): Promise<void> {
  await page.setContent(buildFixtureHtml(), { waitUntil: 'load' });
}
