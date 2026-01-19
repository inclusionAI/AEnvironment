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
from pathlib import Path
from typing import Any, Dict, Optional, Tuple

import click
import requests

from aenv.core.environment import Environment
from cli.cmds.common import Config, pass_config
from cli.utils.api_helpers import (
    format_time_to_local,
    get_api_headers,
    get_system_url_raw,
    make_api_url,
)
from cli.utils.api_helpers import parse_env_vars as _parse_env_vars
from cli.utils.cli_config import get_config_manager
from cli.utils.table_formatter import print_detail_table, print_instance_list


def _parse_arguments(arg_list: tuple) -> list:
    """Parse command line arguments.

    Args:
        arg_list: Tuple of argument strings

    Returns:
        List of arguments
    """
    return list(arg_list) if arg_list else []


def _load_env_config() -> Optional[Dict[str, Any]]:
    """Load build configuration from config.json in current directory.

    Returns:
        Dictionary containing build configuration, or None if not found.
    """
    config_path = Path(".").resolve() / "config.json"
    if not config_path.exists():
        return None

    try:
        with open(config_path, "r") as f:
            config = json.load(f)
        return config
    except Exception:
        return None


def _split_env_name_version(env_name: str) -> Tuple[str, str]:
    """Split environment name into name and version.

    Args:
        env_name: Environment name in format "name@version" or just "name"

    Returns:
        Tuple of (name, version). If no @ symbol, version is empty string.
    """
    if not env_name:
        return "", ""

    parts = env_name.split("@", 1)
    if len(parts) == 1:
        # No @ symbol, use entire string as name
        return parts[0], ""
    else:
        # Has @ symbol, first part as name, second part as version
        return parts[0], parts[1]


def _get_system_url() -> str:
    """Get AEnv system URL from environment variable or config (processed for API).

    Priority order:
    1. AENV_SYSTEM_URL environment variable (highest priority)
    2. system_url in config file
    3. Default value (http://localhost:8080)

    Returns:
        Processed API URL with port
    """
    system_url = get_system_url_raw()

    # Use default if still not found
    if not system_url:
        system_url = "http://localhost:8080"

    return make_api_url(system_url, port=8080)


