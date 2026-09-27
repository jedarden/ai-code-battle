// Packaging contract for the Pages deploy (bead aicodeba-87095e9a): the
// frontend deploy must ship web/functions together with the frontend.
//
// Wrangler resolves the Pages Functions directory as `<cwd>/functions` —
// relative to the process working directory, NOT to the assets directory it
// ships — and silently skips the Functions bundle when that directory is
// missing (workers-sdk packages/wrangler/src/api/pages/deploy.ts:
// `join(cwd(), "functions")` guarded by `existsSync`, no warning). A deploy
// that runs anywhere but web/ therefore ships the SPA alone, and every /api
// route falls back to text/html — exactly the rot the transport
// (docs/notes/api-transport.md) exists to prevent, born invisibly at deploy
// time. The deployed side of that seam is gated after the fact
// (test-api-workflows.js in acb-site-pages-build and
// scripts/verify-deployment.sh); nothing pinned the packaging side, where
// the regression would be born. This audit does, offline, at the default
// gate — the same audit-as-test shape as src/agentation-entries.test.ts.

import { readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';

const libDir = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(libDir, '..', '..');
const repoRoot = resolve(webRoot, '..');

describe('Pages deploy packaging (functions ship with the frontend)', () => {
  it('keeps the Pages Functions catch-alls beside the shipped assets dir', () => {
    // The /api catch-all the platform routes every same-origin /api/* request
    // to. Its behavior is pinned by api-function-adapter.test.ts, which
    // imports this very file; here it is the packaging that must exist, so
    // assert on the bytes a missing/moved directory would take with it.
    const apiFn = readFileSync(join(webRoot, 'functions', 'api', '[[path]].ts'), 'utf8');
    expect(apiFn).toContain('onRequest');
    // The R2 function ships from the same directory, same rule.
    const r2Fn = readFileSync(join(webRoot, 'functions', 'r2', '[[path]].ts'), 'utf8');
    expect(r2Fn).toContain('onRequest');
  });

  it('web/wrangler.toml points wrangler at dist/ inside web/', () => {
    const cfg = readFileSync(join(webRoot, 'wrangler.toml'), 'utf8');
    // web-relative on purpose: from web/ it must resolve to the same assets
    // directory the functions/ folder is a sibling of. A `web/dist` value
    // here would mean the deploy was meant to run from the repo root — where
    // <cwd>/functions resolves to nothing.
    expect(cfg).toMatch(/^pages_build_output_dir\s*=\s*"dist"$/m);
  });

  it('the manual deploy script runs wrangler from web/, not the repo root', () => {
    const script = readFileSync(join(repoRoot, 'scripts', 'deploy-pages.sh'), 'utf8');
    // The deploy invocation must execute inside web/ — where
    // <cwd>/functions resolves to web/functions. A repo-root
    // `pages deploy web/dist` silently ships no Functions at all.
    expect(script).toMatch(/cd\s+web\b[^\n]*&&[^\n]*wrangler[^\n]*pages\s+deploy\s+dist\b/);
    // And it must trip before deploying if the catch-all is missing.
    expect(script).toContain('web/functions/api/[[path]].ts');
  });

  it('the transport decision record documents the packaging rule', () => {
    const doc = readFileSync(join(repoRoot, 'docs', 'notes', 'api-transport.md'), 'utf8');
    // The rule and its silent-skip failure mode, in the deploy's own docs.
    expect(doc).toContain('cwd()/functions');
    expect(doc).toContain('deploy-packaging.test.ts');
  });
});
