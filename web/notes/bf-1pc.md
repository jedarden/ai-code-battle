# bf-1pc: Fix SPA TypeScript - Replace /r2/ with /b2/

## Status: ✓ Already Complete

This task was completed in commit `76369b5` on 2026-06-16.

## Changes Made

### Commit 76369b5: "fix(spa): replace /r2/ data path prefix with /b2/"
- **File changed:** `web/src/api-types.ts`
- **Change:** `const R2_COMMENTARY_BASE = '/r2';` → `const R2_COMMENTARY_BASE = '/b2';`
- **Lines:** 1 insertion(+), 1 deletion(-)

### Commit 724f516: "fix(spa): replace r2.aicodebattle.com with b2.aicodebattle.com"
- Updated domain references (earlier commit)

## Acceptance Criteria Verification (2026-06-16)

1. ✓ `grep -r "/r2/" web/src/` returns no hits
2. ✓ `npm run build` succeeds (completed in 2.65s)
3. ✓ `grep -r "r2.aicodebattle" web/dist/` returns no hits

## Current State

The constant in `src/api-types.ts`:
```typescript
const R2_COMMENTARY_BASE = '/b2';
```

All data-fetch URLs now use `/b2/` prefix and `b2.aicodebattle.com` subdomain.
