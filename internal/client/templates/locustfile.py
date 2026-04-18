import os, json, re, random, csv, threading, uuid, string, time
from typing import Any, Dict, List, Optional
from urllib.parse import parse_qs
from locust import HttpUser, SequentialTaskSet, between, constant, events

# -------------------------------------------------------------------
# Config / Spec Loading
# -------------------------------------------------------------------

JSON_PATH = os.getenv("AM_LOCUST_JSON", "locust_endpoints.json")
HOST_ENV  = os.getenv("AM_HOST")  # Optional. If not set, host can be set in Locust UI.

_env_re = re.compile(r"\$\{env\.([A-Za-z_][A-Za-z0-9_]*)\}")
_data_re = re.compile(r"\$\{data\.([A-Za-z_][A-Za-z0-9_]*)\}")
_user_re = re.compile(r"\$\{user\.(id|index)\}")

def _expand_env(v: Any):
    if isinstance(v, str):
        return _env_re.sub(lambda m: os.getenv(m.group(1), ""), v)
    if isinstance(v, dict):
        return {k: _expand_env(x) for k, x in v.items()}
    if isinstance(v, list):
        return [_expand_env(x) for x in v]
    return v

with open(JSON_PATH, "r", encoding="utf-8") as f:
    SPEC = _expand_env(json.load(f))

AUTH    = SPEC.get("auth") or {"mode": "none"}
CFG     = SPEC.get("config") or {}
EPS     = SPEC.get("endpoints") or []
FLOWS   = SPEC.get("flows") or []
COHORTS = SPEC.get("cohorts") or []

# Data assignment config
DATA_ASSIGNMENT: str = (CFG.get("data_assignment") or "round_robin").lower()  # shared | round_robin | random
try:
    DATA_SHARED_INDEX: int = int(CFG.get("data_shared_index", 0))
except Exception:
    DATA_SHARED_INDEX = 0

# -------------------------------------------------------------------
# Optional per-user data (CSV/JSON) for parameterization
# -------------------------------------------------------------------

USER_DATA: List[Dict[str, Any]] = []
_DATA_LOCK = threading.Lock()
_USER_COUNTER = 0

def _load_user_data(base_dir: Optional[str] = None):
    global USER_DATA
    path = os.getenv("AM_USER_DATA")
    if not path and base_dir:
        # Auto-detect files next to the JSON spec
        for name in ["user_data.yaml", "user_data.yml", "user_data.csv", "user_data.json"]:
            candidate = os.path.join(base_dir, name)
            if os.path.exists(candidate):
                path = candidate
                break
    if not path or not os.path.exists(path):
        if path:
            print(f"[data] User data file not found: {path}")
        return
    try:
        if path.lower().endswith(".csv"):
            with open(path, newline="", encoding="utf-8") as f:
                reader = csv.DictReader(f)
                USER_DATA = [row for row in reader]
        elif path.lower().endswith(".yaml") or path.lower().endswith(".yml"):
            try:
                import yaml  # type: ignore
            except Exception as e:
                print("[data] PyYAML not installed; add 'pyyaml' to requirements.txt or use CSV/JSON")
                USER_DATA = []
            else:
                with open(path, "r", encoding="utf-8") as yf:
                    data = yaml.safe_load(yf) or []
                    if isinstance(data, list):
                        USER_DATA = data
                    else:
                        print("[data] YAML must be a list of objects")
                        USER_DATA = []
        else:
            # JSON array or NDJSON
            with open(path, "r", encoding="utf-8") as f:
                txt = f.read().strip()
                if not txt:
                    return
                if txt[0] == "[":
                    USER_DATA = json.loads(txt)
                else:
                    USER_DATA = [json.loads(line) for line in txt.splitlines() if line.strip()]
        if USER_DATA:
            print(f"[data] Loaded {len(USER_DATA)} user data rows from {path}")
    except Exception as e:
        print(f"[data] Failed to load AM_USER_DATA: {e}")

