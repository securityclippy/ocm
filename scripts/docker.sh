#!/bin/bash
# OCM Docker Setup Script
# Usage: ./scripts/docker.sh [ocm-only|with-openclaw]

set -e

MODE=${1:-with-openclaw}

echo "🐳 OCM Docker Setup"
echo "==================="

# Run setup first
if [ ! -f .env ]; then
    ./scripts/setup.sh
fi

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
    
    # Check for gateway token
    if grep -q "^OPENCLAW_GATEWAY_TOKEN=your-gateway-token-here" .env 2>/dev/null; then
        echo ""
        echo "⚠️  OPENCLAW_GATEWAY_TOKEN is not set in .env"
        echo "   Please edit .env and set your gateway token."
        echo ""
        read -p "Continue anyway? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
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
