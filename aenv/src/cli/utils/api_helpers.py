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
Shared API helper functions for CLI commands.

This module provides common utility functions for API interactions, configuration
management, and environment variable parsing used across multiple CLI commands.
"""

import os
from datetime import datetime
from typing import Dict, Optional, Union
from urllib.parse import urlparse, urlunparse

import click

from cli.utils.cli_config import get_config_manager


def parse_env_vars(env_var_list: tuple) -> Dict[str, str]:
    """Parse environment variables from command line arguments.

    Args:
        env_var_list: Tuple of strings in format "KEY=VALUE"

    Returns:
        Dictionary of environment variables

    Raises:
        click.BadParameter: If any variable is not in KEY=VALUE format
    """
    env_vars = {}
    for env_var in env_var_list:
        if "=" not in env_var:
            raise click.BadParameter(
                f"Environment variable must be in format KEY=VALUE, got: {env_var}"
            )
        key, value = env_var.split("=", 1)
        env_vars[key.strip()] = value.strip()
    return env_vars


def get_system_url_raw() -> Optional[str]:
    """Get raw AEnv system URL from environment variable or config (without processing).

    Priority order:
    1. AENV_SYSTEM_URL environment variable (highest priority)
    2. system_url in config file
    3. None (no default)

    Returns:
        Raw system URL string or None if not found
    """
    # First check environment variable
    system_url = os.getenv("AENV_SYSTEM_URL")

    # If not in env, check config
    if not system_url:
        config_manager = get_config_manager()
        system_url = config_manager.get("system_url")

    return system_url


def make_api_url(aenv_url: str, port: int = 8080) -> str:
    """Make API URL with specified port.

    Args:
        aenv_url: Base URL (with or without protocol)
        port: Port number (default 8080)

    Returns:
        URL with specified port
    """
    if not aenv_url:
        return f"http://localhost:{port}"

    if "://" not in aenv_url:
        aenv_url = f"http://{aenv_url}"

    p = urlparse(aenv_url)
    host = p.hostname or "127.0.0.1"
    new = p._replace(
        scheme="http",
        netloc=f"{host}:{port}",
        path="",
        params="",
        query="",
        fragment="",
    )
    return urlunparse(new).rstrip("/")


def get_system_url() -> str:
    """Get AEnv system URL from environment variable or config (with default fallback).

    Priority order:
    1. AENV_SYSTEM_URL environment variable (highest priority)
    2. system_url in config file
    3. Default value (http://localhost:8080)

    Returns:
        System URL string
    """
    system_url = get_system_url_raw()
    if not system_url:
        system_url = "http://localhost:8080"
    return system_url.rstrip("/")


def get_api_headers() -> Dict[str, str]:
    """Get API headers with authentication if available.

    Returns:
        Dictionary of HTTP headers including authentication if API key is configured
    """
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    return headers


def format_time_to_local(time_value: Optional[Union[str, datetime]]) -> str:
    """Convert UTC time to local timezone and format as string.

    Args:
        time_value: UTC time as string (ISO format) or datetime object

    Returns:
        Formatted time string in local timezone (YYYY-MM-DD HH:MM:SS)
        Returns "-" if time_value is None or invalid
    """
    if not time_value:
        return "-"

    try:
        # Parse string to datetime if needed
        if isinstance(time_value, str):
            # Try parsing ISO format with or without timezone info
            if time_value.endswith("Z"):
                dt = datetime.fromisoformat(time_value.replace("Z", "+00:00"))
            elif "+" in time_value or time_value.count("-") > 2:
                dt = datetime.fromisoformat(time_value)
            else:
                # Assume UTC if no timezone info
                dt = datetime.fromisoformat(time_value).replace(tzinfo=None)
                # Convert from UTC to local
                from datetime import timezone

                dt = dt.replace(tzinfo=timezone.utc).astimezone()
        elif isinstance(time_value, datetime):
            dt = time_value
            # Convert to local timezone if it has timezone info
            if dt.tzinfo is not None:
                dt = dt.astimezone()
        else:
            return "-"

        # Convert to local timezone if it has timezone info
        if dt.tzinfo is not None:
            dt = dt.astimezone()

        # Format as local time string
        return dt.strftime("%Y-%m-%d %H:%M:%S")
    except (ValueError, AttributeError, TypeError):
        # If parsing fails, return the original value as string
        return str(time_value) if time_value else "-"
