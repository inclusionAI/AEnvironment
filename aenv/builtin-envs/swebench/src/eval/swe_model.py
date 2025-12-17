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

class SweRun:
    instance_id: str
    repo: str
    version: str
    model_patch: str

    eval_script: str
    test_cmd: str

    eval_file: str
    local_code_space: str

    FAIL_TO_PASS: list[str]
    PASS_TO_PASS: list[str]

    def __init__(self, instance, repo, version, model_patch, eval_script, test_cmd):
        self.instance_id = instance
        self.repo = repo
        self.version = version

        self.model_patch = model_patch
        self.eval_script = eval_script
        self.test_cmd = test_cmd

        self.eval_file = "/dev/shm/swe-sandbox/eval.sh"
        self.local_code_space = "/testbed"
