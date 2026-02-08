#!/usr/bin/env python3
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

# -*- coding: utf-8 -*-
"""
tmux_session.py
Pure nerdctl version TmuxSession, supports automatic container ID resolution via "ns+container name fragment"
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import time
from collections import namedtuple
from pathlib import Path
from typing import List, Optional, Sequence, Union

ExecResult = namedtuple("ExecResult", "exit_code,output")
""" A result of Container.exec_run with the properties ``exit_code`` and
    ``output``. """


class TmuxSession:
    _ENTER_KEYS = {"Enter", "C-m", "KPEnter", "C-j", "^M", "^J"}
    _ENDS_WITH_NEWLINE_PATTERN = re.compile(r"[\r\n]$")
    _NEWLINE_CHARS = "\r\n"
    _TMUX_COMPLETION_COMMAND = "; tmux wait -S done"
    _GET_TS_SCRIPT_NAME = "get-asciinema-timestamp.sh"

    # ------------------------------------------------------------------
    # Constructor: supports two ways - "container ID" or "ns+name fragment"
    # ------------------------------------------------------------------
    def __init__(
        self,
        session_name: str,
        container_id: str,
        namespace: str,
        commands_path: Optional[Path] = None,
        disable_recording: bool = False,
    ):
        self._container_id = container_id
        self._session_name = session_name
        self._commands_path = commands_path
        self._disable_recording = disable_recording
        self._logger = print  # Can be replaced with logging.getLogger(...)
        self._asciinema_markers: List[float] = []
        self._previous_buffer: Optional[str] = None
        self.namespace = namespace

        # Verify tools in container
        self._require_in_container("tmux")
        if not disable_recording:
            self._require_in_container("asciinema")

        # Copy timestamp script
        host_script = Path(__file__).with_name(self._GET_TS_SCRIPT_NAME)
        if not host_script.exists():
            host_script.write_text(self._GET_TS_SCRIPT_CONTENT, encoding="utf8")
        self._copy_into_container(host_script)

    @classmethod
    def from_nerdctl_name(
        cls,
        session_name: str,
        namespace_list: list[str],
        container_name: str,
        commands_path: Optional[Path] = None,
        disable_recording: bool = False,
    ) -> "TmuxSession":
        """Automatically find container ID through namespace list + container name fragment"""
        pod_name = os.getenv("POD_NAME", "")
        pod_namespace = os.getenv("POD_NAMESPACE", "")
        if not pod_name or not pod_namespace:
            raise RuntimeError("env POD_NAME or env POD_NAMESPACE not set")

        container_name = (
            f"k8s://{pod_namespace}/{pod_name}/{container_name}"  # Complete literal
        )

        # Backward compatibility for single namespace string
        if isinstance(namespace_list, str):
            namespace_list = [namespace_list]

        # Default namespace list
        if not namespace_list:
            raise RuntimeError("nerdctl namespace not set")

        cid, found_namespace = cls._find_container_id(namespace_list, container_name)
        return cls(
            session_name=session_name,
            container_id=cid,
            namespace=found_namespace,
            commands_path=commands_path,
            disable_recording=disable_recording,
        )

    @staticmethod
    def _find_container_id(
        namespace_list: list[str], container_name: str
    ) -> tuple[str, str]:
        """Find container ID with retry mechanism (10 attempts, 1s delay each)."""
        max_retries = 10
        retry_delay = 1.0

        for attempt in range(max_retries):
            for ns in namespace_list:
                cmd = ["nerdctl", "-n", ns, "ps", "--format", "json"]
                proc = subprocess.run(cmd, capture_output=True, text=True, check=False)

                if proc.returncode != 0:
                    continue

                for line in proc.stdout.splitlines():
                    try:
                        info = json.loads(line)
                        if info.get("Names") == container_name:
                            return info["ID"], ns
                    except Exception:
                        continue

            # If container not found in all namespaces, wait and retry
            if attempt < max_retries - 1:  # Don't wait on last attempt
                time.sleep(retry_delay)
                print(
                    f"🔍 Container '{container_name}' not found, retrying... ({attempt + 1}/{max_retries})"
                )

        raise RuntimeError(
            f"no container matched '{container_name}' in namespaces {namespace_list} after {max_retries} attempts"
        )

    def _nerdctl(
        self, cmd: List[str], *, capture: bool = True
    ) -> subprocess.CompletedProcess:
        full = ["nerdctl", "-n", self.namespace, *cmd]
        self._logger(f"exec: {' '.join(full)}")
        return subprocess.run(full, text=capture, capture_output=capture, check=False)

    def _exec_in_container(self, container_cmd: List[str], capture=True):
        proc = self._nerdctl(
            ["exec", self._container_id, *container_cmd], capture=capture
        )
        return ExecResult(proc.returncode, proc.stdout)

    def _require_in_container(self, prog: str) -> None:
        self._exec_in_container(["which", prog])

    def _copy_into_container(
        self, host_paths: Union[Path, str, Sequence[Union[Path, str]]]
    ) -> None:
        import shutil
        from pathlib import Path

        if not isinstance(host_paths, (list, tuple)):
            host_paths = [host_paths]

        # Ensure /shared directory exists
        shared_dir = Path("/shared")
        shared_dir.mkdir(parents=True, exist_ok=True)

        # Copy all files to /shared
        for path_str in host_paths:
            host_path = Path(path_str)
            if host_path.exists():
                if host_path.is_dir():
                    target = shared_dir / host_path.name
                    if target.exists():
                        shutil.rmtree(target)
                    shutil.copytree(host_path, target)
                else:
                    shutil.copy2(host_path, shared_dir)
                print(f"Copied {host_path.name} to /shared/")
            else:
                print(f"Source not found: {host_path}")

    @property
    def _recording_path(self) -> Optional[Path]:
        return (
            None
            if self._disable_recording
            else Path("/tmp") / f"{self._session_name}.cast"
        )

    @property
    def _logging_path(self) -> Path:
        return Path("/tmp") / f"{self._session_name}.log"

    def start(self) -> None:
        # Idempotent: if session with same name exists, kill it first
        self._exec_in_container(
            [
                "bash",
                "-c",
                f"tmux kill-session -t {self._session_name} 2>/dev/null || true",
            ]
        )
        self._exec_in_container(
            [
                "bash",
                "-c",
                f"tmux new-session -d -s {self._session_name} \\; "
                f"set-option -t {self._session_name} history-limit 50000 \\; ",
            ]
        )
        if self._recording_path:
            self._logger("Starting asciinema recording")
            self.send_keys([f"asciinema rec --stdin {self._recording_path}", "Enter"])
            self.send_keys(["clear", "Enter"])

    def stop(self) -> None:
        if self._recording_path:
            self._logger("Stopping recording")
            self.send_keys(["C-d"])

    def send_keys(
        self,
        keys: Union[str, List[str]],
        *,
        block: bool = False,
        min_timeout_sec: float = 0.0,
        max_timeout_sec: float = 180.0,
    ) -> None:
        if isinstance(keys, str):
            keys = [keys]
        if self._commands_path:
            self._commands_path.open("a").write(f"{repr(keys)}\n")

        prepared, is_blocking = self._prepare_keys(keys, block)
        self._logger(f"sending keys: {prepared}  block={is_blocking}")

        # Blocking mode → send in two parts to avoid blank screen/freeze
        if is_blocking:
            # 1. Send actual command first (without Enter)
            if prepared[:-2]:  # Remove tail ["; tmux wait -S done", "Enter"]
                self._exec_in_container(
                    ["tmux", "send-keys", "-t", self._session_name, *prepared[:-2]]
                )
            # 2. Then send Enter + wait signal
            self._exec_in_container(
                [
                    "tmux",
                    "send-keys",
                    "-t",
                    self._session_name,
                    self._TMUX_COMPLETION_COMMAND,
                    "Enter",
                ]
            )
            self._wait_tmux_done(max_timeout_sec)
        else:
            # Non-blocking remains unchanged
            self._exec_in_container(
                ["tmux", "send-keys", "-t", self._session_name, *prepared]
            )
            if min_timeout_sec:
                time.sleep(min_timeout_sec)

    def _prepare_keys(self, keys: List[str], block: bool) -> tuple[List[str], bool]:
        if not block or not keys or keys[-1] not in self._ENTER_KEYS:
            return keys, False
        # Remove all trailing Enter, then append a wait command
        tail_idx = len(keys)
        while tail_idx > 0 and keys[tail_idx - 1] in self._ENTER_KEYS:
            tail_idx -= 1
        new_keys = keys[:tail_idx] + [self._TMUX_COMPLETION_COMMAND, "Enter"]
        return new_keys, True

    def _wait_tmux_done(self, timeout: float) -> None:
        start = time.time()
        while time.time() - start < timeout:
            out = self._exec_in_container(["tmux", "wait", "done"]).output
            if out.strip() == "":
                return
            time.sleep(0.1)
        raise TimeoutError(f"tmux wait done timeout ({timeout}s)")

    # ------------------------------------------------------------------
    # State capture
    # ------------------------------------------------------------------
    def capture_pane(self, capture_entire: bool = False) -> str:
        # self._exec_in_container(["tmux", "refresh-client", "-S"], capture=False)
        cmd = ["tmux", "capture-pane", "-p", "-t", self._session_name]
        if capture_entire:
            cmd.extend(["-S", "-"])
        return self._exec_in_container(cmd).output

    def get_incremental_output(self) -> str:
        current = self.capture_pane(capture_entire=True)
        if self._previous_buffer is None:
            self._previous_buffer = current
            return f"Current Terminal Screen:\n{self._get_visible_screen()}"
        new_content = self._find_new_content(current)
        self._previous_buffer = current
        if new_content and new_content.strip():
            return f"New Terminal Output:\n{new_content}"
        return f"Current Terminal Screen:\n{self._get_visible_screen()}"

    def _find_new_content(self, current: str) -> Optional[str]:
        prev = self._previous_buffer
        if prev is None:
            return None
        prev_stripped = prev.strip()
        if prev_stripped in current:
            idx = current.rfind(prev_stripped) + len(prev_stripped)
            return current[idx:]
        return None

    def _get_visible_screen(self) -> str:
        return self.capture_pane(capture_entire=False)

    def is_session_alive(self) -> bool:
        return (
            self._exec_in_container(
                ["tmux", "has-session", "-t", self._session_name]
            ).exit_code
            == 0
        )

    def clear_history(self) -> None:
        self._exec_in_container(["tmux", "clear-history", "-t", self._session_name])

    def get_asciinema_timestamp(self) -> float:
        if self._recording_path is None:
            return 0.0
        out = self._exec_in_container(
            ["bash", f"/shared/{self._GET_TS_SCRIPT_NAME}", str(self._recording_path)]
        ).output
        return float(out.strip())

    # ------------------------------------------------------------------
    # Script content (embedded, no additional file needed)
    # ------------------------------------------------------------------
    _GET_TS_SCRIPT_CONTENT = """#!/usr/bin/env bash
