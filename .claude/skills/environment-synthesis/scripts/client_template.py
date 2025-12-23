#!/usr/bin/env python3
"""
AEnvironment Client Template
Template for creating client applications that use AEnvironment agents
"""

import asyncio
import json
import os
import sys
from pathlib import Path
from typing import Dict, Any, Optional

from aenv import Environment


def get_env_config() -> Dict[str, str]:
    """Get LLM configuration from environment variables or user input"""
    config = {}

    api_key = os.getenv("OPENAI_API_KEY")
    base_url = os.getenv("OPENAI_BASE_URL")
    model = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
    aenv_system_url = os.getenv("AENV_SYSTEM_URL")

    if not api_key:
        api_key = input("Enter OPENAI_API_KEY: ").strip()
        if not api_key:
            print("❌ OPENAI_API_KEY is required")
            sys.exit(1)

    if not base_url:
        base_url = input(
            "Enter OPENAI_BASE_URL (optional, press Enter for default): "
        ).strip()
        if not base_url:
            base_url = None
    if not aenv_system_url:
        aenv_system_url = input(
            "Enter AENV_SYSTEM_URL (optional, press Enter for default): "
        ).strip()
        if not aenv_system_url:
            aenv_system_url = None

    config["OPENAI_API_KEY"] = api_key
    if base_url:
        config["OPENAI_BASE_URL"] = base_url
    config["OPENAI_MODEL"] = model
    if aenv_system_url:
        config["AENV_SYSTEM_URL"] = aenv_system_url
    return config


def parse_tool_result(result) -> Dict[str, Any]:
    """Parse tool result that may be in different formats"""
    content_text = None

    if isinstance(result.content, list):
        for item in result.content:
            if isinstance(item, dict) and item.get("type") == "text":
                content_text = item.get("text", "")
                break
    elif isinstance(result.content, str):
        content_text = result.content
    else:
        if isinstance(result.content, dict):
            return result.content
        raise ValueError(f"Cannot parse result content: {type(result.content)}")

    if content_text:
        try:
            return json.loads(content_text)
        except json.JSONDecodeError:
            return {"content": content_text, "status": "success"}

    raise ValueError("Cannot parse result content")


async def main():
    """Main function - customize for your use case"""
    print("🚀 AEnvironment Client")
    print("=" * 50)

    # Get configuration
    env_config = get_env_config()
    os.environ["AENV_SYSTEM_URL"] = env_config["AENV_SYSTEM_URL"]

    # Set your environment name
    env_name = "your-agent-name@1.0.0"

    print(f"\n🔧 Initializing Environment: {env_name}")

    try:
        async with Environment(
            env_name,
            environment_variables=env_config,
            timeout=120,
        ) as env:
            print("✅ Environment ready")

            # List available tools
            tools = await env.list_tools()
            print(f"\n🔧 Available tools:")
            for tool in tools:
                print(
                    f"   - {tool.get('name', 'unknown')}: {tool.get('description', 'No description')}"
                )

            # Your custom logic here
            # Example: Call a tool
            # result = await env.call_tool("tool_name", {"param": "value"})
            # response = parse_tool_result(result)
            # print(response)

    except Exception as e:
        print(f"❌ Environment initialization failed: {e}")
        import traceback

        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
