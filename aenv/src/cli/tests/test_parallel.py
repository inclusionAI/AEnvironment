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

"""Tests for parallel execution utilities."""

import os
import time
from unittest.mock import patch

import pytest

from cli.utils.parallel import (
    TaskResult,
    is_parallel_disabled,
    parallel_execute,
    _execute_sequential,
)


class TestIsParallelDisabled:
    def test_disabled_when_env_is_1(self):
        with patch.dict(os.environ, {"AENV_DISABLE_PARALLEL": "1"}):
            assert is_parallel_disabled() is True

    def test_disabled_when_env_is_true(self):
        with patch.dict(os.environ, {"AENV_DISABLE_PARALLEL": "true"}):
            assert is_parallel_disabled() is True

    def test_disabled_when_env_is_yes(self):
        with patch.dict(os.environ, {"AENV_DISABLE_PARALLEL": "yes"}):
            assert is_parallel_disabled() is True

    def test_not_disabled_when_env_is_0(self):
        with patch.dict(os.environ, {"AENV_DISABLE_PARALLEL": "0"}):
            assert is_parallel_disabled() is False

    def test_not_disabled_when_env_not_set(self):
        with patch.dict(os.environ, {}, clear=True):
            os.environ.pop("AENV_DISABLE_PARALLEL", None)
            assert is_parallel_disabled() is False


class TestParallelExecute:
    def test_empty_tasks(self):
        results = parallel_execute([])
        assert results == {}

    def test_single_task_success(self):
        tasks = [("task1", lambda: "result1")]
        results = parallel_execute(tasks)

        assert "task1" in results
        assert results["task1"].success is True
        assert results["task1"].result == "result1"
        assert results["task1"].error is None

    def test_multiple_tasks_success(self):
        tasks = [
            ("task1", lambda: "result1"),
            ("task2", lambda: "result2"),
            ("task3", lambda: "result3"),
        ]
        results = parallel_execute(tasks)

        assert len(results) == 3
        for i in range(1, 4):
            name = f"task{i}"
            assert results[name].success is True
            assert results[name].result == f"result{i}"

    def test_task_with_exception(self):
        def failing_task():
            raise ValueError("test error")

        tasks = [
            ("success_task", lambda: "ok"),
            ("failing_task", failing_task),
        ]
        results = parallel_execute(tasks)

        assert results["success_task"].success is True
        assert results["success_task"].result == "ok"

        assert results["failing_task"].success is False
        assert isinstance(results["failing_task"].error, ValueError)
        assert str(results["failing_task"].error) == "test error"

    def test_tasks_run_concurrently(self):
        def slow_task(delay, result):
            time.sleep(delay)
            return result

        tasks = [
            ("task1", lambda: slow_task(0.1, "result1")),
            ("task2", lambda: slow_task(0.1, "result2")),
        ]

        start_time = time.time()
        results = parallel_execute(tasks)
        elapsed = time.time() - start_time

        assert results["task1"].success is True
        assert results["task2"].success is True
        assert elapsed < 0.2

    def test_respects_disable_flag(self):
        call_order = []

        def task1():
            call_order.append("task1")
            return "result1"

        def task2():
            call_order.append("task2")
            return "result2"

        tasks = [("task1", task1), ("task2", task2)]

        with patch.dict(os.environ, {"AENV_DISABLE_PARALLEL": "1"}):
            results = parallel_execute(tasks)

        assert results["task1"].success is True
        assert results["task2"].success is True
        assert call_order == ["task1", "task2"]


class TestExecuteSequential:
    def test_executes_in_order(self):
        call_order = []

        def task1():
            call_order.append("task1")
            return "result1"

        def task2():
            call_order.append("task2")
            return "result2"

        tasks = [("task1", task1), ("task2", task2)]
        results = _execute_sequential(tasks)

        assert call_order == ["task1", "task2"]
        assert results["task1"].success is True
        assert results["task2"].success is True

    def test_continues_after_exception(self):
        def failing_task():
            raise RuntimeError("error")

        tasks = [
            ("fail", failing_task),
            ("success", lambda: "ok"),
        ]
        results = _execute_sequential(tasks)

        assert results["fail"].success is False
        assert results["success"].success is True
        assert results["success"].result == "ok"


class TestTaskResult:
    def test_task_result_creation(self):
        result = TaskResult(name="test", success=True, result="data")
        assert result.name == "test"
        assert result.success is True
        assert result.result == "data"
        assert result.error is None

    def test_task_result_with_error(self):
        error = ValueError("test error")
        result = TaskResult(name="test", success=False, error=error)
        assert result.success is False
        assert result.error is error
        assert result.result is None
