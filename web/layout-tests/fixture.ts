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
 * Mirrors renderDesktopRow (web/src/pages/leaderboard.ts) node for node with
 * static sample values. That renderer is module-private and this child ships
 * only the measuring capability, so the mirror lives here; a parity child can
 * swap in the exported renderer without touching the measurement API. Keep the
 * element types and classes identical — geometry comes from the live rules, so
 * drift here silently re-bases every parity assertion.
 */
function liveDesktopRow(rank: number): string {
  return `
    <div class="lb-row rank-${rank}" data-bot-id="bot-${rank}" tabindex="0" role="button" aria-expanded="false">
      <span class="lb-rank">${rank}</span>
      <span class="lb-name"><a href="#/bot/bot-${rank}">Bot ${rank}</a></span>
      <span class="lb-rating"><span class="rating-value">${1000 + rank}</span><span class="rating-dev">±50</span></span>
      <span class="lb-wl">10/${10 + rank}</span>
      <span class="lb-winrate">83.${rank}%</span>
      <span class="lb-status status-healthy">healthy</span>
      <span class="lb-expand-icon" aria-hidden="true">▸</span>
    </div>`;
}

/**
 * Mirrors renderMobileCard (web/src/pages/leaderboard.ts) with the same static
 * values — button toggle, arrow span, four stats, full-stats link.
 */
function liveMobileCard(rank: number): string {
  return `
    <div class="leaderboard-mobile-card" role="listitem" data-bot-id="bot-${rank}" aria-expanded="false">
      <button class="mobile-card-toggle" aria-label="Expand details for Bot ${rank}" type="button">
        <div class="leaderboard-mobile-rank rank-${rank}">${rank}</div>
        <div class="leaderboard-mobile-info">
          <div class="leaderboard-mobile-name">Bot ${rank}</div>
          <div class="leaderboard-mobile-rating">${1000 + rank} <span style="opacity:.6;font-size:.8em">±50</span></div>
        </div>
        <span class="mobile-card-arrow" aria-hidden="true">▸</span>
      </button>
      <div class="leaderboard-mobile-details">
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">W / L</span>
          <span class="leaderboard-mobile-stat-value">10 / ${rank}</span>
        </div>
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">Win Rate</span>
          <span class="leaderboard-mobile-stat-value">83.${rank}%</span>
        </div>
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">Matches</span>
          <span class="leaderboard-mobile-stat-value">${10 + rank}</span>
        </div>
        <div class="leaderboard-mobile-stat">
          <span class="leaderboard-mobile-stat-label">Status</span>
          <span class="leaderboard-mobile-stat-value status-healthy">healthy</span>
        </div>
        <a href="#/bot/bot-${rank}"
           class="btn small"
           style="margin-top:10px;display:block;text-align:center"
           aria-label="Full stats for Bot ${rank}">Full Stats →</a>
      </div>
    </div>`;
}

export function buildFixtureHtml(): string {
  // One document carries both blocks so parity assertions compare skeleton and
  // live geometry under the same viewport and the same stylesheet set. The
  // live containers get their own ids (#live-lb-*) — the skeleton markup owns
  // the real page's #lb-desktop/#lb-mobile — while the live *classes* stay
  // identical so the same rules govern them.
  const rows = [1, 2, 3].map(liveDesktopRow).join('\n');
  const cards = [1, 2, 3].map(liveMobileCard).join('\n');
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
    <div id="live-lb-desktop">${rows}</div>
    <div id="live-lb-mobile" class="mobile-cards">${cards}</div>
  </section>
</body>
</html>`;
}

/** Loads the fixture into a page. Call once at the top of each test. */
export async function openFixture(page: Page): Promise<void> {
  await page.setContent(buildFixtureHtml(), { waitUntil: 'load' });
}
