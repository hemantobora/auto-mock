param(
  [string]$AM_LOCUST_JSON = "locust_endpoints.json",
  [int]$WEB_PORT = 8089,
  [string]$AM_HOST = ""
)

python -m venv .venv
. .\.venv\Scripts\Activate.ps1
python -m pip install --upgrade pip
pip install -r requirements.txt

$env:AM_LOCUST_JSON = $AM_LOCUST_JSON
try {
  if ($AM_HOST -ne "") {
    locust -f locustfile.py --master --web-port $WEB_PORT --host $AM_HOST
  } else {
    locust -f locustfile.py --master --web-port $WEB_PORT
  }
}
finally {
  if (Get-Command deactivate -ErrorAction SilentlyContinue) { deactivate }
}
