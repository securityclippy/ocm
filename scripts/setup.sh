#!/bin/bash
# OCM Setup Script
# Creates secure configuration for OCM + OpenClaw
#
# Usage: ./scripts/setup.sh [--force]
#
# Security:
# - Generates cryptographically secure keys
# - Sets restrictive file permissions (600)
# - Never echoes secrets to terminal
# - Validates all inputs

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() { echo -e "${BLUE}ℹ${NC}  $1"; }
success() { echo -e "${GREEN}✓${NC}  $1"; }
warn() { echo -e "${YELLOW}⚠${NC}  $1"; }
error() { echo -e "${RED}✗${NC}  $1" >&2; }

# ===========================================
# Pre-flight checks
# ===========================================

preflight_checks() {
    local failed=0

    # Check openssl
    if ! command -v openssl &>/dev/null; then
        error "openssl not found (required for key generation)"
        failed=1
    fi

    # Check docker (warning only)
    if ! command -v docker &>/dev/null; then
        warn "docker not found - you'll need it to run the stack"
    fi

    # Check docker compose
    if ! docker compose version &>/dev/null 2>&1; then
        warn "docker compose not found - you'll need it to run the stack"
    fi

    # Warn if running as root
    if [ "$EUID" -eq 0 ]; then
        warn "Running as root is not recommended"
        warn "Containers should run as non-root user"
        echo ""
        read -p "Continue anyway? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    if [ $failed -eq 1 ]; then
        exit 1
    fi
    
    # Check for hardcoded token in openclaw.docker.json5
    if [ -f openclaw.docker.json5 ]; then
        if grep -q '"token"' openclaw.docker.json5 2>/dev/null; then
            warn "openclaw.docker.json5 contains a hardcoded gateway token"
            warn "This will cause token mismatch errors with OCM"
            echo ""
            read -p "   Reset to default? (backup will be saved) [Y/n] " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Nn]$ ]]; then
                # Backup existing config
                backup_file="openclaw.docker.json5.backup.$(date +%Y%m%d_%H%M%S)"
                cp openclaw.docker.json5 "$backup_file"
                info "Backed up to: $backup_file"
                
                # Reset to default config
                cat > openclaw.docker.json5 << 'OCCONFIG'
// OpenClaw config for Docker deployment with OCM
// Mount this to /home/node/.openclaw/openclaw.json
{
  gateway: {
    controlUi: {
      enabled: true,
      // Allow CORS from any origin using Host header (safe for local/dev)
      dangerouslyAllowHostHeaderOriginFallback: true,
    },
  },
}
OCCONFIG
                success "Reset openclaw.docker.json5 to default"
            else
                warn "Keeping hardcoded token - you may need to sync manually"
            fi
        fi
    fi
}

# ===========================================
# Detect existing OpenClaw installation
# ===========================================

detect_openclaw() {
    local default_config="$HOME/.openclaw"
    
    # Check common locations
    if [ -d "$default_config" ]; then
        if [ -f "$default_config/openclaw.json" ] || [ -f "$default_config/openclaw.json5" ]; then
            echo "$default_config"
            return 0
        fi
    fi
    
    # Check if there's an existing OpenClaw container
    if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^openclaw$"; then
        # Try to get the config path from container
        local config_path
        config_path=$(docker inspect openclaw --format '{{range .Mounts}}{{if eq .Destination "/home/node/.openclaw"}}{{.Source}}{{end}}{{end}}' 2>/dev/null || true)
        if [ -n "$config_path" ] && [ -d "$config_path" ]; then
            echo "$config_path"
            return 0
        fi
    fi
    
    echo ""
    return 1
}

# ===========================================
# Generate secure key
# ===========================================

generate_key() {
    openssl rand -hex 32
}

# ===========================================
# Main setup
# ===========================================