# Initialize user data with auto-discovery next to the spec JSON
_load_user_data(os.path.dirname(JSON_PATH) or ".")

def _expand_runtime(v: Any, ctx: Dict[str, Any]):
    if isinstance(v, str):
        s = v
        s = _data_re.sub(lambda m: str((ctx.get("data") or {}).get(m.group(1), "")), s)
        s = _user_re.sub(lambda m: str((ctx.get("user") or {}).get(m.group(1), "")), s)
        return s
    if isinstance(v, dict):
        return {k: _expand_runtime(x, ctx) for k, x in v.items()}
    if isinstance(v, list):
        return [_expand_runtime(x, ctx) for x in v]
    return v

_GEN_RE = re.compile(r'^!(uuid|digits|alpha|alphanum|hex)(?::(\d+))?(?::([A-Za-z0-9]+))?$')

def _resolve_generators(row: Dict[str, Any]) -> Dict[str, Any]:
    """Replace generator placeholders in a user data row with freshly generated values.

    Called once per virtual user at on_start, so each user gets their own stable
    generated values that remain consistent for the duration of their session.

    Supported syntax (values in user_data.yaml):
      !uuid              → random UUID4           e.g. f47ac10b-58cc-4372-a567-0e02b2c3d479
      !digits:N          → N random digits        e.g. !digits:10  → 3847201938
      !digits:N:PREFIX   → N digits, starts with PREFIX
                                                  e.g. !digits:10:80 → 8034729183
      !alpha:N           → N random letters       e.g. !alpha:8   → kjhTpwQz
      !alphanum:N        → N random alphanumeric  e.g. !alphanum:12 → aB3kP9xQr2Tz
      !alphanum:N:PREFIX → N alphanumeric, starts with PREFIX
                                                  e.g. !alphanum:8:USR → USRaB3kP
      !hex:N             → N random hex chars     e.g. !hex:16 → 3f9a2b8c1d4e7f0a
    """
    if not row:
        return row
    out = {}
    for k, v in row.items():
        if isinstance(v, str):
            m = _GEN_RE.match(v.strip())
            if m:
                kind   = m.group(1)
                n      = int(m.group(2)) if m.group(2) else None
                prefix = m.group(3) or ""
                if kind == "uuid":
                    v = str(uuid.uuid4())
                elif kind == "digits" and n:
                    remaining = max(0, n - len(prefix))
                    v = (prefix + "".join(random.choices("0123456789", k=remaining)))[:n]
                elif kind == "alpha" and n:
                    remaining = max(0, n - len(prefix))
                    v = (prefix + "".join(random.choices(string.ascii_letters, k=remaining)))[:n]
                elif kind == "alphanum" and n:
                    remaining = max(0, n - len(prefix))
                    v = (prefix + "".join(random.choices(string.ascii_letters + string.digits, k=remaining)))[:n]
                elif kind == "hex" and n:
                    remaining = max(0, n - len(prefix))
                    v = (prefix + "".join(random.choices("0123456789abcdef", k=remaining)))[:n]
        out[k] = v
    return out

def _claim_user_index() -> int:
    global _USER_COUNTER
    with _DATA_LOCK:
        idx = _USER_COUNTER
        _USER_COUNTER += 1
        return idx

def _assign_user_data(user_index: int):
    if not USER_DATA:
        return None
    if DATA_ASSIGNMENT == "shared":
        return USER_DATA[DATA_SHARED_INDEX % len(USER_DATA)]
    if DATA_ASSIGNMENT == "random":
        return random.choice(USER_DATA)
    # round_robin (default)
    return USER_DATA[user_index % len(USER_DATA)]

# -------------------------------------------------------------------
# Config defaults & helpers
# -------------------------------------------------------------------

def _cfg_bool(name: str, default: bool) -> bool:
    v = CFG.get(name, default)
    return bool(v)

def _cfg_float(name: str, default: float) -> float:
    v = CFG.get(name, default)
    try:
        return float(v)
    except Exception:
        return default

