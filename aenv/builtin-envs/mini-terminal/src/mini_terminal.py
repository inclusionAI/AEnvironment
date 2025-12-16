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

import traceback

from bash_session import MiniBashSession

from aenv import register_tool

GLOBAL_SESSION = MiniBashSession()


@register_tool
def execute_command(command: str, timeout=60):
    try:
        output = GLOBAL_SESSION.send_stdin(command, timeout=timeout)
        return {"output": output, "returncode": 0}
    except BaseException as e:
        print(f"session execute error:{e}")
        traceback.print_exc()
        return {"output": str(e), "returncode": 1}


if __name__ == "__main__":
    result = execute_command("echo 'Hello World'")
    print(result)
