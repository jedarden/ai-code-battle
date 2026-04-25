# Replay Viewer Test Results

**Date:** 2026-04-25
**Task:** Verify replay viewer loads and plays a real match replay

## Summary

The replay viewer code is functional and works correctly with local replay files. However, the storage backend infrastructure (R2/B2) for serving real match replays is not working.

## What Works ✅

1. **Replay Viewer Implementation**
   - Canvas renders correctly with grid, bots, and energy cells
   - Playback controls work (play/pause, step, reset)
   - Turn navigation functions properly
   - Transcript panel generates turn-by-turn events
   - Mobile responsive layout is functional

2. **Local Test Files**
   - `/data/demo-replay-v2.json` - 4-player match (294 turns)
   - `/data/demo-replay-v1.json` - Basic 2-player match
   - `/data/real-replay.json` - Real match data (m_tprjf4ij, 713 turns, 4 players)
   - `/data/demo-replay-v2-6p.json` - 6-player match

3. **Mobile Testing (Pixel 6 via ADB)**
   - Page loads correctly in Chrome
   - Layout is responsive and touch targets are usable
   - No horizontal overflow issues
   - Test page: `/test-replay-viewer-real.html` created for real replay testing

## What Doesn't Work ❌

1. **Storage Backend Access**
   - R2 endpoint: `https://r2.aicodebattle.com/replays/{match_id}.json.gz` - Returns 404
   - B2 endpoint: `https://b2.aicodebattle.com/replays/{match_id}.json.gz` - Returns 404
   - Production API: `https://ai-code-battle.pages.dev/api/replay/{match_id}` - Returns HTML page (not JSON)

2. **Missing Replay Data**
   - No real match replays are uploaded to R2 or B2 storage
   - This is a known blocker mentioned in the task description

## Known Blockers (from task description)

1. **B2 'Invalid region' error** - Replay upload to B2 is broken
   - Fix needed in acb-worker config

2. **R2 ESO hashed endpoint** - Replay upload to R2 is broken
   - Fix needed: OpenBao → ESO → acb-r2-credentials secret

## Test Results

### Real Replay (m_tprjf4ij)
- Match ID: m_tprjf4ij
- Players: 4 (swarm, hunter, gatherer, random)
- Turns: 713
- Map: 89x89
- Winner: Player 0 (swarm)
- Tests Passed: 15/15
- Warnings: 2 (no win_prob data, no critical_moments data)

### Mobile Browser Testing
- Device: Google Pixel 6 (1080x2400)
- Browser: Chrome via ADB over Tailscale
- Connection: http://100.72.170.64:8080
- Test Page: `/test-replay-viewer-real.html`
- Results: All tests passed, layout responsive

## Recommendations

1. **Fix the replay upload pipeline** - This is the critical blocker
   - Fix B2 'Invalid region' error in acb-worker config
   - Fix R2 ESO credentials (OpenBao → ESO → acb-r2-credentials secret)

2. **Test with production data** - Once storage is fixed:
   - Upload a test replay to R2/B2
   - Verify ?url=/replays/{match_id}.json.gz parameter works
   - Verify win probability sparkline renders with real commentary data

3. **Keep test pages** - The created test pages are useful for future testing:
   - `/test-replay-viewer.html` - Basic structure test
   - `/test-replay-viewer-demo.html` - Demo replay with full test suite
   - `/test-replay-viewer-real.html` - Real replay test (NEW)

## Files Modified/Created

- **Created:** `/web/public/test-replay-viewer-real.html` - Test page for real replay data
