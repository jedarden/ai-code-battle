#!/bin/bash
# Upload test replays to R2 bucket
# Requires: wrangler CLI installed and authenticated
#
# Prerequisites:
#   1. Run ./scripts/generate-test-replays.sh first
#   2. Install wrangler: npm install -g wrangler
#   3. Authenticate: wrangler login

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
BUCKET_NAME="acb-data"
REPLAY_DIR="test-replays"

echo "=== Uploading Test Replays to R2 ==="
echo ""

# Check if wrangler is available
if ! command -v wrangler &> /dev/null; then
    if [ -f ~/.local/bin/wrangler ]; then
        WRANGLER=~/.local/bin/wrangler
    else
        echo -e "${RED}ERROR: wrangler not found${NC}"
        echo "Install with: npm install -g wrangler"
        echo "Then authenticate: wrangler login"
        exit 1
    fi
else
    WRANGLER=wrangler
fi

# Check authentication
echo "Checking wrangler authentication..."
if ! $WRANGLER whoami &>/dev/null; then
    echo -e "${RED}ERROR: Not authenticated with Cloudflare${NC}"
    echo "Run: $WRANGLER login"
    exit 1
fi
$WRANGLER whoami
echo ""

# Check if replay directory exists
if [ ! -d "$REPLAY_DIR" ]; then
    echo -e "${RED}ERROR: $REPLAY_DIR not found${NC}"
    echo "Run: ./scripts/generate-test-replays.sh"
    exit 1
fi

# Upload each replay
echo "Uploading replays to R2 bucket '$BUCKET_NAME'..."
for file in "$REPLAY_DIR"/*.json.gz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        key="replays/$filename"
        echo -n "Uploading $key... "
        $WRANGLER r2 object put "$BUCKET_NAME/$key" --file="$file" &>/dev/null
        echo -e "${GREEN}OK${NC}"
    fi
done

echo ""
echo -e "${GREEN}=== Upload Complete! ===${NC}"
echo ""
echo "Verify with:"
echo "  curl -I https://r2.aicodebattle.com/replays/m_test_upset_v1.json.gz"
echo "  or test in viewer: https://ai-code-battle.pages.dev/#/watch/replay?url=/r2/replays/m_test_upset_v1.json.gz"
