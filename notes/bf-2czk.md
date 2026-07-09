# Audit: User-Facing URL References in web/src

## Summary
All user-facing URLs in the web source use `ai-code-battle.pages.dev` as the canonical domain. The legacy domains `aicodebattle.com` and `b2.aicodebattle.com` only appear in comments indicating they are outdated/NXDOMAIN.

## Canonical Domain Usage (ai-code-battle.pages.dev)

### Open Graph Tags (web/src/og-tags.ts)
- **Lines 14-15**: Default OG tags URL and image
- **Line 76**: Bot profile OG tags (`https://ai-code-battle.pages.dev/#/bot/${bot.id}`)
- **Line 97**: Replay OG tags (`https://ai-code-battle.pages.dev/#/watch/replay/${match.id}`)
- **Line 115**: Playlist OG tags (`https://ai-code-battle.pages.dev/#/watch/replays`)

### Embed URLs (web/src/embed.ts)
- **Line 18**: `PAGES_BASE = 'https://ai-code-battle.pages.dev'`
- **Line 205**: Embed URL construction for OG tags (`${PAGES_BASE}/embed/${matchId}`)

### Clip Maker Share URLs (web/src/pages/clip-maker.ts)
- **Line 453**: Share URL for replay clips (`https://ai-code-battle.pages.dev/replay/${matchId}#turns=${startTurn}-${endTurn}`)

### Playlists Embed Codes (web/src/pages/playlists.ts)
- **Line 539**: Dynamic embed URL using `window.location.origin` - this adapts to whatever domain the user is currently on
  ```javascript
  const embedUrl = `${window.location.origin}/embed/${matchId}`;
  ```

### Bot Card Images (web/src/components/bot-card.ts)
- **Line 264**: Renders text "ai-code-battle.pages.dev" on card images

### API Documentation (web/src/pages/docs-api.ts)
- **Line 19**: `PAGES_BASE = 'https://ai-code-battle.pages.dev'`
- **Line 22**: `B2_BASE = 'https://ai-code-battle.pages.dev/r2'`
- **Line 338**: Schema `$id` URL
- **Line 552**: Example replay URL

### Data Documentation (web/src/pages/docs-data.ts)
- **Lines 29, 86, 132, 138, 143, 148**: curl examples with `https://ai-code-battle.pages.dev`

### Registration Documentation (web/src/pages/docs.ts)
- **Line 88**: Email contact `hello@ai-code-battle.pages.dev`
- **Line 90**: API endpoint `https://api.ai-code-battle.pages.dev/api/register`

### Sandbox Page (web/src/pages/sandbox.ts)
- **Line 175**: Displays sandbox URL `ai-code-battle.pages.dev/#/compete/sandbox`

### Replay Format Documentation (web/src/pages/docs-replay-format.ts)
- **Lines 23, 26**: curl examples with `https://ai-code-battle.pages.dev/r2/replays/`

## Legacy Domain References (Comments Only)

### API Documentation (web/src/pages/docs-api.ts)
- **Lines 21-22**: Comment stating `b2.aicodebattle.com` and `aicodebattle.com` are NXDOMAIN (not registered)

### Data Documentation (web/src/pages/docs-data.ts)
- **Line 19**: Comment stating legacy references are outdated

## Dynamic URL Construction

### Potential Concern
- **web/src/pages/playlists.ts:539**: Uses `window.location.origin` to construct embed codes
  - This will use whatever domain the user is currently accessing the site from
  - If accessed via aicodebattle.com (if DNS were to be pointed there), embed codes would reference that domain
  - Currently safe since aicodebattle.com is NXDOMAIN

## Recommendation
The codebase is properly standardized on `ai-code-battle.pages.dev` as the canonical public domain. The only dynamic URL construction (`window.location.origin` in playlists.ts) is appropriate for embed codes since it adapts to the current access domain. No changes needed.
