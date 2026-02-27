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
echo "🔧 Applying OCM patches..."
cd "$TMPDIR/openclaw"

# Patch 1: Fix EBUSY error handling on WSL2/Windows
# The atomic rename fails with EBUSY when file watchers hold locks.
# This adds EBUSY to the fallback handling alongside EPERM and EEXIST.
EBUSY_PATCH='
--- a/src/config/io.ts
+++ b/src/config/io.ts
@@ -517,7 +517,7 @@ export function createConfigIO(deps?: Partial<ConfigIODeps>) {
     try {
       await deps.fs.promises.rename(tmp, configPath);
     } catch (err) {
       const code = (err as { code?: string }).code;
-      // Windows doesn'\''t reliably support atomic replace via rename when dest exists.
-      if (code === "EPERM" || code === "EEXIST") {
+      // Windows/WSL2 doesn'\''t reliably support atomic replace via rename when dest exists.
+      // EBUSY occurs when file watchers hold locks on WSL2.
+      if (code === "EPERM" || code === "EEXIST" || code === "EBUSY") {
         await deps.fs.promises.copyFile(tmp, configPath);
         await deps.fs.promises.chmod(configPath, 0o600).catch(() => {
'

# Apply patch (use sed for simple inline edit since patch may not be available)
if grep -q 'code === "EPERM" || code === "EEXIST"' src/config/io.ts; then
    sed -i 's/code === "EPERM" || code === "EEXIST"/code === "EPERM" || code === "EEXIST" || code === "EBUSY"/g' src/config/io.ts
    # Update comment too
    sed -i "s/Windows doesn't reliably support/Windows\/WSL2 doesn't reliably support/g" src/config/io.ts
    echo "   ✓ Applied EBUSY fix patch"
else
    echo "   ⚠ EBUSY patch location not found (may already be fixed upstream)"
fi

echo ""
echo "🐳 Building Docker image..."
docker build -t "$OPENCLAW_IMAGE" .

echo ""
echo "✅ Successfully built: $OPENCLAW_IMAGE"
echo ""
echo "   You can now run: ./scripts/docker.sh"
