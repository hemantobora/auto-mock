
#!/usr/bin/env bash
set -euo pipefail

# ── Bootstrap tools ────────────────────────────────────────────────────────────
if ! command -v curl >/dev/null 2>&1; then yum install -y -q curl || dnf install -y -q curl; fi
if ! command -v jq   >/dev/null 2>&1; then yum install -y -q jq   || dnf install -y -q jq;   fi
if ! command -v aws  >/dev/null 2>&1; then echo "❌ awscli required"; exit 1; fi

# ── Env (from task definition) ────────────────────────────────────────────────
S3_BUCKET="${S3_BUCKET:-}"
PROJECT_NAME="${PROJECT_NAME:-}"
MOCKSERVER_URL="${MOCKSERVER_URL:-http://localhost:1080}"
POLL_INTERVAL="${POLL_INTERVAL:-30}"
CONFIG_PATH="${CONFIG_PATH:-configs/${PROJECT_NAME}/current.json}"

export AWS_REGION="${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-${AWS_REGION:-}}"

echo "🔄 Config Watcher Starting"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "S3 Bucket:      ${S3_BUCKET}"
echo "Project:        ${PROJECT_NAME}"
echo "Config Path:    ${CONFIG_PATH}"
echo "MockServer URL: ${MOCKSERVER_URL}"
echo "Poll Interval:  ${POLL_INTERVAL}s"
echo "AWS Region:     ${AWS_REGION:-<not set>}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -z "${S3_BUCKET}" || -z "${PROJECT_NAME}" ]]; then
  echo "❌ Error: S3_BUCKET and PROJECT_NAME are required"
  exit 1
fi

