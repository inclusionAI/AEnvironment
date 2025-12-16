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

"""
Configuration utility
"""
import os
from pathlib import Path


def get_default_config_dir() -> str:
    """Get default configuration directory"""
    return os.path.expanduser("~/.aenv")


def ensure_config_dir() -> str:
    """Ensure configuration directory exists"""
    config_dir = get_default_config_dir()
    Path(config_dir).mkdir(exist_ok=True)
    return config_dir


def get_env_file_path(name: str) -> str:
    """Get environment configuration file path"""
    config_dir = get_default_config_dir()
    return os.path.join(config_dir, "envs", f"{name}.yaml")
