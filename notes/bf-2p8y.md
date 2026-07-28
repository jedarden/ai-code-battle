# Bead bf-2p8y — Configure and verify custom domain aicodebattle.com

## Outcome: BLOCKED — Prerequisite Not Met

This bead cannot be completed because its core prerequisite is **not satisfied**: `aicodebattle.com` is **not a registered domain**.

---

## Investigation Findings

### Domain Status
1. **DNS Resolution**: `aicodebattle.com` returns **NXDOMAIN** (domain does not exist in DNS)
   ```bash
   host aicodebattle.com
   # Output: Host aicodebattle.com not found: 3(NXDOMAIN)
   ```

2. **Cloudflare Pages Attachment**: The domain appears in the Pages project as `deactivated`:
   ```json
   {
     "name": "aicodebattle.com",
     "status": "deactivated",
     "created_on": "2026-06-27T21:22:10.780221Z"
   }
   ```

3. **Cloudflare DNS Zone**: The zone does **not exist** in the Cloudflare account:
   - Zone lookup for `aicodebattle.com` returns "NOT_FOUND"
   - No DNS records can be created without a zone

### Project Decision Context

According to **[[bf-5kk-canonical-domain-decision]]** (dated 2026-07-02):

> **Decision:** Use `ai-code-battle.pages.dev` as the canonical public domain  
> **Status:** ACTIVE

The project explicitly decided **NOT** to register `aicodebattle.com`:

- **Option A (Register aicodebattle.com)**: Status: **BLOCKED - Requires human action + payment**
- **Option B (Use ai-code-battle.pages.dev)**: Status: **IMPLEMENTED ✅**

The rationale was:
- Zero cost (no registration fees)
- Zero configuration (works out of the box)
- Already implemented (web code already uses pages.dev)
- Reversible (can add custom domain later)

### Acceptance Criteria Status

| Criterion | Status | Reason |
|-----------|--------|--------|
| Domain registered or nameservers point to Cloudflare | ❌ | Domain is NXDOMAIN (not registered) |
| wrangler pages domain add succeeds or already attached | ⚠️ | Attached but **deactivated** (zone doesn't exist) |
| curl -sI https://aicodebattle.com returns HTTP 200 | ❌ | Domain does not resolve (NXDOMAIN) |
| https://aicodebattle.com serves SPA HTML shell | ❌ | Domain does not resolve (NXDOMAIN) |
| aicodebattle.com resolves to Cloudflare Pages project | ❌ | Domain does not resolve (NXDOMAIN) |

---

## Why This Bead Cannot Be Completed

1. **Prerequisite Failure**: The bead explicitly requires "aicodebattle.com is a registered domain on the same Cloudflare account (or its nameservers point to Cloudflare)" — this condition is **false**.

2. **Project Decision Override**: There is an **active project decision** (bf-5kk) to use `ai-code-battle.pages.dev` instead of registering `aicodebattle.com`.

3. **Technical Block**: Even if we wanted to proceed, the DNS zone doesn't exist, so:
   - No DNS records can be created
   - The domain cannot resolve
   - HTTPS verification is impossible

4. **No Path Forward**: Completing this bead would require:
   - Registering the domain (requires payment + human action)
   - Creating the Cloudflare DNS zone
   - Configuring DNS records
   - Waiting for propagation
   - **All of which contradicts the existing project decision.**

---

## Current Working State

The application is **already successfully deployed and accessible** at:
- **https://ai-code-battle.pages.dev** ✅ (HTTP 200, serves SPA correctly)

All web code already uses this domain correctly:
- OG tags use `ai-code-battle.pages.dev`
- Share URLs use `ai-code-battle.pages.dev`
- Replay viewer uses `ai-code-battle.pages.dev`

## Fresh Verification (2026-07-28 19:40 UTC)

Re-verified domain status:
- `host aicodebattle.com` → Host not found: 3(NXDOMAIN) ✅ Confirmed
- `curl -sI https://aicodebattle.com` → Could not resolve host ✅ Confirmed
- `curl -sI https://ai-code-battle.pages.dev` → HTTP 200 ✅ Working correctly

The blockage remains unchanged from original investigation. The domain aicodebattle.com is not registered and does not exist in DNS, while the canonical domain ai-code-battle.pages.dev is serving the application correctly.

---

## Recommended Path Forward

This bead should be **left open but marked as blocked** with a note that:

1. The prerequisite (domain registration) is not met
2. There is an active project decision to use `ai-code-battle.pages.dev` instead
3. If/when the project decides to register the custom domain, this bead can be revisited

The bead should **NOT be closed** because the acceptance criteria cannot be met in the current state.

---

## Current Session (2026-07-28 19:40 UTC)

Re-verified all acceptance criteria and domain status. Findings confirm the original investigation:

1. **Domain Registration**: Still NOT registered - confirmed via host command returning NXDOMAIN
2. **HTTPS Resolution**: Cannot resolve - curl to aicodebattle.com fails
3. **Working Domain**: ai-code-battle.pages.dev continues serving correctly (HTTP 200)
4. **Project Decision**: No change - bf-5kk decision to use pages.dev domain remains active

### Conclusion
This bead **cannot be completed** because:
- Core prerequisite (domain registration) is not met
- Active project decision (bf-5kk) explicitly chooses NOT to register aicodebattle.com
- All acceptance criteria fail (domain doesn't exist, can't resolve, can't serve)

The bead should remain open as blocked, not closed.

---

## Cross-references

- **[[bf-5kk-canonical-domain-decision]]**: Active decision to use `ai-code-battle.pages.dev` instead of registering `aicodebattle.com`
- **[[bf-4w5]]**: Successfully deployed SPA to `ai-code-battle.pages.dev` (parent bead)
- **[[DEPLOYMENT_STEPS.md]]**: Notes that "aicodebattle.com is not registered"
- **[[web/CLOUDFLARE_DEPLOYMENT.md]]**: Notes that "aicodebattle.com is not currently registered"

---

## Notes

- The domain appears in Cloudflare Pages as "deactivated" because it was likely attached at some point but never fully configured (zone doesn't exist)
- The NXDOMAIN status confirms the domain has never been registered or its registration has lapsed
- The project made a conscious decision to avoid domain registration costs and use the free Pages domain instead
