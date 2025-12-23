"""
Example client application using AEnvironment SDK.
Copy this to your client directory and customize as needed.
"""

import asyncio
import json
import os
import subprocess
from pathlib import Path
from aenv import Environment


def parse_tool_result(result):
    """Parse ToolResult content into structured data."""
    content = result.content
    response_data = None
    
    if isinstance(content, list):
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                text = item.get("text", "")
                if text:
                    try:
                        response_data = json.loads(text)
                        break
                    except json.JSONDecodeError:
                        response_data = {"raw": text}
                        break
    elif isinstance(content, str):
        try:
            response_data = json.loads(content)
        except json.JSONDecodeError:
            response_data = {"raw": content}
    elif isinstance(content, dict):
        response_data = content
    else:
        raise ValueError(f"Cannot parse content type: {type(content)}")
    
    return response_data


async def main():
    # Get LLM configuration from environment variables
    api_key = os.getenv("OPENAI_API_KEY")
    base_url = os.getenv("OPENAI_BASE_URL")
    model = os.getenv("OPENAI_MODEL", "gpt-4")
    
    if not api_key:
        print("❌ Error: OPENAI_API_KEY not set")
        print("   Please set it: export OPENAI_API_KEY='your-key'")
        return
    
    # Replace with your deployed environment name
    env_name = "your-agent@1.0.0"
    
    print("🚀 Initializing Environment...")
    try:
        async with Environment(
            env_name,
            environment_variables={
                "OPENAI_API_KEY": api_key,
                "OPENAI_BASE_URL": base_url,
                "OPENAI_MODEL": model
            },
            timeout=120
        ) as env:
            # List available tools
            tools = await env.list_tools()
            print("✅ Environment ready. Available tools:")
            for tool in tools:
                print(f"   - {tool['name']}: {tool.get('description', '')}")
            
            # Interactive loop
            print("\n" + "="*50)
            while True:
                user_request = input("\n📝 Enter HTML generation request (or 'quit' to exit): ")
                if user_request.lower() in ['quit', 'exit', 'q']:
                    break
                
                if not user_request.strip():
                    continue
                
                print("⏳ Generating HTML...")
                try:
                    result = await env.call_tool("chat", {"user_request": user_request})
                    data = parse_tool_result(result)
                    
                    if data.get("status") == "success":
                        html_code = data.get("html_code", "")
                        if html_code:
                            output_file = Path("output.html")
                            output_file.write_text(html_code, encoding="utf-8")
                            print(f"✅ HTML generated and saved to: {output_file.absolute()}")
                            
                            # Open in browser
                            try:
                                subprocess.run(["open", "-a", "Google Chrome", str(output_file)])
                                print("🌐 Opened in Chrome browser")
                            except Exception as e:
                                print(f"⚠️  Could not open browser: {e}")
                        else:
                            print("⚠️  No HTML code in response")
                    else:
                        print(f"❌ Error: {data.get('message', 'Unknown error')}")
                        
                except Exception as e:
                    print(f"❌ Error calling tool: {e}")
                    import traceback
                    traceback.print_exc()
    
    except Exception as e:
        print(f"❌ Environment error: {e}")
        import traceback
        traceback.print_exc()


if __name__ == "__main__":
    asyncio.run(main())

