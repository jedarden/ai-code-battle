# acb-index-builder compile fix (bf-16k)

## Summary

`cmd/acb-index-builder` failed to compile at the bead's dispatch point: `deploy.go`
was a half-finished rewrite that redeclared `S3Client`, `getR2Client`/`getB2Client`,
`uploadFileToR2`/`uploadCardsToB2`, and the `objectExists`/`downloadObject`/`uploadObject`
methods already implemented properly in `s3.go` (real aws-sdk-go-v2 `S3Client`) and
`cards.go`. The `deploy.go` copies were stubs (no-op upload, "not implemented" download,
always-false `objectExists`). Several references were also unresolved (`cfg.PagesProject`,
the `R2Object` type, `B2Client.uploadFile`).

## Resolution

The fix was applied in commit `724f1d9` (originally `e62a501` before rebase onto
`origin/master`):

- **Canonical implementation kept:** `s3.go`'s real aws-sdk-go-v2 `S3Client`
  (`listObjects`/`objectExists`/`downloadObject`/`uploadFile`) and `cards.go`'s
  `uploadFileToR2`/`uploadCardsToB2`. The duplicate/stub declarations were dropped from
  `deploy.go`, leaving only its unique functions.
- **`B2Client` is now an interface** (`deploy.go:18`) satisfied by `*S3Client`
  (`s3.go:26`) and `MockS3Client` (`s3_test.go`). `getB2Client` returns the interface, so
  `cards.go:471`'s `client.uploadFile(...)` resolves to `S3Client.uploadFile` (s3.go:134).
- **Missing `R2Object` type** added to `s3.go` (used by `listObjects` and `MockS3Client`).
- **`deployToPages`** fixed to use the real `Config` field names
  (`PagesProjectName`/`CloudflareAccountID`/`CloudflareAPIToken`) instead of the
  nonexistent `PagesProject`/`CFAccountID`/`CFAPIToken`.
- `generator.go` and `sitebuild.go` follow-on fixes (botNameMap arg, `copyWebAssets`).

## Acceptance Criteria Met

Run from repo root after the fix:

```
go build ./cmd/acb-index-builder/...   # OK (binary builds, ~20MB)
go vet  ./cmd/acb-index-builder/...    # OK
go test ./cmd/acb-index-builder/...    # ok  github.com/aicodebattle/acb/cmd/acb-index-builder (s3_test.go etc.)
```

No duplicate declarations remain — each of `S3Client`, `uploadFileToR2`,
`uploadCardsToB2`, `getR2Client`, `getB2Client` has exactly one declaration.

## This session

The fix commit predated this session but had not been pushed (the lab branch had diverged
5 ahead / 1 behind `origin/master`; the remote-only commit `86b0715 docs(adr-001)` only
touches `docs/plan/plan.md`, zero file overlap). This session:

1. Re-verified all acceptance criteria (build/vet/test, fresh `-count=1` run).
2. Rebased the 5 local commits onto `origin/master` (clean, linear history; no force-push).
3. Pushed to `origin/master`.
4. Closed the bead.

## Note

Companion bead `bf-g1xv` (filed from ADR-001 in commit `86b0715`) tracks the same P0
compile break; this work resolves it. Recovering the CrashLooping `acb-index-builder` pod
additionally requires the missing `acb-cloudflare-api-token` secret (separate bead).
