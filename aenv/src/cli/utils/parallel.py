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
Parallel execution utilities for aenv CLI

Provides concurrent task execution with graceful error handling and fallback.
"""

import os
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Optional, Tuple


@dataclass
class TaskResult:
    """Result of a parallel task execution."""

    name: str
    success: bool
    result: Any = None
    error: Optional[Exception] = None


def is_parallel_disabled() -> bool:
    """Check if parallel execution is disabled via environment variable."""
    return os.environ.get("AENV_DISABLE_PARALLEL", "").lower() in ("1", "true", "yes")


def parallel_execute(
    tasks: List[Tuple[str, Callable[[], Any]]],
    timeout: Optional[float] = None,
    max_workers: Optional[int] = None,
) -> Dict[str, TaskResult]:
    """
    Execute multiple tasks in parallel using ThreadPoolExecutor.

    Args:
        tasks: List of (name, callable) tuples to execute
        timeout: Optional timeout for each task in seconds
        max_workers: Maximum number of worker threads (default: min(len(tasks), 4))

    Returns:
        Dictionary mapping task names to TaskResult objects

    Example:
        tasks = [
            ("check_env", lambda: client.check_env(name, version)),
            ("state_env", lambda: client.state_environment(name, version)),
        ]
        results = parallel_execute(tasks)
        check_result = results["check_env"].result
    """
    if not tasks:
        return {}

    if is_parallel_disabled():
        return _execute_sequential(tasks)

    results: Dict[str, TaskResult] = {}
    max_workers = max_workers or min(len(tasks), 4)

    try:
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_name = {
                executor.submit(task_fn): name for name, task_fn in tasks
            }

            for future in as_completed(future_to_name, timeout=timeout):
                name = future_to_name[future]
                try:
                    result = future.result()
                    results[name] = TaskResult(
                        name=name, success=True, result=result
                    )
                except Exception as e:
                    results[name] = TaskResult(
                        name=name, success=False, error=e
                    )

    except Exception:
        # Fallback to sequential execution if parallel fails
        return _execute_sequential(tasks)

    return results


def _execute_sequential(
    tasks: List[Tuple[str, Callable[[], Any]]]
) -> Dict[str, TaskResult]:
    """Execute tasks sequentially as fallback."""
    results: Dict[str, TaskResult] = {}

    for name, task_fn in tasks:
        try:
            result = task_fn()
            results[name] = TaskResult(name=name, success=True, result=result)
        except Exception as e:
            results[name] = TaskResult(name=name, success=False, error=e)

    return results
