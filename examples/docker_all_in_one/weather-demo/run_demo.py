#!/usr/bin/env python3
"""
Demo script showing how to use the weather-demo environment with Docker Engine mode.

This script demonstrates:
1. Creating an environment instance using Docker containers
2. Listing available tools
3. Calling tools and functions
4. Using reward functions
5. Cleaning up resources
"""

import asyncio
import os
import sys
import time
from typing import Dict, Any

try:
    from aenv import Environment
except ImportError:
    print("Error: aenvironment package not installed")
    print("Install it with: pip install aenvironment")
    sys.exit(1)


def print_section(title: str):
    """Print a formatted section header."""
    print(f"\n{'=' * 60}")
    print(f"  {title}")
    print('=' * 60)


def print_result(action: str, result: Any):
    """Print formatted result."""
    print(f"\n[{action}]")
    print(f"Response: {result}")


async def main():
    """Main demo function."""

    # Set API Service URL (default to localhost)
    api_url = os.environ.get("AENV_SYSTEM_URL", "http://localhost:8080/")
    print_section("AEnvironment Docker Engine Demo")
    print(f"API Service: {api_url}")

    # Configure environment
    os.environ["AENV_SYSTEM_URL"] = api_url

    # Create environment instance
    print_section("Creating Environment Instance")
    print("Environment: weather-demo@1.0.0-docker")

    try:
        env = Environment("weather-demo@1.0.0-docker", timeout=60)
        print("✓ Environment instance created")

        # List available tools
        print_section("Listing Available Tools")
        tools = await env.list_tools()
        print_result("list_tools()", tools)

        # Call get_weather tool
        print_section("Calling Tools")
        weather_beijing = await env.call_tool("get_weather", {"city": "Beijing"})
        print_result("get_weather('Beijing')", weather_beijing)

        weather_shanghai = await env.call_tool("get_weather", {"city": "Shanghai"})
        print_result("get_weather('Shanghai')", weather_shanghai)

        # Call get_weather_func function
        print_section("Calling Functions")
        weather_func = await env.call_function("get_weather_func", {"city": "Hangzhou"})
        print_result("get_weather_func('Hangzhou')", weather_func)

        # Call is_good_weather reward
        print_section("Calling Reward Functions")
        reward = await env.call_reward({"city": "Beijing"})
        print_result("is_good_weather('Beijing')", reward)

        # Keep environment alive for a moment
        print_section("Environment Active")
        print("Environment is running. Waiting 5 seconds...")
        time.sleep(5)

    except Exception as e:
        print(f"\n✗ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

    finally:
        # Clean up
        print_section("Cleaning Up")
        try:
            await env.release()
            print("✓ Environment released successfully")
        except Exception as e:
            print(f"✗ Error releasing environment: {e}")

    print_section("Demo Completed")
    print("✓ All operations completed successfully")
    print()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\n\nDemo interrupted by user")
        sys.exit(0)
