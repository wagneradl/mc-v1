#!/bin/bash
# Memory Cloud — Update Script v1.1
# Usage: ./update.sh [--rebuild]
#   No args:   pull latest base images and restart
#   --rebuild: also rebuild mcp-proxy image (after code changes)

set -e
cd /opt/mcp-hub

echo "$(date) Starting update..."

# Pull latest code from GitHub
echo "Pulling latest code from GitHub..."
git pull origin main

if [ "$1" = "--rebuild" ]; then
    echo "Pulling base images + rebuilding mcp-proxy..."
    docker compose build --pull --no-cache mcp-proxy
else
    echo "Pulling latest images..."
    docker compose pull caddy
fi

echo "Restarting services..."
docker compose up -d

echo "Waiting for services to start..."
sleep 15

echo ""
echo "=== Service Status ==="
docker compose ps

echo ""
echo "=== MCP Proxy Health ==="
docker compose logs mcp-proxy --tail=5

echo ""
echo "$(date) Update complete!"
