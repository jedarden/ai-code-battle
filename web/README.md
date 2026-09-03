# acb-web

AI Code Battle frontend (React + Vite). Deployed via Argo Workflows to Cloudflare Pages.

## Tests

Two suites, deliberately separate:

| Suite | Engine | What it can check | Command |
|---|---|---|---|
| `src/**/*.test.ts` | vitest + jsdom | DOM structure, CSS *text* parsing | `npm run test:run` |
| `layout-tests/*.spec.ts` | Playwright + headless Chromium | **rendered geometry** (`getBoundingClientRect`, media queries) | `npm run test:browser` |

jsdom has no layout engine — every `getBoundingClientRect()` there returns
zeros and no media query is ever evaluated. Anything that needs real laid-out
geometry (e.g. leaderboard skeleton-vs-live parity) belongs in
`layout-tests/`; `layout-tests/fixture.ts` builds the page (skeleton output
next to a live-classed `.lb-row` / `.mobile-cards` fixture with the app
stylesheets inlined) and `layout-tests/measure.ts` reads geometry off it.

### One-time browser download

```sh
npm run test:browser:install   # downloads Chromium into ~/.cache/ms-playwright
```

### NixOS hosts

The downloaded Chromium cannot launch on NixOS (no FHS `libglib-2.0`, no sudo
to add it). Point the harness at the nix store Chromium instead — nix patches
its RPATH so it runs as-is, and Playwright drives it over CDP regardless of
version:

```sh
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH="$(nix-build '<nixpkgs>' -A chromium --no-out-link)/bin/chromium" \
  npm run test:browser
```
