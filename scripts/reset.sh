#!/bin/bash
# OCM Reset Script
#
# Usage: ./scripts/reset.sh
#
# Complete reset: tears down everything and starts fresh.
# Equivalent to: clean.sh full && quickstart.sh
#
# ⚠️  DESTRUCTIVE: Deletes all data, credentials, and configuration!

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
echo -e "${BLUE}🔄 OCM Reset${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "This will:"
echo "  1. Stop all containers"
echo "  2. Remove all images"
echo "  3. Delete all volumes (credentials, database)"
echo "  4. Delete .env (secrets)"
echo "  5. Run fresh setup"
echo "  6. Start the stack"
echo ""
echo -e "${YELLOW}${BOLD}⚠️  ALL DATA WILL BE LOST!${NC}"
echo ""
read -p "Type 'reset' to confirm: " -r

if [[ "$REPLY" != "reset" ]]; then
    echo ""
    echo -e "${BLUE}ℹ${NC}  Cancelled"
    exit 0
fi

echo ""

# Run full cleanup (skip confirmation since we already confirmed)
echo -e "${BLUE}[1/2]${NC} Cleaning up..."
echo ""

# Stop containers
for compose_file in docker-compose.yml docker-compose.openclaw.yml; do
    if [ -f "$compose_file" ]; then
        docker compose -f "$compose_file" down 2>/dev/null || true
    fi
done

# Remove images
docker rmi ocm:local 2>/dev/null || true
docker rmi openclaw:local 2>/dev/null || true

# Remove volumes
for vol in $(docker volume ls -q | grep -E "^ocm[_-]" 2>/dev/null || true); do
    docker volume rm "$vol" 2>/dev/null || true
done

# Remove project .env
rm -f .env

# Also clear any stale gateway token from OpenClaw's config dir
# (OpenClaw loads ~/.openclaw/.env which might have an old token)
OC_CONFIG_DIR="${OPENCLAW_CONFIG_DIR:-$HOME/.openclaw}"
if [ -f "$OC_CONFIG_DIR/.env" ]; then
    echo -e "${BLUE}ℹ${NC}  Clearing stale tokens from $OC_CONFIG_DIR/.env"
    # Remove gateway token line but keep other credentials (OCM manages those)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' '/OPENCLAW_GATEWAY_TOKEN/d' "$OC_CONFIG_DIR/.env"
    else
        sed -i '/OPENCLAW_GATEWAY_TOKEN/d' "$OC_CONFIG_DIR/.env"
    fi
fi

echo -e "${GREEN}✓${NC}  Cleanup complete"
echo ""

# Run quickstart
echo -e "${BLUE}[2/2]${NC} Starting fresh..."
echo ""
./scripts/quickstart.sh
