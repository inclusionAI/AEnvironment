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
instance command - Manage environment instances

This command provides a unified interface for managing environment instances:
- instance create: Create new instances
- instance list: List running instances
- instance get: Get detailed instance information
- instance delete: Delete an instance

Uses HTTP API for control plane operations (list, get, delete)
Uses Environment SDK for deployment operations (create)
"""
import asyncio
import json
import os
from typing import Dict, Any, Optional
from urllib.parse import urlparse, urlunparse

import click
import requests
from tabulate import tabulate

from cli.cmds.common import Config, pass_config
from cli.utils.cli_config import get_config_manager
from aenv.core.environment import Environment


def _parse_env_vars(env_var_list: tuple) -> Dict[str, str]:
    """Parse environment variables from command line arguments.

    Args:
        env_var_list: Tuple of strings in format "KEY=VALUE"

    Returns:
        Dictionary of environment variables
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


def _parse_arguments(arg_list: tuple) -> list:
    """Parse command line arguments.

    Args:
        arg_list: Tuple of argument strings

    Returns:
        List of arguments
    """
    return list(arg_list) if arg_list else []


def _make_api_url(aenv_url: str, port: int = 8080) -> str:
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


def _get_system_url() -> str:
    """Get AEnv system URL from environment variable or config."""
    system_url = os.getenv("AENV_SYSTEM_URL")
    if not system_url:
        system_url = "http://localhost:8080"
    return _make_api_url(system_url, port=8080)


def _get_api_headers() -> Dict[str, str]:
    """Get API headers with authentication if available."""
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    return headers


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
    # Build the API endpoint
    if env_name:
        if version:
            env_id = f"{env_name}@{version}"
        else:
            env_id = env_name
    else:
        env_id = "*"

    url = f"{system_url}/env-instance/{env_id}/list"
    headers = _get_api_headers()

    try:
        response = requests.get(url, headers=headers, timeout=30)
        response.raise_for_status()

        result = response.json()
        if result.get("success") and result.get("data"):
            return result["data"]
        return []
    except requests.exceptions.RequestException as e:
        raise click.ClickException(f"Failed to query instances: {str(e)}")


def _get_instance_from_api(system_url: str, instance_id: str) -> Optional[dict]:
    """Get detailed information for a single instance.

    Args:
        system_url: AEnv system URL
        instance_id: Instance ID

    Returns:
        Instance details dict or None if failed
    """
    url = f"{system_url}/env-instance/{instance_id}"
    headers = _get_api_headers()

    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()

        result = response.json()
        if result.get("success") and result.get("data"):
            return result["data"]
        return None
    except requests.exceptions.RequestException as e:
        raise click.ClickException(f"Failed to get instance info: {str(e)}")


def _delete_instance_from_api(system_url: str, instance_id: str) -> bool:
    """Delete an instance via API.

    Args:
        system_url: AEnv system URL
        instance_id: Instance ID

    Returns:
        True if deletion successful
    """
    url = f"{system_url}/env-instance/{instance_id}"
    headers = _get_api_headers()

    try:
        response = requests.delete(url, headers=headers, timeout=30)
        response.raise_for_status()

        result = response.json()
        return result.get("success", False)
    except requests.exceptions.RequestException as e:
        raise click.ClickException(f"Failed to delete instance: {str(e)}")


async def _deploy_instance(
    env_name: str,
    datasource: str,
    ttl: str,
    environment_variables: Dict[str, str],
    arguments: list,
    aenv_url: Optional[str],
    timeout: float,
    startup_timeout: float,
    max_retries: int,
    api_key: Optional[str],
    skip_health: bool,
) -> Environment:
    """Deploy a new environment instance.

    Returns:
        Environment object
    """
    env = Environment(
        env_name=env_name,
        datasource=datasource,
        ttl=ttl,
        environment_variables=environment_variables,
        arguments=arguments,
        aenv_url=aenv_url,
        timeout=timeout,
        startup_timeout=startup_timeout,
        max_retries=max_retries,
        api_key=api_key,
        skip_for_healthy=skip_health,
    )

    await env.initialize()
    return env


async def _get_instance_info(env: Environment) -> Dict[str, Any]:
    """Get environment instance information.

    Args:
        env: Environment object

    Returns:
        Dictionary with instance information
    """
    return await env.get_env_info()


async def _stop_instance(env: Environment):
    """Stop and release environment instance.

    Args:
        env: Environment object
    """
    await env.release()


@click.group("instance")
@pass_config
def instance(cfg: Config):
    """Manage environment instances

    Manage the lifecycle of environment instances including creation,
    querying, and deletion.
    """
    pass


