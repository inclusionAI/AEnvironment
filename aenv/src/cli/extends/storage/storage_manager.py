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

import os
import shutil
import tarfile
import tempfile
import zipfile
from abc import ABC, abstractmethod
from dataclasses import dataclass
from pathlib import Path
from typing import Literal, Optional

import requests

from cli.client.aenv_hub_client import AEnvHubClient
from cli.utils.archive_tool import TempArchive
from cli.utils.cli_config import get_config_manager


@dataclass
class StorageContext:
    """Context for storage operations."""

    src_url: str
    infos: dict[str, str]


@dataclass
class StorageStatus:
    state: bool
    dest_url: str


class Storage(ABC):
    """Abstract base class for storage operations."""

    @abstractmethod
    def upload(self, context: StorageContext) -> StorageStatus:
        """
        Upload file using storage context.

        Args:
            context: Storage context containing src_url and dest_url

        Returns:
            True if upload successful, False otherwise
        """
        pass

    @abstractmethod
    def download(self, context: StorageContext) -> bool:
        """
        Download file using storage context.

        Args:
            context: Storage context containing src_url and dest_url

        Returns:
            True if download successful, False otherwise
        """
        pass


class LocalStorage(Storage):
    """Local file system storage implementation with compression support."""

    def __init__(
        self,
        context: Optional[StorageContext] = None,
        compression_format: Literal["zip", "tar", "tar.gz"] = "zip",
    ):
        """
        Initialize LocalStorage with optional context and compression settings.

        Args:
            context: Storage context for configuration
            compression_format: Compression format for directories (zip, tar, tar.gz)
        """
        self.context = context
        self.compression_format = compression_format

    def upload(self, context: StorageContext) -> StorageStatus:
        """
        Upload file/directory by compressing source and copying to destination.

        For directories: Creates a compressed archive before copying
        For files: Copies directly without compression

        Args:
            context: Storage context containing src_url (source path)
                    and dest_url (destination path)

        Returns:
            True if upload was successful

        Raises:
            FileNotFoundError: If source path doesn't exist
            PermissionError: If lacking permissions to read/write
            ValueError: If compression format is invalid
            OSError: If other file system errors occur
        """
        src_path = Path(context.src_url)

        config = get_config_manager().get_storage_config()

        custom = config.get("custom", {})
        prefix = custom.get("prefix", "/home/aenvs")
        infos = context.infos
        name = infos.get("name")
        version = infos.get("version")
        full_path = f"{prefix}/{name}_{version}.zip"
        dest_path = Path(full_path)

        # Validate source path exists
        if not src_path.exists():
            raise FileNotFoundError(f"Source path not found: {context.src_url}")

        # Create destination directory
        dest_path.parent.mkdir(parents=True, exist_ok=True)

        if src_path.is_dir():
            # Directory: compress before copying
            status = self._upload_directory_compressed(src_path, dest_path)
        else:
            # File: copy directly
            status = self._upload_file_direct(src_path, dest_path)

        return StorageStatus(state=status, dest_url=full_path)

    def _upload_directory_compressed(self, src_path: Path, dest_path: Path) -> bool:
        """
        Upload directory by creating compressed archive.

        Args:
            src_path: Storage context
            dest_path: Destination path
        Returns:
            True if upload successful
        """
        # Create temporary archive
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)

            # Determine archive name and format
            archive_name = src_path.name or "archive"
            archive_filename = f"{archive_name}.{self.compression_format}"
            archive_path = temp_path / archive_filename

            print(f"📦 Compressing directory: {src_path} → {archive_filename}")

            # Create archive based on format
            if self.compression_format == "zip":
                self._create_zip_archive(src_path, archive_path)
            elif self.compression_format == "tar":
                self._create_tar_archive(src_path, archive_path, "w")
            elif self.compression_format == "tar.gz":
                self._create_tar_archive(src_path, archive_path, "w:gz")
            else:
                raise ValueError(
                    f"Unsupported compression format: {self.compression_format}"
                )

            # Copy compressed archive to destination
            final_dest = (
                dest_path / archive_filename if dest_path.is_dir() else dest_path
            )
            shutil.copy2(archive_path, final_dest)

            print(f"✅ Successfully uploaded compressed directory: {final_dest}")
            return True

    def _upload_file_direct(self, src_path: Path, dest_path: Path) -> bool:
        """
        Upload file directly without compression.

        Args:
            src_path: Src path
            dest_path: Destination path

        Returns:
            True if upload successful
        """
        # Ensure destination is a file path
        if dest_path.is_dir():
            dest_path = dest_path / src_path.name

        shutil.copy2(src_path, dest_path)
        print(f"✅ Successfully uploaded file: {dest_path}")
        return True

    def _create_zip_archive(self, source_dir: Path, archive_path: Path) -> None:
        """Create ZIP archive from directory."""
        with zipfile.ZipFile(archive_path, "w", zipfile.ZIP_DEFLATED) as zipf:
            for root, dirs, files in os.walk(source_dir):
                for file in files:
                    file_path = Path(root) / file
                    arc_path = file_path.relative_to(source_dir)
                    zipf.write(file_path, arc_path)

    def _create_tar_archive(
        self, source_dir: Path, archive_path: Path, mode: str
    ) -> None:
        """Create TAR archive from directory."""
        with tarfile.open(archive_path, mode) as tarf:
            tarf.add(source_dir, arcname=source_dir.name)

    def download(self, context: StorageContext) -> bool:
        """
        Download file/directory by extracting from compressed archive.

        For compressed archives: Extracts to destination
        For regular files: Copies directly

        Args:
            context: Storage context containing src_url (source path)
                    and dest_url (destination path)

        Returns:
            True if download successful

        Raises:
            FileNotFoundError: If source path doesn't exist
            PermissionError: If lacking permissions to read/write
            ValueError: If archive format is unsupported
            OSError: If other file system errors occur
        """
        src_path = Path(context.src_url)
        dest_path = Path(context.dest_url)

        # Validate source path exists
        if not src_path.exists():
            raise FileNotFoundError(f"Source path not found: {context.src_url}")

        # Create destination directory
        dest_path.mkdir(parents=True, exist_ok=True)

        # Check if source is a compressed archive
        if src_path.suffix in [".zip", ".tar", ".tar.gz", ".tgz"]:
            return self._download_archive_extracted(context)
        else:
            return self._download_file_direct(context)

    def _download_archive_extracted(self, context: StorageContext) -> bool:
        """
        Download by extracting compressed archive.

        Args:
            context: Storage context

        Returns:
            True if download successful
        """
        src_path = Path(context.src_url)
        dest_path = Path(context.dest_url)

        print(f"📂 Extracting archive: {src_path}")

        # Create temporary extraction directory
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)

            # Extract based on archive format
            if src_path.suffix == ".zip":
                with zipfile.ZipFile(src_path, "r") as zipf:
                    zipf.extractall(temp_path)
            elif src_path.suffix == ".tar":
                with tarfile.open(src_path, "r") as tarf:
                    tarf.extractall(temp_path)
            elif src_path.suffix in [".tar.gz", ".tgz"]:
                with tarfile.open(src_path, "r:gz") as tarf:
                    tarf.extractall(temp_path)
            else:
                raise ValueError(f"Unsupported archive format: {src_path.suffix}")

            # Move extracted contents to destination
            extracted_items = list(temp_path.iterdir())
            if len(extracted_items) == 1 and extracted_items[0].is_dir():
                # Single directory extracted
                shutil.move(str(extracted_items[0]), str(dest_path))
            else:
                # Multiple items extracted
                for item in extracted_items:
                    dest_item = dest_path / item.name
                    if item.is_dir():
                        shutil.move(str(item), str(dest_item))
                    else:
                        shutil.move(str(item), str(dest_item))

        print(f"✅ Successfully extracted archive to: {dest_path}")
        return True

    def _download_file_direct(self, context: StorageContext) -> bool:
        """
        Download file directly without extraction.

        Args:
            context: Storage context

        Returns:
            True if download successful
        """
        src_path = Path(context.src_url)
        dest_path = Path(context.dest_url)

        # Ensure destination is a file path
        if dest_path.is_dir():
            dest_path = dest_path / src_path.name

        shutil.copy2(src_path, dest_path)
        print(f"✅ Successfully downloaded file: {dest_path}")
        return True

    def upload_directory(self, context: StorageContext) -> bool:
        """
        Upload directory by compressing to archive.

        Args:
            context: Storage context containing src_url (source directory)
                    and dest_url (destination path)

        Returns:
            True if upload successful
        """
        return self.upload(context)

    def exists(self, context: StorageContext) -> bool:
        """
        Check if a file or directory exists at the given path using storage context.

        Args:
            context: Storage context containing src_url (path to check)

        Returns:
            True if path exists, False otherwise
        """
        return Path(context.src_url).exists()

    def delete(self, context: StorageContext) -> bool:
        """
        Delete a file or directory using storage context.

        Args:
            context: Storage context containing src_url (path to delete)

        Returns:
            True if deletion was successful

        Raises:
            FileNotFoundError: If path doesn't exist
            PermissionError: If lacking permissions to delete
        """
        path_obj = Path(context.src_url)

        if not path_obj.exists():
            raise FileNotFoundError(f"Path not found: {context.src_url}")

        if path_obj.is_file():
            path_obj.unlink()
        else:
            shutil.rmtree(path_obj)

        print(f"✅ Successfully deleted {context.src_url}")
        return True

    def get_compression_info(self) -> dict:
        """
        Get information about current compression settings.

        Returns:
            Dictionary with compression format and supported formats
        """
        return {
            "current_format": self.compression_format,
            "supported_formats": ["zip", "tar", "tar.gz"],
            "description": {
                "zip": "ZIP format with DEFLATE compression",
                "tar": "Uncompressed TAR archive",
                "tar.gz": "Gzip compressed TAR archive",
            },
        }


