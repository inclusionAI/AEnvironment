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

import asyncio
import logging
import os
import time
from pathlib import Path
from typing import Dict, List

from mcp.types import TextContent
from terminal_bench.handlers.trial_handler import Task
from terminal_bench.parsers.base_parser import UnitTestStatus
from terminal_bench.parsers.parser_factory import ParserFactory
from tmuxsession import TmuxSession

from aenv import register_reward, register_tool

logger = logging.getLogger(__name__)


_SESSION = TmuxSession.from_nerdctl_name(
    session_name="demo",
    namespace_list=["k8s.io", "default"],
    container_name="second",
    commands_path=Path("/tmp/cmd.log"),
    disable_recording=False,
)
if not _SESSION.is_session_alive():
    _SESSION.start()


@register_tool
def keystrokes(
    keystrokes: str, append_enter: bool = False, wait_time_sec: float = 0.0
) -> List[TextContent]:
    """
    Send keystrokes to the tmux session terminal.

    Args:
        keystrokes: Keystrokes to execute in the terminal. Use tmux-style escape sequences for special characters (e.g. C-c for ctrl-c).
        append_enter: Whether to append a newline character to the end of the keystrokes. (This is necessary to execute bash commands.)
        wait_time_sec: The number of expected seconds to wait for the command to complete.

    Returns:
        List of TextContent with terminal output
    """

    keys = [keystrokes, "Enter"] if append_enter else keystrokes

    _SESSION.send_keys(
        keys=keys,
        min_timeout_sec=wait_time_sec,
        max_timeout_sec=wait_time_sec,
        block=True,
    )

    pane_content = _SESSION.capture_pane(capture_entire=True)

    return [TextContent(type="text", text=pane_content)]


@register_tool
async def capture_pane(wait_before_capture_sec: float = 0.0) -> List[TextContent]:
    """
    Capture the current content of the tmux pane.

    Args:
        wait_before_capture_sec: The number of seconds to wait before capturing the pane. This is useful if you just executed a command and want to wait a bit to capture the output.

    Returns:
        List of TextContent with captured pane content
    """
    if wait_before_capture_sec > 0:
        await asyncio.sleep(wait_before_capture_sec)

    result = _SESSION.capture_pane()

    return [TextContent(type="text", text=result)]


@register_reward
def caculate_reward(task_name: str):
    op_start = time.time()
    tasks_dir = Path(os.environ.get("T_BENCH_TASKS_DIR", "/data/tb"))
    task_dir = tasks_dir / task_name
    task_config_path = task_dir / "task.yaml"

    max_test_timeout_sec = 60.0  # Default timeout
    parser_name = "pytest"  # Default parser

    if task_config_path.exists():
        try:
            task = Task.from_yaml(task_config_path)
            max_test_timeout_sec = task.max_test_timeout_sec
            parser_name = task.parser_name
        except Exception as e:
            logger.warning(f"Failed to load task config for {task_name}: {e}")

    # Copy test files to container
    run_tests_path = task_dir / "run-tests.sh"
    test_dir = task_dir / "tests"

    if not run_tests_path.exists():
        return {
            "task_name": task_name,
            "status": "error",
            "error": f"Test script not found: {run_tests_path}",
        }

    try:
        paths_to_copy = [run_tests_path, test_dir]
        if test_dir.exists():
            paths_to_copy.append(test_dir)

        _SESSION._copy_into_container(host_paths=paths_to_copy)
        logger.info(f"Copied test files to task {task_name}")
    except Exception as e:
        logger.error(f"Failed to copy test files: {e}")
        return {
            "task_name": task_name,
            "status": "error",
            "error": f"Failed to copy test files: {str(e)}",
        }

    # Run test script
    test_script_path = "/shared/run-tests.sh"

    logger.info(
        f"Running tests for container {task_name} with timeout {max_test_timeout_sec}s"
    )

    try:
        run_tests_start = time.time()
        _SESSION.send_keys(
            [
                f"export TEST_DIR=/shared/tests && bash {test_script_path}",
                "Enter",
            ],
            block=True,
            max_timeout_sec=max_test_timeout_sec,
        )
        log_operation_time(
            f"validate_container run tests for {task_name}", run_tests_start
        )
    except TimeoutError:
        logger.warning(f"Test timeout for container {task_name}")
        log_operation_time(f"validate_container (timeout) for {task_name}", op_start)
        return {
            "task_name": task_name,
            "status": "timeout",
            "error": f"Test execution timed out after {max_test_timeout_sec} seconds",
        }

    # Capture test output
    capture_start = time.time()
    test_output = _SESSION.capture_pane(capture_entire=True)
    log_operation_time(
        f"validate_container capture output for {task_name}", capture_start
    )
    score = 0.0
    # Parse test results
    try:
        parser = ParserFactory.get_parser(parser_name)
        results = parser.parse(test_output)

        # Calculate weighted score
        score = _calculate_weighted_test_score(results, None)

        log_operation_time(
            f"validate_container (success) for {task_name} with score {score}", op_start
        )
        #  status_code=200,
        return {
            "task_name": task_name,
            "status": "success",
            "score": score,
            "raw_output": test_output,
        }

    except Exception as e:
        logger.error(f"Error parsing test results for {task_name}: {e}")
        log_operation_time(
            f"validate_container (parse_error) for {task_name}", op_start
        )
        return {
            "task_name": task_name,
            "status": "parse_error",
            "error": f"Failed to parse test results: {str(e)}",
            "raw_output": test_output,
        }


def _calculate_weighted_test_score(
    results: Dict[str, UnitTestStatus],
    test_weights: Dict[str, float] | None,
) -> float:
    """
    Calculate weighted score from test results.

    Args:
        results: Test name to status mapping
        test_weights: Test name to weight mapping

    Returns:
        Weighted score between 0.0 and 1.0
    """
    if not results:
        return 0.0

    # If no test weights provided or only placeholder, use equal weights
    # Filter out placeholder key used when test_weights.json doesn't exist
    filtered_weights = {
        k: v for k, v in (test_weights or {}).items() if not k.startswith("_")
    }

    if not filtered_weights:
        equal_weight = 1.0 / len(results)
        total_score = sum(
            equal_weight if status == UnitTestStatus.PASSED else 0.0
            for status in results.values()
        )
        return total_score

    # Calculate weighted score
    total_score = 0.0
    total_weight = 0.0

    for test_name, status in results.items():
        weight = filtered_weights.get(test_name, 0.0)
        if weight > 0:
            score = 1.0 if status == UnitTestStatus.PASSED else 0.0
            total_score += score * weight
            total_weight += weight

    # Normalize if weights don't sum to 1.0
    if total_weight > 0:
        return total_score / total_weight

    equal_weight = 1.0 / len(results)
    return sum(
        equal_weight if status == UnitTestStatus.PASSED else 0.0
        for status in results.values()
    )


def log_operation_time(operation_name: str, start_time: float):
    """Log the time taken for an operation."""
    duration = time.time() - start_time
    logger.info(f"[PERF] {operation_name} took {duration:.3f}s ({duration*1000:.1f}ms)")
