#!/usr/bin/env bash
set -euo pipefail

: "${MASTER_HOST:=127.0.0.1}"
: "${MASTER_PORT:=5557}"
: "${AM_LOCUST_JSON:=locust_endpoints.json}"

python3 -m venv .venv
# shellcheck disable=SC1091
source .venv/bin/activate
python -m pip install --upgrade pip
pip install -r requirements.txt

cleanup() { command -v deactivate >/dev/null 2>&1 && deactivate || true; }
trap cleanup EXIT

AM_LOCUST_JSON="$AM_LOCUST_JSON" \
locust -f locustfile.py --worker --master-host "$MASTER_HOST" --master-port "$MASTER_PORT"
