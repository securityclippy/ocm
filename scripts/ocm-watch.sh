#!/bin/bash
# ocm-watch.sh - Auto-rebuild OCM on git changes
#
# Usage: ./scripts/ocm-watch.sh
#
# Polls git for changes and rebuilds only the OCM container when
# new commits are pulled. OpenClaw container stays running.

set -e

COMPOSE_FILE="docker-compose.openclaw.yml"
POLL_INTERVAL="${POLL_INTERVAL:-10}"  # seconds, configurable via env

echo "Watching for OCM changes (polling every ${POLL_INTERVAL}s)..."
echo "Press Ctrl+C to stop."

while true; do
    # Get current commit before pull
    OLD_HEAD=$(git rev-parse HEAD)
    
    # Pull changes
    git pull --quiet
    
    # Get new commit after pull
    NEW_HEAD=$(git rev-parse HEAD)
    
    # Only rebuild if there are changes
    if [ "$OLD_HEAD" != "$NEW_HEAD" ]; then
        echo ""
        echo "$(date): Changes detected"
        echo "  $OLD_HEAD → $NEW_HEAD"
        echo ""
        git log --oneline "${OLD_HEAD}..${NEW_HEAD}"
        echo ""
        echo "Rebuilding OCM..."
        
        # Stop only ocm, rebuild, and start it back up
        docker compose -f "$COMPOSE_FILE" stop ocm
        docker compose -f "$COMPOSE_FILE" build ocm
        docker compose -f "$COMPOSE_FILE" up -d ocm
        
        echo ""
        echo "✓ OCM rebuilt and restarted."
        echo ""
    fi
    
    sleep "$POLL_INTERVAL"
done
