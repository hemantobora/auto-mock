# AutoMock Load Test Bundle

This folder contains a ready-to-run Locust bundle.

## Files

- `locustfile.py` — Test runner. Reads `locust_endpoints.json` and optional `user_data.yaml`.
- `locust_endpoints.json` — Endpoints and config (generated). Edit to change target behavior.
- `user_data.yaml` — Optional per-user data. Each Locust user can get its own row.
- `requirements.txt` — Python packages. Install in a virtualenv.
- `run_locust_ui.sh` / `run_locust_headless.sh` — Convenience scripts (Linux/macOS). PowerShell variants on Windows.

## Data parameterization

Placeholders in `locust_endpoints.json` are expanded at runtime:

- `${data.<field>}` — Use a field from the current user’s data row (from `user_data.yaml`/`csv`/`json`).
- `${user.id}` / `${user.index}` — This user’s index (0-based).
- `${env.VAR}` — Environment variables.

Example (in `locust_endpoints.json`):
```json
{
  "endpoints": [
    {
      "name": "Get balance",
      "method": "GET",
      "path": "/accounts/${data.account_number}/balances",
      "params": { "account": "${data.account_number}" },
      "headers": { "X-User": "${data.username}" }
    }
  ]
}
```

## Auth

- `auth.mode: shared` — One login at test start; shared token for all users (no user-data expansion in auth).
- `auth.mode: per_user` — Each user logs in once; you can use `${data.username}` and `${data.password}` in login headers/body.

## User data file

By default, Locust will auto-detect a file named `user_data.yaml` in this folder. You can also use `user_data.csv` or `user_data.json`.
Alternatively, set `AM_USER_DATA=/path/to/file` to override.

CSV header becomes field names; YAML/JSON should be a list of objects.

## Data assignment options

Configure in `locust_endpoints.json` under the `config` block:

- `data_assignment`: "round_robin" | "shared" | "random" (default: "round_robin")
- `data_shared_index`: integer (row index to use when `data_assignment` = "shared", default: 0)

Examples:
```json
{
  "config": {
    "data_assignment": "shared",
    "data_shared_index": 0
  }
}
```

- round_robin: Users cycle through rows 0..N-1.
- shared: All users use the same row (good when you do not need per-user separation).
- random: Each user picks a random row.

## Running

- UI mode (macOS/Linux):
  ```bash
  # optional: export AM_HOST to set base URL; otherwise set host in Locust UI
  export AM_HOST="https://api.example.com"
  # optional: override data file
  # export AM_USER_DATA="./user_data.yaml"
  ./run_locust_ui.sh
  ```

- Headless mode:
  ```bash
  export AM_HOST="https://api.example.com"
  ./run_locust_headless.sh
  ```

Windows has PowerShell equivalents.

