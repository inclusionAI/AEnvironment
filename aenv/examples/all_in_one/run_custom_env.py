import asyncio
import os
import time

from aenv import Environment


async def main():
    os.environ["AENV_SYSTEM_URL"] = "http://localhost:8080/"
    env = Environment("weather-demo@1.0.0", timeout=60)
    try:
        print(await env.list_tools())
        print(await env.call_tool("get_weather", {"city": "Beijing"}))
        print(await env.call_function("get_weather_func", {"city": "Beijing"}))
        print(await env.call_reward({"city": "Beijing"}))
    except Exception as e:
        print("发生错误:", e)
    finally:
        time.sleep(10)
        await env.release()


asyncio.run(main())