@instance.command("create")
@click.argument("env_name")
@click.option(
    "--datasource",
    "-d",
    default="",
    help="Data source for mounting on the MCP server",
)
@click.option(
    "--ttl",
    "-t",
    default="30m",
    help="Time to live for the instance (e.g., 30m, 1h, 2h)",
)
@click.option(
    "--env",
    "-e",
    "environment_variables",
    multiple=True,
    help="Environment variables in format KEY=VALUE (can be used multiple times)",
)
@click.option(
    "--arg",
    "-a",
    "arguments",
    multiple=True,
    help="Command line arguments for the instance entrypoint (can be used multiple times)",
)
@click.option(
    "--system-url",
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var)",
)
@click.option(
    "--timeout",
    type=float,
    default=60.0,
    help="Request timeout in seconds",
)
@click.option(
    "--startup-timeout",
    type=float,
    default=500.0,
    help="Startup timeout in seconds",
)
@click.option(
    "--max-retries",
    type=int,
    default=10,
    help="Maximum retry attempts for failed requests",
)
@click.option(
    "--api-key",
    help="API key for authentication (defaults to AENV_API_KEY env var)",
)
@click.option(
    "--skip-health",
    is_flag=True,
    help="Skip health check during initialization",
)
@click.option(
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@click.option(
    "--keep-alive",
    is_flag=True,
    help="Keep the instance running after deployment (doesn't auto-release)",
)
@pass_config
def create(
    cfg: Config,
    env_name: str,
    datasource: str,
    ttl: str,
    environment_variables: tuple,
    arguments: tuple,
    system_url: Optional[str],
    timeout: float,
    startup_timeout: float,
    max_retries: int,
    api_key: Optional[str],
    skip_health: bool,
    output: str,
    keep_alive: bool,
):
    """Create a new environment instance

    Create and initialize a new environment instance with the specified configuration.

    Examples:
        # Create a basic instance
        aenv instance create flowise-xxx@1.0.2

        # Create with custom TTL and environment variables
        aenv instance create flowise-xxx@1.0.2 --ttl 1h -e DB_HOST=localhost -e DB_PORT=5432

        # Create with arguments and skip health check
        aenv instance create flowise-xxx@1.0.2 --arg --debug --arg --verbose --skip-health

        # Create and keep alive (doesn't auto-release)
        aenv instance create flowise-xxx@1.0.2 --keep-alive
    """
    console = cfg.console.console()

    # Parse environment variables and arguments
    try:
        env_vars = _parse_env_vars(environment_variables)
        args = _parse_arguments(arguments)
    except click.BadParameter as e:
        console.print(f"[red]Error:[/red] {str(e)}")
        raise click.Abort()

    # Get API key from env if not provided
    if not api_key:
        api_key = os.getenv("AENV_API_KEY")

    # Get system URL from env if not provided
    if not system_url:
        system_url = os.getenv("AENV_SYSTEM_URL")

    console.print(f"[cyan]🚀 Deploying environment instance:[/cyan] {env_name}")
    if datasource:
        console.print(f"   Datasource: {datasource}")
    console.print(f"   TTL: {ttl}")
    if env_vars:
        console.print(f"   Environment Variables: {len(env_vars)} variables")
    if args:
        console.print(f"   Arguments: {len(args)} arguments")
    console.print()

    try:
        # Deploy the instance
        with console.status("[bold green]Deploying instance..."):
            env = asyncio.run(
                _deploy_instance(
                    env_name=env_name,
                    datasource=datasource,
                    ttl=ttl,
                    environment_variables=env_vars,
                    arguments=args,
                    aenv_url=system_url,
                    timeout=timeout,
                    startup_timeout=startup_timeout,
                    max_retries=max_retries,
                    api_key=api_key,
                    skip_health=skip_health,
                )
            )

        # Get instance info
        info = asyncio.run(_get_instance_info(env))

        console.print("[green]✅ Instance deployed successfully![/green]\n")

        # Display instance information
        if output == "json":
            console.print(json.dumps(info, indent=2, ensure_ascii=False))
        else:
            table_data = [
                {"Property": "Instance ID", "Value": info.get("instance_id", "-")},
                {"Property": "Environment", "Value": info.get("name", "-")},
                {"Property": "Status", "Value": info.get("status", "-")},
                {"Property": "IP Address", "Value": info.get("ip", "-")},
                {"Property": "Created At", "Value": info.get("created_at", "-")},
            ]
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))

        # Store instance reference for potential cleanup
        if not keep_alive:
            console.print("\n[yellow]⚠️  Instance will be released when the command exits[/yellow]")
            console.print("[yellow]   Use --keep-alive flag to keep the instance running[/yellow]")
            # Release the instance
            asyncio.run(_stop_instance(env))
            console.print("[green]✅ Instance released[/green]")
        else:
            console.print("\n[green]✅ Instance is running and will stay alive[/green]")
            console.print(f"[cyan]Instance ID:[/cyan] {info.get('instance_id')}")
            console.print(f"[cyan]Use 'aenv instances' to view all running instances[/cyan]")

    except Exception as e:
        console.print(f"[red]❌ Deployment failed:[/red] {str(e)}")
        if cfg.verbose:
            import traceback
            console.print(traceback.format_exc())
        raise click.Abort()


