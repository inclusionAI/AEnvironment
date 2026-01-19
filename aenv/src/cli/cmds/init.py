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
init command - Initialize aenv project using environmental scaffolding tools
"""

import json
import os
import re
from pathlib import Path

import click
from rich import box
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

from cli.client.aenv_hub_client import AEnvHubClient
from cli.cmds.common import Config, pass_config
from cli.utils.scaffold import ScaffoldParams, load_aenv_scaffold


def validate_env_name(name: str) -> tuple[bool, str]:
    """
    Validate environment name according to Kubernetes pod naming rules.

    Pod names must:
    - Contain only lowercase letters, numbers, and hyphens (-)
    - Start with a letter or number
    - End with a letter or number
    - Be at most 253 characters long

    Returns:
        tuple: (is_valid, error_message)
    """
    if not name:
        return False, "Environment name cannot be empty"

    if len(name) > 253:
        return (
            False,
            f"Environment name is too long (max 253 characters, got {len(name)})",
        )

    # Check if name contains only lowercase letters, numbers, and hyphens
    if not re.match(r"^[a-z0-9-]+$", name):
        return (
            False,
            "Environment name must contain only lowercase letters, numbers, and hyphens (-)",
        )

    # Check if name starts with a letter or number
    if not re.match(r"^[a-z0-9]", name):
        return False, "Environment name must start with a lowercase letter or number"

    # Check if name ends with a letter or number
    if not re.search(r"[a-z0-9]$", name):
        return False, "Environment name must end with a lowercase letter or number"

    return True, ""


@click.command()
@click.argument("name")
@click.option("--version", "-v", help="Specify aenv version number", default="1.0.0")
@click.option(
    "--template",
    "-t",
    type=click.Choice(["default"]),
    help="Scaffolding template selection",
    default="default",
)
@click.option(
    "--work-dir", "-w", help="Working directory for initialization", default=os.getcwd()
)
@click.option("--force", is_flag=True, help="Force overwrite existing directory")
@click.option(
    "--config-only",
    is_flag=True,
    help="Only create config.json file, skip other files and directories",
)
@pass_config
def init(cfg: Config, name, version, template, work_dir, force, config_only):
    """
    Initialize aenv project using environmental scaffolding tools

    NAME: aenv name

    Example:
        aenv init myproject --version 1.0.0
    """
    console = cfg.console.console()

    # Validate environment name (used as pod name prefix)
    is_valid, error_msg = validate_env_name(name)
    if not is_valid:
        console.print(
            Panel(
                f"❌ Invalid environment name: {error_msg}\n\n"
                "[yellow]Valid names must:[/yellow]\n"
                "  • Contain only lowercase letters, numbers, and hyphens (-)\n"
                "  • Start with a lowercase letter or number\n"
                "  • End with a lowercase letter or number\n"
                "  • Be at most 253 characters long\n\n"
                "[cyan]Examples:[/cyan] my-env, prod-env-01, test123",
                title="Validation Error",
                style="bold red",
                box=box.ROUNDED,
            )
        )
        raise click.Abort()

    # Display initialization header
    console.print(
        Panel(
            Text(f"🚀 Initializing AEnv Project: {name}", style="bold cyan"),
            title="AEnv Scaffolding",
            subtitle="Environment Setup",
            box=box.ROUNDED,
        )
    )

    # Check if environment already exists in hub (skip for config-only mode)
    if not config_only:
        with console.status("[bold green]Checking environment registry..."):
            hub_client = AEnvHubClient.load_client()
            exist = hub_client.check_env(name=name, version=version)

        if exist:
            console.print(
                Panel(
                    f"❌ Environment name '{name}' already exists in registry",
                    title="Error",
                    style="bold red",
                    box=box.ROUNDED,
                )
            )
            raise click.Abort()

    # Use default template
    if not template:
        template = "default"

    project_dir = f"{work_dir}/{name}"

    # Display configuration summary
    config_table = Table(
        title="Configuration",
        box=box.ROUNDED,
        show_header=True,
        header_style="bold magenta",
    )
    config_table.add_column("Parameter", style="cyan", no_wrap=True)
    config_table.add_column("Value", style="green")
    config_table.add_row("Project Name", name)
    config_table.add_row("Version", version)
    if not config_only:
        config_table.add_row("Template", template)
        config_table.add_row("Target Directory", project_dir)
    config_table.add_row("Config Only", "✅ Enabled" if config_only else "❌ Disabled")
    config_table.add_row("Force Mode", "✅ Enabled" if force else "❌ Disabled")

    console.print(config_table)
    console.print()

    if config_only:
        # Only create config.json in current directory from template
        config_path = Path(work_dir) / "config.json"
        if config_path.exists() and not force:
            console.print(
                Panel(
                    f"❌ config.json already exists at {config_path}\nUse --force to overwrite",
                    title="Error",
                    style="bold red",
                    box=box.ROUNDED,
                )
            )
            raise click.Abort()

        try:
            with console.status("[bold green]Loading config.json from template..."):
                # Load scaffold to get template config
                scaffold = load_aenv_scaffold()
                # Get config.json content from template
                template_config = scaffold.get_template_config(template)
                # Update name and version
                template_config["name"] = name
                template_config["version"] = version
                # Update pvcName in service config to match environment name
                if "deployConfig" in template_config:
                    deploy_config = template_config["deployConfig"]
                    if "service" in deploy_config and isinstance(
                        deploy_config["service"], dict
                    ):
                        deploy_config["service"]["pvcName"] = name

            with console.status("[bold green]Creating config.json..."):
                with open(config_path, "w") as f:
                    json.dump(template_config, f, indent=4)
        except Exception as e:
            console.print(
                Panel(
                    f"❌ Failed to create config.json from template\n\n[red]{str(e)}[/red]",
                    title="Error",
                    style="bold red",
                    box=box.ROUNDED,
                )
            )
            if cfg.verbose:
                console.print_exception()
            raise click.Abort()

        console.print(
            Panel(
                f"✅ Successfully created config.json at {config_path}",
                title="Success",
                style="bold green",
                box=box.ROUNDED,
            )
        )
    else:
        # Full scaffolding initialization
        params = ScaffoldParams(
            name=name,
            version=version,
            template=template,
            target_dir=project_dir,
            policy=force,
        )
        try:
            with console.status("[bold green]Creating project scaffolding..."):
                scaffold = load_aenv_scaffold()
                scaffold.init(params)

        except Exception as e:
            console.print(
                Panel(
                    f"❌ Scaffolding creation failed\n\n[red]{str(e)}[/red]",
                    title="Error",
                    style="bold red",
                    box=box.ROUNDED,
                )
            )
            if cfg.verbose:
                console.print_exception()
            raise click.Abort()

    if not config_only:
        # Success message
        console.print(
            Panel(
                f"✅ Successfully initialized project '{name}'",
                title="Success",
                style="bold green",
                box=box.ROUNDED,
            )
        )

        # Next steps
        next_steps_table = Table(title="Next Steps", box=box.ROUNDED, show_header=False)
        next_steps_table.add_column("Action", style="cyan")
        next_steps_table.add_column("Command", style="yellow")

        next_steps_table.add_row(
            "Navigate to project", f"cd {os.path.basename(project_dir)}"
        )
        next_steps_table.add_row("Review project structure", "ls -la")
        next_steps_table.add_row("Test environment locally", "aenv test")
        next_steps_table.add_row("Build the environment", "aenv build")
        next_steps_table.add_row("View examples", "aenv examples")

        console.print(next_steps_table)

        # Quick start guide
        console.print(
            Panel(
                Text("💡 Quick Start Guide\n\n", style="bold blue")
                + Text("1. Edit config.json to customize your environment\n")
                + Text("2. Modify Dockerfile for your specific requirements\n")
                + Text("3. Implement your custom environment logic in src/\n")
                + Text("4. Test with 'aenv test' before building"),
                title="Tips",
                box=box.ROUNDED,
                style="blue",
            )
        )
    else:
        # Next steps for config-only mode
        next_steps_table = Table(title="Next Steps", box=box.ROUNDED, show_header=False)
        next_steps_table.add_column("Action", style="cyan")
        next_steps_table.add_column("Command", style="yellow")

        next_steps_table.add_row("Edit config.json", "Edit config.json to customize")
        next_steps_table.add_row("Build the environment", "aenv build")

        console.print(next_steps_table)
