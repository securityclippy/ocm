#!/bin/bash
# OCM Cleanup Script
#
# Usage: ./scripts/clean.sh [ocm|all|volumes|full]
#
# Modes:
#   ocm      - Stop OCM container, remove image (default)
#   all      - Stop all containers (OCM + OpenClaw), remove images
#   volumes  - Remove data volumes (requires confirmation)
#   full     - Complete teardown: containers, images, volumes, .env
#
# Safe by default - won't delete data without confirmation.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

MODE=${1:-ocm}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info() { echo -e "${BLUE}ℹ${NC}  $1"; }
success() { echo -e "${GREEN}✓${NC}  $1"; }
warn() { echo -e "${YELLOW}⚠${NC}  $1"; }
error() { echo -e "${RED}✗${NC}  $1" >&2; }

confirm() {
    local prompt="$1"
    echo ""
    echo -e "${YELLOW}${BOLD}⚠️  $prompt${NC}"
    read -p "   Type 'yes' to confirm: " -r
    if [[ "$REPLY" != "yes" ]]; then
        info "Cancelled"
        return 1
    fi
    return 0
}

stop_containers() {
    local compose_file="$1"
    if [ -f "$compose_file" ]; then
        info "Stopping containers ($compose_file)..."
        docker compose -f "$compose_file" down 2>/dev/null || true
        success "Containers stopped"
    fi
}

remove_images() {
    info "Removing images..."
    docker rmi ocm:local 2>/dev/null && success "Removed: ocm:local" || true
    if [ "$1" = "all" ]; then
        docker rmi openclaw:local 2>/dev/null && success "Removed: openclaw:local" || true
    fi
}

remove_volumes() {
    info "Removing volumes..."
    
    # List OCM-related volumes
    local volumes=$(docker volume ls -q | grep -E "^ocm[_-]" 2>/dev/null || true)
    
    if [ -z "$volumes" ]; then
        info "No OCM volumes found"
        return
    fi
    
    for vol in $volumes; do
        docker volume rm "$vol" 2>/dev/null && success "Removed volume: $vol" || warn "Could not remove: $vol"
    done
}

remove_env() {
    if [ -f .env ]; then
        info "Removing .env..."
        rm .env
        success "Removed: .env"
    fi
}

echo ""
echo -e "${BLUE}🧹 OCM Cleanup${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

case $MODE in
    ocm)
        info "Mode: OCM only (containers + image)"
        echo ""
        stop_containers "docker-compose.yml"
        stop_containers "docker-compose.openclaw.yml"
        remove_images "ocm"
        ;;
        
    all)
        info "Mode: All containers + images"
        echo ""
        stop_containers "docker-compose.yml"
        stop_containers "docker-compose.openclaw.yml"
        remove_images "all"
        ;;
        
    volumes)
        info "Mode: Remove data volumes"
        if confirm "This will DELETE all OCM data (credentials, database, audit logs)!"; then
            echo ""
            stop_containers "docker-compose.yml"
            stop_containers "docker-compose.openclaw.yml"
            remove_volumes
        fi
        ;;
        
    full)
        info "Mode: Complete teardown"
        if confirm "This will DELETE everything: containers, images, volumes, and .env!"; then
            echo ""
            stop_containers "docker-compose.yml"
            stop_containers "docker-compose.openclaw.yml"
            remove_images "all"
            remove_volumes
            remove_env
            echo ""
            success "Complete teardown finished"
            echo ""
            info "To start fresh: ./scripts/quickstart.sh"
        fi
        ;;
        
    *)
        echo "Usage: ./scripts/clean.sh [ocm|all|volumes|full]"
        echo ""
        echo "Modes:"
        echo "  ocm      Stop OCM, remove ocm:local image (default)"
        echo "  all      Stop all, remove all images"
        echo "  volumes  Remove data volumes (destructive!)"
        echo "  full     Complete teardown (destructive!)"
        exit 1
        ;;
esac

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
success "Cleanup complete"
echo ""
