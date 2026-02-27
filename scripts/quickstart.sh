#!/bin/bash
# OCM Quickstart - Get running in one command
#
# Usage: ./scripts/quickstart.sh
#
# This script:
# 1. Runs setup (generates secure keys, creates .env)
# 2. Builds the OpenClaw image (if needed)
# 3. Builds OCM
# 4. Starts the full stack
#
# After completion:
# - OpenClaw Gateway: http://localhost:18789
# - OCM Admin UI:     http://localhost:8080

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

echo ""
echo -e "${BOLD}⚡ OCM Quickstart${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Step 1: Setup
echo -e "${BLUE}[1/4]${NC} Running setup..."
echo ""
./scripts/setup.sh
echo ""

# Step 2: Check/Build OpenClaw image
echo -e "${BLUE}[2/4]${NC} Checking OpenClaw image..."
source .env
OPENCLAW_IMAGE=${OPENCLAW_IMAGE:-openclaw:local}

if docker image inspect "$OPENCLAW_IMAGE" &>/dev/null; then
    echo -e "${GREEN}✓${NC}  Found: $OPENCLAW_IMAGE"
else
    echo -e "${YELLOW}⚠${NC}  OpenClaw image not found, building..."
    echo ""
    ./scripts/build-openclaw.sh
fi
echo ""

# Step 3: Build OCM
echo -e "${BLUE}[3/4]${NC} Building OCM..."
docker build -t ocm:local . --quiet
echo -e "${GREEN}✓${NC}  Built: ocm:local"
echo ""

# Step 4: Start stack
echo -e "${BLUE}[4/4]${NC} Starting stack..."
docker compose -f docker-compose.openclaw.yml up -d
echo ""

# Done!
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${GREEN}${BOLD}✅ OCM + OpenClaw are running!${NC}"
echo ""
echo "  🌐 OpenClaw Gateway: http://localhost:${OPENCLAW_GATEWAY_PORT:-18789}"
echo "  🔐 OCM Admin UI:     http://localhost:${OCM_ADMIN_PORT:-8080}"
echo ""
echo -e "${BLUE}Commands:${NC}"
echo "  View logs:    docker compose -f docker-compose.openclaw.yml logs -f"
echo "  Stop:         docker compose -f docker-compose.openclaw.yml down"
echo "  Restart:      docker compose -f docker-compose.openclaw.yml restart"
echo ""
echo -e "${BLUE}What's next:${NC}"
echo "  1. Open the OCM Admin UI to add credentials"
echo "  2. Connect to OpenClaw Gateway from your client"
echo "  3. Your agent can now use credentials without seeing them!"
echo ""
