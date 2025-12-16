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
Test script for the build command
"""
import json
import subprocess
import sys
import tempfile
from pathlib import Path

from cli.client.aenv_hub_client import AEnvHubClient
from cli.extends.artifacts.artifacts_builder import DockerArtifactBuilder


def test_build_command():
    """Test the build command with a simple project."""

    with tempfile.TemporaryDirectory() as temp_dir:
        temp_path = Path(temp_dir)

        # Create a simple project structure
        project_dir = temp_path / "test-project"
        project_dir.mkdir()

        # Create a simple Dockerfile
        dockerfile_content = """
FROM python:3.9-slim
WORKDIR /app
COPY . .
CMD ["python", "-c", "print('Hello from Docker!')"]
"""

        dockerfile_path = project_dir / "Dockerfile"
        with open(dockerfile_path, "w") as f:
            f.write(dockerfile_content)

        # Create a simple Python file
        app_py = project_dir / "app.py"
        with open(app_py, "w") as f:
            f.write('print("Hello from test app!")')

        # Create config.json
        config = {
            "name": "test-app",
            "version": "1.0.0",
            "buildConfig": {"dockerfile": "./Dockerfile"},
        }

        config_path = project_dir / "config.json"
        with open(config_path, "w") as f:
            json.dump(config, f, indent=2)

        print(f"Created test project at: {project_dir}")
        print("Project contents:")
        for item in project_dir.iterdir():
            print(f"  - {item.name}")

        # Test build command help
        try:
            result = subprocess.run(
                [sys.executable, "-m", "cli.cli", "build", "--help"],
                capture_output=True,
                text=True,
                cwd="/AEnvironment/aenv",
            )

            print("\nBuild command help:")
            print(result.stdout)

            if result.stderr:
                print("Stderr:", result.stderr)

        except Exception as e:
            print(f"Error running build --help: {e}")


def test_hub_client():
    client = AEnvHubClient.load_client()
    envs = client.list_environments()
    print(envs)


def test_docker_build():
    docker = DockerArtifactBuilder().docker_client
    images = docker.images.list()
    print(images)
