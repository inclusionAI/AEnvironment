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
service command - Manage environment services (Deployment + Service + Storage)

This command provides interface for managing long-running services:
- service create: Create new services
- service list: List running services
- service get: Get detailed service information
- service delete: Delete a service
- service update: Update service (replicas, image, env vars)
"""
import asyncio
import json
import os
from pathlib import Path
from typing import Any, Dict, Optional

import click

from aenv.client.scheduler_client import AEnvSchedulerClient
from cli.cmds.common import Config, pass_config
from cli.utils.api_helpers import (
    format_time_to_local,
    get_system_url_raw,
    make_api_url,
    parse_env_vars,
)
from cli.utils.cli_config import get_config_manager
from cli.utils.table_formatter import print_detail_table, print_service_list


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

    # Ensure port is set for API communication
    return make_api_url(system_url, port=8080)


@click.group("service")
@pass_config
def service(cfg: Config):
    """Manage environment services (long-running deployments)

    Services are persistent deployments with:
    - Multiple replicas
    - Persistent storage
    - Cluster DNS service URL
    - No TTL (always running)
    """
    pass


@service.command("create")
@click.argument("env_name", required=False)
@click.option(
    "--service-name",
    "-s",
    type=str,
    help="Custom service name (default: auto-generated as envName-svc-random). Must follow Kubernetes DNS naming conventions (lowercase alphanumeric and hyphens).",
)
@click.option(
    "--replicas",
    "-r",
    type=int,
    help="Number of replicas (default: 1 or from config.json)",
)
@click.option(
    "--port",
    "-p",
    type=int,
    help="Service port (default: 8080 or from config.json)",
)
@click.option(
    "--env",
    "-e",
    "environment_variables",
    multiple=True,
    help="Environment variables in format KEY=VALUE (can be used multiple times)",
)
@click.option(
    "--enable-storage",
    is_flag=True,
    default=False,
    help="Enable storage. Storage configuration (storageSize, storageName, mountPath) will be read from config.json's deployConfig.service.",
)
@click.option(
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@pass_config
def create(
    cfg: Config,
    env_name: Optional[str],
    service_name: Optional[str],
    replicas: Optional[int],
    port: Optional[int],
    environment_variables: tuple,
    enable_storage: bool,
    output: str,
):
    """Create a new environment service

    Creates a long-running service with Deployment, Service, and optionally persistent storage.

    The env_name argument is optional. If not provided, it will be read from config.json
    in the current directory.

    Configuration priority (high to low):
    1. CLI parameters (--replicas, --port, --enable-storage)
    2. config.json's deployConfig.service (new structure)
    3. config.json's deployConfig (legacy flat structure, for backward compatibility)
    4. System defaults

    Storage creation behavior:
    - Use --enable-storage flag to enable persistent storage
    - Storage configuration (storageSize, storageName, mountPath) is read from config.json's deployConfig.service
    - When storage is created, replicas must be 1 (enforced by backend)
    - storageClass is configured in helm values.yaml deployment, not in config.json

    config.json deployConfig.service fields (new structure):
    - replicas: Number of replicas (default: 1)
    - port: Service port (default: 8080)
    - enableStorage: Enable storage by default (default: false, CLI --enable-storage overrides)
    - storageSize: Storage size like "10Gi", "20Gi" (required when --enable-storage is used)
    - storageName: Storage name (default: environment name)
    - mountPath: Mount path (default: /home/admin/data)

    config.json deployConfig fields (shared by both Pod and Service):
    - cpu: CPU resource (used as both request and limit, default: "1")
    - memory: Memory resource (used as both request and limit, default: "2Gi")
    - ephemeralStorage: Ephemeral storage (used as both request and limit, default: "5Gi")
    - environmentVariables: Environment variables dict

    Legacy config.json deployConfig fields (deprecated, kept for backward compatibility):
    - cpuRequest, cpuLimit: CPU resources (if not set, both use cpu value)
    - memoryRequest, memoryLimit: Memory resources (if not set, both use memory value)
    - ephemeralStorageRequest, ephemeralStorageLimit: Storage (if not set, both use ephemeralStorage value)

    Examples:
        # Create using config.json in current directory
        aenv service create

        # Create with explicit environment name
        aenv service create myapp@1.0.0

        # Create with 3 replicas and custom port (no storage)
        aenv service create myapp@1.0.0 --replicas 3 --port 8000

        # Create with storage enabled (storageSize must be in config.json)
        aenv service create myapp@1.0.0 --enable-storage

        # Create with environment variables
        aenv service create myapp@1.0.0 -e DB_HOST=postgres -e CACHE_SIZE=1024
    """
    console = cfg.console.console()

    # Load config.json if exists
    config = _load_env_config()
    deploy_config = config.get("deployConfig", {}) if config else {}

    # Get service config (support both new nested structure and legacy flat structure)
    service_config = deploy_config.get("service", {})
    # For backward compatibility, fall back to root deployConfig if service config is empty
    if not service_config:
        service_config = deploy_config

    # If env_name not provided, try to load from config.json
    if not env_name:
        if config and "name" in config and "version" in config:
            env_name = f"{config['name']}@{config['version']}"
            console.print(f"[dim]📄 Reading from config.json: {env_name}[/dim]\n")
        else:
            console.print(
                "[red]Error:[/red] env_name not provided and config.json not found or invalid.\n"
                "Either provide env_name as argument or ensure config.json exists in current directory."
            )
            raise click.Abort()

    # Validate service_name if provided (must follow Kubernetes DNS naming conventions)
    if service_name:
        import re

        # Kubernetes DNS-1123 subdomain: lowercase alphanumeric, hyphens, dots; max 253 chars; must start/end with alphanumeric
        dns_pattern = (
            r"^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
        )
        if not re.match(dns_pattern, service_name) or len(service_name) > 253:
            console.print(
                "[red]Error:[/red] Invalid service name. Service name must:\n"
                "  - Use only lowercase letters, numbers, hyphens, and dots\n"
                "  - Start and end with an alphanumeric character\n"
                "  - Be no longer than 253 characters\n"
                "  - Example: 'my-service', 'app-v1', 'web-frontend-prod'"
            )
            raise click.Abort()

    # Merge parameters: CLI > config.json > defaults
    final_replicas = (
        replicas if replicas is not None else service_config.get("replicas", 1)
    )
    final_port = port if port is not None else service_config.get("port")

    # Storage configuration - enabled by --enable-storage flag OR config.json enableStorage
    enable_storage_from_config = service_config.get("enableStorage", False)
    should_enable_storage = enable_storage or enable_storage_from_config

    final_storage_size = None
    final_storage_name = None
    final_mount_path = None
    if should_enable_storage:
        final_storage_size = service_config.get("storageSize")
        if not final_storage_size:
            console.print(
                "[red]Error:[/red] Storage is enabled but 'storageSize' is not found in config.json's deployConfig.service.\n"
                "Please add 'storageSize' (e.g., '10Gi', '20Gi') to deployConfig.service in config.json."
            )
            raise click.Abort()
        final_storage_name = service_config.get("storageName")
        final_mount_path = service_config.get("mountPath")

        # Validate replicas must be 1 when storage is enabled
        if final_replicas != 1:
            console.print(
                "[red]Error:[/red] When storage is enabled (enableStorage=true or --enable-storage), replicas must be 1.\n"
                f"Current replicas: {final_replicas}. Please set replicas to 1 in config.json or use --replicas 1."
            )
            raise click.Abort()

    # Resource configurations from deployConfig (kept at root level for backward compatibility)
    # These can be derived from simplified parameters (cpu, memory, ephemeralStorage)
    # Priority: explicit resource params > derived from simplified params > defaults
    cpu = deploy_config.get("cpu", "1")
    memory = deploy_config.get("memory", "2Gi")
    ephemeral_storage = deploy_config.get("ephemeralStorage", "5Gi")

    # Try explicit resource configs first (for backward compatibility with old configs)
    cpu_request = deploy_config.get("cpuRequest") or cpu
    cpu_limit = deploy_config.get("cpuLimit") or cpu
    memory_request = deploy_config.get("memoryRequest") or memory
    memory_limit = deploy_config.get("memoryLimit") or memory
    ephemeral_storage_request = (
        deploy_config.get("ephemeralStorageRequest") or ephemeral_storage
    )
    ephemeral_storage_limit = (
        deploy_config.get("ephemeralStorageLimit") or ephemeral_storage
    )

    # Parse environment variables from CLI
    try:
        env_vars = (
            parse_env_vars(environment_variables) if environment_variables else None
        )
    except click.BadParameter as e:
        console.print(f"[red]Error:[/red] {str(e)}")
        raise click.Abort()

    # Merge with environment variables from config
    if deploy_config.get("environmentVariables"):
        if env_vars is None:
            env_vars = {}
        # CLI env vars override config env vars
        for k, v in deploy_config["environmentVariables"].items():
            if k not in env_vars:
                env_vars[k] = v

    # Get config
    system_url = _get_system_url()
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    # Get owner from config (unified management)
    owner = config_manager.get("owner")

    # Display configuration summary
    console.print(f"[cyan]🚀 Creating environment service:[/cyan] {env_name}")
    if service_name:
        console.print(f"   Service Name: {service_name} (custom)")
    else:
        console.print("   Service Name: auto-generated")
    console.print(f"   Replicas: {final_replicas}")
    if final_port:
        console.print(f"   Port: {final_port}")
    if env_vars:
        console.print(f"   Environment Variables: {len(env_vars)} variables")
    if owner:
        console.print(f"   Owner: {owner}")

    if should_enable_storage:
        storage_source = "CLI flag" if enable_storage else "config.json"
        console.print(f"[cyan]   Storage Configuration (from {storage_source}):[/cyan]")
        console.print(f"     - Size: {final_storage_size}")
        if final_storage_name:
            console.print(f"     - Storage Name: {final_storage_name}")
        else:
            console.print(f"     - Storage Name: {env_name.split('@')[0]} (default)")
        if final_mount_path:
            console.print(f"     - Mount Path: {final_mount_path}")
        else:
            console.print("     - Mount Path: /home/admin/data (default)")
        console.print("   [yellow]⚠️  With storage enabled, replicas must be 1[/yellow]")
    else:
        console.print(
            "[dim]   Storage: Disabled (use --enable-storage to enable storage)[/dim]"
        )
    console.print()

    async def _create():
        async with AEnvSchedulerClient(
            base_url=system_url,
            api_key=api_key,
        ) as client:
            return await client.create_env_service(
                name=env_name,
                service_name=service_name,
                replicas=final_replicas,
                environment_variables=env_vars,
                owner=owner,
                port=final_port,
                pvc_name=final_storage_name,
                storage_size=final_storage_size,
                mount_path=final_mount_path,
                cpu_request=cpu_request,
                cpu_limit=cpu_limit,
                memory_request=memory_request,
                memory_limit=memory_limit,
                ephemeral_storage_request=ephemeral_storage_request,
                ephemeral_storage_limit=ephemeral_storage_limit,
            )

    try:
        with console.status("[bold green]Creating service..."):
            svc = asyncio.run(_create())

        console.print("[green]✅ Service created successfully![/green]\n")

        if output == "json":
            console.print(json.dumps(svc.model_dump(), indent=2, default=str))
        else:
            table_data = [
                {"Property": "Service ID", "Value": svc.id},
                {"Property": "Status", "Value": svc.status},
                {"Property": "Service URL", "Value": svc.service_url or "-"},
                {
                    "Property": "Replicas",
                    "Value": f"{svc.available_replicas}/{svc.replicas}",
                },
                {"Property": "Storage Name", "Value": svc.pvc_name or "-"},
                {
                    "Property": "Created At",
                    "Value": format_time_to_local(svc.created_at),
                },
            ]
            print_detail_table(table_data, console, title="Service Created")

    except Exception as e:
        console.print(f"[red]❌ Creation failed:[/red] {str(e)}")
        if cfg.verbose:
            import traceback

            console.print(traceback.format_exc())
        raise click.Abort()


@service.command("list")
@click.option(
    "--name",
    "-n",
    type=str,
    help="Filter by environment name",
)
@click.option(
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@pass_config
def list_services(cfg: Config, name, output):
    """List running environment services

    Examples:
        # List all services
        aenv service list

        # List services for specific environment
        aenv service list --name myapp

        # Output as JSON
        aenv service list --output json
    """
    console = cfg.console.console()

    system_url = _get_system_url()
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    async def _list():
        async with AEnvSchedulerClient(
            base_url=system_url,
            api_key=api_key,
        ) as client:
            return await client.list_env_services(env_name=name)

    try:
        services_list = asyncio.run(_list())
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
            console.print("\n[dim]Cannot connect to the API service.[/dim]")
            console.print(
                "[dim]Please check your network connection and system_url configuration.[/dim]"
            )
        else:
            console.print("[red]❌ Failed to list services[/red]")
            console.print(f"\n[yellow]Error:[/yellow] {error_msg}")

        if cfg.verbose:
            console.print("\n[dim]--- Full error trace ---[/dim]")
            import traceback

            console.print(traceback.format_exc())

        raise click.Abort()

    if not services_list:
        if name:
            console.print(f"📭 No running services found for {name}")
        else:
            console.print("📭 No running services found")
        return

    if output == "json":
        console.print(
            json.dumps([s.model_dump() for s in services_list], indent=2, default=str)
        )
    else:
        # Convert service objects to dictionaries for the formatter
        services_data = []
        for svc in services_list:
            env_info = {}
            if svc.env:
                env_info["name"] = svc.env.name
                env_info["version"] = svc.env.version

            services_data.append(
                {
                    "id": svc.id,
                    "env": env_info if env_info else None,
                    "owner": svc.owner,
                    "status": svc.status,
                    "available_replicas": svc.available_replicas,
                    "replicas": svc.replicas,
                    "storage_name": svc.pvc_name,
                    "created_at": format_time_to_local(svc.created_at),
                }
            )

        print_service_list(services_data, console)


@service.command("get")
@click.argument("service_id")
@click.option(
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@pass_config
def get_service(cfg: Config, service_id, output):
    """Get detailed information for a specific service

    Examples:
        # Get service information
        aenv service get myapp-svc-abc123

        # Get in JSON format
        aenv service get myapp-svc-abc123 --output json
    """
    console = cfg.console.console()

    system_url = _get_system_url()
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    console.print(f"[cyan]ℹ️  Retrieving service information:[/cyan] {service_id}\n")

    async def _get():
        async with AEnvSchedulerClient(
            base_url=system_url,
            api_key=api_key,
        ) as client:
            return await client.get_env_service(service_id)

    try:
        svc = asyncio.run(_get())

        console.print("[green]✅ Service information retrieved![/green]\n")

        if output == "json":
            console.print(json.dumps(svc.model_dump(), indent=2, default=str))
        else:
            env_name = svc.env.name if svc.env else "-"
            env_version = svc.env.version if svc.env else "-"

            table_data = [
                {"Property": "Service ID", "Value": svc.id},
                {"Property": "Environment", "Value": env_name},
                {"Property": "Version", "Value": env_version},
                {"Property": "Owner", "Value": svc.owner or "-"},
                {"Property": "Status", "Value": svc.status},
                {
                    "Property": "Replicas",
                    "Value": f"{svc.available_replicas}/{svc.replicas}",
                },
                {"Property": "Service URL", "Value": svc.service_url or "-"},
                {"Property": "Storage Name", "Value": svc.pvc_name or "-"},
                {
                    "Property": "Created At",
                    "Value": format_time_to_local(svc.created_at),
                },
                {
                    "Property": "Updated At",
                    "Value": format_time_to_local(svc.updated_at),
                },
            ]
            print_detail_table(table_data, console, title="Service Details")

    except Exception as e:
        error_msg = str(e).lower()

        # Parse and simplify error messages
        if (
            "404" in error_msg and "not found" in error_msg
        ) or "deployment" in error_msg:
            console.print(
                f"[red]❌ Service not found:[/red] [yellow]{service_id}[/yellow]"
            )
            console.print(
                "\n[dim]The service does not exist or has been deleted.[/dim]"
            )
            console.print(
                "[dim]Use [cyan]aenv service list[/cyan] to see available services.[/dim]"
            )
        elif "403" in error_msg or "401" in error_msg:
            console.print("[red]❌ Authentication failed[/red]")
            console.print("\n[dim]Please check your API key configuration.[/dim]")
            console.print(
                "[dim]You can set it with: [cyan]aenv config set hub_config.api_key <your-key>[/cyan][/dim]"
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
            console.print("[red]❌ Failed to get service information[/red]")
            console.print(
                f"\n[dim]The service [yellow]{service_id}[/yellow] could not be retrieved.[/dim]"
            )
            console.print("[dim]It may have been deleted or never existed.[/dim]")

        if cfg.verbose:
            console.print(f"\n[dim]Technical details: {str(e)}[/dim]")
            import traceback

            console.print(f"[dim]{traceback.format_exc()}[/dim]")

        raise click.Abort()


@service.command("delete")
@click.argument("service_id")
@click.option(
    "--yes",
    "-y",
    is_flag=True,
    help="Skip confirmation prompt",
)
@click.option(
    "--delete-storage",
    is_flag=True,
    help="Also delete the associated storage. Warning: This will permanently delete all data.",
)
@pass_config
def delete_service(cfg: Config, service_id, yes, delete_storage):
    """Delete a running service

    By default, this deletes the Deployment and Service, but keeps the storage for reuse.
    Use --delete-storage to also delete the storage and all associated data.

    Examples:
        # Delete a service (with confirmation), keep storage
        aenv service delete myapp-svc-abc123

        # Delete without confirmation
        aenv service delete myapp-svc-abc123 --yes

        # Delete service and storage
        aenv service delete myapp-svc-abc123 --delete-storage
    """
    console = cfg.console.console()

    if not yes:
        console.print(
            f"[yellow]⚠️  You are about to delete service:[/yellow] {service_id}"
        )
        if delete_storage:
            console.print(
                "[red]⚠️  WARNING: Storage will be PERMANENTLY deleted (all data will be lost)[/red]"
            )
        else:
            console.print("[yellow]Note: Storage will be kept for reuse[/yellow]")
        if not click.confirm("Are you sure you want to continue?"):
            console.print("[cyan]Deletion cancelled[/cyan]")
            return

    system_url = _get_system_url()
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    console.print(f"[cyan]🗑️  Deleting service:[/cyan] {service_id}")
    if delete_storage:
        console.print("[yellow]   Also deleting storage...[/yellow]")
    console.print()

    async def _delete():
        async with AEnvSchedulerClient(
            base_url=system_url,
            api_key=api_key,
        ) as client:
            return await client.delete_env_service(
                service_id, delete_storage=delete_storage
            )

    try:
        with console.status("[bold green]Deleting service..."):
            success = asyncio.run(_delete())

        if success:
            console.print("[green]✅ Service deleted successfully![/green]")
            if delete_storage:
                console.print("[cyan]Note: Storage was also deleted[/cyan]")
            else:
                console.print("[cyan]Note: Storage was kept for reuse[/cyan]")
        else:
            console.print("[red]❌ Failed to delete service[/red]")
            raise click.Abort()

    except Exception as e:
        console.print(f"[red]❌ Failed to delete service:[/red] {str(e)}")
        if cfg.verbose:
            import traceback

            console.print(traceback.format_exc())
        raise click.Abort()


@service.command("update")
@click.argument("service_id")
@click.option(
    "--replicas",
    "-r",
    type=int,
    help="Update number of replicas",
)
@click.option(
    "--image",
    type=str,
    help="Update container image",
)
@click.option(
    "--env",
    "-e",
    "environment_variables",
    multiple=True,
    help="Environment variables in format KEY=VALUE (can be used multiple times)",
)
@click.option(
    "--output",
    "-o",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
@pass_config
def update_service(
    cfg: Config,
    service_id: str,
    replicas: Optional[int],
    image: Optional[str],
    environment_variables: tuple,
    output: str,
):
    """Update a running service

    Can update replicas, image, and environment variables.

    Examples:
        # Scale to 5 replicas
        aenv service update myapp-svc-abc123 --replicas 5

        # Update image
        aenv service update myapp-svc-abc123 --image myapp:2.0.0

        # Update environment variables
        aenv service update myapp-svc-abc123 -e DB_HOST=newhost -e DB_PORT=3306

        # Update multiple things at once
        aenv service update myapp-svc-abc123 --replicas 3 --image myapp:2.0.0
    """
    console = cfg.console.console()

    if not replicas and not image and not environment_variables:
        console.print(
            "[red]Error:[/red] At least one of --replicas, --image, or --env must be provided"
        )
        raise click.Abort()

    # Parse environment variables
    env_vars = None
    if environment_variables:
        try:
            env_vars = parse_env_vars(environment_variables)
        except click.BadParameter as e:
            console.print(f"[red]Error:[/red] {str(e)}")
            raise click.Abort()

    system_url = _get_system_url()
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")

    console.print(f"[cyan]🔄 Updating service:[/cyan] {service_id}")
    if replicas is not None:
        console.print(f"   Replicas: {replicas}")
    if image:
        console.print(f"   Image: {image}")
    if env_vars:
        console.print(f"   Environment Variables: {len(env_vars)} variables")
    console.print()

    async def _update():
        async with AEnvSchedulerClient(
            base_url=system_url,
            api_key=api_key,
        ) as client:
            return await client.update_env_service(
                service_id=service_id,
                replicas=replicas,
                image=image,
                environment_variables=env_vars,
            )

    try:
        with console.status("[bold green]Updating service..."):
            svc = asyncio.run(_update())

        console.print("[green]✅ Service updated successfully![/green]\n")

        if output == "json":
            console.print(json.dumps(svc.model_dump(), indent=2, default=str))
        else:
            table_data = [
                {"Property": "Service ID", "Value": svc.id},
                {"Property": "Status", "Value": svc.status},
                {
                    "Property": "Replicas",
                    "Value": f"{svc.available_replicas}/{svc.replicas}",
                },
                {"Property": "Service URL", "Value": svc.service_url or "-"},
                {
                    "Property": "Updated At",
                    "Value": format_time_to_local(svc.updated_at),
                },
            ]
            print_detail_table(table_data, console, title="Service Updated")

    except Exception as e:
        console.print(f"[red]❌ Update failed:[/red] {str(e)}")
        if cfg.verbose:
            import traceback

            console.print(traceback.format_exc())
        raise click.Abort()
