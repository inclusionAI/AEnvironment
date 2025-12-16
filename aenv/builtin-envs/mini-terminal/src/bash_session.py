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

import subprocess
import threading
import time
from queue import Empty, Queue

import psutil


class MiniBashSession:
    def __init__(self):
        self.bash_process = subprocess.Popen(
            ["bash", "-i"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
            universal_newlines=True,
        )
        self.stdout_queue = Queue()
        self.lock = threading.Lock()

        self.stdout_thread = threading.Thread(target=self._read_stdout)
        self.stdout_thread.daemon = True
        self.stdout_thread.start()

    def _read_stdout(self):
        while True:
            line = self.bash_process.stdout.readline()
            if line:
                self.stdout_queue.put(line)
            else:
                break

    def send_stdin(self, command, timeout=5):
        with self.lock:
            while not self.stdout_queue.empty():
                try:
                    self.stdout_queue.get_nowait()
                except Empty:
                    break

            self.bash_process.stdin.write(command + "\n")
            self.bash_process.stdin.flush()

            start_time = time.time()
            output_lines = []
            while True:
                while True:
                    try:
                        line = self.stdout_queue.get(timeout=0.1)
                        output_lines.append(line)
                        if time.time() - start_time > timeout:
                            result = "".join(output_lines)
                            return result, False
                    except Empty:
                        if time.time() - start_time > timeout:
                            result = "".join(output_lines)
                            return result, False
                        break

                try:
                    parent_pid = self.bash_process.pid
                    parent = psutil.Process(parent_pid)
                    children = parent.children(recursive=True)
                    if not children:
                        result = "".join(output_lines)
                        return result, True
                except psutil.NoSuchProcess:
                    result = "".join(output_lines)
                    return result, True

    def get_remaining_stdout(self):
        with self.lock:
            output_lines = []
            while not self.stdout_queue.empty():
                try:
                    output_lines.append(self.stdout_queue.get_nowait())
                except Empty:
                    break
            return "".join(output_lines)
