#!/bin/bash
# OCM Quickstart - Get running in one command
# Usage: ./scripts/quickstart.sh [local|docker]

set -e

MODE=${1:-docker}

echo "⚡ OCM Quickstart"
echo "================="
echo ""

cd "$(dirname "$0")/.."

case $MODE in
    local)
        echo "Mode: Local development"
        ./scripts/setup.sh
        echo ""
        ./scripts/dev.sh
        ;;
    docker)
        echo "Mode: Docker (OCM + OpenClaw)"
        ./scripts/setup.sh
        echo ""
        ./scripts/docker.sh with-openclaw
        ;;
    docker-ocm)
        echo "Mode: Docker (OCM only)"
        ./scripts/setup.sh
        echo ""
        ./scripts/docker.sh ocm-only
        ;;
    *)
        echo "Usage: ./scripts/quickstart.sh [local|docker|docker-ocm]"
        echo ""
        echo "Options:"
        echo "  local      - Build and run locally (requires Go + Node)"
        echo "  docker     - Run OCM + OpenClaw in Docker (default)"
        echo "  docker-ocm - Run OCM only in Docker"
        exit 1
        ;;
esac