main() {
    echo ""
    echo -e "${BLUE}🔐 OCM Setup${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    preflight_checks

    local force=0
    if [ "$1" = "--force" ]; then
        force=1
    fi

    # Check for existing .env
    if [ -f .env ] && [ $force -eq 0 ]; then
        info "Found existing .env file"
        echo ""
        read -p "   Overwrite? (secrets will be regenerated) [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo ""
            info "Keeping existing configuration"
            info "Run with --force to overwrite"
            exit 0
        fi
        echo ""
    fi

    # Detect or prompt for paths
    local detected_path
    detected_path=$(detect_openclaw)
    
    if [ -n "$detected_path" ]; then
        info "Detected existing OpenClaw config: $detected_path"
        OPENCLAW_CONFIG_DIR="$detected_path"
    else
        OPENCLAW_CONFIG_DIR="${OPENCLAW_CONFIG_DIR:-$HOME/.openclaw}"
        info "Using default config path: $OPENCLAW_CONFIG_DIR"
    fi
    
    OPENCLAW_WORKSPACE_DIR="${OPENCLAW_WORKSPACE_DIR:-$OPENCLAW_CONFIG_DIR/workspace}"

    # Create directories with correct permissions
    echo ""
    info "Creating directories..."
    mkdir -p "$OPENCLAW_CONFIG_DIR"
    mkdir -p "$OPENCLAW_WORKSPACE_DIR"
    chmod 700 "$OPENCLAW_CONFIG_DIR"
    success "Config directory: $OPENCLAW_CONFIG_DIR"
    success "Workspace directory: $OPENCLAW_WORKSPACE_DIR"

    # Generate secrets
    echo ""
    info "Generating secure keys..."
    local master_key gateway_token
    master_key=$(generate_key)
    gateway_token=$(generate_key)
    success "Master key generated (256-bit AES)"
    success "Gateway token generated"

    # Write .env with restricted permissions
    echo ""
    info "Writing configuration..."
    
    # Create with restrictive permissions from the start
    umask 077
    cat > .env << EOF
# OCM + OpenClaw Configuration
# Generated by setup.sh on $(date -u +"%Y-%m-%d %H:%M:%S UTC")
#
# ⚠️  SECURITY: This file contains secrets. Keep it safe.
#     - Do not commit to version control
#     - Do not share or expose publicly
#     - Permissions should be 600 (owner read/write only)

# ===========================================
# Secrets (auto-generated)
# ===========================================

# OCM master encryption key - encrypts all stored credentials
# If lost, stored credentials cannot be recovered
OCM_MASTER_KEY=$master_key

# OpenClaw Gateway authentication token
OPENCLAW_GATEWAY_TOKEN=$gateway_token

# ===========================================
# Paths
# ===========================================

# Host directory containing OpenClaw config
OPENCLAW_CONFIG_DIR=$OPENCLAW_CONFIG_DIR

# Host directory for agent workspace
OPENCLAW_WORKSPACE_DIR=$OPENCLAW_WORKSPACE_DIR

# ===========================================
# Optional: Ports (uncomment to customize)
# ===========================================

# OPENCLAW_GATEWAY_PORT=18789
# OCM_ADMIN_PORT=8080

# ===========================================
# Optional: Custom OpenClaw image
# ===========================================

# OPENCLAW_IMAGE=openclaw:local
EOF

    chmod 600 .env
    success "Configuration saved to .env (mode 600)"

    # Also write gateway token to OpenClaw config dir
    # This is needed for CLI commands (docker exec) to authenticate with the gateway
    # The gateway process gets the token from docker-compose env vars, but CLI processes
    # spawned via docker exec don't inherit those - they need to read from .env
    local oc_env_file="$OPENCLAW_CONFIG_DIR/.env"
    info "Writing gateway token to $oc_env_file..."
    
    # Remove any existing gateway token line
    if [ -f "$oc_env_file" ]; then
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' '/OPENCLAW_GATEWAY_TOKEN/d' "$oc_env_file"
        else
            sed -i '/OPENCLAW_GATEWAY_TOKEN/d' "$oc_env_file"
        fi
    fi
    
    # Append the new token
    echo "OPENCLAW_GATEWAY_TOKEN=$gateway_token" >> "$oc_env_file"
    chmod 600 "$oc_env_file"
    success "Gateway token written to $oc_env_file"

    # Backup reminder
    echo ""
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}⚠️  IMPORTANT: Back up your .env file!${NC}"
    echo -e "${YELLOW}   The master key encrypts all credentials.${NC}"
    echo -e "${YELLOW}   If lost, stored credentials cannot be recovered.${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    # Next steps
    echo ""
    echo -e "${GREEN}✅ Setup complete!${NC}"
    echo ""
    echo "Next steps:"
    echo ""
    echo "  ${BLUE}Quick start (builds everything):${NC}"
    echo "    ./scripts/quickstart.sh"
    echo ""
    echo "  ${BLUE}Or step by step:${NC}"
    echo "    1. Build OpenClaw:  ./scripts/build-openclaw.sh"
    echo "    2. Start stack:     ./scripts/docker.sh"
    echo ""
    echo "  ${BLUE}For local development:${NC}"
    echo "    ./scripts/dev.sh"
    echo ""
}

main "$@"
