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
Tau2-bench environment integration with AEnv SDK.

This module directly integrates tau2-bench's AgentGymEnv with the aenv SDK,
allowing tau2 environments to be used through the MCP protocol.
"""

import inspect
import json
import logging
import os
from typing import Any, Dict, List

from tau2.gym.gym_agent import AgentGymEnv

from aenv import get_registry, register_function, register_reward

logger = logging.getLogger(__name__)


# Global state
_ENV: AgentGymEnv | None = None
_TASK = None
_TOOLS: list = []
_TOOLS_SCHEMA: list[dict] = []
_POLICY: str = ""
_OBS: str = ""
_REWARDS: list[float] = []
_CURRENT_STEP: int = 0
_EPISODE_DONE: bool = False
_REGISTERED_TOOLS: set[str] = set()

# Configuration
_DOMAIN: str = ""
_TASK_ID: str | None = None
_MAX_STEPS: int = 50
_SOLO_MODE: bool = True

# System prompt templates
STOP_FUNCTION_NAME = "done"

TAU2_AGENT_INSTRUCTION_SOLO = f"""
You are a customer service agent that helps the user according to the <policy> provided below.
You will be provided with a ticket that contains the user's request.
You will need to plan and call the appropriate tools to solve the ticket.

You cannot communicate with the user, only make tool calls.
Stop when you consider that you have solved the ticket.
To do so, send a message containing a single tool call to the `{STOP_FUNCTION_NAME}` tool. Do not include any other tool calls in this last message.

Always follow the policy.
""".strip()

TAU2_SYSTEM_PROMPT_SOLO = """
<instructions>
{agent_instruction}
</instructions>
<policy>
{domain_policy}
</policy>
<ticket>
{ticket}
</ticket>
""".strip()

TAU2_AGENT_INSTRUCTION = """
You are a customer service agent that helps the user according to the <policy> provided below.
In each turn you can either:
- Send a message to the user.
- Make a tool call.
You cannot do both at the same time.

Try to be helpful and always follow the policy.
""".strip()

TAU2_SYSTEM_PROMPT = """
<instructions>
{agent_instruction}
</instructions>
<policy>
{domain_policy}
</policy>
""".strip()


def _get_user_llm_args() -> dict | None:
    """Get user LLM arguments from environment variables."""
    api_base = os.getenv("TAU2_USER_LLM_API_BASE")
    api_key = os.getenv("TAU2_USER_LLM_API_KEY")
    if api_base and api_key:
        return {"api_base": api_base, "api_key": api_key}
    return None


def _create_tool_wrapper(tool_name: str, tool_schema: Dict[str, Any]):
    """
    Create a wrapper function for a tau2 tool that can be registered with MCP.

    Args:
        tool_name: Name of the tau2 tool
        tool_schema: OpenAI function calling schema for the tool

    Returns:
        A wrapper function that executes the tool via env.step()
    """
    func_schema = tool_schema.get("function", {})
    params = func_schema.get("parameters", {}).get("properties", {})
    required = set(func_schema.get("parameters", {}).get("required", []))

    # Type mapping from JSON schema to Python types
    type_mapping = {
        "string": str,
        "integer": int,
        "number": float,
        "boolean": bool,
        "array": list,
        "object": dict,
    }

    # Build parameter list for signature
    parameters = []
    annotations = {}
    param_names = list(params.keys())

    for param_name in param_names:
        param_info = params[param_name]
        param_type = param_info.get("type", "string")
        py_type = type_mapping.get(param_type, str)
        annotations[param_name] = py_type

        # Create parameter with or without default
        if param_name in required:
            param = inspect.Parameter(
                param_name,
                inspect.Parameter.POSITIONAL_OR_KEYWORD,
            )
        else:
            # Use None as default for optional parameters
            param = inspect.Parameter(
                param_name,
                inspect.Parameter.POSITIONAL_OR_KEYWORD,
                default=None,
            )
        parameters.append(param)

    annotations["return"] = Dict[str, Any]

    # Create the wrapper function with explicit signature
    # We use exec to create a function with the exact parameter names
    param_str = ", ".join(param_names) if param_names else ""
    func_code = f"""
def {tool_name}({param_str}) -> Dict[str, Any]:
    '''Execute tau2 tool: {tool_name}'''
    kwargs = {{}}
    for name in {param_names!r}:
        val = locals()[name]
        if val is not None:
            kwargs[name] = val
    try:
        tool_call_action = {{"name": {tool_name!r}, "arguments": kwargs}}
        action_str = json.dumps(tool_call_action)
        obs, reward, done, info = _step(action_str)
        return {{
            "observation": _get_last_obs(),
            "reward": reward,
            "done": done,
            "step": info.get("step", _get_current_step()),
        }}
    except Exception as e:
        return {{"error": str(e), "done": True}}