class AEnvHubStorage(Storage):
    def download(self, context: StorageContext) -> bool:
        pass

    def upload(self, context: StorageContext) -> StorageStatus:
        config = get_config_manager().get_storage_config()
        custom = config.get("custom", {})
        prefix = custom.get("prefix", "")

        work_dir = context.src_url
        infos = context.infos
        if infos:
            name = infos.get("name")
            version = infos.get("version")

        with TempArchive(str(work_dir)) as archive:
            print(f"🔄 Archive: {archive}")
            infos = context.infos
            name = infos.get("name")
            version = infos.get("version")
            with open(archive, "rb") as tar:
                hub_client = AEnvHubClient.load_client()
                oss_url = hub_client.apply_sign_url(name, version)
                headers = {"x-oss-object-acl": "public-read-write"}
                response = requests.put(oss_url, data=tar, headers=headers)
                response.raise_for_status()

        dest = f"{prefix}/{name}-{version}.tar"
        return StorageStatus(state=True, dest_url=dest)


def load_storage():
    store_config = get_config_manager().get_storage_config()
    clazz = store_config.get("type", "local")
    if clazz == "local":
        return LocalStorage()
    elif clazz == "aenv_hub":
        return AEnvHubStorage()
    raise ValueError(f"Unknown storage type: {clazz}")
