#!/bin/bash
# OCM Quick Setup Script
# Usage: ./scripts/setup.sh

set -e

echo "🔧 OCM Setup"
echo "============"

# Check for .env
if [ ! -f .env ]; then
    echo "📄 Creating .env from template..."
    cp .env.example .env
    
    # Generate master key
    echo "🔑 Generating master key..."
    MASTER_KEY=$(openssl rand -hex 32)
    
    # Update .env with the key
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/^OCM_MASTER_KEY=.*/OCM_MASTER_KEY=$MASTER_KEY/" .env
    else
        sed -i "s/^OCM_MASTER_KEY=.*/OCM_MASTER_KEY=$MASTER_KEY/" .env
    fi
    
    echo "✅ Generated master key and saved to .env"
else
    echo "📄 .env already exists, skipping..."
fi

# Check if OCM_MASTER_KEY is set
if grep -q "^OCM_MASTER_KEY=$" .env 2>/dev/null; then
    echo "⚠️  OCM_MASTER_KEY is empty in .env"
    echo "   Run: openssl rand -hex 32"
    echo "   And paste the result into .env"
fi

# Check if OPENCLAW_GATEWAY_TOKEN is set
if grep -q "^OPENCLAW_GATEWAY_TOKEN=your-gateway-token-here" .env 2>/dev/null; then
    echo ""
    echo "⚠️  Don't forget to set OPENCLAW_GATEWAY_TOKEN in .env"
    echo "   (Required for OpenClaw integration)"
fi

echo ""
echo "📦 Next steps:"
echo "   1. Edit .env if needed"
echo "   2. Run: ./scripts/dev.sh      # Local development"
echo "   3. Or:  ./scripts/docker.sh   # Docker setup"