@instance.command("info")
@click.argument("env_name")
@click.option(
    "--system-url",
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var)",
)
@click.option(
    "--timeout",
    type=float,
    default=60.0,
    help="Request timeout in seconds",
)
@click.option(
    "--api-key",
    help="API key for authentication (defaults to AENV_API_KEY env var)",
)
@click.option(
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@pass_config
def info(
    cfg: Config,
    env_name: str,
    system_url: Optional[str],
    timeout: float,
    api_key: Optional[str],
    output: str,
):
    """Get information about a deployed instance

    Retrieve detailed information about a running environment instance.
    Note: This command requires an active instance. Use with DUMMY_INSTANCE_IP
    environment variable for testing.

    Examples:
        # Get info for a test instance
        DUMMY_INSTANCE_IP=localhost aenv instance info flowise-xxx@1.0.2

        # Get info in JSON format
        DUMMY_INSTANCE_IP=localhost aenv instance info flowise-xxx@1.0.2 --output json
    """
    console = cfg.console.console()

    # Get API key from env if not provided
    if not api_key:
        api_key = os.getenv("AENV_API_KEY")

    # Get system URL from env if not provided
    if not system_url:
        system_url = os.getenv("AENV_SYSTEM_URL")

    console.print(f"[cyan]ℹ️  Retrieving instance information:[/cyan] {env_name}\n")

    try:
        # Create environment instance (will use DUMMY_INSTANCE_IP if set)
        with console.status("[bold green]Connecting to instance..."):
            env = Environment(
                env_name=env_name,
                aenv_url=system_url,
                timeout=timeout,
                api_key=api_key,
                skip_for_healthy=True,
            )
            asyncio.run(env.initialize())
            info = asyncio.run(_get_instance_info(env))

        console.print("[green]✅ Instance information retrieved![/green]\n")

        # Display instance information
        if output == "json":
            console.print(json.dumps(info, indent=2, ensure_ascii=False))
        else:
            table_data = [
                {"Property": "Instance ID", "Value": info.get("instance_id", "-")},
                {"Property": "Environment", "Value": info.get("name", "-")},
                {"Property": "Status", "Value": info.get("status", "-")},
                {"Property": "IP Address", "Value": info.get("ip", "-")},
                {"Property": "Created At", "Value": info.get("created_at", "-")},
                {"Property": "Updated At", "Value": info.get("updated_at", "-")},
            ]
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))

        # Release the environment
        asyncio.run(_stop_instance(env))

    except Exception as e:
        console.print(f"[red]❌ Failed to get instance information:[/red] {str(e)}")
        if cfg.verbose:
            import traceback
            console.print(traceback.format_exc())
        raise click.Abort()


@instance.command("list")
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
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@click.option(
    "--system-url",
    type=str,
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var)",
)
@pass_config
def list_instances(cfg: Config, name, version, output, system_url):
    """List running environment instances

    Query and display running environment instances. Can filter by environment
    name and optionally by version.

    Examples:
        # List all running instances
        aenv instance list

        # List instances for a specific environment
        aenv instance list --name my-env

        # List instances for a specific environment and version
        aenv instance list --name my-env --version 1.0.0

        # Output as JSON
        aenv instance list --output json

        # Use custom system URL
        aenv instance list --system-url http://api.example.com:8080
    """
    console = cfg.console.console()

    if version and not name:
        raise click.BadOptionUsage(
            "--version", "Version filter requires --name to be specified"
        )

    # Get system URL
    if not system_url:
        system_url = _get_system_url()
    else:
        system_url = _make_api_url(system_url, port=8080)

    try:
        instances_list = _list_instances_from_api(system_url, name, version)
    except Exception as e:
        console.print(f"[red]❌ Failed to list instances:[/red] {str(e)}")
        if cfg.verbose:
            import traceback
            console.print(traceback.format_exc())
        raise click.Abort()

    if not instances_list:
        if name:
            if version:
                console.print(f"📭 No running instances found for {name}@{version}")
            else:
                console.print(f"📭 No running instances found for {name}")
        else:
            console.print("📭 No running instances found")
        return

    if output == "json":
        console.print(json.dumps(instances_list, indent=2, ensure_ascii=False))
    elif output == "table":
        # Prepare table data
        table_data = []
        for instance in instances_list:
            instance_id = instance.get("id", "")
            if not instance_id:
                continue

            # Try to get detailed info for each instance
            detailed_info = _get_instance_from_api(system_url, instance_id)

            # Use detailed info if available, otherwise use list data
            if detailed_info:
                env_info = detailed_info.get("env") or {}
            else:
                env_info = instance.get("env") or {}

            env_name = env_info.get("name") if env_info else None
            env_version = env_info.get("version") if env_info else None

            # If env is None, try to extract from instance ID
            if not env_name and instance_id:
                parts = instance_id.split("-")
                if len(parts) >= 2:
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

            # Get created_at from list data
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
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))
        else:
            console.print("📭 No running instances found")


