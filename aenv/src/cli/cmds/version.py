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
version command - Display version information
"""
import json
from importlib.resources import files

import click


def _load_config():
    data_file = files("cli.data").joinpath("version_info.json")
    return json.loads(data_file.read_text(encoding="utf-8"))


@click.command()
@click.option("--short", "-s", is_flag=True, help="Show only version number")
@click.option("--json", "-j", "output_json", is_flag=True, help="Output in JSON format")
def version(short, output_json):
    """Display version number and corresponding build/commit information"""
    build_info = _load_config()
    package_version = build_info["version"]

    if short:
        click.echo(package_version)
        return

    if output_json:
        click.echo(json.dumps(build_info, indent=2, ensure_ascii=False))
        return

    # Format output
    click.echo(f"AEnv SDK Version: {package_version}")

    if build_info.get("source") == "development":
        click.echo("Environment: Development")
        click.echo(f"Git Branch: {build_info['branch']}")
        click.echo(f"Commit: {build_info['commit']}")
        click.echo(f"Commit Time: {build_info['date']}")
        if build_info.get("dirty"):
            click.echo("⚠️  Working directory has uncommitted changes")
        else:
            click.echo("✅  Working directory is clean")
    else:
        click.echo("Environment: PyPI Package")
        click.echo(f"Build Version: {build_info.get('build_version', package_version)}")
        click.echo(f"Build Time: {build_info.get('build_date', 'unknown')}")
        click.echo(f"Build Commit: {build_info.get('build_commit', 'unknown')}")

        # Additional build info
        if build_info.get("python_version"):
            click.echo(f"Python Version: {build_info['python_version']}")
        if build_info.get("build_host"):
            click.echo(f"Build Host: {build_info['build_host']}")
