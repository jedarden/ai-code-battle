#!/bin/bash
# DNS Configuration Script for AI Code Battle
# Configures all required DNS records via Cloudflare API
#
# Prerequisites:
#   export CLOUDFLARE_API_TOKEN=your_token_here
#   export CLOUDFLARE_ZONE_ID=your_zone_id  # Optional, will auto-detect
#   export TRAEFIK_IP=your_traefik_ip       # Required for api.aicodebattle.com
#
# Usage:
#   ./scripts/configure-dns.sh
#
# ALTERNATIVE: Manual Dashboard Configuration
#   See DEPLOYMENT.md for step-by-step dashboard instructions
#
# DNS Records Required:
#   1. aicodebattle.com      → CNAME → aicodebattle.pages.dev (proxied)
#   2. r2.aicodebattle.com   → CNAME → acb-data.r2.cloudflarestorage.com
#   3. api.aicodebattle.com  → A     → <Traefik IP> (proxied)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

DOMAIN="aicodebattle.com"

echo "=== AI Code Battle - DNS Configuration ==="
echo ""

# Check for required environment variables
if [ -z "$CLOUDFLARE_API_TOKEN" ]; then
    echo -e "${RED}ERROR: CLOUDFLARE_API_TOKEN not set${NC}"
    echo "Get your API token from: https://dash.cloudflare.com/profile/api-tokens"
    echo "Required permissions: Zone.DNS (Edit)"
    echo ""
    echo "Run: export CLOUDFLARE_API_TOKEN=your_token_here"
    exit 1
fi

if [ -z "$TRAEFIK_IP" ]; then
    echo -e "${YELLOW}WARNING: TRAEFIK_IP not set${NC}"
    echo "The api.aicodebattle.com record will NOT be configured."
    echo "Get the Traefik LoadBalancer IP from Kubernetes:"
    echo "  kubectl --server=http://kubectl-apexalgo-iad:8001 get svc -n traefik"
    echo ""
    echo "Run: export TRAEFIK_IP=your_traefik_ip"
    echo ""
    READ_SKIP_API=${SKIP_API:-"n"}
    if [ "$READ_SKIP_API" != "y" ]; then
        read -p "Continue without API record? (y/n) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
fi

# Cloudflare API base URL
CF_API="https://api.cloudflare.com/client/v4"

# Function to make authenticated API calls
cf_api() {
    local method=$1
    local endpoint=$2
    local data=$3

    if [ -n "$data" ]; then
        curl -s -X "$method" "${CF_API}${endpoint}" \
            -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
            -H "Content-Type: application/json" \
            -d "$data"
    else
        curl -s -X "$method" "${CF_API}${endpoint}" \
            -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
            -H "Content-Type: application/json"
    fi
}

# Get Zone ID if not provided
if [ -z "$CLOUDFLARE_ZONE_ID" ]; then
    echo "Looking up Zone ID for $DOMAIN..."
    ZONE_RESPONSE=$(cf_api GET "/zones?name=$DOMAIN")
    CLOUDFLARE_ZONE_ID=$(echo "$ZONE_RESPONSE" | jq -r '.result[0].id // empty')

    if [ -z "$CLOUDFLARE_ZONE_ID" ]; then
        echo -e "${RED}ERROR: Could not find zone for $DOMAIN${NC}"
        echo "Make sure the domain is added to your Cloudflare account."
        exit 1
    fi
    echo -e "${GREEN}Found Zone ID: $CLOUDFLARE_ZONE_ID${NC}"
fi

ZONE_ID=$CLOUDFLARE_ZONE_ID

# Function to create or update DNS record
configure_record() {
    local name=$1
    local type=$2
    local content=$3
    local proxied=${4:-false}

    echo ""
    echo "Configuring $name.$DOMAIN ($type -> $content)..."

    # Check if record exists
    EXISTING=$(cf_api GET "/zones/$ZONE_ID/dns_records?name=${name}.${DOMAIN}&type=$type")
    EXISTING_ID=$(echo "$EXISTING" | jq -r '.result[0].id // empty')

    if [ -n "$EXISTING_ID" ]; then
        # Update existing record
        echo "  Updating existing record..."
        RESPONSE=$(cf_api PUT "/zones/$ZONE_ID/dns_records/$EXISTING_ID" \
            "{\"type\":\"$type\",\"name\":\"$name\",\"content\":\"$content\",\"ttl\":3600,\"proxied\":$proxied}")
    else
        # Create new record
        echo "  Creating new record..."
        RESPONSE=$(cf_api POST "/zones/$ZONE_ID/dns_records" \
            "{\"type\":\"$type\",\"name\":\"$name\",\"content\":\"$content\",\"ttl\":3600,\"proxied\":$proxied}")
    fi

    SUCCESS=$(echo "$RESPONSE" | jq -r '.success // false')
    if [ "$SUCCESS" = "true" ]; then
        echo -e "${GREEN}  ✓ Success${NC}"
    else
        ERRORS=$(echo "$RESPONSE" | jq -r '.errors[0].message // "Unknown error"')
        echo -e "${RED}  ✗ Failed: $ERRORS${NC}"
    fi
}

# 1. Configure main domain (Pages CNAME)
# Note: For Pages custom domains, the CNAME target is <project>.pages.dev
echo ""
echo "=== 1. Main domain (Pages) ==="
echo "Note: For Cloudflare Pages, you should add the custom domain via:"
echo "  Workers & Pages > aicodebattle > Settings > Custom domains"
echo ""
echo "If the Pages project is already configured, we'll set up the CNAME:"
configure_record "@" "CNAME" "aicodebattle.pages.dev" true

# 2. Configure R2 subdomain
echo ""
echo "=== 2. R2 subdomain ==="
echo "Note: For R2 custom domains, you should add the custom domain via:"
echo "  R2 > acb-data > Settings > Custom Domains"
echo ""
echo "If already configured, the CNAME should point to R2's public endpoint:"
# R2 custom domain CNAME format: <bucket>.r2.cloudflarestorage.com
configure_record "r2" "CNAME" "acb-data.r2.cloudflarestorage.com" false

# 3. Configure API subdomain (if Traefik IP provided)
echo ""
echo "=== 3. API subdomain ==="
if [ -n "$TRAEFIK_IP" ]; then
    configure_record "api" "A" "$TRAEFIK_IP" true
else
    echo -e "${YELLOW}Skipping - TRAEFIK_IP not set${NC}"
    echo "To configure later, run:"
    echo "  TRAEFIK_IP=<ip> ./scripts/configure-dns.sh"
fi

echo ""
echo "=== DNS Configuration Complete ==="
echo ""
echo "Verification commands:"
echo "  dig +short aicodebattle.com"
echo "  dig +short r2.aicodebattle.com"
echo "  dig +short api.aicodebattle.com"
echo ""
echo "Or run: ./scripts/verify-deployment.sh"
