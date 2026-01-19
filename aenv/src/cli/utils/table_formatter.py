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
Rich table formatting utilities for CLI output

Provides beautiful, colorful table displays inspired by modern CLI tools.
"""
from typing import Any, Dict, List, Optional

from rich.console import Console
from rich.table import Table
from rich.text import Text


def create_rich_table(
    title: Optional[str] = None,
    show_header: bool = True,
    border_style: str = "cyan",
    title_style: str = "bold cyan",
) -> Table:
    """Create a rich table with consistent styling.

    Args:
        title: Optional title for the table
        show_header: Whether to show table headers
        border_style: Style for table borders
        title_style: Style for table title

    Returns:
        Configured Rich Table instance
    """
    table = Table(
        show_header=show_header,
        header_style="bold magenta",
        border_style=border_style,
        title=title,
        title_style=title_style,
        padding=(0, 1),
        show_lines=False,
    )
    return table


def format_status(status: str) -> Text:
    """Format status with appropriate color.

    Args:
        status: Status string

    Returns:
        Colored Text object
    """
    status_lower = status.lower()

    if status_lower in ["running", "active", "ready", "healthy"]:
        return Text(status, style="bold green")
    elif status_lower in ["pending", "creating", "starting"]:
        return Text(status, style="bold yellow")
    elif status_lower in ["failed", "error", "terminated", "stopped"]:
        return Text(status, style="bold red")
    elif status_lower in ["deleting", "stopping", "terminating"]:
        return Text(status, style="bold orange3")
    else:
        return Text(status, style="dim")


def format_replicas(available: int, desired: int) -> Text:
    """Format replica counts with color based on availability.

    Args:
        available: Number of available replicas
        desired: Number of desired replicas

    Returns:
        Colored Text object
    """
    text = f"{available}/{desired}"

    if available == 0:
        return Text(text, style="bold red")
    elif available < desired:
        return Text(text, style="bold yellow")
    else:
        return Text(text, style="bold green")


def truncate_text(text: str, max_length: int = 50) -> str:
    """Truncate text with ellipsis if too long.

    Args:
        text: Text to truncate
        max_length: Maximum length before truncation

    Returns:
        Truncated text
    """
    if len(text) <= max_length:
        return text
    return text[: max_length - 3] + "..."


def print_instance_list(instances: List[Dict[str, Any]], console: Console) -> None:
    """Print a formatted list of instances.

    Args:
        instances: List of instance dictionaries
        console: Rich console instance
    """
    table = create_rich_table(title="Environment Instances")

    # Add columns
    table.add_column("Instance ID", style="cyan", no_wrap=False)
    table.add_column("Environment", style="bright_blue")
    table.add_column("Version", style="bright_magenta")
    table.add_column("Owner", style="dim")
    table.add_column("Status", justify="center")
    table.add_column("IP Address", style="green")
    table.add_column("Created", style="dim")

    # Add rows
    for instance in instances:
        instance_id = instance.get("id", "-")

        # Environment info
        env_info = instance.get("env") or {}
        env_name = env_info.get("name") if env_info else None
        env_version = env_info.get("version") if env_info else None

        # If env is None, try to extract from instance ID
        if not env_name and instance_id and instance_id != "-":
            parts = instance_id.split("-")
            if len(parts) >= 2:
                env_name = parts[0]

        # Get other fields
        owner = instance.get("owner") or "-"
        status_str = instance.get("status", "-")
        status = format_status(status_str)
        ip = instance.get("ip") or "-"
        created_at = instance.get("created_at", "-")

        table.add_row(
            truncate_text(instance_id, 40),
            env_name or "-",
            env_version or "-",
            owner,
            status,  # Pass Text object directly
            ip,
            created_at,
        )

    console.print(table)


def print_service_list(services: List[Dict[str, Any]], console: Console) -> None:
    """Print a formatted list of services.

    Args:
        services: List of service dictionaries
        console: Rich console instance
    """
    table = create_rich_table(title="Environment Services")

    # Add columns
    table.add_column("Service ID", style="cyan", no_wrap=False)
    table.add_column("Env@Version", style="bright_blue")
    table.add_column("Owner", style="dim")
    table.add_column("Status", justify="center")
    table.add_column("Replicas", justify="center")
    table.add_column("Storage Name", style="bright_magenta")
    table.add_column("Created", style="dim")

    # Add rows
    for svc in services:
        service_id = svc.get("id", "-")

        # Environment info - combine name@version
        env_info = svc.get("env") or {}
        env_name = env_info.get("name") or "-"
        env_version = env_info.get("version") or "-"
        env_display = (
            f"{env_name}@{env_version}"
            if env_name != "-" and env_version != "-"
            else "-"
        )

        # Get other fields
        owner = svc.get("owner") or "-"
        status_str = svc.get("status", "-")
        status = format_status(status_str)

        # Replicas
        available = svc.get("available_replicas", 0)
        desired = svc.get("replicas", 0)
        replicas = format_replicas(available, desired)

        # Storage name
        storage_name = svc.get("storage_name") or "-"

        created_at = svc.get("created_at", "-")

        table.add_row(
            truncate_text(service_id, 40),
            env_display,
            owner,
            status,  # Pass Text object directly
            replicas,  # Pass Text object directly
            storage_name,
            created_at,
        )

    console.print(table)


def print_environment_list(
    environments: List[Dict[str, Any]], console: Console
) -> None:
    """Print a formatted list of environments.

    Args:
        environments: List of environment dictionaries
        console: Rich console instance
    """
    table = create_rich_table(title="Available Environments")

    # Add columns
    table.add_column("Name", style="bright_blue", no_wrap=False)
    table.add_column("Version", style="bright_magenta")
    table.add_column("Description", style="dim")
    table.add_column("Created", style="dim")

    # Add rows
    for env in environments:
        name = env.get("name", "-")
        version = env.get("version", "-")
        description = env.get("description", "-")
        created_at = env.get("created_at", "-")

        table.add_row(
            name,
            version,
            truncate_text(description, 60),
            created_at,
        )

    console.print(table)


def print_detail_table(
    data: List[Dict[str, str]],
    console: Console,
    title: Optional[str] = None,
) -> None:
    """Print a property-value detail table.

    Args:
        data: List of dicts with 'Property' and 'Value' keys
        console: Rich console instance
        title: Optional title for the table
    """
    table = create_rich_table(title=title, border_style="blue")

    table.add_column("Property", style="cyan", no_wrap=True)
    table.add_column("Value", style="white")

    for row in data:
        prop = row.get("Property", "")
        value = row.get("Value", "")

        # Special formatting for status
        if prop == "Status" and isinstance(value, str):
            value = format_status(value)
        # Special formatting for replicas
        elif prop == "Replicas" and "/" in str(value):
            parts = str(value).split("/")
            if len(parts) == 2:
                try:
                    available = int(parts[0])
                    desired = int(parts[1])
                    value = format_replicas(available, desired)
                except ValueError:
                    pass

        table.add_row(prop, value)

    console.print(table)
