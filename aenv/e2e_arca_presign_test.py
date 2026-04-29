#!/usr/bin/env python3
"""
Example: aenv + arca engine — fast create with no ready-wait, then access
the sandbox via a presigned URL (direct data-plane, bypassing api-service).

When to use this pattern instead of ``Environment.initialize``:
    * You want to expose an in-sandbox HTTP/WebSocket service to arbitrary
      external clients (e.g. a UI, a webhook, an LLM agent on another host),
      not just the local Python process.
    * You don't need aenv's MCP/tool-call flows — only lifecycle + a URL.
    * You want minimum latency at create time: arca's presign accepts the
      sandbox even in PENDING, and the in-sandbox app itself exposes
      readiness (so the SDK's _wait_for_healthy is redundant).

High-level flow:
    1. create sandbox         (AEnvSchedulerClient.create_env_instance)
    2. presign URL            (AEnvSchedulerClient.presign_url)
    3. caller polls the URL   (httpx, business-defined readiness path)
    4. caller does real work  (whatever they want to call on the sandbox)
    5. delete sandbox         (AEnvSchedulerClient.delete_env_instance)

There is NO call to ``Environment.initialize`` / ``wait_for_ready`` /
``_wait_for_healthy`` — all waiting is pushed to the business layer.

Env vars:
    ARCA_API_SERVICE_URL    api-service-arca base URL  (default http://localhost:18080)
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

from aenv.client.scheduler_client import AEnvSchedulerClient
from aenv.core.environment import make_mcp_url

API_SERVICE_URL = os.environ.get("ARCA_API_SERVICE_URL", "http://localhost:18080")
ENV_NAME = os.environ.get("ARCA_TEST_ENV_NAME", "")
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

    control_url = make_mcp_url(API_SERVICE_URL, 8080)
    print(f"api-service: {control_url}")
    print(f"env_name:    {ENV_NAME}")

    async with AEnvSchedulerClient(
        base_url=control_url,
        api_key=AENV_API_KEY or None,
    ) as client:
        # -- 1. create sandbox (no wait for RUNNING/healthy) -----------------
        t0 = time.time()
        inst = await client.create_env_instance(name=ENV_NAME, ttl="10m")
        print(
            f"[1] created sandbox in {time.time()-t0:.2f}s "
            f"(id={inst.id}, status={inst.status})"
        )

        try:
            # -- 2. presign a URL pointing at sandbox:$PORT ------------------
            url = await client.presign_url(
                inst.id,
                port=SERVICE_PORT,
                expiration_time_in_minutes=PRESIGN_TTL_MIN,
            )
            print(f"[2] presigned URL ({SERVICE_PORT}): {url}")

            # -- 3. poll the in-sandbox service until it's up ----------------
            target = url.rstrip("/") + (
                SERVICE_PATH if SERVICE_PATH.startswith("/") else "/" + SERVICE_PATH
            )
            print(f"[3] waiting for {SERVICE_PATH} to return 2xx...")
            resp = await wait_ready(target)
            print(f"    ready in {time.time()-t0:.1f}s; status={resp.status_code}")
            print(f"    body: {resp.text[:200]!r}")

            # -- 4. (your real traffic goes here) ----------------------------
            # The same ``url`` is a valid base for HTTP / WebSocket / anything
            # the sandbox process listens for on that port. It remains valid
            # until the presign TTL expires.
            print("[4] (business traffic would run here)")
        finally:
            # -- 5. always release -------------------------------------------
            await client.delete_env_instance(inst.id)
            print(f"[5] released sandbox (id={inst.id})")

    return 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
