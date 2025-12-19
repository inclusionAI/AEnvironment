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
MCP Inspector Management Tool
Used to launch MCP Inspector for debugging during testing
"""

import platform
import shutil
import subprocess

import click


def _is_npm_available() -> bool:
    """Check if npm is available"""
    if platform.system() == "Windows":
        # On Windows, we need shell=True for npm (batch file) or check for npm.cmd
        # shutil.which is cleaner than catching subprocess errors
        return shutil.which("npm") is not None

    try:
        subprocess.run(["npm", "--version"], capture_output=True, check=True)
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def install_inspector():
    """Install MCP Inspector"""
    if not _is_npm_available():
        msg = "npm not found, please install Node.js and npm first"
        click.secho(f"❌ {msg}", fg="red", err=True)
        click.abort()

    is_windows = platform.system() == "Windows"

    try:
        # Check if inspector is installed
        # On Windows, npx also needs shell=True
        result = subprocess.run(
            ["npx", "@modelcontextprotocol/inspector", "-h"],
            capture_output=True,
            timeout=60,
            shell=is_windows,
        )

        if result.returncode != 0:
            msg = "MCP Inspector not installed, attempting automatic installation..."
            click.secho(f"⚠️  {msg}", fg="yellow")
            subprocess.run(
                ["npm", "install", "-g", "@modelcontextprotocol/inspector"],
                check=True,
                capture_output=True,
                shell=is_windows,
            )
            click.echo("MCP Inspector installed successfully")
    except subprocess.TimeoutExpired:
        msg = "MCP Inspector check timed out. This may happen on first run when downloading the package. Please try again or install manually: npm install -g @modelcontextprotocol/inspector"
        click.secho(f"⚠️  {msg}", fg="yellow")
        # Don't abort, allow the process to continue - inspector might still work
    except subprocess.CalledProcessError as e:
        msg = f"Installation failed: {e}"
        click.secho(f"❌ {msg}", fg="red", err=True)
        click.abort()
    click.echo("MCP Inspector Is Installed...!")


def check_inspector_requirements() -> tuple[bool, str]:
    """Check inspector runtime requirements"""
    is_windows = platform.system() == "Windows"

    # Check Node.js
    try:
        # node usually works without shell=True even on Windows (it's an exe), but consistentcy is good
        result = subprocess.run(
            ["node", "--version"],
            capture_output=True,
            text=True,
            timeout=5,
            shell=is_windows,
        )
        node_version = result.stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False, "Node.js not installed, please install Node.js (>=14.x)"

    # Check npm
    try:
        subprocess.run(
            ["npm", "--version"],
            capture_output=True,
            check=True,
            timeout=5,
            shell=is_windows,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False, "npm not found, please ensure Node.js is installed correctly"

    return True, f"Node.js {node_version} installed"


def auto_install_inspector():
    stat, msg = check_inspector_requirements()
    if not stat:
        click.echo(f"❌ {msg}", fg="red", err=True)
        click.abort()
    install_inspector()
