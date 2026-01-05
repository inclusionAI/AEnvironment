"""
Example client application - Complete AEnvironment SDK usage template

This file demonstrates best practices for using AEnvironment SDK:
- Proper async/await usage with context manager
- Environment variable configuration
- Robust result parsing helper function
- Comprehensive error handling
- Interactive user interface
- Multi-tool support

Copy this to your client directory and customize as needed.
"""

import asyncio
import json
import os
import sys
from typing import Dict, Any
from aenv import Environment


def parse_tool_result(result) -> Dict[str, Any]:
    """
    Parse ToolResult.content into structured data.

    Handles three content types from MCP protocol:
    1. List of items with "text" type (most common)
    2. String content (direct JSON or plain text)
    3. Dict content (direct structured data)

    Args:
        result: ToolResult object from env.call_tool()

    Returns:
        Parsed dictionary with tool response data

    Raises:
        ValueError: If content type cannot be parsed
    """
    content = result.content

    # List content (most common from MCP)
    if isinstance(content, list):
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                text = item.get("text", "")
                if text:
                    try:
                        return json.loads(text)
                    except json.JSONDecodeError:
                        return {"raw": text, "status": "success"}

    # String content
    elif isinstance(content, str):
        try:
            return json.loads(content)
        except json.JSONDecodeError:
            return {"raw": content, "status": "success"}

    # Dict content
    elif isinstance(content, dict):
        return content

    raise ValueError(f"Cannot parse content type: {type(content)}")


async def main():
    """
    Main client application demonstrating AEnvironment SDK usage.
    """
    # Get configuration from environment variables
    api_key = os.getenv("OPENAI_API_KEY")
    base_url = os.getenv("OPENAI_BASE_URL")
    model = os.getenv("OPENAI_MODEL", "gpt-4")

    # Validate required configuration
    if not api_key:
        print("❌ Error: OPENAI_API_KEY not set")
        print("   Please set it: export OPENAI_API_KEY='your-key'")
        sys.exit(1)

    # Replace with your deployed environment name
    # Format: "environment-name@version"
    env_name = "agent-XXXX@1.0.0"

    print("=" * 60)
    print("🚀 AEnvironment Client Application")
    print("=" * 60)
    print(f"Environment: {env_name}")
    print(f"Model: {model}")
    print("=" * 60)
    print()

    try:
        # Initialize environment with context manager (recommended)
        async with Environment(
            env_name,
            environment_variables={
                "OPENAI_API_KEY": api_key,
                "OPENAI_BASE_URL": base_url,
                "OPENAI_MODEL": model
            },
            timeout=120  # Adjust based on expected response time
        ) as env:

            # List available tools
            print("📋 Listing available tools...")
            tools = await env.list_tools()

            if not tools:
                print("⚠️  No tools available in this environment")
                return

            print(f"✅ Found {len(tools)} tool(s):\n")
            for i, tool in enumerate(tools, 1):
                name = tool.get("name", "unknown")
                desc = tool.get("description", "No description")
                print(f"   {i}. {name}")
                print(f"      {desc}")

            print()
            print("=" * 60)
            print("🤖 Interactive Chat Mode")
            print("=" * 60)
            print("Commands:")
            print("  - Type your message to chat")
            print("  - Type '/reset' to clear conversation history")
            print("  - Type '/info' to see session information")
            print("  - Type '/quit' or '/exit' to quit")
            print("=" * 60)
            print()

            # Interactive loop
            session_id = "user-session-001"

            while True:
                try:
                    # Get user input
                    user_input = input("You: ").strip()

                    if not user_input:
                        continue

                    # Handle commands
                    if user_input.lower() in ['/quit', '/exit', 'quit', 'exit']:
                        print("\n👋 Goodbye!")
                        break

                    elif user_input.lower() == '/reset':
                        print("\n🔄 Resetting conversation...")
                        result = await env.call_tool(
                            "reset_conversation",
                            {"session_id": session_id}
                        )
                        data = parse_tool_result(result)
                        print(f"✅ {data.get('message', 'Conversation reset')}\n")
                        continue

                    elif user_input.lower() == '/info':
                        print("\n📊 Getting session information...")
                        result = await env.call_tool(
                            "get_session_info",
                            {"session_id": session_id}
                        )
                        data = parse_tool_result(result)
                        if data.get("status") == "success":
                            print(f"   Session ID: {data.get('session_id')}")
                            print(f"   Total messages: {data.get('message_count')}")
                            print(f"   User messages: {data.get('user_messages')}")
                            print(f"   Assistant messages: {data.get('assistant_messages')}")
                            print()
                        continue

                    # Call chat tool
                    print("\n⏳ Thinking...", end="", flush=True)

                    result = await env.call_tool(
                        "chat",
                        {
                            "user_request": user_input,
                            "session_id": session_id
                        }
                    )

                    # Clear "Thinking..." message
                    print("\r" + " " * 20 + "\r", end="")

                    # Parse result
                    data = parse_tool_result(result)

                    # Display response
                    if data.get("status") == "success":
                        response = data.get("response", "")
                        print(f"Agent: {response}\n")
                    else:
                        error_msg = data.get("message", "Unknown error")
                        print(f"❌ Error: {error_msg}\n")

                except KeyboardInterrupt:
                    print("\n\n👋 Interrupted. Goodbye!")
                    break

                except Exception as e:
                    print(f"\n❌ Error: {e}\n")
                    continue

    except TimeoutError:
        print("❌ Environment initialization timeout")
        print("   Try increasing the timeout or check environment availability")
        sys.exit(1)

    except Exception as e:
        print(f"❌ Environment error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


async def test_basic_functionality():
    """
    Simple test function demonstrating basic SDK usage.
    Use this for quick testing or integration tests.
    """
    env_name = "agent-XXXX@1.0.0"

    async with Environment(
        env_name,
        environment_variables={
            "OPENAI_API_KEY": os.getenv("OPENAI_API_KEY"),
            "OPENAI_MODEL": "gpt-4"
        },
        timeout=60
    ) as env:
        # List tools
        tools = await env.list_tools()
        print(f"Available tools: {[t['name'] for t in tools]}")

        # Call tool
        result = await env.call_tool("chat", {"user_request": "Hello!"})
        data = parse_tool_result(result)

        print(f"Status: {data.get('status')}")
        print(f"Response: {data.get('response')}")


if __name__ == "__main__":
    # Run main interactive application
    asyncio.run(main())

    # Or run simple test:
    # asyncio.run(test_basic_functionality())
