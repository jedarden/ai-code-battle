#!/bin/bash
# Cloudflare Setup Script for AI Code Battle
# Run this script after authenticating with wrangler

set -e

echo "=== AI Code Battle - Cloudflare Setup ==="
echo ""

# Check if wrangler is authenticated
if ! ~/.local/bin/wrangler whoami &>/dev/null; then
    echo "ERROR: Not authenticated with Cloudflare."
    echo "Run: ~/.local/bin/wrangler login"
    exit 1
fi

echo "Authenticated as:"
~/.local/bin/wrangler whoami
echo ""

# Step 1: Create Pages project
echo "=== Step 1: Creating Pages project 'aicodebattle' ==="
if ~/.local/bin/wrangler pages project list 2>/dev/null | grep -q "aicodebattle"; then
    echo "Pages project 'aicodebattle' already exists."
else
    echo "Creating Pages project..."
    ~/.local/bin/wrangler pages project create aicodebattle --production-branch master
fi
echo ""

# Step 2: Deploy to Pages
echo "=== Step 2: Deploying web/dist to Pages ==="
~/.local/bin/wrangler pages deploy web/dist --project-name=aicodebattle --commit-dirty=true
echo ""

echo "=== Automated setup complete! ==="
echo ""
echo "=== MANUAL STEPS REQUIRED (Cloudflare Dashboard) ==="
echo ""
echo "1. Configure Custom Domain for Pages:"
echo "   - Go to: Workers & Pages > aicodebattle > Settings > Custom domains"
echo "   - Add domain: aicodebattle.com"
echo "   - This auto-configures DNS CNAME"
echo ""
echo "2. Configure API DNS (requires Traefik LoadBalancer IP):"
echo "   - Go to: DNS > Records"
echo "   - Add A record: api.aicodebattle.com -> <Traefik IP> (proxied)"
echo ""
echo "3. Get Traefik IP from Kubernetes:"
echo "   kubectl --server=http://kubectl-apexalgo-iad:8001 get svc -n traefik"
echo ""
echo "=== B2 CDN Setup (Separate from Cloudflare) ==="
echo ""
echo "B2 storage setup is documented separately. See ./scripts/setup-b2.sh for:"
echo "  - B2 bucket endpoint information"
echo "  - CNAME configuration for b2.aicodebattle.com"
echo "  - B2 API authentication verification"
echo ""
echo "Run: ./scripts/setup-b2.sh"
echo ""