def _cfg_int(name: str, default: int) -> int:
    v = CFG.get(name, default)
    try:
        return int(v)
    except Exception:
        return default

def _cfg_list(name: str) -> List[str]:
    v = CFG.get(name, [])
    if isinstance(v, list):
        return [str(x) for x in v]
    if isinstance(v, str) and v.strip():
        return [s.strip() for s in v.split(",")]
    return []

DEFAULT_HEADERS: Dict[str, str] = CFG.get("default_headers") or {}
DEFAULT_PARAMS: Dict[str, str]  = CFG.get("default_params")  or {}

WAIT_STRATEGY: str = (CFG.get("wait_strategy") or "between").lower()  # "between" | "constant" | "random_exp"
MIN_WAIT = _cfg_float("min_wait_seconds", 0.2)
MAX_WAIT = _cfg_float("max_wait_seconds", 1.0)
CONST_WAIT = _cfg_float("constant_wait_seconds", 1.0)

REQUEST_TIMEOUT = _cfg_float("request_timeout_seconds", 30.0)
VERIFY_TLS = _cfg_bool("verify_tls", True)

INCLUDE_TAGS = set(_cfg_list("include_tags"))
EXCLUDE_TAGS = set(_cfg_list("exclude_tags"))

# -------------------------------------------------------------------
# Auth helpers
# -------------------------------------------------------------------

_SHARED_TOKEN: Optional[str] = None
_SHARED_TOKEN_LOCK = threading.Lock()

def _json_get(d: Any, path: str, default=None):
    cur = d
    if not path:
        return default
    for part in path.split("."):
        if not isinstance(cur, dict) or part not in cur:
            return default
        cur = cur[part]
    return cur

def _do_auth_from_spec(client, auth_spec: Dict[str, Any], ctx: Optional[Dict[str, Any]] = None) -> Optional[str]:
    """Fetch an auth token using an arbitrary auth spec dict.

    Used for both the global AUTH config and per-flow auth overrides.
    Returns the extracted token string, or None on failure.
    """
    method  = (auth_spec.get("method") or "POST").upper()
    path    = auth_spec.get("path") or "/"
    headers = auth_spec.get("headers") or {}
    body    = auth_spec.get("body")
    if ctx is not None:
        headers = _expand_runtime(headers, ctx)
        body    = _expand_runtime(body, ctx)
        path    = _expand_runtime(path, ctx)

    kwargs = {"headers": headers, "timeout": REQUEST_TIMEOUT, "verify": VERIFY_TLS}
    if body:
        kwargs["json" if isinstance(body, (dict, list)) else "data"] = body

    r = client.request(method, path, name="AUTH "+path, **kwargs)
    if r.status_code >= 400:
        print(f"[auth] failed: HTTP {r.status_code} - {r.text[:200]}")
        return None

    token_path = auth_spec.get("token_json_path", "access_token")
    token = None
    try:
        data = r.json()
        token = _json_get(data, token_path)
    except Exception:
        # Fallback: try URL-encoded body
        try:
            kv = parse_qs(r.text or "")
            token_list = kv.get(token_path, []) or kv.get("access_token", [])
            token = token_list[0] if token_list else None
        except Exception:
            token = None

    if not token:
        print(f"[auth] token not found at path '{token_path}'. Raw body (truncated): {r.text[:200]}")
        return None
    return token


def _do_auth(client, ctx: Optional[Dict[str, Any]] = None) -> Optional[str]:
    """Fetch a token using the global AUTH spec. Returns None when mode is 'none'."""
    mode = (AUTH.get("mode") or "none").lower()
    if mode == "none":
        return None
    return _do_auth_from_spec(client, AUTH, ctx)

