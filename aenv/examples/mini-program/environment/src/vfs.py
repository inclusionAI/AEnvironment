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
Virtual File System (VFS) for managing mini program files.
"""

from pathlib import Path
from typing import Dict, List


class VirtualFileSystem:
    """Virtual file system for managing mini program files."""

    def __init__(self, base_dir: str = "/tmp/code"):
        """
        Initialize virtual file system.

        Args:
            base_dir: Base directory for virtual file system (default: /tmp/code)
        """
        self.base_dir = Path(base_dir)
        self.base_dir.mkdir(parents=True, exist_ok=True)
        self.files: Dict[str, str] = {}  # file_path -> content

    def list_files(self, directory: str = "/") -> List[str]:
        """
        List all files in the virtual file system.

        Args:
            directory: Directory path (default: "/")

        Returns:
            List of file paths
        """
        if directory == "/":
            return list(self.files.keys())

        directory = directory.rstrip("/")
        return [
            path
            for path in self.files.keys()
            if path.startswith(directory + "/") or path == directory
        ]

    def read_file(self, file_path: str) -> str:
        """
        Read file content from virtual file system.

        Args:
            file_path: File path

        Returns:
            File content

        Raises:
            FileNotFoundError: If file does not exist
        """
        if file_path not in self.files:
            raise FileNotFoundError(f"File not found: {file_path}")
        return self.files[file_path]

    def write_file(self, file_path: str, content: str) -> bool:
        """
        Write file content to virtual file system.

        Args:
            file_path: File path
            content: File content

        Returns:
            True if successful
        """
        self.files[file_path] = content
        return True

    def delete_file(self, file_path: str) -> bool:
        """
        Delete file from virtual file system.

        Args:
            file_path: File path

        Returns:
            True if successful
        """
        if file_path in self.files:
            del self.files[file_path]
            return True
        return False

    def get_file_tree(self) -> Dict[str, any]:
        """
        Get file tree structure.

        Returns:
            File tree as nested dictionary
        """
        tree = {}
        for file_path in sorted(self.files.keys()):
            parts = file_path.split("/")
            current = tree
            for part in parts[:-1]:
                if part not in current:
                    current[part] = {}
                current = current[part]
            current[parts[-1]] = {"type": "file", "path": file_path}
        return tree


# Global VFS instance
_vfs = VirtualFileSystem()


def get_vfs() -> VirtualFileSystem:
    """Get global VFS instance."""
    return _vfs
