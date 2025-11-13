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

# Optional: load environment variables from a local .env file
if (Test-Path ".env") {
  Get-Content .env | ForEach-Object {
    if ($_ -match '^\s*#') { return }
    if ($_ -match '^\s*$') { return }
    $parts = $_ -split '=', 2
    if ($parts.Length -eq 2) {
      $key = $parts[0].Trim()
      $val = $parts[1].Trim().Trim('"\'')
      Set-Item -Path Env:$key -Value $val
    }
  }
  Write-Host "Loaded environment variables from .env"
}

$env:AM_LOCUST_JSON = $AM_LOCUST_JSON
try {
  if ($AM_HOST -ne "") {
    locust -f locustfile.py --headless -u $AM_USERS -r $AM_SPAWN_RATE --run-time $AM_DURATION --host $AM_HOST
  } else {
    locust -f locustfile.py --headless -u $AM_USERS -r $AM_SPAWN_RATE --run-time $AM_DURATION
  }
}
finally {
  if (Get-Command deactivate -ErrorAction SilentlyContinue) { deactivate }
}