@events.test_start.add_listener
def _on_test_start(environment, **_):
    global _SHARED_TOKEN
    if (AUTH.get("mode") or "none").lower() == "shared":
        # Prefer newer API; fallback for older Locust versions
        base_host = HOST_ENV or getattr(environment, "host", None)
        if not base_host:
            # Host not provided yet (e.g., user will set it in UI). Defer shared auth to user on_start.
            print("[auth] Host not set at test start; will initialize shared token lazily when users start.")
            return
        client = None
        # Attempt context-based client only; if unavailable, defer to lazy init without emitting errors.
        try:
            ctx = environment.create_local_http_context(base_host)
            client = ctx.client
        except Exception:
            # Silent defer; on_start will acquire token.
            return

        if client is not None:
            client.verify = VERIFY_TLS
            # Pass the shared data row so ${data.*} placeholders in auth headers/path/body
            # are substituted before the request is sent.
            shared_data = USER_DATA[DATA_SHARED_INDEX % len(USER_DATA)] if USER_DATA else {}
            shared_ctx = {"data": shared_data, "user": {"id": 0, "index": 0}}
            _SHARED_TOKEN = _do_auth(client, shared_ctx)
        if _SHARED_TOKEN:
            print("🔐 Auth OK (shared token)")

# -------------------------------------------------------------------
# Wait-time strategies
# -------------------------------------------------------------------

def _random_exp_wait():
    # Exponential-like wait with mean roughly between MIN and MAX; cap at MAX
    # Lambda chosen so that ~63% values are under (MAX - MIN)
    span = max(MAX_WAIT - MIN_WAIT, 0.001)
    val = random.expovariate(1.0 / (span / 1.5))  # tune as desired
    return min(MIN_WAIT + val, MAX_WAIT)

def _select_wait_strategy():
    if WAIT_STRATEGY == "constant":
        return constant(CONST_WAIT)
    if WAIT_STRATEGY == "random_exp":
        # emulate via custom function wrapper
        class _Exp:
            def __call__(self):
                return _random_exp_wait()
        return _Exp()
    # default
    return between(MIN_WAIT, MAX_WAIT)

# -------------------------------------------------------------------
# Task model
# -------------------------------------------------------------------

def _should_include(endpoint: Dict[str, Any]) -> bool:
    tags = set(endpoint.get("tags") or [])
    if INCLUDE_TAGS and not (tags & INCLUDE_TAGS):
        return False
    if EXCLUDE_TAGS and (tags & EXCLUDE_TAGS):
        return False
    return True

