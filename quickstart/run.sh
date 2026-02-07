#!/bin/bash
set -e

echo "🏗️  Sandkasten Quickstart"
echo ""

# Check if images exist
if ! docker images | grep -q "sandbox-runtime"; then
    echo "❌ Sandbox images not found. Building..."
    echo ""
    cd ..
    make images
    cd quickstart
    echo ""
fi

echo "🚀 Starting Sandkasten daemon..."
cd daemon
docker-compose up -d
cd ..

echo ""
echo "⏳ Waiting for daemon to start..."
sleep 3

# Health check
if ! curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "❌ Daemon failed to start. Check logs:"
    echo "  cd daemon && docker-compose logs"
    exit 1
fi

echo "✅ Daemon ready at http://localhost:8080"
echo ""
echo "📚 Next steps:"
echo ""
echo "  1. Set your OpenAI API key:"
echo "     export OPENAI_API_KEY='sk-...'"
echo ""
echo "  2. Run the enhanced interactive agent (recommended):"
echo "     cd agent"
echo "     uv sync"
echo "     uv run enhanced_agent.py"
echo ""
echo "  3. Or try other examples:"
echo "     uv run coding_agent.py        # Simple Fibonacci example"
echo "     uv run interactive_agent.py   # Basic interactive mode"
echo ""
echo "  4. When done, stop the daemon:"
echo "     cd ../daemon && docker-compose down"
echo ""
