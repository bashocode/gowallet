#!/bin/bash

# Helper script to get ngrok public URLs for Jenkins Webhook and SonarQube

echo "🔍 Getting ngrok tunnel URLs..."
echo ""

# Check if ngrok container is running
if ! docker ps | grep -q gowallet-ngrok; then
    echo "❌ ngrok container is not running!"
    echo "Start it with: docker compose --profile cicd --profile tools up -d"
    exit 1
fi

# Wait a bit for ngrok to establish tunnels
sleep 2

# Fetch tunnels JSON
TUNNELS_JSON=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null)

JENKINS_URL=$(echo "$TUNNELS_JSON" | grep -o 'https://[^"]*\.ngrok[^"]*' | head -1)
SONAR_URL=$(echo "$TUNNELS_JSON" | grep -o 'https://[^"]*\.ngrok[^"]*' | sed -n '2p')

if [ -z "$JENKINS_URL" ]; then
    echo "⚠️  Could not get ngrok URL from API"
    echo ""
    echo "Try these alternatives:"
    echo "1. Check ngrok logs: docker logs gowallet-ngrok"
    echo "2. Open ngrok dashboard: open http://localhost:4040"
    exit 1
fi

echo "✅ ngrok tunnels are active!"
echo ""
echo "🔗 Jenkins Public URL:   $JENKINS_URL"
echo "🔗 SonarQube Public URL: $SONAR_URL"
echo ""
echo "📋 GitHub Webhook Setup (for Jenkins CD Deployment):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Payload URL: ${JENKINS_URL}/github-webhook/"
echo "Content type: application/json"
echo "Events: Just the push event"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🔑 GitHub Repository Secrets Setup (for PR Check):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "SONAR_HOST_URL = ${SONAR_URL}"
echo "SONAR_TOKEN    = <your-local-sonarqube-token>"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🌐 Jenkins Local Dashboard:   http://localhost:8088"
echo "📊 SonarQube Local Dashboard: http://localhost:9009"
echo "🔗 ngrok Dashboard:           http://localhost:4040"
