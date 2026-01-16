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
Command module
"""
# mycli/commands/__init__.py

from cli.cmds.build import build
from cli.cmds.config import config
from cli.cmds.get import get
from cli.cmds.init import init
from cli.cmds.instance import instance
from cli.cmds.list import list_env as list
from cli.cmds.push import push
from cli.cmds.run import run
from cli.cmds.service import service
from cli.cmds.version import version

# Optional: define all available commands
__all__ = [
    "init",
    "push",
    "get",
    "run",
    "list",
    "version",
    "build",
    "config",
    "instance",
    "service",
]
