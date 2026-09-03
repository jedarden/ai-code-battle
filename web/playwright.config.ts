import { defineConfig } from '@playwright/test';

/**
 * Rendered-layout harness (web/layout-tests) — deliberately separate from the
 * vitest/jsdom suite, which has no layout engine. Runs headless Chromium
 * against the fixture built in layout-tests/fixture.ts; no webServer is
 * needed because the fixture is set via page.setContent.
 *
 * One-time setup (downloads Chromium into ~/.cache/ms-playwright):
 *   npm run test:browser:install
 * Run:
 *   npm run test:browser
 *
 * On a NixOS box the downloaded Chromium cannot launch (no FHS libglib and no
 * sudo to install it). Point PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH at the nix
 * store Chromium instead — nix patches its RPATH, so it runs as-is and
 * Playwright drives it over CDP regardless of version:
 *   PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=$(nix-build '<nixpkgs>' -A chromium \
 *     --no-out-link)/bin/chromium npm run test:browser
 * Unset (the normal case), the config falls back to the downloaded browser.
 */
export default defineConfig({
  testDir: './layout-tests',
  testMatch: '**/*.spec.ts',
  fullyParallel: true,
  forbidOnly: true,
  reporter: 'list',
  use: {
    launchOptions: {
      // undefined on a normal machine -> Playwright's own Chromium.
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    },
  },
  projects: [
    // Desktop: .mobile-cards hidden (min-width: 1024px block), .lb-row at full
    // width with all seven columns visible.
    { name: 'desktop', use: { viewport: { width: 1280, height: 800 } } },
    // Phone: .mobile-cards flex (max-width: 639px block), .lb-wl/.lb-status
    // collapsed (max-width: 768px block).
    { name: 'phone', use: { viewport: { width: 390, height: 844 } } },
  ],
});
