#!/usr/bin/env python3
"""
Contract tests for arca + ``enable_data_plane=False``.

Round 2: every data-plane method on ``Environment`` raises
``EnvironmentError`` mentioning ``enable_data_plane=False`` instead of
silently hitting api-service:8081.

Round 3: api-service:8081 (the MCP gateway port) returns HTTP 501 with an
actionable message for any path when running in arca schedule mode. This
is the server-side guarantee that pairs with Round 2.

Both rounds talk to a real api-service-arca + arca sandbox, so the script
must run against tydd-staging (or another arca-mode deployment).

Env vars:
    AENV_SYSTEM_URL         api-service base URL on :8080  (required)
    ARCA_TEST_ENV_NAME      envhub env name (e.g. arca-real@1.0.0)  (required)
    AENV_API_KEY            optional bearer token
"""

from __future__ import annotations

import asyncio
import json
import os
import sys
from urllib.parse import urlparse, urlunparse

import httpx

from aenv import Environment
from aenv.core.exceptions import EnvironmentError

CONTROL_URL = os.environ.get(
    "AENV_SYSTEM_URL", os.environ.get("ARCA_API_SERVICE_URL", "")
)
ENV_NAME = os.environ.get("ARCA_TEST_ENV_NAME", "")
AENV_API_KEY = os.environ.get("AENV_API_KEY", "")

DATA_PLANE_METHODS: list[tuple[str, tuple]] = [
    ("call_tool", ("t", {})),
    ("list_tools", ()),
    ("list_functions", ()),
    ("call_reward", ({},)),
    ("check_health", ({},)),
    ("call_function", ("f", {})),
]


def _data_plane_url(control_url: str) -> str:
    """Swap the :8080 port in control_url for :8081 (the MCP gateway)."""
    parsed = urlparse(control_url if "://" in control_url else f"http://{control_url}")
    host = parsed.hostname or "127.0.0.1"
    return urlunparse(parsed._replace(netloc=f"{host}:8081", path="", query=""))


async def round2_guards() -> bool:
    print("=== Round 2: Environment guards under enable_data_plane=False ===")
    async with Environment(
        env_name=ENV_NAME,
        aenv_url=CONTROL_URL,
        api_key=AENV_API_KEY or None,
        enable_data_plane=False,
        ttl="5m",
    ) as env:
        for name, args in DATA_PLANE_METHODS:
            try:
                await getattr(env, name)(*args)
            except EnvironmentError as e:
                if "enable_data_plane=False" not in str(e):
                    print(f"  {name}: raised wrong message: {e}")
                    return False
                print(f"  {name}: blocked OK")
                continue
            print(f"  {name}: did NOT raise -- BUG")
            return False
    print("Round 2 PASS")
    return True


async def round3_501() -> bool:
    print("=== Round 3: api-service:8081 returns 501 for arca data plane ===")
    data_plane_base = _data_plane_url(CONTROL_URL)
    async with Environment(
        env_name=ENV_NAME,
        aenv_url=CONTROL_URL,
        api_key=AENV_API_KEY or None,
        enable_data_plane=False,
        ttl="5m",
    ) as env:
        info = await env.get_env_info()
        sandbox_id = info["instance_id"]
        async with httpx.AsyncClient(timeout=10.0) as client:
            for path in ["/health", "/mcp", "/some/random"]:
                resp = await client.get(
                    f"{data_plane_base}{path}",
                    headers={"AEnvCore-EnvInstance-ID": sandbox_id},
                )
                try:
                    body = resp.json()
                except Exception:
                    body = {"_raw": resp.text[:200]}

                ok = (
                    resp.status_code == 501
                    and isinstance(body, dict)
                    and "data plane" in str(body.get("message", ""))
                )
                tag = "OK" if ok else "FAIL"
                print(
                    f"  {path}: status={resp.status_code} body={json.dumps(body)} [{tag}]"
                )
                if not ok:
                    return False
    print("Round 3 PASS")
    return True


async def main() -> int:
    if not CONTROL_URL or not ENV_NAME:
        print(
            "missing required env vars AENV_SYSTEM_URL and/or ARCA_TEST_ENV_NAME",
            file=sys.stderr,
        )
        return 2
    r2 = await round2_guards()
    r3 = await round3_501()
    return 0 if (r2 and r3) else 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
