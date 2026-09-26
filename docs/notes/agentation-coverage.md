# Agentation coverage — audited page list

Audited 2026-09-26 (bead `aicodeba-a58e2876`). Standard:
`~/.claude/rules/agentation.md` — Agentation goes on **every page**, each
HTML entry point wires it independently, and verification is **by mounting**
(`#agentation-root` exists after load, with the toolbar in it), never by
grepping for a script tag.

## Audited pages

All HTML entry points live at the `web/` root; the audit enumerates that
directory, so a page added anywhere else must move the enumeration (see
"Adding a page" below).

| Page | Entry module | How it mounts | Shipped? |
|---|---|---|---|
| `web/index.html` | `/src/app.ts` | `DOMContentLoaded` → async `import('./agentation-overlay')` → `initAgentation()` | yes — vite input `main` |
| `web/replay.html` | `/src/main.ts` | module eval calls `initAgentation()` directly | yes — vite input `replay` |
| `web/embed.html` | `/src/embed.ts` | `DOMContentLoaded` → async `import('./agentation-overlay')` → `initAgentation()` | yes — vite input `embed` |
| `web/test-replay-viewer.html` | `/src/main.ts` | same mount as `replay.html` (shared entry module) | no — internal test harness page, not a vite input, never deployed |

The SPA's hash routes (`#/watch`, `#/leaderboard`, …) are one page
(`index.html`): the toolbar is a fixed overlay on `<body>`, so a single
mount covers every route. `web/dist/*.html` are build outputs, not entry
points; `web/functions/` serves the API, no HTML.

## Import map shape (and why the pages have none)

The rule requires an import map only when a page loads agentation as a raw
browser module (`agentation.js` imports bare `react` specifiers no browser
resolves). This repo never does that: every entry loads a bundled
`/src/*.ts` module and vite resolves `react` at build time (the
`manualChunks` `'agentation'` chunk). The audit asserts that shape and
rejects a raw `agentation.js` script tag — a tag like that, without an
import map, is exactly the silent-failure state the standard warns about.

## Verification layers (all four must stay green)

1. `web/src/agentation-overlay.test.ts` — `initAgentation()` creates
   `#agentation-root` and the toolbar renders into it, under jsdom.
2. `web/layout-tests/agentation-mount.spec.ts` — the mount mechanism again,
   in real Chromium, on a synthetic fixture (proves real-layout annotation:
   non-zero boundingBox, selector round-trips).
3. `web/src/agentation-entries.test.ts` — **wiring per entry point**: every
   web-root `*.html` must reach `initAgentation` through the `/src` module
   it loads. Enumerates the directory at run time — a new page is audited
   automatically, no list to update.
4. `web/layout-tests/agentation-pages-mount.spec.ts` — **the real built
   pages mount**: vite-builds the actual inputs, serves them to headless
   Chromium over route fulfillment, asserts `#agentation-root` + a visible
   `[data-agentation-toolbar="true"]` on each, and tripwires when the build
   emits a page the spec doesn't know.

Layer 4 runs with `npm run test:browser` (desktop project; needs the nix
Chromium on NixOS — see `web/playwright.config.ts`). Run on 2026-09-26
against commit `2fa0d27` + the coverage files: 4/4 passed (tripwire + three
page mounts).

## Adding a page

1. Wire `initAgentation()` in its entry module (async import is the
   pattern; `initAgentation` is idempotent). Layer 3 fails if you forget.
2. Add it to `vite.config.ts` `rollupOptions.input` if it ships. Layer 4's
   tripwire fails until the new page joins its `PAGES` list.
3. Add a row to the table above. Layers 1–2 need nothing per page.
