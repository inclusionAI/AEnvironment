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
from aenv import Environment


@pytest.mark.asyncio
async def async_run_swebench():
    swebench = Environment("swebench@1.0.0", datasource="swebench/astropy__astropy-12907")
    try:
        await swebench.initialize()
        instance_id = "astropy__astropy-12907"
        model_patch = "diff --git a/astropy/modeling/separable.py b/astropy/modeling/separable.py\n--- a/astropy/modeling/separable.py\n+++ b/astropy/modeling/separable.py\n@@ -242,7 +242,7 @@ def _cstack(left, right):\n         cright = _coord_matrix(right, 'right', noutp)\n     else:\n         cright = np.zeros((noutp, right.shape[1]))\n-        cright[-right.shape[0]:, -right.shape[1]:] = 1\n+        cright[-right.shape[0]:, -right.shape[1]:] = right\n \n     return np.hstack([cleft, cright])\n \n",

        args = {
            "instance": instance_id,
            "model_patch": model_patch,
            "timeout": 300,
        }
        reward = await swebench.call_reward(arguments=args)
        print("swebench reward:", reward)
    except Exception as e:
        print("run with error:", str(e))
    finally:
        await swebench.release()
