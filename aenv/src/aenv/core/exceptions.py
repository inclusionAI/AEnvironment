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
AEnv SDK exceptions.
"""

from typing import Any, Dict, Optional


class AEnvError(Exception):
    """Base exception for AEnv SDK."""

    def __init__(self, message: str, details: Optional[Dict[str, Any]] = None):
        super().__init__(message)
        self.message = message
        self.details = details or {}


class ToolError(AEnvError):
    """Exception raised when tool execution fails."""

    def __init__(
        self,
        message: str,
        tool_name: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
    ):
        super().__init__(message, details)
        self.tool_name = tool_name


class EnvironmentError(AEnvError):
    """Exception raised when environment operations fail."""

    def __init__(
        self,
        message: str,
        env_name: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
    ):
        super().__init__(message, details)
        self.env_name = env_name


class ToolTimeoutError(ToolError):
    """Exception raised when tool execution times out."""

    def __init__(
        self,
        message: str,
        tool_name: Optional[str] = None,
        timeout: Optional[float] = None,
        details: Optional[Dict[str, Any]] = None,
    ):
        super().__init__(message, tool_name, details)
        self.timeout = timeout


class ToolServerError(ToolError):
    """Exception raised when tool server returns 5xx error."""

    def __init__(
        self,
        message: str,
        tool_name: Optional[str] = None,
        status_code: Optional[int] = None,
        details: Optional[Dict[str, Any]] = None,
    ):
        super().__init__(message, tool_name, details)
        self.status_code = status_code


class NetworkError(AEnvError):
    """Exception raised for network-related errors."""

    pass


class ValidationError(AEnvError):
    """Exception raised for input validation errors."""

    pass
