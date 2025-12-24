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
import time
from pathlib import Path
from typing import Any, Dict

import click
from rich.console import Console
from rich.panel import Panel
from rich.progress import (
    BarColumn,
    Progress,
    SpinnerColumn,
    TaskProgressColumn,
    TextColumn,
    TimeElapsedColumn,
    TimeRemainingColumn,
)
from rich.table import Table

from cli.cmds.common import Config, pass_config
from cli.extends.artifacts.artifacts_builder import ArtifactBuildContext, load_builder
from cli.utils.cli_config import get_config_manager


class BuildProgressTracker:
    """Real-time build progress tracker with rich UI components."""

    def __init__(self):
        self.console = None
        self.progress = Progress(
            SpinnerColumn(),
            TextColumn("[bold blue]{task.description}"),
            BarColumn(bar_width=40),
            TaskProgressColumn(),
            TimeElapsedColumn(),
            TimeRemainingColumn(),
            console=self.console,
        )

    def start_build(self, image_name: str, tag: str, platform: str) -> None:
        """Start the build process with progress display."""
        self.console.print(
            Panel(
                f"[bold green]Starting Docker Build[/bold green]\n"
                f"Image: [cyan]{image_name}[/cyan]\n"
                f"Tags: [yellow]{tag}[/yellow]\n"
                f"Platform: [yellow]{platform}[/yellow]",
                title="Build Information",
                border_style="green",
            )
        )

    def update_stage(self, stage: str, status: str, progress: float = 0.0) -> None:
        """Update build stage with progress."""
        self.console.print(
            f"[dim]{stage}[/dim]: [bold]{status}[/bold] ({progress:.1f}%)"
        )

    def log_message(self, message: str, style: str = "info") -> None:
        """Log a message with appropriate styling."""
        styles = {
            "info": "[blue]ℹ[/blue]",
            "success": "[green]✓[/green]",
            "warning": "[yellow]⚠[/yellow]",
            "error": "[red]✗[/red]",
        }
        prefix = styles.get(style, "[blue]ℹ[/blue]")
        self.console.print(f"{prefix} {message}")

    def show_build_summary(self, image_name: str, tag: str, duration: float) -> None:
        """Display build summary."""
        table = Table(
            title="Build Summary", show_header=True, header_style="bold magenta"
        )
        table.add_column("Property", style="cyan")
        table.add_column("Value", style="green")

        table.add_row("Image Name", image_name)
        table.add_row("Tags", tag)
        table.add_row("Duration", f"{duration:.2f}s")
        table.add_row("Status", "[bold green]SUCCESS[/bold green]")

        self.console.print(table)


