#!/bin/bash
# R2 Bucket Setup Script for AI Code Battle
# Creates R2 bucket and configures custom domain
#
# Prerequisites:
#   - wrangler CLI installed and authenticated
#   - CLOUDFLARE_API_TOKEN set (for custom domain configuration)
#   - CLOUDFLARE_ACCOUNT_ID set (optional, will auto-detect)
#
# Usage:
#   ./scripts/setup-r2.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BUCKET_NAME="acb-data"
CUSTOM_DOMAIN="r2.aicodebattle.com"
DOMAIN="aicodebattle.com"

echo -e "${BLUE}=== AI Code Battle - R2 Bucket Setup ===${NC}"
echo ""

# Step 1: Check wrangler authentication
echo -e "${BLUE}Step 1: Checking wrangler authentication...${NC}"
if ! ~/.local/bin/wrangler whoami &>/dev/null; then
    echo -e "${RED}ERROR: Not authenticated with Cloudflare.${NC}"
    echo "Run: ~/.local/bin/wrangler login"
    exit 1
fi

echo -e "${GREEN}Authenticated as:${NC}"
~/.local/bin/wrangler whoami
echo ""

# Get account ID
ACCOUNT_ID=$(~/.local/bin/wrangler whoami 2>/dev/null | grep "Account ID" | awk '{print $3}' | tr -d '()"')
if [ -z "$ACCOUNT_ID" ]; then
    # Try alternative parsing
    ACCOUNT_ID=$(~/.local/bin/wrangler whoami 2>/dev/null | grep -oP 'Account ID:\s*\K[0-9a-f-]+' || true)
fi

if [ -n "$ACCOUNT_ID" ]; then
    echo -e "${GREEN}Account ID: $ACCOUNT_ID${NC}"
else
    echo -e "${YELLOW}Could not auto-detect Account ID${NC}"
    if [ -z "$CLOUDFLARE_ACCOUNT_ID" ]; then
        echo "Set CLOUDFLARE_ACCOUNT_ID environment variable to proceed."
    else
        ACCOUNT_ID="$CLOUDFLARE_ACCOUNT_ID"
        echo -e "${GREEN}Using CLOUDFLARE_ACCOUNT_ID: $ACCOUNT_ID${NC}"
    fi
fi
echo ""

# Step 2: Create R2 bucket
echo -e "${BLUE}Step 2: Creating R2 bucket '$BUCKET_NAME'...${NC}"
if ~/.local/bin/wrangler r2 bucket list 2>/dev/null | grep -q "$BUCKET_NAME"; then
    echo -e "${YELLOW}R2 bucket '$BUCKET_NAME' already exists.${NC}"
else
    echo "Creating R2 bucket..."
    ~/.local/bin/wrangler r2 bucket create "$BUCKET_NAME"
    echo -e "${GREEN}✓ R2 bucket '$BUCKET_NAME' created.${NC}"
fi
echo ""

# Step 3: Configure custom domain
echo -e "${BLUE}Step 3: Configuring custom domain '$CUSTOM_DOMAIN'...${NC}"

if [ -z "$CLOUDFLARE_API_TOKEN" ]; then
    echo -e "${YELLOW}WARNING: CLOUDFLARE_API_TOKEN not set${NC}"
    echo "Custom domain configuration requires manual setup via Cloudflare Dashboard:"
    echo ""
    echo "1. Go to: https://dash.cloudflare.com/"
    echo "2. Navigate to: R2 > $BUCKET_NAME > Settings > Custom Domains"
    echo "3. Add domain: $CUSTOM_DOMAIN"
    echo ""
    echo "Get your API token from: https://dash.cloudflare.com/profile/api-tokens"
    echo "Required permissions: Account > Cloudflare Pages > Edit"
    echo ""
    echo "Then run: CLOUDFLARE_API_TOKEN=your_token $0"
    echo ""
