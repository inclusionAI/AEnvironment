from typing import Any, Dict

from agents import Agent as OpenAIAgent
from agents import ModelSettings, RunConfig, Runner, set_default_openai_api
import os

from aenv.core.environment import Environment

set_default_openai_api("chat_completions")


async def run_agent_return_reward(data: Dict[str, Any]) -> float:
    """
    run_agent_return_reward: entrypoint of areal training.

    Args:
        data (dict):
            domain (str): domain name
            task_id (str): task id

    Returns:
        float: reward
    """
    env = None
    try:
        env = Environment(
            env_name="tau2-env@1.0.0",
            environment_variables=dict(
                TAU2_DOMAIN=data.get("domain", "telecom"),
                TAU2_TASK_ID=data.get("task_id", ""),
                TAU2_SOLO_MODE="true" if os.getenv("TAU2_USER_LLM") is None else "false",
                TAU2_USER_LLM_API_BASE=os.getenv("TAU2_USER_LLM_API_BASE", ""),
                TAU2_USER_LLM_API_KEY=os.getenv("TAU2_USER_LLM_API_KEY", ""),
                TAU2_USER_LLM=os.getenv("TAU2_USER_LLM")
            ),
        )

        # Initialize environment and get tools
        await env.initialize()

        # Get system prompt and available tools
        system_prompt = await env.call_function("tau2_get_system_prompt", {})
        print(f"System prompt: {system_prompt}")

        openai_tools = await env.list_openai_tools()
        agent = OpenAIAgent(
            name="Tau2 Agent",
            instructions=system_prompt if isinstance(system_prompt, str) else str(system_prompt),
            tools=openai_tools,
        )

        # Get initial status
        status = await env.call_function("tau2_get_status", {})
        max_steps = status.get("max_steps", 50)

        # Run agent loop
        step = 0
        while step < max_steps:
            step += 1

            # Check if episode is done
            status = await env.call_function("tau2_get_status", {})
            if status.get("done", False):
                break

            # Get last observation for agent input
            last_obs = status.get("last_observation", "")
            current_input = last_obs if last_obs else "Please start working on the task."
            print(f"Current input: {current_input}")

            try:
                # Run agent for one turn (tools are called automatically via MCP)
                result = await Runner.run(
                    agent,
                    input=current_input,
                    run_config=RunConfig(
                        tracing_disabled=True,
                        model_settings=ModelSettings(
                            temperature=1.0,
                            top_p=1.0,
                            extra_args={"max_completion_tokens": 8192},
                        ),
                    ),
                    max_turns=10,
                )

                # Sent message to user
                status = await env.call_function("tau2_send_message", {"message": result.final_output})
                if status.get("done", False):
                    break

            except Exception as e:
                print(f"Agent turn error: {e}")
                continue

        # Get final reward
        reward = await env.call_reward({})
        return float(reward.get("total_reward", 0.0))

    except Exception as e:
        print(f"Error running agent: {e}")
        return 0.0

if __name__ == "__main__":
    """
    You can run this agent locally!
    """
    import asyncio
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--domain", type=str, default="telecom")
    parser.add_argument("--task_id", type=str, default="")
    args = parser.parse_args()
    data = {
        "domain": args.domain,
        "task_id": args.task_id,
    }
    asyncio.run(run_agent_return_reward(data))