/**
 * Bot-profile fixture for the rendered-layout harness (§16.14 skeleton
 * parity) — the same shape as ./fixture.ts, for skeletonBotProfile().
 *
 * jsdom has no layout engine, so the CSS-text guards in
 * src/components/skeleton.test.ts cannot check the rendered geometry the
 * parity criterion talks about. This module lays skeletonBotProfile() output
 * out next to the real profile markup — renderProfileMarkup, imported from
 * pages/bot-profile.ts — under the app's stylesheets, so a headless Chromium
 * measures both as actually rendered.
 *
 * The live side is the markup only, with none of renderProfile's wiring: the
 * lazy observer would swap the history placeholder for the real section
 * mid-measurement, and the skeleton stands in for the pre-reveal page (its
 * 80px placeholder mirrors the live placeholder string renderProfileMarkup
 * emits). The ratings chart is filled through the real renderRatingChart,
 * because the sparkline it injects is part of the ratings section's height.
 */

import type { Page } from '@playwright/test';
import { inlineStyles } from './fixture';
import { renderProfileMarkup, renderRatingChart } from '../src/pages/bot-profile';
import { skeletonBotProfile } from '../src/components/skeleton';
import type { BotProfile, RivalryEntry } from '../src/api-types';

/**
 * A typical profile: enough rating_history for renderRatingChart to draw the
 * sparkline (it needs ≥ 2 points), one rivalry so the live page renders the
 * rivals section the skeleton always renders, and a short name that stays on
 * one line the way the skeleton's single 220px bar does. Meta/rivals content
 * and recent matches never reach the layout — those sections stand collapsed
 * and the history block is unrevealed — so their sample values are inert.
 */
function sampleProfile(): BotProfile {
  return {
    id: 'bot-parity',
    name: 'Parity',
    owner_id: 'owner-parity',
    rating: 1204,
    rating_deviation: 48,
    rating_volatility: 0.06,
    matches_played: 24,
    matches_won: 15,
    win_rate: 62.5,
    health_status: 'healthy',
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-09-01T10:00:00Z',
    rating_history: [1150, 1168, 1175, 1190, 1188, 1204].map((rating, i) => ({
      bot_id: 'bot-parity',
      rating,
      rating_deviation: 50 - i,
      recorded_at: `2026-08-${10 + i * 3}T12:00:00Z`,
    })),
    recent_matches: Array.from({ length: 3 }, (_, i) => ({
      id: `match-${i}`,
      completed_at: `2026-08-2${i}T09:00:00Z`,
      participants: [
        { bot_id: 'bot-parity', name: 'Parity', score: 12, won: true },
        { bot_id: 'bot-other', name: 'Other', score: 7, won: false },
      ],
      winner_id: 'bot-parity',
      turns: 42,
      end_reason: 'elimination',
    })),
  };
}

function sampleRivalry(): RivalryEntry {
  return {
    bot_a: { id: 'bot-parity', name: 'Parity' },
    bot_b: { id: 'bot-rival', name: 'Rival' },
    matches: 6,
    record: { a_wins: 4, b_wins: 2, draws: 0 },
    recent_matches: [],
    narrative: 'A one-sided series.',
    score: 0.67,
  };
}

function buildBotProfileFixtureHtml(): string {
  // One document carries both blocks so parity assertions compare skeleton and
  // live geometry under the same viewport and stylesheet set. The skeleton
  // brings its own .skeleton-page > .bot-profile-page wrapper (components.css
  // keeps that wrapper layout-transparent); the live block mirrors the swap's
  // innerHTML — .bot-profile-page.fade-in > nav.breadcrumb + #profile-content
  // + renderProfileMarkup's output — with an id on the page wrapper the spec
  // can address. #profile-content carries no rule of its own, exactly as on
  // the real page, and the breadcrumb text matches the sample profile's name.
  const profile = sampleProfile();
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ACB bot-profile rendered-layout fixture</title>
  <style>${inlineStyles()}</style>
</head>
<body>
  <section id="skeleton-fixture" aria-label="skeleton bot profile">
    ${skeletonBotProfile()}
  </section>

  <section id="live-fixture" aria-label="live bot profile fixture">
    <div class="bot-profile-page fade-in" id="live-profile-root">
      <nav class="breadcrumb">
        <a href="#/leaderboard">Leaderboard</a> / <span>${profile.name}</span>
      </nav>
      <div id="profile-content">
        ${renderProfileMarkup(profile, [sampleRivalry()])}
      </div>
    </div>
  </section>
</body>
</html>`;
}

/**
 * Loads the fixture and fills the live ratings chart. renderRatingChart is
 * evaluated inside the page — it is self-contained (document + Math only), so
 * Playwright serializes it as-is and it runs against #rating-chart exactly as
 * renderProfile runs it after mounting the markup.
 */
export async function openBotProfileFixture(page: Page): Promise<void> {
  await page.setContent(buildBotProfileFixtureHtml(), { waitUntil: 'load' });
  await page.evaluate(renderRatingChart, sampleProfile());
}
