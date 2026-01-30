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

import click

from cli.cmds import (
    build,
    config,
    get,
    init,
    instance,
    list,
    push,
    run,
    service,
    version,
)
from cli.cmds.common import Config, global_error_handler, pass_config
from cli.utils.common.aenv_logger import configure_logging


class CLI(click.Group):
    @global_error_handler
    def invoke(self, ctx):
        return super().invoke(ctx)


@click.group(cls=CLI)
@click.option("--debug", is_flag=True)
@click.option("--verbose", "-v", is_flag=True, help="Enable verbose output")
@pass_config
def cli(cfg: Config, debug: bool, verbose: bool):
    """Aenv cli helps build your custom aenv"""
    cfg.debug = debug
    cfg.verbose = verbose
    # Configure logging based on verbose flag
    configure_logging(verbose)


# add subcommand
cli.add_command(init)
cli.add_command(push)
cli.add_command(get)
cli.add_command(run)
cli.add_command(list)
cli.add_command(version)
cli.add_command(build)
cli.add_command(config)
cli.add_command(instance)
cli.add_command(service)

if __name__ == "__main__":
    cli()
