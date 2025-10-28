param(
  [string]$AM_LOCUST_JSON = "locust_endpoints.json",
  [int]$WEB_PORT = 8089
)

python -m venv .venv
. .\.venv\Scripts\Activate.ps1
python -m pip install --upgrade pip
pip install -r requirements.txt

$env:AM_LOCUST_JSON = $AM_LOCUST_JSON
try {
  locust -f locustfile.py --web-port $WEB_PORT
}
finally {
  if (Get-Command deactivate -ErrorAction SilentlyContinue) { deactivate }
}
