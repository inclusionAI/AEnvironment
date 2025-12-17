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
import logging
import os
import shlex
import subprocess
import time
import uuid
from dataclasses import asdict, dataclass, field
from typing import Any


@dataclass
class DockerEnvironmentConfig:
    image: str = ""
    cwd: str = "/"
    """Working directory in which to execute commands."""
    env: dict[str, str] = field(default_factory=dict)
    """Environment variables to set in the container."""
    forward_env: list[str] = field(default_factory=list)
    """Environment variables to forward to the container.
    Variables are only forwarded if they are set in the host environment.
    In case of conflict with `env`, the `env` variables take precedence.
    """
    timeout: int = 30
    """Timeout for executing commands in the container."""
    executable: str = os.getenv("MSWEA_DOCKER_EXECUTABLE", "nerdctl")
    """Path to the docker/container executable."""
    run_args: list[str] = field(default_factory=lambda: ["--rm"])
    """Additional arguments to pass to the docker/container executable.
    Default is ["--rm"], which removes the container after it exits.
    """
    container_timeout: str = "2h"
    """Max duration to keep container running. Uses the same format as the sleep command."""
    namespace: str = ""
    container_id: str = ""


class DockerClient:
    def __init__(self, config_class: type = DockerEnvironmentConfig, logger: logging.Logger | None = None, **kwargs):
        """This class executes bash commands in a Docker container using direct docker commands.
        See `DockerEnvironmentConfig` for keyword arguments.
        """
        self.logger = logger or logging.getLogger("mini_swe.environment")
        # self.container_id: str | None = None
        self.config = config_class(**kwargs)

    def get_template_vars(self) -> dict[str, Any]:
        return asdict(self.config)

    def _start_container(self):
        """Start the Docker container and return the container ID."""
        container_name = f"minisweagent-{uuid.uuid4().hex[:8]}"
        cmd = [
            self.config.executable,
            "run",
            "-d",
            "--name",
            container_name,
            "-w",
            self.config.cwd,
            *self.config.run_args,
            self.config.image,
            "sleep",
            self.config.container_timeout,
        ]
        self.logger.debug(f"Starting container with command: {shlex.join(cmd)}")
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=self.config.pull_timeout,  # docker pull might take a while
            check=True,
        )
        self.logger.info(f"Started container {container_name} with ID {result.stdout.strip()}")
        self.config.container_id = result.stdout.strip()

    def execute(self, command: str, cwd: str = "", *, timeout: int | None = None) -> dict[str, Any]:
        """Execute a command in the Docker container and return the result as a dict."""
        cwd = cwd or self.config.cwd
        assert self.config.container_id, "Container not started"

        cmd = [self.config.executable, "exec", "-w", cwd]

        for key in self.config.forward_env:
            if (value := os.getenv(key)) is not None:
                cmd.extend(["-e", f"{key}={value}"])
        for key, value in self.config.env.items():
            cmd.extend(["-e", f"{key}={value}"])
        cmd.extend([self.config.container_id, "bash", "-lc", command])
        self.logger.info(f"Executing command: {shlex.join(cmd)}")
        try:
            result = subprocess.run(
                cmd,
                text=True,
                timeout=timeout or self.config.timeout,
                encoding="utf-8",
                errors="replace",
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
            )
        except Exception as e:
            return {"stdout": "", "returncode": 1, "stderr": f"run err:{e}"}

        return {"stdout": result.stdout, "returncode": result.returncode, "stderr": result.stderr}

    def cleanup(self):
        """Stop and remove the Docker container."""
        if getattr(self, "container_id", None) is not None:  # if init fails early, container_id might not be set
            cmd = f"(timeout 60 {self.config.executable} stop {self.config.container_id} || {self.config.executable} rm -f {self.config.container_id}) >/dev/null 2>&1 &"
            subprocess.Popen(cmd, shell=True)

    def __del__(self):
        """Cleanup container when object is destroyed."""
        self.cleanup()

    @classmethod
    def load_container(
            cls,
            namespace_list: list[str],
            container_name: str,
    ):
        """通过 namespace 列表 + 容器名片段自动找到容器 ID"""
        pod_name = os.getenv("POD_NAME", "")
        pod_namespace = os.getenv("POD_NAMESPACE", "")
        if not pod_name or not pod_namespace:
            raise RuntimeError("env POD_NAME or env POD_NAMESPACE not set")

        container_name = (
            f"k8s://{pod_namespace}/{pod_name}/{container_name}"  # 完整字面量
        )

        # 支持单个namespace字符串的向后兼容
        if isinstance(namespace_list, str):
            namespace_list = [namespace_list]

        # 默认namespace列表
        if not namespace_list:
            raise RuntimeError("nerdctl namespace not set")

        retries = 0
        status = False
        cid = ""
        found_namespace = ""
        while retries < 5:
            try:
                cid, found_namespace = cls._find_container_id(namespace_list, container_name)
                status = True
                break
            except RuntimeError as e:
                time.sleep(1)
                print("Failed to find container try again:", e)
                retries += 1
        if status:
            return cls(
                container_id=cid,
                namespace=found_namespace,
            )
        raise RuntimeError("Failed to find target container")

    @staticmethod
    def _find_container_id(namespace_list: list[str], container_name: str) -> tuple[str, str]:
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
        raise RuntimeError(
            f"no container matched '{container_name}' in namespaces {namespace_list}"
        )