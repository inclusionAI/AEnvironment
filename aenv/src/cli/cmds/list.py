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
list command - List all environments with pagination
"""
import json

import click
from tabulate import tabulate

from cli.client.aenv_hub_client import AEnvHubClient


@click.command("list")
@click.option(
    "--limit",
    "-l",
    type=int,
    default=100,
    show_default=True,
    help="number of envs per page",
)
@click.option(
    "--offset",
    "-o",
    type=int,
    default=0,
    show_default=True,
    help="page offset",
)
@click.option(
    "--format",
    "-f",
    type=click.Choice(["table", "json"]),
    default="table",
    help="Output format",
)
def list_env(limit, offset, format):
    """List all environments with pagination

    Example:
        aenv list
        aenv list --limit 10 --offset 0
        aenv list --format json
        aenv list --format table
    """
    hub_client = AEnvHubClient.load_client()
    try:
        environments = hub_client.list_environments(limit=limit, offset=offset)
    except Exception as e:
        raise RuntimeError("Failed to get environment list") from e

    if not environments:
        click.echo("📭 No environment data")
        return

    if format == "json":
        click.echo(json.dumps(environments, indent=2, ensure_ascii=False))
    elif format == "table":
        # Assume environments is a list with name, version, description fields
        # Adjust keys based on actual response structure
        table_data = []
        for env in environments:
            table_data.append(
                {
                    "Name": env.get("name", "-"),
                    "Version": env.get("version", "-"),
                    "Description": env.get("description", "-"),
                    "Created At": env.get("created_at", "-"),
                }
            )
        # Use grid format for clarity
        click.echo(tabulate(table_data, headers="keys", tablefmt="grid"))