# ── jq filter written to file (no quoting issues, no $! expansion) ────────────
JQ_FILTER="$(mktemp -t jq_clean.XXXXXX)"
cat >"$JQ_FILTER" <<'JQ'
((.expectations // .) | if type=="array" then . else [] end)
| map(
  del(.description)
  |
  if ((.httpResponse|type)=="object")
     and ( (.httpResponse.body? // null) | tostring | contains("$!") )
  then
    (
      {
        statusCode: (.httpResponse.statusCode // 200),
        headers:    ((.httpResponse.headers // {}) + {"Content-Type":["application/json"]}),
        body:       (.httpResponse.body)
      }
      + ( if ((.httpResponse.delay|type)=="object") then { delay: .httpResponse.delay } else {} end )
    ) as $tmpl
    |
    .httpResponseTemplate = { templateType:"VELOCITY", template: ($tmpl|tojson) }
    | del(.httpResponse)
  else
    .
  end
)
JQ
echo "🧩 jq filter written to: $JQ_FILTER"

# ── Helpers ───────────────────────────────────────────────────────────────────
now() { date "+%Y-%m-%d %H:%M:%S"; }

validate_file() { # $1 = expectations.json
  local exp_file="$1"
  local payload_file
  payload_file="$(mktemp -t validate.XXXXXX).json"
  jq -nc --slurpfile exp "$exp_file" \
    '{ type:"EXPECTATION", value: ($exp[0] | tostring) }' \
    > "$payload_file"
  curl -s -o /tmp/validate.out -w "%{http_code}" \
    -X PUT "${MOCKSERVER_URL}/mockserver/validate" \
    -H "Content-Type: application/json" \
    --data-binary @"$payload_file"
}

load_file() { # $1 = expectations.json
  local exp_file="$1"
  curl -s -o /tmp/load.out -w "%{http_code}" \
    -X PUT "${MOCKSERVER_URL}/mockserver/expectation" \
    -H "Content-Type: application/json" \
    --data-binary @"$exp_file"
}

transform_file() { # in: /tmp/current.json -> out: /tmp/exp.json
  local out="$(mktemp -t expectations.XXXXXX).json"
  jq -c -f "$JQ_FILTER" /tmp/current.json > "$out"
  echo "$out"
}

add_health_check() {
  # ── Seed /health (optional) ───────────────────────────────────────────────────
  curl -s -X PUT "${MOCKSERVER_URL}/mockserver/expectation" \
    -H "Content-Type: application/json" \
    -d '[
      {
        "httpRequest": { "method": "GET", "path": "/health" },
        "httpResponse": { "statusCode": 200, "body": "OK" },
        "priority": 0,
        "times": { "unlimited": true }
      }
    ]' >/dev/null || true
}

# ── Wait for MockServer ───────────────────────────────────────────────────────
echo "⏳ Waiting for MockServer to be ready..."
MAX_WAIT=60; WAITED=0
while [[ $WAITED -lt $MAX_WAIT ]]; do
  status_code="$(curl -s -o /dev/null -w "%{http_code}" "${MOCKSERVER_URL}/" || echo 000)"
  if [[ "$status_code" == "200" || "$status_code" == "404" ]]; then
    echo "✅ MockServer is responding (HTTP $status_code)"
    break
  fi
  echo "   Waiting... ${WAITED}s (got $status_code)"
  sleep 2; WAITED=$((WAITED + 2))
done
if [[ $WAITED -ge $MAX_WAIT ]]; then
  echo "⚠️  Warning: MockServer not ready after ${MAX_WAIT}s, continuing anyway..."
fi

add_health_check
LAST_ETAG=""
UPDATE_COUNT=0
ERROR_COUNT=0

# ── Initial load ──────────────────────────────────────────────────────────────
echo "📥 Loading initial configuration..."
if aws s3 cp "s3://${S3_BUCKET}/${CONFIG_PATH}" /tmp/current.json --only-show-errors 2>/dev/null; then
  jq -e . /tmp/current.json >/dev/null || { echo "❌ raw JSON invalid"; exit 1; }

  EXP_FILE="$(transform_file)"
  jq -e . "$EXP_FILE" >/dev/null || { echo "❌ transformed JSON invalid"; exit 1; }

  EXP_COUNT="$(jq 'length' "$EXP_FILE")"
  echo "🔎 Validating ${EXP_COUNT} expectations..."
  VAL_CODE="$(validate_file "$EXP_FILE")"
  echo "Validate: HTTP ${VAL_CODE}"
  cat /tmp/validate.out || true
  echo

  echo "🧹 Resetting MockServer before loading new expectations..."
  curl -s -X PUT "${MOCKSERVER_URL}/mockserver/reset" >/dev/null || true

  HTTP_CODE="$(load_file "$EXP_FILE")"
  if [[ "$HTTP_CODE" =~ ^20[01]$ ]]; then
    UPDATE_COUNT=$((UPDATE_COUNT + 1))
    echo "✅ Initial expectations loaded (HTTP $HTTP_CODE)"
    LAST_ETAG="$(aws s3api head-object --bucket "${S3_BUCKET}" --key "${CONFIG_PATH}" --query 'ETag' --output text 2>/dev/null || echo "")"
  else
    echo "❌ Failed to load initial expectations (HTTP $HTTP_CODE)"
    cat /tmp/load.out || true; echo
    ERROR_COUNT=$((ERROR_COUNT + 1))
  fi
else
  echo "⚠️  Warning: Could not download initial config from S3"
  ERROR_COUNT=$((ERROR_COUNT + 1))
fi

echo
echo "🔄 Starting continuous polling (every ${POLL_INTERVAL}s)..."
echo "Press Ctrl+C to stop"
echo

# ── Poll loop ────────────────────────────────────────────────────────────────
while true; do
  sleep "${POLL_INTERVAL}"

  CURRENT_ETAG="$(aws s3api head-object --bucket "${S3_BUCKET}" --key "${CONFIG_PATH}" --query 'ETag' --output text 2>/dev/null || echo "")"
  if [[ -z "${CURRENT_ETAG}" || "${CURRENT_ETAG}" == "None" ]]; then
    echo "⚠️  [$(now)] Could not fetch ETag from S3"
    ERROR_COUNT=$((ERROR_COUNT + 1))
    continue
  fi

  if [[ "${CURRENT_ETAG}" == "${LAST_ETAG}" ]]; then
    echo "✓ [$(now)] No changes detected (ETag: ${CURRENT_ETAG:0:8}...)"
    continue
  fi

  echo "🔔 [$(now)] Change detected! Updating expectations..."
  if ! aws s3 cp "s3://${S3_BUCKET}/${CONFIG_PATH}" /tmp/current.json --only-show-errors 2>/dev/null; then
    echo "❌ [$(now)] Failed to download config"
    ERROR_COUNT=$((ERROR_COUNT + 1))
    continue
  fi

  # Validate raw + transform to file
  if ! jq -e . /tmp/current.json >/dev/null; then
    echo "❌ [$(now)] Raw JSON invalid"
    ERROR_COUNT=$((ERROR_COUNT + 1))
    continue
  fi

  EXP_FILE="$(transform_file)"
  if ! jq -e . "$EXP_FILE" >/dev/null; then
    echo "❌ [$(now)] Transformed JSON invalid"
    ERROR_COUNT=$((ERROR_COUNT + 1))
    continue
  fi

  EXP_COUNT="$(jq 'length' "$EXP_FILE")"
  echo "🔎 Validating ${EXP_COUNT} expectations..."
  VAL_CODE="$(validate_file "$EXP_FILE")"
  echo "Validate: HTTP ${VAL_CODE}"
  cat /tmp/validate.out || true
  echo

  echo "🧹 Resetting MockServer before loading new expectations..."
  curl -s -X PUT "${MOCKSERVER_URL}/mockserver/reset" >/dev/null || true

  add_health_check
  HTTP_CODE="$(load_file "$EXP_FILE")"
  if [[ "$HTTP_CODE" =~ ^20[01]$ ]]; then
    UPDATE_COUNT=$((UPDATE_COUNT + 1))
    LAST_ETAG="${CURRENT_ETAG}"
    echo "✅ [$(now)] Updated ${EXP_COUNT} expectations (HTTP ${HTTP_CODE})"
    echo "   Total updates: ${UPDATE_COUNT}, Errors: ${ERROR_COUNT}"
  else
    ERROR_COUNT=$((ERROR_COUNT + 1))
    echo "❌ [$(now)] Failed to update MockServer (HTTP ${HTTP_CODE})"
    cat /tmp/load.out || true; echo
  fi
done