def _make_flow_taskset(flow: Dict[str, Any]):
    """Dynamically build a SequentialTaskSet for a flow definition.

    Each step calls self.user._do(endpoint) so auth tokens, user data, and
    request logic are identical to flat endpoints.  After the last step the
    TaskSet calls self.interrupt() so the virtual user returns to the top-level
    task pool and the next task (flow or flat endpoint) is chosen by weight.

    Flow-level auth override:
      If the flow defines its own "auth" block, the flow fetches a fresh token
      at the start of every iteration and injects it into all steps, overriding
      the global auth token for the duration of that flow run only.

      Example:
        {
          "name": "BYO Flow",
          "auth": {
            "method": "GET",
            "path": "https://auth.example.com/token/${data.account_id}",
            "token_json_path": "serviceAccessToken",
            "header_name": "Authorization",
            "header_prefix": "Bearer "
          },
          "steps": [ ... ]
        }

    Wait behaviour:
      - wait_time = constant(0) is set on the TaskSet so Locust does not add
        its own inter-step pause automatically.
      - Steps WITHOUT delay_ms: the user-level wait strategy fires after the
        step (replicates the normal between-task pause).
      - Steps WITH delay_ms: that fixed sleep replaces the user-level wait for
        that step only.  Useful for polling or async-init scenarios.
    """
    flow_name  = flow.get("name", "UnnamedFlow")
    steps      = flow.get("steps") or []
    flow_auth  = flow.get("auth") or None   # optional per-flow auth spec

    step_fns: List[Any] = []

    # ── Optional flow-level auth step (runs first, every iteration) ──────────
    if flow_auth:
        def _flow_auth_step(self):
            ctx = {
                "data": self.user._data or {},
                "user": {"id": self.user._user_index, "index": self.user._user_index},
            }
            self._flow_token = _do_auth_from_spec(self.user.client, flow_auth, ctx)
        _flow_auth_step.__name__ = "auth_" + re.sub(r"[^A-Za-z0-9_]+", "_", flow_name)[:50]
        step_fns.append(_flow_auth_step)

    # ── Regular steps ─────────────────────────────────────────────────────────
    for ep in steps:
        delay_ms = ep.get("delay_ms")

        def _make_step(endpoint: Dict[str, Any], post_delay, auth_spec):
            def _step(self):
                # Pass flow token when this flow has its own auth; otherwise None
                # falls back to the global token inside _do / _apply_token.
                ft   = getattr(self, "_flow_token", None) if auth_spec else None
                self.user._do(endpoint, flow_token=ft, flow_auth_spec=auth_spec)
                if post_delay is not None:
                    time.sleep(post_delay / 1000.0)
                else:
                    # Mirror the user-level wait strategy between steps
                    secs = self.user.wait_time()
                    if secs > 0:
                        time.sleep(secs)
            nm = endpoint.get("name") or f"{endpoint.get('method','GET')} {endpoint.get('path','/')}"
            _step.__name__ = "step_" + re.sub(r"[^A-Za-z0-9_]+", "_", nm)[:60]
            return _step

        step_fns.append(_make_step(ep, delay_ms, flow_auth))

    # ── Final pseudo-step: return to the top-level task pool ─────────────────
    def _done(self):
        self.interrupt()
    _done.__name__ = "_done_" + re.sub(r"[^A-Za-z0-9_]+", "_", flow_name)[:40]
    step_fns.append(_done)

    cls_name = "Flow_" + re.sub(r"[^A-Za-z0-9_]+", "_", flow_name)[:60]
    return type(cls_name, (SequentialTaskSet,), {
        "tasks":     step_fns,
        "wait_time": constant(0),   # inter-step waits handled manually above
    })


