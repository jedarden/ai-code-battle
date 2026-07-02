# URL Audit Report: User-Facing URL References in web/src/

**Bead:** bf-2czk  
**Date:** 2026-07-02  
**Scope:** Audit all user-facing URL references to aicodebattle.com and b2.aicodebattle.com

## Executive Summary

**Status:** ✅ CLEAN - No user-facing references to old domains found

All user-facing URLs correctly use `ai-code-battle.pages.dev`. The old domains `aicodebattle.com` and `b2.aicodebattle.com` only appear in documentation comments noting they are outdated/NXDOMAIN.

## Files with User-Facing URLs

### 1. web/src/og-tags.ts (Lines 14, 15, 76, 97, 115)
**Purpose:** Open Graph meta tags for social sharing

- **Line 14:** Default site URL: `https://ai-code-battle.pages.dev/`
- **Line 15:** Default OG image: `https://ai-code-battle.pages.dev/img/og-default.png`
- **Line 76:** Bot profile URLs: `https://ai-code-battle.pages.dev/#/bot/${bot.id}`
- **Line 97:** Replay URLs: `https://ai-code-battle.pages.dev/#/watch/replay/${match.id}`
- **Line 115:** Playlist URLs: `https://ai-code-battle.pages.dev/#/watch/replays`

**Context:** These URLs are emitted into `<meta>` tags for social media previews (Facebook, Twitter, LinkedIn, etc.). When users share bot profiles or replays, these domains appear in card previews.

---

### 2. web/src/pages/clip-maker.ts (Line 453)
**Purpose:** Share/clip URL construction for social sharing

- **Line 453:** `return 'https://ai-code-battle.pages.dev/replay/${matchId}#turns=${startTurn}-${endTurn}';`

**Context:** This URL is:
- Copied to clipboard when users click "Copy" in the share panel
- Shared to Twitter via intent URL (line 495)
- Shared to Reddit as markdown (line 499)
- Displayed in share panel text (line 463)

**Function signature:** `function replayURL(matchId: string, startTurn: number, endTurn: number): string`

---

### 3. web/src/embed.ts (Lines 18, 205)
**Purpose:** Embeddable replay viewer widget

- **Line 18:** `const PAGES_BASE = 'https://ai-code-battle.pages.dev';`
- **Line 205:** `const embedUrl = '${PAGES_BASE}/embed/${replay.match_id}';`

**Context:** The `embedUrl` is written to:
- Open Graph `og:url` meta tag (line 207)
- Twitter `twitter:player` meta tag (line 213)

Used when external sites embed the ACB replay viewer via iframe.

---

### 4. web/src/pages/docs-api.ts (Lines 19, 22, 552)
**Purpose:** API documentation page

- **Line 19:** `const PAGES_BASE = 'https://ai-code-battle.pages.dev';`
- **Line 22:** `const B2_BASE = 'https://ai-code-battle.pages.dev/r2';` (R2 via Pages Functions, not legacy B2)
- **Line 552:** `const url = 'https://ai-code-battle.pages.dev/r2/replays/${matchId}.json.gz';`

**Context:** These URLs appear in code examples and curl commands in the API documentation. Users copy these URLs to fetch replay data.

---

### 5. web/src/pages/docs-data.ts (Lines 29, 86, 132, 138, 143, 148)
**Purpose:** Public data documentation page

**User-facing curl examples:**
- **Line 29:** `curl https://ai-code-battle.pages.dev/data/leaderboard.json | jq`
- **Line 86:** `curl https://ai-code-battle.pages.dev/r2/replays/m_7f3a9b2c.json.gz`
- **Line 132:** `curl https://ai-code-battle.pages.dev/data/leaderboard.json | jq '.entries[:5]'`
- **Line 138:** `curl https://ai-code-battle.pages.dev/data/bots/b_swarmbot.json | jq`
- **Line 143:** `curl https://ai-code-battle.pages.dev/data/matches/index.json | jq`
- **Line 148:** `curl https://ai-code-battle.pages.dev/data/playlists/closest-finishes.json | jq`

**Context:** Documentation examples that users copy and run to fetch public data.

---

### 6. web/src/pages/docs-replay-format.ts (Lines 23, 26)
**Purpose:** Replay format documentation page

**Curl examples:**
- **Line 23:** `curl https://ai-code-battle.pages.dev/r2/replays/\${match_id}.json.gz`
- **Line 26:** `curl https://ai-code-battle.pages.dev/r2/replays/\${match_id}.json.gz` (duplicate)

**Context:** Documentation examples for downloading replay JSON.

---

### 7. web/src/pages/register.ts (Line 90)
**Purpose:** Bot registration page

**API endpoint:**
- **Line 90:** `curl -X POST https://api.ai-code-battle.pages.dev/api/register \\`

**Context:** Documentation example showing how to register a bot programmatically. Note: This uses the `api` subdomain (`api.ai-code-battle.pages.dev`), which is a separate Pages deployment.

---

### 8. web/src/pages/compete-hub.ts (Line 36)
**Purpose:** Compete hub page with external links

**GitHub link:**
- **Line 36:** `<a href="https://github.com/aicodebattle/acb" class="compete-card" target="_blank" rel="noopener">`

**Context:** External link to GitHub repository (not an ACB domain).

---

## Old Domain References (Comments Only)

### web/src/pages/docs-api.ts (Line 21)
```typescript
// Legacy B2 CDN reference — current implementation uses R2 via Pages Functions at /r2/
// b2.aicodebattle.com and aicodebattle.com domains are NXDOMAIN (not registered)
```

### web/src/pages/docs-data.ts (Line 19)
```typescript
// Legacy: b2.aicodebattle.com and aicodebattle.com references are outdated
```

**Status:** These are documentation comments only, not emitted to users. They correctly note that the old domains are no longer in use.

---

## Search Methodology

Searched `web/src/` for:
1. Direct domain references: `aicodebattle.com` and `b2.aicodebattle.com`
2. All `https://` URLs to identify user-facing URL patterns

---

## Conclusion

**All user-facing URLs correctly use `ai-code-battle.pages.dev`.** No action required. The old domains only appear in documentation comments explaining they are legacy references.

**Total user-facing URL locations:** 8 files, 23 URL references
**Old domain references:** 0 (only in comments)
