#!/usr/bin/env bash
set -euo pipefail

: "${AM_LOCUST_JSON:=locust_endpoints.json}"
: "${WEB_PORT:=8089}"
# Optional: AM_HOST for master to set target host centrally

# Optional: load environment variables from a local .env file
if [ -f ".env" ]; then
	# shellcheck disable=SC1091
	set +u
	set -a
	. ./.env
	set +a
	set -u
	echo "Loaded environment variables from .env"
fi

python3 -m venv .venv
# shellcheck disable=SC1091
source .venv/bin/activate
python -m pip install --upgrade pip
pip install -r requirements.txt

cleanup() { command -v deactivate >/dev/null 2>&1 && deactivate || true; }
trap cleanup EXIT

AM_LOCUST_JSON="$AM_LOCUST_JSON" \
locust -f locustfile.py --master --web-port "$WEB_PORT" ${AM_HOST:+--host "$AM_HOST"}
