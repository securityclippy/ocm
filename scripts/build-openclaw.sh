#!/bin/bash
# Build OpenClaw Docker image from source
# Usage: ./scripts/build-openclaw.sh

set -e

OPENCLAW_REPO="https://github.com/openclaw/openclaw.git"
OPENCLAW_BRANCH="${OPENCLAW_BRANCH:-main}"
OPENCLAW_IMAGE="${OPENCLAW_IMAGE:-openclaw:local}"

echo "🔨 Building OpenClaw Docker Image"
echo "=================================="
echo ""
echo "   Repo:   $OPENCLAW_REPO"
echo "   Branch: $OPENCLAW_BRANCH"
echo "   Image:  $OPENCLAW_IMAGE"
echo ""

# Check if image already exists
if docker image inspect "$OPENCLAW_IMAGE" &>/dev/null; then
    echo "⚠️  Image '$OPENCLAW_IMAGE' already exists."
    read -p "   Rebuild? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Skipping build."
        exit 0
    fi
fi

# Create temp directory
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

echo "📥 Cloning OpenClaw..."
git clone --depth 1 --branch "$OPENCLAW_BRANCH" "$OPENCLAW_REPO" "$TMPDIR/openclaw"

echo ""
echo "🐳 Building Docker image..."
cd "$TMPDIR/openclaw"
docker build -t "$OPENCLAW_IMAGE" .

echo ""
echo "✅ Successfully built: $OPENCLAW_IMAGE"
echo ""
echo "   You can now run: ./scripts/docker.sh"
