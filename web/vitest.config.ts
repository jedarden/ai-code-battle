import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    // The .tsx entry is load-bearing, not cosmetic: a positional path passed
    // to `vitest run` is only collected when it matches `include`, and an
    // unmatched path is dropped *silently* — the run still exits green with
    // that file's tests never executed (SkeletonScreen.test.tsx sat in this
    // state for two days: committed, but running under a 35-test gate that
    // reported 59).
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    setupFiles: ['./src/test-setup.ts'],
  },
});
