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
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Dict, Optional


@dataclass
class CLIConfig:
    """Unified CLI configuration structure."""

    # Global settings
    global_mode: str = "local"

    # Build configuration
    build_config: Dict[str, Any] = None

    # Storage configuration
    storage_config: Dict[str, Any] = None

    # Logging configuration
    logging_config: Dict[str, Any] = None

    # AEnv hub backend configuration
    hub_config: Dict[str, Any] = None

    def __post_init__(self):
        """Initialize default configurations."""
        if self.build_config is None:
            socket_path = (
                "npipe:////./pipe/docker_engine"
                if sys.platform == "win32"
                else "unix:///var/run/docker.sock"
            )
            self.build_config = {
                "type": "local",
                "build_args": {
                    "socket": socket_path,
                },
                "registry": {
                    "host": "docker.io",
                    "username": "",
                    "password": "",
                    "namespace": "aenv",
                },
            }
        if self.storage_config is None:
            self.storage_config = {
                "type": "local",
                "custom": {"prefix": "~/.aenv/envs"},
            }
        if self.logging_config is None:
            self.logging_config = {
                "level": "INFO",
                "format": "%(asctime)s - %(name)s - %(levelname)s - %(message)s",
                "file": None,
                "console": True,
            }

        if self.hub_config is None:
            self.hub_config = {
                "hub_backend": "http://localhost:8080",
                "api_key": "",
                "timeout": 30,
            }


class CLIConfigManager:
    """Manager for CLI configuration files."""

    DEFAULT_CONFIG_NAME = "cli_config.json"
    DEFAULT_CONFIG_PATHS = [
        "~/.aenv/cli_config.json",
        ".aenv/cli_config.json",
        "/etc/aenv/cli_config.json",
    ]

    def __init__(self, config_path: Optional[str] = None):
        """Initialize CLI config manager.

        Args:
            config_path: Optional custom config file path
        """
        self.config_path = self._resolve_config_path(config_path)
        self.config = self._load_config()

    def _resolve_config_path(self, custom_path: Optional[str] = None) -> Path:
        """Resolve the configuration file path.

        Args:
            custom_path: Custom config path if provided

        Returns:
            Path to the configuration file
        """
        if custom_path:
            return Path(custom_path).expanduser().resolve()

        # Check default paths in order of preference
        for path_str in self.DEFAULT_CONFIG_PATHS:
            path = Path(path_str).expanduser().resolve()
            if path.exists():
                return path

        # Return the first default path (local .aenv)
        return Path(self.DEFAULT_CONFIG_PATHS[0]).expanduser().resolve()

    def _load_config(self) -> CLIConfig:
        """Load configuration from file.

        Returns:
            CLIConfig instance
        """
        if not self.config_path.exists():
            # Create default config if it doesn't exist
            self.config_path.parent.mkdir(parents=True, exist_ok=True)
            config = CLIConfig()
            self.save_config(config)
            return config

        try:
            with open(self.config_path, "r", encoding="utf-8") as f:
                data = json.load(f)
            config = CLIConfig(**data)

            # Auto-fix Windows socket path if it's using the Unix default
            if sys.platform == "win32" and config.build_config:
                current_socket = config.build_config.get("build_args", {}).get("socket")
                if current_socket == "unix:///var/run/docker.sock":
                    config.build_config["build_args"][
                        "socket"
                    ] = "npipe:////./pipe/docker_engine"
                    self.save_config(config)

            return config
        except (json.JSONDecodeError, TypeError) as e:
            # If config is invalid, create default
            print(
                f"Warning: Invalid config file {self.config_path}, using defaults: {e}"
            )
            config = CLIConfig()
            self.save_config(config)
            return config

    def save_config(self, config: CLIConfig) -> None:
        """Save configuration to file.

        Args:
            config: CLIConfig instance to save
        """
        self.config_path.parent.mkdir(parents=True, exist_ok=True)

        with open(self.config_path, "w", encoding="utf-8") as f:
            json.dump(asdict(config), f, indent=2, ensure_ascii=False)

    def get(self, key: str, default: Any = None) -> Any:
        """Get configuration value by key.

        Args:
            key: Configuration key (supports dot notation)
            default: Default value if key not found

        Returns:
            Configuration value
        """
        keys = key.split(".")
        value = asdict(self.config)

        try:
            for k in keys:
                value = value[k]
            return value
        except (KeyError, TypeError):
            return default

    def set(self, key: str, value: Any) -> None:
        """Set configuration value by key.

        Args:
            key: Configuration key (supports dot notation)
            value: Value to set
        """
        keys = key.split(".")
        config_dict = asdict(self.config)

        # Navigate to the parent dict
        current = config_dict
        for k in keys[:-1]:
            if k not in current:
                current[k] = {}
            current = current[k]

        # Set the value
        current[keys[-1]] = value

        # Update config and save
        self.config = CLIConfig(**config_dict)
        self.save_config(self.config)

    def update_from_dict(self, updates: Dict[str, Any]) -> None:
        """Update configuration from dictionary.

        Args:
            updates: Dictionary of updates to apply
        """
        config_dict = asdict(self.config)
        config_dict.update(updates)
        self.config = CLIConfig(**config_dict)
        self.save_config(self.config)

    def get_build_config(self) -> Dict[str, Any]:
        """Get build configuration."""
        return self.config.build_config

    def get_hub_config(self) -> Dict[str, Any]:
        """Get deploy configuration."""
        return self.config.hub_config

    def get_storage_config(self) -> Dict[str, Any]:
        """Get storage configuration."""
        return self.config.storage_config

    def get_logging_config(self) -> Dict[str, Any]:
        """Get logging configuration."""
        return self.config.logging_config


# Global config manager instance
_config_manager = None


def get_config_manager(config_path: Optional[str] = None) -> CLIConfigManager:
    """Get global config manager instance.

    Args:
        config_path: Optional custom config path

    Returns:
        CLIConfigManager instance
    """
    global _config_manager
    if _config_manager is None or config_path is not None:
        _config_manager = CLIConfigManager(config_path)
    return _config_manager
