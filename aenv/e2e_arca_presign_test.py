#!/usr/bin/env python3
"""
Example: aenv + arca engine — fast create with no data-plane wait, then
access the sandbox via a presigned URL (direct data-plane, bypassing
api-service).

When to use this pattern:
    * Engine is arca and the sandbox image runs an arbitrary user app
      (no aenv MCP server inside).
    * You want minimum latency at create time and prefer to wait on the
      in-sandbox app's own readiness path.

High-level flow:
    1. create sandbox        (Environment.__aenter__, no MCP session)
    2. presign URL           (Environment.presign_url)
    3. caller polls the URL  (httpx, business-defined readiness path)
    4. caller does real work (whatever the sandbox process listens for)
    5. release               (Environment.__aexit__)

``enable_data_plane=False`` is what makes this work on arca: the SDK
skips the MCP session and the /health probe entirely, so no traffic
hits api-service:8081 (which on arca returns 501 by design). The
``call_tool`` / ``list_tools`` / ``call_function`` / ``call_reward`` /
``check_health`` methods will raise if invoked under this flag.

Env vars:
    AENV_SYSTEM_URL         api-service base URL  (default http://localhost)
    ARCA_TEST_ENV_NAME      envhub env name (arcaTemplateId inside)  (required)
    AENV_API_KEY            bearer token if api-service has auth enabled  (optional)
    ARCA_SERVICE_PORT       in-sandbox port to expose  (default 18080)
    ARCA_SERVICE_PATH       path to GET for readiness  (default /healthz)
    ARCA_PRESIGN_TTL_MIN    presign URL ttl  (default 5)
    ARCA_READINESS_TIMEOUT  readiness polling budget  (default 45s)
"""

from __future__ import annotations

import asyncio
import os
import sys
import time

import httpx

from aenv import Environment

ENV_NAME = os.environ.get("ARCA_TEST_ENV_NAME", "")
AENV_URL = os.environ.get("AENV_SYSTEM_URL", os.environ.get("ARCA_API_SERVICE_URL", ""))
AENV_API_KEY = os.environ.get("AENV_API_KEY", "")

SERVICE_PORT = int(os.environ.get("ARCA_SERVICE_PORT", "18080"))
SERVICE_PATH = os.environ.get("ARCA_SERVICE_PATH", "/healthz")
PRESIGN_TTL_MIN = float(os.environ.get("ARCA_PRESIGN_TTL_MIN", "5"))
READINESS_TIMEOUT_S = float(os.environ.get("ARCA_READINESS_TIMEOUT", "45"))


async def wait_ready(
    target: str, timeout_s: float = READINESS_TIMEOUT_S
) -> httpx.Response:
    """Poll GET target until 2xx or timeout. Returns the last response."""
    deadline = time.time() + timeout_s
    last: httpx.Response | None = None
    async with httpx.AsyncClient(timeout=10.0, follow_redirects=True) as c:
        attempt = 0
        while time.time() < deadline:
            attempt += 1
            try:
                last = await c.get(target)
                if 200 <= last.status_code < 300:
                    return last
                print(f"  readiness attempt {attempt}: status={last.status_code}")
            except Exception as e:
                print(f"  readiness attempt {attempt}: {type(e).__name__}: {e}")
            await asyncio.sleep(2.0)
    if last is None:
        raise TimeoutError(f"never got a response within {timeout_s}s")
    raise TimeoutError(
        f"never became ready within {timeout_s}s "
        f"(last status={last.status_code}, body={last.text[:200]!r})"
    )


async def main() -> int:
    if not ENV_NAME:
        print("missing required env var ARCA_TEST_ENV_NAME", file=sys.stderr)
        return 1

    print(f"env_name: {ENV_NAME}")
    print(f"aenv_url: {AENV_URL or '(default)'}")

    t0 = time.time()
    async with Environment(
        env_name=ENV_NAME,
        aenv_url=AENV_URL or None,
        api_key=AENV_API_KEY or None,
        ttl="10m",
        enable_data_plane=False,
    ) as env:
        info = await env.get_env_info()
        print(
            f"[1] created sandbox in {time.time()-t0:.2f}s "
            f"(id={info['instance_id']}, status={info['status']})"
        )

        url = await env.presign_url(
            port=SERVICE_PORT,
            expiration_time_in_minutes=PRESIGN_TTL_MIN,
        )
        print(f"[2] presigned URL ({SERVICE_PORT}): {url}")

        target = url.rstrip("/") + (
            SERVICE_PATH if SERVICE_PATH.startswith("/") else "/" + SERVICE_PATH
        )
        print(f"[3] waiting for {SERVICE_PATH} to return 2xx...")
        resp = await wait_ready(target)
        print(f"    ready in {time.time()-t0:.1f}s; status={resp.status_code}")
        print(f"    body: {resp.text[:200]!r}")

        # The same ``url`` is a valid base for HTTP / WebSocket / anything
        # the sandbox process listens for on that port. It remains valid
        # until the presign TTL expires.
        print("[4] (business traffic would run here)")

    print(f"[5] released sandbox in {time.time()-t0:.1f}s total")
    return 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
