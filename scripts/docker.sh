#!/bin/bash
# OCM Docker Setup Script
# Usage: ./scripts/docker.sh [ocm-only|with-openclaw]

set -e

MODE=${1:-with-openclaw}

echo "🐳 OCM Docker Setup"
echo "==================="

# Run setup first (generates keys if needed)
./scripts/setup.sh

# Build OCM image
echo ""
echo "🔨 Building OCM Docker image..."
docker build -t ocm:local .
echo "✅ Image built: ocm:local"

if [ "$MODE" = "ocm-only" ]; then
    echo ""
    echo "🚀 Starting OCM standalone..."
    docker compose up -d
    echo ""
    echo "✅ OCM is running!"
    echo "   Admin UI: http://localhost:8080"
    echo ""
    echo "📋 Commands:"
    echo "   docker compose logs -f      # View logs"
    echo "   docker compose down          # Stop"
else
    echo ""
    echo "🚀 Starting OCM + OpenClaw..."
    
    # Verify tokens are set
    if grep -q "^OPENCLAW_GATEWAY_TOKEN=$" .env 2>/dev/null; then
        echo "❌ OPENCLAW_GATEWAY_TOKEN is empty. Run ./scripts/setup.sh"
        exit 1
    fi
    if grep -q "^OCM_MASTER_KEY=$" .env 2>/dev/null; then
        echo "❌ OCM_MASTER_KEY is empty. Run ./scripts/setup.sh"
        exit 1
    fi
    
    docker compose -f docker-compose.openclaw.yml up -d
    echo ""
    echo "✅ OCM + OpenClaw are running!"
    echo "   OpenClaw Gateway: http://localhost:18789"
    echo "   OCM Admin UI:     http://localhost:8080"
    echo ""
    echo "📋 Commands:"
    echo "   docker compose -f docker-compose.openclaw.yml logs -f   # View logs"
    echo "   docker compose -f docker-compose.openclaw.yml down      # Stop"
fi
