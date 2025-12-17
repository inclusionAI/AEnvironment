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

import logging
import os
import platform
import shlex
import subprocess
import threading
import time
from typing import Optional, Dict, List, Union, Callable


class ExecResult(object):
    def __init__(self, code, stdout, stderr, duration):
        self.code = code
        self.stdout = stdout
        self.stderr = stderr
        self.duration = duration

    def ok(self):
        return self.code == 0

    def output(self):
        return (f'++++++++START_STDOUT++++++++\n{self.stdout}\n++++++++END_STDOUT++++++++\n'
                f'++++++++START_STDERR++++++++\n{self.stderr}\n++++++++END_STDERR++++++++')

    def __str__(self):
        return f"ExecResult:{{code: {self.code}, duration: {self.duration}s}}"


class ShellExecutor:
    """safe execute shell command"""

    def __init__(self, timeout: int = 60, working_dir: str = None,
                 env: Dict[str, str] = None, encoding: str = "utf-8",
                 logger: logging.Logger = logging.getLogger("ShellExecutor")):
        """
        initialize shell executor
        :param timeout: timeout (seconds)
        :param working_dir: shell work dir
        :param env: custom env
        :param encoding: encoding
        """
        self.timeout = timeout
        self.working_dir = working_dir or os.getcwd()
        self.env = self._prepare_env(env)
        self.encoding = encoding
        self.logger = logger

    def _prepare_env(self, custom_env: Dict[str, str] = None) -> Dict[str, str]:
        """Prepare safe environment variables"""
        base_env = os.environ.copy()

        # Block certain dangerous environment variables from being passed
        # blocked_vars = ["LD_PRELOAD", "LD_LIBRARY_PATH", "PYTHONPATH"]
        # for var in blocked_vars:
        #     base_env.pop(var, None)

        # Add custom environment variables
        if custom_env:
            base_env.update(custom_env)

        # Set secure PATH
        if platform.system() == "Linux":
            base_env["PATH"] = "/bin:/usr/bin:/usr/local/bin"
        elif platform.system() == "Windows":
            base_env["PATH"] = os.environ["SystemRoot"] + "\\System32"
        else:
            base_env["PATH"] = os.environ["PATH"]

        return base_env

    def _sanitize_command(self, command: Union[str, List[str]]) -> List[str]:
        """Command sanitization processing"""
        if isinstance(command, str):
            # Safely split command
            if platform.system() == "Windows":
                return shlex.split(command, posix=False)
            return shlex.split(command)
        return command

    def _requires_shell(self, command: Union[str, List[str]]) -> bool:
        """
        Returns:
            bool:
        """
        if not isinstance(command, str):
            return False
        """Check if command requires shell"""
        shell_chars = ['&&', '||', ';', '|', '$', '>', '<', '`', 'cd', ]
        return any(char in command for char in shell_chars) if isinstance(command, str) else False

    def execute(
            self,
            command: Union[str, List[str]],
            timeout: Optional[int] = None,
            realtime_callback: Optional[Callable[[str], None]] = None,
            input_data: Optional[str] = None,
            out_record: Optional[str] = None
    ) -> ExecResult:
        """
        Execute command and return result
        :param command: command to execute (string or list)
        :param timeout: custom timeout (overrides default)
        :param realtime_callback: real-time output callback function
        :param input_data: data to input to the command
        :return: dictionary containing returncode, stdout, stderr
        """

        # Parameter processing
        timeout = timeout or self.timeout

        shell_required = self._requires_shell(command)
        if shell_required:
            sanitized_cmd = command
        else:
            sanitized_cmd = self._sanitize_command(command)

        self.logger.info(f"++++++++Begin executing command++++++++")
        self.logger.info(f"Command:{command} Workdir: {self.working_dir}, Timeout: {timeout}s")

        # Execute command
        start_time = time.time()
        try:
            if realtime_callback:
                return self._execute_with_realtime_output(
                    sanitized_cmd, timeout, realtime_callback, input_data
                )
            else:
                return self._execute_with_capture(
                    sanitized_cmd, timeout, shell_required, out_record, input_data
                )
        except Exception as e:
            end_time = time.time()
            duration = end_time - start_time
            self.logger.error(f"Execution failed after {duration:.2f}s: {str(e)}")
            return ExecResult(-1, "", f"Execution error: {str(e)}", duration)
        finally:
            self.logger.info(f"++++++++End executing command++++++++")

    def _execute_with_capture(
            self,
            command: List[str],
            timeout: int,
            shell_need: bool,
            out_record: str,
            input_data: str = None
    ) -> ExecResult:
        """Execution method with complete output capture"""
        # Execute command
        start_time = time.time()

        def process_run(stdout):
            return subprocess.run(
                command,
                stdout=subprocess.PIPE if not stdout else stdout,
                stderr=subprocess.STDOUT,
                stdin=subprocess.PIPE if input_data else None,
                timeout=timeout,
                text=True,
                shell=shell_need,
                cwd=self.working_dir,
                env=self.env,
                encoding=self.encoding,
                errors="replace",
                check=False
            )

        try:
            input_bytes = input_data.encode(self.encoding) if input_data else None
            if out_record:
                with open(out_record, "wb") as out:
                    global result
                    result = process_run(out)
            else:
                pipe = subprocess.PIPE
                result = process_run(pipe)
            return ExecResult(result.returncode, result.stdout, result.stderr, time.time() - start_time)
        except subprocess.TimeoutExpired:
            return ExecResult(-2, "", f"Timeout after {timeout} seconds", timeout)

    def _execute_with_realtime_output(
            self,
            command: List[str],
            timeout: int,
            callback: Callable[[str], None],
            input_data: str = None
    ) -> ExecResult:
        """Execution method with real-time output"""
        start_time = time.time()
        stdout_lines = []
        stderr_lines = []

        try:
            with subprocess.Popen(
                    command,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    stdin=subprocess.PIPE if input_data else None,
                    cwd=self.working_dir,
                    env=self.env,
                    bufsize=1,  # Line buffering
                    text=True,
                    encoding=self.encoding,
                    errors="replace"
            ) as proc:
                # Handle input
                if input_data:
                    proc.stdin.write(input_data)
                    proc.stdin.close()

                # Start reading threads
                stdout_thread = threading.Thread(
                    target=self._read_stream,
                    args=(proc.stdout, stdout_lines, callback, "stdout")
                )
                stderr_thread = threading.Thread(
                    target=self._read_stream,
                    args=(proc.stderr, stderr_lines, callback, "stderr")
                )

                stdout_thread.start()
                stderr_thread.start()

                # Wait for process to end or timeout
                try:
                    proc.wait(timeout=timeout)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    stdout_thread.join(timeout=1)
                    stderr_thread.join(timeout=1)
                    raise

                # Wait for threads to finish
                stdout_thread.join(timeout=1)
                stderr_thread.join(timeout=1)

                return ExecResult(proc.returncode, "".join(stdout_lines), "".join(stderr_lines),
                                  time.time() - start_time)

        except subprocess.TimeoutExpired:
            return ExecResult(-2, "".join(stdout_lines), "".join(stderr_lines) + f"\nTimeout after {timeout} seconds",
                              time.time() - start_time)

    def _read_stream(
            self,
            stream,
            output_list: List[str],
            callback: Callable[[str], None],
            stream_name: str
    ):
        """Thread function for reading output streams"""
        for line in iter(stream.readline, ''):
            output_list.append(line)
            if callback:
                callback(f"[{stream_name.upper()}] {line.rstrip()}")
        stream.close()

    def execute_script(
            self,
            script_path: str,
            args: List[str] = None,
            interpreter: str = None,
            **kwargs
    ) -> ExecResult:
        """
        Execute script file
        :param script_path: script path
        :param args: arguments to pass to the script
        :param interpreter: specify interpreter (e.g., /bin/bash, python3)
        :param kwargs: parameters to pass to execute method
        :return: execution result
        """
        # Check if script exists
        if not os.path.exists(script_path):
            raise FileNotFoundError(f"Script not found: {script_path}")

        # Get file permissions
        st = os.stat(script_path)
        if not st.st_mode & 0o100:  # Check executable permission
            self.logger.warning(f"Script {script_path} is not executable. Adding permission.")
            os.chmod(script_path, st.st_mode | 0o100)  # Add execute permission

        # Auto-detect interpreter
        if interpreter is None:
            with open(script_path, 'r', encoding=self.encoding) as f:
                first_line = f.readline().strip()

            if first_line.startswith("#!"):
                interpreter = first_line[2:].strip()
                self.logger.info(f"Detected interpreter: {interpreter}")

        # Build command
        command = []
        if interpreter:
            command.append(interpreter)
        command.append(script_path)

        if args:
            command.extend(args)

        return self.execute(command, **kwargs)