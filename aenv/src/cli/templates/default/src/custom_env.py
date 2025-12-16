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

from typing import Any, Dict

from aenv import register_reward, register_tool


@register_tool
def my_custom_echo_env(content: str) -> Dict[str, Any]:
    """
    Tool description for your custom env.

    Args:
        content: echo whatever you want

    Returns:
        Dictionary with results
    """
    return {"result": f"{content}"}


@register_reward
def my_custom_reward(task: str) -> Dict[str, Any]:
    return {
        "task_name": task,
        "status": "success",
        "score": 1.0,
        "raw_output": "hello reward!",
    }
