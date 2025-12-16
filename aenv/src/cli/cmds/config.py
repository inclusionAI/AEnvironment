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

import json
from pathlib import Path

import click
from rich.console import Console
from rich.panel import Panel
from rich.table import Table

from cli.cmds.common import Config, pass_config
from cli.utils.cli_config import get_config_manager

console = Console()


@click.group()
@pass_config
def config(cfg: Config):
    """Manage CLI configuration."""
    global console
    if cfg.console:
        console = cfg.console.console()


@config.command()
@click.option("--key", help="Configuration key to show")
@click.option("--format", type=click.Choice(["json", "table"]), default="table")
def show(key, format):
    """Show current configuration."""
    manager = get_config_manager()

    if key:
        value = manager.get(key)
        if value is not None:
            console.print(f"{key}: {value}")
        else:
            console.print(f"[red]Key '{key}' not found[/red]")
    else:
        config_dict = {
            "global_config": {"global_mode": manager.config.global_mode},
            "build_config": manager.config.build_config,
            "storage_config": manager.config.storage_config,
            "hub_config": manager.config.hub_config,
        }

        if format == "json":
            console.print_json(json.dumps(config_dict, indent=2))
        else:
            table = Table(
                title="CLI Configuration", show_header=True, header_style="bold magenta"
            )
            table.add_column("Section", style="cyan")
            table.add_column("Key", style="yellow")
            table.add_column("Value", style="green")

            for section, values in config_dict.items():
                if isinstance(values, dict):
                    for key, value in values.items():
                        table.add_row(section, key, str(value))
                else:
                    table.add_row(section, "", str(values))

            console.print(table)


@config.command()
@click.argument("key")
@click.argument("value")
def set(key, value):
    """Set configuration value."""
    manager = get_config_manager()

    # Try to parse JSON values
    try:
        parsed_value = json.loads(value)
    except json.JSONDecodeError:
        parsed_value = value

    manager.set(key, parsed_value)

    console.print(f"[green]Set {key} = {parsed_value}[/green]")


@config.command()
@click.argument("key")
def get(key):
    """Get configuration value."""
    manager = get_config_manager()
    value = manager.get(key)

    if value is not None:
        console.print(value)
    else:
        console.print(f"[red]Key '{key}' not found[/red]")


@config.command()
@click.option("--path", help="Configuration file path")
def init(path):
    """Initialize configuration file."""
    if path:
        manager = get_config_manager(path)
    else:
        manager = get_config_manager()

    console.print(
        Panel(
            f"[green]Configuration initialized at:[/green]\n{manager.config_path}",
            title="Config Initialized",
            border_style="green",
        )
    )


@config.command()
@click.option("--force", is_flag=True, help="Force reset to defaults")
def reset(force):
    """Reset configuration to defaults."""
    if not force:
        if not click.confirm(
            "Are you sure you want to reset configuration to defaults?"
        ):
            return

    manager = get_config_manager()
    manager.save_config(manager.config.__class__())
    console.print("[green]Configuration reset to defaults[/green]")


@config.command()
@click.argument("file_path", type=click.Path(exists=True))
def load(file_path):
    """Load configuration from file."""

    with open(file_path, "r") as f:
        config_data = json.load(f)

    manager = get_config_manager()
    manager.update_from_dict(config_data)
    console.print(f"[green]Configuration loaded from {file_path}[/green]")


@config.command()
@click.option("--output", type=click.Path(), help="Output file path")
def export(output):
    """Export current configuration."""
    manager = get_config_manager()

    if output:
        output_path = Path(output)
    else:
        output_path = Path("cli_config_export.json")

    with open(output_path, "w") as f:
        json.dump(
            {
                "global": {
                    "default_registry": manager.config.default_registry,
                    "default_namespace": manager.config.default_namespace,
                },
                "build_config": manager.config.build_config,
                "storage_config": manager.config.storage_config,
                "logging_config": manager.config.logging_config,
            },
            f,
            indent=2,
            ensure_ascii=False,
        )

    console.print(f"[green]Configuration exported to {output_path}[/green]")


@config.command()
def path():
    """Show configuration file path."""
    manager = get_config_manager()
    console.print(f"Configuration file: {manager.config_path}")
