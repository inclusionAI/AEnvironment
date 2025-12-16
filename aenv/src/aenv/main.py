#!/usr/bin/env python3
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
AEnv MCP Server - Main entry point

This script provides a simple way to start an MCP server with tools
from specified directories or files.
"""

import argparse
import json
import sys
from pathlib import Path

from aenv.core.logging import getLogger
from aenv.server.mcp_server import create_server

# Use new logging system
logger = getLogger("aenv.main", "system")


def show_version():
    try:
        import importlib.util
        from importlib.resources import files

        if importlib.util.find_spec("cli.data") is None:
            raise RuntimeError("cli.data not found")
        data_file = files("cli.data").joinpath("version_info.json")
        build_info = json.loads(data_file.read_text(encoding="utf-8"))
        logger.info("AEnv SDK version info", extra=build_info)
    except Exception as exc:
        logger.warning("Failed to load version info", extra={"error": str(exc)})


def run_cli():
    """CLI entry point that handles both sync and async environments."""
    parser = argparse.ArgumentParser(description="Start AEnv MCP server with tools")
    parser.add_argument(
        "tools_path", type=str, help="Path to directory or Python file containing tools"
    )
    parser.add_argument("--host", type=str, default="0.0.0.0")
    parser.add_argument("--port", type=int, default=8081)
    parser.add_argument("--name", type=str, default="aenv-server")
    parser.add_argument(
        "--log-level",
        type=str,
        choices=["DEBUG", "INFO", "WARNING", "ERROR"],
        default="INFO",
    )
    parser.add_argument(
        "--log-dir",
        type=str,
        default="/home/admin/logs/aenv",
        help="Directory to store log files (default: stdout only)",
    )

    args = parser.parse_args()

    show_version()

    server = create_server(name=args.name)
    tools_path = Path(args.tools_path)

    if tools_path.is_file() and tools_path.suffix == ".py":
        tools_loaded = server.load_tools_from_module(str(tools_path))
        logger.info(f"Loaded {tools_loaded} tools from {tools_path}")
    elif tools_path.is_dir():
        tools_loaded = server.load_tools_from_directory(tools_path)
        logger.info(f"Loaded {tools_loaded} tools from directory {tools_path}")
    else:
        logger.error(f"Invalid tools path: {tools_path}")
        sys.exit(1)

    server.run(host=args.host, port=args.port)


if __name__ == "__main__":
    run_cli()
