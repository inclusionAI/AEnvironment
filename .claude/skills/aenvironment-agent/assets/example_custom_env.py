"""
Example custom_env.py - Complete agent tool implementation template

This file demonstrates best practices for implementing AEnvironment agent tools:
- Proper error handling with try/except
- Consistent return format with status and message
- LLM integration with lazy client initialization
- Type hints for all functions
- Clear docstrings
- Multi-turn conversation support (optional)

Copy this to your project's src/custom_env.py and customize as needed.
"""

import os
import logging
from typing import Dict, Any, Optional
from collections import defaultdict
from openai import AsyncOpenAI
from aenv import register_tool

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Module-level client (singleton pattern for efficiency)
_client: Optional[AsyncOpenAI] = None

# Conversation history storage (for multi-turn conversations)
_conversations = defaultdict(list)


def get_llm_client() -> AsyncOpenAI:
    """
    Get or create OpenAI client instance (lazy initialization).

    Reads configuration from environment variables:
    - OPENAI_API_KEY: Required API key
    - OPENAI_BASE_URL: Optional custom base URL

    Returns:
        AsyncOpenAI client instance
    """
    global _client
    if _client is None:
        api_key = os.getenv("OPENAI_API_KEY")
        base_url = os.getenv("OPENAI_BASE_URL")

        if not api_key:
            raise ValueError("OPENAI_API_KEY environment variable not set")

        _client = AsyncOpenAI(
            api_key=api_key,
            base_url=base_url
        )
        logger.info("Initialized OpenAI client")

    return _client


@register_tool
async def chat(user_request: str, session_id: str = "default") -> Dict[str, Any]:
    """
    Process user requests using LLM and generate responses.

    This tool supports multi-turn conversations by maintaining session history.
    Each session maintains separate conversation context.

    Args:
        user_request: User's input message or request
        session_id: Session identifier for maintaining conversation context

    Returns:
        Dictionary with standardized format:
        - response: LLM-generated response text
        - status: "success" or "error"
        - message: Human-readable status message
        - session_id: Session identifier used

    Example:
        >>> result = await chat("Hello, who are you?")
        >>> print(result["response"])
        "I am an AI assistant..."
    """
    try:
        # Get LLM client
        client = get_llm_client()
        model = os.getenv("OPENAI_MODEL", "gpt-4")

        # Get conversation history for this session
        messages = _conversations[session_id]

        # Add system message if this is first message in session
        if not messages:
            system_prompt = """You are a helpful AI assistant.
            Provide clear, accurate, and helpful responses to user questions.
            Be concise but thorough in your explanations."""
            messages.append({"role": "system", "content": system_prompt})

        # Add user message
        messages.append({"role": "user", "content": user_request})

        # Call LLM
        logger.info(f"Calling LLM with model: {model}")
        response = await client.chat.completions.create(
            model=model,
            messages=messages,
            temperature=0.7,
            max_tokens=2000
        )

        # Extract response
        assistant_message = response.choices[0].message.content.strip()

        # Store assistant response in history
        messages.append({"role": "assistant", "content": assistant_message})

        # Keep last 20 messages to manage memory (10 exchanges)
        if len(messages) > 21:  # 1 system + 20 conversation messages
            # Keep system message and last 20 messages
            _conversations[session_id] = [messages[0]] + messages[-20:]

        logger.info(f"Successfully generated response for session: {session_id}")

        return {
            "response": assistant_message,
            "status": "success",
            "message": "Response generated successfully",
            "session_id": session_id
        }

    except ValueError as e:
        logger.error(f"Configuration error: {e}")
        return {
            "response": "",
            "status": "error",
            "message": f"Configuration error: {str(e)}",
            "session_id": session_id
        }

    except Exception as e:
        logger.exception("Unexpected error in chat tool")
        return {
            "response": "",
            "status": "error",
            "message": f"Error generating response: {str(e)}",
            "session_id": session_id
        }


@register_tool
def reset_conversation(session_id: str = "default") -> Dict[str, Any]:
    """
    Reset conversation history for a specific session.

    Args:
        session_id: Session identifier to reset

    Returns:
        Dictionary with status information

    Example:
        >>> result = reset_conversation("user123")
        >>> print(result["message"])
        "Conversation history cleared"
    """
    try:
        if session_id in _conversations:
            del _conversations[session_id]
            logger.info(f"Reset conversation for session: {session_id}")
            message = f"Conversation history cleared for session: {session_id}"
        else:
            message = f"No conversation history found for session: {session_id}"

        return {
            "status": "success",
            "message": message,
            "session_id": session_id
        }

    except Exception as e:
        logger.exception("Error resetting conversation")
        return {
            "status": "error",
            "message": f"Error resetting conversation: {str(e)}",
            "session_id": session_id
        }


# Additional tool examples:

@register_tool
def get_session_info(session_id: str = "default") -> Dict[str, Any]:
    """
    Get information about a conversation session.

    Args:
        session_id: Session identifier

    Returns:
        Dictionary with session information
    """
    try:
        messages = _conversations.get(session_id, [])

        # Count user and assistant messages (excluding system)
        user_count = sum(1 for m in messages if m.get("role") == "user")
        assistant_count = sum(1 for m in messages if m.get("role") == "assistant")

        return {
            "session_id": session_id,
            "message_count": len(messages),
            "user_messages": user_count,
            "assistant_messages": assistant_count,
            "has_history": len(messages) > 0,
            "status": "success",
            "message": "Session info retrieved"
        }

    except Exception as e:
        logger.exception("Error getting session info")
        return {
            "status": "error",
            "message": f"Error: {str(e)}",
            "session_id": session_id
        }

