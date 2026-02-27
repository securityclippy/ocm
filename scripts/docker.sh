#!/bin/bash
# OCM Docker Setup Script
#
# Usage: ./scripts/docker.sh [ocm-only|with-openclaw]
#
# Builds and starts the Docker stack.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

MODE=${1:-with-openclaw}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() { echo -e "${BLUE}ℹ${NC}  $1"; }
success() { echo -e "${GREEN}✓${NC}  $1"; }
warn() { echo -e "${YELLOW}⚠${NC}  $1"; }
error() { echo -e "${RED}✗${NC}  $1" >&2; }

echo ""
echo -e "${BLUE}🐳 OCM Docker Setup${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Run setup if .env doesn't exist
if [ ! -f .env ]; then
    info "No .env found, running setup..."
    echo ""
    ./scripts/setup.sh
    echo ""
fi

# Load and validate .env
source .env

validate_env() {
    local missing=0
    
    if [ -z "$OCM_MASTER_KEY" ]; then
        error "OCM_MASTER_KEY is not set"
        missing=1
    elif [ ${#OCM_MASTER_KEY} -ne 64 ]; then
        error "OCM_MASTER_KEY must be 64 hex characters (got ${#OCM_MASTER_KEY})"
        missing=1
    fi
    
    if [ -z "$OPENCLAW_GATEWAY_TOKEN" ]; then
        error "OPENCLAW_GATEWAY_TOKEN is not set"
        missing=1
    fi
    
    if [ -z "$OPENCLAW_CONFIG_DIR" ]; then
        error "OPENCLAW_CONFIG_DIR is not set"
        missing=1
    elif [ ! -d "$OPENCLAW_CONFIG_DIR" ]; then
        warn "OPENCLAW_CONFIG_DIR does not exist: $OPENCLAW_CONFIG_DIR"
        info "Creating directory..."
        mkdir -p "$OPENCLAW_CONFIG_DIR"
        chmod 700 "$OPENCLAW_CONFIG_DIR"
    fi
    
    if [ -z "$OPENCLAW_WORKSPACE_DIR" ]; then
        error "OPENCLAW_WORKSPACE_DIR is not set"
        missing=1
    elif [ ! -d "$OPENCLAW_WORKSPACE_DIR" ]; then
        warn "OPENCLAW_WORKSPACE_DIR does not exist: $OPENCLAW_WORKSPACE_DIR"
        info "Creating directory..."
        mkdir -p "$OPENCLAW_WORKSPACE_DIR"
    fi
    
    if [ $missing -eq 1 ]; then
        echo ""
        error "Missing required configuration. Run: ./scripts/setup.sh"
        exit 1
    fi
}

validate_env
success "Configuration validated"

# Build OCM image
echo ""
info "Building OCM Docker image..."
docker build -t ocm:local . --quiet
success "Built: ocm:local"

if [ "$MODE" = "ocm-only" ]; then
    echo ""
    info "Starting OCM standalone..."
    docker compose up -d
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    success "OCM is running!"
    echo ""
    echo "  🔐 Admin UI: http://localhost:${OCM_ADMIN_PORT:-8080}"
    echo ""
    echo -e "${BLUE}Commands:${NC}"
    echo "  View logs:  docker compose logs -f"
    echo "  Stop:       docker compose down"
    echo ""
else
    # Check for OpenClaw image
    OPENCLAW_IMAGE=${OPENCLAW_IMAGE:-openclaw:local}
    
    if ! docker image inspect "$OPENCLAW_IMAGE" &>/dev/null; then
        echo ""
        warn "OpenClaw image '$OPENCLAW_IMAGE' not found"
        echo ""
        read -p "   Build it now? (clones from GitHub) [Y/n] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Nn]$ ]]; then
            echo ""
            info "To build manually: ./scripts/build-openclaw.sh"
            info "Or run OCM standalone: ./scripts/docker.sh ocm-only"
            exit 1
        fi
        
        echo ""
        ./scripts/build-openclaw.sh
    else
        success "Found OpenClaw image: $OPENCLAW_IMAGE"
    fi
    
    echo ""
    info "Starting OCM + OpenClaw..."
    docker compose -f docker-compose.openclaw.yml up -d
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    success "OCM + OpenClaw are running!"
    echo ""
    echo "  🌐 OpenClaw Gateway: http://localhost:${OPENCLAW_GATEWAY_PORT:-18789}"
    echo "  🔐 OCM Admin UI:     http://localhost:${OCM_ADMIN_PORT:-8080}"
    echo ""
    echo -e "${BLUE}Commands:${NC}"
    echo "  View logs:  docker compose -f docker-compose.openclaw.yml logs -f"
    echo "  Stop:       docker compose -f docker-compose.openclaw.yml down"
    echo "  Restart:    docker compose -f docker-compose.openclaw.yml restart"
    echo ""
fi