"""

    # Execute the function definition
    # Use a getter function for _CURRENT_STEP to get the current value at runtime
    def _get_current_step():
        return _CURRENT_STEP

    local_ns: Dict[str, Any] = {
        "Dict": Dict,
        "Any": Any,
        "json": json,
        "_step": _step,
        "_get_last_obs": _get_last_obs,
        "_get_current_step": _get_current_step,
    }
    exec(func_code, local_ns)
    tool_wrapper = local_ns[tool_name]

    # Set metadata
    tool_wrapper.__doc__ = func_schema.get(
        "description", f"Execute tau2 tool: {tool_name}"
    )
    tool_wrapper.__annotations__ = annotations
    tool_wrapper.__signature__ = inspect.Signature(parameters)

    return tool_wrapper


def _register_env_tools():
    """
    Dynamically register all tau2 environment tools as MCP tools.

    This function reads the tools from the initialized environment and
    registers each one as a native MCP tool, allowing direct invocation
    without the wrapper layer.
    """
    global _REGISTERED_TOOLS

    registry = get_registry()

    for tool_schema in _TOOLS_SCHEMA:
        func_info = tool_schema.get("function", {})
        tool_name = func_info.get("name")

        if not tool_name or tool_name in _REGISTERED_TOOLS:
            continue

        # Create and register the wrapper
        wrapper = _create_tool_wrapper(tool_name, tool_schema)
        try:
            print(
                f"Registering tau2 tool: {tool_name}, description: {func_info.get('description')}"
            )
            registry.register(
                wrapper,
                name=tool_name,
                description=func_info.get("description"),
            )
            print(f"Registered tau2 tool: {tool_name}")
            _REGISTERED_TOOLS.add(tool_name)
            logger.info(f"Registered tau2 tool: {tool_name}")
        except ValueError as e:
            # Tool already registered (possibly from a previous session)
            logger.debug(f"Tool {tool_name} already registered: {e}")
            _REGISTERED_TOOLS.add(tool_name)


def init_tau2_env() -> AgentGymEnv:
    """
    Initialize the global tau2 environment.

    Args:
        domain: The tau2 domain name (e.g., 'retail', 'airline', 'telecom')
        task_id: Specific task ID to run
        max_steps: Maximum number of steps per episode
        solo_mode: Whether to use solo mode (no user interaction)
        user_llm: User LLM to use for simulation
        user_llm_args: Arguments for user LLM

    Returns:
        Initialized AgentGymEnv instance
    """
    global _ENV, _TASK, _TOOLS, _TOOLS_SCHEMA, _POLICY, _OBS, _REWARDS
    global _CURRENT_STEP, _EPISODE_DONE, _DOMAIN, _TASK_ID, _MAX_STEPS, _SOLO_MODE
    domain = os.getenv("TAU2_DOMAIN", "telecom")
    task_id = os.getenv("TAU2_TASK_ID")
    max_steps = int(os.getenv("TAU2_MAX_STEPS", "50"))
    solo_mode = os.getenv("TAU2_SOLO_MODE", "true").lower() == "true"
    user_llm_args = _get_user_llm_args()
    user_llm = os.getenv("TAU2_USER_LLM")

    # Store configuration
    _DOMAIN = domain
    _TASK_ID = task_id
    _MAX_STEPS = max_steps
    _SOLO_MODE = solo_mode

    # Try to create environment, fallback to domain_full if needed
    try:
        _ENV = AgentGymEnv(
            domain=domain,
            task_id=task_id,
            max_steps=max_steps,
            solo_mode=solo_mode,
            user_llm=user_llm,
            user_llm_args=user_llm_args,
        )
        obs, info = _ENV.reset()
    except Exception as e:
        logger.warning(
            f"Failed to init with domain={domain}: {e}, retrying with {domain}_full"
        )
        _DOMAIN = f"{domain}_full"
        _ENV = AgentGymEnv(
            domain=_DOMAIN,
            task_id=task_id,
            max_steps=max_steps,
            solo_mode=solo_mode,
            user_llm=user_llm,
            user_llm_args=user_llm_args,
        )
        obs, info = _ENV.reset()

    # Extract environment info
    _TASK = info["task"]
    _TOOLS = info["tools"]
    _TOOLS_SCHEMA = [tool.openai_schema for tool in _TOOLS]
    _POLICY = info["policy"]
    _OBS = obs
    _REWARDS = []
    _CURRENT_STEP = 0
    _EPISODE_DONE = False

    logger.info(f"Initialized tau2 environment: domain={_DOMAIN}, task_id={task_id}")

    # Register env tools as native MCP tools
    _register_env_tools()

    return _ENV


def _get_env() -> AgentGymEnv:
    """Get the global AgentGymEnv instance."""
    global _ENV
    if _ENV is None:
        init_tau2_env()
    return _ENV


def cleanup_tau2_env():
    """Cleanup the global tau2 environment."""
    global _ENV, _REGISTERED_TOOLS, _TASK, _TOOLS, _TOOLS_SCHEMA, _POLICY
    global _OBS, _REWARDS, _CURRENT_STEP, _EPISODE_DONE

    _ENV = None
    _TASK = None
    _TOOLS = []
    _TOOLS_SCHEMA = []
    _POLICY = ""
    _OBS = ""
    _REWARDS = []
    _CURRENT_STEP = 0
    _EPISODE_DONE = False
    _REGISTERED_TOOLS.clear()

    logger.info("Cleaned up tau2 environment")


def _step(action: str) -> tuple[str, float, bool, dict]:
    """
    Execute one step in the environment.

    Args:
        action: Action string (JSON tool call or message)

    Returns:
        Tuple of (observation, reward, done, info)
    """
    global _OBS, _REWARDS, _CURRENT_STEP, _EPISODE_DONE

    env = _get_env()
    try:
        obs, reward, done, _, _ = env.step(action)
        _OBS = obs if obs not in ["", None] else _OBS
        _REWARDS.append(float(reward))
        _CURRENT_STEP += 1
        _EPISODE_DONE = done
        return obs, float(reward), done, {"step": _CURRENT_STEP}
    except Exception as e:
        logger.error(f"Error executing step: {e}")
        _EPISODE_DONE = True
        return f"Error: {e}", 0.0, True, {"error": str(e)}


def _get_last_obs() -> str:
    """Get the last observation, trimming prefixes."""
    obs = _OBS
    if obs:
        if obs.startswith("user: "):
            return obs[len("user: ") :].strip()
        elif obs.startswith("tool: "):
            return obs[len("tool: ") :].strip()
        return obs.strip()
    return ""


init_tau2_env()
# =============================================================================
# Registered Functions (HTTP endpoints for env management)
# =============================================================================


@register_function
def tau2_get_task_info() -> Dict[str, Any]:
    """
    Get current tau2 task information including task details, available tools, and policy.

    Returns:
        Dictionary containing task_id, domain, ticket, tools, policy_preview, max_steps, solo_mode
    """
    return {
        "task_id": _TASK_ID,
        "domain": _DOMAIN,
        "ticket": _TASK.ticket if _TASK else None,
        "tools": [t["function"]["name"] for t in _TOOLS_SCHEMA],
        "policy_preview": _POLICY[:500] + "..." if len(_POLICY) > 500 else _POLICY,
        "max_steps": _MAX_STEPS,
        "solo_mode": _SOLO_MODE,
    }


@register_function
def tau2_get_system_prompt() -> str:
    """
    Get the system prompt for the tau2 environment.

    Returns:
        System prompt string containing instructions and policy
    """
    if _TASK is not None and _TASK.ticket is not None:
        return TAU2_SYSTEM_PROMPT_SOLO.format(
            agent_instruction=TAU2_AGENT_INSTRUCTION_SOLO,
            domain_policy=_POLICY,
            ticket=_TASK.ticket,
        )
    else:
        return TAU2_SYSTEM_PROMPT.format(
            agent_instruction=TAU2_AGENT_INSTRUCTION,
            domain_policy=_POLICY,
        )


@register_function
def tau2_get_available_tools() -> List[Dict[str, Any]]:
    """
    Get the list of available tau2 tools with their schemas.

    Returns:
        List of tool schemas in OpenAI function calling format
    """
    return _TOOLS_SCHEMA


@register_function
def tau2_get_status() -> Dict[str, Any]:
    """
    Get the current status of the tau2 environment.

    Returns:
        Dictionary containing step, max_steps, done, last_observation, rewards, total_reward
    """
    return {
        "step": _CURRENT_STEP,
        "max_steps": _MAX_STEPS,
        "done": _EPISODE_DONE,
        "last_observation": _get_last_obs(),
        "rewards": _REWARDS,
        "total_reward": sum(_REWARDS) if _REWARDS else 0.0,
        "environment": {k: v for k, v in os.environ.items() if k.startswith("TAU2_")},
    }


# =============================================================================
# Registered Tools (MCP tools for agent actions)
# =============================================================================


@register_function
def tau2_send_message(message: str) -> Dict[str, Any]:
    """
    Send a message to the user in non-solo mode.

    Args:
        message: Message to send to the user

    Returns:
        Dictionary containing observation, reward, done
    """
    obs, reward, done, _ = _step(message)
    return {
        "observation": _get_last_obs(),
        "reward": reward,
        "done": done,
    }


@register_function
def tau2_done() -> Dict[str, Any]:
    """
    Signal that the agent has completed the task.

    Returns:
        Dictionary containing done, total_reward, steps, final_observation
    """
    done_action = json.dumps({"name": "done", "arguments": {}})
    obs, reward, done, _ = _step(done_action)
    return {
        "done": True,
        "total_reward": sum(_REWARDS) if _REWARDS else 0.0,
        "steps": _CURRENT_STEP,
        "final_observation": _get_last_obs(),
    }


# =============================================================================
# Registered Reward Function
# =============================================================================


@register_reward
def tau2_reward() -> Dict[str, Any]:
    """
    Get the reward information for the current tau2 episode.

    Returns:
        Dictionary containing rewards, total_reward, done, steps
    """
    return {
        "rewards": _REWARDS,
        "total_reward": sum(_REWARDS) if _REWARDS else 0.0,
        "done": _EPISODE_DONE,
        "steps": _CURRENT_STEP,
    }
