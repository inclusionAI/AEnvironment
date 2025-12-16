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
AEnv Hub client usage examples
"""

import json

import pytest

from cli.client.aenv_hub_client import AEnvHubClient, EnvStatus


class TestEnvHub:

    def test_state(self):
        env_stat = EnvStatus("pending")
        print(env_stat)

    @pytest.fixture
    def hub_client(self):
        return AEnvHubClient(
            base_url="http://localhost:8080/",
            api_key="no_need",
        )

    def test_usage(self, hub_client):
        """Basic usage example"""
        print("=== Basic Usage Example ===")

        # Create environment
        # env = client.create_environment(
        #     name="production",
        #     description="Production environment configuration",
        #     variables={
        #         "DATABASE_URL": "postgresql://user:pass@localhost:5432/mydb",
        #         "API_KEY": "secret123",
        #         "DEBUG": "false",
        #     },
        #     tags=["production", "critical"],
        # )
        # print(f"✅ Created environment: {env['name']}")

        # Get environment details
        list_environments = hub_client.list_environments()
        print(f"🔍 Environment details: {len(list_environments)} environments")
        print(f"{json.dumps(list_environments[0])}")
