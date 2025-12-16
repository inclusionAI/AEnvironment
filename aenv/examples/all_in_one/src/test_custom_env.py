from aenv import Environment
import pytest
import os

@pytest.mark.asyncio
async def test_weather():
    os.environ["DUMMY_INSTANCE_IP"] = "127.0.0.1"
    env = Environment("weather-demo@1.0.0")
    print(await env.list_tools())
    print(await env.call_tool("get_weather", {"city": "Beijing"}))
    print(await env.call_function("get_weather_func", {"city": "Beijing"}))
    print(await env.call_reward({"city": "Beijing"}))