@instance.command("get")
@click.argument("instance_id")
@click.option(
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@click.option(
    "--system-url",
    type=str,
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var)",
)
@pass_config
def get_instance(cfg: Config, instance_id, output, system_url):
    """Get detailed information for a specific instance

    Retrieve detailed information about a running environment instance by its ID.

    Examples:
        # Get instance information
        aenv instance get flowise-xxx-abc123

        # Get instance information in JSON format
        aenv instance get flowise-xxx-abc123 --output json
    """
    console = cfg.console.console()

    # Get system URL
    if not system_url:
        system_url = _get_system_url()
    else:
        system_url = _make_api_url(system_url, port=8080)

    console.print(f"[cyan]ℹ️  Retrieving instance information:[/cyan] {instance_id}\n")

    try:
        instance_info = _get_instance_from_api(system_url, instance_id)

        if not instance_info:
            console.print(f"[red]❌ Instance not found:[/red] {instance_id}")
            raise click.Abort()

        console.print("[green]✅ Instance information retrieved![/green]\n")

        if output == "json":
            console.print(json.dumps(instance_info, indent=2, ensure_ascii=False))
        else:
            # Extract environment info
            env_info = instance_info.get("env") or {}
            env_name = env_info.get("name") or "-"
            env_version = env_info.get("version") or "-"

            table_data = [
                {"Property": "Instance ID", "Value": instance_info.get("id", "-")},
                {"Property": "Environment", "Value": env_name},
                {"Property": "Version", "Value": env_version},
                {"Property": "Status", "Value": instance_info.get("status", "-")},
                {"Property": "IP Address", "Value": instance_info.get("ip", "-")},
                {"Property": "Created At", "Value": instance_info.get("created_at", "-")},
                {"Property": "Updated At", "Value": instance_info.get("updated_at", "-")},
            ]
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))

    except click.Abort:
        raise
    except Exception as e:
        console.print(f"[red]❌ Failed to get instance information:[/red] {str(e)}")
        if cfg.verbose:
            import traceback
            console.print(traceback.format_exc())
        raise click.Abort()


@instance.command("delete")
@click.argument("instance_id")
@click.option(
    "--yes",
    "-y",
    is_flag=True,
    help="Skip confirmation prompt",
)
@click.option(
    "--system-url",
    type=str,
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var)",
)
@pass_config
def delete_instance(cfg: Config, instance_id, yes, system_url):
    """Delete a running instance

    Delete a running environment instance by its ID.

    Examples:
        # Delete an instance (with confirmation)
        aenv instance delete flowise-xxx-abc123

        # Delete an instance (skip confirmation)
        aenv instance delete flowise-xxx-abc123 --yes
    """
    console = cfg.console.console()

    # Get system URL
    if not system_url:
        system_url = _get_system_url()
    else:
        system_url = _make_api_url(system_url, port=8080)

    # Confirm deletion unless --yes flag is provided
    if not yes:
        console.print(f"[yellow]⚠️  You are about to delete instance:[/yellow] {instance_id}")
        if not click.confirm("Are you sure you want to continue?"):
            console.print("[cyan]Deletion cancelled[/cyan]")
            return

    console.print(f"[cyan]🗑️  Deleting instance:[/cyan] {instance_id}\n")

    try:
        with console.status("[bold green]Deleting instance..."):
            success = _delete_instance_from_api(system_url, instance_id)

        if success:
            console.print("[green]✅ Instance deleted successfully![/green]")
        else:
            console.print("[red]❌ Failed to delete instance[/red]")
            raise click.Abort()

    except click.Abort:
        raise
    except Exception as e:
        console.print(f"[red]❌ Failed to delete instance:[/red] {str(e)}")
        if cfg.verbose:
            import traceback
            console.print(traceback.format_exc())
        raise click.Abort()
