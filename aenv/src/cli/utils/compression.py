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
Parallel compression utilities for aenv CLI

Provides multi-threaded compression using pigz when available,
with automatic fallback to standard gzip.
"""

import logging
import os
import shutil
import subprocess
import tarfile
import tempfile
import time
from pathlib import Path
from typing import List, Optional

from cli.utils.parallel import is_parallel_disabled

logger = logging.getLogger(__name__)


def get_pigz_path() -> Optional[str]:
    """
    Check if pigz is available in the system.

    Returns:
        Path to pigz executable or None if not available
    """
    return shutil.which("pigz")


def get_cpu_count() -> int:
    """Get the number of CPUs available for compression."""
    try:
        return os.cpu_count() or 1
    except Exception:
        return 1


def pack_directory_parallel(
    source_dir: str,
    output_path: Optional[str] = None,
    exclude_patterns: Optional[List[str]] = None,
    use_parallel: bool = True,
    compression_level: int = 6,
) -> str:
    """
    Package directory as tar.gz file using parallel compression when available.

    Uses pigz for multi-threaded compression if available, otherwise falls back
    to standard tarfile compression.

    Args:
        source_dir: Source directory path
        output_path: Output file path, generates temporary file if None
        exclude_patterns: List of file patterns to exclude
        use_parallel: Whether to use parallel compression (default: True)
        compression_level: Compression level 1-9 (default: 6)

    Returns:
        Path to the compressed archive file
    """
    source_path = Path(source_dir)
    if not source_path.exists():
        raise FileNotFoundError(f"Directory does not exist: {source_dir}")

    if output_path is None:
        timestamp = int(time.time())
        filename = f"{source_path.name}_{timestamp}.tar.gz"
        output_path = str(Path(tempfile.gettempdir()) / filename)

    pigz_path = get_pigz_path()
    use_pigz = (
        use_parallel
        and not is_parallel_disabled()
        and pigz_path is not None
    )

    if use_pigz:
        try:
            return _pack_with_pigz(
                source_dir=source_dir,
                output_path=output_path,
                exclude_patterns=exclude_patterns,
                pigz_path=pigz_path,
                compression_level=compression_level,
            )
        except Exception as e:
            logger.warning(f"Pigz compression failed, falling back to tarfile: {e}")

    return _pack_with_tarfile(
        source_dir=source_dir,
        output_path=output_path,
        exclude_patterns=exclude_patterns,
    )


def _pack_with_pigz(
    source_dir: str,
    output_path: str,
    exclude_patterns: Optional[List[str]],
    pigz_path: str,
    compression_level: int = 6,
) -> str:
    """
    Create tar.gz archive using pigz for parallel compression.

    This creates an uncompressed tar first, then pipes it through pigz.
    """
    source_path = Path(source_dir)
    exclude_set = set(exclude_patterns or [])
    cpu_count = get_cpu_count()

    with tempfile.NamedTemporaryFile(suffix=".tar", delete=False) as tmp_tar:
        tmp_tar_path = tmp_tar.name

    try:
        with tarfile.open(tmp_tar_path, "w") as tar:
            for root, dirs, files in os.walk(source_dir):
                dirs[:] = [
                    d for d in dirs
                    if not any(exclude in d for exclude in exclude_set)
                ]
                files = [
                    f for f in files
                    if not any(exclude in f for exclude in exclude_set)
                ]
                for file in files:
                    file_path = Path(root) / file
                    arc_path = file_path.relative_to(source_path.parent)
                    tar.add(file_path, arcname=arc_path)

        with open(tmp_tar_path, "rb") as tar_input:
            with open(output_path, "wb") as gz_output:
                process = subprocess.run(
                    [pigz_path, f"-{compression_level}", "-p", str(cpu_count)],
                    stdin=tar_input,
                    stdout=gz_output,
                    stderr=subprocess.PIPE,
                    check=True,
                )

        logger.info(
            f"Parallel compression completed with pigz ({cpu_count} threads): "
            f"{output_path} ({os.path.getsize(output_path)} bytes)"
        )
        return output_path

    finally:
        if os.path.exists(tmp_tar_path):
            os.unlink(tmp_tar_path)


def _pack_with_tarfile(
    source_dir: str,
    output_path: str,
    exclude_patterns: Optional[List[str]],
) -> str:
    """Create tar.gz archive using standard tarfile module."""
    source_path = Path(source_dir)
    exclude_set = set(exclude_patterns or [])

    with tarfile.open(output_path, "w:gz") as tar:
        for root, dirs, files in os.walk(source_dir):
            dirs[:] = [
                d for d in dirs
                if not any(exclude in d for exclude in exclude_set)
            ]
            files = [
                f for f in files
                if not any(exclude in f for exclude in exclude_set)
            ]
            for file in files:
                file_path = Path(root) / file
                arc_path = file_path.relative_to(source_path.parent)
                tar.add(file_path, arcname=arc_path)

    logger.info(
        f"Standard compression completed: {output_path} "
        f"({os.path.getsize(output_path)} bytes)"
    )
    return output_path