# Get latest asciinema timestamp
tail -n 1 "$1" | jq -r .time || echo 0
"""


if __name__ == "__main__":
    # 1. Automatically find container ID through container name fragment
    session = TmuxSession.from_nerdctl_name(
        session_name="demo",
        namespace_list=["k8s.io", "default"],
        container_name="second",
        commands_path=Path("/tmp/cmd.log"),
        disable_recording=False,
    )

    # 2. Start session + recording
    session.start()
    session.send_keys(["echo 'Hello from nerdctl+k8s+tmux'", "Enter"], block=True)
    time.sleep(0.5)  # 👈 Wait for command output to refresh
    print("=== captured pane (entire) ===")
    print(repr(session.capture_pane(capture_entire=True)))

    session._copy_into_container(
        [
            "/data/tb/magsac-install/tests",
            "/data/tb/magsac-install/run-tests.sh",
        ]
    )
    # 3. Send command
    # session.send_keys(["echo 'Hello from nerdctl+k8s+tmcux'", "Enter"], block=True)
    # print(session.capture_pane())
    # session.send_keys(["ps -ef", "Enter"], block=True)
    # print("=== pane content ===")
    # 4. Capture output
    # print(session.capture_pane())
    # print(repr(session.capture_pane()))
    # 5. Cleanup
    session.stop()
