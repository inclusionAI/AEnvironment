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
Concise and efficient archiving tool
Implements directory packaging and cleanup functionality, supports classmethod and with operations
"""

import logging
import os
import shutil
import tarfile
import tempfile
import time
from pathlib import Path
from typing import List, Optional


class ArchiveTool:
    """Concise and efficient archiving tool - all methods are classmethod"""

    _logger = None

    @classmethod
    def _get_logger(cls):
        if cls._logger is None:
            cls._logger = logging.getLogger(__name__)
        return cls._logger

    @classmethod
    def pack_directory(
        cls,
        source_dir: str,
        output_path: Optional[str] = None,
        compression: str = "gz",
        exclude_patterns: Optional[List[str]] = None,
    ) -> str:
        """
        Package directory as tar file

        Args:
            source_dir: Source directory path
            output_path: Output file path, use temporary file if None
            compression: Compression format ('gz', 'bz2', 'xz')
            exclude_patterns: List of file patterns to exclude
        Returns:
            Package file path
        """
        logger = cls._get_logger()
        source_path = Path(source_dir)
        if not source_path.exists():
            raise FileNotFoundError(f"Directory does not exist: {source_dir}")

        # Generate output filename
        if output_path is None:
            timestamp = int(time.time())
            filename = f"{source_path.name}_{timestamp}.tar.{compression}"
            output_path = str(Path(tempfile.gettempdir()) / filename)

        # Exclude patterns
        exclude_set = set(exclude_patterns or [])
        # Package file
        mode = f"w:{compression}" if compression != "none" else "w"

        with tarfile.open(output_path, mode) as tar:
            for root, dirs, files in os.walk(source_dir):
                # Filter excluded files and directories
                dirs[:] = [
                    d for d in dirs if not any(exclude in d for exclude in exclude_set)
                ]
                files = [
                    f for f in files if not any(exclude in f for exclude in exclude_set)
                ]
                for file in files:
                    file_path = Path(root) / file
                    arc_path = file_path.relative_to(source_path.parent)
                    tar.add(file_path, arcname=arc_path)
        logger.info(
            f"✅ Packaging completed: {output_path} ({os.path.getsize(output_path)} bytes)"
        )
        return output_path

    @classmethod
    def pack_and_cleanup(
        cls, source_dir: str, remove_source: bool = False, **pack_kwargs
    ) -> str:
        """Package and optionally cleanup source directory"""
        archive_path = cls.pack_directory(source_dir, **pack_kwargs)

        if remove_source:
            cls.cleanup_directory(source_dir)

        return archive_path

    @classmethod
    def cleanup_directory(cls, directory: str, force: bool = False) -> bool:
        """Cleanup directory"""
        logger = cls._get_logger()
        dir_path = Path(directory)
        if not dir_path.exists():
            logger.warning(f"Directory does not exist: {directory}")
            return False

        try:
            if dir_path.is_file():
                dir_path.unlink()
            else:
                shutil.rmtree(dir_path)

            logger.info(f"🗑️ Cleanup completed: {directory}")
            return True

        except Exception as e:
            logger.error(f"Cleanup failed: {directory} - {e}")
            return False

    @classmethod
    def cleanup_archive(cls, archive_path: str) -> bool:
        """Cleanup archive file"""
        logger = cls._get_logger()
        archive_file = Path(archive_path)
        if not archive_file.exists():
            logger.warning(f"Archive file does not exist: {archive_path}")
            return False

        try:
            archive_file.unlink()
            logger.info(f"🗑️ Archive file cleanup completed: {archive_path}")
            return True
        except Exception as e:
            logger.error(f"Archive file cleanup failed: {archive_path} - {e}")
            return False

    @classmethod
    def get_archive_info(cls, archive_path: str) -> dict:
        """Get archive file information"""
        if not os.path.exists(archive_path):
            raise FileNotFoundError(f"Archive file does not exist: {archive_path}")

        with tarfile.open(archive_path, "r") as tar:
            members = tar.getmembers()

        return {
            "path": archive_path,
            "size": os.path.getsize(archive_path),
            "file_count": len(members),
            "compression": (
                archive_path.split(".")[-1] if "." in archive_path else "none"
            ),
            "members": [member.name for member in members[:10]],  # First 10 files
        }

    @classmethod
    def extract_archive(
        cls,
        archive_path: str,
        extract_dir: Optional[str] = None,
        cleanup_archive: bool = False,
    ) -> str:
        """Extract archive file"""
        logger = cls._get_logger()
        if not os.path.exists(archive_path):
            raise FileNotFoundError(f"Archive file does not exist: {archive_path}")

        if extract_dir is None:
            extract_dir = os.path.splitext(os.path.basename(archive_path))[0]

        extract_path = Path(extract_dir)
        extract_path.mkdir(parents=True, exist_ok=True)

        with tarfile.open(archive_path, "r") as tar:
            tar.extractall(extract_path)

        logger.info(f"📦 Extraction completed: {archive_path} -> {extract_path}")

        if cleanup_archive:
            cls.cleanup_archive(archive_path)

        return str(extract_path)


class ArchiveContext:
    """Archive context manager supporting with operations"""

    def __init__(self, *paths: str):
        self.paths = paths
        self.success = True

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.cleanup()

    def cleanup(self):
        """Cleanup all paths"""
        for path in self.paths:
            ArchiveTool.cleanup_directory(path)


class ArchiveCleanup:
    """Archive cleanup tool supporting with operations"""

    def __init__(self, *paths: str):
        self.paths = paths

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        for path in self.paths:
            ArchiveTool.cleanup_directory(path)


def quick_pack(source_dir: str, remove_source: bool = False, **pack_kwargs) -> str:
    return ArchiveTool.pack_and_cleanup(
        source_dir, remove_source=remove_source, **pack_kwargs
    )


def quick_cleanup(*paths: str) -> bool:
    success = True
    for path in paths:
        if not ArchiveTool.cleanup_directory(path):
            success = False
    return success


# Convenient class supporting with operations
class TempArchive:
    """Temporary archive context manager"""

    def __init__(self, source_dir: str, **pack_kwargs):
        self.source_dir = source_dir
        self.pack_kwargs = pack_kwargs
        self.archive_path = None

        if pack_kwargs is None:
            pack_kwargs = {"exclude_patterns": ["__pycache__"]}
            self.pack_kwargs = pack_kwargs

    def __enter__(self):
        self.archive_path = ArchiveTool.pack_directory(
            self.source_dir, **self.pack_kwargs
        )
        return self.archive_path

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.archive_path and os.path.exists(self.archive_path):
            ArchiveTool.cleanup_archive(self.archive_path)
