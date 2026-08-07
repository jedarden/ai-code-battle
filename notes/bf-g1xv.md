# Bead bf-g1xv: Fix cmd/acb-index-builder compile break

**Status:** Already resolved in commit 724f1d9 (2026-07-26)

## Issue Summary

The `go build ./cmd/acb-index-builder/...` was failing with duplicate S3 client declarations across `deploy.go`, `s3.go`, and `cards.go`.

## Root Cause

`deploy.go` contained half-finished rewrites that redeclared types and functions already properly implemented in `s3.go` and `cards.go`:
- Duplicate `S3Client` type
- Duplicate `getR2Client`/`getB2Client` functions  
- Duplicate `uploadFileToR2`/`uploadCardsToB2` functions
- Duplicate `objectExists`/`downloadObject`/`uploadObject` methods
- Missing `R2Object` type
- Incorrect Config field references (`PagesProject` vs `PagesProjectName`)

## Resolution (Already Applied)

Commit 724f1d9 applied the following fixes:

### deploy.go
- Removed 135 lines of duplicate/stub declarations
- Fixed `deployToPages` to use correct Config field names:
  - `PagesProjectName` (not `PagesProject`)
  - `CloudflareAccountID` (not `CFAccountID`)  
  - `CloudflareAPIToken` (not `CFAPIToken`)
- Removed unused `time` and `metrics` imports
- Fixed replay bundling to use `.json.gz` format
- Made missing `leaderboard.json` a warning (not error) for first builds

### s3.go
- Added missing `R2Object` type definition
- Added required `time` import

### generator.go
- Fixed `buildPlaylistMatch` call sites to pass `botNameMap`

### sitebuild.go
- Added missing `copyWebAssets` function and `copyFile` helper

### s3_test.go
- Fixed test expectations for card paths (cards/ not data/cards/)

## Verification

All checks pass:
```bash
✓ go build ./cmd/acb-index-builder/...
✓ go vet ./cmd/acb-index-builder/...
✓ go test ./cmd/acb-index-builder/...
```

Binary successfully builds (20M executable with debug info).

## Impact

This binary is critical for the site's data pipeline:
- Rebuilds `data/leaderboard.json`
- Rebuilds `data/matches/index.json`
- Rebuilds `data/bots/*.json` files
- Deploys to Cloudflare Pages

The fix ensures the index builder can compile and run, enabling ADR-001 (live-tail freshness) from docs/plan/plan.md.
