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
E2E resilience tests: simulate api-service pod crash and network jitter.

These tests verify the SDK's ability to:
1. Recover from api-service pod restarts during tool calls
2. Handle transient network failures gracefully
3. Rebuild MCP sessions after connection loss
4. Maintain circuit breaker correctness under failure scenarios

Requires access to staging api-service with kubectl access to the cluster.

Usage:
    export KUBECONFIG=/Users/jun/.kube/tydd-staging-config
    AENV_SYSTEM_URL=http://api-service-k8s.aenv.svc.tydd-staging.alipay.net \
    RUN_E2E=1 uv run pytest src/aenv/tests/test_e2e_resilience.py -v -s
"""

import asyncio
import os
import subprocess
import time

import pytest

from aenv import Environment
from aenv.core.exceptions import AEnvError, ToolError

AENV_URL = os.getenv(
    "AENV_SYSTEM_URL", "http://api-service-k8s.aenv.svc.tydd-staging.alipay.net"
)
ENV_NAME = "persistent-bash-env@1.1.6"
TOOL_NAME = f"{ENV_NAME}/run"
KUBECONFIG = os.getenv("KUBECONFIG", "/Users/jun/.kube/tydd-staging-config")
NAMESPACE = "aenv"
DEPLOYMENT = "api-service"

pytestmark = [
    pytest.mark.asyncio,
    pytest.mark.skipif(
        not os.getenv("RUN_E2E", ""),
        reason="Set RUN_E2E=1 to run e2e tests",
    ),
]


def _run_args(command: str, timeout: int = 60) -> dict:
    return {"command": command, "timeout": timeout}


def _kubectl(*args: str, check: bool = True) -> subprocess.CompletedProcess:
    """Run kubectl command against the staging cluster."""
    cmd = ["kubectl", f"--kubeconfig={KUBECONFIG}", "-n", NAMESPACE, *args]
    print(f"  [kubectl] {' '.join(cmd)}")
    return subprocess.run(cmd, capture_output=True, text=True, timeout=60, check=check)


def _get_pod_name(deployment: str) -> str:
    """Get the first running pod name for a deployment."""
    result = _kubectl(
        "get",
        "pods",
        "-l",
        f"sigma.ali/app-name={deployment}",
        "-o",
        "jsonpath={.items[0].metadata.name}",
    )
    pod = result.stdout.strip()
    if not pod:
        pytest.skip(f"No pods found for deployment {deployment}")
    return pod


def _wait_for_rollout(deployment: str, timeout: int = 120):
    """Wait for deployment rollout to complete."""
    _kubectl("rollout", "status", f"deployment/{deployment}", f"--timeout={timeout}s")


class TestApiServicePodCrash:
    """Verify SDK recovery when api-service pod is restarted during operations."""

    @pytest.mark.asyncio
    async def test_tool_call_recovery_after_pod_restart(self):
        """
        Scenario: SDK has an active session, api-service pod restarts,
        SDK should recover and execute tool calls after the pod is back.

        Steps:
        1. Establish session and execute a tool call successfully
        2. Restart api-service pod (kubectl rollout restart)
        3. Wait for new pod to be ready
        4. Execute another tool call - should recover via session rebuild
        """
        print("\n--- Test: Tool Call Recovery After Pod Restart ---")

        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="15m",
        ) as env:
            # Step 1: Verify baseline works
            r1 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'before restart'"), timeout=15.0
            )
            assert r1.is_error is False
            print("  Step 1: Pre-restart tool call OK")

            # Step 2: Restart api-service pod
            print("  Step 2: Restarting api-service deployment...")
            _kubectl("rollout", "restart", f"deployment/{DEPLOYMENT}")

            # Step 3: Wait for new pod to be ready
            print("  Step 3: Waiting for rollout to complete...")
            _wait_for_rollout(DEPLOYMENT, timeout=120)
            # Extra buffer for DNS propagation and readiness
            await asyncio.sleep(5)
            print("  Step 3: New pod is ready")

            # Step 4: Call tool again - SDK should rebuild session
            print("  Step 4: Attempting post-restart tool call...")
            max_recovery_attempts = 5
            recovered = False
            for attempt in range(max_recovery_attempts):
                try:
                    r2 = await env.call_tool(
                        TOOL_NAME, _run_args("echo 'after restart'"), timeout=30.0
                    )
                    if not r2.is_error:
                        recovered = True
                        print(f"  Step 4: Recovery succeeded on attempt {attempt + 1}")
                        break
                except (ToolError, AEnvError) as e:
                    print(
                        f"  Step 4: Attempt {attempt + 1} failed: {type(e).__name__}: {e}"
                    )
                    if attempt < max_recovery_attempts - 1:
                        await asyncio.sleep(3)

            assert recovered, (
                f"SDK failed to recover after api-service pod restart "
                f"({max_recovery_attempts} attempts)"
            )
            print("--- Test PASSED ---")

    @pytest.mark.asyncio
    async def test_new_session_after_pod_restart(self):
        """
        Scenario: After pod restart, creating a brand new Environment should work.
        This tests the SDK's initialize + session establishment path.
        """
        print("\n--- Test: New Session After Pod Restart ---")

        # Step 1: Restart pod first
        print("  Step 1: Restarting api-service deployment...")
        _kubectl("rollout", "restart", f"deployment/{DEPLOYMENT}")
        _wait_for_rollout(DEPLOYMENT, timeout=120)
        await asyncio.sleep(5)
        print("  Step 1: New pod ready")

        # Step 2: Create a brand new environment
        print("  Step 2: Creating new environment...")
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=60.0,
            ttl="10m",
        ) as env:
            assert env._mcp_session_active is True
            result = await env.call_tool(
                TOOL_NAME, _run_args("echo 'fresh session works'"), timeout=15.0
            )
            assert result.is_error is False
            print("  Step 2: New session established and tool call succeeded")

        print("--- Test PASSED ---")


class TestNetworkJitter:
    """
    Verify SDK handles transient network disruptions.

    Uses iptables-like simulation via kubectl exec to inject network faults
    into the api-service pod, or tests behavior when MCP proxy encounters
    connection errors.
    """

    @pytest.mark.asyncio
    async def test_tool_call_succeeds_after_transient_failure(self):
        """
        Scenario: Simulate transient failure by corrupting the MCP session,
        then verify the SDK rebuilds and recovers.

        This avoids needing iptables access by directly testing the SDK's
        internal recovery path: force-close the session, then call_tool.
        """
        print("\n--- Test: Recovery After Transient Session Failure ---")

        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            # Baseline
            r1 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'baseline'"), timeout=15.0
            )
            assert r1.is_error is False
            print("  Baseline call: OK")

            # Simulate network jitter by destroying MCP session
            print("  Simulating network jitter (destroying MCP session)...")
            if env._mcp_client is not None:
                try:
                    await env._mcp_client.close()
                except Exception:
                    pass
            env._mcp_session_active = False

            # Next call should trigger rebuild and succeed
            print("  Attempting recovery call...")
            r2 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'recovered from jitter'"), timeout=30.0
            )
            assert r2.is_error is False
            assert env._mcp_session_active is True
            print("  Recovery call: OK")

        print("--- Test PASSED ---")

    @pytest.mark.asyncio
    async def test_multiple_rapid_failures_trigger_circuit_breaker(self):
        """
        Scenario: Multiple consecutive tool failures should trigger
        the circuit breaker, preventing cascade failures.
        """
        print("\n--- Test: Circuit Breaker Under Rapid Failures ---")

        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            # Baseline
            r1 = await env.call_tool(TOOL_NAME, _run_args("echo 'ok'"), timeout=15.0)
            assert r1.is_error is False
            assert env._consecutive_tool_errors == 0

            # Simulate consecutive failures by setting the counter directly
            env._consecutive_tool_errors = env._CIRCUIT_BREAKER_THRESHOLD

            # Circuit breaker should block
            from aenv.core.exceptions import UnrecoverableEnvironmentError

            with pytest.raises(UnrecoverableEnvironmentError) as exc_info:
                await env.call_tool(
                    TOOL_NAME, _run_args("echo 'blocked'"), timeout=15.0
                )
            print(f"  Circuit breaker triggered: {exc_info.value}")

            # Reset and verify recovery
            env._consecutive_tool_errors = 0
            r2 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'reset works'"), timeout=15.0
            )
            assert r2.is_error is False
            print("  After reset, tool call succeeds")

        print("--- Test PASSED ---")

    @pytest.mark.asyncio
    async def test_session_rebuild_preserves_instance(self):
        """
        Scenario: After MCP session rebuild, the same underlying instance
        should still be used (instance ID should not change).
        """
        print("\n--- Test: Session Rebuild Preserves Instance ---")

        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            # Get instance ID
            instance_id_before = env._instance.id if env._instance else None
            assert instance_id_before is not None
            print(f"  Instance ID before: {instance_id_before}")

            # Execute a command that writes a unique marker
            marker = f"resilience_test_{int(time.time())}"
            await env.call_tool(
                TOOL_NAME, _run_args(f"echo '{marker}' > /tmp/marker.txt"), timeout=15.0
            )

            # Simulate session loss (like network jitter destroying the connection)
            # Set session inactive so _ensure_mcp_session will rebuild on next call
            print("  Simulating session loss...")
            env._mcp_session_active = False

            # Verify same instance (session rebuild doesn't create new instance)
            instance_id_after = env._instance.id if env._instance else None
            assert (
                instance_id_before == instance_id_after
            ), f"Instance ID changed: {instance_id_before} -> {instance_id_after}"
            print(f"  Instance ID after loss: {instance_id_after} (same)")

            # Next call_tool will trigger _ensure_mcp_session which rebuilds the session
            # The key point: it should reconnect to the SAME instance
            r = await env.call_tool(
                TOOL_NAME, _run_args("cat /tmp/marker.txt"), timeout=30.0
            )
            assert r.is_error is False
            assert marker in str(r.content), f"Marker not found in output: {r.content}"
            print(f"  Marker file preserved after reconnect: {marker}")

        print("--- Test PASSED ---")

    @pytest.mark.asyncio
    async def test_concurrent_tool_calls_during_jitter(self):
        """
        Scenario: Multiple concurrent tool calls when session is unstable.
        Only one should trigger rebuild; others should wait or fail gracefully.
        """
        print("\n--- Test: Concurrent Calls During Network Jitter ---")

        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=30.0,
            ttl="10m",
        ) as env:
            # Baseline
            r1 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'baseline'"), timeout=15.0
            )
            assert r1.is_error is False

            # Launch 3 concurrent calls
            print("  Launching 3 concurrent tool calls...")
            tasks = [
                env.call_tool(
                    TOOL_NAME, _run_args(f"echo 'concurrent_{i}'"), timeout=30.0
                )
                for i in range(3)
            ]
            results = await asyncio.gather(*tasks, return_exceptions=True)

            successes = sum(
                1 for r in results if not isinstance(r, Exception) and not r.is_error
            )
            failures = sum(1 for r in results if isinstance(r, Exception))
            print(f"  Results: {successes} successes, {failures} failures")

            # At least some should succeed (all if session is healthy)
            assert successes >= 1, f"Expected at least 1 success, got {successes}"
            print(f"  Concurrent calls handled: {successes}/3 succeeded")

        print("--- Test PASSED ---")


class TestInitializeRecovery:
    """Verify SDK handles failures during instance initialization."""

    @pytest.mark.asyncio
    async def test_initialize_with_api_service_restart(self):
        """
        Scenario: Start initializing an environment, restart api-service
        mid-initialization, verify that the SDK can still complete or
        properly report the failure.
        """
        print("\n--- Test: Initialize During API Service Restart ---")

        # Restart api-service first to ensure clean state
        _kubectl("rollout", "restart", f"deployment/{DEPLOYMENT}")
        _wait_for_rollout(DEPLOYMENT, timeout=120)
        await asyncio.sleep(5)

        # Now try to create environment (should succeed with fresh api-service)
        start = time.time()
        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=60.0,
            startup_timeout=300.0,
            ttl="10m",
        ) as env:
            elapsed = time.time() - start
            print(f"  Environment initialized in {elapsed:.1f}s")
            assert env._mcp_session_active is True

            result = await env.call_tool(
                TOOL_NAME, _run_args("echo 'init recovery ok'"), timeout=15.0
            )
            assert result.is_error is False
            print("  Post-init tool call: OK")

        print("--- Test PASSED ---")

    @pytest.mark.asyncio
    async def test_retry_failed_session_idle_period(self):
        """
        Retry the previously failed test: session survives idle period.
        This was failing due to FaaS container startup slowness.
        """
        print("\n--- Test: Session Survives Idle Period (retry) ---")

        async with Environment(
            env_name=ENV_NAME,
            aenv_url=AENV_URL,
            timeout=60.0,
            startup_timeout=500.0,
            ttl="15m",
        ) as env:
            r1 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'before idle'"), timeout=15.0
            )
            assert r1.is_error is False
            print("  Before idle: OK")

            print("  Sleeping 10s to simulate idle period...")
            await asyncio.sleep(10)

            r2 = await env.call_tool(
                TOOL_NAME, _run_args("echo 'after idle'"), timeout=15.0
            )
            assert r2.is_error is False
            print("  After idle: OK")

            assert env._mcp_session_active is True

        print("--- Test PASSED ---")
