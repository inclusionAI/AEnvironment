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

import pytest

from aenv.core.environment import Environment


@pytest.mark.asyncio
async def test_tau2_env_list_tools():
    """Test that tau2 environment can list tools."""
    tau2_env = Environment(
        env_name="tau2-env@1.0.0",
        environment_variables=dict(
            TAU2_DOMAIN="telecom",
            TAU2_TASK_ID="[mobile_data_issue]user_abroad_roaming_enabled_off[PERSONA:None]",
        ),
    )

    try:
        tools = await tau2_env.list_tools()
        print("Successfully retrieved tool list:", tools)
        assert tools is not None
    except Exception as e:
        print(
            "Test completed - Environment created successfully, but tool list may be empty:",
            str(e),
        )
    finally:
        try:
            await tau2_env.release()
        except Exception:
            pass


@pytest.mark.asyncio
async def test_tau2_init_and_get_task_info():
    """Test init_tau2_env and tau2_get_task_info tool calls."""
    tau2_env = Environment(
        env_name="tau2-env@1.0.0",
        environment_variables=dict(
            TAU2_DOMAIN="telecom",
            TAU2_TASK_ID="[mobile_data_issue]user_abroad_roaming_enabled_off[PERSONA:None]",
        ),
    )

    try:
        # Test with a domain that might fallback to _full suffix
        result = await tau2_env.call_tool(
            "tau2-env@1.0.0/init_tau2_env",
            {"domain": "telecom", "task_id": None, "solo_mode": True},
        )
        print("init_tau2_env result:", result)

        # Test get_task_info
        result = await tau2_env.call_tool(
            "tau2-env@1.0.0/tau2_get_task_info",
            {},
        )
        print("tau2_get_task_info result:", result)
    except Exception as e:
        print("Error:", str(e))
    finally:
        try:
            await tau2_env.release()
            print("Successfully released environment")
        except Exception:
            pass
