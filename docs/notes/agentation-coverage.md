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
   pages mount and identify**: vite-builds the actual inputs, serves them to
   headless Chromium over route fulfillment, asserts `#agentation-root` + a
   visible `[data-agentation-toolbar="true"]` on each, tripwires when the
   build emits a page the spec doesn't know, and — per page — drives the
   toolbar's real annotate flow (Ctrl+Shift+F, click the page's own DOM,
   comment in the popup) and proves the identify pipeline from the toolbar's
   own store: the stored annotation's `elementPath` resolves back inside the
   clicked element with a non-zero bounding box, and a marker renders.
   (Demo mode is not drivable on the built pages — `initAgentation` passes
   no demo props — so the UI is the path; see the spec header.)

Layer 4 runs with `npm run test:browser` (desktop project; needs the nix
Chromium on NixOS — see `web/playwright.config.ts`).

**Final verdict 2026-09-27 (bead `aicodeba-4fb184a8`): layer 4 is green on
every committed tree; the 2026-09-26/27 red records are retracted as an
audit artifact, not a tree defect.** Those records (beads
`aicodeba-11b6f07f` / `aicodeba-88d11430`) reported all three built pages
mounting the toolbar but throwing uncaught pageerrors — embed
`null.addEventListener` + TDZ `F8`, index `null.getContext` + TDZ `F8`,
replay `null.addEventListener` — from entry chunk `main-XjMtku3y.js`. That
chunk cannot be produced by any committed tree: with the pinned toolchain
every committed tree emits a 3-line minified `main` (`main-ZyLcA4yJ`, the
hash production serves) with react inside the `agentation` chunk, while the
inventory cites positions up to `:4977` in `main` — an unminified
~5000-line entry chunk, i.e. a build in which the `manualChunks` split and
minification never ran. No committed config produces that shape
(`vite build --minify false` yields `main-Di1Bk8nm`, 337 lines), and
`web/package-lock.json` is byte-identical across `dec7648..HEAD` and fully
integrity-pinned, so `npm ci` cannot vary the inputs either. Re-running the
red runs' own protocol (git-archive extraction, `npm ci`, nix Chromium)
proves it: at `96e9583` — the exact commit twice recorded red — layer 4
passes 4/4, and at HEAD `e7cbf69` 7/7, with the full stack green (9/9
vitest layers 1+3, 8/8 fixture, 7/7 built pages, live 3 passed + 3
skipped). The red runs therefore built something other than the committed
tree — in-flight workspace edits, the same contamination that produced the
earlier false "4/4 passed" record. The throwing bundle never reached
production (the live origin serves `main-ZyLcA4yJ`; layer 5 green).
`aicodeba-88d11430` has no code defect left to fix.

Live verdict 2026-09-27 (bead `aicodeba-199a231a`): 7/7 desktop tests green —
per page beyond the mount, the identify tests now drive the real annotate
flow (Ctrl+Shift+F → click the page's own DOM → comment in the popup) and
read the toolbar's own store: the stored annotation's `elementPath` resolves
back inside the clicked element with a non-zero bounding box and a rendered
marker. Click targets: `index.html` → `nav`, `embed.html` →
`.embed-container` (the harness 404s every API path, so embed sits in its
error-overlay state — that overlay is the app DOM the flow identifies),
`replay.html` → `.canvas-wrapper`.

## Production

The four layers above all run against repo artifacts; nothing they do can
see a deployment regression. `web/layout-tests/agentation-live-mount.spec.ts`
is the fifth layer — headless Chromium against the deployed origin, run with
`npm run test:agentation-live` (live-only, skipped unless `ACB_LIVE_ORIGIN`
is set; override it to audit a Pages preview before promotion). Per route —
`/`, `/replay`, `/embed` — it asserts the response is text/html, the served
document carries its own vite entry chunk in a script tag (a route this
origin does not deploy does not 404: Cloudflare Pages answers with a 200
text/html SPA fallback serving index.html, whose toolbar mounts and would
pass a mount-only check), and `#agentation-root` plus a visible
`[data-agentation-toolbar="true"]` mount after load. Verdicts are recorded
on the bead that runs them.

Production verdict 2026-09-27 (bead `aicodeba-71824b16`): **all three routes
mount** — 3 passed, 3 skipped (phone project), exit 0, against
`https://ai-code-battle.pages.dev` serving entry chunks `main-ZyLcA4yJ.js`,
`replay-U4tqtRgK.js`, `embed-DiOqgewn.js` (agentation chunk
`agentation-blICokqV.js`). No fix bead filed; nothing failed.

## Adding a page

1. Wire `initAgentation()` in its entry module (async import is the
   pattern; `initAgentation` is idempotent). Layer 3 fails if you forget.
2. Add it to `vite.config.ts` `rollupOptions.input` if it ships. Layer 4's
   tripwire fails until the new page joins its `PAGES` list.
3. Add a row to the table above. Layers 1–2 need nothing per page.
