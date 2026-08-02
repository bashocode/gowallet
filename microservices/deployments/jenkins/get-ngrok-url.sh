#!/bin/bash

# Helper script to get ngrok public URL for GitHub webhook configuration

echo "🔍 Getting ngrok tunnel URL..."
echo ""

# Check if ngrok container is running
if ! docker ps | grep -q gowallet-ngrok; then
    echo "❌ ngrok container is not running!"
    echo "Start it with: docker compose --profile cicd up -d"
    exit 1
fi

# Wait a bit for ngrok to establish tunnel
sleep 2

# Get URL from ngrok API
NGROK_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o 'https://[^"]*\.ngrok[^"]*' | head -1)

if [ -z "$NGROK_URL" ]; then
    echo "⚠️  Could not get ngrok URL from API"
    echo ""
    echo "Try these alternatives:"
    echo "1. Check ngrok logs: docker logs gowallet-ngrok"
    echo "2. Open ngrok dashboard: open http://localhost:4040"
    exit 1
fi

echo "✅ ngrok tunnel is active!"
echo ""
echo "Public URL: $NGROK_URL"
echo ""
echo "📋 GitHub Webhook Configuration:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Payload URL: ${NGROK_URL}/github-webhook/"
echo "Content type: application/json"
echo "Events: Just the push event"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🌐 Jenkins Dashboard: http://localhost:8088"
echo "🔗 ngrok Dashboard: http://localhost:4040"
