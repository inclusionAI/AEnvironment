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
Test script for instances command
"""
import json
import os
from unittest.mock import Mock, patch

import pytest
from click.testing import CliRunner

from cli.cmds.instances import instances


class TestInstances:
    """Test cases for instances command"""

    @pytest.fixture
    def runner(self):
        """Create a CliRunner instance"""
        return CliRunner()

    @pytest.fixture
    def mock_instances_list_response(self):
        """Mock response for list instances API"""
        return {
            "success": True,
            "data": [
                {
                    "id": "test-env-abc123",
                    "ip": "192.168.1.100",
                    "status": "running",
                    "created_at": "2025-01-15T10:30:00Z",
                    "env": {
                        "name": "test-env",
                        "version": "1.0.0",
                    },
                },
                {
                    "id": "test-env-def456",
                    "ip": "192.168.1.101",
                    "status": "running",
                    "created_at": "2025-01-15T11:00:00Z",
                    "env": {
                        "name": "test-env",
                        "version": "1.0.0",
                    },
                },
            ],
        }

    @pytest.fixture
    def mock_instance_detail_response(self):
        """Mock response for get instance detail API"""
        return {
            "success": True,
            "data": {
                "id": "test-env-abc123",
                "ip": "192.168.1.100",
                "status": "running",
                "created_at": "2025-01-15T10:30:00Z",
                "env": {
                    "name": "test-env",
                    "version": "1.0.0",
                },
            },
        }

    def test_list_all_instances(self, runner, mock_instances_list_response):
        """Test listing all instances"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            # Mock list API response
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            # Mock detail API response (called for each instance in table format)
            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = mock_instances_list_response["data"][0]

                result = runner.invoke(instances, [])

                assert result.exit_code == 0
                assert "test-env-abc123" in result.output
                assert "test-env-def456" in result.output
                assert "test-env" in result.output
                assert "1.0.0" in result.output

    def test_list_instances_with_name_filter(
        self, runner, mock_instances_list_response
    ):
        """Test listing instances filtered by name"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            # Filter to only one instance
            filtered_data = {
                "success": True,
                "data": [mock_instances_list_response["data"][0]],
            }
            mock_response.json.return_value = filtered_data
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = mock_instances_list_response["data"][0]

                result = runner.invoke(instances, ["--name", "test-env"])

                assert result.exit_code == 0
                # Verify the API was called with correct env_id
                mock_get.assert_called_once()
                call_args = mock_get.call_args
                assert "test-env/list" in call_args[0][0]

    def test_list_instances_with_name_and_version_filter(
        self, runner, mock_instances_list_response
    ):
        """Test listing instances filtered by name and version"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            filtered_data = {
                "success": True,
                "data": [mock_instances_list_response["data"][0]],
            }
            mock_response.json.return_value = filtered_data
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = mock_instances_list_response["data"][0]

                result = runner.invoke(
                    instances, ["--name", "test-env", "--version", "1.0.0"]
                )

                assert result.exit_code == 0
                # Verify the API was called with correct env_id format
                mock_get.assert_called_once()
                call_args = mock_get.call_args
                assert "test-env@1.0.0/list" in call_args[0][0]

    def test_list_instances_json_format(self, runner, mock_instances_list_response):
        """Test listing instances with JSON output format"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            result = runner.invoke(instances, ["--format", "json"])

            assert result.exit_code == 0
            # Verify output is valid JSON
            output_data = json.loads(result.output)
            assert isinstance(output_data, list)
            assert len(output_data) == 2
            assert output_data[0]["id"] == "test-env-abc123"

    def test_list_instances_table_format(self, runner, mock_instances_list_response):
        """Test listing instances with table output format"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = mock_instances_list_response["data"][0]

                result = runner.invoke(instances, ["--format", "table"])

                assert result.exit_code == 0
                # Verify table format contains headers
                assert "Instance ID" in result.output
                assert "Environment" in result.output
                assert "Version" in result.output
                assert "Status" in result.output
                assert "IP" in result.output
                assert "Created At" in result.output

    def test_list_instances_empty_result(self, runner):
        """Test listing instances when no instances are running"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = {"success": True, "data": []}
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            result = runner.invoke(instances, [])

            assert result.exit_code == 0
            assert "No running instances found" in result.output

    def test_list_instances_empty_result_with_name(self, runner):
        """Test listing instances with name filter when no instances found"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = {"success": True, "data": []}
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            result = runner.invoke(instances, ["--name", "nonexistent-env"])

            assert result.exit_code == 0
            assert "No running instances found for nonexistent-env" in result.output

    def test_list_instances_empty_result_with_name_and_version(self, runner):
        """Test listing instances with name and version filter when no instances found"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = {"success": True, "data": []}
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            result = runner.invoke(
                instances, ["--name", "nonexistent-env", "--version", "1.0.0"]
            )

            assert result.exit_code == 0
            assert (
                "No running instances found for nonexistent-env@1.0.0" in result.output
            )

    def test_list_instances_version_without_name_error(self, runner):
        """Test that version option requires name option"""
        result = runner.invoke(instances, ["--version", "1.0.0"])

        assert result.exit_code != 0
        assert "Version filter requires --name" in result.output

    def test_list_instances_with_custom_system_url(
        self, runner, mock_instances_list_response
    ):
        """Test listing instances with custom system URL"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = mock_instances_list_response["data"][0]

                result = runner.invoke(
                    instances, ["--system-url", "http://custom.example.com:8080"]
                )

                assert result.exit_code == 0
                # Verify the API was called with custom URL
                mock_get.assert_called_once()
                call_args = mock_get.call_args
                assert "custom.example.com:8080" in call_args[0][0]

    def test_list_instances_api_error(self, runner):
        """Test handling of API errors"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            import requests

            mock_get.side_effect = requests.exceptions.RequestException(
                "Connection error"
            )

            result = runner.invoke(instances, [])

            assert result.exit_code != 0
            assert "Failed to list instances" in result.output

    def test_list_instances_api_non_success_response(self, runner):
        """Test handling of API non-success response"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = {"success": False, "data": None}
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            result = runner.invoke(instances, [])

            assert result.exit_code == 0
            assert "No running instances found" in result.output

    def test_list_instances_with_api_key(self, runner, mock_instances_list_response):
        """Test that API key is included in headers when available"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances.get_config_manager") as mock_config:
                mock_config_manager = Mock()
                mock_hub_config = {"api_key": "test-api-key"}
                mock_config_manager.get_hub_config.return_value = mock_hub_config
                mock_config.return_value = mock_config_manager

                with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                    mock_detail.return_value = mock_instances_list_response["data"][0]

                    result = runner.invoke(instances, [])

                    assert result.exit_code == 0
                    # Verify Authorization header was set
                    mock_get.assert_called_once()
                    call_kwargs = mock_get.call_args[1]
                    assert "Authorization" in call_kwargs["headers"]
                    assert (
                        call_kwargs["headers"]["Authorization"] == "Bearer test-api-key"
                    )

    def test_list_instances_with_env_api_key(
        self, runner, mock_instances_list_response
    ):
        """Test that API key from environment variable is used"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances.get_config_manager") as mock_config:
                mock_config_manager = Mock()
                mock_config_manager.get_hub_config.return_value = {}
                mock_config.return_value = mock_config_manager

                with patch.dict(os.environ, {"AENV_API_KEY": "env-api-key"}):
                    with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                        mock_detail.return_value = mock_instances_list_response["data"][
                            0
                        ]

                        result = runner.invoke(instances, [])

                        assert result.exit_code == 0
                        # Verify Authorization header was set from env var
                        mock_get.assert_called_once()
                        call_kwargs = mock_get.call_args[1]
                        assert "Authorization" in call_kwargs["headers"]
                        assert (
                            call_kwargs["headers"]["Authorization"]
                            == "Bearer env-api-key"
                        )

    def test_list_instances_with_env_system_url(
        self, runner, mock_instances_list_response
    ):
        """Test that system URL from environment variable is used"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = mock_instances_list_response["data"][0]

                with patch.dict(
                    os.environ, {"AENV_SYSTEM_URL": "http://env.example.com"}
                ):
                    result = runner.invoke(instances, [])

                    assert result.exit_code == 0
                    # Verify the API was called with env URL
                    mock_get.assert_called_once()
                    call_args = mock_get.call_args[0]
                    assert "env.example.com:8080" in call_args[0]

    def test_list_instances_detail_api_failure_fallback(
        self, runner, mock_instances_list_response
    ):
        """Test that list API data is used when detail API fails"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            mock_response.json.return_value = mock_instances_list_response
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            # Mock detail API to return None (simulating failure)
            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = None

                result = runner.invoke(instances, [])

                assert result.exit_code == 0
                # Should still show instances using list data
                assert "test-env-abc123" in result.output

    def test_list_instances_missing_env_info(self, runner):
        """Test handling of instances with missing environment info"""
        with patch("cli.cmds.instances.requests.get") as mock_get:
            mock_response = Mock()
            # Instance without env info
            mock_response.json.return_value = {
                "success": True,
                "data": [
                    {
                        "id": "test-env-xyz789",
                        "ip": "192.168.1.102",
                        "status": "running",
                        "created_at": "2025-01-15T12:00:00Z",
                        # Missing env field
                    },
                ],
            }
            mock_response.raise_for_status = Mock()
            mock_get.return_value = mock_response

            with patch("cli.cmds.instances._get_instance_info") as mock_detail:
                mock_detail.return_value = None

                result = runner.invoke(instances, [])

                assert result.exit_code == 0
                # Should extract name from instance ID
                assert "test-env-xyz789" in result.output
                # Environment name should be extracted from ID
                assert "test-env" in result.output

    def test_make_api_url_with_protocol(self):
        """Test _make_api_url with protocol in URL"""
        from cli.cmds.instances import _make_api_url

        url = _make_api_url("http://example.com", port=8080)
        assert url == "http://example.com:8080"

    def test_make_api_url_without_protocol(self):
        """Test _make_api_url without protocol in URL"""
        from cli.cmds.instances import _make_api_url

        url = _make_api_url("example.com", port=8080)
        assert url == "http://example.com:8080"

    def test_make_api_url_empty(self):
        """Test _make_api_url with empty URL"""
        from cli.cmds.instances import _make_api_url

        url = _make_api_url("", port=8080)
        assert url == "http://localhost:8080"

    def test_get_system_url_from_env(self):
        """Test _get_system_url with environment variable"""
        from cli.cmds.instances import _get_system_url

        with patch.dict(os.environ, {"AENV_SYSTEM_URL": "http://test.example.com"}):
            url = _get_system_url()
            assert url == "http://test.example.com:8080"

    def test_get_system_url_default(self):
        """Test _get_system_url without environment variable"""
        from cli.cmds.instances import _get_system_url

        with patch.dict(os.environ, {}, clear=True):
            # Remove AENV_SYSTEM_URL if it exists
            if "AENV_SYSTEM_URL" in os.environ:
                del os.environ["AENV_SYSTEM_URL"]
            url = _get_system_url()
            assert url == "http://localhost:8080"
