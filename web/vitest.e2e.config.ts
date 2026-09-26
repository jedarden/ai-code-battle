import { defineConfig } from 'vitest/config';

/**
 * Runner for the live /api flow suite (web/e2e/) — bead aicodeba-6c793229.
 *
 * Deliberately a separate config from vitest.config.ts: these tests need the
 * network (they complete the register / predictions / map-vote / feedback
 * flows against the real origin), so the default `npm test` include (the
 * src-tree test glob) must never collect them and the offline gate stays
 * deterministic. The include here only matches `e2e/`, which also means a
 * positional path typo fails collection instead of exiting green with
 * nothing run (the silent-drop trap recorded in vitest.config.ts).
 *
 * Invoke via `npm run test:e2e-api`; ACB_ORIGIN overrides the target origin.
 */
export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['e2e/**/*.e2e.test.ts'],
    setupFiles: ['./src/test-setup.ts'],
    // Every test makes several real round trips to the origin; leave room
    // for edge latency instead of timing out a healthy but distant origin.
    testTimeout: 30_000,
    hookTimeout: 30_000,
  },
});
