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

# Check Pages (SPA)
check_url "https://aicodebattle.com" "Pages (SPA)" || true

# Check the R2 Pages Function (replays). It answers 404 on an empty bucket,
# which still proves the function and binding are live — unlike the old
# r2.aicodebattle.com custom domain, which never resolved.
check_url "https://ai-code-battle.pages.dev/r2/replays/index.json" "R2 function" || true
curl -s -o /dev/null -w "  (function status: %{http_code})\n" "https://ai-code-battle.pages.dev/r2/replays/index.json" || true

# Check API (K8s Traefik)
check_url "https://api.aicodebattle.com/health" "API health" || true

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
