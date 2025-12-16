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

import os

import click
from rich import box
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

from cli.client.aenv_hub_client import AEnvHubClient
from cli.cmds.common import Config, pass_config
from cli.utils.scaffold import ScaffoldParams, load_aenv_scaffold


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
@pass_config
def init(cfg: Config, name, version, template, work_dir, force):
    """
    Initialize aenv project using environmental scaffolding tools

    NAME: aenv name

    Example:
        aenv init myproject --version 1.0.0
    """
    console = cfg.console.console()
    # Display initialization header
    console.print(
        Panel(
            Text(f"🚀 Initializing AEnv Project: {name}", style="bold cyan"),
            title="AEnv Scaffolding",
            subtitle="Environment Setup",
            box=box.ROUNDED,
        )
    )

    # Check if environment already exists in hub
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
    config_table.add_row("Template", template)
    config_table.add_row("Target Directory", project_dir)
    config_table.add_row("Force Mode", "✅ Enabled" if force else "❌ Disabled")

    console.print(config_table)
    console.print()

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
