#!/bin/bash
# OCM Local Development Script
# Usage: ./scripts/dev.sh

set -e

echo "🚀 OCM Local Development"
echo "========================"

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.22+"
    exit 1
fi

if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 20+"
    exit 1
fi

if ! command -v pnpm &> /dev/null; then
    echo "📦 Installing pnpm..."
    npm install -g pnpm
fi

echo "✅ Prerequisites OK"

# Generate master key if needed
if [ ! -f ~/.ocm/master.key ]; then
    echo ""
    echo "🔑 Generating master key..."
    mkdir -p ~/.ocm
    openssl rand -hex 32 > ~/.ocm/master.key
    chmod 600 ~/.ocm/master.key
    echo "✅ Master key saved to ~/.ocm/master.key"
fi

# Build frontend if needed
if [ ! -d "web/node_modules" ]; then
    echo ""
    echo "📦 Installing frontend dependencies..."
    cd web && pnpm install && cd ..
fi

if [ ! -d "internal/web/build/_app" ] || [ "web/src" -nt "internal/web/build/index.html" ]; then
    echo ""
    echo "🔨 Building frontend..."
    cd web && pnpm build && cd ..
    rm -rf internal/web/build/_app internal/web/build/*.html internal/web/build/*.png 2>/dev/null || true
    cp -r web/build/* internal/web/build/
fi

# Build backend
echo ""
echo "🔨 Building backend..."
go build -o ocm .

echo ""
echo "✅ Build complete!"
echo ""
echo "🚀 Starting OCM..."
echo "   Admin UI: http://localhost:8080"
echo "   Agent API: http://localhost:9999"
echo ""

./ocm serve
