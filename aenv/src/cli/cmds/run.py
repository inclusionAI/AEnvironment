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
run command - Start local environment for testing the current aenv project
"""

import os
from pathlib import Path

import click

from cli.cmds.common import Config, DependencyError, EnvironmentSetupError, pass_config
from cli.utils.common.aenv_logger import get_logger
from cli.utils.mcp import mcp_inspector
from cli.utils.mcp.mcp_task_manager import MCPManager


def run_environment(work_dir: str) -> None:
    """Run environment setup"""
    work_path = Path(work_dir)

    if not work_path.exists():
        raise EnvironmentSetupError(
            f"Working directory does not exist: {work_dir}",
            details={"directory": work_dir, "absolute_path": str(work_path.absolute())},
            suggestion="Please confirm the specified directory path is correct",
        )

    if not work_path.is_dir():
        raise EnvironmentSetupError(
            f"Specified path is not a directory: {work_dir}",
            details={
                "path": work_dir,
                "type": "file" if work_path.is_file() else "unknown",
            },
            suggestion="Please specify a valid directory path",
        )

    if not os.access(work_dir, os.R_OK):
        raise EnvironmentSetupError(
            f"No permission to access working directory: {work_dir}",
            details={
                "directory": work_dir,
                "permissions": oct(os.stat(work_dir).st_mode)[-3:],
            },
            suggestion="Please check directory permissions or run with a user that has access",
        )


def validate_dependencies() -> None:
    """Validate dependencies"""
    try:
        mcp_inspector.check_inspector_requirements()
    except Exception as e:
        raise DependencyError(
            f"Dependency check failed: {str(e)}",
            details={"error_type": type(e).__name__, "error_detail": str(e)},
            suggestion="Please ensure Node.js and npm are installed, then try again",
        )


@click.command(name="run")
@click.option(
    "--work-dir", help="Specify aenv development root directory", default=os.getcwd()
)
@click.option("--inspector-port", type=int, default=6274, help="MCP Inspector port")
@click.option(
    "--quiet", is_flag=True, help="Only start local environment no need inspector"
)
@pass_config
def run(cfg: Config, work_dir, inspector_port, quiet):
    """Start local environment for testing the current aenv project

    This command validates the working directory, checks dependencies,
    installs MCP Inspector, and starts the MCP server and Inspector locally.

    Example:
        aenv run --work-dir /tmp/aenv/search
    """
    console = cfg.console
    # Display startup information
    console.info("🚀 Starting local environment for testing...")
    console.console().print(f"   Working Directory: [cyan]{work_dir}[/cyan]")
    console.console().print(f"   Inspector Port: [cyan]{inspector_port}[/cyan]")
    console.console().print()

    # Validate working environment
    console.info("📁 Validating working environment...")
    run_environment(work_dir)
    console.success("✅ Working environment validation passed")
    if not quiet:
        # Validate dependencies
        console.info("🔧 Checking dependencies...")
        validate_dependencies()
        console.success("✅ Dependency check passed")

        # Install inspector
        console.info("📦 Installing MCP Inspector...")
        mcp_inspector.install_inspector()
        console.success("✅ MCP Inspector installation completed")

        # Start MCP server and Inspector
        console.info("🚀 Starting MCP server and Inspector...")
        console.console().print(
            f"   MCP Inspector will be available at: [cyan]http://localhost:{inspector_port}[/cyan]"
        )
    console.console().print("Press Ctrl+C to stop services\n")

    aenv_logger = get_logger("mcp_manager")
    if cfg.verbose:
        aenv_logger.setLevel("DEBUG")
    manager = MCPManager(logger=aenv_logger)
    manager.start(work_dir, inspector_port, quiet)
