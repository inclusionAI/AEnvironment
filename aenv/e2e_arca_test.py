#!/usr/bin/env python3
"""
End-to-end smoke for api-service-arca.

Runs against the in-cluster service via kubectl port-forward.

Env:
  ARCA_API_SERVICE_URL = http://localhost:<local-port>   (default 18080)
  ARCA_TEST_ENV_NAME   = envhub env name (with @version) to create sandbox
                         from. If unset, only the proxy liveness probe runs.

The SDK is engine-unaware: this script never asserts on labels or NotImplementedError.
"""
from __future__ import annotations

import asyncio
import os
import sys
import time
import traceback

from aenv.core.environment import Environment

ARCA_URL = os.environ.get("ARCA_API_SERVICE_URL", "http://localhost:18080")
ENV_NAME = os.environ.get("ARCA_TEST_ENV_NAME", "")
API_KEY = os.environ.get("AENV_API_KEY", "")


def _fail(msg: str) -> None:
    print(f"[FAIL] {msg}")
    sys.exit(1)


def _ok(msg: str) -> None:
    print(f"[ OK ] {msg}")


async def probe_health() -> None:
    """Plain HTTP liveness via httpx (proves port-forward works)."""
    import httpx

    async with httpx.AsyncClient(timeout=5.0) as c:
        r = await c.get(f"{ARCA_URL}/health")
        if r.status_code != 200:
            _fail(f"/health returned {r.status_code}: {r.text}")
        _ok("/health -> 200")


async def lifecycle() -> None:
    if not ENV_NAME:
        print("[SKIP] ARCA_TEST_ENV_NAME not set; skipping create/release")
        return

    env = Environment(
        env_name=ENV_NAME,
        aenv_url=ARCA_URL,
        ttl="10m",
        startup_timeout=180.0,
        timeout=60.0,
        max_retries=1,
        api_key=API_KEY or None,
    )

    t0 = time.time()
    try:
        await env.initialize()
    except Exception as e:
        print("[FAIL] initialize raised")
        traceback.print_exc()
        _fail(f"create failed: {e!r}")

    _ok(f"initialize ok in {time.time()-t0:.1f}s, instance={env._instance}")

    if not env._instance:
        _fail("env._instance is None after initialize")

    await env.release()
    _ok("release ok")


async def main() -> None:
    await probe_health()
    await lifecycle()


if __name__ == "__main__":
    asyncio.run(main())
