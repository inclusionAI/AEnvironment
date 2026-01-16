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
service command - Manage environment services (Deployment + Service + PVC)

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
from tabulate import tabulate

from aenv.client.scheduler_client import AEnvSchedulerClient
from cli.cmds.common import Config, pass_config
from cli.utils.api_helpers import (
    get_api_headers,
    get_system_url_raw,
    make_api_url,
    parse_env_vars,
)
from cli.utils.cli_config import get_config_manager


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
    - Persistent storage (PVC)
    - Cluster DNS service URL
    - No TTL (always running)
    """
    pass


@service.command("create")
@click.argument("env_name", required=False)
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
    help="Enable PVC storage. Storage configuration (storageSize, pvcName, mountPath) will be read from config.json's deployConfig.",
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
    replicas: Optional[int],
    port: Optional[int],
    environment_variables: tuple,
    enable_storage: bool,
    output: str,
):
    """Create a new environment service

    Creates a long-running service with Deployment, Service, and optionally PVC.

    The env_name argument is optional. If not provided, it will be read from config.json
    in the current directory.

    Configuration priority (high to low):
    1. CLI parameters (--replicas, --port, --enable-storage)
    2. config.json's deployConfig
    3. System defaults

    PVC creation behavior:
    - Use --enable-storage flag to enable PVC
    - Storage configuration (storageSize, pvcName, mountPath) is read from config.json's deployConfig
    - When PVC is created, replicas must be 1 (enforced by backend)
    - storageClass is configured in helm values.yaml deployment, not in config.json

    config.json deployConfig fields:
    - replicas: Number of replicas (default: 1)
    - port: Service port (default: 8080)
    - storageSize: Storage size like "10Gi", "20Gi" (required when --enable-storage is used)
    - pvcName: PVC name (default: environment name)
    - mountPath: Mount path (default: /home/admin/data)
    - cpuRequest, cpuLimit: CPU resources (default: 1, 2)
    - memoryRequest, memoryLimit: Memory resources (default: 2Gi, 4Gi)
    - ephemeralStorageRequest, ephemeralStorageLimit: Storage (default: 5Gi, 10Gi)
    - environmentVariables: Environment variables dict

    Examples:
        # Create using config.json in current directory
        aenv service create

        # Create with explicit environment name
        aenv service create myapp@1.0.0

        # Create with 3 replicas and custom port (no PVC)
        aenv service create myapp@1.0.0 --replicas 3 --port 8000

        # Create with PVC enabled (storageSize must be in config.json)
        aenv service create myapp@1.0.0 --enable-storage

        # Create with environment variables
        aenv service create myapp@1.0.0 -e DB_HOST=postgres -e CACHE_SIZE=1024
    """
    console = cfg.console.console()

    # Load config.json if exists
    config = _load_env_config()
    deploy_config = config.get("deployConfig", {}) if config else {}

    # If env_name not provided, try to load from config.json
    if not env_name:
        if config and "name" in config and "version" in config:
            env_name = f"{config['name']}@{config['version']}"
            console.print(
                f"[dim]📄 Reading from config.json: {env_name}[/dim]\n"
            )
        else:
            console.print(
                "[red]Error:[/red] env_name not provided and config.json not found or invalid.\n"
                "Either provide env_name as argument or ensure config.json exists in current directory."
            )
            raise click.Abort()

    # Merge parameters: CLI > config.json > defaults
    final_replicas = replicas if replicas is not None else deploy_config.get("replicas", 1)
    final_port = port if port is not None else deploy_config.get("port")

    # Storage configuration - only use if --enable-storage is set
    final_storage_size = None
    final_pvc_name = None
    final_mount_path = None
    if enable_storage:
        final_storage_size = deploy_config.get("storageSize")
        if not final_storage_size:
            console.print(
                "[red]Error:[/red] --enable-storage flag is set but 'storageSize' is not found in config.json's deployConfig.\n"
                "Please add 'storageSize' (e.g., '10Gi', '20Gi') to deployConfig in config.json."
            )
            raise click.Abort()
        final_pvc_name = deploy_config.get("pvcName")
        final_mount_path = deploy_config.get("mountPath")

    # Resource configurations from deployConfig
    cpu_request = deploy_config.get("cpuRequest")
    cpu_limit = deploy_config.get("cpuLimit")
    memory_request = deploy_config.get("memoryRequest")
    memory_limit = deploy_config.get("memoryLimit")
    ephemeral_storage_request = deploy_config.get("ephemeralStorageRequest")
    ephemeral_storage_limit = deploy_config.get("ephemeralStorageLimit")

    # Parse environment variables from CLI
    try:
        env_vars = parse_env_vars(environment_variables) if environment_variables else None
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
    console.print(f"   Replicas: {final_replicas}")
    if final_port:
        console.print(f"   Port: {final_port}")
    if env_vars:
        console.print(f"   Environment Variables: {len(env_vars)} variables")
    if owner:
        console.print(f"   Owner: {owner}")

    if enable_storage:
        console.print(f"[cyan]   Storage Configuration:[/cyan]")
        console.print(f"     - Size: {final_storage_size}")
        if final_pvc_name:
            console.print(f"     - PVC Name: {final_pvc_name}")
        else:
            console.print(f"     - PVC Name: {env_name.split('@')[0]} (default)")
        if final_mount_path:
            console.print(f"     - Mount Path: {final_mount_path}")
        else:
            console.print(f"     - Mount Path: /home/admin/data (default)")
        console.print(f"   [yellow]⚠️  With PVC enabled, replicas must be 1[/yellow]")
    else:
        console.print(f"[dim]   Storage: Disabled (use --enable-storage to enable PVC)[/dim]")
    console.print()

    async def _create():
        async with AEnvSchedulerClient(
            base_url=system_url,
            api_key=api_key,
        ) as client:
            return await client.create_env_service(
                name=env_name,
                replicas=final_replicas,
                environment_variables=env_vars,
                owner=owner,
                port=final_port,
                pvc_name=final_pvc_name,
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
                {"Property": "Replicas", "Value": f"{svc.available_replicas}/{svc.replicas}"},
                {"Property": "PVC Name", "Value": svc.pvc_name or "-"},
                {"Property": "Created At", "Value": svc.created_at},
            ]
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))

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
        console.print(f"[red]❌ Failed to list services:[/red] {str(e)}")
        if cfg.verbose:
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
        console.print(json.dumps([s.model_dump() for s in services_list], indent=2, default=str))
    else:
        table_data = []
        for svc in services_list:
            env_name = svc.env.name if svc.env else "-"
            env_version = svc.env.version if svc.env else "-"
            
            table_data.append({
                "Service ID": svc.id,
                "Environment": env_name,
                "Version": env_version,
                "Owner": svc.owner or "-",
                "Status": svc.status,
                "Replicas": f"{svc.available_replicas}/{svc.replicas}",
                "Service URL": svc.service_url or "-",
                "Created At": svc.created_at,
            })
        
        if table_data:
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))
        else:
            console.print("📭 No running services found")


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
                {"Property": "Replicas", "Value": f"{svc.available_replicas}/{svc.replicas}"},
                {"Property": "Service URL", "Value": svc.service_url or "-"},
                {"Property": "PVC Name", "Value": svc.pvc_name or "-"},
                {"Property": "Created At", "Value": svc.created_at},
                {"Property": "Updated At", "Value": svc.updated_at},
            ]
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))
    
    except Exception as e:
        console.print(f"[red]❌ Failed to get service information:[/red] {str(e)}")
        if cfg.verbose:
            import traceback
            console.print(traceback.format_exc())
        raise click.Abort()


@service.command("delete")
@click.argument("service_id")
@click.option(
    "--yes",
    "-y",
    is_flag=True,
    help="Skip confirmation prompt",
)
@pass_config
def delete_service(cfg: Config, service_id, yes):
    """Delete a running service
    
    Note: This deletes the Deployment and Service, but keeps the PVC for reuse.
    
    Examples:
        # Delete a service (with confirmation)
        aenv service delete myapp-svc-abc123
        
        # Delete without confirmation
        aenv service delete myapp-svc-abc123 --yes
    """
    console = cfg.console.console()
    
    if not yes:
        console.print(f"[yellow]⚠️  You are about to delete service:[/yellow] {service_id}")
        console.print("[yellow]Note: PVC will be kept for reuse[/yellow]")
        if not click.confirm("Are you sure you want to continue?"):
            console.print("[cyan]Deletion cancelled[/cyan]")
            return
    
    system_url = _get_system_url()
    config_manager = get_config_manager()
    hub_config = config_manager.get_hub_config()
    api_key = hub_config.get("api_key") or os.getenv("AENV_API_KEY")
    
    console.print(f"[cyan]🗑️  Deleting service:[/cyan] {service_id}\n")
    
    async def _delete():
        async with AEnvSchedulerClient(
            base_url=system_url,
            api_key=api_key,
        ) as client:
            return await client.delete_env_service(service_id)
    
    try:
        with console.status("[bold green]Deleting service..."):
            success = asyncio.run(_delete())
        
        if success:
            console.print("[green]✅ Service deleted successfully![/green]")
            console.print("[cyan]Note: PVC was kept for reuse[/cyan]")
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
        console.print("[red]Error:[/red] At least one of --replicas, --image, or --env must be provided")
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
                {"Property": "Replicas", "Value": f"{svc.available_replicas}/{svc.replicas}"},
                {"Property": "Service URL", "Value": svc.service_url or "-"},
                {"Property": "Updated At", "Value": svc.updated_at},
            ]
            console.print(tabulate(table_data, headers="keys", tablefmt="grid"))
    
    except Exception as e:
        console.print(f"[red]❌ Update failed:[/red] {str(e)}")
        if cfg.verbose:
            import traceback
            console.print(traceback.format_exc())
        raise click.Abort()