else
    echo "Using Cloudflare API to configure custom domain..."

    # Cloudflare API base URL
    CF_API="https://api.cloudflare.com/client/v4"

    # Use the account ID we found
    if [ -z "$ACCOUNT_ID" ]; then
        echo -e "${RED}ERROR: Could not determine Account ID${NC}"
        echo "Set CLOUDFLARE_ACCOUNT_ID environment variable."
        exit 1
    fi

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

    # Check if custom domain already exists for this bucket
    echo "Checking existing custom domains..."
    EXISTING_DOMAINS=$(cf_api GET "/accounts/$ACCOUNT_ID/r2/buckets/$BUCKET_NAME/custom_domains")

    # Check if our domain is already configured
    if echo "$EXISTING_DOMAINS" | jq -e '.result[] | select(.domain == "'$CUSTOM_DOMAIN'")' > /dev/null 2>&1; then
        echo -e "${YELLOW}Custom domain '$CUSTOM_DOMAIN' already configured.${NC}"

        # Get current status
        STATUS=$(echo "$EXISTING_DOMAINS" | jq -r '.result[] | select(.domain == "'$CUSTOM_DOMAIN'") | .status // "unknown"')
        echo "Status: $STATUS"

        if [ "$STATUS" = "active" ]; then
            echo -e "${GREEN}✓ Custom domain is active and ready!${NC}"
        else
            echo -e "${YELLOW}Custom domain configuration in progress. Check dashboard for status.${NC}"
        fi
    else
        echo "Adding custom domain '$CUSTOM_DOMAIN' to bucket '$BUCKET_NAME'..."

        # Add custom domain via API
        RESPONSE=$(cf_api POST "/accounts/$ACCOUNT_ID/r2/buckets/$BUCKET_NAME/custom_domains" \
            "{\"domain\": \"$CUSTOM_DOMAIN\"}")

        SUCCESS=$(echo "$RESPONSE" | jq -r '.success // false')
        if [ "$SUCCESS" = "true" ]; then
            echo -e "${GREEN}✓ Custom domain '$CUSTOM_DOMAIN' configured successfully!${NC}"
            echo ""
            echo "DNS Configuration:"
            echo "  CNAME: $CUSTOM_DOMAIN -> $BUCKET_NAME.r2.cloudflarestorage.com"
            echo ""
            echo "The custom domain will be ready once DNS propagates."
            echo "Check status at: https://dash.cloudflare.com/$ACCOUNT_ID/r2/$BUCKET_NAME/custom_domains"
        else
            ERRORS=$(echo "$RESPONSE" | jq -r '.errors[0].message // "Unknown error"')
            echo -e "${RED}✗ Failed to configure custom domain: $ERRORS${NC}"
            echo ""
            echo "Manual configuration required:"
            echo "1. Go to: R2 > $BUCKET_NAME > Settings > Custom Domains"
            echo "2. Add domain: $CUSTOM_DOMAIN"
        fi
    fi
fi

echo ""
echo -e "${BLUE}=== R2 Bucket Setup Summary ===${NC}"
echo ""
echo "Bucket: $BUCKET_NAME"
echo "Custom Domain: $CUSTOM_DOMAIN"
echo ""
echo "Expected R2 data layout:"
echo "  replays/{match_id}.json.gz       - Recent replay files"
echo "  matches/{match_id}.json          - Recent per-match metadata"
echo "  thumbnails/{match_id}.png        - Match thumbnails"
echo "  cards/{bot_id}.png               - Bot profile cards"
echo "  evolution/live.json              - Evolution live feed"
echo ""
echo "Public URL: https://$CUSTOM_DOMAIN/"
echo ""
echo -e "${GREEN}=== Setup Complete! ===${NC}"
echo ""
echo "Verification commands:"
echo "  ~/.local/bin/wrangler r2 bucket list"
echo "  curl -I https://$CUSTOM_DOMAIN/"
echo ""
