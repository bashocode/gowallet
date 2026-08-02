# Jenkins CI/CD Setup for GoWallet Microservices

This tutorial explains how to set up self-hosted Jenkins for building and deploying GoWallet microservices locally.

## 🎯 Scenario: Hybrid Enterprise Architecture

```
[ Developer Push PR ]
         │
         ▼
 1. GitHub Actions ──► Run `make test`, Linting (Cloud PR Check)
         │ (PR Approved & Merged)
         ▼
 2. Webhook Event Trigger
         ▼
 3. Jenkins (Self-Hosted) ──► Checkout ──► Build Docker Images ──► Deploy
```

## 📋 Prerequisites

- Docker and Docker Compose installed
- ngrok account (to expose Jenkins to the internet for webhooks)
- GoWallet repository cloned

## 🚀 Step 1: Start Jenkins & ngrok

### 1.1 Setup ngrok Token

Register at [ngrok.com](https://ngrok.com) and obtain an authtoken, then update it in `.env`:

```bash
NGROK_AUTHTOKEN=your_actual_ngrok_token_here
```

### 1.2 Start Jenkins with Docker Compose

```bash
# Start Jenkins and ngrok with the cicd profile
docker compose --profile cicd up -d

# Or with the tools profile (includes all monitoring tools)
docker compose --profile tools up -d
```

### 1.3 Get Initial Admin Password

```bash
docker logs gowallet-jenkins 2>&1 | grep -A 2 "Please use the following password"
```

Copy the password that appears.

### 1.4 Setup Jenkins

1. Open your browser and navigate to `http://localhost:8088`
2. Paste the initial admin password
3. Install suggested plugins
4. Create an admin user
5. Finish setup

## 🔧 Step 2: Create Jenkins Pipeline Job

### 2.1 Create New Pipeline

1. In the Jenkins dashboard, click **"New Item"**
2. Name: `gowallet-deploy`
3. Type: **Pipeline**
4. Click **OK**

### 2.2 Configure Pipeline

On the job configuration page:

#### General Section
- ✅ Check **"GitHub project"**
- Project url: `https://github.com/your-username/gowallet`

#### Build Triggers
- ✅ Check **"GitHub hook trigger for GITScm polling"**

#### Pipeline Section
- Definition: **Pipeline script from SCM**
- SCM: **Git**
- Repository URL: `https://github.com/your-username/gowallet.git`
- Branch: `*/main` (or your desired branch)
- Script Path: `microservices/Jenkinsfile`

Click **Save**.

## 🌐 Step 3: Setup ngrok Tunnel

### 3.1 Get ngrok Public URL

```bash
# Check ngrok logs to get the public URL
docker logs gowallet-ngrok

# Or open the ngrok web interface
open http://localhost:4040
```

Copy the public URL provided by ngrok (e.g., `https://abc123.ngrok.io`).

### 3.2 Setup GitHub Webhook

1. Open your GitHub repository: **Settings** → **Webhooks** → **Add webhook**
2. Payload URL: `https://abc123.ngrok.io/github-webhook/`
3. Content type: `application/json`
4. Which events: **Just the push event**
5. ✅ Active
6. Click **Add webhook**

## 🧪 Step 4: Test Pipeline

### 4.1 Manual Trigger (Initial Test)

1. Open the `gowallet-deploy` job in Jenkins
2. Click **"Build Now"**
3. Monitor console output

### 4.2 Automatic Trigger via Git Push

```bash
# Make a change
echo "test" > test.txt
git add test.txt
git commit -m "test: trigger Jenkins pipeline"
git push origin main
```

Jenkins will automatically trigger a build via webhook.

## 📁 Pipeline Stages Explanation

`Jenkinsfile` executes the following stages:

1. **Checkout Source** - Clone repository
2. **Verify Environment** - Check if Go and Docker are available
3. **Run Tests** - Run `make test`
4. **Build Linux Binaries** - Compile all services with `make build-linux`
5. **Build Docker Images** - Build Docker image for each service
6. **Deploy to Staging** - Deploy via `docker compose up -d --build`

## 🛠️ Troubleshooting

### Jenkins cannot access Docker

If a "permission denied" error appears when accessing the Docker socket:

```bash
# Restart Jenkins container
docker compose --profile cicd restart jenkins
```

### ngrok tunnel expired

The ngrok free tier provides a random URL that changes on every restart. For a persistent domain, upgrade to an ngrok paid plan or restart ngrok and update the webhook URL in GitHub.

```bash
# Restart ngrok
docker compose --profile cicd restart ngrok

# Check new URL
docker logs gowallet-ngrok
```

### Build failed: "go: command not found"

Ensure the Jenkins container has been rebuilt with the custom Dockerfile:

```bash
docker compose --profile cicd down
docker compose --profile cicd build --no-cache jenkins
docker compose --profile cicd up -d
```

## 🔐 Security Notes

- Jenkins runs on `127.0.0.1:8088` (accessible only from localhost)
- ngrok exposes Jenkins to the internet for webhooks (use ngrok auth for production)
- Docker socket is mounted (`/var/run/docker.sock`) - Jenkins can control the Docker host
- For production, use a Jenkins agent/slave architecture with proper network isolation

## 📊 Monitoring

### Jenkins Logs

```bash
docker logs -f gowallet-jenkins
```

### ngrok Web Interface

```bash
open http://localhost:4040
```

Displays:
- Request history
- ngrok tunnel status
- Public URL

## 🧹 Cleanup

```bash
# Stop Jenkins and ngrok
docker compose --profile cicd down

# Remove volumes (delete Jenkins data)
docker compose --profile cicd down -v
```

## 📚 Related Files

- `docker-compose.yml` - Jenkins & ngrok service definitions
- `deployments/jenkins/Dockerfile` - Custom Jenkins image with Docker + Go
- `Jenkinsfile` - Declarative pipeline script
- `.env` - Environment variables for ports and tokens