class AutoMockUser(HttpUser):
    wait_time = _select_wait_strategy()
    if HOST_ENV:
        host = HOST_ENV  # otherwise set in UI

    def on_start(self):
        # Set per-session TLS verification
        self.client.verify = VERIFY_TLS

        # Per-user auth
        self._token = None
        # Assign deterministic user index and optional data row
        self._user_index = _claim_user_index()
        self._data = _resolve_generators(dict(_assign_user_data(self._user_index) or {}))
        if (AUTH.get("mode") or "none").lower() == "per_user":
            ctx = {"data": self._data or {}, "user": {"id": self._user_index, "index": self._user_index}}
            self._token = _do_auth(self.client, ctx)

        # Lazy init for shared auth if host wasn't available at test_start
        if (AUTH.get("mode") or "none").lower() == "shared" and not _SHARED_TOKEN:
            # Ensure host is set on this user if provided via env
            if HOST_ENV and not getattr(self, "host", None):
                self.host = HOST_ENV
            if getattr(self, "host", None):
                with _SHARED_TOKEN_LOCK:
                    if not _SHARED_TOKEN:
                        shared_data = USER_DATA[DATA_SHARED_INDEX % len(USER_DATA)] if USER_DATA else {}
                        shared_ctx = {"data": shared_data, "user": {"id": 0, "index": 0}}
                        tok = _do_auth(self.client, shared_ctx)
                        if tok:
                            globals()["_SHARED_TOKEN"] = tok
                            print("🔐 Auth OK (shared token, lazy)")

    def _apply_token(
        self,
        headers: Dict[str, str],
        flow_token: Optional[str] = None,
        flow_auth_spec: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, str]:
        # Flow-level token takes precedence over the global auth token.
        # The flow's own auth spec supplies the header name/prefix.
        if flow_token is not None and flow_auth_spec is not None:
            name   = flow_auth_spec.get("header_name", "Authorization")
            prefix = flow_auth_spec.get("header_prefix", "Bearer ")
            merged = dict(headers or {})
            merged[name] = f"{prefix}{flow_token}" if prefix else flow_token
            return merged

        mode = (AUTH.get("mode") or "none").lower()
        if mode == "none":
            return headers or {}

        token = _SHARED_TOKEN if mode == "shared" else self._token
        if not token:
            return headers or {}

        name   = AUTH.get("header_name", "Authorization")
        prefix = AUTH.get("header_prefix", "Bearer ")

        merged = dict(headers or {})
        merged[name] = f"{prefix}{token}" if prefix else token
        return merged

    def _do(
        self,
        ep: Dict[str, Any],
        flow_token: Optional[str] = None,
        flow_auth_spec: Optional[Dict[str, Any]] = None,
    ):
        method = (ep["method"] or "GET").upper()
        path   = ep["path"]
        name   = ep.get("name") or f"{method} {path}"

        # Merge defaults with endpoint-specific
        headers = {**DEFAULT_HEADERS, **(ep.get("headers") or {})}
        params  = {**DEFAULT_PARAMS,  **(ep.get("params")  or {})}
        body    = ep.get("body")

        # Runtime parameterization: ${data.field} and ${user.id|index}
        ctx = {"data": self._data or {}, "user": {"id": self._user_index, "index": self._user_index}}
        path   = _expand_runtime(path, ctx)
        headers = _expand_runtime(headers, ctx)
        params  = _expand_runtime(params, ctx)
        body    = _expand_runtime(body, ctx)

        # Apply Authorization — flow-level token wins over global when provided
        headers = self._apply_token(headers, flow_token=flow_token, flow_auth_spec=flow_auth_spec)

        kwargs = {
            "headers": headers,
            "params": params,
            "timeout": REQUEST_TIMEOUT,
        }
        if body:
            kwargs["json" if isinstance(body, (dict, list)) else "data"] = body

        # Perform request (path may be absolute URL or relative — Locust handles both)
        with self.client.request(method, path, name=name, **kwargs, catch_response=True) as resp:
            if 200 <= resp.status_code < 400:
                resp.success()
            else:
                try:
                    resp_text = resp.text or ""
                except Exception:
                    resp_text = "<no text>"

                # Build a concise but complete failure line
                snippet = resp_text.replace("\n", " ").strip()[:500]
                req_body_hint = ""
                if body:
                    try:
                        raw = json.dumps(body) if isinstance(body, (dict, list)) else str(body)
                        req_body_hint = f" | req_body={raw[:200]}"
                    except Exception:
                        req_body_hint = " | req_body=<unserializable>"
                print(
                    f"[FAIL] {name} | {method} {path}"
                    f" | HTTP {resp.status_code}{req_body_hint}"
                    f" | resp={snippet}"
                )
                resp.failure(f"HTTP {resp.status_code}")

# -------------------------------------------------------------------
# Task building helpers
# -------------------------------------------------------------------

def _make_endpoint_task(endpoint: Dict[str, Any]):
    """Wrap a single endpoint dict into a Locust task function."""
    def _t(self: AutoMockUser):
        self._do(endpoint)
    nm = endpoint.get("name") or f"{endpoint.get('method','GET')} {endpoint.get('path','/')}"
    _t.__name__ = "task_" + re.sub(r"[^A-Za-z0-9_]+", "_", nm)[:80]
    return _t


def _build_task_pool(
    eps: List[Dict[str, Any]],
    flows: List[Dict[str, Any]],
    include_tags: set,
    exclude_tags: set,
) -> Dict[Any, int]:
    """Build a weighted task dict from endpoints and flows, filtered by tags."""

    def _ok(item: Dict[str, Any]) -> bool:
        tags = set(item.get("tags") or [])
        if include_tags and not (tags & include_tags):
            return False
        if exclude_tags and (tags & exclude_tags):
            return False
        return True

    pool: Dict[Any, int] = {}

    for ep in eps:
        if not _ok(ep):
            continue
        w = int(ep.get("weight", 1))
        pool[_make_endpoint_task(ep)] = w if w > 0 else 1

    for flow in flows:
        if not _ok(flow):
            continue
        if not (flow.get("steps") or []):
            continue
        w = int(flow.get("weight", 1))
        pool[_make_flow_taskset(flow)] = w if w > 0 else 1

    return pool


