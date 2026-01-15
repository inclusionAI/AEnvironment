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

import sys
from abc import ABC, abstractmethod
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Optional

from cli.utils.cli_config import get_config_manager

try:
    import docker
except ImportError:
    docker = None


@dataclass
class ArtifactBuildContext:
    """Context for artifact building operations."""

    work_dir: str
    platform: str
    build_config: Optional[Dict[str, Any]] = None
    image_name: Optional[str] = None
    image_tag: Optional[str] = None
    registry: Optional[str] = None
    namespace: Optional[str] = None
    push_required: Optional[bool] = False


class ArtifactBuilder(ABC):
    """Abstract base class for artifact builders."""

    @abstractmethod
    def trigger(self, ctx: ArtifactBuildContext):
        """Trigger the CI pipeline for artifact building.

        Args:
            ctx: The build context containing configuration and paths.
        """
        pass


class DockerArtifactBuilder(ArtifactBuilder):
    """Docker artifact builder for building and pushing Docker images."""

    def __init__(self, config):
        """Initialize Docker artifact builder.

        Args:
            sock_address: Docker daemon socket address.
        """
        if docker is None:
            raise ImportError("docker package is required but not installed")
        if not config:
            raise Exception("config is required for DockerArtifactBuilder")
        build_args = config.get("build_args", {})
        sock_address = build_args.get("socket")
        if not sock_address:
            raise Exception("you must provide socket address by config")

        self.docker_client = docker.DockerClient(base_url=sock_address)

    def trigger(self, ctx: ArtifactBuildContext):
        """Trigger Docker image build and push operations.

        This method orchestrates the complete Docker image lifecycle:
        1. Parse build configuration
        2. Build Docker image from Dockerfile using Docker Python API
        3. Tag image with appropriate registry and tags
        4. Push image to registry using Docker Python API

        Args:
            ctx: The build context containing configuration and paths.

        Raises:
            docker.errors.BuildError: If image build fails.
            docker.errors.APIError: If Docker API returns an error.
            FileNotFoundError: If Dockerfile is not found.
        """

        dockerfile_path = self._get_dockerfile_path(ctx)
        image_name = self._get_image_name(ctx)
        tags = self._get_image_tags(ctx)

        # Build the Docker image
        self._build_image(dockerfile_path, image_name, ctx.platform, tags, ctx.work_dir)

        # Push the image to registry
        if ctx.push_required:
            self._push_image(image_name, tags)

    def _get_dockerfile_path(self, ctx: ArtifactBuildContext) -> str:
        """Get the absolute path to the Dockerfile.

        Args:
            ctx: The build context.

        Returns:
            Absolute path to the Dockerfile.

        Raises:
            FileNotFoundError: If Dockerfile is not found.
        """
        dockerfile = ctx.build_config.get("dockerfile", "./Dockerfile")
        dockerfile_path = Path(ctx.work_dir) / dockerfile

        if not dockerfile_path.exists():
            raise FileNotFoundError(f"Dockerfile not found at: {dockerfile_path}")

        return str(dockerfile_path)

    def _get_image_name(self, ctx: ArtifactBuildContext) -> str:
        """Get the full image name including registry.

        Args:
            ctx: The build context.

        Returns:
            Full image name with registry prefix.
        """
        if ctx.image_name:
            name = ctx.image_name
        else:
            raise Exception("Image name not provided")
        if ctx.namespace:
            name = f"{ctx.namespace}/{name}"
        if ctx.registry:
            name = f"{ctx.registry}/{name}"
        return name

    def _get_image_tags(self, ctx: ArtifactBuildContext) -> list[str]:
        """Get list of tags to apply to the image.

        Args:
            ctx: The build context.

        Returns:
            List of tags to apply.
        """
        if ctx.image_tag:
            # image_tag is already a list
            return [ctx.image_tag]

        # Default tags: latest and git commit hash if available
        tags = ["latest"]

        return tags

    def _build_image(
        self,
        dockerfile_path: str,
        image_name: str,
        platform: str,
        tags: list[str],
        build_context: str,
    ):
        """Build Docker image using Docker Python API with real-time logging.

        This method streams build logs in real-time from the Docker API, providing
        detailed progress information including each build step, layer caching,
        and any warnings or errors.

        Args:
            dockerfile_path: Path to the Dockerfile.
            image_name: Base name for the image.
            tags: List of tags to apply.
            build_context: Directory to use as build context.

        Raises:
            docker.errors.BuildError: If build fails.
            docker.errors.APIError: If Docker API returns an error.
        """
        if not build_context:
            build_context = str(Path(dockerfile_path).parent)
        dockerfile_name = Path(dockerfile_path).name

        for tag in tags:
            full_image_name = f"{image_name}:{tag}"

            print(f"🐳 Building Docker image: {full_image_name}")
            print("─" * 80)

            try:
                # Build the image with streaming logs using low-level API
                response = self.docker_client.api.build(
                    path=build_context,
                    dockerfile=dockerfile_name,
                    tag=full_image_name,
                    platform=platform,
                    rm=True,
                    forcerm=True,
                    decode=True,
                )
                # Process streaming build logs
                current_step = 0
                # last_output_time = time.time()
                # heartbeat_interval = 30  # Show heartbeat every 30 seconds if no output
                # last_heartbeat_time = time.time()

                for log_line in response:
                    if not log_line:
                        continue

                    # current_time = time.time()
                    # last_output_time = current_time

                    # Handle different types of log messages
                    if "stream" in log_line:
                        stream_text = log_line["stream"].strip()
                        if stream_text:
                            # Detect build steps
                            if stream_text.startswith("Step"):
                                current_step += 1
                                step_description = stream_text
                                print(f"\n📦 Step {current_step}: {step_description}")
                                sys.stdout.flush()
                            elif "Running in" in stream_text:
                                container_id = stream_text.split("Running in ")[
                                    1
                                ].strip()
                                print(
                                    f"   🏃 Running in container: {container_id[:12]}"
                                )
                                sys.stdout.flush()
                            elif "Removing intermediate container" in stream_text:
                                container_id = stream_text.split(
                                    "Removing intermediate container "
                                )[1].split()[0]
                                print(
                                    f"   🗑️  Removing intermediate container: {container_id[:12]}"
                                )
                                sys.stdout.flush()
                            elif "Successfully built" in stream_text:
                                image_id = stream_text.split("Successfully built ")[
                                    1
                                ].strip()
                                print(f"   ✅ Successfully built: {image_id[:12]}")
                                sys.stdout.flush()
                            elif "Successfully tagged" in stream_text:
                                tagged_image = stream_text.split(
                                    "Successfully tagged "
                                )[1].strip()
                                print(f"   🏷️  Successfully tagged: {tagged_image}")
                                sys.stdout.flush()
                            else:
                                # Regular build output - print all output to show progress
                                if stream_text:
                                    # Filter out very verbose output but keep important ones
                                    if not stream_text.startswith("--->"):
                                        print(f"   {stream_text}")
                                        sys.stdout.flush()
                                    elif stream_text.startswith("--->"):
                                        print(f"   {stream_text}")
                                        sys.stdout.flush()

                    elif "status" in log_line:
                        status = log_line["status"]
                        if "id" in log_line:
                            layer_id = log_line["id"]
                            progress = log_line.get("progress", "")
                            if progress:
                                # Show progress for layer downloads
                                current = log_line.get("progressDetail", {}).get(
                                    "current", 0
                                )
                                total = log_line.get("progressDetail", {}).get(
                                    "total", 0
                                )
                                if total > 0:
                                    percentage = (current / total) * 100
                                    bar_length = 20
                                    filled_length = int(bar_length * current // total)
                                    bar = "█" * filled_length + "░" * (
                                        bar_length - filled_length
                                    )
                                    print(
                                        f"   📥 {layer_id}: {status} [{bar}] {percentage:.1f}%",
                                        end="\r",
                                    )
                                    sys.stdout.flush()
                                else:
                                    print(f"   📥 {layer_id}: {status}")
                                    sys.stdout.flush()
                            else:
                                print(f"   📥 {layer_id}: {status}")
                                sys.stdout.flush()

                    elif "aux" in log_line:
                        aux_data = log_line["aux"]
                        if "ID" in aux_data:
                            print(f"   🎯 Build complete: {aux_data['ID'][:12]}")
                            sys.stdout.flush()

                    elif "error" in log_line:
                        error_msg = log_line["error"].strip()
                        print(f"   ❌ Error: {error_msg}")
                        sys.stdout.flush()
                        raise docker.errors.BuildError(error_msg, [log_line])

                    elif "errorDetail" in log_line:
                        error_detail = log_line["errorDetail"]
                        error_msg = error_detail.get("message", "Unknown error")
                        print(f"   ❌ Error: {error_msg}")
                        sys.stdout.flush()
                        raise docker.errors.BuildError(error_msg, [log_line])

                    # Show heartbeat if no output for a while (handled in a separate check)
                    # Note: This is a simple approach. For better UX, consider using threading
                    # to show periodic heartbeats during long-running steps

                    # Handle any other log line types that might be present
                    else:
                        # Log unknown log line types for debugging
                        if log_line:
                            # Only print if it's not empty and might be useful
                            if any(
                                key in log_line for key in ["message", "log", "output"]
                            ):
                                message = (
                                    log_line.get("message")
                                    or log_line.get("log")
                                    or log_line.get("output", "")
                                )
                                if message:
                                    print(f"   {message}")
                                    sys.stdout.flush()

                print("\n─" * 80)
                print(f"✅ Successfully built image: {full_image_name}")

            except docker.errors.BuildError as e:
                print(f"\n❌ Build failed: {e}")
                raise
            except docker.errors.APIError as e:
                print(f"\n❌ Docker API error: {e}")
                raise
            except Exception as e:
                print(f"\n❌ Unexpected error during build: {e}")
                raise

    def _push_image(self, image_name: str, tags: list[str]):
        """Push Docker image to registry using Docker Python API.

        Args:
            image_name: Base name of the image.
            tags: List of tags to push.

        Raises:
            docker.errors.APIError: If push fails.
        """
        config_manager = get_config_manager()
        build_config = config_manager.get_build_config()
        registry_settings = build_config.get("registry", {})
        user = registry_settings.get("username", "")
        passwd = registry_settings.get("password")
        auth_config = {
            "username": user,
            "password": passwd,
        }
        for tag in tags:
            full_image_name = f"{image_name}:{tag}"

            print(f"Pushing Docker image: {full_image_name}")

            try:
                # Push the image and process streaming response
                push_logs = self.docker_client.images.push(
                    repository=image_name,
                    tag=tag,
                    stream=True,
                    decode=True,
                    auth_config=auth_config,
                )

                # Process push logs
                for log_data in push_logs:
                    if "status" in log_data:
                        print(log_data["status"])
                    if "error" in log_data:
                        raise docker.errors.APIError(log_data["error"])
                    if "progress" in log_data and log_data.get("progress"):
                        print(f"{log_data.get('id', '')}: {log_data['progress']}")

                print(f"Successfully pushed image: {full_image_name}")

            except Exception as e:
                print(f"Unexpected error during push: {e}")
                raise


def load_builder():
    build_config = get_config_manager().get_build_config()
    build_type = build_config.get("type", "local")
    if build_type == "local":
        return DockerArtifactBuilder(build_config)
    else:
        raise ValueError(f"Unknown build type: {build_type}")
