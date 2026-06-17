#!/usr/bin/env bash
# B2 CDN Setup Information for AI Code Battle
# Prints B2 bucket endpoint, CNAME target, and verifies credentials
#
# Prerequisites:
#   - B2_ENDPOINT environment variable set (or reads from cluster secret)
#   - B2_KEY_ID environment variable set
#   - B2_APPLICATION_KEY environment variable set
#   - kubectl configured with access to apexalgo-iad cluster (for secret fallback)
#
# Usage:
#   ./scripts/setup-b2.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BUCKET_NAME="acb-data"
REGION="us-west-002"
B2_ENDPOINT="${B2_ENDPOINT:-https://s3.us-west-002.backblazeb2.com}"
CUSTOM_DOMAIN="b2.aicodebattle.com"
CNAME_TARGET="${BUCKET_NAME}.s3.${REGION}.backblazeb2.com"

echo -e "${BLUE}=== AI Code Battle - B2 CDN Setup Information ===${NC}"
echo ""

# Try to get credentials from environment variables first
if [ -n "$B2_KEY_ID" ] && [ -n "$B2_APPLICATION_KEY" ]; then
    echo -e "${GREEN}✓ Using B2 credentials from environment variables${NC}"
    USE_ENV_CREDENTIALS=true
else
    echo -e "${YELLOW}⚠ B2_KEY_ID and B2_APPLICATION_KEY not set in environment${NC}"
    echo "Attempting to read from cluster secret..."
    USE_ENV_CREDENTIALS=false

    # Try to read from Kubernetes secret (requires cluster access)
    if kubectl --server=http://traefik-apexalgo-iad:8001 get secret backblaze-secret -n ai-code-battle &>/dev/null; then
        echo -e "${YELLOW}⚠ Cannot read secret values via read-only kubectl proxy${NC}"
        echo "  To test B2 API authentication, set environment variables:"
        echo "  export B2_KEY_ID=<your-key-id>"
        echo "  export B2_APPLICATION_KEY=<your-application-key>"
        echo ""
    fi
fi

# Step 1: Print B2 bucket endpoint
echo -e "${BLUE}Step 1: B2 Bucket Endpoint${NC}"
echo ""
echo "  Bucket Name: ${BUCKET_NAME}"
echo "  Region: ${REGION}"
echo "  S3 Endpoint: ${B2_ENDPOINT}"
echo "  Friendly Endpoint: f002.backblazeb2.com"
echo ""

# Step 2: Print CNAME configuration
echo -e "${BLUE}Step 2: Required CNAME Configuration${NC}"
echo ""
echo "  Type: CNAME"
echo "  Name: ${CUSTOM_DOMAIN}"
echo "  Target: ${CNAME_TARGET}"
echo "  Proxy: On (orange cloud) ← REQUIRED for Bandwidth Alliance"
echo "  TTL: Auto (3600)"
echo ""

# Step 3: Verify B2 credentials (if available)
echo -e "${BLUE}Step 3: B2 API Authentication Verification${NC}"
echo ""

if [ "$USE_ENV_CREDENTIALS" = true ]; then
    echo "Testing B2 API authentication..."
    AUTH_RESPONSE=$(curl -s -u "${B2_KEY_ID}:${B2_APPLICATION_KEY}" "${B2_ENDPOINT}/b2api/v2/b2_authorize_account" 2>&1)

    if echo "$AUTH_RESPONSE" | jq -e '.accountId' > /dev/null 2>&1; then
        echo -e "${GREEN}✓ B2 API authentication successful${NC}"
        echo ""
        echo "Account Details:"
        echo "  Account ID: $(echo "$AUTH_RESPONSE" | jq -r '.accountId')"
        echo "  API URL: $(echo "$AUTH_RESPONSE" | jq -r '.apiUrl')"
        echo "  Download URL: $(echo "$AUTH_RESPONSE" | jq -r '.downloadUrl')"
        echo ""
        echo "Allowed Capabilities:"
        echo "$AUTH_RESPONSE" | jq -r '.allowed.capabilities[]' | sed 's/^/  - /'
    else
        echo -e "${RED}✗ B2 API authentication failed${NC}"
        echo "Response:"
        echo "$AUTH_RESPONSE"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠ Skipping authentication test (credentials not available)${NC}"
    echo ""
    echo "To test B2 API authentication, set the following environment variables:"
    echo "  export B2_ENDPOINT=${B2_ENDPOINT}"
    echo "  export B2_KEY_ID=<your-key-id>"
    echo "  export B2_APPLICATION_KEY=<your-application-key>"
    echo ""
    echo "Then run this script again."
fi

echo ""

# Step 4: Print expected URLs
echo -e "${BLUE}Step 4: Expected URLs After Configuration${NC}"
echo ""
echo "Once the CNAME is configured and public access is enabled:"
echo ""
echo "  Replay files:"
echo "    https://${CUSTOM_DOMAIN}/replays/{match_id}.json.gz"
echo ""
echo "  Match metadata:"
echo "    https://${CUSTOM_DOMAIN}/matches/{match_id}.json"
echo ""
echo "  Evolution feed:"
echo "    https://${CUSTOM_DOMAIN}/evolution/live.json"
echo ""
echo "  Bot cards:"
echo "    https://${CUSTOM_DOMAIN}/bots/{bot_id}.json"
echo ""

# Step 5: Print manual setup steps
echo -e "${BLUE}Step 5: Manual Setup Steps${NC}"
echo ""
echo "This script is informational only. To complete B2 CDN setup:"
echo ""
echo "1. Enable Public Access on B2 Bucket:"
echo "   - Go to: https://secure.backblaze.com/sign_in.htm"
echo "   - Navigate to: B2 Cloud Storage > Buckets > ${BUCKET_NAME}"
echo "   - Settings > Bucket Info > Files in Bucket are: Public"
echo ""
echo "2. Create CNAME Record in Cloudflare:"
echo "   - Type: CNAME"
echo "   - Name: b2"
echo "   - Target: ${CNAME_TARGET}"
echo "   - Proxy: On (orange cloud) ← REQUIRED for Bandwidth Alliance"
echo ""
echo "3. Verify CNAME Resolution:"
echo "   dig +short ${CUSTOM_DOMAIN}"
echo "   # Expected: ${CNAME_TARGET}"
echo ""
echo "4. Test CDN Access:"
echo "   curl -I https://${CUSTOM_DOMAIN}/"
echo "   # Should return 404 from B2 (bucket public but file not found)"
echo ""
echo "5. Verify Bandwidth Alliance:"
echo "   - Cloudflare Dashboard → Traffic → Bandwidth Alliance"
echo "   - Should show Backblaze as active partner"
echo ""

echo -e "${BLUE}=== B2 CDN Setup Information Complete ===${NC}"
echo ""
echo -e "${GREEN}Next Steps:${NC}"
echo "  1. Enable public access on B2 bucket"
echo "  2. Create CNAME record in Cloudflare DNS"
echo "  3. Test CDN access with curl"
echo ""
