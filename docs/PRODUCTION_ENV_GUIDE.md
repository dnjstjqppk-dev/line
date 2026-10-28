# Production Environment Configuration for LINE Messenger

## Required Environment Variables

### Backend (Go API Server)

```bash
# Database
DATABASE_URL=postgres://messenger:PASSWORD@postgres:5432/messenger?sslmode=require

# Authentication
JWT_SECRET=<32+ byte random string>
MESSAGE_ENCRYPTION_KEY=<32 byte key for AES-256-GCM at-rest encryption>

# Server
PORT=8080
SERVICE_NAME=gateway
ALLOWED_ORIGINS=https://your-domain.com

# Push Notifications - FCM
FCM_PROJECT_ID=your-firebase-project-id
FCM_SERVICE_ACCOUNT_JSON={"type":"service_account","project_id":"...","private_key":"...","client_email":"..."}

# Push Notifications - APNs
APNS_KEY_ID=ABC123DEFG
APNS_TEAM_ID=TEAM123456
APNS_PRIVATE_KEY="-----BEGIN EC PRIVATE KEY-----\n...\n-----END EC PRIVATE KEY-----"
APNS_BUNDLE_ID=com.yourcompany.messenger
APNS_PRODUCTION=true

# Push Token Encryption
PUSH_TOKEN_ENCRYPTION_KEY=<32 byte key>

# Redis (for distributed rate limiting)
REDIS_URL=redis://redis:6379
```

### Frontend (Vue Web App)

```bash
# API
VITE_API_URL=https://api.your-domain.com

# WebRTC
VITE_STUN_SERVERS=stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302
VITE_TURN_SERVER=turn:turn.your-domain.com:3478
VITE_TURN_USERNAME=turnuser
VITE_TURN_PASSWORD=turnpass
```

### Infrastructure

```bash
# PostgreSQL
POSTGRES_DB=messenger
POSTGRES_USER=messenger
POSTGRES_PASSWORD=<strong password>

# Redis
REDIS_PASSWORD=<strong password>

# TURN Server
TURN_USERNAME=turnuser
TURN_PASSWORD=<strong password>
TURN_REALM=your-domain.com
```

## Setup Instructions

### 1. FCM Setup
1. Go to Firebase Console → Project Settings → Service Accounts
2. Click "Generate New Private Key"
3. Save the JSON file
4. Set `FCM_SERVICE_ACCOUNT_JSON` to the entire JSON content
5. Set `FCM_PROJECT_ID` to your Firebase project ID

### 2. APNs Setup
1. Go to Apple Developer Portal → Certificates, Identifiers & Profiles
2. Create an APNs Authentication Key (.p8 file)
3. Note the Key ID and Team ID
4. Set `APNS_KEY_ID`, `APNS_TEAM_ID`, `APNS_PRIVATE_KEY` (contents of .p8 file)
5. Set `APNS_BUNDLE_ID` to your app's bundle identifier
6. Set `APNS_PRODUCTION=true` for production

### 3. Message Encryption Key
Generate a 32-byte key:
```bash
openssl rand -base64 32
```

### 4. JWT Secret
Generate a 32+ byte secret:
```bash
openssl rand -base64 48
```

### 5. Push Token Encryption Key
Generate a 32-byte key:
```bash
openssl rand -base64 32
```

## Docker Compose Production

Use `docker-compose.full.yml` for production deployment:

```bash
# Create .env file with all variables above
cp .env.example .env
# Edit .env with your values
docker-compose -f docker-compose.full.yml up -d
```

## Kubernetes Deployment

Use the manifests in `k8s/` directory:

```bash
# Create secrets
kubectl apply -f k8s/secrets-template.yaml
# Edit secrets with actual values
kubectl apply -f k8s/postgres-statefulset.yaml
kubectl apply -f k8s/backend-deployment.yaml
```

## Health Check

Verify deployment:
```bash
curl https://api.your-domain.com/healthz
# Expected: {"status":"ok","service":"gateway"}
```