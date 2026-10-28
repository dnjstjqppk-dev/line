#!/usr/bin/env bash
# Deployment verification script for LINE messenger
set -e

echo "=== LINE Messenger Deployment Verification ==="

# 1. Backend health check
echo "[1/6] Checking backend health..."
if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
  echo "  ✅ Backend health check passed"
else
  echo "  ❌ Backend health check failed"
  exit 1
fi

# 2. Database connectivity
echo "[2/6] Checking database connectivity..."
if curl -sf http://localhost:8080/readyz > /dev/null 2>&1; then
  echo "  ✅ Database connectivity check passed"
else
  echo "  ❌ Database connectivity check failed"
  exit 1
fi

# 3. Push notification health
echo "[3/6] Checking push notification credentials..."
PUSH_HEALTH=$(curl -sf http://localhost:8080/api/v1/push/health -H "Authorization: Bearer $TOKEN" 2>/dev/null || echo '{"status":"no_auth"}')
echo "  Push health: $PUSH_HEALTH"

# 4. Frontend build
echo "[4/6] Verifying frontend build..."
if [ -d "dist_cf" ]; then
  echo "  ✅ Frontend build directory exists"
else
  echo "  ⚠️ Frontend build directory not found (run npm run build:apps)"
fi

# 5. Docker Compose
echo "[5/6] Checking Docker Compose..."
if command -v docker-compose > /dev/null 2>&1; then
  echo "  ✅ Docker Compose available"
else
  echo "  ⚠️ Docker Compose not found"
fi

# 6. Environment variables
echo "[6/6] Checking environment variables..."
REQUIRED_VARS="DATABASE_URL JWT_SECRET"
for var in $REQUIRED_VARS; do
  if [ -z "${!var}" ]; then
    echo "  ⚠️ $var is not set"
  else
    echo "  ✅ $var is set"
  fi
done

echo ""
echo "=== Verification Complete ==="