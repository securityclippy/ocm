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
    echo "🔑 Generating OCM master key..."
    MASTER_KEY=$(openssl rand -hex 32)
    
    # Generate gateway token
    echo "🔑 Generating OpenClaw gateway token..."
    GATEWAY_TOKEN=$(openssl rand -hex 32)
    
    # Update .env with the keys
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/^OCM_MASTER_KEY=.*/OCM_MASTER_KEY=$MASTER_KEY/" .env
        sed -i '' "s/^OPENCLAW_GATEWAY_TOKEN=.*/OPENCLAW_GATEWAY_TOKEN=$GATEWAY_TOKEN/" .env
    else
        sed -i "s/^OCM_MASTER_KEY=.*/OCM_MASTER_KEY=$MASTER_KEY/" .env
        sed -i "s/^OPENCLAW_GATEWAY_TOKEN=.*/OPENCLAW_GATEWAY_TOKEN=$GATEWAY_TOKEN/" .env
    fi
    
    # Add OPENCLAW_IMAGE if not present
    if ! grep -q "^OPENCLAW_IMAGE=" .env 2>/dev/null; then
        echo "OPENCLAW_IMAGE=openclaw:local" >> .env
    fi
    
    echo "✅ Generated keys and saved to .env"
else
    echo "📄 .env already exists"
    
    # Check if keys need to be generated
    NEEDS_UPDATE=false
    
    if grep -q "^OCM_MASTER_KEY=$" .env 2>/dev/null; then
        echo "🔑 Generating OCM master key..."
        MASTER_KEY=$(openssl rand -hex 32)
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s/^OCM_MASTER_KEY=.*/OCM_MASTER_KEY=$MASTER_KEY/" .env
        else
            sed -i "s/^OCM_MASTER_KEY=.*/OCM_MASTER_KEY=$MASTER_KEY/" .env
        fi
        NEEDS_UPDATE=true
    fi
    
    if grep -q "^OPENCLAW_GATEWAY_TOKEN=your-gateway-token-here" .env 2>/dev/null || grep -q "^OPENCLAW_GATEWAY_TOKEN=$" .env 2>/dev/null; then
        echo "🔑 Generating OpenClaw gateway token..."
        GATEWAY_TOKEN=$(openssl rand -hex 32)
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s/^OPENCLAW_GATEWAY_TOKEN=.*/OPENCLAW_GATEWAY_TOKEN=$GATEWAY_TOKEN/" .env
        else
            sed -i "s/^OPENCLAW_GATEWAY_TOKEN=.*/OPENCLAW_GATEWAY_TOKEN=$GATEWAY_TOKEN/" .env
        fi
        NEEDS_UPDATE=true
    fi
    
    if [ "$NEEDS_UPDATE" = true ]; then
        echo "✅ Updated .env with generated keys"
    else
        echo "✅ All keys already configured"
    fi
fi

echo ""
echo "📦 Next steps:"
echo "   1. Run: ./scripts/dev.sh      # Local development"
echo "   2. Or:  ./scripts/docker.sh   # Docker setup"
