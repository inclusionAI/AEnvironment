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

"""Integration tests for optimized push command."""

import json
import os
import tempfile
import time
from unittest.mock import MagicMock, patch

import pytest
from click.testing import CliRunner

from cli.cmds.push import push
from cli.extends.storage.storage_manager import (
    AEnvHubStorage,
    StorageContext,
)
from cli.utils.parallel import parallel_execute


@pytest.fixture
def mock_project_dir():
    """Create a mock aenv project directory."""
    with tempfile.TemporaryDirectory() as tmpdir:
        config = {
            "name": "test-env",
            "version": "1.0.0",
            "tags": ["test"],
        }
        config_path = os.path.join(tmpdir, "config.json")
        with open(config_path, "w") as f:
            json.dump(config, f)

        (Path(tmpdir) / "src").mkdir()
        (Path(tmpdir) / "src" / "main.py").write_text("print('hello')")

        yield tmpdir


from pathlib import Path


class TestParallelHttpRequests:
    def test_check_env_and_state_run_concurrently(self):
        call_times = {}

        def mock_check_env():
            call_times["check_env_start"] = time.time()
            time.sleep(0.1)
            call_times["check_env_end"] = time.time()
            return True

        def mock_state_env():
            call_times["state_env_start"] = time.time()
            time.sleep(0.1)
            call_times["state_env_end"] = time.time()
            return "pending"

        tasks = [
            ("check_env", mock_check_env),
            ("state_env", mock_state_env),
        ]

        start = time.time()
        results = parallel_execute(tasks)
        elapsed = time.time() - start

        assert results["check_env"].success is True
        assert results["state_env"].success is True
        assert elapsed < 0.2

    def test_handles_check_env_failure(self):
        def mock_check_env():
            raise ConnectionError("Network error")

        def mock_state_env():
            return "pending"

        tasks = [
            ("check_env", mock_check_env),
            ("state_env", mock_state_env),
        ]

        results = parallel_execute(tasks)

        assert results["check_env"].success is False
        assert isinstance(results["check_env"].error, ConnectionError)
        assert results["state_env"].success is True


class TestAEnvHubStorageOptimized:
    def test_concurrent_upload(self, mock_project_dir):
        storage = AEnvHubStorage()

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()

        with (
            patch("cli.extends.storage.storage_manager.get_config_manager") as mock_config,
            patch("cli.extends.storage.storage_manager.AEnvHubClient") as mock_client_cls,
            patch("requests.put", return_value=mock_response),
            patch.dict(os.environ, {}, clear=True),
        ):
            os.environ.pop("AENV_DISABLE_PARALLEL", None)

            mock_config.return_value.get_storage_config.return_value = {
                "custom": {"prefix": "/test"}
            }

            mock_client = MagicMock()
            mock_client.apply_sign_url.return_value = "https://oss.example.com/signed"
            mock_client_cls.load_client.return_value = mock_client

            ctx = StorageContext(
                src_url=mock_project_dir,
                infos={"name": "test-env", "version": "1.0.0"},
            )

            result = storage.upload(ctx)

            assert result.state is True
            assert "test-env" in result.dest_url
            mock_client.apply_sign_url.assert_called_once_with("test-env", "1.0.0")

    def test_sequential_upload_when_disabled(self, mock_project_dir):
        storage = AEnvHubStorage()

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()

        with (
            patch("cli.extends.storage.storage_manager.get_config_manager") as mock_config,
            patch("cli.extends.storage.storage_manager.AEnvHubClient") as mock_client_cls,
            patch("requests.put", return_value=mock_response),
            patch.dict(os.environ, {"AENV_DISABLE_PARALLEL": "1"}),
        ):
            mock_config.return_value.get_storage_config.return_value = {
                "custom": {"prefix": "/test"}
            }

            mock_client = MagicMock()
            mock_client.apply_sign_url.return_value = "https://oss.example.com/signed"
            mock_client_cls.load_client.return_value = mock_client

            ctx = StorageContext(
                src_url=mock_project_dir,
                infos={"name": "test-env", "version": "1.0.0"},
            )

            result = storage.upload(ctx)

            assert result.state is True


class TestPushCommandIntegration:
    def test_push_with_parallel_checks(self, mock_project_dir):
        runner = CliRunner()

        with (
            patch("cli.cmds.push.AEnvHubClient") as mock_client_cls,
            patch("cli.cmds.push.load_storage") as mock_load_storage,
        ):
            mock_client = MagicMock()
            mock_client.check_env.return_value = False
            mock_client.state_environment.return_value = "completed"
            mock_client.create_environment.return_value = {}
            mock_client_cls.load_client.return_value = mock_client

            mock_storage = MagicMock()
            mock_storage.upload.return_value = MagicMock(
                state=True, dest_url="/test/path"
            )
            mock_load_storage.return_value = mock_storage

            result = runner.invoke(push, ["--work-dir", mock_project_dir])

            assert result.exit_code == 0
            assert "Push successfully" in result.output

    def test_push_existing_env_not_running(self, mock_project_dir):
        runner = CliRunner()

        with (
            patch("cli.cmds.push.AEnvHubClient") as mock_client_cls,
            patch("cli.cmds.push.load_storage") as mock_load_storage,
        ):
            mock_client = MagicMock()
            mock_client.check_env.return_value = True
            mock_client.state_environment.return_value = "completed"
            mock_client.update_environment.return_value = {}
            mock_client_cls.load_client.return_value = mock_client

            mock_storage = MagicMock()
            mock_storage.upload.return_value = MagicMock(
                state=True, dest_url="/test/path"
            )
            mock_load_storage.return_value = mock_storage

            result = runner.invoke(push, ["--work-dir", mock_project_dir])

            assert result.exit_code == 0
            mock_client.update_environment.assert_called_once()

    def test_push_existing_env_running_without_force(self, mock_project_dir):
        runner = CliRunner()

        with patch("cli.cmds.push.AEnvHubClient") as mock_client_cls:
            mock_client = MagicMock()
            mock_client.check_env.return_value = True
            mock_client.state_environment.return_value = "pending"
            mock_client_cls.load_client.return_value = mock_client

            result = runner.invoke(push, ["--work-dir", mock_project_dir])

            assert result.exit_code == 1
            assert "being prepared" in result.output

    def test_push_existing_env_running_with_force(self, mock_project_dir):
        runner = CliRunner()

        with (
            patch("cli.cmds.push.AEnvHubClient") as mock_client_cls,
            patch("cli.cmds.push.load_storage") as mock_load_storage,
        ):
            mock_client = MagicMock()
            mock_client.check_env.return_value = True
            mock_client.state_environment.return_value = "pending"
            mock_client.update_environment.return_value = {}
            mock_client_cls.load_client.return_value = mock_client

            mock_storage = MagicMock()
            mock_storage.upload.return_value = MagicMock(
                state=True, dest_url="/test/path"
            )
            mock_load_storage.return_value = mock_storage

            result = runner.invoke(push, ["--work-dir", mock_project_dir, "--force"])

            assert result.exit_code == 0
            mock_client.update_environment.assert_called_once()
