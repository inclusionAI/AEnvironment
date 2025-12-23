"""
Example custom_env.py template for AEnvironment agent.
Copy this to your project's src/custom_env.py and customize as needed.
"""

import os
import json
from typing import Dict, Any
from openai import AsyncOpenAI
from aenv import register_tool

# Initialize OpenAI client (module-level singleton)
_client = None

def get_client():
    """Get or create OpenAI client instance."""
    global _client
    if _client is None:
        _client = AsyncOpenAI(
            api_key=os.getenv("OPENAI_API_KEY"),
            base_url=os.getenv("OPENAI_BASE_URL")
        )
    return _client

@register_tool
async def chat(user_request: str) -> dict:
    """
    Generate HTML code based on user's natural language request.
    
    This tool receives a user's request and uses LLM to generate 
    corresponding HTML code that fulfills the request.
    
    Args:
        user_request: Natural language description of the HTML page to generate
        
    Returns:
        Dictionary with keys:
        - html_code: The generated HTML code as string
        - status: Success status ("success" or "error")
        - message: Status message or error description
    """
    try:
        client = get_client()
        model = os.getenv("OPENAI_MODEL", "gpt-4")
        
        system_prompt = """You are an expert HTML developer. 
Generate clean, modern HTML code based on user requests.
Return only valid HTML code without markdown formatting.
Include inline CSS for styling when appropriate."""
        
        response = await client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_request}
            ],
            temperature=0.7
        )
        
        html_code = response.choices[0].message.content.strip()
        
        # Remove markdown code blocks if present
        if html_code.startswith("```"):
            parts = html_code.split("```")
            if len(parts) >= 3:
                html_code = parts[1]
                if html_code.startswith("html"):
                    html_code = html_code[4:]
                html_code = html_code.strip()
        
        return {
            "html_code": html_code,
            "status": "success",
            "message": "HTML generated successfully"
        }
    except Exception as e:
        return {
            "html_code": "",
            "status": "error",
            "message": f"Error generating HTML: {str(e)}"
        }

