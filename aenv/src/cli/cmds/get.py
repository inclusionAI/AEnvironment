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
get command - Get specified environment details
"""

import json

import click

from cli.client.aenv_hub_client import AEnvHubClient
from cli.cmds.common import Config, pass_config


@click.command()
@click.argument("name")
@click.option("--version", "-v", help="Specify aenv version number", default="1.0.0")
@pass_config
@click.option(
    "--format",
    "-f",
    type=click.Choice(["table", "json"]),
    default="json",
    help="Output format",
)
def get(config: Config, name, version, format):
    """Get specified environment details

    NAME: aenv name
    VERSION: aenv version
    FORMAT: aenv format

    Example:
        aenv get search
    """
    client = AEnvHubClient.load_client()
    env = client.get_environment(name, version)

    if format == "json":
        click.echo(json.dumps(env, indent=2, ensure_ascii=False))
    else:
        click.echo(f"Environment: {name}:{version}")
        click.echo(f"Status: {env.get('status', 'unknown')}")
        click.echo(f"Description: {env.get('description', 'No description')}")
