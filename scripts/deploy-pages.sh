#!/bin/bash
# Cloudflare Pages Deployment Script
# Deploys the AI Code Battle SPA to Cloudflare Pages

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== AI Code Battle - Cloudflare Pages Deployment ===${NC}"
echo ""

# Check if wrangler is installed
if ! command -v wrangler &> /dev/null; then
    echo -e "${RED}ERROR: wrangler is not installed${NC}"
    echo "Install with: npm install -g wrangler"
    exit 1
fi

# Check if authenticated
echo -e "${BLUE}Checking authentication...${NC}"
if ! wrangler whoami &> /dev/null; then
    echo -e "${YELLOW}Not authenticated with Cloudflare${NC}"
    echo "Please run: wrangler login"
    echo ""
    echo "Or set environment variables:"
    echo "  export CLOUDFLARE_API_TOKEN=your_token"
    echo "  export CLOUDFLARE_ACCOUNT_ID=your_account_id"
    exit 1
fi

echo -e "${GREEN}Authenticated as:${NC}"
wrangler whoami
echo ""

# Build the site
echo -e "${BLUE}Building the site...${NC}"
cd web
npm install
npm run build
cd ..
echo -e "${GREEN}✓ Build complete${NC}"
echo ""

# Deploy to Pages
echo -e "${BLUE}Deploying to Cloudflare Pages...${NC}"
wrangler pages deploy web/dist --project-name=ai-code-battle --commit-dirty=true
echo ""

echo -e "${GREEN}=== Deployment Complete! ===${NC}"
echo ""
echo "Site URLs:"
echo "  - Pages URL: https://ai-code-battle.pages.dev"
echo "  - Custom domain: https://aicodebattle.com (if configured)"
echo ""
