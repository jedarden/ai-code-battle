/**
 * Network-condition coverage for the three §16.14 skeleton pages.
 *
 * The parity specs prove the geometry of each settled skeleton/content pair.
 * This spec proves the other half of the loading contract: the placeholder is
 * painted before a response, remains stable while a slow response is pending,
 * and is replaced by an opacity-faded page (or a visible error) when the
 * request completes. The page is served by a tiny local HTTP server so the
 * delay is a real network wait rather than a timer-only mock.
 */

import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from '@playwright/test';
import { inlineStyles } from './fixture';
import { skeletonBotProfile, skeletonLeaderboard, skeletonReplay } from '../src/components/skeleton';
import { renderProfileMarkup } from '../src/pages/bot-profile';
import { replayPageMarkup } from '../src/pages/replay';
import type { BotProfile } from '../src/api-types';

type PageKind = 'leaderboard' | 'bot-profile' | 'replay';

interface NetworkCondition {
  name: string;
  delayMs: number;
  offline?: boolean;
}

const CONDITIONS: readonly NetworkCondition[] = [
  { name: 'fast-4g', delayMs: 150 },
  { name: 'slow-3g', delayMs: 2000 },
  { name: 'offline', delayMs: 0, offline: true },
];

const PAGE_KINDS: readonly PageKind[] = ['leaderboard', 'bot-profile', 'replay'];

let server: Server | undefined;
let origin = '';

const profile: BotProfile = {
  id: 'network-profile',
  name: 'Network profile',
  owner_id: 'network-owner',
  rating: 1200,
  rating_deviation: 50,
  rating_volatility: 0.06,
  matches_played: 12,
  matches_won: 7,
  win_rate: 58.3,
  health_status: 'healthy',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  rating_history: [],
  recent_matches: [],
};

function skeletonFor(kind: PageKind): string {
  if (kind === 'leaderboard') return skeletonLeaderboard();
  if (kind === 'bot-profile') return skeletonBotProfile();
  return skeletonReplay();
}

function contentFor(kind: PageKind): string {
  if (kind === 'leaderboard') {
    return `<div class="leaderboard-page fade-in"><h1 class="page-title">Leaderboard</h1><div class="updated-at">Last updated</div><div class="lb-hint">Click a row to see full stats</div></div>`;
  }
  if (kind === 'bot-profile') {
    return `<div class="bot-profile-page fade-in"><nav class="breadcrumb"><a href="#/leaderboard">Leaderboard</a> / <span>${profile.name}</span></nav><div id="profile-content">${renderProfileMarkup(profile)}</div></div>`;
  }
  return replayPageMarkup();
}

function pageHtml(kind: PageKind): string {
  const payload = JSON.stringify({
    skeleton: skeletonFor(kind),
    content: contentFor(kind),
  }).replace(/</g, '\\u003c');

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>${inlineStyles()}</style>
</head>
<body>
  <main id="app"></main>
  <script>
    const payload = ${payload};
    const app = document.getElementById('app');
    const params = new URLSearchParams(location.search);
    const delay = Number(params.get('delay') || 0);
    window.__layoutShiftValue = 0;
    window.__networkOutcome = '';
    if (window.PerformanceObserver && PerformanceObserver.supportedEntryTypes.includes('layout-shift')) {
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (!entry.hadRecentInput) window.__layoutShiftValue += entry.value;
        }
      }).observe({ type: 'layout-shift', buffered: true });
    }
    app.innerHTML = payload.skeleton;
    window.__skeletonPainted = true;
    const skeletonRect = app.querySelector('.skeleton-page').getBoundingClientRect();
    window.__skeletonSignature = { width: skeletonRect.width, height: skeletonRect.height };
    fetch('/network-data?delay=' + encodeURIComponent(delay))
      .then((response) => {
        if (!response.ok) throw new Error('network request failed');
        return response.json();
      })
      .then(() => {
        app.innerHTML = payload.content;
        window.__networkOutcome = 'content';
      })
      .catch(() => {
        app.innerHTML = '<div class="network-error error">Unable to load this page.</div>';
        window.__networkOutcome = 'error';
      });
  </script>
</body>
</html>`;
}

async function startFixtureServer(): Promise<void> {
  server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1');
    if (url.pathname === '/network-data') {
      const delay = Number(url.searchParams.get('delay') ?? 0);
      setTimeout(() => {
        response.writeHead(200, { 'content-type': 'application/json' });
        response.end('{}');
      }, delay);
      return;
    }

    const kind = url.searchParams.get('page') as PageKind | null;
    if (!kind || !PAGE_KINDS.includes(kind)) {
      response.writeHead(400);
      response.end('missing page');
      return;
    }
    response.writeHead(200, { 'content-type': 'text/html' });
    response.end(pageHtml(kind));
  });

  await new Promise<void>((resolve, reject) => {
    server?.once('error', reject);
    server?.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address() as AddressInfo;
  origin = `http://127.0.0.1:${address.port}`;
}

async function stopFixtureServer(): Promise<void> {
  if (!server) return;
  await new Promise<void>((resolve, reject) => server?.close((error) => error ? reject(error) : resolve()));
  server = undefined;
}

test.beforeAll(startFixtureServer);
test.afterAll(stopFixtureServer);

test.describe('skeleton loading under network conditions', () => {
  for (const kind of PAGE_KINDS) {
    for (const condition of CONDITIONS) {
      test(`${kind} paints a stable skeleton on ${condition.name}`, async ({ page }, testInfo) => {
        // Each test drives its own page state; one desktop pass covers all
        // conditions and page types, while the parity suite owns the phone
        // geometry matrix.
        test.skip(testInfo.project.name === 'phone', 'network lifecycle is viewport-independent');

        if (condition.offline) {
          await page.route('**/network-data*', (route) => route.abort('internetdisconnected'));
        }

        await page.goto(
          `${origin}/?page=${kind}&delay=${condition.delayMs}`,
          { waitUntil: 'domcontentloaded' }
        );

        const before = await page.evaluate(() => {
          const state = window.__skeletonSignature;
          if (!window.__skeletonPainted || !state) {
            throw new Error('skeleton was not painted before the request');
          }
          return state;
        });

        if (!condition.offline && condition.delayMs > 500) {
          await page.waitForTimeout(100);
          await expect(page.locator('#app .skeleton-page')).toBeVisible();
          const during = await page.evaluate(() => {
            if (window.__networkOutcome) throw new Error('slow response completed too early');
            const root = document.querySelector<HTMLElement>('#app .skeleton-page');
            if (!root) throw new Error('skeleton disappeared before the response');
            const rect = root.getBoundingClientRect();
            return { width: rect.width, height: rect.height };
          });
          expect(during, 'network latency must not alter skeleton geometry').toEqual(before);
          expect(
            await page.evaluate(() => window.__layoutShiftValue),
            'the placeholder must not accumulate CLS while the response is pending'
          ).toBe(0);
          expect(await page.locator('#app .fade-in').count()).toBe(0);
        }

        await page.waitForFunction(() => Boolean(window.__networkOutcome));

        if (condition.offline) {
          await expect(page.locator('#app .network-error')).toBeVisible();
          return;
        }

        await expect(page.locator('#app .fade-in')).toBeVisible();
        const fade = await page.locator('#app .fade-in').evaluate((element) => {
          const style = getComputedStyle(element);
          return { name: style.animationName, duration: style.animationDuration };
        });
        expect(fade).toEqual({ name: 'fade-in', duration: '0.15s' });
        expect(await page.locator('#app .skeleton-page').count()).toBe(0);
      });
    }
  }
});
