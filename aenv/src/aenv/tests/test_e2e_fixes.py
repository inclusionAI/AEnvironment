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
E2E tests for PR #71 fixes: session persistence, circuit breaker, __aenter__ retry.

Requires access to staging api-service. Set AENV_SYSTEM_URL env var.

Usage:
    AENV_SYSTEM_URL=http://6.3.209.180 RUN_E2E=1 uv run pytest src/aenv/tests/test_e2e_fixes.py -v -s
"""

import asyncio
import os
import time

import pytest

from aenv import Environment
from aenv.core.exceptions import UnrecoverableEnvironmentError

AENV_URL = os.getenv("AENV_SYSTEM_URL", "http://6.3.209.180")
ENV_NAME = "persistent-bash-env@1.1.6"
# Tool name as registered in persistent_bash_session: "run"
TOOL_NAME = f"{ENV_NAME}/run"

pytestmark = [
    pytest.mark.asyncio,
    pytest.mark.skipif(
        not os.getenv("RUN_E2E", ""),
        reason="Set RUN_E2E=1 to run e2e tests",
    ),
]


def _run_args(command: str, timeout: int = 60) -> dict:
    """Build arguments for the persistent-bash-session 'run' tool."""
    return {"command": command, "timeout": timeout}


class TestSessionPersistence:
    """Verify MCP session is reused across multiple tool calls (fix.md core fix)."""

    @pytest.mark.asyncio
    async def test_multiple_tool_calls_reuse_session(self):
        """Multiple call_tool invocations should use the same MCP session."""
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            assert env._mcp_session_active is True

            tools = await env.list_tools()
            assert len(tools) > 0
            print(f"\n  Found {len(tools)} tools: {[t['name'] for t in tools]}")

            for i in range(5):
                result = await env.call_tool(
                    TOOL_NAME,
                    _run_args(f"echo 'e2e test iteration {i}'"),
                    timeout=15.0,
                )
                assert result.is_error is False
                print(f"  call_tool #{i}: ok")

            assert env._mcp_session_active is True
            assert env._consecutive_tool_errors == 0

    @pytest.mark.asyncio
    async def test_session_survives_idle_period(self):
        """Session should survive a short idle period without initialize/DELETE cycle."""
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            r1 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'before idle'"), timeout=15.0
            )
            assert r1.is_error is False
            print("\n  Before idle: ok")

            print("  Sleeping 10s to simulate idle period...")
            await asyncio.sleep(10)

            r2 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'after idle'"), timeout=15.0
            )
            assert r2.is_error is False
            print("  After idle: ok")

            assert env._mcp_session_active is True


class TestCircuitBreakerE2E:
    """Verify circuit breaker prevents cascade failures."""

    @pytest.mark.asyncio
    async def test_circuit_breaker_counter_resets_on_success(self):
        """Successful calls should reset the consecutive error counter."""
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            result = await env.call_tool(
                TOOL_NAME, _run_args("echo 'success'"), timeout=15.0
            )
            assert result.is_error is False
            assert env._consecutive_tool_errors == 0

            # Simulate prior failures then successful call should reset
            env._consecutive_tool_errors = 4
            result = await env.call_tool(
                TOOL_NAME, _run_args("echo 'reset'"), timeout=15.0
            )
            assert result.is_error is False
            assert env._consecutive_tool_errors == 0
            print(f"\n  Counter reset verified: {env._consecutive_tool_errors}")

    @pytest.mark.asyncio
    async def test_circuit_breaker_blocks_after_threshold(self):
        """Setting counter to threshold should block next call."""
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            env._consecutive_tool_errors = env._CIRCUIT_BREAKER_THRESHOLD

            with pytest.raises(UnrecoverableEnvironmentError) as exc_info:
                await env.call_tool(
                    TOOL_NAME,
                    _run_args("echo 'should not execute'"),
                    timeout=15.0,
                )

            assert "Circuit breaker open" in str(exc_info.value)
            print(f"\n  Circuit breaker triggered: {exc_info.value}")


class TestToolFailureRebuild:
    """Verify tool failure triggers MCP client rebuild and recovery."""

    @pytest.mark.asyncio
    async def test_recovery_after_tool_error(self):
        """After a tool error, the next call should work via rebuilt client."""
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            r1 = await env.call_tool(TOOL_NAME, _run_args("echo 'ok'"), timeout=15.0)
            assert r1.is_error is False

            # Call a non-existent tool to trigger error + rebuild
            try:
                await env.call_tool(
                    f"{ENV_NAME}/nonexistent_tool_xyz",
                    {"arg": "should fail"},
                    timeout=10.0,
                )
            except Exception as e:
                print(f"\n  Expected error on bad tool: {type(e).__name__}: {e}")
                assert env._consecutive_tool_errors >= 1

            # Recovery: next call should work (client was rebuilt)
            r2 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'recovered'"), timeout=15.0
            )
            assert r2.is_error is False
            assert env._consecutive_tool_errors == 0
            print("  Recovery verified: ok")


class TestAenterRetryE2E:
    """Verify __aenter__ session establishment with retry."""

    @pytest.mark.asyncio
    async def test_normal_aenter_succeeds(self):
        """Normal __aenter__ should establish session successfully."""
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            assert env._mcp_session_active is True
            result = await env.call_tool(
                TOOL_NAME, _run_args("echo 'aenter ok'"), timeout=15.0
            )
            assert result.is_error is False
            print("\n  __aenter__ success: ok")


class TestEndToEndWorkflow:
    """Full workflow test simulating real agent usage."""

    @pytest.mark.asyncio
    async def test_full_agent_workflow(self):
        """Simulate a real agent workflow: create env, list tools, execute
        multiple commands, idle, execute more, release."""
        print("\n--- Full E2E Workflow ---")
        start = time.time()

        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            # Step 1: List tools
            tools = await env.list_tools()
            tool_names = [t["name"] for t in tools]
            print(f"  1. Listed {len(tools)} tools: {tool_names}")
            assert len(tools) > 0

            # Step 2: Execute a sequence of commands
            commands = [
                "pwd",
                "ls -la",
                "echo 'hello world' > /tmp/test.txt",
                "cat /tmp/test.txt",
                "rm /tmp/test.txt && echo 'cleaned up'",
            ]
            for cmd in commands:
                result = await env.call_tool(TOOL_NAME, _run_args(cmd), timeout=15.0)
                assert result.is_error is False
                print(f"  2. cmd='{cmd}' -> ok")

            # Step 3: Idle (simulate LLM thinking)
            print("  3. Idle 5s (simulating LLM thinking)...")
            await asyncio.sleep(5)

            # Step 4: More commands after idle
            result = await env.call_tool(
                TOOL_NAME, _run_args("echo 'still alive after idle'"), timeout=15.0
            )
            assert result.is_error is False
            print("  4. Post-idle call: ok")

            # Verify session health
            assert env._mcp_session_active is True
            assert env._consecutive_tool_errors == 0

        elapsed = time.time() - start
        print(f"  Total elapsed: {elapsed:.1f}s")
        print("--- Workflow Complete ---")
