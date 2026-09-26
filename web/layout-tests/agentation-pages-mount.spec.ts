// Per-page Agentation mount audit on the REAL built pages (workspace
// standard: "verify by mounting, never by grepping the tag").
//
// This is the fourth and last layer of the audit started by
// aicodeba-a58e2876; the other three:
//   src/agentation-overlay.test.ts  — initAgentation() works under jsdom
//   src/agentation-entries.test.ts  — every web-root HTML entry reaches
//                                     initAgentation (source-level; auto-
//                                     enumerates new pages)
//   ./agentation-mount.spec.ts      — the mount mechanism works in real
//                                     Chromium (synthetic fixture)
// None of those loads the artifact a human actually loads. A page can wire
// the call (proven by source) to a mechanism that works (proven by fixture)
// and still ship a bundle where manualChunks dropped the agentation chunk or
// a build regression stripped the async import — the page renders perfectly
// and carries no toolbar, the exact silent failure the standard warns about.
// This spec builds the real vite inputs and mounts each one in Chromium,
// served through route fulfillment so the harness stays serverless (no dev
// server, no port).
//
// test-replay-viewer.html is not here on purpose: it is an internal test
// harness page, not a vite input, and never ships (docs/notes/
// agentation-coverage.md records the full audited page list). It loads
// /src/main.ts — the same entry module replay.html mounts and this spec
// verifies in built form.

import { expect, test } from '@playwright/test';
import { build } from 'vite';
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, '..');

// The vite rollup inputs (vite.config.ts rollupOptions.input). A new HTML
// entry point added to that map is picked up here by the tripwire below —
// the build then emits a fourth page, the equality fails, and the new page
// joins PAGES in the same commit.
const PAGES = ['embed.html', 'index.html', 'replay.html'];

const PAGE_ORIGIN = 'http://acb-mount-audit.test';

const MIME: Record<string, string> = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.mjs': 'text/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.map': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.txt': 'text/plain',
  '.wasm': 'application/wasm',
};

// One build per worker, one worker for this file: the desktop project alone
// runs it (the toolbar has no viewport-dependent behavior), and serial mode
// keeps fullyParallel from spreading the tests — and the beforeAll build —
// across workers. Skipping inside beforeAll skips the file's tests with it.
test.describe.configure({ mode: 'serial' });

let distDir: string;

test.beforeAll(async ({}, testInfo) => {
  test.skip(
    testInfo.project.name !== 'desktop',
    'the mount audit builds once and has no viewport-dependent assertions'
  );
  distDir = mkdtempSync(join(tmpdir(), 'acb-mount-audit-'));
  // A private outDir, not the shared web/dist: another worker may be
  // building/deploying from web/dist while this runs. (The config's
  // closeBundle hook still copies public/_redirects into web/dist —
  // gitignored and byte-identical, so harmless.)
  await build({
    configFile: join(webRoot, 'vite.config.ts'),
    root: webRoot,
    logLevel: 'error',
    build: { outDir: distDir, emptyOutDir: true },
  });
}, 240_000);

test.afterAll(() => {
  if (distDir) rmSync(distDir, { recursive: true, force: true });
});

/**
 * Serve the built pages from distDir on a reserved .test origin. API and
 * data paths answer 404 JSON — the app treats any fetch failure as a loading
 * or error state, and the mount is independent of all of them.
 */
async function serveDist(page: import('@playwright/test').Page): Promise<void> {
  await page.route(`${PAGE_ORIGIN}/**`, route => {
    const { pathname } = new URL(route.request().url());
    const fsPath = join(distDir, pathname === '/' ? '/index.html' : pathname);
    if (existsSync(fsPath) && statSync(fsPath).isFile()) {
      return route.fulfill({
        body: readFileSync(fsPath),
        contentType: MIME[extname(fsPath)] ?? 'application/octet-stream',
      });
    }
    if (pathname.startsWith('/api/') || pathname.startsWith('/data/')) {
      return route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: '{"error":"not served by the mount audit"}',
      });
    }
    return route.fulfill({ status: 404, body: 'not found' });
  });
}

test('the build emits exactly the shipped entry pages', () => {
  expect(readdirSync(distDir).filter(f => f.endsWith('.html')).sort()).toEqual(PAGES);
});

for (const page_ of PAGES) {
  test(`${page_} mounts the agentation toolbar after load`, async ({ page }) => {
    const pageErrors: string[] = [];
    page.on('pageerror', e => pageErrors.push(e.message));
    await serveDist(page);
    await page.goto(`${PAGE_ORIGIN}/${page_}`, { waitUntil: 'load' });

    // #agentation-root alone is the div initAgentation creates; the toolbar
    // element inside it is the assertion that React really committed.
    await expect(page.locator('#agentation-root')).toHaveCount(1, { timeout: 15_000 });
    const toolbar = page.locator('[data-agentation-toolbar="true"]');
    await expect(toolbar).toHaveCount(1, { timeout: 15_000 });
    await expect(toolbar).toBeVisible();
    expect(
      pageErrors,
      `${page_} must mount without throwing: ${pageErrors.join(' | ')}`
    ).toEqual([]);
  });
}
