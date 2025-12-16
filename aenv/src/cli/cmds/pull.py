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
pull command - Pull an aenv project to local repository
"""
import os

import click


@click.command()
@click.argument("name")
@click.option("--version", "-v", help="Version number", default="latest")
@click.argument("--work-dir", "-w", default=os.getcwd())
@click.option("--force", is_flag=True, help="Force overwrite local configuration")
def pull(name, version, work_dir, force):
    """Pull an aenv project to local repository

    NAME: aenv name

    Example:
        aenv pull dev --url https://api.example.com/env/dev
    """
    pass
