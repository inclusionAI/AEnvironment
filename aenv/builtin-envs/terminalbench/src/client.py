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

# -*- coding: utf-8 -*-
"""
FastMCP official library interactive MCP Client
usage:
    python client.py                    # Default connection to http://127.0.0.1:8081/mcp
    python client.py http://ip:port/mcp # Custom HTTP endpoint
    python client.py ./server.py        # Use STDIO transport
"""
import asyncio
import json
import sys
from typing import Any, Dict, List

# Note: Import modification here
from fastmcp import Client
from fastmcp.client.transports import PythonStdioTransport, StreamableHttpTransport
from rich.console import Console
from rich.prompt import Confirm, Prompt
from rich.syntax import Syntax

console = Console()


# -------------------------------------------------
# Utility functions
# -------------------------------------------------
def pprint_json1(obj: Any) -> None:
    """Color print JSON"""
    console.print(
        Syntax(json.dumps(obj, indent=2, ensure_ascii=False), "json", theme="monokai")
    )


def pprint_json(obj: Any) -> None:
    """Color print JSON or process FastMCP results"""
    try:
        console.print(
            Syntax(
                json.dumps(obj, indent=2, ensure_ascii=False), "json", theme="monokai"
            )
        )
    except TypeError:
        if hasattr(obj, "__dict__"):
            try:
                console.print(
                    Syntax(
                        json.dumps(obj.__dict__, indent=2, ensure_ascii=False),
                        "json",
                        theme="monokai",
                    )
                )
            except Exception:
                console.print(obj)
        else:
            console.print(str(obj))


def choose(items: List[str], title: str) -> str:
    """Simple numbered selection"""
    console.print(f"\n[cyan]{title}[/cyan]")
    for idx, it in enumerate(items, 1):
        console.print(f"  {idx}. {it}")
    while True:
        raw = Prompt.ask("Please select number", default="1")
        try:
            return items[int(raw) - 1]
        except (ValueError, IndexError):
            console.print("[red]Invalid number, please try again[/red]")


def build_args_from_schema(schema: Dict[str, Any]) -> Dict[str, Any]:
    """Interactively construct parameter dictionary from JSONSchema"""
    args: Dict[str, Any] = {}
    required = set(schema.get("required", []))
    props = schema.get("properties", {})
    if not props:
        return {}
    console.print("\n[yellow]Please enter parameters:[/yellow]")
    for k, v in props.items():
        typ = v.get("type", "any")
        desc = v.get("description", "")
        default = v.get("default", "")
        prompt_text = f"  {k} ({typ})" + (" *" if k in required else "")
        if desc:
            prompt_text += f"  # {desc}"
        val = Prompt.ask(prompt_text, default=str(default))
        # Simple type conversion
        if typ == "integer":
            val = int(val)
        elif typ == "number":
            val = float(val)
        elif typ == "boolean":
            val = val.lower() in ("true", "1", "yes", "y")
        args[k] = val
    return args


# -------------------------------------------------
# Interactive session
# -------------------------------------------------
async def interactive_session(client: Client) -> None:
    async with client:
        # 1. Enumerate tools / resources / prompts
        tools = await client.list_tools()
        resources = await client.list_resources()
        prompts = await client.list_prompts()

        while True:
            console.rule("[bold cyan]Please select operation to execute")
            action = choose(
                ["Call Tool", "Read Resource", "Render Prompt", "Exit"],
                "Operation list",
            )
            if action == "Exit":
                return

            # ---------- 1. Call Tool ----------
            if action == "Call Tool":
                if not tools:
                    console.print("[red]Server has not exposed any Tool[/red]")
                    continue
                tool = choose([t.name for t in tools], "Available Tools")
                schema = next(t.inputSchema for t in tools if t.name == tool)
                args = build_args_from_schema(schema)
                if not Confirm.ask(
                    f"\nConfirm calling [bold]{tool}[/bold] ?", default=True
                ):
                    continue
                with console.status("[bold green]Running..."):
                    try:
                        result = await client.call_tool(tool, args)
                    except Exception as e:
                        console.print(f"[red]Call failed: {e}[/red]")
                        continue
                console.print("\n[bold cyan]📤 Result:")
                pprint_json(result)

            # ---------- 2. Read Resource ----------
            elif action == "Read Resource":
                if not resources:
                    console.print("[red]Server has not exposed any Resource[/red]")
                    continue
                res = choose([r.uri for r in resources], "Available Resources")
                data = await client.read_resource(res)
                console.print(f"\n[bold cyan]📄 {res} content:")
                pprint_json(data)

            # ---------- 3. Render Prompt ----------
            elif action == "Render Prompt":
                if not prompts:
                    console.print("[red]Server has not exposed any Prompt[/red]")
                    continue
                pr = choose([p.name for p in prompts], "Available Prompts")
                schema = next(p.arguments for p in prompts if p.name == pr)
                args = build_args_from_schema(schema)
                prompt = await client.get_prompt(pr, args)
                console.print("\n[bold cyan]📝 Rendered Prompt:")
                pprint_json(prompt)


# -------------------------------------------------
# Entry point
# -------------------------------------------------
async def main() -> None:
    url_or_script = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8081/mcp"
    if url_or_script.startswith("http"):
        # Use StreamableHTTPTransport instead of SSETransport
        transport = StreamableHttpTransport(url_or_script)
    else:
        transport = PythonStdioTransport(url_or_script)

    client = Client(transport)
    await interactive_session(client)


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        console.print("\n[bold red]User interrupted, exiting.[/bold red]")
