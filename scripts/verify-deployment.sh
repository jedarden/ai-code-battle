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

# Check R2 (replays) - just check the domain responds
check_url "https://r2.aicodebattle.com" "R2 (custom domain)" || true

# Check API (K8s Traefik)
check_url "https://api.aicodebattle.com/health" "API health" || true

echo ""
echo "=== DNS Verification ==="
echo ""

# DNS checks
echo "aicodebattle.com:"
dig +short aicodebattle.com || echo "  (not configured)"

echo ""
echo "r2.aicodebattle.com:"
dig +short r2.aicodebattle.com || echo "  (not configured)"

echo ""
echo "api.aicodebattle.com:"
dig +short api.aicodebattle.com || echo "  (not configured)"

echo ""
echo "=== Done ==="
