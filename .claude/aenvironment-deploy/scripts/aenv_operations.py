#!/usr/bin/env python3
"""
AEnvironment Operations Library

Core operations for interacting with AEnvironment CLI.
Used by high-level workflow scripts.
"""

import json
import subprocess
import sys
from pathlib import Path
from typing import Dict, List, Optional, Tuple


class AEnvError(Exception):
    """Base exception for AEnv operations."""
    pass


class AEnvOperations:
    """Core AEnvironment CLI operations."""

    def __init__(self, verbose: bool = False):
        self.verbose = verbose
        self._check_aenv_installed()

    def _check_aenv_installed(self) -> None:
        """Check if aenv CLI is installed."""
        try:
            result = subprocess.run(
                ["aenv", "--help"],
                capture_output=True,
                text=True,
                timeout=5
            )
            if result.returncode != 0:
                raise AEnvError("aenv CLI is not properly installed")
        except (subprocess.TimeoutExpired, FileNotFoundError) as e:
            raise AEnvError(f"aenv CLI is not available: {e}")

    def _run_command(
        self,
        cmd: List[str],
        cwd: Optional[str] = None,
        timeout: int = 300,
        retry: int = 2
    ) -> Tuple[int, str, str]:
        """
        Run command with retry logic.

        Returns: (exit_code, stdout, stderr)
        """
        for attempt in range(retry + 1):
            try:
                if self.verbose:
                    print(f"Running: {' '.join(cmd)}", file=sys.stderr)

                result = subprocess.run(
                    cmd,
                    capture_output=True,
                    text=True,
                    timeout=timeout,
                    cwd=cwd
                )

                if result.returncode == 0 or attempt == retry:
                    return result.returncode, result.stdout, result.stderr

                if self.verbose:
                    print(f"Attempt {attempt + 1} failed, retrying...", file=sys.stderr)

            except subprocess.TimeoutExpired:
                if attempt == retry:
                    return 1, "", f"Command timed out after {timeout}s"
            except Exception as e:
                if attempt == retry:
                    return 1, "", str(e)

        return 1, "", "All retry attempts failed"

    def configure_cli(
        self,
        owner_name: str,
        api_service_url: str,
        envhub_url: str,
        storage_type: str = "local",
        registry_config: Optional[Dict[str, str]] = None
    ) -> Dict:
        """Configure AEnvironment CLI."""
        commands = [
            ["aenv", "config", "set", "owner", owner_name],
            ["aenv", "config", "set", "storage_config.type", storage_type],
            ["aenv", "config", "set", "system_url", api_service_url],
            ["aenv", "config", "set", "hub_config.hub_backend", envhub_url],
        ]

        if registry_config:
            for key in ["host", "username", "password", "namespace"]:
                if registry_config.get(key):
                    commands.append([
                        "aenv", "config", "set",
                        f"build_config.registry.{key}",
                        registry_config[key]
                    ])

        for cmd in commands:
            exit_code, stdout, stderr = self._run_command(cmd)
            if exit_code != 0:
                raise AEnvError(f"Configuration failed: {stderr}")

        return {"status": "success", "message": "CLI configured"}

    def init_environment(
        self,
        env_name: str,
        config_only: bool = False,
        target_dir: Optional[str] = None
    ) -> Dict:
        """Initialize environment configuration."""
        work_dir = target_dir or "."
        Path(work_dir).mkdir(parents=True, exist_ok=True)

        cmd = ["aenv", "init", env_name]
        if config_only:
            cmd.append("--config-only")

        exit_code, stdout, stderr = self._run_command(cmd, cwd=work_dir)

        if exit_code != 0:
            raise AEnvError(f"Init failed: {stderr}")

        env_path = Path(work_dir) / env_name
        return {
            "status": "success",
            "env_directory": str(env_path),
            "message": f"Environment '{env_name}' initialized"
        }

    def update_config_with_image(
        self,
        env_directory: str,
        image_name: str
    ) -> None:
        """Update config.json with existing image."""
        config_path = Path(env_directory) / "config.json"

        if not config_path.exists():
            raise AEnvError(f"config.json not found in {env_directory}")

        with open(config_path, "r") as f:
            config = json.load(f)

        config["artifacts"] = [{"type": "image", "content": image_name}]

        with open(config_path, "w") as f:
            json.dump(config, f, indent=2)

    def build_image(
        self,
        env_directory: str,
        platform: str = "linux/amd64",
        push: bool = True
    ) -> Dict:
        """Build Docker image."""
        if not Path(env_directory).exists():
            raise AEnvError(f"Directory not found: {env_directory}")

        cmd = ["aenv", "build", "--platform", platform]
        if push:
            cmd.append("--push")

        exit_code, stdout, stderr = self._run_command(
            cmd,
            cwd=env_directory,
            timeout=600
        )

        if exit_code != 0:
            raise AEnvError(f"Build failed: {stderr}")

        return {
            "status": "success",
            "message": "Image built successfully",
            "output": stdout
        }

    def register_environment(self, env_directory: str) -> Dict:
        """Register environment to EnvHub."""
        if not Path(env_directory).exists():
            raise AEnvError(f"Directory not found: {env_directory}")

        cmd = ["aenv", "push"]
        exit_code, stdout, stderr = self._run_command(
            cmd,
            cwd=env_directory,
            timeout=300
        )

        if exit_code != 0:
            raise AEnvError(f"Registration failed: {stderr}")

        # Extract env info from config.json
        config_path = Path(env_directory) / "config.json"
        if config_path.exists():
            with open(config_path, "r") as f:
                config = json.load(f)
                env_name = config.get("name", "unknown")
                version = config.get("version", "unknown")
        else:
            env_name = version = "unknown"

        return {
            "status": "success",
            "env_name": env_name,
            "version": version,
            "message": f"Environment {env_name}@{version} registered"
        }

    def list_environments(self) -> List[str]:
        """List registered environments."""
        cmd = ["aenv", "list"]
        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"List failed: {stderr}")

        # Parse environment names from rich table output
        envs = []
        in_table = False
        for line in stdout.split("\n"):
            line = line.strip()
            # Skip empty lines, headers, and table decorations
            if not line or line.startswith("Available") or line.startswith("┏") or \
               line.startswith("┃") or line.startswith("┡") or line.startswith("└"):
                continue
            # Data rows start with │
            if line.startswith("│"):
                parts = [p.strip() for p in line.split("│") if p.strip()]
                if len(parts) >= 2:  # name and version
                    env_name = parts[0]
                    version = parts[1]
                    envs.append(f"{env_name}@{version}")

        return envs

    def get_environment(self, env_name: str) -> Dict:
        """Get environment details."""
        cmd = ["aenv", "get", env_name]
        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"Environment {env_name} not found: {stderr}")

        return {"status": "success", "output": stdout}

    def create_instance(
        self,
        env_spec: str,
        ttl: str = "24h",
        keep_alive: bool = True,
        skip_health: bool = True,
        env_vars: Optional[Dict[str, str]] = None
    ) -> Dict:
        """Create environment instance."""
        cmd = ["aenv", "instance", "create", env_spec, "--ttl", ttl]

        if keep_alive:
            cmd.append("--keep-alive")
        if skip_health:
            cmd.append("--skip-health")

        if env_vars:
            for key, value in env_vars.items():
                cmd.extend(["-e", f"{key}={value}"])

        exit_code, stdout, stderr = self._run_command(cmd, timeout=180)

        if exit_code != 0:
            raise AEnvError(f"Instance creation failed: {stderr}")

        # Extract instance info from output
        instance_id = "unknown"
        ip_address = "unknown"

        for line in stdout.split("\n"):
            if "id" in line.lower():
                parts = line.split()
                if len(parts) >= 2:
                    instance_id = parts[-1]
            elif "ip" in line.lower() or "address" in line.lower():
                parts = line.split()
                for part in parts:
                    if self._is_valid_ip(part):
                        ip_address = part

        return {
            "status": "success",
            "instance_id": instance_id,
            "ip_address": ip_address,
            "message": f"Instance created: {instance_id}",
            "output": stdout
        }

    def create_service(
        self,
        env_spec: str,
        replicas: int = 1,
        port: int = 8080,
        enable_storage: bool = False,
        env_vars: Optional[Dict[str, str]] = None
    ) -> Dict:
        """Create environment service."""
        if enable_storage and replicas > 1:
            raise AEnvError("Storage requires replicas=1 (ReadWriteOnce)")

        cmd = [
            "aenv", "service", "create", env_spec,
            "--replicas", str(replicas),
            "--port", str(port)
        ]

        if enable_storage:
            cmd.append("--enable-storage")

        if env_vars:
            for key, value in env_vars.items():
                cmd.extend(["-e", f"{key}={value}"])

        exit_code, stdout, stderr = self._run_command(cmd, timeout=180)

        if exit_code != 0:
            raise AEnvError(f"Service creation failed: {stderr}")

        # Extract service info from log output and table
        service_id = "unknown"
        access_url = "unknown"

        for line in stdout.split("\n"):
            # Look for service ID in log line or table
            if "service created:" in line.lower():
                parts = line.split(":")
                if len(parts) >= 2:
                    service_id = parts[-1].strip().rstrip('[0m')
            elif "│ service id" in line.lower():
                parts = [p.strip() for p in line.split("│") if p.strip()]
                if len(parts) >= 2:
                    service_id = parts[1]
            elif "│ service url" in line.lower():
                parts = [p.strip() for p in line.split("│") if p.strip()]
                if len(parts) >= 2:
                    access_url = parts[1]

        return {
            "status": "success",
            "service_id": service_id,
            "access_url": access_url,
            "message": f"Service created: {service_id}",
            "output": stdout
        }

    def list_instances(self) -> str:
        """List instances."""
        cmd = ["aenv", "instance", "list"]
        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"List instances failed: {stderr}")

        return stdout

    def get_instance_details(self, instance_id: str) -> Dict:
        """Get instance details."""
        cmd = ["aenv", "instance", "get", instance_id]
        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"Get instance failed: {stderr}")

        return {"status": "success", "output": stdout}

    def list_services(self) -> str:
        """List services."""
        cmd = ["aenv", "service", "list"]
        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"List services failed: {stderr}")

        return stdout

    def get_service_details(self, service_id: str) -> Dict:
        """Get service details."""
        cmd = ["aenv", "service", "get", service_id]
        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"Get service failed: {stderr}")

        return {"status": "success", "output": stdout}

    def delete_instance(self, instance_id: str) -> Dict:
        """Delete instance."""
        cmd = ["aenv", "instance", "delete", instance_id, "-y"]
        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"Delete instance failed: {stderr}")

        return {
            "status": "success",
            "message": f"Instance {instance_id} deleted"
        }

    def delete_service(
        self,
        service_id: str,
        delete_storage: bool = False
    ) -> Dict:
        """Delete service."""
        cmd = ["aenv", "service", "delete", service_id, "-y"]
        if delete_storage:
            cmd.append("--delete-storage")

        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"Delete service failed: {stderr}")

        return {
            "status": "success",
            "message": f"Service {service_id} deleted"
        }

    def update_service(
        self,
        service_id: str,
        replicas: Optional[int] = None,
        environment_variables: Optional[Dict[str, str]] = None
    ) -> Dict:
        """Update service configuration."""
        cmd = ["aenv", "service", "update", service_id]

        if replicas is not None:
            cmd.extend(["--replicas", str(replicas)])

        if environment_variables:
            for key, value in environment_variables.items():
                cmd.extend(["-e", f"{key}={value}"])

        exit_code, stdout, stderr = self._run_command(cmd)

        if exit_code != 0:
            raise AEnvError(f"Update service failed: {stderr}")

        return {
            "status": "success",
            "message": f"Service {service_id} updated"
        }

    @staticmethod
    def _is_valid_ip(ip_str: str) -> bool:
        """Check if string is valid IP address."""
        parts = ip_str.split(".")
        if len(parts) != 4:
            return False
        try:
            return all(0 <= int(part) <= 255 for part in parts)
        except ValueError:
            return False
