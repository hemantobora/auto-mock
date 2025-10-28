param(
  [int]$AM_USERS = 20,
  [int]$AM_SPAWN_RATE = 5,
  [string]$AM_DURATION = "5m",
  [string]$AM_LOCUST_JSON = "locust_endpoints.json",
  [string]$AM_HOST = ""
)

python -m venv .venv
. .\.venv\Scripts\Activate.ps1
python -m pip install --upgrade pip
pip install -r requirements.txt

$env:AM_LOCUST_JSON = $AM_LOCUST_JSON
if ($AM_HOST -ne "") {
  locust -f locustfile.py --headless -u $AM_USERS -r $AM_SPAWN_RATE --run-time $AM_DURATION --host $AM_HOST
} else {
  locust -f locustfile.py --headless -u $AM_USERS -r $AM_SPAWN_RATE --run-time $AM_DURATION
}
