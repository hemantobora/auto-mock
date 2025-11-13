param(
  [string]$MASTER_HOST = "127.0.0.1",
  [int]$MASTER_PORT = 5557,
  [string]$AM_LOCUST_JSON = "locust_endpoints.json"
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
  locust -f locustfile.py --worker --master-host $MASTER_HOST --master-port $MASTER_PORT
}
finally {
  if (Get-Command deactivate -ErrorAction SilentlyContinue) { deactivate }
}
