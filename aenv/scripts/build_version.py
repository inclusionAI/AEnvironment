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

"""
Build-time version information generator
Automatically creates version_info.json containing Git information during packaging
"""

import json
import platform
import subprocess
from datetime import datetime
from pathlib import Path


def get_git_info():
    """Get current git repository information"""
    info = {"commit": "unknown", "branch": "unknown", "date": "unknown", "dirty": False}

    try:
        repo_root = Path(__file__).parent.parent

        # Get commit hash
        result = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            cwd=repo_root,
            capture_output=True,
            text=True,
            check=True,
        )
        info["commit"] = result.stdout.strip()

        # Get branch name
        result = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "HEAD"],
            cwd=repo_root,
            capture_output=True,
            text=True,
            check=True,
        )
        info["branch"] = result.stdout.strip()

        # Get commit time
        result = subprocess.run(
            ["git", "show", "-s", "--format=%ci", "HEAD"],
            cwd=repo_root,
            capture_output=True,
            text=True,
            check=True,
        )
        info["date"] = result.stdout.strip()

        # Check if there are uncommitted changes
        result = subprocess.run(
            ["git", "status", "--porcelain"],
            cwd=repo_root,
            capture_output=True,
            text=True,
            check=True,
        )
        info["dirty"] = bool(result.stdout.strip())

    except (subprocess.CalledProcessError, FileNotFoundError):
        pass

    return info


def get_package_version():
    """Read version number from pyproject.toml"""
    try:
        import tomllib

        pyproject_path = Path(__file__).parent.parent / "pyproject.toml"
        with open(pyproject_path, "rb") as f:
            data = tomllib.load(f)
            return data.get("project", {}).get("version", "unknown")
    except ImportError:
        try:
            import tomli

            pyproject_path = Path(__file__).parent.parent / "pyproject.toml"
            with open(pyproject_path, "rb") as f:
                data = tomli.load(f)
                return data.get("project", {}).get("version", "unknown")
        except BaseException as e:
            print(f"Can't load pyproject use tomlii with error:{e}")
            return "unknown"
    except BaseException as e:
        print(f"Could not load pyproject use tomllib with error:{e}")
        return "unknown"


def generate_version_info():
    """Generate version information file"""
    git_info = get_git_info()
    package_version = get_package_version()

    version_info = {
        "version": package_version,
        "source": "pypi",
        "build_version": package_version,
        "build_commit": git_info["commit"],
        "build_date": datetime.now().isoformat(),
        "build_branch": git_info["branch"],
        "python_version": platform.python_version(),
        "build_host": platform.node(),
        "git_commit": git_info["commit"],
        "git_branch": git_info["branch"],
        "git_date": git_info["date"],
        "dirty": git_info["dirty"],
    }

    # Write to cli package directory
    output_path = (
        Path(__file__).parent.parent / "src" / "cli" / "data" / "version_info.json"
    )
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(version_info, f, indent=2, ensure_ascii=False)

    print(f"✅ Version information generated: {output_path}")
    print(f"   Version: {package_version}")
    print(f"   Commit: {git_info['commit']}")
    print(f"   Branch: {git_info['branch']}")

    return version_info


if __name__ == "__main__":
    generate_version_info()