@click.command()
@click.option(
    "--work-dir",
    "-w",
    type=click.Path(exists=True, file_okay=False, dir_okay=True),
    default=".",
    help="Working directory for the build",
)
@click.option("--image-name", "-n", type=str, help="Name for the Docker image")
@click.option(
    "--image-tag",
    "-t",
    type=str,
    multiple=True,
    help="Tags for the Docker image (can be used multiple times)",
)
@click.option("--registry", "-r", type=str, help="Docker registry URL")
@click.option("--namespace", "-s", type=str, help="Namespace for the Docker image")
@click.option(
    "--push/--no-push", default=False, help="Push image to registry after build"
)
@click.option("--platform", "-p", type=str, help="Platform for the Docker image")
@pass_config
def build(
    cfg: Config, work_dir, image_name, image_tag, registry, namespace, platform, push
):
    """
    Build Docker images with real-time progress display.

    This command builds Docker images from your project and provides
    real-time progress updates with beautiful UI components.

    Examples:
        aenv build
        aenv build --image-name myapp --image-tag v1.0
        aenv build --work-dir ./myproject --registry myregistry.com
    """
    console = cfg.console.console() if cfg.console else Console()
    env_build_config = _load_env_config(work_dir=work_dir)

    # Get build configuration from CLI config
    config_manager = get_config_manager()
    config_path = config_manager.config_path
    build_config = config_manager.get_build_config().copy()
    docker_sock = build_config.get("build_args", {}).get("socket", "")
    docker_sock_path = docker_sock.removeprefix("unix://")

    # Skip path check for Windows named pipes
    if not docker_sock.startswith("npipe://"):
        docker_sock_idx = Path(docker_sock_path)
        if not docker_sock_idx.exists():
            console.print(
                f"[red]Error: Docker sock:{docker_sock_idx} your provided in config:{config_path} is not exist[/red]"
            )
            return

    # Initialize build context
    work_path = Path(work_dir).resolve()
    if not work_path.exists():
        console.print(f"[red]Error: Working directory {work_path} does not exist[/red]")
        raise click.Abort()

    custom_build_config = env_build_config.get("buildConfig", {})
    if image_name is None:
        image_name = custom_build_config.get("image_name", "")
        if not image_name:
            image_name = env_build_config.get("name")
    if not image_tag:
        image_tag = custom_build_config.get("image_tag")
        if not image_tag:
            image_tag = env_build_config.get("version")
    if not image_name or not image_tag:
        console.print("[red]Error: image name/tag is not config[/red]")
        return

    registry_settings = build_config.get("registry", {})
    if registry is None:
        registry = registry_settings.get("host", "docker.io")
    if not namespace:
        namespace = registry_settings.get("namespace", "")
    full_image = f"{image_name}:{image_tag}"
    if namespace:
        full_image = f"{namespace}/{full_image}"
    if registry:
        full_image = f"{registry}/{full_image}"

    user = registry_settings.get("username", "")
    password = registry_settings.get("password", "")
    if push:
        if not user or not password:
            warn_text = f"""Warning: Your provided username or password in config:{config_path} is invalid.
             Push may fail due to authentication issues.!"""
            console.print(warn_text, style="bold yellow")
    # Create build context
    ctx = ArtifactBuildContext(
        work_dir=str(work_path),
        image_name=image_name,
        image_tag=image_tag,
        registry=registry,
        build_config=build_config,
        push_required=push,
        namespace=namespace,
        platform=platform,
    )

    # Initialize progress tracker
    tracker = BuildProgressTracker()
    tracker.console = console

    # Display build information
    tracker.start_build(image_name, image_tag, platform)

    # Create progress display
    with Progress(
        SpinnerColumn(),
        TextColumn("[bold blue]{task.description}"),
        BarColumn(bar_width=40),
        TaskProgressColumn(),
        TimeElapsedColumn(),
        console=console,
    ) as progress:

        # Build stages
        stages = [
            "Loading build configuration",
            "Building Docker image",
        ]
        if push:
            ctx.push_required = True
        tasks = []
        for stage in stages:
            task = progress.add_task(stage, total=100)
            tasks.append((task, stage))

        start_time = time.time()

        try:
            # Initialize Docker builder
            builder = load_builder()

            # Stage 1: Load build configuration
            task, stage = tasks[0]
            progress.update(task, description=f"[cyan]{stage}[/cyan]")
            if cfg.verbose:
                tracker.log_message(f"Loading build config from {work_path}")
            progress.update(task, completed=100)

            # Stage 2: Build Docker image
            task, stage = tasks[1]
            progress.update(task, description=f"[cyan]{stage}[/cyan]")
            with console.status("[bold green]Building Docker image..."):
                try:
                    builder.trigger(ctx)
                    if cfg.verbose:
                        tracker.log_message(
                            "Docker image built successfully", "success"
                        )
                except Exception as e:
                    console.print(f"[red]Build failed: {e}[/red]")
                    raise
            progress.update(task, completed=100)

            # Calculate duration
            duration = time.time() - start_time

            # Display success summary
            console.print(
                Panel(
                    f"[bold green]Build completed successfully![/bold green]\n"
                    f"Image: [cyan]{image_name}[/cyan]\n"
                    f"Tags: [yellow]{image_tag}[/yellow]\n"
                    f"Duration: [magenta]{duration:.2f}s[/magenta]",
                    title="Build Complete",
                    border_style="green",
                )
            )

            tracker.show_build_summary(image_name, image_tag, duration)

        except Exception as e:
            console.print(
                Panel(
                    f"[bold red]Build failed[/bold red]\n{str(e)}",
                    title="Build Error",
                    border_style="red",
                )
            )
            raise click.Abort()

    _refresh_env_artifact(env_build_config, work_dir, full_image)


def _load_env_config(work_dir: str) -> Dict[str, Any]:
    """Load build configuration from config.json.

    Args:
        work_dir: Working directory to search for config.json.

    Returns:
        Dictionary containing build configuration.
    """
    config_path = Path(work_dir) / "config.json"
    if not config_path.exists():
        raise click.Abort("current working directory config.json does not exist")

    with open(config_path, "r") as f:
        config = json.load(f)

    return config


def _refresh_env_artifact(env_config, work_dir, artifact):
    """Refresh build configuration to config.json.

    Args:
        env_config: Environment configs.
        work_dir: Working directory to search for config.json.
        artifact: Current build.
    """
    config_path = Path(work_dir) / "config.json"
    if not config_path.exists():
        raise click.Abort("current working directory config.json does not exist")

    with open(config_path, "w+") as f:
        artifacts = env_config.get("artifacts", [])
        image_exist_flag = False
        for arti in artifacts:
            arti_type = arti.get("type")
            if arti_type != "image":
                continue
            arti["content"] = artifact
            image_exist_flag = True
        if not image_exist_flag:
            artifacts.append({"type": "image", "content": artifact})
        f.write(json.dumps(env_config, indent=4))
