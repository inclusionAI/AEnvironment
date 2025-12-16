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
    mini_terminal = Environment("mini-terminal@1.0.1")

    try:
        tools = await mini_terminal.list_tools()
        print("Successfully retrieved tool list:", tools)
        assert tools is not None
    except Exception as e:
        print(
            "Test completed - Environment created successfully, but tool list may be empty:",
            str(e),
        )

    while True:
        try:
            user_input = input(">>> ").strip()
            if user_input.lower() in ("exit", "quit"):
                print("Exiting interactive mode.")
                break

            # Call remote tool to execute user input command
            result = await mini_terminal.call_tool(
                "mini-terminal@1.0.1/execute_command",  # Tool name
                {"command": user_input, "timeout": 5},  # User input command
            )
            print("Execution result:\n", result)
            print("-" * 60)

        except KeyboardInterrupt:
            print(
                "\nInterrupt detected, enter 'exit' to quit or continue entering commands."
            )
            await mini_terminal.release()
            print("Successfully released environment")


if __name__ == "__main__":
    asyncio.run(main())
