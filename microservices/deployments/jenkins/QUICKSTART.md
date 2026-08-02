# Jenkins CI/CD Quick Reference

## Start Jenkins

```bash
# Start Jenkins + ngrok
docker compose --profile cicd up -d

# Check status
docker compose --profile cicd ps

# Get initial admin password
docker logs gowallet-jenkins 2>&1 | grep -A 2 "Please use the following password"
```

## Access Services

- **Jenkins**: http://localhost:8088
- **ngrok Dashboard**: http://localhost:4040 (after ngrok starts)

## Get ngrok Public URL

```bash
# Method 1: Check logs
docker logs gowallet-ngrok

# Method 2: Via ngrok API
curl -s http://localhost:4040/api/tunnels | grep -o 'https://[^"]*\.ngrok\.io'
```

## Common Commands

```bash
# Restart Jenkins
docker compose --profile cicd restart jenkins

# View Jenkins logs
docker logs -f gowallet-jenkins

# Stop Jenkins + ngrok
docker compose --profile cicd down

# Rebuild Jenkins (after Dockerfile changes)
docker compose --profile cicd build --no-cache jenkins
docker compose --profile cicd up -d
```

## Trigger Build

### Manual Trigger
1. Open Jenkins: http://localhost:8088
2. Click job `gowallet-deploy`
3. Click "Build Now"

### Via Git Push (Webhook)
```bash
git add .
git commit -m "feat: trigger Jenkins build"
git push origin main
```

## Troubleshooting

### Jenkins can't access Docker
```bash
docker compose --profile cicd restart jenkins
```

### ngrok URL changed
```bash
# Get new URL
docker logs gowallet-ngrok

# Update GitHub webhook with new URL
# https://<new-url>.ngrok.io/github-webhook/
```

### Build fails: "go: command not found"
```bash
# Rebuild Jenkins with Go support
docker compose --profile cicd down
docker compose --profile cicd build --no-cache jenkins
docker compose --profile cicd up -d
```

## Pipeline Stages

1. **Checkout Source** - Clone repository
2. **Verify Environment** - Check Go and Docker
3. **Run Tests** - Execute `make test`
4. **Build Linux Binaries** - Run `make build-linux`
5. **Build Docker Images** - Build all service images
6. **Deploy to Staging** - Deploy via `docker compose up -d`

## GitHub Webhook Setup

1. Get ngrok URL: `docker logs gowallet-ngrok`
2. GitHub → Repository → Settings → Webhooks
3. Payload URL: `https://<ngrok-url>/github-webhook/`
4. Content type: `application/json`
5. Events: "Just the push event"
6. Active: ✅
