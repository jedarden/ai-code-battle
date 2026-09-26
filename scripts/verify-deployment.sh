#!/bin/bash
# Verify Cloudflare deployment end-to-end

set -e

echo "=== AI Code Battle - Deployment Verification ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

check_url() {
    local url=$1
    local name=$2
    if curl -sf -o /dev/null "$url"; then
        echo -e "${GREEN}✓${NC} $name: $url"
        return 0
    else
        echo -e "${RED}✗${NC} $name: $url"
        return 1
    fi
}

echo "Checking endpoints..."
echo ""

# Check Pages (SPA) on the canonical origin. The aicodebattle.com apex is
# NXDOMAIN — the one hostname the Pages project can ever have is
# ai-code-battle.pages.dev (docs/notes/canonical-public-domain.md).
check_url "https://ai-code-battle.pages.dev" "Pages (SPA, canonical origin)" || true

# Check the R2 Pages Function (replays). It answers 404 on an empty bucket,
# which still proves the function and binding are live — unlike the old
# r2.aicodebattle.com custom domain, which never resolved.
check_url "https://ai-code-battle.pages.dev/r2/replays/index.json" "R2 function" || true
curl -s -o /dev/null -w "  (function status: %{http_code})\n" "https://ai-code-battle.pages.dev/r2/replays/index.json" || true

# ─── /api Pages Function: the release gate ──────────────────────────────────
# The SPA's /api transport ships with every site deploy as the Pages Function
# at web/functions/api/[[path]].ts. The deployed origin must answer
# function-owned JSON for health, the community reads, and every documented
# match-tier 503 — any text/html answer is the SPA fallback, i.e. the
# transport regressed (docs/notes/api-transport.md).
# web/test-api-workflows.js is the arbiter of that contract; this is the only
# hard-failing check in this script, so a deploy is not "verified" until the
# function answers. (The old api.aicodebattle.com/health K8s check answered
# nothing since the compute-tier descope — it checked a host that no longer
# resolves, which is why /api could silently rot to the SPA fallback.)
echo ""
echo "Checking /api transport gate (hard failure on regression)..."
if ( cd "$(dirname "$0")/../web" && ACB_SKIP_BUILD_CHECK=1 node test-api-workflows.js ); then
    echo -e "${GREEN}✓${NC} /api transport gate passed — function-owned JSON on every documented route"
else
    echo -e "${RED}✗${NC} /api transport gate FAILED — /api is answering the SPA fallback or wrong envelopes"
    exit 1
fi

echo ""
echo "=== DNS Verification ==="
echo ""

# DNS checks
echo "aicodebattle.com:"
dig +short aicodebattle.com || echo "  (not configured)"

echo ""
echo "R2 function path (no custom DNS - served on the Pages origin):"
echo "  https://ai-code-battle.pages.dev/r2/<key>"

echo ""
echo "api.aicodebattle.com:"
dig +short api.aicodebattle.com || echo "  (not configured)"

echo ""
echo "=== Done ==="
