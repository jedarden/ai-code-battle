---
name: bf-5kk-canonical-domain-decision
description: Decision to use ai-code-battle.pages.dev as canonical public domain instead of registering aicodebattle.com
metadata:
  type: project
---

# Canonical Public Domain Decision

**Date:** 2026-07-02
**Decision:** Use `ai-code-battle.pages.dev` as the canonical public domain
**Status:** ACTIVE

## Problem

The domain `aicodebattle.com` is NXDOMAIN (not registered/no DNS zone). The site is only reachable at `ai-code-battle.pages.dev`. Documentation throughout the codebase references `aicodebattle.com` as the canonical domain.

## Options Considered

### Option A: Register aicodebattle.com and attach as custom domain
**Status:** BLOCKED - Requires human action + payment
- Register domain (~$10-15/year)
- Configure Cloudflare custom domain for Pages project
- Update DNS and SSL
- Deferred to future when needed for branding

### Option B: Standardize on ai-code-battle.pages.dev ✅ SELECTED
**Status:** IMPLEMENTED
- No registration cost or DNS setup required
- Already works correctly (HTTP 200 verified)
- Web code already uses pages.dev URLs
- Only requires documentation updates
- Future migration path: add custom domain later, update URLs then

## Implementation

### Web Code Status
The following files **already use** `ai-code-battle.pages.dev` correctly:
- `web/src/og-tags.ts` - All OG tags use pages.dev ✅
- `web/src/pages/clip-maker.ts` - Share URLs use pages.dev ✅
- `web/src/pages/sandbox.ts` - Mobile notice references pages.dev ✅
- `web/src/pages/*.ts` - All shareable/replay URLs use pages.dev ✅

### Documentation Updates Required
The following references `aicodebattle.com` and need updates:
- `docs/plan/plan.md` - Multiple references to aicodebattle.com for shareable URLs
- `docs/plan/plan.md` - References to b2.aicodebattle.com CDN (outdated - not used, current code uses R2 via Pages Functions at `web/functions/r2/[[path]].ts`)
- `web/src/pages/docs.ts` - Example POST to api.aicodebattle.com (deferred API, doesn't exist)

### Changes Made

1. **Updated docs/plan/plan.md** - Replaced shareable URL examples from `aicodebattle.com` to `ai-code-battle.pages.dev`
2. **Updated docs/plan/plan.md** - Added note that B2 CDN references are legacy, current implementation uses R2 via Pages Functions
3. **Updated web/src/pages/docs.ts** - Marked api.aicodebattle.com examples as deferred/planned

### Verification

All user-facing absolute URLs emitted by web/src now resolve correctly:
- ✅ https://ai-code-battle.pages.dev/ - Landing page
- ✅ https://ai-code-battle.pages.dev/#/bot/{bot_id} - Bot profiles
- ✅ https://ai-code-battle.pages.dev/#/watch/replay/{match_id} - Replay viewer
- ✅ https://ai-code-battle.pages.dev/replay/{match_id}#turns=X-Y - Clip share URLs

## Future Migration Path

If/when `aicodebattle.com` is registered and configured:
1. Add custom domain in Cloudflare Pages dashboard
2. Search-and-replace `ai-code-battle.pages.dev` → `aicodebattle.com` in:
   - docs/plan/plan.md
   - web/src/og-tags.ts
   - web/src/pages/clip-maker.ts
   - web/src/pages/sandbox.ts
3. Deploy and verify all redirects work
4. Update this note with migration date

## Rationale

Using the Cloudflare Pages-provided domain (`ai-code-battle.pages.dev`) is the pragmatic choice:
- **Zero cost** - No domain registration or DNS hosting fees
- **Zero configuration** - Works out of the box with Pages deployment
- **Zero delay** - No DNS propagation or SSL setup time
- **Already implemented** - Web code already uses pages.dev correctly
- **Reversible** - Can add custom domain later without breaking URLs

The only downside is the less-branded domain name, which is acceptable for a pre-launch/prototype phase. When the project reaches production readiness and branding becomes important, the custom domain can be added with a simple find-replace and deploy.

## References

- Task: bf-5kk - Resolve canonical public domain
- Cloudflare Pages custom domains: https://developers.cloudflare.com/pages/platform/custom-domains/
- R2 Pages Functions: web/functions/r2/[[path]].ts
