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

"""Tests for parallel compression utilities."""

import os
import tarfile
import tempfile
from pathlib import Path
from unittest.mock import patch

import pytest

from cli.utils.compression import (
    get_pigz_path,
    get_cpu_count,
    pack_directory_parallel,
    _pack_with_tarfile,
)


@pytest.fixture
def temp_source_dir():
    """Create a temporary directory with test files."""
    with tempfile.TemporaryDirectory() as tmpdir:
        source_dir = Path(tmpdir) / "test_source"
        source_dir.mkdir()

        (source_dir / "file1.txt").write_text("content1")
        (source_dir / "file2.txt").write_text("content2")

        subdir = source_dir / "subdir"
        subdir.mkdir()
        (subdir / "file3.txt").write_text("content3")

        pycache = source_dir / "__pycache__"
        pycache.mkdir()
        (pycache / "cache.pyc").write_text("cached")

        yield str(source_dir)


class TestGetPigzPath:
    def test_returns_path_when_available(self):
        with patch("shutil.which") as mock_which:
            mock_which.return_value = "/usr/bin/pigz"
            result = get_pigz_path()
            assert result == "/usr/bin/pigz"

    def test_returns_none_when_not_available(self):
        with patch("shutil.which") as mock_which:
            mock_which.return_value = None
            result = get_pigz_path()
            assert result is None


class TestGetCpuCount:
    def test_returns_positive_number(self):
        count = get_cpu_count()
        assert count >= 1

    def test_handles_exception(self):
        with patch("os.cpu_count", side_effect=Exception("error")):
            count = get_cpu_count()
            assert count == 1


class TestPackDirectoryParallel:
    def test_creates_archive(self, temp_source_dir):
        output_path = pack_directory_parallel(temp_source_dir, use_parallel=False)

        try:
            assert os.path.exists(output_path)
            assert output_path.endswith(".tar.gz")

            with tarfile.open(output_path, "r:gz") as tar:
                names = tar.getnames()
                assert any("file1.txt" in n for n in names)
                assert any("file2.txt" in n for n in names)
                assert any("file3.txt" in n for n in names)
        finally:
            if os.path.exists(output_path):
                os.unlink(output_path)

    def test_excludes_patterns(self, temp_source_dir):
        output_path = pack_directory_parallel(
            temp_source_dir,
            exclude_patterns=["__pycache__"],
            use_parallel=False,
        )

        try:
            with tarfile.open(output_path, "r:gz") as tar:
                names = tar.getnames()
                assert not any("__pycache__" in n for n in names)
                assert not any("cache.pyc" in n for n in names)
        finally:
            if os.path.exists(output_path):
                os.unlink(output_path)

    def test_custom_output_path(self, temp_source_dir):
        with tempfile.TemporaryDirectory() as tmpdir:
            custom_path = os.path.join(tmpdir, "custom_archive.tar.gz")
            output_path = pack_directory_parallel(
                temp_source_dir,
                output_path=custom_path,
                use_parallel=False,
            )

            assert output_path == custom_path
            assert os.path.exists(custom_path)

    def test_raises_for_nonexistent_dir(self):
        with pytest.raises(FileNotFoundError):
            pack_directory_parallel("/nonexistent/path")

    def test_falls_back_to_tarfile_when_parallel_disabled(self, temp_source_dir):
        with patch.dict(os.environ, {"AENV_DISABLE_PARALLEL": "1"}):
            output_path = pack_directory_parallel(temp_source_dir)

            try:
                assert os.path.exists(output_path)
            finally:
                if os.path.exists(output_path):
                    os.unlink(output_path)

    def test_uses_pigz_when_available(self, temp_source_dir):
        with (
            patch("cli.utils.compression.get_pigz_path") as mock_pigz,
            patch("cli.utils.compression._pack_with_pigz") as mock_pack_pigz,
            patch.dict(os.environ, {}, clear=True),
        ):
            os.environ.pop("AENV_DISABLE_PARALLEL", None)
            mock_pigz.return_value = "/usr/bin/pigz"
            mock_pack_pigz.return_value = "/tmp/test.tar.gz"

            pack_directory_parallel(temp_source_dir)

            mock_pack_pigz.assert_called_once()

    def test_falls_back_when_pigz_fails(self, temp_source_dir):
        with (
            patch("cli.utils.compression.get_pigz_path") as mock_pigz,
            patch("cli.utils.compression._pack_with_pigz") as mock_pack_pigz,
            patch.dict(os.environ, {}, clear=True),
        ):
            os.environ.pop("AENV_DISABLE_PARALLEL", None)
            mock_pigz.return_value = "/usr/bin/pigz"
            mock_pack_pigz.side_effect = Exception("pigz failed")

            output_path = pack_directory_parallel(temp_source_dir)

            try:
                assert os.path.exists(output_path)
            finally:
                if os.path.exists(output_path):
                    os.unlink(output_path)


class TestPackWithTarfile:
    def test_creates_valid_archive(self, temp_source_dir):
        with tempfile.TemporaryDirectory() as tmpdir:
            output_path = os.path.join(tmpdir, "test.tar.gz")
            result = _pack_with_tarfile(temp_source_dir, output_path, None)

            assert result == output_path
            assert os.path.exists(output_path)

            with tarfile.open(output_path, "r:gz") as tar:
                assert len(tar.getnames()) > 0

    def test_respects_exclude_patterns(self, temp_source_dir):
        with tempfile.TemporaryDirectory() as tmpdir:
            output_path = os.path.join(tmpdir, "test.tar.gz")
            _pack_with_tarfile(
                temp_source_dir,
                output_path,
                ["__pycache__", "file1"],
            )

            with tarfile.open(output_path, "r:gz") as tar:
                names = tar.getnames()
                assert not any("__pycache__" in n for n in names)
                assert not any("file1" in n for n in names)
                assert any("file2" in n for n in names)
