// Audit-as-test for the workspace Agentation standard ("agentation goes on
// EVERY page — each HTML entry point wires it independently", verified by
// mounting, never by grepping for a tag).
//
// agentation-overlay.test.ts proves the mount mechanism itself (initAgentation
// creates #agentation-root and the toolbar actually renders into it, in
// jsdom). agentation-mount.spec.ts (layout-tests) proves it again in real
// Chromium. Neither of those can notice a page that never *calls* it — a
// silently unwired entry point renders perfectly and just carries no toolbar,
// which is exactly the failure mode the standard warns about. This file pins
// the wiring: every web-root HTML entry point must reach initAgentation
// through the entry module it loads.
//
// Import maps: the workspace rule requires an import map when a page loads
// agentation as a raw browser module (agentation.js imports bare `react`
// specifiers no browser can resolve, and the module dies silently without
// one). This repo never does that — every entry loads a bundled /src/*.ts
// module and vite resolves react at build time (manualChunks 'agentation') —
// so the audit asserts that shape and rejects a raw agentation.js script tag,
// which would need an import map to work.

import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

const htmlFiles = readdirSync(webRoot).filter(f => f.endsWith('.html')).sort();

/** src="..." value of every external module script in an entry page. */
function moduleSrcs(html: string): string[] {
  return [...html.matchAll(/<script\b[^>]*type="module"[^>]*>/g)]
    .map(tag => tag[0].match(/\ssrc="([^"]+)"/)?.[1])
    .filter((s): s is string => !!s);
}

describe('agentation on every web entry point', () => {
  it('finds the vite build inputs — the audit must never scan nothing', () => {
    // The three inputs in vite.config.ts rollupOptions.input. If one goes
    // away on purpose, update this list in the same commit; if an HTML file
    // appears or disappears by accident, this is the tripwire.
    expect(htmlFiles).toContain('index.html');
    expect(htmlFiles).toContain('replay.html');
    expect(htmlFiles).toContain('embed.html');
  });

  it('loads no raw agentation module that would need an import map', () => {
    for (const file of htmlFiles) {
      const html = readFileSync(join(webRoot, file), 'utf8');
      // A page that switches to <script type="module" src=".../agentation.js">
      // must add the react import map from ~/.claude/rules/agentation.md in
      // the same change — the tag alone mounts nothing.
      expect(html, `${file} must not load agentation as a raw browser module`).not.toMatch(
        /<script\b[^>]*src="[^"]*agentation\.js"/
      );
    }
  });

  for (const file of htmlFiles) {
    it(`${file} reaches initAgentation through its entry module`, () => {
      const html = readFileSync(join(webRoot, file), 'utf8');
      const srcs = moduleSrcs(html);
      expect(
        srcs.length,
        `${file} loads no external module script — a page with no wired entry cannot carry the toolbar. Load a /src module that calls initAgentation.`
      ).toBeGreaterThan(0);

      for (const src of srcs) {
        expect(
          src,
          `${file}: module scripts must bundle from /src (vite-resolved, no import map needed); got "${src}"`
        ).toMatch(/^\/src\//);

        const entrySource = readFileSync(join(webRoot, src), 'utf8');
        expect(
          entrySource,
          `${src} (the entry module ${file} loads) never calls initAgentation — the page renders fine but carries no Agentation toolbar. Mount it in the entry (async import('./agentation-overlay').then(m => m.initAgentation()) is the pattern used by app.ts and embed.ts).`
        ).toMatch(/initAgentation\s*\(/);
      }
    });
  }
});
