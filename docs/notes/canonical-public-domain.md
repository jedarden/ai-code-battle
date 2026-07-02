---
name: canonical-public-domain
description: Standardized on ai-code-battle.pages.dev as canonical public domain
metadata:
  type: project
---

# Canonical Public Domain Decision

## Decision

**Standardized on `ai-code-battle.pages.dev` as the canonical public domain.**

## Background

The domain `aicodebattle.com` is NXDOMAIN (not registered). The site is deployed and functional at `ai-code-battle.pages.dev`. References to `aicodebattle.com` and `b2.aicodebattle.com` existed in documentation and test files.

## Rationale

- Registering `aicodebattle.com` requires human action and payment
- Cloudflare Pages provides a production-ready public URL at `ai-code-battle.pages.dev`
- R2 storage is now served through Pages Functions (`/r2/*` path), eliminating the need for a separate `b2.aicodebattle.com` CDN host
- Standardizing on pages.dev ensures all user-facing URLs work immediately

## Changes Made

### Updated Files

1. **web/public/replay-schema-v1.json**
   - Updated `$id` field from `https://aicodebattle.com/replay-schema-v1.json` to `https://ai-code-battle.pages.dev/replay-schema-v1.json`

2. **web/public/robots.txt**
   - Updated sitemap URL from `https://aicodebattle.com/sitemap.xml` to `https://ai-code-battle.pages.dev/sitemap.xml`

3. **web/public/test-match-list.html**
   - Updated thumbnail URL from `https://b2.aicodebattle.com/thumbnails/...` to `https://ai-code-battle.pages.dev/r2/thumbnails/...`

### Files Already Correct

- **web/src/og-tags.ts** - Already uses `https://ai-code-battle.pages.dev`
- **web/src/pages/playlists.ts** - Uses dynamic `window.location.origin` for embed URLs
- **web/src/pages/clip-maker.ts** - Twitter share uses external domain (correct)
- **web/index.html** - OG tags already use pages.dev
- **web/embed.html** - OG tags already use pages.dev

## URL Pattern

All user-facing absolute URLs now follow the pattern:

```
https://ai-code-battle.pages.dev/<path>
```

Dynamic URLs use `window.location.origin` which resolves to pages.dev in production.

## Replay and Data URLs

Replays and match data are served through Cloudflare Pages Functions:

- Replays: `https://ai-code-battle.pages.dev/r2/replays/{match_id}.json.gz`
- Match metadata: `https://ai-code-battle.pages.dev/r2/matches/{match_id}.json`
- Thumbnails: `https://ai-code-battle.pages.dev/r2/thumbnails/{match_id}.png`
- Bot cards: `https://ai-code-battle.pages.dev/r2/cards/{bot_id}.png`
- Evolution status: `https://ai-code-battle.pages.dev/r2/evolution/live.json`

## Future Consideration

If `aicodebattle.com` is registered later, it can be added as a custom domain in Cloudflare Pages. The internal URLs will remain valid as pages.dev will continue to work as the origin.

## Verified

All user-facing absolute URLs emitted by the web application resolve correctly in a browser.
