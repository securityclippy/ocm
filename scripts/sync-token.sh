#!/bin/bash
# Sync OCM's gateway token with OpenClaw's existing token
#
# Use this when you get a "token mismatch" error - it reads OpenClaw's
# configured token and updates OCM's .env to match.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo ""
echo -e "${BLUE}🔑 Token Sync${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Try to get token from OpenClaw's config
echo -e "${BLUE}ℹ${NC}  Reading OpenClaw's gateway token..."

# Try multiple locations where the token might be
TOKEN=""

# Method 1: From openclaw.json gateway.auth.token
if [ -z "$TOKEN" ]; then
    TOKEN=$(docker exec openclaw cat /home/node/.openclaw/openclaw.json 2>/dev/null | grep -o '"token"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | cut -d'"' -f4 || true)
fi

# Method 2: From environment variable in container
if [ -z "$TOKEN" ]; then
    TOKEN=$(docker exec openclaw printenv OPENCLAW_GATEWAY_TOKEN 2>/dev/null || true)
fi

# Method 3: From .env file in container
if [ -z "$TOKEN" ]; then
    TOKEN=$(docker exec openclaw grep OPENCLAW_GATEWAY_TOKEN /home/node/.openclaw/.env 2>/dev/null | cut -d= -f2 || true)
fi

if [ -z "$TOKEN" ]; then
    echo -e "${RED}✗${NC}  Could not find OpenClaw's gateway token"
    echo ""
    echo "   Checked:"
    echo "   - /home/node/.openclaw/openclaw.json (gateway.auth.token)"
    echo "   - OPENCLAW_GATEWAY_TOKEN env var"
    echo "   - /home/node/.openclaw/.env"
    echo ""
    echo "   Make sure OpenClaw is running and has a gateway token configured."
    exit 1
fi

echo -e "${GREEN}✓${NC}  Found token: ${TOKEN:0:16}..."
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo -e "${RED}✗${NC}  .env file not found. Run ./scripts/setup.sh first."
    exit 1
fi

# Get current OCM token
CURRENT_TOKEN=$(grep OPENCLAW_GATEWAY_TOKEN .env | cut -d= -f2 || true)

if [ "$CURRENT_TOKEN" = "$TOKEN" ]; then
    echo -e "${GREEN}✓${NC}  Tokens already match!"
    exit 0
fi

echo -e "${YELLOW}⚠${NC}  Current OCM token: ${CURRENT_TOKEN:0:16}..."
echo -e "${BLUE}ℹ${NC}  Updating to match OpenClaw..."

# Update .env
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s/^OPENCLAW_GATEWAY_TOKEN=.*/OPENCLAW_GATEWAY_TOKEN=$TOKEN/" .env
else
    sed -i "s/^OPENCLAW_GATEWAY_TOKEN=.*/OPENCLAW_GATEWAY_TOKEN=$TOKEN/" .env
fi

echo -e "${GREEN}✓${NC}  Updated .env"
echo ""

# Recreate OCM to pick up .env changes (restart doesn't reload .env)
echo -e "${BLUE}ℹ${NC}  Recreating OCM container..."
docker compose -f docker-compose.openclaw.yml up -d --force-recreate ocm

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✓${NC}  Token synced! Refresh the OCM UI to verify."
echo ""
