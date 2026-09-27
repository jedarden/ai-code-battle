// Live-production Agentation mount audit — the fifth layer, and the only one
// that loads what a human actually loads. Layers 1–4 (docs/notes/
// agentation-coverage.md) all run against repo artifacts: jsdom, a synthetic
// fixture, source-level wiring, and the vite build served from route
// fulfillment. None of them can see a deployment regression — an older
// bundle pushed to Pages, a chunk the CDN stopped serving, an entry page the
// deploy dropped — while the site renders and carries no toolbar, the exact
// silent failure the standard warns about. This spec drives headless
// Chromium against the deployed Pages origin and asserts the mount there.
//
// Live-only and opt-in so the default suites stay hermetic: it is skipped
// unless ACB_LIVE_ORIGIN is set, which only the npm script sets —
//   npm run test:agentation-live
// Point ACB_LIVE_ORIGIN at a Pages preview deployment to audit it before
// promotion. Needs the nix Chromium on NixOS (see ../playwright.config.ts).
//
// Per-route proof, not just a mount check: a route this origin does not
// deploy does NOT 404 — Cloudflare Pages answers with a 200 text/html SPA
// fallback that serves index.html, whose toolbar mounts fine and would make
// a mount-only assertion pass on a page that was never shipped. The verdict
// therefore branches on content, in two steps: the response must be
// text/html, and its HTML must carry the route's own vite entry chunk in a
// script tag (the fallback references main-*, never the route's entry; a
// loose substring check would not do — the modulepreload <link>s name
// replay-viewer/replay-page chunks that share the replay- prefix, and
// dynamically imported chunks inject their own script tags after load, so
// the anchor is the entry script tag of the served document itself).
// Only then is the mount asserted: #agentation-root is the div
// initAgentation creates; the toolbar element inside it is the assertion
// that React really committed, and it must be visible, not merely attached.
//
// The mount verdict is deliberately the only assertion — pageerror
// freedom is layer 4's proof on the built pages, and on the live origin an
// uncaught error can come from the deployment's data endpoints just as
// well as from the mount, which would make this spec flaky in a way the
// mount never is.

import { expect, test } from '@playwright/test';

// The origin to audit. Set only by `npm run test:agentation-live` (which
// defaults it to production) or explicitly in the environment.
const LIVE_ORIGIN = process.env.ACB_LIVE_ORIGIN;

// The shipped routes on the Pages origin with the vite entry chunk each
// must reference (the rollupOptions.input keys in ../vite.config.ts).
// Confirmed live 2026-09-27: all three answer 200 text/html and /replay.html
// and /embed.html redirect to the extensionless forms.
const ROUTES = [
  { path: '/', entry: 'main' },
  { path: '/replay', entry: 'replay' },
  { path: '/embed', entry: 'embed' },
] as const;

for (const route of ROUTES) {
  test(`${route.path} serves its own page and mounts the agentation toolbar on the live origin`, async ({
    page,
  }, testInfo) => {
    test.setTimeout(120_000);
    test.skip(
      testInfo.project.name !== 'desktop',
      'one project is enough for a mount verdict'
    );
    test.skip(
      !LIVE_ORIGIN,
      'live-only: run through `npm run test:agentation-live` (sets ACB_LIVE_ORIGIN)'
    );

    const response = await page.goto(`${LIVE_ORIGIN}${route.path}`, {
      waitUntil: 'load',
      timeout: 60_000,
    });
    expect(
      response,
      `${LIVE_ORIGIN}${route.path} must answer`
    ).not.toBeNull();
    const contentType = response!.headers()['content-type'] ?? '';
    expect(
      contentType,
      `${route.path} must serve HTML (got "${contentType}")`
    ).toContain('text/html');

    // The served document must be this route's page, not the SPA fallback.
    const entryScript = new RegExp(
      `<script[^>]*src="/assets/${route.entry}-[^"]*\\.js"`
    );
    const html = await response!.text();
    expect(
      html,
      `${route.path} must reference its ${route.entry} entry chunk in a script tag; ` +
        'a document without it is the SPA fallback serving index.html, not this page'
    ).toMatch(entryScript);

    // #agentation-root alone is the div initAgentation creates; the toolbar
    // element inside it is the assertion that React really committed.
    await expect(page.locator('#agentation-root')).toHaveCount(1, {
      timeout: 20_000,
    });
    const toolbar = page.locator('[data-agentation-toolbar="true"]');
    await expect(toolbar).toHaveCount(1, { timeout: 20_000 });
    await expect(toolbar).toBeVisible();
  });
}
