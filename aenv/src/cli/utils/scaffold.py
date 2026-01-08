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
Scaffolding tool - for project initialization
"""
import dataclasses
import json
import shutil
from abc import ABC, abstractmethod
from pathlib import Path

import click

from cli.utils.cli_config import get_config_manager


@dataclasses.dataclass
class ScaffoldParams(ABC):
    name: str
    version: str
    target_dir: str
    template: str
    policy: str


class AEnvScaffold(ABC):
    @abstractmethod
    def init(self, params: ScaffoldParams):
        pass

    def show_directory_structure(self, path: Path, prefix: str = ""):
        """Display directory structure"""
        items = sorted(path.iterdir())
        for i, item in enumerate(items):
            is_last = i == len(items) - 1
            current_prefix = "└── " if is_last else "├── "

            if item.is_dir():
                click.echo(f"{prefix}{current_prefix}{item.name}/")
                extension = "    " if is_last else "│   "
                self.show_directory_structure(item, prefix + extension)
            else:
                click.echo(f"{prefix}{current_prefix}{item.name}")

    def get_template_config(self, template_name: str) -> dict:
        """Get config.json content from template.

        Args:
            template_name: Name of the template

        Returns:
            Dictionary containing config.json content from template

        Raises:
            click.ClickException: If template or config.json not found
        """
        template_dir = self._find_template_directory(template_name)
        if not template_dir or not template_dir.exists():
            available_templates = self._list_available_templates()
            raise click.ClickException(
                f"Template '{template_name}' not found. Available templates: {', '.join(available_templates)}"
            )

        config_path = template_dir / "config.json"
        if not config_path.exists():
            raise click.ClickException(
                f"config.json not found in template '{template_name}' at {config_path}"
            )

        try:
            with open(config_path, "r") as f:
                return json.load(f)
        except (json.JSONDecodeError, IOError) as e:
            raise click.ClickException(f"Failed to read config.json from template: {e}")

    def update_config_json(self, target_path: Path, name: str, version: str):
        """Update config.json with project-specific information.

        Args:
            target_path: Path to the initialized project
            name: Project name
            version: Project version
        """
        config_path = target_path / "config.json"
        if not config_path.exists():
            # Create default config if it doesn't exist
            default_config = {
                "name": name,
                "version": version,
                "description": "Project initialized from local template",
                "type": "local",
            }
            with open(config_path, "w") as f:
                json.dump(default_config, f, indent=2)
            return

        # Update existing config.json
        try:
            with open(config_path, "r+") as cf:
                # Read and parse config
                config = json.load(cf)
                config["name"] = name
                config["version"] = version

                # Write back updated config
                cf.seek(0)
                cf.truncate()
                json.dump(config, cf, indent=2)
        except (json.JSONDecodeError, IOError) as e:
            click.echo(f"Warning: Could not update config.json: {e}", err=True)


class LocalScaffold(AEnvScaffold):
    def init(self, params: ScaffoldParams):
        """Initialize project from local templates.

        This method:
        1. Locates the template directory within the package's templates directory
        2. Copies the template to the target directory
        3. Updates the config.json with project-specific information

        Args:
            params: Scaffold parameters including name, version, target_dir, template, and policy

        Raises:
            click.ClickException: If template not found or directory already exists
        """
        force = params.policy == "force"
        target_path = Path(params.target_dir)

        # Check if target directory already exists
        if target_path.exists() and not force:
            if any(target_path.iterdir()):  # Directory is not empty
                raise click.ClickException(
                    f"Target directory '{params.target_dir}' already exists and is not empty, use --force to overwrite"
                )

        # Find template directory within package
        template_dir = self._find_template_directory(params.template)
        if not template_dir or not template_dir.exists():
            available_templates = self._list_available_templates()
            raise click.ClickException(
                f"Template '{params.template}' not found. Available templates: {', '.join(available_templates)}"
            )

        # Copy template to target directory
        self._copy_template(template_dir, target_path, force)

        # Update config.json with project information
        self.update_config_json(target_path, params.name, params.version)

        click.echo(
            f"✓ Successfully initialized project from template '{params.template}'"
        )
        self.show_directory_structure(target_path)

    def _find_template_directory(self, template_name: str) -> Path:
        """Find template directory within package resources.

        This method works both in development and when packaged as SDK.

        Args:
            template_name: Name of the template to find

        Returns:
            Path to the template directory, or None if not found
        """
        try:
            # Try to get template directory using importlib.resources
            try:
                import importlib.resources

                # For Python 3.9+
                if hasattr(importlib.resources, "files"):
                    try:
                        templates_root = importlib.resources.files("cli.templates")
                        template_path = templates_root / template_name
                        if template_path.exists() and template_path.is_dir():
                            return template_path
                    except (ImportError, ModuleNotFoundError):
                        pass
                else:
                    # For Python 3.8 compatibility
                    try:
                        with importlib.resources.path(
                            "cli.templates", template_name
                        ) as template_path:
                            if template_path.exists() and template_path.is_dir():
                                return template_path
                    except (ImportError, ModuleNotFoundError, FileNotFoundError):
                        pass
            except ImportError:
                pass

            # Fallback 1: Try relative path from current file (development mode)
            current_file_dir = Path(__file__).parent
            templates_dir = current_file_dir / ".." / "templates"
            template_path = templates_dir / template_name
            if template_path.exists() and template_path.is_dir():
                return template_path.resolve()

            # Fallback 2: Try absolute path within package (installed mode)
            import cli

            package_dir = Path(cli.__file__).parent
            templates_dir = package_dir / "templates"
            template_path = templates_dir / template_name
            if template_path.exists() and template_path.is_dir():
                return template_path

        except Exception as e:
            click.echo(f"Debug: Error finding template: {e}", err=True)

        return None

    def _list_available_templates(self) -> list[str]:
        """List all available templates in the templates directory.

        Returns:
            List of available template names
        """
        try:
            import importlib.resources

            try:
                # For Python 3.9+
                if hasattr(importlib.resources, "files"):
                    templates_root = importlib.resources.files("cli.templates")
                    return [
                        item.name
                        for item in templates_root.iterdir()
                        if item.is_dir() and not item.name.startswith("__")
                    ]
                else:
                    # For Python 3.8 compatibility
                    import pkg_resources

                    templates_root = pkg_resources.resource_filename("cli", "templates")
                    templates_path = Path(templates_root)
                    if templates_path.exists():
                        return [
                            item.name
                            for item in templates_path.iterdir()
                            if item.is_dir() and not item.name.startswith("__")
                        ]
            except (ImportError, ModuleNotFoundError):
                pass

            # Fallback to file system
            current_file_dir = Path(__file__).parent
            templates_dir = current_file_dir / ".." / "templates"
            if templates_dir.exists():
                return [
                    item.name
                    for item in templates_dir.iterdir()
                    if item.is_dir() and not item.name.startswith("__")
                ]

        except Exception:
            pass
        return []

    def _copy_template(
        self, template_dir: Path, target_path: Path, force: bool = False
    ):
        """Copy template directory to target location.

        Args:
            template_dir: Source template directory
            target_path: Target directory path
            force: Whether to force overwrite existing directory
        """
        if target_path.exists() and force:
            shutil.rmtree(target_path)

        # Ensure target directory exists
        target_path.mkdir(parents=True, exist_ok=True)

        # Copy all files and directories
        for item in template_dir.iterdir():
            if item.name.startswith("__"):  # Skip __pycache__ and similar
                continue

            dest_path = target_path / item.name
            if item.is_dir():
                shutil.copytree(item, dest_path, dirs_exist_ok=True)
            else:
                shutil.copy2(item, dest_path)


def load_aenv_scaffold() -> AEnvScaffold:
    mode = get_config_manager().get("global_mode")
    if mode == "local":
        return LocalScaffold()
    else:
        raise click.ClickException(
            f"Unsupported mode: {mode}. Only 'local' mode is supported for scaffolding."
        )
