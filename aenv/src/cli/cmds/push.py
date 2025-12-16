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
push command - Push current aenv project to remote backend aenv hub
"""
import json
import os

import click

from cli.client.aenv_hub_client import AEnvHubClient, AEnvHubError, EnvStatus
from cli.extends.storage.storage_manager import StorageContext, load_storage


@click.command()
@click.option(
    "--work-dir",
    help="Specify aenv project root directory, defaults to current directory",
    default=os.getcwd(),
)
@click.option(
    "--dry-run",
    is_flag=True,
    help="Only display data to be pushed, do not actually push",
)
@click.option("--force", is_flag=True, help="Force overwrite existing environment")
@click.option("--version", "-v", help="Pointed aenv version")
def push(work_dir, dry_run, force, version):
    """Push current aenv project to remote backend aenv hub

    Example:
        aenv push
    """
    config_file = f"{work_dir}/config.json"
    if not os.path.exists(config_file):
        click.echo("Please push in root directory", err=True)
        raise click.Abort()
    with open(config_file) as config:
        meta_data = json.load(config)

    env_name = meta_data["name"]
    version = version if version else meta_data["version"]
    meta_data["version"] = version

    click.echo(
        f"Releasing aenv:{env_name}:{version} to remote aenv_hub",
        err=False,
    )
    hub_client = AEnvHubClient.load_client()
    exist = hub_client.check_env(name=env_name, version=version)
    if exist:
        click.echo(
            f"aenv:{env_name}:{version} already exists in remote aenv_hub", err=False
        )
        state = hub_client.state_environment(env_name, version)
        env_state = EnvStatus.parse_state(state)
        if env_state.running() and not force:
            click.echo("❌ Environment is being prepared, use --force to overwrite")
            raise click.Abort()

    storage = load_storage()
    infos = {"name": env_name, "version": version}
    directory = work_dir if work_dir else os.getcwd()
    ctx = StorageContext(src_url=directory, infos=infos)
    # store_response = None

    store_response = storage.upload(ctx)

    if not store_response.state:
        click.echo("❌Upload failed......")
        raise click.Abort()
    meta_data["codeUrl"] = store_response.dest_url

    try:
        if exist:
            hub_client.update_environment(meta_data)
        else:
            hub_client.create_environment(meta_data)
    except AEnvHubError as e:
        raise RuntimeError("Failed call hub") from e
    click.echo(
        "✅Push successfully! Now you can get environment status with `aenv get`"
    )
