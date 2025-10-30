import os, json, re, math, random
from typing import Any, Dict, List, Optional
from urllib.parse import parse_qs
from locust import HttpUser, between, constant, events

# -------------------------------------------------------------------
# Config / Spec Loading
# -------------------------------------------------------------------

JSON_PATH = os.getenv("AM_LOCUST_JSON", "locust_endpoints.json")
HOST_ENV  = os.getenv("AM_HOST")  # Optional. If not set, host can be set in Locust UI.

_env_re = re.compile(r"\$\{env\.([A-Za-z_][A-Za-z0-9_]*)\}")

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

AUTH   = SPEC.get("auth") or {"mode": "none"}
CFG    = SPEC.get("config") or {}
EPS    = SPEC.get("endpoints") or []

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

def _json_get(d: Any, path: str, default=None):
    cur = d
    if not path:
        return default
    for part in path.split("."):
        if not isinstance(cur, dict) or part not in cur:
            return default
        cur = cur[part]
    return cur

def _do_auth(client):
    mode = (AUTH.get("mode") or "none").lower()
    if mode == "none":
        return None

    method  = (AUTH.get("method") or "POST").upper()
    path    = AUTH.get("path") or "/"
    headers = AUTH.get("headers") or {}
    body    = AUTH.get("body")

    kwargs = {"headers": headers, "timeout": REQUEST_TIMEOUT, "verify": VERIFY_TLS}
    if body is not None:
        kwargs["json" if isinstance(body, (dict, list)) else "data"] = body

    url_or_path = path if (path.startswith("http://") or path.startswith("https://")) else path

    r = client.request(method, url_or_path, name="AUTH "+path, **kwargs)
    if r.status_code >= 400:
        print(f"[auth] failed: HTTP {r.status_code} - {r.text[:200]}")
        return None

    # Try JSON first
    token_path = AUTH.get("token_json_path", "access_token")
    token = None
    try:
        data = r.json()
        token = _json_get(data, token_path)
    except Exception:
        # Fallback: try URL-encoded
        try:
            kv = parse_qs(r.text or "")
            # parse_qs returns lists; pick the first value
            token_list = kv.get(token_path, []) or kv.get("access_token", [])
            token = token_list[0] if token_list else None
        except Exception:
            token = None

    if not token:
        print(f"[auth] token not found at path '{token_path}'. Raw body (truncated): {r.text[:200]}")
        return None
    return token

@events.test_start.add_listener
def _on_test_start(environment, **_):
    global _SHARED_TOKEN
    if (AUTH.get("mode") or "none").lower() == "shared":
        # Locust 2.16: HttpUser.client.verify is honored per session; here we create a local context.
        ctx = environment.create_local_http_context(HOST_ENV or environment.host)
        ctx.client.verify = VERIFY_TLS
        _SHARED_TOKEN = _do_auth(ctx.client)
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

class AutoMockUser(HttpUser):
    wait_time = _select_wait_strategy()
    if HOST_ENV:
        host = HOST_ENV  # otherwise set in UI

    def on_start(self):
        # Set per-session TLS verification
        self.client.verify = VERIFY_TLS

        # Per-user auth
        self._token = None
        if (AUTH.get("mode") or "none").lower() == "per_user":
            self._token = _do_auth(self.client)

    def _apply_token(self, headers: Dict[str, str]) -> Dict[str, str]:
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

    def _do(self, ep: Dict[str, Any]):
        method = (ep["method"] or "GET").upper()
        path   = ep["path"]
        name   = ep.get("name") or f"{method} {path}"

        # Merge defaults with endpoint-specific
        headers = {**DEFAULT_HEADERS, **(ep.get("headers") or {})}
        params  = {**DEFAULT_PARAMS,  **(ep.get("params")  or {})}
        body    = ep.get("body")

        # Apply Authorization from auth flow (overrides same header if present)
        headers = self._apply_token(headers)

        kwargs = {
            "headers": headers,
            "params": params,
            "timeout": REQUEST_TIMEOUT,
        }
        if body is not None:
            kwargs["json" if isinstance(body, (dict, list)) else "data"] = body

        # Absolute URL supported; otherwise path is relative to host/UI
        url_or_path = path if (path.startswith("http://") or path.startswith("https://")) else path

        # Perform request
        with self.client.request(method, url_or_path, name=name, **kwargs, catch_response=True) as resp:
            if 200 <= resp.status_code < 400:
                resp.success()
            else:
                resp.failure(f"HTTP {resp.status_code}")

# Build weighted tasks dynamically, honoring include/exclude tags
_tasks: Dict[Any, int] = {}
for ep in EPS:
    if not _should_include(ep):
        continue
    w = int(ep.get("weight", 1))

    def make_task(endpoint: Dict[str, Any]):
        def _t(self: AutoMockUser):
            self._do(endpoint)
        # Stable python method name
        nm = endpoint.get("name") or f"{endpoint.get('method','GET')} {endpoint.get('path','/')}"
        _t.__name__ = "task_" + re.sub(r"[^A-Za-z0-9_]+", "_", nm)[:80]
        return _t

    fn = make_task(ep)
    _tasks[fn] = w if w > 0 else 1

AutoMockUser.tasks = _tasks
