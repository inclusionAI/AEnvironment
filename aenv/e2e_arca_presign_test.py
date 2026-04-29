#!/usr/bin/env python3
"""
End-to-end test for arca engine: aenv SDK lifecycle + presigned URL access.

Pipeline:
    1. aenv SDK  -> api-service-arca -> Arca: create sandbox
    2. aenv SDK  -> api-service-arca -> Arca: env.presign_url(port)
    3. httpx GET <presigned URL>                   (direct, not via api-service)
    4. aenv SDK  -> api-service-arca -> Arca: release

The SDK has no engine awareness; api-service resolves the engine-specific
behaviour (presign endpoint, error codes).

Required env vars:
    ARCA_API_SERVICE_URL    api-service-arca base URL (default http://localhost:18080)
    ARCA_TEST_ENV_NAME      envhub env name with @version
    AENV_API_KEY            (optional) bearer token for api-service

Optional env vars:
    ARCA_SERVICE_PORT       in-sandbox service port (default 8080)
    ARCA_SERVICE_PATH       path to GET on the presigned URL (default /)
    ARCA_PRESIGN_TTL_MIN    presign url ttl in minutes (default 5)
"""
from __future__ import annotations

import asyncio
import os
import sys
import time
import traceback
from typing import Optional

import httpx

from aenv.core.environment import Environment

API_SERVICE_URL = os.environ.get("ARCA_API_SERVICE_URL", "http://localhost:18080")
ENV_NAME = os.environ.get("ARCA_TEST_ENV_NAME", "")
AENV_API_KEY = os.environ.get("AENV_API_KEY", "")

SERVICE_PORT = int(os.environ.get("ARCA_SERVICE_PORT", "8080"))
SERVICE_PATH = os.environ.get("ARCA_SERVICE_PATH", "/")
PRESIGN_TTL_MIN = float(os.environ.get("ARCA_PRESIGN_TTL_MIN", "5"))


def _ok(msg: str) -> None:
    print(f"[ OK ] {msg}", flush=True)


def _fail(msg: str) -> None:
    print(f"[FAIL] {msg}", flush=True)
    sys.exit(1)


def _info(msg: str) -> None:
    print(f"[INFO] {msg}", flush=True)


def _require_env() -> None:
    if not ENV_NAME:
        _fail("missing required env var ARCA_TEST_ENV_NAME")


async def lifecycle() -> None:
    env = Environment(
        env_name=ENV_NAME,
        aenv_url=API_SERVICE_URL,
        ttl="10m",
        startup_timeout=180.0,
        timeout=60.0,
        max_retries=1,
        api_key=AENV_API_KEY or None,
    )

    sandbox_id: Optional[str] = None
    try:
        t0 = time.time()
        try:
            await env.initialize()
        except Exception as e:
            traceback.print_exc()
            _fail(f"aenv initialize failed: {e!r}")

        if not env._instance:
            _fail("env._instance is None after initialize")
        sandbox_id = env._instance.id
        _ok(f"initialize ok in {time.time()-t0:.1f}s (sandbox_id={sandbox_id})")

        try:
            presigned = await env.presign_url(
                port=SERVICE_PORT,
                expiration_time_in_minutes=PRESIGN_TTL_MIN,
            )
        except Exception as e:
            traceback.print_exc()
            _fail(f"env.presign_url failed: {e!r}")
        if not presigned or not presigned.startswith("http"):
            _fail(f"presigned url malformed: {presigned!r}")
        _ok(f"presign_url ok (port={SERVICE_PORT}, ttl={PRESIGN_TTL_MIN}m)")
        _info(f"presigned URL: {presigned}")

        target = presigned.rstrip("/") + (
            SERVICE_PATH if SERVICE_PATH.startswith("/") else "/" + SERVICE_PATH
        )
        async with httpx.AsyncClient(timeout=15.0, follow_redirects=True) as client:
            try:
                resp = await client.get(target)
            except Exception as e:
                traceback.print_exc()
                _fail(f"GET presigned URL failed: {e!r}")
        body_excerpt = (resp.text or "")[:200].replace("\n", " ")
        _info(
            f"sandbox service GET {SERVICE_PATH} -> {resp.status_code} body={body_excerpt!r}"
        )
        if resp.status_code >= 500:
            _fail(f"in-sandbox service 5xx: {resp.status_code}")
        _ok(f"in-sandbox service reachable via presigned URL ({resp.status_code})")

    finally:
        try:
            await env.release()
            _ok(f"release ok (sandbox_id={sandbox_id})")
        except Exception as e:
            traceback.print_exc()
            _fail(f"aenv release failed: {e!r}")


async def main() -> None:
    _require_env()
    _info(f"api-service: {API_SERVICE_URL}")
    _info(f"env name: {ENV_NAME}")
    await lifecycle()
    print("\n[PASS] e2e arca presign happy path", flush=True)


if __name__ == "__main__":
    asyncio.run(main())
