// Real-browser regression for the Agentation mount (workspace standard:
// "verify by mounting, never by grepping the tag").
//
// The vitest suite (src/agentation-overlay.test.ts) proves initAgentation()
// under jsdom; src/agentation-entries.test.ts proves every entry point
// actually calls it. This spec proves both halves of the standard in the
// engine a human actually renders with: #agentation-root exists after load
// with the toolbar really in it, and the toolbar can identify page elements —
// checked against real layout, which jsdom cannot give (every
// getBoundingClientRect there is zero, so a non-zero boundingBox here is
// proof the number came out of a real rendering engine).
//
// The probe (./agentation-probe.ts — real react, real agentation) is bundled
// once in-process with esbuild (vite's own bundler) and injected as an inline
// script, keeping this spec serverless like the rest of the harness: no dev
// server, no network, no temp files.

import { expect, test, type Page } from '@playwright/test';
import { build } from 'esbuild';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

/** The probe's in-page API. The functions live in the page — every call goes
 * through page.evaluate; they cannot cross into Node. */
type ProbeApi = {
  identifyElement: (el: HTMLElement) => { name: string; path: string };
  getElementPath: (el: HTMLElement) => string;
  loadAnnotations: (pathname: string) => Array<{
    comment?: string;
    element?: string;
    elementPath?: string;
    cssClasses?: string[];
    boundingBox?: { width: number; height: number };
  }>;
};

// page.evaluate serializes its callback — Node-scope helpers do not exist in
// the page, so every callback re-reads window.__agentation itself.

let probe: string;
test.beforeAll(async () => {
  const result = await build({
    stdin: {
      contents: readFileSync(join(here, 'agentation-probe.ts'), 'utf8'),
      resolveDir: here,
      loader: 'ts',
    },
    bundle: true,
    format: 'iife',
    // The npm react builds branch on process.env.NODE_ENV, which has no
    // meaning in a browser — leave it and the bundle dies on `process` being
    // undefined. Same substitution vite makes at build time.
    define: { 'process.env.NODE_ENV': '"production"' },
    write: false,
    logLevel: 'silent',
  });
  probe = result.outputFiles[0].text;
});

// The fixture must be served from a real origin, not page.setContent:
// setContent pages live on about:blank, where Chromium throws SecurityError
// on any localStorage access — and the toolbar reads localStorage for its
// annotation store, so the mount dies there before rendering (jsdom does not
// enforce that, which is why the vitest suite cannot catch it). Routing a
// reserved .test hostname keeps the harness serverless — the request is
// fulfilled locally, nothing touches a network.
const FIXTURE_URL = 'https://agentation-fixture.test/';

/**
 * A page with real, identifiable elements for the toolbar to point at.
 * Collects page errors so a mount that dies is reported as one.
 */
async function openFixture(page: Page): Promise<string[]> {
  const pageErrors: string[] = [];
  page.on('pageerror', e => pageErrors.push(e.message));
  await page.route(FIXTURE_URL, route =>
    route.fulfill({
      contentType: 'text/html',
      body: `
        <html><body>
          <nav><a href="/" class="nav-link">Home</a></nav>
          <main>
            <section class="card card-status">
              <h2>Status card</h2>
              <button id="demo-target" class="demo-cta" type="button">Demo target</button>
            </section>
          </main>
        </body></html>
      `,
    })
  );
  await page.goto(FIXTURE_URL, { waitUntil: 'load' });
  await page.addScriptTag({ content: probe });
  return pageErrors;
}

test.describe('agentation mounts in a real browser', () => {
  test('#agentation-root mounts after load and the toolbar renders into it', async ({ page }) => {
    const pageErrors = await openFixture(page);
    // The mount div alone is the silent failure mode the standard warns
    // about — the toolbar element inside it is the real assertion.
    await expect(page.locator('#agentation-root')).toHaveCount(1);
    const toolbar = page.locator('[data-agentation-toolbar="true"]');
    await expect(toolbar).toHaveCount(1);
    await expect(toolbar).toBeVisible();
    expect(
      pageErrors,
      `the mount must not throw: ${pageErrors.join(' | ')}`
    ).toEqual([]);
  });

  test('identifyElement names a fixture element with a path that resolves back to it', async ({ page }) => {
    await openFixture(page);
    const ident = await page.evaluate(() => {
      const el = document.querySelector('#demo-target') as HTMLElement;
      const { identifyElement } =
        (window as unknown as { __agentation: ProbeApi }).__agentation;
      return identifyElement(el);
    });
    expect(ident.name).toBeTruthy();
    expect(ident.path).toBeTruthy();
    // The identification is only useful if an agent can turn the path back
    // into the element — that round-trip is the whole point of the tool.
    const resolves = await page.evaluate(
      p => document.querySelector(p) !== null,
      ident.path
    );
    expect(resolves, `path "${ident.path}" must querySelector back to the element`).toBe(true);
  });

  test('getElementPath produces a usable selector for a nested element', async ({ page }) => {
    await openFixture(page);
    const path = await page.evaluate(() => {
      const el = document.querySelector('.card-status h2') as HTMLElement;
      const { getElementPath } =
        (window as unknown as { __agentation: ProbeApi }).__agentation;
      return getElementPath(el);
    });
    expect(path).toBeTruthy();
    const roundTrips = await page.evaluate(
      p => document.querySelector(p) !== null,
      path
    );
    expect(roundTrips, `getElementPath "${path}" must resolve in the document`).toBe(true);
  });

  test('the toolbar identifies a real element end-to-end through its annotation pipeline', async ({ page }) => {
    await openFixture(page);
    // Demo mode makes the toolbar run its own identify pipeline against
    // #demo-target and store the resulting annotation; the probe reads the
    // toolbar's own store. Waiting here proves the pipeline ran; the shape
    // proves what it identified.
    const handle = await page.waitForFunction(
      () => {
        const { loadAnnotations } =
          (window as unknown as { __agentation: ProbeApi }).__agentation;
        return loadAnnotations(location.pathname)[0] ?? null;
      },
      null,
      { timeout: 10_000 }
    );
    const ann = (await handle.jsonValue()) as Awaited<
      ReturnType<ProbeApi['loadAnnotations']>
    >[number];

    expect(ann.comment).toContain('probe annotation');
    expect(ann.element).toBeTruthy();
    expect(ann.elementPath).toBeTruthy();
    // The annotation must point at the element the demo named.
    const pointsAtTarget = await page.evaluate(
      p => document.querySelector(p)?.id === 'demo-target',
      ann.elementPath
    );
    expect(pointsAtTarget, `annotation path "${ann.elementPath}" must resolve to #demo-target`).toBe(true);
    // Real layout only — jsdom's zeros can never pass this.
    expect(ann.boundingBox?.width ?? 0).toBeGreaterThan(0);
    // And the structured context the markdown is built from is present.
    expect(ann.cssClasses).toContain('demo-cta');
  });
});
