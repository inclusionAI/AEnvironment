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
instances command - List running environment instances
"""
import json
import os
from typing import Optional
from urllib.parse import urlparse, urlunparse

import click
from tabulate import tabulate

from cli.cmds.common import Config, pass_config
from cli.utils.cli_config import get_config_manager


def _make_api_url(aenv_url: str, port: int = 8080) -> str:
    """Make API URL with specified port, similar to make_mcp_url logic.

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
        path=p.path,
        params="",
        query="",
        fragment="",
    )
    return urlunparse(new).rstrip("/")


def _get_system_url() -> str:
    """Get AEnv system URL from environment variable or config.

    Uses make_api_url logic to ensure port 8080 is specified.
    """
    system_url = os.getenv("AENV_SYSTEM_URL")
    if not system_url:
        # Try to get from config, but for now default to localhost
        system_url = "http://localhost:8080"
    # Use make_api_url to ensure port 8080 is set
    return _make_api_url(system_url, port=8080)


def _get_instance_info(system_url: str, instance_id: str) -> Optional[dict]:
    """Get detailed information for a single instance.

    Args:
        system_url: AEnv system URL
        instance_id: Instance ID

    Returns:
        Instance details dict or None if failed
    """
    import requests  # noqa: I001

    url = f"{system_url}/env-instance/{instance_id}"

    # Get API key from config if available
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"

    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()

        result = response.json()
        if result.get("success") and result.get("data"):
            return result["data"]
        return None
    except requests.exceptions.RequestException:
        # Silently fail and return None - we'll use list data as fallback
        return None


def _list_instances_from_api(
    system_url: str, env_name: Optional[str] = None, version: Optional[str] = None
) -> list:
    """List running instances from API service.

    Args:
        system_url: AEnv system URL
        env_name: Optional environment name filter
        version: Optional version filter

    Returns:
        List of running instances
    """
    import requests  # noqa: I001

    # Build the API endpoint
    # Route is /env-instance/:id/list where :id is required
    # Use "*" to list all instances when no filter is specified
    if env_name:
        if version:
            # Format: name@version
            env_id = f"{env_name}@{version}"
        else:
            env_id = env_name
    else:
        # Use "*" to list all instances
        env_id = "*"
    url = f"{system_url}/env-instance/{env_id}/list"

    # Get API key from config if available
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"

    try:
        response = requests.get(url, headers=headers, timeout=30)
        response.raise_for_status()

        result = response.json()
        if result.get("success") and result.get("data"):
            return result["data"]
        return []
    except requests.exceptions.RequestException as e:
        raise click.ClickException(f"Failed to query instances: {str(e)}")


@click.command("instances")
@click.option(
    "--name",
    "-n",
    type=str,
    help="Filter by environment name",
)
@click.option(
    "--version",
    "-v",
    type=str,
    help="Filter by environment version (requires --name)",
)
@click.option(
    "--format",
    "-f",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@click.option(
    "--system-url",
    type=str,
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var or http://localhost:8080)",
)
@pass_config
def instances(cfg: Config, name, version, format, system_url):
    """List running environment instances

    Query and display running environment instances. Can filter by environment
    name and optionally by version.

    Examples:
        # List all running instances
        aenv instances

        # List instances for a specific environment
        aenv instances --name my-env

        # List instances for a specific environment and version
        aenv instances --name my-env --version 1.0.0

        # Output as JSON
        aenv instances --format json

        # Use custom system URL
        aenv instances --system-url http://api.example.com:8080
    """
    if version and not name:
        raise click.BadOptionUsage(
            "--version", "Version filter requires --name to be specified"
        )

    # Get system URL and ensure port 8080 is set
    if not system_url:
        system_url = _get_system_url()
    else:
        # Apply make_api_url logic to ensure port 8080
        system_url = _make_api_url(system_url, port=8080)

    try:
        instances_list = _list_instances_from_api(system_url, name, version)
    except Exception as e:
        raise click.ClickException(f"Failed to list instances: {str(e)}")

    if not instances_list:
        if name:
            if version:
                click.echo(f"📭 No running instances found for {name}@{version}")
            else:
                click.echo(f"📭 No running instances found for {name}")
        else:
            click.echo("📭 No running instances found")
        return

    if format == "json":
        click.echo(json.dumps(instances_list, indent=2, ensure_ascii=False))
    elif format == "table":
        # Prepare table data
        table_data = []
        for instance in instances_list:
            instance_id = instance.get("id", "")
            if not instance_id:
                continue

            # Try to get detailed info for each instance
            detailed_info = _get_instance_info(system_url, instance_id)

            # Use detailed info if available, otherwise use list data
            if detailed_info:
                env_info = detailed_info.get("env") or {}
            else:
                env_info = instance.get("env") or {}

            env_name = env_info.get("name") if env_info else None
            env_version = env_info.get("version") if env_info else None

            # If env is None, try to extract from instance ID (format: envname-randomid)
            if not env_name and instance_id:
                # Try to extract environment name from instance ID
                parts = instance_id.split("-")
                if len(parts) >= 2:
                    # Assume format: envname-randomid or envname-version-randomid
                    env_name = parts[0]

            # Get IP from detailed info or list data
            if detailed_info:
                ip = detailed_info.get("ip") or ""
            else:
                ip = instance.get("ip") or ""

            if not ip:
                ip = "-"

            # Get status from detailed info or list data
            status = (
                detailed_info.get("status")
                if detailed_info
                else instance.get("status") or "-"
            )

            # Get created_at from list data (list API already includes this)
            created_at = instance.get("created_at") or "-"

            table_data.append(
                {
                    "Instance ID": instance_id,
                    "Environment": env_name or "-",
                    "Version": env_version or "-",
                    "Status": status,
                    "IP": ip,
                    "Created At": created_at,
                }
            )

        if table_data:
            click.echo(tabulate(table_data, headers="keys", tablefmt="grid"))
        else:
            click.echo("📭 No running instances found")
