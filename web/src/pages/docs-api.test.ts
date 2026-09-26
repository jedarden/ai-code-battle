/**
 * API Reference page honesty contract (bead aicodeba-4be62ac3).
 *
 * The 2026-09-26 static-endpoint decision (docs/notes/public-api-descope.md,
 * "Static-data endpoint decision") retired the page's advertisements of asset
 * URLs nothing serves: a missing Pages asset answers the 200 text/html SPA
 * fallback, so advertising such a URL repeats the exact "client gets HTML"
 * misrepresentation the 2026-09-25 descope removed from docs.ts. These tests
 * pin the resulting contract: only verified-live paths in the Pages and
 * Interactive sections, the corrected blog-post path, the retired B2-origin
 * paths gone, and the replay/media pipeline documented only under its OFFLINE
 * marking — the static-tier analogue of the match-tier 503 contract.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { renderDocsApiPage, docsApiSections } from './docs-api';

// The retired B2-origin forms: b2.aicodebattle.com is dead and the Pages
// origin never served these paths — each answered the SPA fallback HTML.
const RETIRED_B2_ORIGIN_PATHS = [
  '/evolution/live.json',
  '/replays/{match_id}.json.gz',
  '/matches/{match_id}.json',
  '/cards/{bot_id}.png',
  '/thumbnails/{match_id}.png',
];

function allPaths(): string[] {
  return docsApiSections.flatMap((s) => s.endpoints.map((e) => e.path));
}

function sectionTitles(): string[] {
  return docsApiSections.map((s) => s.title);
}

describe('docs-api page honesty contract', () => {
  it('advertises the real blog-post path, not the dead /data/blog/{slug} form', () => {
    const paths = allPaths();
    expect(paths).toContain('/data/blog/posts/{slug}.json');
    expect(paths).not.toContain('/data/blog/{slug}.json');
  });

  it('advertises no retired B2-origin asset path and keeps no B2 section', () => {
    const paths = allPaths();
    for (const retired of RETIRED_B2_ORIGIN_PATHS) {
      expect(paths).not.toContain(retired);
    }
    expect(sectionTitles()).not.toContain('B2 Endpoints (Warm Cache)');
    expect(sectionTitles()).not.toContain('B2 Endpoints (Archive)');
  });

  it('marks every replay/media pipeline endpoint OFFLINE', () => {
    const pipeline = docsApiSections.find((s) => s.title.includes('Pipeline Offline'));
    expect(pipeline).toBeDefined();
    expect(pipeline!.endpoints.length).toBeGreaterThan(0);
    for (const endpoint of pipeline!.endpoints) {
      expect(endpoint.cache.startsWith('OFFLINE')).toBe(true);
    }
    // The documented bases are the builder's real output paths and the SPA's
    // own fetch bases — not the retired origin-root forms.
    const paths = pipeline!.endpoints.map((e) => e.path);
    expect(paths).toContain('/data/replays/{match_id}.json.gz');
    expect(paths).toContain('/data/evolution/live.json');
    expect(paths).toContain('/maps/index.json');
  });

  it('keeps the live Pages data tier advertised, entirely under /data/', () => {
    const pages = docsApiSections.find((s) => s.title.startsWith('Pages Endpoints'));
    expect(pages).toBeDefined();
    expect(pages!.endpoints.length).toBeGreaterThan(0);
    for (const endpoint of pages!.endpoints) {
      expect(endpoint.path.startsWith('/data/')).toBe(true);
    }
  });

  it('keeps the interactive /api contract documented, match-tier routes included', () => {
    const paths = allPaths();
    expect(paths).toContain('/api/health');
    expect(paths).toContain('/api/register');
    expect(paths).toContain('/api/predict');
  });

  it('renders the offline marking and the live interactive section, and no card serves a retired path', () => {
    document.body.innerHTML = '<div id="app"></div>';
    renderDocsApiPage();
    const html = document.getElementById('app')!.innerHTML;
    expect(html).toContain('Pipeline Offline');
    expect(html).toContain('Interactive API');
    expect(html).toContain('/api/health');
    expect(html).toContain('/data/blog/posts/{slug}.json');
    // An endpoint card puts the origin and the path in adjacent nodes, so a
    // retired advertisement would render as `</span><path>` — the designed
    // /data/... forms never produce that substring.
    for (const retired of RETIRED_B2_ORIGIN_PATHS) {
      expect(html).not.toContain(`</span>${retired}`);
    }
  });
});