def _list_instances_from_api(
    system_url: str,
    env_name: Optional[str] = None,
    version: Optional[str] = None,
    verbose: bool = False,
    console=None,
) -> list:
    """List running instances from API service.

    Args:
        system_url: AEnv system URL
        env_name: Optional environment name filter
        version: Optional version filter
        verbose: Enable debug logging
        console: Console object for logging (if verbose)

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
    headers = get_api_headers()

    # Add query parameters
    params = {}

    # Debug logging
    if verbose and console:
        console.print(f"[dim]🔍 Debug: Request URL: {url}[/dim]")
        console.print(
            f"[dim]🔍 Debug: Query params: {params if params else 'none'}[/dim]"
        )
        # Don't log full headers for security, but show if auth is present
        has_auth = "Authorization" in headers
        console.print(
            f"[dim]🔍 Debug: Headers: Content-Type={headers.get('Content-Type')}, "
            f"Authorization={'present' if has_auth else 'not set'}[/dim]"
        )

    try:
        if verbose and console:
            console.print("[dim]🔍 Debug: Sending GET request...[/dim]")

        response = requests.get(url, headers=headers, params=params, timeout=30)

        if verbose and console:
            console.print(
                f"[dim]🔍 Debug: Response status: {response.status_code}[/dim]"
            )
            console.print(
                f"[dim]🔍 Debug: Response headers: {dict(response.headers)}[/dim]"
            )

        # Check for HTTP errors
        if response.status_code == 403:
            error_detail = ""
            if verbose and console:
                try:
                    error_body = response.json()
                    error_detail = f"\nResponse: {error_body}"
                except BaseException:
                    error_detail = f"\nResponse body: {response.text[:200]}"
            raise click.ClickException(
                "Authentication failed (403). Please check your API key configuration.\n"
                "You can set it with: aenv config set hub_config.api_key <your-key>\n"
                "Or use AENV_API_KEY environment variable." + error_detail
            )
        elif response.status_code == 401:
            error_detail = ""
            if verbose and console:
                try:
                    error_body = response.json()
                    error_detail = f"\nResponse: {error_body}"
                except BaseException:
                    error_detail = f"\nResponse body: {response.text[:200]}"
            raise click.ClickException(
                "Unauthorized (401). Invalid or missing API key.\n"
                "Please check your API key configuration." + error_detail
            )

        response.raise_for_status()

        result = response.json()

        if verbose and console:
            console.print(
                f"[dim]🔍 Debug: Response body (success): {result.get('success')}[/dim]"
            )
            console.print(
                f"[dim]🔍 Debug: Response body (code): {result.get('code')}[/dim]"
            )
            if result.get("data"):
                data_len = (
                    len(result.get("data", []))
                    if isinstance(result.get("data"), list)
                    else 1
                )
                console.print(
                    f"[dim]🔍 Debug: Response data: {data_len} item(s) returned[/dim]"
                )
            else:
                console.print("[dim]🔍 Debug: Response data: empty or null[/dim]")

        if result.get("success") and result.get("data"):
            instances = result["data"]

            if verbose and console:
                console.print(
                    f"[dim]🔍 Debug: Found {len(instances)} instance(s)[/dim]"
                )

            return instances
        elif not result.get("success"):
            error_msg = result.get("message", "Unknown error")
            if verbose and console:
                console.print(f"[dim]🔍 Debug: API returned error: {error_msg}[/dim]")
            raise click.ClickException(f"API returned error: {error_msg}")

        if verbose and console:
            console.print(
                "[dim]🔍 Debug: No data in response, returning empty list[/dim]"
            )

        return []
    except requests.exceptions.ConnectionError as e:
        error_msg = (
            f"Failed to connect to API service at {system_url}.\n"
            f"Please check:\n"
            f"  1. Is the API service running?\n"
            f"  2. Is the system_url correct? (current: {system_url})\n"
            f"  3. You can set it with: aenv config set system_url <url>\n"
            f"  4. Or use AENV_SYSTEM_URL environment variable.\n"
            f"Error: {str(e)}"
        )
        if verbose and console:
            console.print(f"[dim]🔍 Debug: Connection error details: {str(e)}[/dim]")
        raise click.ClickException(error_msg)
    except requests.exceptions.Timeout:
        error_msg = (
            f"Request timeout while connecting to {system_url}.\n"
            f"The API service may be slow or unreachable."
        )
        if verbose and console:
            console.print("[dim]🔍 Debug: Timeout after 30 seconds[/dim]")
        raise click.ClickException(error_msg)
    except requests.exceptions.RequestException as e:
        error_msg = f"Failed to query instances: {str(e)}"
        if verbose and console:
            console.print(f"[dim]🔍 Debug: Request exception: {str(e)}[/dim]")
            import traceback

            console.print(f"[dim]🔍 Debug: Traceback:\n{traceback.format_exc()}[/dim]")
        raise click.ClickException(error_msg)


def _get_instance_from_api(
    system_url: str,
    instance_id: str,
    verbose: bool = False,
    console=None,
) -> Optional[dict]:
    """Get detailed information for a single instance.

    Args:
        system_url: AEnv system URL
        instance_id: Instance ID
        verbose: Enable debug logging
        console: Console object for logging (if verbose)

    Returns:
        Instance details dict or None if failed
    """
    url = f"{system_url}/env-instance/{instance_id}"
    headers = get_api_headers()

    # Debug logging
    if verbose and console:
        console.print(f"[dim]🔍 Debug: Request URL: {url}[/dim]")
        has_auth = "Authorization" in headers
        console.print(
            f"[dim]🔍 Debug: Headers: Content-Type={headers.get('Content-Type')}, "
            f"Authorization={'present' if has_auth else 'not set'}[/dim]"
        )

    try:
        if verbose and console:
            console.print("[dim]🔍 Debug: Sending GET request...[/dim]")

        response = requests.get(url, headers=headers, timeout=10)

        if verbose and console:
            console.print(
                f"[dim]🔍 Debug: Response status: {response.status_code}[/dim]"
            )
            console.print(
                f"[dim]🔍 Debug: Response headers: {dict(response.headers)}[/dim]"
            )

        # Check for HTTP errors
        if response.status_code == 403:
            error_detail = ""
            if verbose and console:
                try:
                    error_body = response.json()
                    error_detail = f"\nResponse: {error_body}"
                except BaseException:
                    error_detail = f"\nResponse body: {response.text[:200]}"
            raise click.ClickException(
                "Authentication failed (403). Please check your API key configuration.\n"
                "You can set it with: aenv config set hub_config.api_key <your-key>\n"
                "Or use AENV_API_KEY environment variable." + error_detail
            )
        elif response.status_code == 401:
            error_detail = ""
            if verbose and console:
                try:
                    error_body = response.json()
                    error_detail = f"\nResponse: {error_body}"
                except BaseException:
                    error_detail = f"\nResponse body: {response.text[:200]}"
            raise click.ClickException(
                "Unauthorized (401). Invalid or missing API key.\n"
                "Please check your API key configuration." + error_detail
            )

        response.raise_for_status()

        result = response.json()

        if verbose and console:
            console.print(
                f"[dim]🔍 Debug: Response body (success): {result.get('success')}[/dim]"
            )
            console.print(
                f"[dim]🔍 Debug: Response body (code): {result.get('code')}[/dim]"
            )
            if result.get("data"):
                console.print("[dim]🔍 Debug: Response data: instance found[/dim]")
            else:
                console.print("[dim]🔍 Debug: Response data: empty or null[/dim]")

        if result.get("success") and result.get("data"):
            return result["data"]
        elif not result.get("success"):
            error_msg = result.get("message", "Unknown error")
            if verbose and console:
                console.print(f"[dim]🔍 Debug: API returned error: {error_msg}[/dim]")
            raise click.ClickException(f"API returned error: {error_msg}")
        return None
    except requests.exceptions.HTTPError as e:
        # Extract error details from response body
        error_detail = ""
        if hasattr(e.response, "text"):
            try:
                error_body = e.response.json()
                if isinstance(error_body, dict):
                    error_msg = (
                        error_body.get("message")
                        or error_body.get("error")
                        or str(error_body)
                    )
                    error_detail = f": {error_msg}"
                else:
                    error_detail = f": {str(error_body)[:200]}"
            except BaseException:
                error_detail = f": {e.response.text[:200]}"

        error_msg = f"Failed to get instance info: {str(e)}{error_detail}"
        if verbose and console:
            console.print(f"[dim]🔍 Debug: HTTP error details: {error_detail}[/dim]")
        raise click.ClickException(error_msg)
    except requests.exceptions.ConnectionError as e:
        error_msg = (
            f"Failed to connect to API service at {system_url}.\n"
            f"Please check:\n"
            f"  1. Is the API service running?\n"
            f"  2. Is the system_url correct? (current: {system_url})\n"
            f"  3. You can set it with: aenv config set system_url <url>\n"
            f"  4. Or use AENV_SYSTEM_URL environment variable.\n"
            f"Error: {str(e)}"
        )
        if verbose and console:
            console.print(f"[dim]🔍 Debug: Connection error details: {str(e)}[/dim]")
        raise click.ClickException(error_msg)
    except requests.exceptions.Timeout:
        error_msg = (
            f"Request timeout while connecting to {system_url}.\n"
            f"The API service may be slow or unreachable."
        )
        if verbose and console:
            console.print("[dim]🔍 Debug: Timeout after 10 seconds[/dim]")
        raise click.ClickException(error_msg)
    except requests.exceptions.RequestException as e:
        error_msg = f"Failed to get instance info: {str(e)}"
        if verbose and console:
            console.print(f"[dim]🔍 Debug: Request exception: {str(e)}[/dim]")
            import traceback

            console.print(f"[dim]🔍 Debug: Traceback:\n{traceback.format_exc()}[/dim]")
        raise click.ClickException(error_msg)


def _delete_instance_from_api(system_url: str, instance_id: str) -> bool:
    """Delete an instance via API.

    Args:
        system_url: AEnv system URL
        instance_id: Instance ID

    Returns:
        True if deletion successful
    """
    url = f"{system_url}/env-instance/{instance_id}"
    headers = get_api_headers()

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
    owner: Optional[str],
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
        owner=owner,
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
@click.argument("env_name", required=False)
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
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var or config)",
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
@click.option(
    "--owner",
    type=str,
    help="Owner of the instance (defaults to owner in config if not specified)",
)
@pass_config
def create(
    cfg: Config,
    env_name: Optional[str],
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
    owner: Optional[str],
):
    """Create a new environment instance

    Create and initialize a new environment instance with the specified configuration.

    The env_name argument is optional. If not provided, it will be read from config.json
    in the current directory.

    Examples:
        # Create using config.json in current directory
        aenv instance create

        # Create with explicit environment name
        aenv instance create flowise-xxx@1.0.2

        # Create with custom TTL and environment variables
        aenv instance create flowise-xxx@1.0.2 --ttl 1h -e DB_HOST=localhost -e DB_PORT=5432

        # Create with arguments and skip health check
        aenv instance create flowise-xxx@1.0.2 --arg --debug --arg --verbose --skip-health

        # Create and keep alive (doesn't auto-release)
        aenv instance create flowise-xxx@1.0.2 --keep-alive
    """
    console = cfg.console.console()

    # If env_name not provided, try to load from config.json
    if not env_name:
        config = _load_env_config()
        if config and "name" in config and "version" in config:
            env_name = f"{config['name']}@{config['version']}"
            console.print(f"[dim]📄 Reading from config.json: {env_name}[/dim]\n")
        else:
            console.print(
                "[red]Error:[/red] env_name not provided and config.json not found or invalid.\n"
                "Either provide env_name as argument or ensure config.json exists in current directory."
            )
            raise click.Abort()

    # Parse environment variables and arguments
    try:
        env_vars = _parse_env_vars(environment_variables)
        args = _parse_arguments(arguments)
    except click.BadParameter as e:
        console.print(f"[red]Error:[/red] {str(e)}")
        raise click.Abort()

    # Parse env_name to extract name and version, and inject system environment variables
    env_name_parsed, env_version_parsed = _split_env_name_version(env_name)
    env_vars["envNAME"] = env_name_parsed
    env_vars["envversion"] = env_version_parsed

    # Get API key from env if not provided
    if not api_key:
        api_key = os.getenv("AENV_API_KEY")

    # Get system URL from env, config, or use default
    if not system_url:
        system_url = get_system_url_raw()

    # Get owner from command line, config, or None
    if not owner:
        config_manager = get_config_manager()
        owner = config_manager.get("owner")

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
                    owner=owner,
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
                {
                    "Property": "Created At",
                    "Value": format_time_to_local(info.get("created_at")),
                },
            ]
            print_detail_table(table_data, console, title="Instance Deployed")

        # Store instance reference for potential cleanup
        if not keep_alive:
            console.print(
                "\n[yellow]⚠️  Instance will be released when the command exits[/yellow]"
            )
            console.print(
                "[yellow]   Use --keep-alive flag to keep the instance running[/yellow]"
            )
            # Release the instance
            asyncio.run(_stop_instance(env))
            console.print("[green]✅ Instance released[/green]")
        else:
            console.print("\n[green]✅ Instance is running and will stay alive[/green]")
            console.print(f"[cyan]Instance ID:[/cyan] {info.get('instance_id')}")
            console.print(
                "[cyan]Use 'aenv instances' to view all running instances[/cyan]"
            )

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
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var or config)",
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

    # Get system URL from env, config, or use default
    if not system_url:
        system_url = get_system_url_raw()

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
                {
                    "Property": "Created At",
                    "Value": format_time_to_local(info.get("created_at")),
                },
                {
                    "Property": "Updated At",
                    "Value": format_time_to_local(info.get("updated_at")),
                },
            ]
            print_detail_table(table_data, console, title="Instance Information")

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
    help="AEnv system URL (defaults to AENV_SYSTEM_URL env var or config)",
)
@pass_config
def list_instances(cfg: Config, name, version, output, system_url):
    """List running environment instances

    Query and display running environment instances. Can filter by environment
    name and version.

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
        system_url = make_api_url(system_url, port=8080)

    # Use config-level verbose
    is_verbose = cfg.verbose

    # Debug: show configuration if verbose
    if is_verbose:
        console.print("[dim]🔍 Debug: Configuration[/dim]")
        console.print(f"[dim]  Using system URL: {system_url}[/dim]")
        config_manager = get_config_manager()
        config_url = config_manager.get("system_url")
        env_url = os.getenv("AENV_SYSTEM_URL")
        console.print(f"[dim]  Config system_url: {config_url or 'not set'}[/dim]")
        console.print(f"[dim]  Env AENV_SYSTEM_URL: {env_url or 'not set'}[/dim]")
        if name:
            console.print(f"[dim]  Filter by env_name: {name}[/dim]")
        if version:
            console.print(f"[dim]  Filter by version: {version}[/dim]")
        console.print()  # Empty line for readability

    try:
        instances_list = _list_instances_from_api(
            system_url,
            name,
            version,
            verbose=is_verbose,
            console=console if is_verbose else None,
        )
    except Exception as e:
        error_msg = str(e)

        # Parse and simplify error messages
        if "403" in error_msg or "401" in error_msg:
            console.print("[red]❌ Authentication failed[/red]")
            console.print("\n[dim]Please check your API key configuration.[/dim]")
            console.print(
                "[dim]You can set it with: [cyan]aenv config set hub_config.api_key <your-key>[/cyan][/dim]"
            )
        elif "connection" in error_msg.lower() or "timeout" in error_msg.lower():
            console.print("[red]❌ Connection failed[/red]")
            console.print(
                f"\n[dim]Cannot connect to the API service at: [cyan]{system_url}[/cyan][/dim]"
            )
            console.print(
                "[dim]Please check your network connection and system_url configuration.[/dim]"
            )
        else:
            console.print("[red]❌ Failed to list instances[/red]")
            console.print(f"\n[yellow]Error:[/yellow] {error_msg}")

        if is_verbose:
            console.print("\n[dim]--- Full error trace ---[/dim]")
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
        # Prepare data for the rich table formatter
        instances_data = []
        for instance in instances_list:
            instance_id = instance.get("id", "")
            if not instance_id:
                continue

            # Format the data for display
            instances_data.append(
                {
                    "id": instance_id,
                    "env": instance.get("env"),
                    "owner": instance.get("owner"),
                    "status": instance.get("status"),
                    "ip": instance.get("ip"),
                    "created_at": format_time_to_local(instance.get("created_at")),
                }
            )

        if instances_data:
            print_instance_list(instances_data, console)
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

        # Get instance information with verbose output
        aenv instance get flowise-xxx-abc123 --verbose
    """
    console = cfg.console.console()

    # Get system URL
    if not system_url:
        system_url = _get_system_url()
    else:
        system_url = make_api_url(system_url, port=8080)

    # Use config-level verbose
    is_verbose = cfg.verbose

    # Debug: show configuration if verbose
    if is_verbose:
        console.print("[dim]🔍 Debug: Configuration[/dim]")
        console.print(f"[dim]  Using system URL: {system_url}[/dim]")
        config_manager = get_config_manager()
        config_url = config_manager.get("system_url")
        env_url = os.getenv("AENV_SYSTEM_URL")
        console.print(f"[dim]  Config system_url: {config_url or 'not set'}[/dim]")
        console.print(f"[dim]  Env AENV_SYSTEM_URL: {env_url or 'not set'}[/dim]")
        console.print(f"[dim]  Instance ID: {instance_id}[/dim]")
        console.print()  # Empty line for readability

    console.print(f"[cyan]ℹ️  Retrieving instance information:[/cyan] {instance_id}\n")

    try:
        instance_info = _get_instance_from_api(
            system_url,
            instance_id,
            verbose=is_verbose,
            console=console if is_verbose else None,
        )

        if not instance_info:
            console.print(
                f"[red]❌ Instance not found:[/red] [yellow]{instance_id}[/yellow]"
            )
            console.print(
                "\n[dim]The instance does not exist or has been deleted.[/dim]"
            )
            console.print(
                "[dim]Use [cyan]aenv instance list[/cyan] to see available instances.[/dim]"
            )
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
                {
                    "Property": "Created At",
                    "Value": format_time_to_local(instance_info.get("created_at")),
                },
                {
                    "Property": "Updated At",
                    "Value": format_time_to_local(instance_info.get("updated_at")),
                },
            ]
            print_detail_table(table_data, console, title="Instance Details")

    except click.Abort:
        raise
    except Exception as e:
        error_msg = str(e).lower()

        # Parse and simplify error messages - focus on user-friendly messages
        if (
            ("404" in error_msg and "not found" in error_msg)
            or "pods" in error_msg
            or ("500" in error_msg and "not found" in error_msg)
        ):
            console.print(
                f"[red]❌ Instance not found:[/red] [yellow]{instance_id}[/yellow]"
            )
            console.print(
                "\n[dim]The instance does not exist or has been deleted.[/dim]"
            )
            console.print(
                "[dim]Use [cyan]aenv instance list[/cyan] to see available instances.[/dim]"
            )
        elif "403" in error_msg or "401" in error_msg:
            console.print("[red]❌ Authentication failed[/red]")
            console.print("\n[dim]Please check your API key configuration.[/dim]")
            console.print(
                "[dim]You can set it with: [cyan]aenv config set hub_config.api_key <your-key>[/cyan][/dim]"
            )
        elif "500" in error_msg and "internal server error" in error_msg:
            console.print("[red]❌ Server error occurred[/red]")
            console.print("\n[dim]The API service encountered an internal error.[/dim]")
            console.print(
                "[dim]Please try again or contact support if the issue persists.[/dim]"
            )
        elif "connection" in error_msg or "timeout" in error_msg:
            console.print("[red]❌ Connection failed[/red]")
            console.print(
                f"\n[dim]Cannot connect to the API service at: [cyan]{system_url}[/cyan][/dim]"
            )
            console.print(
                "[dim]Please check your network connection and system_url configuration.[/dim]"
            )
        else:
            console.print("[red]❌ Failed to get instance information[/red]")
            console.print(
                f"\n[dim]The instance [yellow]{instance_id}[/yellow] could not be retrieved.[/dim]"
            )
            console.print("[dim]It may have been deleted or never existed.[/dim]")

        if cfg.verbose:
            console.print(f"\n[dim]Technical details: {str(e)}[/dim]")
            import traceback

            console.print(f"[dim]{traceback.format_exc()}[/dim]")

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
        system_url = make_api_url(system_url, port=8080)

    # Confirm deletion unless --yes flag is provided
    if not yes:
        console.print(
            f"[yellow]⚠️  You are about to delete instance:[/yellow] {instance_id}"
        )
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
