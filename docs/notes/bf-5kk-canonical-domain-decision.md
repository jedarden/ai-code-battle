---
name: bf-5kk-canonical-domain-decision
description: SUPERSEDED by canonical-public-domain.md — original 2026-07-02 decision to use ai-code-battle.pages.dev as canonical public domain
metadata:
  type: project
---

# Canonical Public Domain Decision

**Date:** 2026-07-02
**Decision:** Use `ai-code-battle.pages.dev` as the canonical public domain
**Status:** SUPERSEDED

> **Superseded by [canonical-public-domain.md](canonical-public-domain.md) (2026-08-21, bead aicodeba-ee0426a0).**
> That note is now the single source of truth for the canonical-domain decision
> (same decision, same outcome: `ai-code-battle.pages.dev`). The future
> custom-domain migration checklist from this note was folded into its
> "Future Work" section. Do not update this file; update that one.
>
> The implementation record this note once carried has been retired rather
> than maintained: its `web/src/pages/docs.ts` follow-through was rewritten by
> the 2026-09-25 public-API descope ([public-api-descope.md](public-api-descope.md)
> — the deferred-API examples now live in the separate `docs-api.ts` page,
> which uses the pages.dev origin), and its B2-CDN framing is legacy — replays
> are served through the R2 Pages Function at `web/functions/r2/[[path]].ts`
> with the dead `b2.aicodebattle.com` host swept from the codebase. The
> original record remains in git history if needed.

## References

- Original task: bf-5kk — Resolve canonical public domain
- Superseding note: [canonical-public-domain.md](canonical-public-domain.md)
- Cloudflare Pages custom domains: https://developers.cloudflare.com/pages/platform/custom-domains/
