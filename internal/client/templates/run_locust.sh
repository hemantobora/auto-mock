#!/usr/bin/env bash
set -euo pipefail

: "${AM_USERS:=20}"
: "${AM_SPAWN_RATE:=5}"
: "${AM_DURATION:=5m}"
: "${AM_LOCUST_JSON:=locust_endpoints.json}"
# Optional: AM_HOST to override host; otherwise set host in UI (non-headless) or pass here.

python3 -m venv .venv
source .venv/bin/activate
python -m pip install --upgrade pip
pip install -r requirements.txt

AM_LOCUST_JSON="$AM_LOCUST_JSON" \
locust -f locustfile.py --headless \
  -u "$AM_USERS" -r "$AM_SPAWN_RATE" --run-time "$AM_DURATION" ${AM_HOST:+--host "$AM_HOST"}
