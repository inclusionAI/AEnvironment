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

"""
AEnv Python SDK - Production-grade environment for AI agent tools

This package provides a complete SDK for managing AI agent tools in a
containerized environment with MCP protocol support.
"""

from aenv.core.environment import Environment
from aenv.core.exceptions import AEnvError, EnvironmentError, ToolError
from aenv.core.function_registry import (
    register_function,
    register_health,
    register_reward,
)
from aenv.core.models import EnvInstance, EnvStatus
from aenv.core.tool import Tool, get_registry, register_tool

__version__ = "0.1.0"
__all__ = [
    "Tool",
    "register_tool",
    "register_reward",
    "register_health",
    "register_function",
    "get_registry",
    "Environment",
    "AEnvError",
    "ToolError",
    "EnvironmentError",
    "EnvInstance",
    "EnvStatus",
]
