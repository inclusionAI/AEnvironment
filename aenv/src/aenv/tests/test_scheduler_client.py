from __future__ import annotations

import pytest

from aenv.client.scheduler_client import AEnvSchedulerClient


class FakeCreateResponse:
    def json(self) -> dict[str, object]:
        return {
            "success": True,
            "data": {
                "id": "arca-sandbox-1",
                "status": "Pending",
                "created_at": "2026-07-07T04:12:00Z",
                "updated_at": "2026-07-07T04:12:00Z",
                "ip": "6.3.1.2",
            },
        }


class FakeAsyncClient:
    last_json: dict[str, object] | None = None

    def __init__(self, **_kwargs: object) -> None:
        pass

    async def post(self, _path: str, json: dict[str, object]) -> FakeCreateResponse:
        FakeAsyncClient.last_json = json
        return FakeCreateResponse()

    async def aclose(self) -> None:
        pass


@pytest.mark.asyncio
async def test_create_env_instance_forwards_init_command(monkeypatch) -> None:
    command = "sh -lc 'python -m http.server 8081'"
    monkeypatch.setattr(
        "aenv.client.scheduler_client.httpx.AsyncClient",
        FakeAsyncClient,
    )

    client = AEnvSchedulerClient("http://api-service-arca")
    await client.connect()

    try:
        await client.create_env_instance(
            name="swebench@1.0.4",
            datasource="registry.example.com/image:tag",
            ttl="20m",
            owner="jun",
            init_command=command,
        )
    finally:
        await client.close()

    assert FakeAsyncClient.last_json is not None
    assert FakeAsyncClient.last_json["init_command"] == command
