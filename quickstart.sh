#!/bin/bash
set -e

echo "🏗️  Building sandbox runtime images..."
make images

echo ""
echo "🚀 Starting Sandkasten daemon with Docker Compose..."
docker-compose up -d

echo ""
echo "⏳ Waiting for daemon to be ready..."
sleep 3

# Health check
if curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "✅ Sandkasten is running at http://localhost:8080"
    echo ""
    echo "📚 Quick test:"
    echo "  # Create a Python session"
    echo "  curl -X POST http://localhost:8080/v1/sessions \\"
    echo "    -H 'Content-Type: application/json' \\"
    echo "    -d '{\"image\": \"sandbox-runtime:python\"}'"
    echo ""
    echo "  # Check logs"
    echo "  docker-compose logs -f"
    echo ""
    echo "  # Stop"
    echo "  docker-compose down"
else
    echo "❌ Daemon failed to start. Check logs:"
    echo "  docker-compose logs"
    exit 1
fi
