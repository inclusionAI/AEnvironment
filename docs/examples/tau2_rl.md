# Tau2 RL
> Train Your Agent with AReaL & AEnvironment

This example shows how to run a TAU2 task inside **AEnvironment** and expose a single
entrypoint (`run_agent_return_reward`) that can be used by **AReaL** as an RL
trajectory + reward function.

The implementation lives in:

- `aenv/examples/tau2_rl/agent.py`

## What this script does

`agent.py` runs a loop:

1. Create a TAU2 environment (`tau2-env@1.0.0`) via `aenv.core.environment.Environment`
2. Fetch the TAU2 system prompt and available tools from the environment
3. Run an ```OpenAI Agents SDK``` agent turn-by-turn (tools are invoked automatically)
4. Send the agent output back into the environment
5. When the episode ends, call `env.call_reward({})` and return a scalar reward

This makes it suitable as the "episode runner" used in agentic RL.

## Sampling temperature (important for RL)

Following the AReaL agentic RL tutorial, **temperature** is an important knob for
controlling exploration during data collection.

In `aenv/examples/tau2_rl/agent.py`, the sampling parameters are configured here:

- `ModelSettings(temperature=1.0, top_p=1.0, extra_args={"max_completion_tokens": 8192})`

Recommended practice:

- Use **higher temperature** (e.g., `0.8 ~ 1.2`) to collect diverse trajectories.
- Use **lower temperature** (e.g., `0.0 ~ 0.3`) during evaluation to reduce variance.

If you plan to run large-scale training, consider making `temperature` a configurable
argument (so AReaL can sweep it via config).

## Running the agent locally (smoke test)

Install dependencies for this example:

```bash
uv pip install -r aenv/examples/tau2_rl/requirements.txt
```

Run a single episode:

```bash
python aenv/examples/tau2_rl/agent.py --domain telecom --task_id <TASK_ID>
```

Optional environment variables (for using your own LLM in TAU2):

- `TAU2_USER_LLM`
- `TAU2_USER_LLM_API_BASE`
- `TAU2_USER_LLM_API_KEY`

## Using this with AReaL (agentic RL)

In AReaL, you typically configure:

- a reward/rollout function path (Python import path)
- agent sampling parameters (e.g., `temperature`, `max_tokens`, `n_samples`)

For this repository, the reward/rollout entrypoint is:

- `aenv.examples.tau2_rl.agent.run_agent_return_reward`

Example AReaL-style config snippet:

```yaml
# Pseudocode example to mirror AReaL's OpenAI Agents tutorial
reward_fn_path: "aenv.examples.tau2_rl.agent.run_agent_return_reward"

gconfig:
  n_samples: 4
  max_tokens: 8192
  temperature: 1.0
```
