# AutoMock Load Test Bundle

This folder contains a ready-to-run Locust bundle.

## Files

- `locustfile.py` — Test runner. Reads `locust_endpoints.json` and optional `user_data.yaml`.
- `locust_endpoints.json` — Endpoints and config (generated). Edit to change target behavior.
- `user_data.yaml` — Optional per-user data. Each Locust user can get its own row.
- `requirements.txt` — Python packages. Install in a virtualenv.
- `run_locust_ui.sh` / `run_locust_headless.sh` — Convenience scripts (Linux/macOS). PowerShell variants on Windows.

## Data parameterization

Placeholders in `locust_endpoints.json` are expanded at runtime and work in path, headers, query params, and body:

- `${data.<field>}` — Use a field from the current user’s data row (from `user_data.yaml`/`csv`/`json`). Works in `path` too, e.g. `/v2/auth/${data.accountNumber}`.
- `${user.id}` / `${user.index}` — This user’s index (0-based).
- `${env.VAR}` — Environment variables.

Environment variables can be provided in two ways:

1) Local runs (UI/headless/master/worker scripts):
  - Create a `.env` file alongside the scripts with lines like:
     
    ```dotenv
    API_TOKEN=abcd1234
    BASE_URL=https://api.example.com
    ```
  - The provided run scripts auto-load `.env` if present. You can also export variables in your shell (macOS/Linux) or set `$Env:VAR` in PowerShell.
  - In `locust_endpoints.json`, reference with `${env.API_TOKEN}` or `${env.BASE_URL}`.

2) Cloud deploy (ECS/Fargate):
  - During `deploy-loadtest` in the REPL, you will be prompted to load a `.env` file or enter KEY=VALUE pairs.
  - These are injected as ECS task environment variables for both master and workers.
  - Note: values are stored in task definitions in plain text. For sensitive, long-lived secrets, consider using a secrets manager in a future iteration.

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

- `auth.mode: shared` — One login at test start; shared token for all users. `${env.*}` expands in auth; `${data.*}` is not available in shared auth.
- `auth.mode: per_user` — Each user logs in once; you can use `${data.username}` and `${data.password}` in login `path`/headers/body.

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

