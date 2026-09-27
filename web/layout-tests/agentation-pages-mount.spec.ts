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
//
// Mounting alone is the layer-3 fixture's proof transplanted onto real
// bundles. The identify half of the standard is proven here too, per page:
// the toolbar's own annotate flow (Ctrl+Shift+F, click the page, comment in
// the popup) runs its identify pipeline against the app's real DOM. On the
// built pages initAgentation owns the mount and passes no demo props, so
// demo mode (agentation-probe.ts) is not drivable there — driving the real
// toolbar UI is, and it is the stronger proof: nothing is stubbed, the
// pipeline that runs is the one a human triggers.

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

// Where the annotate-flow test clicks on each page: one stable element of
// the page's own DOM (static chrome where present, the app's load-failure
// state where the API is mocked away — the mount audit 404s every /api and
// /data path, so embed shows its error overlay and index/replay render
// around their empty data). The flow identifies the deepest element under
// the pointer, so the target is a container and the assertion accepts the
// identified element itself or any descendant. A page joining PAGES above
// needs an entry here or its identify test fails on an undefined selector.
const IDENTIFY_TARGETS: Record<string, string> = {
  'index.html': 'nav',
  'embed.html': '.embed-container',
  'replay.html': '.canvas-wrapper',
};

// The comment typed into the popup; the store is matched on it. Fresh per
// test — each Playwright test gets its own context, so its localStorage
// starts empty and nothing from another page can leak in.
const IDENTIFY_NOTE = 'pages-mount identify audit';

type IdentifiedAnnotation = {
  comment?: string;
  element?: string;
  elementPath?: string;
  boundingBox?: { width: number; height: number };
};

for (const page_ of PAGES) {
  test(`${page_} identifies an element of the app's own DOM through the toolbar's annotate flow`, async ({ page }) => {
    const pageErrors: string[] = [];
    page.on('pageerror', e => pageErrors.push(e.message));
    await serveDist(page);
    await page.goto(`${PAGE_ORIGIN}/${page_}`, { waitUntil: 'load' });
    await expect(page.locator('#agentation-root')).toHaveCount(1, { timeout: 15_000 });
    await expect(page.locator('[data-agentation-toolbar="true"]')).toBeVisible();

    // Annotate mode is the toolbar's own toggle (Ctrl/Cmd+Shift+F); a click
    // while it is active makes the toolbar identify the element under the
    // pointer and open its popup. This is the pipeline the fixture spec
    // drives in demo mode — here nothing is stubbed and the element named
    // comes out of the page's real bundle and real layout.
    await page.keyboard.press('Control+Shift+F');
    await page.click(IDENTIFY_TARGETS[page_]);

    const popup = page.locator('[data-annotation-popup="true"]');
    await expect(popup).toBeVisible();
    // The popup auto-focuses its textarea and submits on Enter.
    await popup.locator('textarea').fill(IDENTIFY_NOTE);
    await popup.locator('textarea').press('Enter');

    // The toolbar persists accepted annotations under its own key
    // (feedback-annotations-<pathname>): waiting on the store proves the
    // identify pipeline ran, and the shape proves what it identified.
    const handle = await page.waitForFunction(
      note => {
        const stored = localStorage.getItem(
          `feedback-annotations-${location.pathname}`
        );
        if (!stored) return null;
        return (
          (JSON.parse(stored) as IdentifiedAnnotation[]).find(
            a => a.comment === note
          ) ?? null
        );
      },
      IDENTIFY_NOTE,
      { timeout: 10_000 }
    );
    const ann = (await handle.jsonValue()) as IdentifiedAnnotation;

    expect(ann.element, 'the annotation must name the identified element').toBeTruthy();
    expect(ann.elementPath, 'the annotation must carry the identified path').toBeTruthy();

    // The identification is only useful if the path turns back into the
    // element — inside the container that was clicked.
    const targetSelector = IDENTIFY_TARGETS[page_];
    const inTarget = await page.evaluate(
      ({ path, targetSelector }) => {
        const el = document.querySelector(path);
        const target = document.querySelector(targetSelector);
        return el !== null && target !== null && (el === target || target.contains(el));
      },
      { path: ann.elementPath!, targetSelector }
    );
    expect(
      inTarget,
      `annotation path "${ann.elementPath}" must resolve inside ${targetSelector}`
    ).toBe(true);

    // Real layout only — jsdom's zeros can never pass this.
    expect(ann.boundingBox?.width ?? 0).toBeGreaterThan(0);
    expect(ann.boundingBox?.height ?? 0).toBeGreaterThan(0);

    // The toolbar's own state accepted the annotation — it renders a marker.
    await expect(page.locator('[data-annotation-marker]').first()).toBeVisible();

    expect(
      pageErrors,
      `${page_} must run the annotate flow without throwing: ${pageErrors.join(' | ')}`
    ).toEqual([]);
  });
}
