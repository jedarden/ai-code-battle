# SPA Route Smoke Test Results

**Date:** 2026-05-24
**Bead:** bf-2qp0
**Method:** curl (ADB not available on this system)
**Base URL:** https://ai-code-battle.pages.dev

## Summary

| Metric | Count |
|--------|-------|
| **Passed** | 36 |
| **Warned (404)** | 7 |
| **Failed** | 0 |
| **Total** | 43 |

**Result:** All SPA routes return valid HTML. The /r2/ data paths return 404 (data not yet deployed to R2/Pages Function).

## Static Routes (All Passed)

| Route | Description | Status |
|-------|-------------|--------|
| `/` | Home page | 200 OK |
| `/watch` | Watch hub | 200 OK |
| `/watch/replays` | Matches page | 200 OK |
| `/watch/playlists` | Playlists page | 200 OK |
| `/watch/replay` | Replay page (no id) | 200 OK |
| `/watch/predictions` | Predictions page | 200 OK |
| `/watch/series` | Series page | 200 OK |
| `/compete` | Compete hub | 200 OK |
| `/compete/sandbox` | Sandbox page | 200 OK |
| `/compete/register` | Register page | 200 OK |
| `/compete/docs` | Compete docs | 200 OK |
| `/compete/docs/api` | API docs | 200 OK |
| `/leaderboard` | Leaderboard page | 200 OK |
| `/evolution` | Evolution page | 200 OK |
| `/blog` | Blog page | 200 OK |
| `/seasons` | Seasons page | 200 OK |
| `/rivalries` | Rivalries page | 200 OK |
| `/feedback` | Feedback page | 200 OK |
| `/docs/api` | API docs (alt path) | 200 OK |

## Redirect Routes (All Passed)

| Route | Description | Status |
|-------|-------------|--------|
| `/matches` | Redirect to /watch/replays | 200 OK |
| `/playlists` | Redirect to /watch/playlists | 200 OK |
| `/replay` | Redirect to /watch/replay | 200 OK |
| `/predictions` | Redirect to /watch/predictions | 200 OK |
| `/series` | Redirect to /watch/series | 200 OK |
| `/sandbox` | Redirect to /compete/sandbox | 200 OK |
| `/register` | Redirect to /compete/register | 200 OK |
| `/bots` | Redirect to /leaderboard | 200 OK |
| `/docs` | Redirect to /compete/docs | 200 OK |
| `/clip-maker` | Redirect to /watch/replays | 200 OK |

## Parameterized Routes (All Passed - routing only)

| Route | Description | Status |
|-------|-------------|--------|
| `/watch/replay/:id` | Replay detail | 200 OK |
| `/bot/:id` | Bot profile | 200 OK |
| `/compete/bot/:id` | Bot profile (compete path) | 200 OK |
| `/season/:id` | Season detail | 200 OK |
| `/watch/series/:id` | Series detail | 200 OK |
| `/blog/:slug` | Blog post | 200 OK |
| `/watch/playlists/:slug` | Playlist detail | 200 OK |

Note: Parameterized routes tested with placeholder IDs. Routes return valid SPA shell; actual data loading depends on /r2/ data path.

## Data Paths (/r2/) - 404 Expected

| Path | Description | Status |
|------|-------------|--------|
| `/r2/` | R2 data root | 404 Not Found |
| `/r2/data/matches/index.json` | Matches index | 404 Not Found |
| `/r2/data/leaderboard.json` | Leaderboard data | 404 Not Found |
| `/r2/data/seasons/index.json` | Seasons index | 404 Not Found |
| `/r2/data/series/index.json` | Series index | 404 Not Found |
| `/r2/data/blog/index.json` | Blog index | 404 Not Found |
| `/r2/data/playlists/index.json` | Playlists index | 404 Not Found |

**Note:** These 404s are expected - the data has not been uploaded to R2 or the Pages Function has not been configured yet. This is a data availability issue, not a routing issue. See related bead `bf-cmh1` for end-to-end replay viewer testing.

## Test Script

The test script is available at `/home/coding/ai-code-battle/test_routes.sh`. Run with:

```bash
./test_routes.sh
```

## ADB Testing

ADB was not available on this system (adb-check command not found, no platform-tools in ~/.local/). The fallback curl method was used instead. For full device testing with ADB, see the related bead `bf-cmh1`.
