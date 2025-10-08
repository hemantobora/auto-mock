#!/bin/bash
# Config Watcher Sidecar - Polls S3 and updates MockServer automatically
# This runs continuously alongside MockServer container

set -e

# Environment variables (set by ECS task definition)
S3_BUCKET="${S3_BUCKET:-}"
PROJECT_NAME="${PROJECT_NAME:-}"
MOCKSERVER_URL="${MOCKSERVER_URL:-http://localhost:1080}"
POLL_INTERVAL="${POLL_INTERVAL:-30}"
CONFIG_PATH="${CONFIG_PATH:-configs/${PROJECT_NAME}/current.json}"

echo "🔄 Config Watcher Starting"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "S3 Bucket:      ${S3_BUCKET}"
echo "Project:        ${PROJECT_NAME}"
echo "Config Path:    ${CONFIG_PATH}"
echo "MockServer URL: ${MOCKSERVER_URL}"
echo "Poll Interval:  ${POLL_INTERVAL}s"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Validate required variables
if [ -z "$S3_BUCKET" ] || [ -z "$PROJECT_NAME" ]; then
    echo "❌ Error: S3_BUCKET and PROJECT_NAME are required"
    exit 1
fi

# Track last ETag to detect changes
LAST_ETAG=""
UPDATE_COUNT=0
ERROR_COUNT=0

# Wait for MockServer to be ready
echo "⏳ Waiting for MockServer to be ready..."
MAX_WAIT=60
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if curl -s -f "${MOCKSERVER_URL}/mockserver/status" > /dev/null 2>&1; then
        echo "✅ MockServer is ready"
        break
    fi
    sleep 2
    WAITED=$((WAITED + 2))
done

if [ $WAITED -ge $MAX_WAIT ]; then
    echo "⚠️  Warning: MockServer not ready after ${MAX_WAIT}s, continuing anyway..."
fi

# Initial load
echo "📥 Loading initial configuration..."
if aws s3 cp "s3://${S3_BUCKET}/${CONFIG_PATH}" /tmp/current.json 2>/dev/null; then
    EXPECTATIONS=$(cat /tmp/current.json | jq -c '.expectations')
    
    if curl -X PUT "${MOCKSERVER_URL}/mockserver/expectation" \
        -H "Content-Type: application/json" \
        -d "$EXPECTATIONS" \
        -s -w "\nHTTP %{http_code}\n" 2>&1 | grep -q "HTTP 20"; then
        UPDATE_COUNT=$((UPDATE_COUNT + 1))
        echo "✅ Initial expectations loaded successfully"
        LAST_ETAG=$(aws s3api head-object --bucket "${S3_BUCKET}" --key "${CONFIG_PATH}" --query 'ETag' --output text 2>/dev/null || echo "")
    else
        echo "❌ Failed to load initial expectations"
        ERROR_COUNT=$((ERROR_COUNT + 1))
    fi
else
    echo "⚠️  Warning: Could not download initial config from S3"
    ERROR_COUNT=$((ERROR_COUNT + 1))
fi

# Main polling loop
echo ""
echo "🔄 Starting continuous polling (every ${POLL_INTERVAL}s)..."
echo "Press Ctrl+C to stop"
echo ""

while true; do
    sleep "$POLL_INTERVAL"
    
    # Check current ETag
    CURRENT_ETAG=$(aws s3api head-object \
        --bucket "${S3_BUCKET}" \
        --key "${CONFIG_PATH}" \
        --query 'ETag' \
        --output text 2>/dev/null || echo "")
    
    if [ -z "$CURRENT_ETAG" ]; then
        echo "⚠️  [$(date '+%Y-%m-%d %H:%M:%S')] Could not fetch ETag from S3"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        continue
    fi
    
    # Check if file changed
    if [ "$CURRENT_ETAG" = "$LAST_ETAG" ]; then
        echo "✓ [$(date '+%Y-%m-%d %H:%M:%S')] No changes detected (ETag: ${CURRENT_ETAG:0:8}...)"
        continue
    fi
    
    echo "🔔 [$(date '+%Y-%m-%d %H:%M:%S')] Change detected! Updating expectations..."
    
    # Download updated config
    if ! aws s3 cp "s3://${S3_BUCKET}/${CONFIG_PATH}" /tmp/current.json 2>/dev/null; then
        echo "❌ [$(date '+%Y-%m-%d %H:%M:%S')] Failed to download config"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        continue
    fi
    
    # Extract expectations
    EXPECTATIONS=$(cat /tmp/current.json | jq -c '.expectations')
    if [ -z "$EXPECTATIONS" ] || [ "$EXPECTATIONS" = "null" ]; then
        echo "❌ [$(date '+%Y-%m-%d %H:%M:%S')] No expectations found in config"
        ERROR_COUNT=$((ERROR_COUNT + 1))
        continue
    fi
    
    # Count expectations
    EXP_COUNT=$(echo "$EXPECTATIONS" | jq 'length')
    
    # Update MockServer
    HTTP_CODE=$(curl -X PUT "${MOCKSERVER_URL}/mockserver/expectation" \
        -H "Content-Type: application/json" \
        -d "$EXPECTATIONS" \
        -s -w "%{http_code}" \
        -o /tmp/response.json)
    
    if [ "$HTTP_CODE" = "201" ] || [ "$HTTP_CODE" = "200" ]; then
        UPDATE_COUNT=$((UPDATE_COUNT + 1))
        LAST_ETAG="$CURRENT_ETAG"
        echo "✅ [$(date '+%Y-%m-%d %H:%M:%S')] Updated ${EXP_COUNT} expectations (HTTP ${HTTP_CODE})"
        echo "   Total updates: ${UPDATE_COUNT}, Errors: ${ERROR_COUNT}"
    else
        ERROR_COUNT=$((ERROR_COUNT + 1))
        echo "❌ [$(date '+%Y-%m-%d %H:%M:%S')] Failed to update MockServer (HTTP ${HTTP_CODE})"
        if [ -f /tmp/response.json ]; then
            echo "   Response: $(cat /tmp/response.json)"
        fi
    fi
done
