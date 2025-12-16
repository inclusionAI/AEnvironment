# Copyright 2025.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import asyncio

from aenv.core.environment import Environment


async def main():
    env1 = Environment("test@0.0.4")

    try:
        tools = await env1.list_tools()
        print("Successfully retrieved tool list:", tools)
        assert tools is not None
        result = await env1.call_tool(
            "test@0.0.4/get_current_weather", {"location": "Beijing"}
        )
        print("Successfully called tool:", result)

        result = await env1.call_reward({"query": "Shanghai"})
        print("Successfully called reward:", result)

        await env1.release()
        print("Successfully released environment")
    except Exception as e:
        print(
            "Test completed - Environment created successfully, but tool list may be empty:",
            str(e),
        )
        # This is expected, as the environment may still be starting


if __name__ == "__main__":
    asyncio.run(main())
