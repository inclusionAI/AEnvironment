from __future__ import annotations

import pytest

from aenv.core.environment import Environment
from aenv.core.models import EnvInstance


class FakeSchedulerClient:
    last_init_command: str | None = None

    async def create_env_instance(
        self,
        **kwargs: str | dict[str, str] | list[str] | None,
    ) -> EnvInstance:
        init_command = kwargs.get("init_command")
        FakeSchedulerClient.last_init_command = (
            init_command if isinstance(init_command, str) else None
        )
        return EnvInstance(
            id="arca-sandbox-1",
            status="Pending",
            created_at="2026-07-10T00:00:00Z",
            updated_at="2026-07-10T00:00:00Z",
            ip="127.0.0.1",
        )


@pytest.mark.asyncio
async def test_create_env_instance_forwards_init_command_from_environment(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    command = "sh -lc 'python -m http.server 8081'"
    env = Environment("swebench@1.0.4", init_command=command)
    env._client = FakeSchedulerClient()

    async def fake_wait_for_ready(timeout: float | None = None) -> None:
        return None

    monkeypatch.setattr(env, "wait_for_ready", fake_wait_for_ready)

    await env._create_env_instance()

    assert FakeSchedulerClient.last_init_command == command
