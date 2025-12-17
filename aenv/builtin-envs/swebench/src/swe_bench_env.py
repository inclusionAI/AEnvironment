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

import json
import os
import shutil

from aenv import register_reward
from docker_cli import DockerClient

docker = DockerClient.load_container(
    namespace_list=["k8s.io", "default"],
    container_name="second",
)

SOURCE_BIN_PATH = "/app/src/eval"
share_path = os.getenv("WORK_SHARE", "/shared")
if share_path is not None:
    shutil.move(SOURCE_BIN_PATH, share_path)

with open("/app/data/swe-verify-details.json", 'r') as f:
    all_data = json.load(f)


@register_reward
def swebench_reward(instance: str, model_patch: str, timeout: int = 30):
    """
    https://www.modelscope.cn/datasets/kangoal/swebench-scripts/resolve/master/swe-verify-details.json
    details is downloaded datasource from modelscope
    """
    val = all_data[instance]
    if not val:
        return {"returncode": 1, "stderr": "instance_id is not founded!"}
    details = val.get("details")
    if not details:
        return {"returncode": 1, "stderr": "details is empty!"}

    json_details = json.loads(details)
    json_details["model_patch"] = model_patch

    with open("/shared/details.json", "w+") as w:
        w.write(json.dumps(json_details))

    # aenv_evaluation script is copy from swebench project
    command = "/opt/miniconda3/envs/python3.12/bin/python aenv_evaluation.py"
    response = docker.execute(command=command, cwd="/shared/eval", timeout=timeout)

    return response