# -------------------------------------------------------------------
# Register User classes — cohort mode or single-class mode
# -------------------------------------------------------------------

if COHORTS:
    # Each cohort becomes its own Locust User class with a permanent,
    # filtered task pool. AutoMockUser itself is marked abstract so
    # Locust does not try to spawn it directly.
    AutoMockUser.abstract = True

    # Wrap cohort registration in a function so loop variables (especially
    # the class-typed `cohort_cls`) are scoped locally and never appear in
    # module globals.  Locust's user-class discovery scans vars(module) for
    # all non-abstract User subclasses; any leaked class variable would be
    # found twice — once under its real name and once under the loop alias —
    # triggering Locust's duplicate-name validation error.
    def _register_cohorts():
        seen_cls_names: set = set()
        for i, cohort in enumerate(COHORTS):
            inc  = set(cohort.get("include_tags") or [])
            exc  = set(cohort.get("exclude_tags") or [])
            w    = int(cohort.get("weight", 1))
            name = cohort.get("name") or "UnnamedCohort"

            pool = _build_task_pool(EPS, FLOWS, inc, exc)

            # Build the human-readable display name, deduplicating if two
            # cohort names sanitize to the same Python identifier.
            base_cls_name = re.sub(r"[^A-Za-z0-9_]+", "_", name).strip("_") or "Cohort"
            display_name = base_cls_name
            suffix = 2
            while display_name in seen_cls_names:
                display_name = f"{base_cls_name}_{suffix}"
                suffix += 1
            seen_cls_names.add(display_name)

            # Use an index-based internal name for type() so Locust's
            # UserMeta never sees two calls with the same name.  Patch
            # __name__/__qualname__ afterward for human-readable UI labels.
            cohort_cls = type(f"_AMCohort_{i}", (AutoMockUser,), {
                "weight":   w,
                "tasks":    pool,
                "abstract": False,
            })
            cohort_cls.__name__     = display_name
            cohort_cls.__qualname__ = display_name
            # globals() inside a function still refers to the module globals,
            # so this registers the class where Locust can discover it.
            globals()[display_name] = cohort_cls

    _register_cohorts()
    del _register_cohorts  # remove the helper itself from module globals

else:
    # No cohorts — single task pool for all VUs, filtered by global tags
    AutoMockUser.tasks = _build_task_pool(
        EPS, FLOWS, INCLUDE_TAGS, EXCLUDE_TAGS
    )

# -------------------------------------------------------------------
# Entry-point guard
# -------------------------------------------------------------------
# Locust files are NOT run directly with `python3 locustfile.py`.
# Running this way only executes module-level setup (data loading etc.)
# and then exits — no load test is actually started.
#
# Use the provided runner scripts instead:
#   macOS / Linux:   ./run_locust_ui.sh          (opens web UI at :8089)
#                    ./run_locust_headless.sh     (headless, set vars inside)
#   Windows:         .\run_locust_ui.ps1
#                    .\run_locust_headless.ps1
#
# Or run Locust directly:
#   locust -f locustfile.py --host http://your-target-host
#   locust -f locustfile.py --host http://your-target-host --headless -u 10 -r 2 --run-time 60s

if __name__ == "__main__":
    import sys
    print()
    print("⚠️  This file must be run via Locust, not directly via Python.")
    print()
    print("Run the load test with:")
    print("  locust -f locustfile.py --host http://your-target-host")
    print()
    print("Or use the provided scripts:")
    print("  ./run_locust_ui.sh           # web UI at http://localhost:8089")
    print("  ./run_locust_headless.sh     # headless mode (edit script to set params)")
    print("  .\\run_locust_ui.ps1          # Windows PowerShell equivalent")
    print()
    sys.exit(1)
